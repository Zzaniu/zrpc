/*
Author ：zaniu(zzaniu@126.com)
Time   ：2022/3/7 17:55
Desc   :

    ......................我佛慈悲......................

                           _oo0oo_
                          o8888888o
                          88" . "88
                          (| -_- |)
                          0\  =  /0
                        ___/`---'\___
                      .' \\|     |// '.
                     / \\|||  :  |||// \
                    / _||||| -卍-|||||- \
                   |   | \\\  -  /// |   |
                   | \_|  ''\---/''  |_/ |
                   \  .-\__  '-'  ___/-. /
                 ___'. .'  /--.--\  `. .'___
              ."" '<  `.___\_<|>_/___.' >' "".
             | | :  `- \`.;`\ _ /`;.`/ - ` : | |
             \  \ `_.   \_ __\ /__ _/   .-` /  /
         =====`-.____`.___ \_____/___.-`___.-'=====
                           `=---='

    ..................佛祖保佑, 永无BUG...................

*/

package limiter

// 自动限流算法 kratos-bbr https://github.com/go-kratos/aegis/blob/main/ratelimit/bbr/bbr.go

import (
	"bytes"
	"context"
	"errors"
	"github.com/Zzaniu/zrpc/tool/cpu"
	"github.com/Zzaniu/zrpc/tool/window"
	"golang.org/x/sync/singleflight"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"math"
	"strconv"
	"sync/atomic"
	"time"
)

var (
	singleFlight     singleflight.Group
	LimiterDropError = errors.New("drop request")
	decay            = 0.95
	gCPU             uint32 // CPU使用量能有多大呢, uint32完全完全够够的了
)

func cpuProc() {
	ticker := time.NewTicker(time.Millisecond * 500) // same to cpu sample rate
	defer func() {
		ticker.Stop()
		if err := recover(); err != nil {
			// 除非是无法recover的异常, 否则就是挂了就重启挂了就重启...
			go cpuProc()
		}
	}()

	// 滑动均值: https://blog.csdn.net/m0_38106113/article/details/81542863
	for range ticker.C {
		stat := &cpu.Stat{}
		// 获取全局变量 usage, 有定时任务在每隔500ms刷新这个值. 这个值是500ms内CPU的使用率
		stat.ReadStat()
		// 获取全局变量 gCPU, 保存的是上一次通过指数加权平均算法计算出来的CPU使用率
		prevCPU := atomic.LoadUint32(&gCPU)
		// 用指数加权平均算法计算这一次的CPU使用量  stat.Usage: 是CPU使用量乘以10
		curCPU := uint32(float64(prevCPU)*decay + float64(stat.Usage)*(1.0-decay))
		// fmt.Printf("stat.Usage = %d, prevCPU = %d, curCPU = %d\n", stat.Usage, prevCPU, curCPU)
		// 将这次的值刷到gCPU中去, 下次取的prevCPU就是这次计算的
		atomic.StoreUint32(&gCPU, curCPU)
	}
}

func init() {
	go cpuProc()
}

type (
	DoneFunc func()

	BBR interface {
		Allow() (DoneFunc, error)
	}

	counterCache struct {
		val  uint64
		time time.Time
	}

	bbr struct {
		inFlight int64          // 系统当前请求数
		passStat window.Rolling // 流出统计窗口
		rt       window.Rolling // 请求耗时窗口

		cpuThreshold    uint32        // cpu阈值
		bucketPerSecond uint64        // 每秒多少个桶
		bucketDuration  time.Duration // 桶时长

		prevDropTime atomic.Value // 上次限流的时间
		maxPassCache atomic.Value // 最大通过缓存
		minRtCache   atomic.Value // 最小耗时缓存
	}

	Option func(*options)

	options struct {
		// 桶时长
		BucketDuration time.Duration
		// 桶数量
		Bucket int
		// CPUThreshold
		CpuThreshold uint32
	}
)

// WithCpuThreshold 修改 cpuThreshold 参数
func WithCpuThreshold(cpuThreshold uint32) Option {
	return func(opt *options) {
		opt.CpuThreshold = cpuThreshold
	}
}

// WithBucketNum 修改 BucketNum
func WithBucketNum(bucketSize int) Option {
	return func(opt *options) {
		opt.Bucket = bucketSize
	}
}

// WithBucketDuration 修改 bucketDuration
func WithBucketDuration(bucketDuration time.Duration) Option {
	return func(opt *options) {
		opt.BucketDuration = bucketDuration
	}
}

func NewBbrLimiter(opts ...Option) BBR {
	opt := options{
		BucketDuration: time.Millisecond * 250, // 一个bucket, 250ms  这个值越大，拒绝越积极
		Bucket:         50,                     // 一个桶100ms
		CpuThreshold:   800,                    // cpu阈值
	}

	for _, o := range opts {
		o(&opt)
	}

	return &bbr{
		passStat:        window.NewRollingPolicy(window.WithBucket(opt.Bucket), window.WithBucketDuration(opt.BucketDuration)),
		rt:              window.NewRollingPolicy(window.WithBucket(opt.Bucket), window.WithBucketDuration(opt.BucketDuration)),
		cpuThreshold:    opt.CpuThreshold,
		bucketPerSecond: uint64(time.Second / opt.BucketDuration),
		bucketDuration:  opt.BucketDuration,
	}
}

func (b *bbr) cpu() uint32 {
	return atomic.LoadUint32(&gCPU)
}

func (b *bbr) timespan(lastTime time.Time) int {
	span := int(time.Since(lastTime) / b.bucketDuration)
	if span > -1 {
		return span
	}
	return 0
}

func (b *bbr) maxInFlight() uint64 {
	// v := b.maxFlight()*b.bucketPerSecond / 1000.0  1ms多少个请求  因为 rt 存储的也是 ms
	// T ≈ QPS * Avg(RT)
	return uint64(math.Floor(float64(b.maxFlight()*b.bucketPerSecond*b.minRt())/1000.0 + 0.5))
}

// maxFlight 获取最大请求数
// 用 singleFlight 或者 atomic.value ? 不知道到底哪个性能好哎, 有条件的话可以测试一下
// 个人认为 singleFlight 在并发很大的时候会有优势, 因为同一时刻只会有一个 go 程去计算
func (b *bbr) maxFlight() uint64 {
	var span int
	if maxPassCache, ok := b.maxPassCache.Load().(*counterCache); ok {
		span = b.timespan(maxPassCache.time)
		if span < 1 {
			return maxPassCache.val
		}
	}

	buffer := bytes.NewBuffer(make([]byte, 0, 20))
	buffer.WriteString("maxFlight-")
	buffer.WriteString(strconv.Itoa(span))
	doRet, _, _ := singleFlight.Do(buffer.String(), func() (interface{}, error) {
		var maxPass uint64
		b.passStat.Reduce(func(bucket *window.Bucket) {
			if bucket.Sum > maxPass {
				maxPass = bucket.Sum
			}
		})
		if maxPass < 1 {
			maxPass = 1
		}
		b.maxPassCache.Store(&counterCache{time: time.Now(), val: maxPass})
		return maxPass, nil
	})
	return doRet.(uint64)
}

// minRt 获取最小耗时
func (b *bbr) minRt() uint64 {
	var span int
	if minRtCache, ok := b.minRtCache.Load().(*counterCache); ok {
		span = b.timespan(minRtCache.time)
		if span < 1 {
			return minRtCache.val
		}
	}

	buffer := bytes.NewBuffer(make([]byte, 0, 20))
	buffer.WriteString("minRt-")
	buffer.WriteString(strconv.Itoa(span))
	doRet, _, _ := singleFlight.Do(buffer.String(), func() (interface{}, error) {
		// 初始化为最大 Uint64
		var rt uint64 = math.MaxUint64
		b.rt.Reduce(func(bucket *window.Bucket) {
			if bucket.Count == 0 || bucket.Sum == 0 {
				// 为零说明是无效桶, 直接返回即可
				return
			}
			t := bucket.Sum / bucket.Count
			// 取最小值
			if t < rt {
				rt = t
			}
		})
		b.minRtCache.Store(&counterCache{time: time.Now(), val: rt})
		return rt, nil
	})
	return doRet.(uint64)
}

// shouldDrop 判断该请求是否应该被丢弃
func (b *bbr) shouldDrop() bool {
	now := time.Duration(time.Now().UnixNano())
	// cpu 是否达到阈值
	if b.cpu() < b.cpuThreshold {
		// 取上次限流时间
		prevDropTime, _ := b.prevDropTime.Load().(time.Duration)
		if prevDropTime == 0 {
			return false
		}

		// 判定上次限流开始时间距现在如果小于1S, 判断当前负载(请求量)是否大于过去最大负载, 是则触发限流
		if now-prevDropTime <= time.Second {
			inFlight := uint64(atomic.LoadInt64(&b.inFlight))
			return inFlight > 1 && inFlight > b.maxInFlight()
		}

		// CPU已经降下来了, 并且距离开始限流超过1S, 则清除上次限流开始时间 prevDropTime
		b.prevDropTime.Store(time.Duration(0))
		return false
	}

	// 说明CPU已经超过阈值, 这个时候要看负载是不是大于之前最大负载, 是则触发限流, 并设置 prevDropTime. 否也不会清除 prevDropTime
	inFlight := uint64(atomic.LoadInt64(&b.inFlight))
	maxInFlight := b.maxInFlight()
	drop := inFlight > 1 && inFlight > maxInFlight
	if drop {
		prevDrop, _ := b.prevDropTime.Load().(time.Duration)
		// 如果已经设置了限流时间, 直接返回
		if prevDrop != 0 {
			return drop
		}
		// 设置一下限流开始时间
		b.prevDropTime.Store(now)
	}
	return false
}

// Allow 判断该请求是否允许通过
func (b *bbr) Allow() (DoneFunc, error) {
	// 限流了, 这个请求要被抛弃
	if b.shouldDrop() {
		return nil, LimiterDropError
	}
	start := time.Now().UnixNano()
	atomic.AddInt64(&b.inFlight, 1) // 系统请求加一
	return func() {
		atomic.AddInt64(&b.inFlight, -1)                                            // 系统请求减一
		b.passStat.Add(1)                                                           // 请求通过
		b.rt.Add(uint64((time.Now().UnixNano() - start) / int64(time.Millisecond))) // 存储这个请求通过的时间, ms
	}, nil
}

func WithServerLimiterInterceptor() grpc.UnaryServerInterceptor {
	limiter := NewBbrLimiter()
	return func(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo,
		handler grpc.UnaryHandler) (interface{}, error) {
		doneFunc, err := limiter.Allow()
		if err != nil {

			return nil, status.New(codes.Unavailable, err.Error()).Err()
		}
		resp, err := handler(ctx, req)
		doneFunc()
		return resp, err
	}
}

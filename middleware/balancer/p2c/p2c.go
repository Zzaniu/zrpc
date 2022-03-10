/*
Author ：zaniu(zzaniu@126.com)
Time   ：2022/3/3 10:29
Desc   : 链接：https://github.com/zeromicro/go-zero/blob/master/zrpc/internal/balancer/p2c/p2c.go

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

package p2c

import (
	"github.com/Zzaniu/zrpc/utils/errcode"
	"github.com/Zzaniu/zrpc/utils/timex"
	"google.golang.org/grpc/balancer"
	"google.golang.org/grpc/balancer/base"
	"google.golang.org/grpc/resolver"
	"math"
	"math/rand"
	"sync"
	"sync/atomic"
	"time"
)

const (
	// Name is the name of p2c balancer.
	Name = "p2c"

	decayTime       = int64(time.Second * 10) // default value from finagle
	forcePick       = int64(time.Second)
	initSuccess     = 1000
	throttleSuccess = initSuccess / 2
	penalty         = int64(math.MaxInt32)
	pickTimes       = 3
)

var emptyPickResult balancer.PickResult

type (
	PickerBuilderP2c struct{}

	p2cPicker struct {
		conns []*subConn
		r     *rand.Rand
		lock  sync.Mutex
	}

	subConn struct {
		addr     resolver.Address // 链接地址
		conn     balancer.SubConn // 链接对接
		lag      uint64           // 指数移动加权平均法 求出来的请求耗时...
		inflight int64            // 当前处理请求数
		success  uint64           // 健康值
		last     int64            // 上一次请求结束的时间(这个是时间不是标准时间，是一个时间差)
		pick     int64            // pick 记录的时间(这个是时间不是标准时间，是一个时间差)
	}
)

// Build 每次服务更新的时候也会调用(启动的时候其实服务就会发生更新)
func (b *PickerBuilderP2c) Build(info base.PickerBuildInfo) balancer.Picker {
	// ReadySCs 所有的 conn 映射
	readySCs := info.ReadySCs
	// 没有 coon 的话就返回 ErrNoSubConnAvailable
	if len(readySCs) == 0 {
		return base.NewErrPicker(balancer.ErrNoSubConnAvailable)
	}

	var conns []*subConn
	// 给 conn 包装一下，附带一些信息
	for conn, connInfo := range readySCs {
		conns = append(conns, &subConn{
			addr:    connInfo.Address,
			conn:    conn,
			success: initSuccess,
		})
	}

	return &p2cPicker{
		conns: conns,
		r:     rand.New(rand.NewSource(time.Now().UnixNano())),
	}
}

// Pick 挑选 coon 进行使用
func (p *p2cPicker) Pick(info balancer.PickInfo) (balancer.PickResult, error) {
	p.lock.Lock()
	defer p.lock.Unlock()

	var chosen *subConn
	switch len(p.conns) {
	case 0:
		return emptyPickResult, balancer.ErrNoSubConnAvailable
	case 1:
		chosen = p.choose(p.conns[0], nil)
	case 2:
		chosen = p.choose(p.conns[0], p.conns[1])
	default:
		var node1, node2 *subConn
		for i := 0; i < pickTimes; i++ {
			a := p.r.Intn(len(p.conns))
			b := p.r.Intn(len(p.conns) - 1)
			if b >= a {
				b++
			}
			node1 = p.conns[a]
			node2 = p.conns[b]
			// 如果两个都健康的话, 就停止选择
			if node1.healthy() && node2.healthy() {
				break
			}
		}

		// 选择一个负载小的
		chosen = p.choose(node1, node2)
	}

	atomic.AddInt64(&chosen.inflight, 1)

	return balancer.PickResult{
		SubConn: chosen.conn,
		Done:    p.buildDoneFunc(chosen),
	}, nil
}

// buildDoneFunc 调用完了之后，进行一些统计。比如健康值、更新请求总数、请求时间等
func (p *p2cPicker) buildDoneFunc(c *subConn) func(info balancer.DoneInfo) {
	// 请求开始时间
	start := int64(timex.Now())
	return func(info balancer.DoneInfo) {
		// 当前系统对该服务的 当前处理请求数 减一
		atomic.AddInt64(&c.inflight, -1)
		// 请求结束时间
		now := timex.Now()
		// 保存这次请求结束时间，并返回上次请求结束时间
		last := atomic.SwapInt64(&c.last, int64(now))
		td := int64(now) - last
		if td < 0 {
			td = 0
		}
		// 这里是一个牛顿冷却法, 我也不知道这个牛顿冷却法是个啥玩意... 直接抄吧...
		// 公式: β = 1/e**(k*(t2-t1))
		// w 是 通过牛顿冷却法计算出来的指数移动加权平均法的系数 β
		// 是-td而不是td, 是因为是1/嘛, 取倒数就是 -td 了
		// k = 1 / float64(decayTime)
		w := math.Exp(float64(-td) / float64(decayTime))

		// 请求耗时
		lag := int64(now) - start
		if lag < 0 {
			lag = 0
		}
		// EWMA (Exponentially Weighted Moving-Average) 指数移动加权平均法
		// 加载上一次的 EWMA 值
		olag := atomic.LoadUint64(&c.lag)
		if olag == 0 {
			w = 0
		}
		// 滑动均值: https://blog.csdn.net/m0_38106113/article/details/81542863
		// 更新 EWMA 值   uint64(float64(olag)*w+float64(lag)*(1-w))  这个就是指数移动加权平均法
		atomic.StoreUint64(&c.lag, uint64(float64(olag)*w+float64(lag)*(1-w)))
		// 健康值，如果返回错误是指定的错误，健康值为 0 分
		success := initSuccess
		if info.Err != nil && !errcode.Acceptable(info.Err) {
			success = 0
		}
		osucc := atomic.LoadUint64(&c.success)
		// 健康值 也是用 指数移动加权平均法 计算
		atomic.StoreUint64(&c.success, uint64(float64(osucc)*w+float64(success)*(1-w)))
	}
}

// choose 选择 coon 这里是选择负载小的
func (p *p2cPicker) choose(c1, c2 *subConn) *subConn {
	start := int64(timex.Now())
	if c2 == nil {
		atomic.StoreInt64(&c1.pick, start)
		return c1
	}

	// 比较一下负载情况, 将负载大的给到 c2
	if c1.load() > c2.load() {
		c1, c2 = c2, c1
	}

	// 看下负载大的那个节点, 先加载上次挑选的时间
	pick := atomic.LoadInt64(&c2.pick)
	// 如果负载大的节点已经超过1S没有被选择, 且当前没有别的选择这个节点, 那就取这个节点
	// CompareAndSwapInt64 相等就交换, 交换了返回 true, 否则返回 false
	if start-pick > forcePick && atomic.CompareAndSwapInt64(&c2.pick, pick, start) {
		return c2
	}

	// 否则取负载小的节点, 更新节点pick的时间
	atomic.StoreInt64(&c1.pick, start)
	return c1
}

// healthy 判断是否健康
func (c *subConn) healthy() bool {
	return atomic.LoadUint64(&c.success) > throttleSuccess
}

// load 计算节点的负载情况
func (c *subConn) load() int64 {
	// 加 1 避免 0 的情况
	lag := int64(math.Sqrt(float64(atomic.LoadUint64(&c.lag) + 1)))
	// 通过 请求耗时 * 处理总数 计算出一个大概的负载情况
	load := lag * (atomic.LoadInt64(&c.inflight) + 1)
	if load == 0 {
		return penalty
	}

	return load
}

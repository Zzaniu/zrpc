package redis_cache

import (
	"fmt"
	"github.com/go-redis/redis"
	"golang.org/x/sync/singleflight"
	"golang.org/x/xerrors"
	"math/rand"
	"sync"
	"sync/atomic"
	"time"
	"zrpc/tool/cache"
	"zrpc/tool/xlog"
)

const (
	invalidCacheCode  = "-410" // 无效的缓存
	redisOK           = "OK"
	redisBulkNum      = 10
	redisExpireBase   = 10
	TenMinute         = time.Minute * 10
	ThirtyMinute      = time.Minute * 30
	OneHour           = time.Hour
	TwelveHour        = time.Hour * 12
	OneDay            = OneHour * 24
	ThreeDay          = OneDay * 3
	SevenDay          = OneDay * 7
	SevenDayInt       = 60 * 60 * 24 * 7
	redisGetScriptStr = `local ret = redis.call("GET", KEYS[1])
							if ret == ARGV[1] then
								redis.call("DEL", KEYS[1])
							end
							return ret`
)

var (
	once              sync.Once
	redisCache        *RedisCache
	redisGetScript    = redis.NewScript(redisGetScriptStr)
	redisGetScriptSha atomic.Value
)

type RedisCache struct {
	singleFlight singleflight.Group
	client       redis.Cmdable
	random       *rand.Rand
}

// NewRedisCache 实例化一个缓存结构
func NewRedisCache(client redis.Cmdable) cache.Cache {
	once.Do(func() {
		redisCache = &RedisCache{client: client, random: rand.New(rand.NewSource(time.Now().UnixNano()))}
	})
	return redisCache
}

// randomSecond100 10-100以内的随机时间
func (r *RedisCache) randomSecond100() time.Duration {
	return time.Second * time.Duration(redisExpireBase+r.random.Intn(90))
}

func (r *RedisCache) random100() int {
	return redisExpireBase + r.random.Intn(90)
}

// Store 执行 set key value ex nx
// 不允许缓存不设置过期时间
func (r *RedisCache) store(key string, value interface{}, expiration int) (bool, error) {
	if len(key) == 0 {
		return false, nil
	}
	result, err := r.client.SetNX(key, value, time.Duration(expiration+r.random100())*time.Second).Result()
	if err != nil {
		return false, xerrors.Errorf("Store SetNX error: %w", err)
	}
	return result, nil
}

// Get 如果缓存不存在, 执行 set key value ex nx, 如果缓存是 invalidCacheCode, 执行 set key value ex
// 如果缓存无效或不存在, 会执行传入的函数去获取数据, 然后存到缓存
func (r *RedisCache) Get(key string, f func() (string, error)) (string, error) {
	doRet, err, _ := r.singleFlight.Do(fmt.Sprintf("Get:%s", key), func() (interface{}, error) {

		sha, ok := redisGetScriptSha.Load().(string)
		if !ok {
			var err error
			sha, err = redisGetScript.Load(r.client).Result()
			if err != nil {
				return nil, xerrors.Errorf("Get Load error: %w", err)
			}
			redisGetScriptSha.Store(sha)
		}

		result, err := r.client.EvalSha(sha, []string{key}, invalidCacheCode).Result()
		if err != nil && err != redis.Nil {
			return nil, xerrors.Errorf("Get EvalSha error: %w", err)
		}

		if err == redis.Nil || result.(string) == invalidCacheCode {
			result, err = f()
			if err != nil {
				return nil, err
			}
			ret, err := r.store(key, result, SevenDayInt+r.random100())
			if err != nil {
				return nil, err
			}

			if !ret {
				xlog.XLog.Warnf("设置缓存失败, key = %v, val = %v\n", key, result)
			}
		}

		return result, nil
	})

	if err != nil {
		return "", err
	}
	return doRet.(string), nil
}

// MGet 批量获取
func (r *RedisCache) MGet(keys ...string) ([]interface{}, error) {
	keysLength := len(keys)
	if keysLength > redisBulkNum {
		return nil, xerrors.Errorf("一次最多操作 %d 个", redisBulkNum)
	}
	for i := 0; i < keysLength; i++ {
		if len(keys[i]) == 0 {
			return nil, xerrors.Errorf("第%d个key是空值", i)
		}
	}

	result, err := r.client.MGet(keys...).Result()
	if err != nil {
		return nil, xerrors.Errorf("MGet MGet error: %w", err)
	}
	return result, nil
}

// Del 软删除, 就是设置一下状态为 invalidCacheCode
func (r *RedisCache) Del(key string) (bool, error) {
	doRet, err, _ := r.singleFlight.Do(fmt.Sprintf("Del:%s", key), func() (interface{}, error) {
		result, err := r.client.Set(key, invalidCacheCode, TenMinute).Result()
		if err != nil {
			return false, xerrors.Errorf("Del Set error: %w", err)
		}
		return result == redisOK, nil
	})
	if err != nil {
		return false, err
	}
	return doRet.(bool), nil
}

// MDel 批量软删除
func (r *RedisCache) MDel(keys ...string) ([]bool, error) {
	keysLength := len(keys)
	if keysLength > redisBulkNum {
		return nil, xerrors.Errorf("一次最多操作 %d 个", redisBulkNum)
	}
	for i := 0; i < keysLength; i++ {
		if len(keys[i]) == 0 {
			return nil, xerrors.Errorf("第%d个key是空值", i)
		}
	}

	pipeline := r.client.Pipeline()
	for _, key := range keys {
		pipeline.Set(key, invalidCacheCode, TenMinute)
	}
	res, err := pipeline.Exec()
	if err != nil {
		return nil, xerrors.Errorf("MDel pipeline.Exec error: %w", err)
	}
	ret := make([]bool, 0, keysLength)
	for _, cmdRes := range res {
		// 处理方式和直接调用同样处理即可
		cmd, ok := cmdRes.(*redis.StatusCmd)
		if ok {
			val, err := cmd.Result()
			if err != nil {
				return nil, xerrors.Errorf("MDel pipeline.Exec error: %w", err)
			}
			ret = append(ret, val == redisOK)
		}
	}
	return ret, nil
}

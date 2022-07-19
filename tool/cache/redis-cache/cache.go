package redis_cache

import (
    "fmt"
    "github.com/Zzaniu/zrpc/tool/cache"
    "github.com/Zzaniu/zrpc/tool/zlog"
    "github.com/go-redis/redis"
    "golang.org/x/sync/singleflight"
    "golang.org/x/xerrors"
    "math/rand"
    "sync/atomic"
    "time"
)

const (
    invalidCacheCode  = "-410" // 无效的缓存
    redisOK           = "OK"
    redisBulkNum      = 100
    redisExpireBase   = 10
    TenMinute         = time.Minute * 10
    ThirtyMinute      = time.Minute * 30
    OneHour           = time.Hour
    TwelveHour        = time.Hour * 12
    OneDay            = OneHour * 24
    ThreeDay          = OneDay * 3
    SevenDay          = OneDay * 7
    SevenDayInt       = 60 * 60 * 24 * 7
    placeholderTime   = time.Minute
    redisGetScriptStr = `local ret = redis.call("GET", KEYS[1])
                            if ret == ARGV[1] then
                                redis.call("DEL", KEYS[1])
                            end
                            return ret`
    // 在 set 失败的时候, 返回值是 bool, 在 set 成功时, 需要用 .ok 来判断
    redisStoreScriptStr = `local invalidCode = ARGV[3]
                            local ret1 = redis.call("set", KEYS[1], ARGV[1], "px", ARGV[2], "nx")
                            if ret1 ~= false and ret1.ok == "OK" then
                                return 1
                            end
                            local val = redis.call("get", KEYS[1])
                            if val == ARGV[1] or val == invalidCode then
                                return 1
                            end
                            local ret2 = redis.call("set", KEYS[1], invalidCode, "px", ARGV[4])
                            if ret2 ~= false and ret2.ok == "OK" then
                                return 1
                            end
                            return 0`
)

var (
    redisGetScript      = redis.NewScript(redisGetScriptStr)
    redisStoreScript    = redis.NewScript(redisStoreScriptStr)
    redisGetScriptSha   atomic.Value
    redisStoreScriptSha atomic.Value
)

type RedisCache struct {
    singleFlight singleflight.Group
    client       redis.Cmdable
    random       *rand.Rand
}

// NewRedisCache 实例化一个缓存结构
func NewRedisCache(client redis.Cmdable) cache.Cache {
    return &RedisCache{client: client, random: rand.New(rand.NewSource(time.Now().UnixNano()))}
}

// randomSecond100 10-100以内的随机时间秒
func (r *RedisCache) randomSecond100() time.Duration {
    return time.Second * time.Duration(r.random100())
}

// random100 10-100以内的随机数
func (r *RedisCache) random100() int {
    return redisExpireBase + r.random.Intn(90)
}

// store 存储 value, 使用 lua 脚本去处理, 不允许缓存不设置过期时间
// 1. 如果 set px nx 失败, 比对一下缓存里面和当前 value 是否一致, 不一致需要 set ex 设置成无效状态
// 2. 如果发现缓存里面的已经被设置为无效状态了或者 value 是一致的, 那么直接忽略
func (r *RedisCache) store(key string, value interface{}, expiration time.Duration) (bool, error) {
    if len(key) == 0 {
        return false, nil
    }

    sha, ok := redisStoreScriptSha.Load().(string)
    if !ok {
        var err error
        sha, err = redisStoreScript.Load(r.client).Result()
        if err != nil {
            return false, xerrors.Errorf("Get Load error: %w", err)
        }
        redisStoreScriptSha.Store(sha)
    }

    result, err := r.client.EvalSha(sha, []string{key}, value, int64(expiration/time.Millisecond), invalidCacheCode, int64(placeholderTime/time.Millisecond)).Result()
    if err != nil {
        return false, xerrors.Errorf("Store SetNX error: %w", err)
    }

    ret := result.(int64) == 1
    return ret, nil
}

// Get 获取 value, 使用 lua 脚本去处理, 如果缓存不存在, 直接返回, 如果缓存是 invalidCacheCode, 执行 del key.
// 如果在缓存中没有获取到数据, 则执行传入的函数去获取数据, 最后执行 set key value ex nx 存到缓存
func (r *RedisCache) Get(key string, f func() (string, error), opts ...cache.Opts) (string, error) {
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

            opt := cache.Option{
                Timeout:       time.Duration(SevenDayInt) * time.Second,
                RandomTimeout: time.Duration(r.random100()) * time.Second,
            }

            for _, o := range opts {
                o(&opt)
            }

            ret, err := r.store(key, result, opt.Timeout+opt.RandomTimeout)
            if err != nil {
                return nil, err
            }

            if !ret {
                zlog.Warnf("设置缓存失败, key = %v, val = %v\n", key, result)
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
        result, err := r.client.Set(key, invalidCacheCode, placeholderTime).Result()
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
        pipeline.Set(key, invalidCacheCode, placeholderTime)
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

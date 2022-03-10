package sre

import (
	"errors"
	"github.com/Zzaniu/zrpc/tool/window"
	"math"
	"math/rand"
	"sync"
	"sync/atomic"
	"time"
)

const (
	StateOpen int32 = iota
	StateClosed
)

var ErrNotAllowed = errors.New("circuitbreaker: not allowed for circuit open")

// SreBreaker google sre 弹性熔断器
type SreBreaker struct {
	k      float64
	policy window.Rolling

	state    int32  // 开启还是关闭
	request  uint64 // 如果请求总数小于这个数的话, 不开启熔断. 一般是几个
	r        *rand.Rand
	randLock sync.Mutex
}

// Allow 是否允许改请求通过, 如果返回 error 说明被熔断了
func (s *SreBreaker) Allow() error {
	accept, reqTotal := s.policy.Summary()
	requests := s.k * float64(accept)
	if reqTotal < s.request || float64(reqTotal) < requests {
		// 如果 s.state 等于 StateOpen 则更新为 StateClosed
		atomic.CompareAndSwapInt32(&s.state, StateOpen, StateClosed)
		return nil
	}
	atomic.CompareAndSwapInt32(&s.state, StateClosed, StateOpen)
	dr := math.Max(0, (float64(reqTotal)-requests)/float64(reqTotal+1))
	if dr <= 0 {
		return nil
	}
	if s.trueOnProba(dr) {
		return ErrNotAllowed
	}
	return nil
}

// MarkSuccess 标记成功
func (s *SreBreaker) MarkSuccess() {
	s.policy.Add(1)
}

// MarkFailed 标记失败
func (s *SreBreaker) MarkFailed() {
	s.policy.Add(0)
}

func (s *SreBreaker) trueOnProba(proba float64) (truth bool) {
	s.randLock.Lock()
	truth = s.r.Float64() < proba // 这里是伪随机数有什么问题吗？个人感觉是没问题的
	s.randLock.Unlock()
	return
}

func NewSreBreaker() *SreBreaker {
	return &SreBreaker{
		k:       1 / 0.8, // 10个里面允许有2个异常
		request: 5,
		r:       rand.New(rand.NewSource(time.Now().UnixNano())),
		policy:  window.NewRollingPolicy(),
		state:   StateClosed,
	}
}

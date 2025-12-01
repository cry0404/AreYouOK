package cache

import (
	"context"
	"fmt"
	"sync"
	"time"

	"go.uber.org/zap"

	"AreYouOK/pkg/logger"
)

// State 熔断器状态
type State int

const (
	StateClosed State = iota // 关闭状态：正常工作
	StateOpen               // 开启状态：熔断中
	StateHalfOpen           // 半开状态：尝试恢复
)

// CircuitBreaker 缓存熔断器
type CircuitBreaker struct {
	name              string
	maxFailures       int           // 最大失败次数
	resetTimeout      time.Duration // 重置超时时间
	halfOpenMaxCalls  int           // 半开状态最大调用次数

	mu         sync.RWMutex
	state      State
	failures   int
	lastFailTime time.Time
	halfOpenCalls int
}

// NewCircuitBreaker 创建熔断器
func NewCircuitBreaker(name string, maxFailures int, resetTimeout time.Duration) *CircuitBreaker {
	return &CircuitBreaker{
		name:             name,
		maxFailures:      maxFailures,
		resetTimeout:     resetTimeout,
		halfOpenMaxCalls: 3, // 半开状态允许3次尝试
		state:            StateClosed,
	}
}

// Call 执行带熔断保护的操作
func (cb *CircuitBreaker) Call(ctx context.Context, operation func() error) error {
	if !cb.allowRequest() {
		return fmt.Errorf("circuit breaker '%s' is open", cb.name)
	}

	err := operation()
	cb.recordResult(err)
	return err
}

// CallWithResult 执行带熔断保护的操作，并返回结果
func (cb *CircuitBreaker) CallWithResult(ctx context.Context, operation func() (interface{}, error)) (interface{}, error) {
	if !cb.allowRequest() {
		return nil, fmt.Errorf("circuit breaker '%s' is open", cb.name)
	}

	result, err := operation()
	cb.recordResult(err)
	return result, err
}

// allowRequest 检查是否允许请求
func (cb *CircuitBreaker) allowRequest() bool {
	cb.mu.RLock()
	defer cb.mu.RUnlock()

	switch cb.state {
	case StateClosed:
		return true
	case StateOpen:
		// 检查是否可以转为半开状态
		if time.Since(cb.lastFailTime) >= cb.resetTimeout {
			cb.mu.RUnlock()
			cb.mu.Lock()
			if cb.state == StateOpen && time.Since(cb.lastFailTime) >= cb.resetTimeout {
				cb.transitionToHalfOpen()
			}
			cb.mu.Unlock()
			cb.mu.RLock()
		}
		return cb.state == StateHalfOpen
	case StateHalfOpen:
		return cb.halfOpenCalls < cb.halfOpenMaxCalls
	default:
		return false
	}
}

// recordResult 记录操作结果
func (cb *CircuitBreaker) recordResult(err error) {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	if err != nil {
		cb.onFailure()
	} else {
		cb.onSuccess()
	}
}

// onSuccess 处理成功情况
func (cb *CircuitBreaker) onSuccess() {
	switch cb.state {
	case StateClosed:
		cb.failures = 0
	case StateHalfOpen:
		cb.transitionToClosed()
	default:
		// 其他状态不需要处理
	}
}

// onFailure 处理失败情况
func (cb *CircuitBreaker) onFailure() {
	cb.failures++
	cb.lastFailTime = time.Now()

	logger.Logger.Warn("Cache operation failed",
		zap.String("breaker", cb.name),
		zap.Int("failures", cb.failures),
		zap.String("state", cb.stateName()),
	)

	switch cb.state {
	case StateClosed:
		if cb.failures >= cb.maxFailures {
			cb.transitionToOpen()
		}
	case StateHalfOpen:
		cb.transitionToOpen()
	default:
		// 其他状态不需要处理
	}
}

// transitionToClosed 转换到关闭状态
func (cb *CircuitBreaker) transitionToClosed() {
	cb.state = StateClosed
	cb.failures = 0
	cb.halfOpenCalls = 0

	logger.Logger.Info("Circuit breaker transitioned to closed",
		zap.String("breaker", cb.name),
	)
}

// transitionToOpen 转换到开启状态
func (cb *CircuitBreaker) transitionToOpen() {
	cb.state = StateOpen
	cb.halfOpenCalls = 0

	logger.Logger.Warn("Circuit breaker transitioned to open",
		zap.String("breaker", cb.name),
		zap.Int("failures", cb.failures),
		zap.Duration("reset_timeout", cb.resetTimeout),
	)
}

// transitionToHalfOpen 转换到半开状态
func (cb *CircuitBreaker) transitionToHalfOpen() {
	cb.state = StateHalfOpen
	cb.halfOpenCalls = 0

	logger.Logger.Info("Circuit breaker transitioned to half-open",
		zap.String("breaker", cb.name),
	)
}

// stateName 获取状态名称
func (cb *CircuitBreaker) stateName() string {
	switch cb.state {
	case StateClosed:
		return "closed"
	case StateOpen:
		return "open"
	case StateHalfOpen:
		return "half-open"
	default:
		return "unknown"
	}
}

// GetState 获取当前状态
func (cb *CircuitBreaker) GetState() State {
	cb.mu.RLock()
	defer cb.mu.RUnlock()
	return cb.state
}

// GetStats 获取统计信息
func (cb *CircuitBreaker) GetStats() map[string]interface{} {
	cb.mu.RLock()
	defer cb.mu.RUnlock()

	return map[string]interface{}{
		"name":        cb.name,
		"state":       cb.stateName(),
		"failures":    cb.failures,
		"last_fail":   cb.lastFailTime,
		"half_calls":  cb.halfOpenCalls,
	}
}

// 全局熔断器实例
var (
	// Redis缓存熔断器：连续失败5次后熔断，30秒后尝试恢复
	RedisBreaker = NewCircuitBreaker("redis_cache", 5, 30*time.Second)

	// 用户设置缓存熔断器：连续失败3次后熔断，10秒后尝试恢复
	UserSettingsBreaker = NewCircuitBreaker("user_settings_cache", 3, 10*time.Second)

	// 验证码缓存熔断器：连续失败10次后熔断，5秒后尝试恢复
	CaptchaBreaker = NewCircuitBreaker("captcha_cache", 10, 5*time.Second)
)
// Package eventbus 进程内事件总线。
// 限界上下文之间跨模块通信的唯一通道；未来拆分微服务时，
// 将本实现替换为 MQ（NSQ/Kafka）或 gRPC 事件发布即可，业务代码不变。
package eventbus

import "sync"

// Event 领域事件
type Event interface {
	Topic() string
}

// Handler 事件订阅者
type Handler func(e Event)

type bus struct {
	mu       sync.RWMutex
	handlers map[string][]Handler
}

var defaultBus = &bus{handlers: make(map[string][]Handler)}

// Publish 发布事件（同步调用，简单场景够用）
func Publish(e Event) {
	defaultBus.mu.RLock()
	hs := defaultBus.handlers[e.Topic()]
	defaultBus.mu.RUnlock()
	for _, h := range hs {
		h(e)
	}
}

// Subscribe 订阅主题
func Subscribe(topic string, h Handler) {
	defaultBus.mu.Lock()
	defer defaultBus.mu.Unlock()
	defaultBus.handlers[topic] = append(defaultBus.handlers[topic], h)
}

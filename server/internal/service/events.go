package service

import (
	"context"
	"encoding/json"
	"log"
	"sync"
	"time"

	"doyinspark/internal/cache"
)

// Event 推送给控制台的实时事件。
type Event struct {
	UserID  uint      `json:"userId,omitempty"`
	Type    string    `json:"type"`    // send | break | account | system
	Level   string    `json:"level"`   // info | success | warn | danger
	Title   string    `json:"title"`
	Body    string    `json:"body"`
	At      time.Time `json:"at"`
	Payload any       `json:"payload,omitempty"`
}

// EventBus 本地订阅 + Redis Pub/Sub 跨实例广播。
type EventBus struct {
	mu   sync.RWMutex
	subs map[chan Event]struct{}
	rdb  *cache.Cache
}

const eventChannel = "doyinspark:events"

// NewEventBus 创建事件总线并启动跨实例订阅。
func NewEventBus(rdb *cache.Cache) *EventBus {
	bus := &EventBus{subs: make(map[chan Event]struct{}), rdb: rdb}
	if rdb != nil {
		go bus.relay(context.Background())
	}
	return bus
}

// Subscribe 注册一个订阅通道。
func (b *EventBus) Subscribe() chan Event {
	ch := make(chan Event, 32)
	b.mu.Lock()
	b.subs[ch] = struct{}{}
	b.mu.Unlock()
	return ch
}

// Unsubscribe 注销订阅。
func (b *EventBus) Unsubscribe(ch chan Event) {
	b.mu.Lock()
	if _, ok := b.subs[ch]; ok {
		delete(b.subs, ch)
		close(ch)
	}
	b.mu.Unlock()
}

// Publish 广播事件：本地扇出 + Redis 广播给其它实例。
func (b *EventBus) Publish(evt Event) {
	if evt.At.IsZero() {
		evt.At = time.Now()
	}
	b.fanout(evt)
	if b.rdb != nil {
		if raw, err := json.Marshal(evt); err == nil {
			b.rdb.Publish(context.Background(), eventChannel, raw)
		}
	}
}

func (b *EventBus) fanout(evt Event) {
	b.mu.RLock()
	defer b.mu.RUnlock()
	for ch := range b.subs {
		select {
		case ch <- evt:
		default: // 慢消费者直接丢弃，避免阻塞调度器
		}
	}
}

func (b *EventBus) relay(ctx context.Context) {
	pubsub := b.rdb.Subscribe(ctx, eventChannel)
	defer pubsub.Close()
	for msg := range pubsub.Channel() {
		var evt Event
		if json.Unmarshal([]byte(msg.Payload), &evt) == nil {
			b.fanout(evt)
		}
	}
	log.Println("事件总线已订阅 Redis 通道", eventChannel)
}

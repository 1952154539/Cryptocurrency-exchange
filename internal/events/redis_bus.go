package events

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"

	"github.com/go-redis/redis/v8"
	"github.com/rs/zerolog/log"
)

// RedisEventBus implements EventBus using Redis pub/sub for inter-process communication.
type RedisEventBus struct {
	client *redis.Client
	mu     sync.Mutex
	pubSubs []*redis.PubSub
	closed  bool
}

// NewRedisEventBus creates a Redis-backed event bus.
func NewRedisEventBus(client *redis.Client) *RedisEventBus {
	return &RedisEventBus{client: client}
}

// Publish sends events to a Redis pub/sub channel.
func (b *RedisEventBus) Publish(ctx context.Context, topic string, events ...*Event) error {
	for _, evt := range events {
		data, err := json.Marshal(evt)
		if err != nil {
			return fmt.Errorf("marshal event: %w", err)
		}
		if err := b.client.Publish(ctx, "events:"+topic, data).Err(); err != nil {
			return fmt.Errorf("redis publish: %w", err)
		}
	}
	return nil
}

// Subscribe listens for events on a Redis pub/sub channel.
// Each call creates a new subscription goroutine.
func (b *RedisEventBus) Subscribe(ctx context.Context, topic string, group string, handler EventHandler) error {
	ps := b.client.Subscribe(ctx, "events:"+topic)

	b.mu.Lock()
	b.pubSubs = append(b.pubSubs, ps)
	b.mu.Unlock()

	ch := ps.Channel()

	go func() {
		defer func() {
			b.mu.Lock()
			for i, s := range b.pubSubs {
				if s == ps {
					b.pubSubs = append(b.pubSubs[:i], b.pubSubs[i+1:]...)
					break
				}
			}
			b.mu.Unlock()
		}()

		for {
			select {
			case msg, ok := <-ch:
				if !ok {
					return
				}
				var evt Event
				if err := json.Unmarshal([]byte(msg.Payload), &evt); err != nil {
					log.Warn().Err(err).Str("topic", topic).Msg("failed to unmarshal event")
					continue
				}
				if err := handler(ctx, &evt); err != nil {
					log.Warn().Err(err).Str("topic", topic).Str("event_id", evt.ID).Msg("event handler error")
				}
			case <-ctx.Done():
				return
			}
		}
	}()

	return nil
}

// Close shuts down all Redis subscriptions.
func (b *RedisEventBus) Close() error {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.closed = true
	for _, ps := range b.pubSubs {
		ps.Close()
	}
	b.pubSubs = nil
	return nil
}

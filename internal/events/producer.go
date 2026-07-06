package events

import (
	"context"
	"encoding/json"
	"sync"

	"github.com/rs/zerolog/log"
)

// MemoryEventBus is an in-memory event bus for development and testing.
// In production, replace with Kafka-backed implementation.
type MemoryEventBus struct {
	handlers map[string][]handlerEntry
	mu       sync.RWMutex
}

type handlerEntry struct {
	handler EventHandler
	group   string
}

// NewMemoryEventBus creates a new in-memory event bus.
func NewMemoryEventBus() *MemoryEventBus {
	return &MemoryEventBus{
		handlers: make(map[string][]handlerEntry),
	}
}

// Publish sends events to all subscribers of the given topic.
func (b *MemoryEventBus) Publish(ctx context.Context, topic string, events ...*Event) error {
	b.mu.RLock()
	handlers := b.handlers[topic]
	b.mu.RUnlock()

	for _, evt := range events {
		// Log the event
		if payload, err := json.Marshal(evt.Payload); err == nil {
			log.Debug().
				Str("event_id", evt.ID).
				Str("topic", topic).
				Str("type", string(evt.Type)).
				RawJSON("payload", payload).
				Msg("event published")
		}

		for _, entry := range handlers {
			if err := entry.handler(ctx, evt); err != nil {
				log.Error().
					Err(err).
					Str("event_id", evt.ID).
					Str("topic", topic).
					Msg("event handler failed")
			}
		}
	}
	return nil
}

// Subscribe registers a handler for a topic with an optional consumer group.
func (b *MemoryEventBus) Subscribe(ctx context.Context, topic string, group string, handler EventHandler) error {
	b.mu.Lock()
	defer b.mu.Unlock()

	b.handlers[topic] = append(b.handlers[topic], handlerEntry{
		handler: handler,
		group:   group,
	})

	log.Info().
		Str("topic", topic).
		Str("group", group).
		Msg("event handler subscribed")
	return nil
}

// Close shuts down the event bus.
func (b *MemoryEventBus) Close() error {
	b.mu.Lock()
	b.handlers = make(map[string][]handlerEntry)
	b.mu.Unlock()
	return nil
}

package events

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/rs/zerolog/log"
	"github.com/segmentio/kafka-go"
)

// KafkaConfig holds Kafka connection parameters.
type KafkaConfig struct {
	Brokers []string
	GroupID string
}

// KafkaEventBus implements EventBus using Kafka for reliable inter-process messaging.
type KafkaEventBus struct {
	writer   *kafka.Writer
	reader   *kafka.Reader
	config   KafkaConfig
	consumers []*consumer
	mu        sync.Mutex
	closed    bool
}

type consumer struct {
	topic    string
	reader   *kafka.Reader
	cancel   context.CancelFunc
	done     chan struct{}
}

// NewKafkaEventBus creates a Kafka-backed event bus.
func NewKafkaEventBus(cfg KafkaConfig) *KafkaEventBus {
	return &KafkaEventBus{
		writer: &kafka.Writer{
			Addr:         kafka.TCP(cfg.Brokers...),
			Balancer:     &kafka.LeastBytes{},
			BatchTimeout: 10 * time.Millisecond,
			RequiredAcks: kafka.RequireOne,
		},
		config: cfg,
	}
}

// Publish sends events to a Kafka topic.
func (b *KafkaEventBus) Publish(ctx context.Context, topic string, events ...*Event) error {
	msgs := make([]kafka.Message, len(events))
	for i, evt := range events {
		data, err := json.Marshal(evt)
		if err != nil {
			return fmt.Errorf("marshal event: %w", err)
		}
		msgs[i] = kafka.Message{
			Topic: "events." + topic,
			Key:   []byte(evt.ID),
			Value: data,
			Time:  time.Now(),
		}
	}
	return b.writer.WriteMessages(ctx, msgs...)
}

// Subscribe listens for events on a Kafka topic using a consumer group.
func (b *KafkaEventBus) Subscribe(ctx context.Context, topic string, group string, handler EventHandler) error {
	groupID := b.config.GroupID
	if group != "" {
		groupID = b.config.GroupID + "-" + group
	}

	r := kafka.NewReader(kafka.ReaderConfig{
		Brokers:        b.config.Brokers,
		Topic:          "events." + topic,
		GroupID:        groupID,
		MinBytes:        1,
		MaxBytes:        10e6, // 10MB
		MaxWait:         1 * time.Second,
		StartOffset:     kafka.LastOffset,
		CommitInterval:  0, // manual commit
	})

	ctx, cancel := context.WithCancel(ctx)
	c := &consumer{
		topic:  topic,
		reader: r,
		cancel: cancel,
		done:   make(chan struct{}),
	}

	b.mu.Lock()
	b.consumers = append(b.consumers, c)
	b.mu.Unlock()

	go func() {
		defer close(c.done)
		defer r.Close()

		for {
			msg, err := r.FetchMessage(ctx)
			if err != nil {
				if ctx.Err() != nil {
					return
				}
				log.Warn().Err(err).Str("topic", topic).Msg("kafka fetch error")
				continue
			}

			var evt Event
			if err := json.Unmarshal(msg.Value, &evt); err != nil {
				log.Warn().Err(err).Str("topic", topic).Msg("failed to unmarshal event")
				r.CommitMessages(ctx, msg)
				continue
			}

			if err := handler(ctx, &evt); err != nil {
				log.Warn().Err(err).Str("topic", topic).Str("event_id", evt.ID).Msg("event handler error")
			}

			if err := r.CommitMessages(ctx, msg); err != nil {
				log.Warn().Err(err).Str("topic", topic).Msg("kafka commit error")
			}
		}
	}()

	log.Info().Str("topic", topic).Str("group", groupID).Msg("kafka consumer started")
	return nil
}

// Close shuts down all consumers and the producer.
func (b *KafkaEventBus) Close() error {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.closed = true

	// Stop all consumers
	for _, c := range b.consumers {
		c.cancel()
		<-c.done
	}
	b.consumers = nil

	// Close writer
	return b.writer.Close()
}

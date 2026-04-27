package events

import (
	"fmt"
	"sync"
	"time"
)

type Event struct {
	ID        string         `json:"id"`
	Type      string         `json:"type"`
	Timestamp string         `json:"timestamp"`
	Summary   string         `json:"summary"`
	Payload   map[string]any `json:"payload,omitempty"`
}

type Broker struct {
	mu          sync.RWMutex
	subscribers map[chan Event]struct{}
	nextID      uint64
}

func NewBroker() *Broker {
	return &Broker{
		subscribers: make(map[chan Event]struct{}),
	}
}

func (b *Broker) Publish(typ string, summary string, payload map[string]any) {
	if payload == nil {
		payload = map[string]any{}
	}

	event := Event{
		ID:        newEventID(&b.nextID),
		Type:      typ,
		Timestamp: time.Now().UTC().Format(time.RFC3339),
		Summary:   summary,
		Payload:   payload,
	}

	b.mu.RLock()
	defer b.mu.RUnlock()

	for ch := range b.subscribers {
		select {
		case ch <- event:
		default:
		}
	}
}

func (b *Broker) Subscribe() (<-chan Event, func()) {
	ch := make(chan Event, 32)

	b.mu.Lock()
	b.subscribers[ch] = struct{}{}
	b.mu.Unlock()

	cancel := func() {
		b.mu.Lock()
		if _, ok := b.subscribers[ch]; ok {
			delete(b.subscribers, ch)
			close(ch)
		}
		b.mu.Unlock()
	}

	return ch, cancel
}

func newEventID(next *uint64) string {
	*next = *next + 1
	return fmt.Sprintf("evt-%06d", *next)
}

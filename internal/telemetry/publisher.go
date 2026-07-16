package telemetry

import (
	"strconv"
	"sync"
	"time"
)

// LiveEvent is one non-persistent event for the local dashboard. Persistent
// JSONL remains authoritative and is never coupled to this publisher.
type LiveEvent struct {
	Sequence  uint64         `json:"sequence"`
	Timestamp time.Time      `json:"timestamp"`
	Event     string         `json:"event"`
	SessionID string         `json:"session_id,omitempty"`
	GameID    string         `json:"game_id,omitempty"`
	RunID     string         `json:"run_id,omitempty"`
	Run       string         `json:"run,omitempty"`
	Step      string         `json:"step,omitempty"`
	AreaID    uint32         `json:"area_id,omitempty"`
	Area      string         `json:"area,omitempty"`
	Reason    string         `json:"reason,omitempty"`
	Details   map[string]any `json:"details,omitempty"`
}

// Subscription owns a bounded event channel. Slow consumers are disconnected
// instead of blocking the bot, JSONL or other subscribers.
type Subscription struct {
	Events <-chan LiveEvent
	close  func()
}

// Close removes the subscription. It is idempotent.
func (s *Subscription) Close() {
	if s != nil && s.close != nil {
		s.close()
		s.close = nil
	}
}

type liveSubscriber struct {
	channel chan LiveEvent
}

// LivePublisher is a non-blocking monotonic event ring for transient UI use.
type LivePublisher struct {
	mu          sync.Mutex
	capacity    int
	clientQueue int
	sequence    uint64
	ring        []LiveEvent
	subscribers map[uint64]*liveSubscriber
	nextClient  uint64
	closed      bool
	lastDedupe  map[string]string
}

// NewLivePublisher creates a bounded ring and per-client queue.
func NewLivePublisher(capacity, clientQueue int) *LivePublisher {
	if capacity <= 0 {
		capacity = 256
	}
	if clientQueue <= 0 {
		clientQueue = 64
	}
	return &LivePublisher{capacity: capacity, clientQueue: clientQueue, subscribers: make(map[uint64]*liveSubscriber), lastDedupe: make(map[string]string)}
}

// Sequence returns the most recently assigned sequence.
func (p *LivePublisher) Sequence() uint64 {
	if p == nil {
		return 0
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.sequence
}

// Publish appends and broadcasts an event without waiting for any client.
func (p *LivePublisher) Publish(event LiveEvent) uint64 {
	if p == nil || event.Event == "" {
		return 0
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.closed {
		return p.sequence
	}
	if key, value := dedupeIdentity(event); key != "" {
		if p.lastDedupe[key] == value {
			return p.sequence
		}
		p.lastDedupe[key] = value
	}
	p.sequence++
	event.Sequence = p.sequence
	if event.Timestamp.IsZero() {
		event.Timestamp = time.Now().UTC()
	} else {
		event.Timestamp = event.Timestamp.UTC()
	}
	event.Details = cloneDetails(event.Details)
	p.ring = append(p.ring, event)
	if len(p.ring) > p.capacity {
		p.ring = append([]LiveEvent(nil), p.ring[len(p.ring)-p.capacity:]...)
	}
	for id, subscriber := range p.subscribers {
		select {
		case subscriber.channel <- event:
		default:
			close(subscriber.channel)
			delete(p.subscribers, id)
		}
	}
	return event.Sequence
}

// Subscribe returns retained events newer than after and a bounded live stream.
func (p *LivePublisher) Subscribe(after uint64) ([]LiveEvent, *Subscription) {
	if p == nil {
		closed := make(chan LiveEvent)
		close(closed)
		return nil, &Subscription{Events: closed}
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	replay := make([]LiveEvent, 0, len(p.ring))
	for _, event := range p.ring {
		if event.Sequence > after {
			event.Details = cloneDetails(event.Details)
			replay = append(replay, event)
		}
	}
	channel := make(chan LiveEvent, p.clientQueue)
	if p.closed {
		close(channel)
		return replay, &Subscription{Events: channel}
	}
	p.nextClient++
	id := p.nextClient
	p.subscribers[id] = &liveSubscriber{channel: channel}
	var once sync.Once
	return replay, &Subscription{Events: channel, close: func() {
		once.Do(func() {
			p.mu.Lock()
			defer p.mu.Unlock()
			if subscriber, ok := p.subscribers[id]; ok {
				close(subscriber.channel)
				delete(p.subscribers, id)
			}
		})
	}}
}

// Close disconnects all clients and rejects later events.
func (p *LivePublisher) Close() {
	if p == nil {
		return
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.closed {
		return
	}
	p.closed = true
	for id, subscriber := range p.subscribers {
		close(subscriber.channel)
		delete(p.subscribers, id)
	}
}

func dedupeIdentity(event LiveEvent) (string, string) {
	switch event.Event {
	case "area_changed":
		return event.Event, event.Area + ":" + strconv.FormatUint(uint64(event.AreaID), 10)
	case "step_changed":
		return event.Event, event.RunID + ":" + event.Step
	default:
		return "", ""
	}
}

func cloneDetails(details map[string]any) map[string]any {
	if details == nil {
		return nil
	}
	copyOfDetails := make(map[string]any, len(details))
	for key, value := range details {
		copyOfDetails[key] = value
	}
	return copyOfDetails
}

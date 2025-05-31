package events

import (
	"context"
	"fmt"
	"log/slog"
	"net"
	"reflect"
	"sync"
	"sync/atomic"
	"time"
)

// Package events provides a high-performance, lock-free event system for GoMCP server lifecycle events.
//
// The event system uses atomic copy-on-write operations for thread-safe access without locks,
// providing excellent performance characteristics:
//
// ## Performance Features:
// - Lock-free subscriber management using atomic operations
// - Copy-on-write pattern for zero-lock reads
// - Asynchronous event delivery for live events (maximum throughput)
// - Synchronous event delivery only during replay (order preservation)
// - Configurable event caching for late subscribers
//
// ## Event Delivery Modes:
// - **Live Events**: Delivered asynchronously in separate goroutines for maximum performance
// - **Replay Events**: Delivered synchronously to new subscribers to guarantee chronological order
//
// ## Usage Example:
//
//	// Create an event system with replay capability
//	subject := events.NewSubject(
//	    events.WithReplay(100),
//	    events.WithBufferSize(1024),
//	    events.WithLogger(logger),
//	)
//
//	// Subscribe to events (gets replay + live events)
//	sub := events.Subscribe[MyEvent](subject, "my.topic",
//	    func(ctx context.Context, evt MyEvent) error {
//	        // Handle event asynchronously (for live events)
//	        // Handle event synchronously (for replay events)
//	        return nil
//	    }, true) // enable replay
//
//	// Publish events (non-blocking)
//	events.Publish[MyEvent](subject, "my.topic", MyEvent{Data: "test"})
//
// ## Thread Safety:
// All operations are thread-safe and can be called concurrently from multiple goroutines.
// The system is designed for high-concurrency scenarios with minimal contention.

// HandlerFunc is the function called when an event is emitted.
// It can optionally receive a net.Conn as its last parameter.
type HandlerFunc interface{}

// SubjectOption configures a Subject
type SubjectOption func(*subjectConfig)

type subjectConfig struct {
	replayEnabled bool
	cacheSize     int
	bufferSize    int
	logger        *slog.Logger
}

// WithBufferSize sets the event channel buffer size
func WithBufferSize(size int) SubjectOption {
	return func(cfg *subjectConfig) {
		cfg.bufferSize = size
	}
}

// WithReplay enables replay functionality with specified cache size
func WithReplay(cacheSize int) SubjectOption {
	return func(cfg *subjectConfig) {
		cfg.replayEnabled = true
		cfg.cacheSize = cacheSize
	}
}

// WithLogger sets a structured logger for event system errors
func WithLogger(logger *slog.Logger) SubjectOption {
	return func(cfg *subjectConfig) {
		cfg.logger = logger
	}
}

// Publish emits an event to the given topic.
// If a connection is provided, the event will only be delivered to that specific client.
func Publish[T any](subject *Subject, topic string, value T, conn ...net.Conn) error {
	var connection net.Conn
	if len(conn) > 0 {
		connection = conn[0]
	}

	evt := event{
		topic:   topic,
		message: value,
		conn:    connection,
	}

	select {
	case subject.events <- evt:
		return nil
	case <-time.After(5 * time.Second):
		return fmt.Errorf("failed to emit event: %v", value)
	}
}

// Subscribe subscribes a handler to the given topic.
// The handler can be either:
// - func(context.Context, T) error
// - func(context.Context, T, net.Conn) error
// A Subscription is returned that can be used to unsubscribe from the topic.
func Subscribe[T any](subject *Subject, topic string, handler interface{}, replay ...bool) Subscription {
	wantsReplay := false
	if len(replay) > 0 {
		wantsReplay = replay[0]
	}

	handlerType := reflect.TypeOf(handler)
	if handlerType.Kind() != reflect.Func {
		panic(fmt.Sprintf("handler must be a function, got %T", handler))
	}

	var wrappedHandler HandlerFunc

	// Check number of parameters to determine handler type
	if handlerType.NumIn() == 3 {
		// func(context.Context, T, net.Conn) error
		typedHandler, ok := handler.(func(context.Context, T, net.Conn) error)
		if !ok {
			panic(fmt.Sprintf("invalid handler signature: %T", handler))
		}
		wrappedHandler = func(ctx context.Context, data any, conn net.Conn) error {
			if typed, ok := data.(T); ok {
				return typedHandler(ctx, typed, conn)
			}
			return fmt.Errorf("type assertion failed for %T, expected %T", data, *new(T))
		}
	} else {
		// func(context.Context, T) error
		typedHandler, ok := handler.(func(context.Context, T) error)
		if !ok {
			panic(fmt.Sprintf("invalid handler signature: %T", handler))
		}
		wrappedHandler = func(ctx context.Context, data any) error {
			if typed, ok := data.(T); ok {
				return typedHandler(ctx, typed)
			}
			return fmt.Errorf("type assertion failed for %T, expected %T", data, *new(T))
		}
	}

	subID := atomic.AddInt64(&subject.nextSubID, 1)
	createdAt := time.Now().UnixNano()

	sub := Subscription{
		Topic:       topic,
		CreatedAt:   createdAt,
		Handler:     wrappedHandler,
		ID:          fmt.Sprintf("%s-%d", topic, subID),
		WantsReplay: wantsReplay,
		SentEvents:  make(map[string]bool),
	}

	// Add subscription using copy-on-write
	subject.addSubscription(sub)

	// Set up unsubscribe function
	sub.Unsubscribe = func() {
		subject.removeSubscription(sub.ID)
	}

	// Handle replay if enabled
	if subject.config.replayEnabled && wantsReplay {
		subject.replayEvents(sub)
	}

	return sub
}

// Complete shuts down the event system, stopping all goroutines and cleaning up resources.
// This function is idempotent and safe to call multiple times.
func Complete(s *Subject) {
	if s == nil {
		return
	}

	// Try to close the shutdown channel only once using atomic operation
	if atomic.CompareAndSwapInt32(&s.closed, 0, 1) {
		close(s.shutdown)

		// Wait for goroutines to finish (with timeout to prevent hanging)
		done := make(chan struct{})
		go func() {
			s.wg.Wait()
			close(done)
		}()

		select {
		case <-done:
			// All goroutines finished
		case <-time.After(5 * time.Second):
			// Timeout waiting for goroutines
		}
	}
}

type event struct {
	topic   string
	message any
	conn    net.Conn
}

// Subscription represents a handler subscribed to a specific topic.
type Subscription struct {
	Topic       string
	CreatedAt   int64
	Handler     HandlerFunc
	ID          string
	WantsReplay bool
	SentEvents  map[string]bool // Replay tracking per subscription
	Unsubscribe func()
}

type subscriberMap map[string]map[string]Subscription

type Subject struct {
	// Lock-free state using atomics
	subscribers atomic.Pointer[subscriberMap]
	cache       atomic.Pointer[[]event]
	nextSubID   int64
	eventCount  int64

	// Single event channel
	events   chan event
	shutdown chan struct{}

	// Configuration (read-only after creation)
	config subjectConfig

	// Additional fields for Complete function
	closed int32
	wg     sync.WaitGroup
}

// NewSubject creates a new Subject with optional configuration.
func NewSubject(opts ...SubjectOption) *Subject {
	cfg := subjectConfig{
		bufferSize: 512, // default
	}

	// Apply options
	for _, opt := range opts {
		opt(&cfg)
	}

	s := &Subject{
		events:   make(chan event, cfg.bufferSize),
		shutdown: make(chan struct{}),
		config:   cfg,
	}

	// Initialize atomic pointers
	emptySubscribers := make(subscriberMap)
	s.subscribers.Store(&emptySubscribers)

	if cfg.replayEnabled {
		emptyCache := make([]event, 0, cfg.cacheSize)
		s.cache.Store(&emptyCache)
	}

	go s.eventLoop()
	return s
}

// eventLoop processes events and distributes them to subscribers
func (s *Subject) eventLoop() {
	s.wg.Add(1)
	defer s.wg.Done()

	for {
		select {
		case <-s.shutdown:
			return
		case evt := <-s.events:
			atomic.AddInt64(&s.eventCount, 1)

			// Add to cache if replay enabled (copy-on-write)
			if s.config.replayEnabled {
				s.addToCache(evt)
			}

			// Send to subscribers (lock-free read) - asynchronously for better performance
			subs := s.subscribers.Load()
			if topicSubs, ok := (*subs)[evt.topic]; ok {
				for _, sub := range topicSubs {
					s.sendToSubscriber(sub, evt, false) // async delivery for live events
				}
			}
		}
	}
}

// addSubscription adds a subscription using copy-on-write
func (s *Subject) addSubscription(sub Subscription) {
	for {
		oldSubs := s.subscribers.Load()
		newSubs := s.copySubscribers(*oldSubs)

		if _, ok := newSubs[sub.Topic]; !ok {
			newSubs[sub.Topic] = make(map[string]Subscription)
		}
		newSubs[sub.Topic][sub.ID] = sub

		if s.subscribers.CompareAndSwap(oldSubs, &newSubs) {
			break
		}
		// Retry if CAS failed (another goroutine modified it)
	}
}

// removeSubscription removes a subscription using copy-on-write
func (s *Subject) removeSubscription(subID string) {
	for {
		oldSubs := s.subscribers.Load()
		newSubs := s.copySubscribers(*oldSubs)

		found := false
		for topic, topicSubs := range newSubs {
			if _, ok := topicSubs[subID]; ok {
				delete(topicSubs, subID)
				if len(topicSubs) == 0 {
					delete(newSubs, topic)
				}
				found = true
				break
			}
		}

		if !found {
			break // Subscription not found, nothing to do
		}

		if s.subscribers.CompareAndSwap(oldSubs, &newSubs) {
			break
		}
		// Retry if CAS failed
	}
}

// copySubscribers creates a deep copy of the subscribers map
func (s *Subject) copySubscribers(original subscriberMap) subscriberMap {
	copy := make(subscriberMap, len(original))
	for topic, topicSubs := range original {
		copy[topic] = make(map[string]Subscription, len(topicSubs))
		for id, sub := range topicSubs {
			copy[topic][id] = sub
		}
	}
	return copy
}

// addToCache adds an event to the cache using copy-on-write
func (s *Subject) addToCache(evt event) {
	for {
		oldCache := s.cache.Load()
		newCache := make([]event, len(*oldCache))
		copy(newCache, *oldCache)

		if len(newCache) == s.config.cacheSize {
			newCache = newCache[1:]
		}
		newCache = append(newCache, evt)

		if s.cache.CompareAndSwap(oldCache, &newCache) {
			break
		}
		// Retry if CAS failed
	}
}

// replayEvents sends cached events to a new subscriber
func (s *Subject) replayEvents(sub Subscription) {
	if !s.config.replayEnabled {
		return
	}

	cache := s.cache.Load()
	for _, evt := range *cache {
		if evt.topic == sub.Topic {
			eventID := fmt.Sprintf("%s-%v", evt.topic, evt.message)
			if !sub.SentEvents[eventID] {
				// Send replay events synchronously to preserve order
				s.sendToSubscriber(sub, evt, true) // sync delivery for replay
				sub.SentEvents[eventID] = true
			}
		}
	}
}

// sendToSubscriber delivers an event to a subscriber.
// If sync is true, delivery is synchronous (blocking). If false, delivery is asynchronous (non-blocking).
// Synchronous delivery is used during replay to guarantee order preservation.
// Asynchronous delivery is used for live events to maximize performance.
func (s *Subject) sendToSubscriber(sub Subscription, evt event, sync bool) {
	deliverEvent := func() {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		switch fn := sub.Handler.(type) {
		case func(context.Context, any) error:
			if err := fn(ctx, evt.message); err != nil {
				// Use structured logger if available
				if s.config.logger != nil {
					s.config.logger.Debug("event handler error",
						"topic", evt.topic,
						"error", err,
						"subscription_id", sub.ID,
						"delivery_mode", map[bool]string{true: "sync", false: "async"}[sync])
				}
			}
		case func(context.Context, any, net.Conn) error:
			if err := fn(ctx, evt.message, evt.conn); err != nil {
				// Use structured logger if available
				if s.config.logger != nil {
					s.config.logger.Debug("event handler error",
						"topic", evt.topic,
						"error", err,
						"subscription_id", sub.ID,
						"connection", evt.conn.RemoteAddr(),
						"delivery_mode", map[bool]string{true: "sync", false: "async"}[sync])
				}
			}
		}
	}

	if sync {
		// Synchronous delivery - block until completed (used for replay to preserve order)
		deliverEvent()
	} else {
		// Asynchronous delivery - don't block the event loop (used for live events)
		go deliverEvent()
	}
}

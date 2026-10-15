// Package pubsub implements an in-process, generic publish/subscribe broker
// with topics. Delivery is non-blocking: a slow subscriber whose buffer is
// full misses messages instead of holding up publishers.
package pubsub

import "sync"

// Broker routes messages from publishers to the subscribers of a topic. It is
// safe for concurrent use.
type Broker[T any] struct {
	mu     sync.RWMutex
	topics map[string]map[chan T]struct{}
	closed bool
}

// New creates an empty broker.
func New[T any]() *Broker[T] {
	return &Broker[T]{topics: make(map[string]map[chan T]struct{})}
}

// Subscribe registers a subscriber for topic and returns its channel together
// with a cancel function that unsubscribes and closes the channel. buffer is
// the number of messages that can be queued (values < 1 become 1). On a closed
// broker the returned channel is already closed.
func (b *Broker[T]) Subscribe(topic string, buffer int) (<-chan T, func()) {
	ch := make(chan T, max(buffer, 1))

	b.mu.Lock()
	defer b.mu.Unlock()
	if b.closed {
		close(ch)
		return ch, func() {}
	}
	if b.topics[topic] == nil {
		b.topics[topic] = make(map[chan T]struct{})
	}
	b.topics[topic][ch] = struct{}{}

	cancel := func() {
		b.mu.Lock()
		defer b.mu.Unlock()
		if subs, ok := b.topics[topic]; ok {
			if _, ok := subs[ch]; ok {
				delete(subs, ch)
				close(ch)
			}
			if len(subs) == 0 {
				delete(b.topics, topic)
			}
		}
	}
	return ch, cancel
}

// Publish sends msg to every subscriber of topic without blocking and returns
// how many received it. Subscribers with a full buffer are skipped.
func (b *Broker[T]) Publish(topic string, msg T) (delivered int) {
	b.mu.RLock()
	defer b.mu.RUnlock()
	for ch := range b.topics[topic] {
		select {
		case ch <- msg:
			delivered++
		default:
		}
	}
	return delivered
}

// Subscribers returns the number of active subscribers of topic.
func (b *Broker[T]) Subscribers(topic string) int {
	b.mu.RLock()
	defer b.mu.RUnlock()
	return len(b.topics[topic])
}

// Close closes every subscriber channel and rejects new subscriptions. It is
// safe to call more than once.
func (b *Broker[T]) Close() {
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.closed {
		return
	}
	b.closed = true
	for _, subs := range b.topics {
		for ch := range subs {
			close(ch)
		}
	}
	b.topics = make(map[string]map[chan T]struct{})
}

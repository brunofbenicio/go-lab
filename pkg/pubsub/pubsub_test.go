package pubsub

import (
	"sync"
	"testing"
)

func TestPublishFanOutAndTopicIsolation(t *testing.T) {
	b := New[string]()
	a1, cancelA1 := b.Subscribe("news", 4)
	a2, cancelA2 := b.Subscribe("news", 4)
	other, cancelOther := b.Subscribe("sports", 4)
	defer cancelA1()
	defer cancelA2()
	defer cancelOther()

	tests := []struct {
		topic         string
		wantDelivered int
	}{
		{"news", 2},
		{"sports", 1},
		{"unknown", 0},
	}
	for _, tt := range tests {
		if got := b.Publish(tt.topic, "msg-"+tt.topic); got != tt.wantDelivered {
			t.Errorf("Publish(%q) delivered to %d, want %d", tt.topic, got, tt.wantDelivered)
		}
	}
	if m := <-a1; m != "msg-news" {
		t.Errorf("a1 got %q", m)
	}
	if m := <-a2; m != "msg-news" {
		t.Errorf("a2 got %q", m)
	}
	if m := <-other; m != "msg-sports" {
		t.Errorf("sports subscriber got %q", m)
	}
	select {
	case m := <-other:
		t.Errorf("sports subscriber received an unrelated message %q", m)
	default:
	}
}

func TestSlowSubscriberDropsMessages(t *testing.T) {
	b := New[int]()
	ch, cancel := b.Subscribe("t", 2)
	defer cancel()

	got := []int{b.Publish("t", 1), b.Publish("t", 2), b.Publish("t", 3)}
	if want := []int{1, 1, 0}; got[0] != want[0] || got[1] != want[1] || got[2] != want[2] {
		t.Errorf("delivered counts = %v, want %v", got, want)
	}
	if a, c := <-ch, <-ch; a != 1 || c != 2 {
		t.Errorf("received %d, %d; want 1, 2", a, c)
	}
}

func TestCancelClosesChannelAndStopsDelivery(t *testing.T) {
	b := New[int]()
	ch, cancel := b.Subscribe("t", 1)
	if b.Subscribers("t") != 1 {
		t.Fatalf("Subscribers = %d, want 1", b.Subscribers("t"))
	}
	cancel()
	cancel() // idempotent
	if _, ok := <-ch; ok {
		t.Error("channel should be closed after cancel")
	}
	if n := b.Publish("t", 1); n != 0 || b.Subscribers("t") != 0 {
		t.Errorf("after cancel: delivered = %d, subscribers = %d; want 0, 0", n, b.Subscribers("t"))
	}
}

func TestClose(t *testing.T) {
	b := New[int]()
	ch, cancel := b.Subscribe("t", 1)
	b.Close()
	b.Close() // idempotent
	if _, ok := <-ch; ok {
		t.Error("Close should close subscriber channels")
	}
	cancel() // must not panic after Close

	late, _ := b.Subscribe("t", 1)
	if _, ok := <-late; ok {
		t.Error("subscribing to a closed broker should yield a closed channel")
	}
	if n := b.Publish("t", 1); n != 0 {
		t.Errorf("Publish after Close delivered to %d", n)
	}
}

func TestConcurrentPublishSubscribeCancel(t *testing.T) {
	b := New[int]()
	var wg sync.WaitGroup
	for g := 0; g < 4; g++ {
		wg.Add(2)
		go func() {
			defer wg.Done()
			for i := 0; i < 200; i++ {
				b.Publish("t", i)
			}
		}()
		go func() {
			defer wg.Done()
			for i := 0; i < 50; i++ {
				_, cancel := b.Subscribe("t", 2)
				cancel()
			}
		}()
	}
	wg.Wait()
	if n := b.Subscribers("t"); n != 0 {
		t.Errorf("Subscribers = %d, want 0", n)
	}
}

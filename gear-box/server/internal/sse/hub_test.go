package sse

import (
	"encoding/json"
	"testing"
	"time"
)

func TestBroadcastDeliversToSubscriber(t *testing.T) {
	h := NewHub()
	ch, unsubscribe := h.Subscribe()
	defer unsubscribe()

	want := HeardEvent{Transcript: "hello", Waveform: []int16{1, 2, 3}}
	h.Broadcast(want)

	select {
	case data := <-ch:
		var got HeardEvent
		if err := json.Unmarshal(data, &got); err != nil {
			t.Fatalf("unmarshal: %v", err)
		}
		if got.Transcript != want.Transcript {
			t.Fatalf("expected transcript %q, got %q", want.Transcript, got.Transcript)
		}
	case <-time.After(time.Second):
		t.Fatal("expected a message on the subscribed channel")
	}
}

func TestUnsubscribeStopsDelivery(t *testing.T) {
	h := NewHub()
	ch, unsubscribe := h.Subscribe()
	unsubscribe()

	h.Broadcast(HeardEvent{Transcript: "should not arrive"})

	select {
	case <-ch:
		t.Fatal("did not expect a message after unsubscribing")
	case <-time.After(50 * time.Millisecond):
	}
}

func TestBroadcastWithNoSubscribersDoesNotBlock(t *testing.T) {
	h := NewHub()
	done := make(chan struct{})
	go func() {
		h.Broadcast(HeardEvent{Transcript: "nobody listening"})
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("Broadcast blocked with no subscribers")
	}
}

func TestBroadcastDoesNotBlockOnFullSubscriberChannel(t *testing.T) {
	h := NewHub()
	ch, unsubscribe := h.Subscribe()
	defer unsubscribe()

	// Fill the subscriber's buffer (capacity 1) without draining it.
	h.Broadcast(HeardEvent{Transcript: "first"})

	done := make(chan struct{})
	go func() {
		h.Broadcast(HeardEvent{Transcript: "second"})
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("Broadcast blocked on a full subscriber channel")
	}

	<-ch // drain so the goroutine's channel delivery (if any) doesn't leak
}

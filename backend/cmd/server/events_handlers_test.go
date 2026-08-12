package main

import "testing"

func TestDetachEventSubscriberRemovesWithoutClosingChannel(t *testing.T) {
	a := newApp()
	subscriberID, events := a.subscribeEvents()

	a.detachEventSubscriber(subscriberID)

	a.mu.Lock()
	_, stillRegistered := a.eventSubscribers[subscriberID]
	a.mu.Unlock()
	if stillRegistered {
		t.Fatal("detached subscriber remains registered")
	}

	select {
	case _, ok := <-events:
		if !ok {
			t.Fatal("detaching an SSE subscriber closed its channel")
		}
		t.Fatal("detached subscriber unexpectedly received an event")
	default:
	}

	a.publishEvent("test", map[string]any{"ok": true})
	select {
	case _, ok := <-events:
		if !ok {
			t.Fatal("detached subscriber channel was closed after publishing")
		}
		t.Fatal("detached subscriber received an event after removal")
	default:
	}
}

func TestDetachEventSubscriberCanRaceWithPublish(t *testing.T) {
	a := newApp()

	for i := 0; i < 1000; i++ {
		subscriberID, _ := a.subscribeEvents()
		published := make(chan struct{})
		go func(value int) {
			a.publishEvent("test", value)
			close(published)
		}(i)

		a.detachEventSubscriber(subscriberID)
		<-published
	}
}

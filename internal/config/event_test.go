package config

import (
	"testing"
)

func TestNewEventBus(t *testing.T) {
	eb := NewEventBus()
	if eb == nil {
		t.Fatal("expected non-nil EventBus")
	}
	if len(eb.global) != 0 {
		t.Errorf("expected empty global listeners, got %d", len(eb.global))
	}
	if len(eb.listeners) != 0 {
		t.Errorf("expected empty path listeners, got %d", len(eb.listeners))
	}
}

func TestEventBusSubscribeAndEmit(t *testing.T) {
	eb := NewEventBus()
	received := 0

	eb.Subscribe("/ip/address", func(event Event) {
		received++
		if event.Path != "/ip/address" {
			t.Errorf("expected path '/ip/address', got %q", event.Path)
		}
		if event.Type != EventAdd {
			t.Errorf("expected EventAdd, got %v", event.Type)
		}
	})

	eb.Emit(Event{
		Path: "/ip/address",
		Type: EventAdd,
	})

	if received != 1 {
		t.Errorf("expected handler to be called 1 time, got %d", received)
	}
}

func TestEventBusGlobalListener(t *testing.T) {
	eb := NewEventBus()
	received := 0

	eb.Subscribe("*", func(event Event) {
		received++
	})

	eb.Emit(Event{Path: "/ip/address", Type: EventAdd})
	eb.Emit(Event{Path: "/ip/arp", Type: EventRemove})

	if received != 2 {
		t.Errorf("expected global handler to be called 2 times, got %d", received)
	}
}

func TestEventBusMultipleListeners(t *testing.T) {
	eb := NewEventBus()
	count1, count2 := 0, 0

	eb.Subscribe("/test", func(event Event) { count1++ })
	eb.Subscribe("/test", func(event Event) { count2++ })

	eb.Emit(Event{Path: "/test", Type: EventUpdate})

	if count1 != 1 {
		t.Errorf("expected listener 1 to be called 1 time, got %d", count1)
	}
	if count2 != 1 {
		t.Errorf("expected listener 2 to be called 1 time, got %d", count2)
	}
}

func TestEventBusUnsubscribe(t *testing.T) {
	eb := NewEventBus()
	received := 0

	eb.Subscribe("/test", func(event Event) { received++ })
	eb.Unsubscribe("/test")
	eb.Emit(Event{Path: "/test", Type: EventAdd})

	if received != 0 {
		t.Errorf("expected handler not to be called after unsubscribe, got %d", received)
	}
}

func TestEventBusPathSpecificity(t *testing.T) {
	eb := NewEventBus()
	addrCount := 0
	arpCount := 0

	eb.Subscribe("/ip/address", func(event Event) { addrCount++ })
	eb.Subscribe("/ip/arp", func(event Event) { arpCount++ })

	eb.Emit(Event{Path: "/ip/address", Type: EventAdd})

	if addrCount != 1 {
		t.Errorf("expected address handler called 1 time, got %d", addrCount)
	}
	if arpCount != 0 {
		t.Errorf("expected arp handler not called, got %d", arpCount)
	}
}

func TestEventTypeConstants(t *testing.T) {
	if EventAdd != "add" {
		t.Errorf("expected EventAdd='add', got %q", EventAdd)
	}
	if EventRemove != "remove" {
		t.Errorf("expected EventRemove='remove', got %q", EventRemove)
	}
	if EventUpdate != "update" {
		t.Errorf("expected EventUpdate='update', got %q", EventUpdate)
	}
	if EventMove != "move" {
		t.Errorf("expected EventMove='move', got %q", EventMove)
	}
}

func TestEventWithEntry(t *testing.T) {
	entry := NewEntry("test-id", 0)
	event := Event{
		Path:  "/test",
		Type:  EventAdd,
		Entry: entry,
	}

	if event.Entry == nil {
		t.Fatal("expected non-nil entry")
	}
	if event.Entry.ID != "test-id" {
		t.Errorf("expected entry ID 'test-id', got %q", event.Entry.ID)
	}
}

func TestEventBusConcurrentSafe(t *testing.T) {
	eb := NewEventBus()
	done := make(chan bool)

	go func() {
		for i := 0; i < 100; i++ {
			eb.Subscribe("/test", func(event Event) {})
		}
		done <- true
	}()

	go func() {
		for i := 0; i < 100; i++ {
			eb.Emit(Event{Path: "/test", Type: EventAdd})
		}
		done <- true
	}()

	<-done
	<-done
}

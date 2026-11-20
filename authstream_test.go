package authstream

import (
	"context"
	"errors"
	"testing"
	"time"

	stream "github.com/gunesis-game/pulsyflux-stream"
)

func TestPublishAndSubscribe(t *testing.T) {
	events := New[AuthEvent](2)
	defer events.Close()

	sub, err := events.Subscribe(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	defer sub.Close()

	want := UserAuthenticated{Payload: Payload{
		UserID:    "user-42",
		SessionID: "session-7",
		Provider:  "oidc",
		RequestID: "request-9",
	}}
	if err := events.Publish(want); err != nil {
		t.Fatal(err)
	}

	select {
	case got, ok := <-sub.Events():
		if !ok {
			t.Fatal("subscription closed before receiving event")
		}
		if got.Data != want {
			t.Fatalf("got %#v, want %#v", got.Data, want)
		}
		if got.Time.IsZero() {
			t.Fatal("Publish did not populate event timestamp")
		}
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for authentication event")
	}
}

func TestConcreteEventStream(t *testing.T) {
	events := New[UserLoggedOut](1)
	defer events.Close()

	sub, err := events.Subscribe(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	defer sub.Close()

	want := UserLoggedOut{Payload: Payload{UserID: "user-42"}}
	if err := events.Publish(want); err != nil {
		t.Fatal(err)
	}

	select {
	case got := <-sub.Events():
		if got.Data != want {
			t.Fatalf("got %#v, want %#v", got.Data, want)
		}
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for typed event")
	}
}

func TestSlowSubscriberErrorIsPreserved(t *testing.T) {
	events := New[AuthEvent](1)
	defer events.Close()

	sub, err := events.Subscribe(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	defer sub.Close()

	event := UserAuthenticated{}
	if err := events.Publish(event); err != nil {
		t.Fatal(err)
	}
	if err := events.Publish(event); !errors.Is(err, stream.ErrSlowSubscriber) {
		t.Fatalf("got %v, want stream.ErrSlowSubscriber", err)
	}
}

func TestContextCancellation(t *testing.T) {
	events := New[AuthEvent](1)
	defer events.Close()

	ctx, cancel := context.WithCancel(context.Background())
	sub, err := events.Subscribe(ctx)
	if err != nil {
		t.Fatal(err)
	}
	cancel()

	select {
	case _, ok := <-sub.Events():
		if ok {
			t.Fatal("expected subscription channel to close")
		}
	case <-time.After(time.Second):
		t.Fatal("subscription did not close after context cancellation")
	}
}

func TestClose(t *testing.T) {
	events := New[AuthEvent](1)
	sub, err := events.Subscribe(context.Background())
	if err != nil {
		t.Fatal(err)
	}

	events.Close()
	events.Close()

	if err := events.Publish(UserLoggedOut{}); !errors.Is(err, stream.ErrClosed) {
		t.Fatalf("got %v, want stream.ErrClosed", err)
	}
	if _, ok := <-sub.Events(); ok {
		t.Fatal("expected subscription channel to be closed")
	}
}

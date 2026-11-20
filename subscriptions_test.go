package authstream

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	stream "github.com/gunesis-game/pulsyflux-stream"
)

func TestSubscribeBearerTokenPublishesActiveDecision(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := r.ParseForm(); err != nil {
			t.Fatal(err)
		}
		if got := r.Form.Get("token"); got != "opaque-token" {
			t.Fatalf("introspection token = %q", got)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"active":true}`))
	}))
	defer server.Close()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	tokens := make(chan string, 1)
	tokens <- "opaque-token"
	decisions := stream.New[bool](1)
	defer decisions.Close()
	sub, err := decisions.Subscribe(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer sub.Close()

	done := make(chan error, 1)
	go func() {
		done <- SubscribeBearerToken(ctx, tokens, server.URL, server.Client(), decisions)
	}()

	select {
	case got := <-sub.Events():
		if !got.Data {
			t.Fatal("expected active token decision")
		}
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for token decision")
	}
	cancel()
	select {
	case err := <-done:
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("SubscribeBearerToken error = %v, want context.Canceled", err)
		}
	case <-time.After(time.Second):
		t.Fatal("subscription did not stop after cancellation")
	}
}

func TestSubscribeBearerTokenRejectsBlankToken(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("blank token must not be sent to introspection endpoint")
	}))
	defer server.Close()

	tokens := make(chan string, 1)
	tokens <- " "
	decisions := stream.New[bool](1)
	defer decisions.Close()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	sub, err := decisions.Subscribe(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer sub.Close()

	done := make(chan error, 1)
	go func() {
		done <- SubscribeBearerToken(ctx, tokens, server.URL, server.Client(), decisions)
	}()
	select {
	case got := <-sub.Events():
		if got.Data {
			t.Fatal("blank token must be inactive")
		}
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for inactive decision")
	}
	cancel()
	<-done
}

func TestSubscribeBearerTokenRequiresHTTPSOrHTTPURL(t *testing.T) {
	tokens := make(chan string)
	decisions := stream.New[bool](1)
	defer decisions.Close()
	err := SubscribeBearerToken(context.Background(), tokens, "file:///tmp/token", http.DefaultClient, decisions)
	if err == nil {
		t.Fatal("expected invalid introspection URL error")
	}
}

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

type introspectionTestHandler struct {
	token string
	body  string
	calls int
}

func (h *introspectionTestHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	h.calls++
	if err := r.ParseForm(); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		return
	}
	h.token = r.Form.Get("token")
	w.Header().Set("Content-Type", "application/json")
	_, _ = w.Write([]byte(h.body))
}

func runBearerSubscription(
	ctx context.Context,
	tokens *stream.Stream[string],
	endpoint string,
	client *http.Client,
	decisions *stream.Stream[bool],
	done chan<- error,
) {
	done <- SubscribeBearerToken(ctx, tokens, endpoint, client, decisions)
}

func receiveDecision(t *testing.T, events <-chan stream.Event[bool]) bool {
	t.Helper()
	select {
	case event := <-events:
		return event.Data
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for token decision")
		return false
	}
}

func receiveSubscriptionError(t *testing.T, done <-chan error) error {
	t.Helper()
	select {
	case err := <-done:
		return err
	case <-time.After(time.Second):
		t.Fatal("subscription did not stop")
		return nil
	}
}

func TestSubscribeBearerTokenPublishesActiveDecision(t *testing.T) {
	handler := &introspectionTestHandler{body: `{"active":true}`}
	server := httptest.NewTLSServer(handler)
	defer server.Close()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	tokens := stream.New[string](1)
	defer tokens.Close()
	decisions := stream.New[bool](1)
	defer decisions.Close()
	sub, err := decisions.Subscribe(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer sub.Close()

	done := make(chan error, 1)
	go runBearerSubscription(ctx, tokens, server.URL, server.Client(), decisions, done)
	if err := tokens.Publish(stream.Event[string]{Data: "opaque-token"}); err != nil {
		t.Fatal(err)
	}
	if !receiveDecision(t, sub.Events()) {
		t.Fatal("expected active token decision")
	}
	if handler.token != "opaque-token" {
		t.Fatalf("introspection token = %q", handler.token)
	}

	cancel()
	if err := receiveSubscriptionError(t, done); !errors.Is(err, context.Canceled) {
		t.Fatalf("SubscribeBearerToken error = %v, want context.Canceled", err)
	}
}

func TestSubscribeBearerTokenRejectsBlankToken(t *testing.T) {
	handler := &introspectionTestHandler{body: `{"active":true}`}
	server := httptest.NewTLSServer(handler)
	defer server.Close()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	tokens := stream.New[string](1)
	defer tokens.Close()
	decisions := stream.New[bool](1)
	defer decisions.Close()
	sub, err := decisions.Subscribe(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer sub.Close()

	done := make(chan error, 1)
	go runBearerSubscription(ctx, tokens, server.URL, server.Client(), decisions, done)
	if err := tokens.Publish(stream.Event[string]{Data: " "}); err != nil {
		t.Fatal(err)
	}
	if receiveDecision(t, sub.Events()) {
		t.Fatal("blank token must be inactive")
	}
	if handler.calls != 0 {
		t.Fatal("blank token must not be sent to introspection endpoint")
	}

	cancel()
	if err := receiveSubscriptionError(t, done); !errors.Is(err, context.Canceled) {
		t.Fatalf("SubscribeBearerToken error = %v, want context.Canceled", err)
	}
}

func TestSubscribeBearerTokenStopsWhenInputStreamCloses(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	tokens := stream.New[string](1)
	decisions := stream.New[bool](1)
	defer decisions.Close()

	done := make(chan error, 1)
	go runBearerSubscription(ctx, tokens, "https://identity.example.com/introspect", http.DefaultClient, decisions, done)
	tokens.Close()
	if err := receiveSubscriptionError(t, done); err != nil {
		t.Fatalf("SubscribeBearerToken error = %v, want nil", err)
	}
}

func TestSubscribeBearerTokenRejectsMalformedIntrospectionResponse(t *testing.T) {
	handler := &introspectionTestHandler{body: `{}`}
	server := httptest.NewTLSServer(handler)
	defer server.Close()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	tokens := stream.New[string](1)
	defer tokens.Close()
	decisions := stream.New[bool](1)
	defer decisions.Close()

	done := make(chan error, 1)
	go runBearerSubscription(ctx, tokens, server.URL, server.Client(), decisions, done)
	if err := tokens.Publish(stream.Event[string]{Data: "opaque-token"}); err != nil {
		t.Fatal(err)
	}
	if err := receiveSubscriptionError(t, done); err == nil {
		t.Fatal("expected missing active field to fail")
	}
}

func TestSubscribeBearerTokenRequiresHTTPS(t *testing.T) {
	tokens := stream.New[string](1)
	defer tokens.Close()
	decisions := stream.New[bool](1)
	defer decisions.Close()
	err := SubscribeBearerToken(context.Background(), tokens, "http://identity.example.com/introspect", http.DefaultClient, decisions)
	if err == nil {
		t.Fatal("expected invalid introspection URL error")
	}
}

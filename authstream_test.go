package authstream

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	stream "github.com/gunesis-game/pulsyflux-stream"
	vaultstream "github.com/gunesis-game/pulsyflux-vault-stream"
)

const testTimeout = 2 * time.Second

func makeEvent[T any](t *testing.T, data T) stream.Event[T] {
	t.Helper()
	event, err := stream.NewEvent[T](stream.NewRedactionPolicy())
	if err != nil { t.Fatal(err) }
	if err := event.SetData(data); err != nil { t.Fatal(err) }
	return event
}

type testSystem struct {
	channel       *stream.EventChannel
	bearer        *stream.Stream[BearerTokenReceiveEvent]
	noBearer      *stream.Stream[BearerTokenNotReceiveEvent]
	vault         *vaultstream.Vault
	auth          *Auth
	validSub      *stream.Subscription[ValidBearerTokenEvent]
	invalidSub    *stream.Subscription[InvalidBearerTokenEvent]
	storedSub     *stream.Subscription[vaultstream.VaultTokenStoredEvent]
}

func newTestSystem(t *testing.T) *testSystem {
	t.Helper()
	ctx := context.Background()
	channel := stream.NewEventChannel()

	bearer, err := stream.New[BearerTokenReceiveEvent](channel, 64)
	if err != nil {
		t.Fatal(err)
	}
	noBearer, err := stream.New[BearerTokenNotReceiveEvent](channel, 64)
	if err != nil {
		t.Fatal(err)
	}
	vault, err := vaultstream.New(ctx, channel)
	if err != nil {
		t.Fatal(err)
	}
	storedSub, err := vault.VaultTokenStoredEvents.Subscribe(ctx)
	if err != nil {
		t.Fatal(err)
	}
	auth, err := New(ctx, channel)
	if err != nil {
		t.Fatal(err)
	}
	validEvents := stream.Get[ValidBearerTokenEvent](channel)
	if validEvents == nil {
		t.Fatal("valid-token output stream was not registered")
	}
	invalidEvents := stream.Get[InvalidBearerTokenEvent](channel)
	if invalidEvents == nil {
		t.Fatal("invalid-token output stream was not registered")
	}
	validSub, err := validEvents.Subscribe(ctx)
	if err != nil {
		t.Fatal(err)
	}
	invalidSub, err := invalidEvents.Subscribe(ctx)
	if err != nil {
		t.Fatal(err)
	}

	system := &testSystem{
		channel: channel, bearer: bearer, noBearer: noBearer, vault: vault,
		auth: auth, validSub: validSub, invalidSub: invalidSub, storedSub: storedSub,
	}
	t.Cleanup(func() {
		system.validSub.Close()
		system.invalidSub.Close()
		system.storedSub.Close()
		system.auth.Close()
		system.vault.Close()
		system.channel.Close()
	})
	return system
}

func (s *testSystem) store(t *testing.T, key, token string) {
	t.Helper()
	err := s.vault.VaultTokenStoreRequestedEvents.Publish(makeEvent(t, vaultstream.VaultTokenStoreRequestedEvent{Key: key, Token: token}))
	if err != nil {
		t.Fatal(err)
	}
	select {
	case event := <-s.storedSub.Events():
		if event.Data().Key != key {
			t.Fatalf("stored key = %q, want %q", event.Data().Key, key)
		}
	case <-time.After(testTimeout):
		t.Fatal("timed out waiting for Vault store confirmation")
	}
}

func receiveValid(t *testing.T, sub *stream.Subscription[ValidBearerTokenEvent]) {
	t.Helper()
	select {
	case _, ok := <-sub.Events():
		if !ok {
			t.Fatal("valid-token stream closed unexpectedly")
		}
	case <-time.After(testTimeout):
		t.Fatal("timed out waiting for valid-token event")
	}
}

func receiveInvalid(t *testing.T, sub *stream.Subscription[InvalidBearerTokenEvent]) {
	t.Helper()
	select {
	case _, ok := <-sub.Events():
		if !ok {
			t.Fatal("invalid-token stream closed unexpectedly")
		}
	case <-time.After(testTimeout):
		t.Fatal("timed out waiting for invalid-token event")
	}
}

func TestNewRequiresAllExternalStreams(t *testing.T) {
	channel := stream.NewEventChannel()
	defer channel.Close()

	if _, err := New(context.Background(), channel); !errors.Is(err, ErrMissingStream) {
		t.Fatalf("New error = %v, want ErrMissingStream", err)
	}
}

func TestAuthValidatesPresentedTokenAgainstVault(t *testing.T) {
	t.Run("matching token", func(t *testing.T) {
		s := newTestSystem(t)
		s.store(t, "presented-token", "presented-token")
		if err := s.bearer.Publish(makeEvent(t, BearerTokenReceiveEvent{Token: "presented-token"})); err != nil {
			t.Fatal(err)
		}
		receiveValid(t, s.validSub)
	})

	t.Run("mismatching token", func(t *testing.T) {
		s := newTestSystem(t)
		s.store(t, "presented-token", "different-stored-token")
		if err := s.bearer.Publish(makeEvent(t, BearerTokenReceiveEvent{Token: "presented-token"})); err != nil {
			t.Fatal(err)
		}
		receiveInvalid(t, s.invalidSub)
	})

	t.Run("not found", func(t *testing.T) {
		s := newTestSystem(t)
		if err := s.bearer.Publish(makeEvent(t, BearerTokenReceiveEvent{Token: "missing-token"})); err != nil {
			t.Fatal(err)
		}
		receiveInvalid(t, s.invalidSub)
	})

	t.Run("no bearer token", func(t *testing.T) {
		s := newTestSystem(t)
		if err := s.noBearer.Publish(makeEvent(t, BearerTokenNotReceiveEvent{})); err != nil {
			t.Fatal(err)
		}
		receiveInvalid(t, s.invalidSub)
	})
}

func TestAuthHandlesConcurrentLookups(t *testing.T) {
	s := newTestSystem(t)
	tokens := []string{"token-one", "token-two", "token-three"}
	for _, token := range tokens {
		s.store(t, token, token)
	}
	for _, token := range tokens {
		if err := s.bearer.Publish(makeEvent(t, BearerTokenReceiveEvent{Token: token})); err != nil {
			t.Fatal(err)
		}
	}
	for range tokens {
		receiveValid(t, s.validSub)
	}
}

func TestAuthDoesNotLeakTokenInOutcomeEvents(t *testing.T) {
	s := newTestSystem(t)
	secret := "sensitive-bearer-value"
	s.store(t, secret, "not-the-same-value")
	if err := s.bearer.Publish(makeEvent(t, BearerTokenReceiveEvent{Token: secret})); err != nil {
		t.Fatal(err)
	}
	event := waitInvalidEvent(t, s.invalidSub)
	if event.Data() != (InvalidBearerTokenEvent{}) {
		t.Fatalf("invalid outcome unexpectedly contains data: %#v", event.Data())
	}
}

func waitInvalidEvent(t *testing.T, sub *stream.Subscription[InvalidBearerTokenEvent]) stream.Event[InvalidBearerTokenEvent] {
	t.Helper()
	select {
	case event := <-sub.Events():
		return event
	case <-time.After(testTimeout):
		t.Fatal("timed out waiting for invalid-token event")
		return stream.Event[InvalidBearerTokenEvent]{}
	}
}

func TestEventChannelsIsolateWorkflows(t *testing.T) {
	first := newTestSystem(t)
	second := newTestSystem(t)
	first.store(t, "first-token", "first-token")
	second.store(t, "second-token", "second-token")

	if err := first.bearer.Publish(makeEvent(t, BearerTokenReceiveEvent{Token: "first-token"})); err != nil {
		t.Fatal(err)
	}
	receiveValid(t, first.validSub)

	select {
	case event := <-second.validSub.Events():
		t.Fatalf("workflow two received workflow one's event: %#v", event)
	case event := <-second.invalidSub.Events():
		t.Fatalf("workflow two received workflow one's invalid event: %#v", event)
	case <-time.After(100 * time.Millisecond):
	}
}

func TestAuthStopsOnContextCancellation(t *testing.T) {
	channel := stream.NewEventChannel()
	defer channel.Close()
	bearer, err := stream.New[BearerTokenReceiveEvent](channel, 8)
	if err != nil {
		t.Fatal(err)
	}
	noBearer, err := stream.New[BearerTokenNotReceiveEvent](channel, 8)
	if err != nil {
		t.Fatal(err)
	}
	vault, err := vaultstream.New(context.Background(), channel)
	if err != nil {
		t.Fatal(err)
	}
	defer vault.Close()

	ctx, cancel := context.WithCancel(context.Background())
	auth, err := New(ctx, channel)
	if err != nil {
		cancel()
		t.Fatal(err)
	}
	cancel()
	auth.Close()

	// Closing Auth must leave the externally owned streams and shared channel
	// open; upstream can still publish after Auth has stopped.
	if err := bearer.Publish(makeEvent(t, BearerTokenReceiveEvent{Token: "after-close"})); err != nil {
		t.Fatalf("Auth.Close closed upstream stream: %v", err)
	}
	if err := noBearer.Publish(makeEvent(t, BearerTokenNotReceiveEvent{})); err != nil {
		t.Fatalf("Auth.Close closed no-bearer stream: %v", err)
	}
}


func TestNewRejectsNilEventChannel(t *testing.T) {
	if _, err := New(context.Background(), nil); !errors.Is(err, ErrNilEventChannel) {
		t.Fatalf("New error = %v, want ErrNilEventChannel", err)
	}
}

func TestNewAcceptsNilContext(t *testing.T) {
	channel := stream.NewEventChannel()
	defer channel.Close()
	if _, err := stream.New[BearerTokenReceiveEvent](channel, 8); err != nil {
		t.Fatal(err)
	}
	if _, err := stream.New[BearerTokenNotReceiveEvent](channel, 8); err != nil {
		t.Fatal(err)
	}
	vault, err := vaultstream.New(nil, channel)
	if err != nil {
		t.Fatal(err)
	}
	defer vault.Close()
	auth, err := New(nil, channel)
	if err != nil {
		t.Fatalf("New with nil context: %v", err)
	}
	auth.Close()
}

func TestAuthRejectsEmptyAndWhitespaceTokens(t *testing.T) {
	for _, token := range []string{"", " ", "\t\n"} {
		t.Run(fmt.Sprintf("%q", token), func(t *testing.T) {
			s := newTestSystem(t)
			if err := s.bearer.Publish(makeEvent(t, BearerTokenReceiveEvent{Token: token})); err != nil {
				t.Fatal(err)
			}
			receiveInvalid(t, s.invalidSub)
			assertNoValidEvent(t, s.validSub)
		})
	}
}

func TestAuthHandlesRepeatedIdenticalTokensConcurrently(t *testing.T) {
	s := newTestSystem(t)
	const count = 12
	s.store(t, "repeated-token", "repeated-token")

	for i := 0; i < count; i++ {
		if err := s.bearer.Publish(makeEvent(t, BearerTokenReceiveEvent{Token: "repeated-token"})); err != nil {
			t.Fatal(err)
		}
	}
	for i := 0; i < count; i++ {
		receiveValid(t, s.validSub)
	}
	assertNoInvalidEvent(t, s.invalidSub)
}

func TestAuthPublishesOneInvalidOutcomePerNoBearerEvent(t *testing.T) {
	s := newTestSystem(t)
	const count = 5
	for i := 0; i < count; i++ {
		if err := s.noBearer.Publish(makeEvent(t, BearerTokenNotReceiveEvent{})); err != nil {
			t.Fatal(err)
		}
	}
	for i := 0; i < count; i++ {
		receiveInvalid(t, s.invalidSub)
	}
	assertNoValidEvent(t, s.validSub)
}

func TestAuthIgnoresVaultResponsesWithoutPendingLookup(t *testing.T) {
	s := newTestSystem(t)
	if err := s.vault.VaultTokenFoundEvents.Publish(makeEvent(t, vaultstream.VaultTokenFoundEvent{Key: "unsolicited", Token: "unsolicited"})); err != nil {
		t.Fatal(err)
	}
	if err := s.vault.VaultTokenNotFoundEvents.Publish(makeEvent(t, vaultstream.VaultTokenNotFoundEvent{Key: "also-unsolicited"})); err != nil {
		t.Fatal(err)
	}
	assertNoValidEvent(t, s.validSub)
	assertNoInvalidEvent(t, s.invalidSub)
}

func TestAuthCloseIsIdempotentAndLeavesOutputStreamsOpen(t *testing.T) {
	s := newTestSystem(t)
	s.auth.Close()
	s.auth.Close()

	valid := stream.Get[ValidBearerTokenEvent](s.channel)
	invalid := stream.Get[InvalidBearerTokenEvent](s.channel)
	if valid == nil || invalid == nil {
		t.Fatal("Auth.Close removed an Auth output stream")
	}
	validSub, err := valid.Subscribe(context.Background())
	if err != nil {
		t.Fatalf("subscribe to valid output after Auth.Close: %v", err)
	}
	validSub.Close()
	invalidSub, err := invalid.Subscribe(context.Background())
	if err != nil {
		t.Fatalf("subscribe to invalid output after Auth.Close: %v", err)
	}
	invalidSub.Close()
}

func assertNoValidEvent(t *testing.T, sub *stream.Subscription[ValidBearerTokenEvent]) {
	t.Helper()
	select {
	case event := <-sub.Events():
		t.Fatalf("unexpected valid-token event: %#v", event)
	case <-time.After(75 * time.Millisecond):
	}
}

func assertNoInvalidEvent(t *testing.T, sub *stream.Subscription[InvalidBearerTokenEvent]) {
	t.Helper()
	select {
	case event := <-sub.Events():
		t.Fatalf("unexpected invalid-token event: %#v", event)
	case <-time.After(75 * time.Millisecond):
	}
}

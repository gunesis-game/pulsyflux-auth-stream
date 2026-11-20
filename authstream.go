// Package authstream validates bearer tokens against tokens held by a
// pulsyflux-vault-stream on the same pulsyflux-stream EventChannel.
package authstream

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"

	stream "github.com/gunesis-game/pulsyflux-stream"
	vaultstream "github.com/gunesis-game/pulsyflux-vault-stream"
)

const subscriberBuffer = 32

var (
	ErrNilEventChannel = errors.New("authstream: nil event channel")
	ErrMissingStream   = errors.New("authstream: required event stream is not registered on the EventChannel")
)

// BearerTokenReceiveEvent is published by an upstream component when it
// receives a bearer token. Auth owns this event definition; upstream imports
// this package and publishes the event on the workflow's EventChannel.
type BearerTokenReceiveEvent struct {
	Token string
}

// BearerTokenNotReceiveEvent is published by an upstream component when no
// bearer token was received. Auth owns this event definition.
type BearerTokenNotReceiveEvent struct{}

// ValidBearerTokenEvent is published when the presented token matches the
// token returned by Vault. It deliberately contains no credential.
type ValidBearerTokenEvent struct{}

// InvalidBearerTokenEvent is published when no bearer token was received,
// Vault has no token for the lookup key, or the returned token does not match.
// It deliberately contains no credential.
type InvalidBearerTokenEvent struct{}

// pendingLookup retains the credential presented upstream independently of
// the Vault lookup key. Count supports repeated lookups for the same key
// without introducing per-request correlation IDs.
type pendingLookup struct {
	presentedToken string
	count          int
}

// Auth consumes upstream bearer events and Vault lookup responses, and
// publishes Auth-owned validation outcomes. It never owns or closes the
// EventChannel or the upstream/Vault streams.
type Auth struct {
	validBearerTokenEvents   *stream.Stream[ValidBearerTokenEvent]
	invalidBearerTokenEvents *stream.Stream[InvalidBearerTokenEvent]

	cancel context.CancelFunc
	wg     sync.WaitGroup
	once   sync.Once

	mu      sync.Mutex
	pending map[string]*pendingLookup
	errors  chan error
}

// New connects Auth to streams already registered on channel. The upstream
// component must register BearerTokenReceiveEvent and
// BearerTokenNotReceiveEvent streams first; Vault must be initialized on the
// same channel first. Auth registers its two output streams.
//
// The presented bearer token is used as Vault's lookup Key. Auth separately
// retains the presented token as the expected value for comparison. The
// shared EventChannel isolates workflows; no per-request correlation ID is
// used. Subscribe to Auth's output streams before publishing upstream events.
func New(ctx context.Context, channel *stream.EventChannel) (*Auth, error) {
	if channel == nil {
		return nil, ErrNilEventChannel
	}
	if ctx == nil {
		ctx = context.Background()
	}

	bearerReceived := stream.Get[BearerTokenReceiveEvent](channel)
	bearerNotReceived := stream.Get[BearerTokenNotReceiveEvent](channel)
	vaultRequested := stream.Get[vaultstream.VaultTokenRequestedEvent](channel)
	vaultFound := stream.Get[vaultstream.VaultTokenFoundEvent](channel)
	vaultNotFound := stream.Get[vaultstream.VaultTokenNotFoundEvent](channel)
	if bearerReceived == nil || bearerNotReceived == nil ||
		vaultRequested == nil || vaultFound == nil || vaultNotFound == nil {
		return nil, ErrMissingStream
	}

	validEvents, err := stream.New[ValidBearerTokenEvent](channel, subscriberBuffer)
	if err != nil {
		return nil, fmt.Errorf("authstream: register valid-token output: %w", err)
	}
	invalidEvents, err := stream.New[InvalidBearerTokenEvent](channel, subscriberBuffer)
	if err != nil {
		return nil, fmt.Errorf("authstream: register invalid-token output: %w", err)
	}

	runCtx, cancel := context.WithCancel(ctx)
	auth := &Auth{
		validBearerTokenEvents:   validEvents,
		invalidBearerTokenEvents: invalidEvents,
		cancel:                   cancel,
		pending:                  make(map[string]*pendingLookup),
		errors:                   make(chan error, subscriberBuffer),
	}

	// Establish response subscriptions before input subscriptions. This makes
	// sure Auth is listening before an upstream event can trigger a lookup.
	foundSub, err := vaultFound.Subscribe(runCtx)
	if err != nil {
		cancel()
		return nil, fmt.Errorf("authstream: subscribe to Vault found events: %w", err)
	}
	notFoundSub, err := vaultNotFound.Subscribe(runCtx)
	if err != nil {
		foundSub.Close()
		cancel()
		return nil, fmt.Errorf("authstream: subscribe to Vault not-found events: %w", err)
	}
	receivedSub, err := bearerReceived.Subscribe(runCtx)
	if err != nil {
		foundSub.Close()
		notFoundSub.Close()
		cancel()
		return nil, fmt.Errorf("authstream: subscribe to bearer-token events: %w", err)
	}
	notReceivedSub, err := bearerNotReceived.Subscribe(runCtx)
	if err != nil {
		foundSub.Close()
		notFoundSub.Close()
		receivedSub.Close()
		cancel()
		return nil, fmt.Errorf("authstream: subscribe to no-bearer events: %w", err)
	}

	auth.wg.Add(4)
	go auth.handleBearerTokens(runCtx, receivedSub, vaultRequested)
	go auth.handleNoBearerTokens(runCtx, notReceivedSub)
	go auth.handleFoundTokens(runCtx, foundSub)
	go auth.handleNotFoundTokens(runCtx, notFoundSub)

	return auth, nil
}

// Errors returns a best-effort stream of asynchronous publish errors. The
// channel is closed when Auth.Close has stopped all of its workers.
func (a *Auth) Errors() <-chan error {
	if a == nil {
		return nil
	}
	return a.errors
}

// Close stops Auth's subscriptions and workers. It does not close the shared
// EventChannel or any upstream, Vault, or Auth output stream.
func (a *Auth) Close() {
	if a == nil {
		return
	}
	a.once.Do(func() {
		a.cancel()
		a.wg.Wait()
		close(a.errors)
	})
}

func (a *Auth) handleBearerTokens(
	ctx context.Context,
	sub *stream.Subscription[BearerTokenReceiveEvent],
	requests *stream.Stream[vaultstream.VaultTokenRequestedEvent],
) {
	defer a.wg.Done()
	defer sub.Close()

	for {
		select {
		case <-ctx.Done():
			return
		case event, ok := <-sub.Events():
			if !ok {
				return
			}
			presentedToken := event.Data.Token
			if strings.TrimSpace(presentedToken) == "" {
				a.publishInvalid()
				continue
			}

			a.addPending(presentedToken, presentedToken)

			err := requests.Publish(stream.Event[vaultstream.VaultTokenRequestedEvent]{
				Name: "vault.token.requested",
				Data: vaultstream.VaultTokenRequestedEvent{Key: presentedToken},
			})
			if err != nil {
				a.consumePending(presentedToken)
				a.publishInvalid()
				a.report(fmt.Errorf("authstream: publish Vault lookup request: %w", err))
			}
		}
	}
}

func (a *Auth) handleNoBearerTokens(
	ctx context.Context,
	sub *stream.Subscription[BearerTokenNotReceiveEvent],
) {
	defer a.wg.Done()
	defer sub.Close()

	for {
		select {
		case <-ctx.Done():
			return
		case _, ok := <-sub.Events():
			if !ok {
				return
			}
			a.publishInvalid()
		}
	}
}

func (a *Auth) handleFoundTokens(
	ctx context.Context,
	sub *stream.Subscription[vaultstream.VaultTokenFoundEvent],
) {
	defer a.wg.Done()
	defer sub.Close()

	for {
		select {
		case <-ctx.Done():
			return
		case event, ok := <-sub.Events():
			if !ok {
				return
			}
			presentedToken, ok := a.consumePending(event.Data.Key)
			if !ok {
				continue
			}
			if event.Data.Token == presentedToken {
				a.publishValid()
			} else {
				a.publishInvalid()
			}
		}
	}
}

func (a *Auth) handleNotFoundTokens(
	ctx context.Context,
	sub *stream.Subscription[vaultstream.VaultTokenNotFoundEvent],
) {
	defer a.wg.Done()
	defer sub.Close()

	for {
		select {
		case <-ctx.Done():
			return
		case event, ok := <-sub.Events():
			if !ok {
				return
			}
			if _, ok := a.consumePending(event.Data.Key); ok {
				a.publishInvalid()
			}
		}
	}
}

func (a *Auth) addPending(key, presentedToken string) {
	a.mu.Lock()
	defer a.mu.Unlock()

	pending := a.pending[key]
	if pending == nil {
		pending = &pendingLookup{presentedToken: presentedToken}
		a.pending[key] = pending
	}
	pending.count++
}

func (a *Auth) consumePending(key string) (string, bool) {
	a.mu.Lock()
	defer a.mu.Unlock()

	pending := a.pending[key]
	if pending == nil || pending.count == 0 {
		return "", false
	}

	presentedToken := pending.presentedToken
	pending.count--
	if pending.count == 0 {
		delete(a.pending, key)
	}
	return presentedToken, true
}

func (a *Auth) publishValid() {
	if err := a.validBearerTokenEvents.Publish(stream.Event[ValidBearerTokenEvent]{
		Name: "auth.bearer.valid",
		Data: ValidBearerTokenEvent{},
	}); err != nil {
		a.report(fmt.Errorf("authstream: publish valid-token outcome: %w", err))
	}
}

func (a *Auth) publishInvalid() {
	if err := a.invalidBearerTokenEvents.Publish(stream.Event[InvalidBearerTokenEvent]{
		Name: "auth.bearer.invalid",
		Data: InvalidBearerTokenEvent{},
	}); err != nil {
		a.report(fmt.Errorf("authstream: publish invalid-token outcome: %w", err))
	}
}

func (a *Auth) report(err error) {
	select {
	case a.errors <- err:
	default:
		// Error reporting must never block an event-consumer goroutine.
	}
}

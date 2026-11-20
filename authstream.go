// Package authstream provides typed authentication events on top of
// pulsyflux-stream.
package authstream

import (
	"context"
	"errors"

	stream "github.com/gunesis-game/pulsyflux-stream"
)

// Common authentication event names. Applications may publish additional
// namespaced event names when integrating their own authentication flows.
const (
	UserRegistered         = "auth.user.registered"
	UserAuthenticated      = "auth.user.authenticated"
	AuthenticationFailed   = "auth.authentication.failed"
	UserLoggedOut          = "auth.user.logged_out"
	TokenIssued            = "auth.token.issued"
	TokenRefreshed         = "auth.token.refreshed"
	TokenRevoked           = "auth.token.revoked"
	PasswordResetRequested = "auth.password_reset.requested"
	PasswordResetCompleted = "auth.password_reset.completed"
	MFAChallengeIssued     = "auth.mfa.challenge_issued"
	MFAChallengePassed     = "auth.mfa.challenge_passed"
	MFAChallengeFailed     = "auth.mfa.challenge_failed"
)

// ErrInvalidEvent is returned when an event has no name.
var ErrInvalidEvent = errors.New("authstream: event name must not be empty")

// Payload contains non-secret metadata about an authentication event.
//
// Do not put passwords, access tokens, refresh tokens, one-time codes, private
// keys, or other credentials in these fields. Reason should be a short,
// non-sensitive machine-readable description (for example, "invalid_credentials").
type Payload struct {
	UserID    string
	SessionID string
	Provider  string
	RequestID string
	Reason    string
}

// Event is the event envelope used by the underlying pulsyflux stream.
// Name should be one of the constants above or an application-specific,
// namespaced event name. Time is populated by Publish when it is zero.
type Event = stream.Event[Payload]

// Subscription receives authentication events.
type Subscription = stream.Subscription[Payload]

// Stream distributes authentication events to active subscribers.
type Stream struct {
	events *stream.Stream[Payload]
}

// New creates an authentication event stream with bufferSize slots per
// subscriber. bufferSize must be greater than zero; otherwise New panics in
// the same way as pulsyflux-stream.New.
func New(bufferSize int) *Stream {
	return &Stream{events: stream.New[Payload](bufferSize)}
}

// Subscribe registers a subscriber. Its channel closes when ctx is cancelled,
// the subscription is closed, or the stream is closed. A nil context is treated
// as context.Background().
func (s *Stream) Subscribe(ctx context.Context) (*Subscription, error) {
	return s.events.Subscribe(ctx)
}

// Publish sends an authentication event to every active subscriber. An empty
// event name is rejected. Delivery, buffering, timestamps, and slow-subscriber
// behavior are provided by pulsyflux-stream.
func (s *Stream) Publish(event Event) error {
	if event.Name == "" {
		return ErrInvalidEvent
	}
	return s.events.Publish(event)
}

// Close closes the stream and all active subscriber channels. It is idempotent.
func (s *Stream) Close() {
	s.events.Close()
}

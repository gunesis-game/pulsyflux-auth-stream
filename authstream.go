// Package authstream provides typed authentication events on top of
// pulsyflux-stream.
package authstream

import (
	"context"
	"time"

	stream "github.com/gunesis-game/pulsyflux-stream"
)

// AuthEvent is implemented by the authentication event types in this package.
// Its unexported method keeps the set of event types controlled by this package.
type AuthEvent interface {
	authEvent()
}

// Payload contains non-secret metadata shared by authentication events.
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

// Each authentication event has its own Go type. Embed Payload to carry the
// common event metadata without string-based event names.
type UserRegistered struct{ Payload }
type UserAuthenticated struct{ Payload }
type AuthenticationFailed struct{ Payload }
type UserLoggedOut struct{ Payload }
type TokenIssued struct{ Payload }
type TokenRefreshed struct{ Payload }
type TokenRevoked struct{ Payload }
type PasswordResetRequested struct{ Payload }
type PasswordResetCompleted struct{ Payload }
type MFAChallengeIssued struct{ Payload }
type MFAChallengePassed struct{ Payload }
type MFAChallengeFailed struct{ Payload }

func (UserRegistered) authEvent()         {}
func (UserAuthenticated) authEvent()      {}
func (AuthenticationFailed) authEvent()   {}
func (UserLoggedOut) authEvent()          {}
func (TokenIssued) authEvent()            {}
func (TokenRefreshed) authEvent()         {}
func (TokenRevoked) authEvent()           {}
func (PasswordResetRequested) authEvent() {}
func (PasswordResetCompleted) authEvent() {}
func (MFAChallengeIssued) authEvent()     {}
func (MFAChallengePassed) authEvent()     {}
func (MFAChallengeFailed) authEvent()     {}

// Event is the timestamped envelope delivered by a subscription.
type Event[T AuthEvent] struct {
	Data T
	Time time.Time
}

// Subscription receives typed authentication events.
type Subscription[T AuthEvent] struct {
	sub *stream.Subscription[T]
}

// Events returns the read-only channel of typed authentication events.
func (s *Subscription[T]) Events() <-chan Event[T] {
	if s == nil || s.sub == nil {
		return nil
	}
	out := make(chan Event[T])
	go func() {
		defer close(out)
		for event := range s.sub.Events() {
			out <- Event[T]{Data: event.Data, Time: event.Time}
		}
	}()
	return out
}

// Close unregisters the subscription. It is safe to call more than once.
func (s *Subscription[T]) Close() {
	if s != nil && s.sub != nil {
		s.sub.Close()
	}
}

// Stream distributes typed authentication events to active subscribers.
type Stream[T AuthEvent] struct {
	events *stream.Stream[T]
}

// New creates an authentication event stream with bufferSize slots per
// subscriber. Choose AuthEvent to publish all built-in event types, or a
// concrete event type to restrict the stream to that event type.
func New[T AuthEvent](bufferSize int) *Stream[T] {
	return &Stream[T]{events: stream.New[T](bufferSize)}
}

// Subscribe registers a subscriber. Its channel closes when ctx is cancelled,
// the subscription is closed, or the stream is closed. A nil context is treated
// as context.Background().
func (s *Stream[T]) Subscribe(ctx context.Context) (*Subscription[T], error) {
	sub, err := s.events.Subscribe(ctx)
	if err != nil {
		return nil, err
	}
	return &Subscription[T]{sub: sub}, nil
}

// Publish sends a typed authentication event to every active subscriber.
// Delivery, buffering, timestamps, and slow-subscriber behavior are provided
// by pulsyflux-stream.
func (s *Stream[T]) Publish(event T) error {
	return s.events.Publish(stream.Event[T]{Data: event})
}

// Close closes the stream and all active subscriber channels. It is idempotent.
func (s *Stream[T]) Close() {
	s.events.Close()
}

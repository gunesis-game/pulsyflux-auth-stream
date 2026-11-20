# pulsyflux-auth-stream

A generic, typed authentication event stream built on
[pulsyflux-stream](https://github.com/gunesis-game/pulsyflux-stream). Each
authentication event is represented by its own Go type. There are no
string-based event names; the compiler checks event types at publish time.

## Requirements

Go 1.27 or newer.

## Install

```sh
go get github.com/gunesis-game/pulsyflux-auth-stream
```

## Quick start

```go
package main

import (
	"context"
	"log"

	authstream "github.com/gunesis-game/pulsyflux-auth-stream"
)

func main() {
	// AuthEvent lets this stream carry every built-in authentication event.
	events := authstream.New[authstream.AuthEvent](32)
	defer events.Close()

	sub, err := events.Subscribe(context.Background())
	if err != nil {
		log.Fatal(err)
	}
	defer sub.Close()

	err = events.Publish(authstream.UserAuthenticated{
		Payload: authstream.Payload{
			UserID:    "user-42",
			SessionID: "session-7",
			Provider:  "oidc",
			RequestID: "request-9",
		},
	})
	if err != nil {
		log.Fatal(err)
	}

	event := <-sub.Events()
	switch value := event.Data.(type) {
	case authstream.UserAuthenticated:
		log.Printf("user %s authenticated at %s", value.UserID, event.Time)
	case authstream.AuthenticationFailed:
		log.Printf("authentication failed for user %s: %s",
			value.UserID, value.Reason)
	}
}
```

## Event types

Each event is a distinct Go type embedding the common `Payload`:

- `UserRegistered`
- `UserAuthenticated`
- `AuthenticationFailed`
- `UserLoggedOut`
- `TokenIssued`
- `TokenRefreshed`
- `TokenRevoked`
- `PasswordResetRequested`
- `PasswordResetCompleted`
- `MFAChallengeIssued`
- `MFAChallengePassed`
- `MFAChallengeFailed`

Use `New[AuthEvent](bufferSize)` to publish and subscribe to all built-in
authentication event types. You can also restrict a stream to one event type,
for example `New[UserAuthenticated](bufferSize)`. In that case, publishing
any other event type is rejected by the Go compiler.

## Payload and security

`Payload` contains optional `UserID`, `SessionID`, `Provider`,
`RequestID`, and `Reason` fields. These are intended for identifiers and
non-secret metadata. **Never publish passwords, access or refresh tokens,
one-time codes, private keys, or other credentials.** Keep `Reason` short and
non-sensitive (for example, `invalid_credentials`); do not include raw
provider errors or user-submitted data.

The stream is an in-process event fan-out mechanism, not a durable message
broker or an authentication system. It does not persist events, authenticate
callers, or encrypt payloads.

## Semantics

All delivery and lifecycle semantics are inherited from
[pulsyflux-stream](https://github.com/gunesis-game/pulsyflux-stream):

- Each subscriber receives each published event.
- Publishing is safe for concurrent callers.
- Each subscriber has a bounded buffer configured by `New`.
- If any subscriber buffer is full, `Publish` returns
  `stream.ErrSlowSubscriber` and delivers to none of the subscribers for
  that call.
- Publishing with no subscribers succeeds.
- Event timestamps are populated automatically.
- Subscription close, context cancellation, and stream close are safe to race.
- Publishing after shutdown returns `stream.ErrClosed`.
- `New` panics when buffer size is less than one.

## Development

```sh
go test -race ./...
go vet ./...
go build ./...
```

## Releases

Push a semantic version tag such as `v1.0.0` to trigger the release
workflow. It runs the race-enabled test suite, `go vet`, and `go build`
before creating a GitHub Release with automatically generated release notes.
No binary artifacts are published; consumers install the Go module using the
tagged version.

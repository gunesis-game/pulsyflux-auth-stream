# pulsyflux-auth-stream

A typed authentication event stream built on
[pulsyflux-stream](https://github.com/gunesis-game/pulsyflux-stream). It
provides authentication-oriented event names and payload fields while reusing
the underlying package's concurrency-safe fan-out, bounded subscriber
buffers, atomic delivery, and lifecycle behavior.

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
	events := authstream.New(32)
	defer events.Close()

	sub, err := events.Subscribe(context.Background())
	if err != nil {
		log.Fatal(err)
	}
	defer sub.Close()

	if err := events.Publish(authstream.Event{
		Name: authstream.UserAuthenticated,
		Data: authstream.Payload{
			UserID:    "user-42",
			SessionID: "session-7",
			Provider:  "oidc",
			RequestID: "request-9",
		},
	}); err != nil {
		log.Fatal(err)
	}

	event := <-sub.Events()
	log.Printf("%s for user %s at %s",
		event.Name, event.Data.UserID, event.Time)
}
```

## Event names

The package exports these common event names:

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

Names are strings, so applications can also publish additional namespaced
events. Events with an empty name are rejected with `ErrInvalidEvent`.

## Payload and security

`Payload` contains optional `UserID`, `SessionID`, `Provider`, `RequestID`,
and `Reason` fields. These are intended for identifiers and non-secret
metadata. **Never publish passwords, access or refresh tokens, one-time codes,
private keys, or other credentials.** Keep `Reason` short and non-sensitive
(for example, `invalid_credentials`); do not include raw provider errors or
user-submitted data.

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
- A zero event timestamp is populated automatically.
- Subscription close, context cancellation, and stream close are safe to race.
- Publishing after shutdown returns `stream.ErrClosed`.
- `New` panics when buffer size is less than one.

## Development

```sh
go test -race ./...
go vet ./...
go build ./...
```

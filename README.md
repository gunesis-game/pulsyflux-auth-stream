# pulsyflux-auth-stream

A typed authentication event implementation built on
[pulsyflux-stream](https://github.com/gunesis-game/pulsyflux-stream). The
generic stream provides channel-based delivery; this package defines
authentication request events, event-owned subscriptions, and typed decision
events.

## Requirements

Go 1.27 or newer.

## Install

```sh
go get github.com/gunesis-game/pulsyflux-auth-stream
```

## Event-owned subscriptions

Each request event has its own `Subscribe()` method. The method subscribes to
a stream containing only that request type, reads from the subscription
channel, performs the relevant authentication check through a narrow Go
interface, and publishes a corresponding decision event.

There are no callbacks, anonymous functions, generic handler functions, or
central type switches.

```go
ctx, cancel := context.WithCancel(context.Background())
defer cancel()

requests := authstream.New[authstream.UserAuthenticationRequested](16)
decisions := authstream.New[authstream.AuthEvent](16)
defer requests.Close()
defer decisions.Close()

// Implement UserAuthenticator using the application's credential store or
// identity provider. Credential material is not carried in stream payloads.
go (authstream.UserAuthenticationRequested{}).Subscribe(
    ctx, requests, authenticator, decisions,
)
```

The initial event-owned subscription implementations are:

- `UserAuthenticationRequested.Subscribe()`: delegates the credential check
  to `UserAuthenticator`, then publishes
  `UserAuthenticationApproved` or `UserAuthenticationDenied`.
- `MFAVerificationRequested.Subscribe()`: delegates MFA verification to
  `MFAVerifier`, then publishes `MFAVerificationApproved` or
  `MFAVerificationDenied`.

The application supplies these interfaces; this package does not invent
credential-validation policy.

## Event types

Existing lifecycle events remain available:

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

Request and decision events:

- `UserAuthenticationRequested`
- `UserAuthenticationApproved`
- `UserAuthenticationDenied`
- `MFAVerificationRequested`
- `MFAVerificationApproved`
- `MFAVerificationDenied`

Use `New[AuthEvent](bufferSize)` for a stream carrying all built-in event
types, or use a concrete event type for a stream restricted to one type.

## Payload and security

`Payload` contains optional `UserID`, `SessionID`, `Provider`,
`RequestID`, and `Reason` fields. These are identifiers and non-secret
metadata. Never publish passwords, access or refresh tokens, one-time codes,
private keys, or other credentials. Reasons are short machine-readable
values, not raw provider errors or user-submitted data.

The stream is an in-process event fan-out mechanism, not a durable message
broker or an authentication system. It does not persist events, authenticate
callers, or encrypt payloads.

## Development

```sh
go test -race ./...
go vet ./...
go build ./...
```

## Releases

Push a semantic version tag such as `v1.0.0` to trigger the release
workflow. It runs the race-enabled test suite, `go vet`, and `go build`
before creating a GitHub Release.

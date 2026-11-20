# Steering — pulsyflux-auth-stream

This file records the current contract and implementation decisions. Read it
before changing the package. Work directly on `main`; do not create a branch.

## Responsibility

Auth owns bearer-token validation. Upstream components report whether a token
was received and publish Auth-defined input events. Upstream does not validate
the token. Auth requests the stored token from Vault and compares it with the
presented token.

## Event ownership

Auth defines and exposes these input events for upstream publishers:

- `BearerTokenReceiveEvent{Token string}`
- `BearerTokenNotReceiveEvent{}`

Auth publishes only these credential-free outcomes:

- `ValidBearerTokenEvent{}`
- `InvalidBearerTokenEvent{}`

Vault owns and registers:

- `VaultTokenRequestedEvent` — Auth publishes it.
- `VaultTokenFoundEvent` — Auth subscribes.
- `VaultTokenNotFoundEvent` — Auth subscribes.

Auth does not own, register, or close upstream or Vault streams. It registers
only its valid/invalid output streams.

## Workflow and lookup contract

- Components participating in one workflow share one `*stream.EventChannel`.
- Separate workflows use separate EventChannels; this is the isolation boundary.
- No per-request `RequestID` is part of this contract.
- The presented bearer token itself is the Vault lookup `Key`.
- A found response is accepted only when Auth has a pending lookup for its
  `Key`; Auth compares the returned `Token` with that key.
- A not-found response for a pending key produces an invalid outcome.
- A no-bearer input produces an invalid outcome without a Vault lookup.
- Credentials are never copied into Auth outcome events or logged.

The pending lookup count supports repeated concurrent presentations of the same
token. Vault response events are broadcast to subscribers on their typed
stream, so the channel is a workflow scope, not a per-message correlation ID.
Keep one Auth validation consumer per workflow channel.

## Construction and shutdown

1. Upstream registers the two Auth input event streams on the channel.
2. Vault is initialized on that same channel and registers its streams.
3. Auth.New retrieves those streams, registers its output streams, subscribes
   to Vault responses first, then subscribes to upstream inputs.
4. The application subscribes to Auth outputs before publishing input events.
5. Auth.Close cancels and joins only Auth-owned subscriptions/workers. It does
   not close the EventChannel or any upstream/Vault/output streams.
6. The application closes the shared EventChannel after all workflow
   components have stopped.

## Implementation notes

- Typed event payloads use the `Event` suffix.
- Do not add generic lifecycle events, password-reset/MFA events, callbacks,
  reflection dispatch, or string-based event routing.
- Keep Vault events outside any closed Auth-only event union.
- Publish errors from worker goroutines are exposed through Auth.Errors().
- The dependency versions in go.mod must be compatible with the Vault module's
  pinned pulsyflux-stream version.
- Do not claim tests or builds passed unless a real run or CI result confirms it.

## Verification checklist

Run `gofmt`, `go mod tidy`, `go test -race ./...`, `go vet ./...`, and
`go build ./...`. Tests should cover matching and mismatching tokens,
not-found and no-bearer paths, concurrent lookups, channel isolation, and
cancellation/shutdown.

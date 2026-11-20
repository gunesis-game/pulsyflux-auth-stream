# Steering Guide — pulsyflux-auth-stream

Read this file first when continuing work in a new session. It records the agreed architecture, constraints, current direction, and unresolved decisions.

## Repositories and dependency

- Auth repository: https://github.com/gunesis-game/pulsyflux-auth-stream
- Generic stream repository: https://github.com/gunesis-game/pulsyflux-stream
- Working branch: `feat/event-owned-auth-subscriptions`
- Draft PR: https://github.com/gunesis-game/pulsyflux-auth-stream/pull/2
- `pulsyflux-stream` owns generic typed stream/event delivery and its store/registry. This repository owns authentication-specific event types and authentication orchestration.
- Inspect the actual pinned dependency version and current `pulsyflux-stream` API before coding. The store was recently added; do not rely on old API assumptions or guess method signatures.

## Product scope

- Bearer-token authentication only.
- Do not add username/password, MFA, password reset, session authentication, or unrelated authentication mechanisms.
- Keep generic stream mechanics in `pulsyflux-stream`; keep Bearer authentication and Vault orchestration in this repository.

## Public API and ownership

The owner's intended public API is:
- `New(ctx context.Context) (*AuthStream, error)`
- `(s *AuthStream) Subscribe() error`

The package should not expose standalone functions such as `SubscribeBearerToken`, generic handler abstractions, callbacks, or public implementation helpers. The `AuthStream` type and the typed event types required by callers are domain API; implementation fields and helper types/functions remain private.

Expected coordinator shape (private fields):
```go
type AuthStream struct {
    ctx context.Context

    // Streams to publish to.
    invTknES    *stream.Stream[InvalidBearerTokenEvent]
    validTknES  *stream.Stream[ValidBearerTokenEvent]
    vltTknReqES *stream.Stream[VaultTokenRequestedEvent]

    // Streams to subscribe to.
    tknRcvES       *stream.Stream[BearerTokenReceiveEvent]
    tknNotRcvES    *stream.Stream[BearerTokenNotReceiveEvent]
    vltTknNotFndES *stream.Stream[VaultTokenNotFoundEvent]
    vltTknFndES    *stream.Stream[VaultTokenFoundEvent]
}
```

The fields above are acquired from the `pulsyflux-stream` store using typed `stream.Get[T]()` calls, not initialized inside the struct declaration. `New(ctx)` should validate context and retrieve/validate all required registered streams, then retain them on `AuthStream`. Do not create replacement streams in the auth package if the shared store is expected to own them.

## Typed event streams and intended flow

All communication is through typed domain events. Do not publish untyped strings or boolean decisions.

Publish streams:
- `*stream.Stream[InvalidBearerTokenEvent]` — publish when authentication is rejected/invalid.
- `*stream.Stream[ValidBearerTokenEvent]` — publish when authentication succeeds/is accepted.
- `*stream.Stream[VaultTokenRequestedEvent]` — publish a request to look up the corresponding token/credential in the Vault component.

Subscribe streams:
- `*stream.Stream[BearerTokenReceiveEvent]` — input indicating a Bearer token was received.
- `*stream.Stream[BearerTokenNotReceiveEvent]` — input indicating no Bearer token was received.
- `*stream.Stream[VaultTokenNotFoundEvent]` — Vault lookup response indicating no matching token was found.
- `*stream.Stream[VaultTokenFoundEvent]` — Vault lookup response indicating a matching token was found.

Intended orchestration:
1. Subscribe to the Bearer-token receive and not-received streams.
2. On a received token, publish a typed `VaultTokenRequestedEvent` to request the relevant Vault lookup/verification.
3. Subscribe to the Vault found and not-found response streams.
4. On a matching Vault response, publish the corresponding typed valid or invalid Bearer-token event.
5. Handle the no-token-received event according to the event contract; do not invent an authentication result or Vault lookup when no token exists.

Important: the exact event payload fields, Vault lookup semantics, and correlation mechanism are not yet confirmed. In particular, concurrent requests need a reliable correlation identifier shared by request and response events. Do not implement concurrency-sensitive matching by assuming that the next found/not-found event belongs to the most recently received token. Ask for clarification if the event definitions or correlation contract are not discoverable in the repository.

## Subscribe implementation rules

- `Subscribe()` owns the subscriptions and event-processing loops for this coordinator.
- Retrieve/store stream pointers in `New`; call each stream's actual `Subscribe(ctx)` API in `Subscribe()`.
- Consume subscription channels with ordinary Go channel reads and `for/select` loops.
- Publish typed events through the typed stream's actual `Publish` API.
- Respect `s.ctx.Done()`, closed channels, subscription cleanup, and errors from subscribe/publish operations.
- Do not introduce callbacks, anonymous function handlers, function-valued handlers, generic dispatchers, central type switches, or separate `Handle...` handler layers.
- Do not close store-owned streams from `AuthStream`; close only subscriptions/resources owned by the instance.
- Avoid leaking goroutines or leaving subscriptions running after cancellation/error.

## Vault and token security

- Vault is a separate event-driven component in the architecture. `AuthStream` requests lookup via `VaultTokenRequestedEvent` and consumes typed `VaultTokenFoundEvent` / `VaultTokenNotFoundEvent` responses.
- Do not assume HashiCorp Vault API details or add a direct Vault SDK/network client unless the owner explicitly asks. The current design is typed stream communication with the Vault component.
- Never log raw Bearer tokens, publish credentials in decision events, or include secrets in errors.
- Do not claim a token is valid merely because it was received or has a JWT-like shape. The Vault/authentication contract must establish what constitutes successful verification.
- Preserve least exposure of token material. Event payloads should carry only fields required by the relevant operation.

## Current state and important correction for future sessions

The repository previously had an earlier implementation using `Stream[string]`, HTTPS introspection, and `Stream[bool]`. That design is superseded by the owner's latest typed, store-backed `AuthStream` architecture above.

Do not continue or restore the old `SubscribeBearerToken` API, raw string input stream, or boolean decision stream. First inspect the current branch files and reconcile them with the architecture in this guide. The latest requested work is to implement the `AuthStream` coordinator and its Vault event flow.

The latest owner-provided intended stream names/types are:
- Outputs: `InvalidBearerTokenEvent`, `ValidBearerTokenEvent`, `VaultTokenRequestedEvent`.
- Inputs: `BearerTokenReceiveEvent`, `BearerTokenNotReceiveEvent`, `VaultTokenNotFoundEvent`, `VaultTokenFoundEvent`.

Do not assume these event types or their payloads are already correctly defined. Inspect the repository and define/fix them as typed authentication-domain events where needed.

## Required implementation and verification steps

1. Inspect current branch, PR, files, and recent commits before editing.
2. Inspect the pinned `pulsyflux-stream` dependency and verify store registration, `Get[T]()`, `Subscribe(ctx)`, subscription channel, event envelope, and `Publish` signatures.
3. Inspect all existing event definitions and their payloads. Confirm Vault request/response correlation before implementing concurrent processing.
4. Implement `AuthStream`, `New(ctx)`, and `(*AuthStream).Subscribe()` with the exact typed stream responsibilities above.
5. Ensure the intended public API is limited to `AuthStream`, `New`, `(*AuthStream).Subscribe`, and event types required for typed inter-package communication. Keep helpers private.
6. Add tests for stream registration/missing streams, received and absent token events, Vault found/not-found responses, correlation, cancellation, closed streams, publish/subscribe errors, and cleanup.
7. Run `gofmt`, `go test -race ./...`, `go vet ./...`, and `go build ./...`.
8. Verify exported symbols and package docs. Do not claim tests pass unless they have actually run successfully.
9. Update README and this steering guide when architecture changes.
10. Do not mark the PR ready or merge it unless explicitly requested.

## Collaboration preferences

- Follow the owner's Go design and corrections literally.
- The owner wants typed authentication events on both input and output; no raw string or boolean event streams.
- The owner wants store-backed streams obtained using typed `stream.Get[T]()`.
- The owner wants only the `New` constructor and `AuthStream.Subscribe` operation as the principal public API, with event types exposed as needed for typed integration.
- No callbacks, anonymous callback handlers, generic handler layers, or JavaScript-style abstractions.
- Ask one concise clarification only when a material contract (especially event payload or request/response correlation) cannot be determined from repository code or this guide.
- Keep changes focused; do not modify `pulsyflux-stream` unless explicitly requested.
- Never claim code was tested, committed, pushed, or merged unless the action succeeded and was verified.

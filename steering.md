# Steering — pulsyflux-auth-stream

Read this file first when continuing work. It is the hand-off record for the next session. It describes the verified repository state, agreed conventions, integration target, and the next concrete work. Do not assume work described as planned has already been implemented.

## Repositories

- Auth event package: https://github.com/gunesis-game/pulsyflux-auth-stream
- Generic typed stream: https://github.com/gunesis-game/pulsyflux-stream
- Token vault: https://github.com/gunesis-game/pulsyflux-vault-stream

All current work should be made directly on the Auth repository's `main` branch unless explicitly instructed otherwise.

## Verified Auth repository state (2026-11-20)

The current `main` branch contains:

- `authstream.go`: generic typed authentication event stream wrapper.
- `authstream_test.go`: publish/subscribe, concrete event stream, slow-subscriber, context-cancellation, and close tests.
- `README.md`: package usage, event list, payload/security guidance, semantics, development and release instructions.
- `.github/workflows/ci.yml`: Go 1.27.x CI; `go mod tidy`, `go test -race ./...`, `go vet ./...`, `go build ./...`.
- `.github/workflows/release.yml`: semantic version tag/manual dispatch release workflow.
- `go.mod`: module `github.com/gunesis-game/pulsyflux-auth-stream`, Go 1.27.0, currently requires only `github.com/gunesis-game/pulsyflux-stream v0.0.0-20261120181413-6c915bdd85a0`.

There is currently **no `steering.md` on Auth main**; this file is being added as the authoritative hand-off.

The package currently defines a closed `AuthEvent` interface and these event types (each embeds `Payload`):

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

`Payload` carries non-secret metadata: `UserID`, `SessionID`, `Provider`, `RequestID`, and `Reason`.

The public API currently includes `New[T AuthEvent](bufferSize int)`, `(*Stream[T]).Subscribe(ctx)`, `(*Stream[T]).Publish(event)`, and `(*Stream[T]).Close()`. The wrapper uses `pulsyflux-stream` internally and exposes timestamped `Event[T]` envelopes.

## Important design constraint to resolve before coding

Auth currently owns an isolated stream per `New[T]` call. Vault currently exposes five streams registered against a shared `*stream.EventChannel`. These are different construction/ownership models.

Also, Auth's `AuthEvent` interface is intentionally closed by its unexported `authEvent()` method. The vault event types do not implement that interface and should not be forced into the authentication-event union: vault domain events are not authentication lifecycle events.

Before implementing the integration, inspect the actual current APIs in `pulsyflux-stream` and `pulsyflux-vault-stream`, including event channel registration, event envelope shape, stream creation, subscription, publish, and close semantics. Then choose a clean integration design that:
- preserves the existing Auth event stream API unless a necessary, explicitly documented extension is approved;
- keeps Auth lifecycle events and Vault request/response events as separate typed domains;
- uses one shared in-process `EventChannel` for components that must communicate;
- avoids adapting events through `any`, string event names, reflection, or a central type switch;
- avoids duplicating stream transport logic in Auth;
- makes ownership and shutdown of shared channels/streams explicit.

Do not guess constructor or method signatures from previous messages. Read the live dependency source first.

## Vault package contract (verified on vault main)

The Vault module path is `github.com/gunesis-game/pulsyflux-vault-stream`. Its current `go.mod` requires Go 1.27.0, `github.com/google/uuid v1.6.0`, and `pulsyflux-stream v0.0.0-20261120210309-11cce17ce0d6`.

Its event payload types are explicitly named with an `Event` suffix:

- `VaultTokenStoreRequestedEvent`: `RequestID uuid.UUID`, `Key string`, `Token string`.
- `VaultTokenStoredEvent`: `RequestID uuid.UUID`, `Key string`.
- `VaultTokenRequestedEvent`: `RequestID uuid.UUID`, `Key string`.
- `VaultTokenFoundEvent`: `RequestID uuid.UUID`, `Key string`, `Token string`.
- `VaultTokenNotFoundEvent`: `RequestID uuid.UUID`, `Key string`.

The three auth integration events are exactly `VaultTokenRequestedEvent`, `VaultTokenFoundEvent`, and `VaultTokenNotFoundEvent`. Correlate concurrent lookups by `RequestID`; never correlate by arrival order. The vault is currently in-memory only and is not a persistent or production secrets manager.

## Mandatory conventions

- Every type used as a generic stream payload must be a properly named domain event type ending in `Event`. Do not introduce generic payload structs such as `Request`, `Result`, `Found`, or `NotFound` as stream payload types.
- Keep Auth lifecycle event types and Vault domain event types distinct.
- Use typed events and explicit request correlation; do not use string-based event dispatch.
- Never put passwords, access/refresh tokens, one-time codes, private keys, or other credentials into Auth `Payload`. The vault's found event is the explicit token-bearing response; do not log it or copy secrets into ordinary Auth events.
- Keep the package small. Avoid callbacks, handler registries, reflection, generic dispatch layers, and unrelated abstractions unless explicitly requested.
- Preserve Go formatting, documentation, and idiomatic error handling.
- Do not claim builds/tests/workflows passed unless they have actually been run and observed passing.

## Next session — concrete tasks

1. Start by reading this file, then inspect Auth `main` and the current live versions of the three repositories. Confirm the current branch and clean working tree before editing.
2. Inspect `pulsyflux-stream` source and the vault package source/API. Resolve the integration design constraint above before changing Auth.
3. Add the vault module dependency to Auth using the actual available Go module version. Do not guess a pseudo-version. Run `go get github.com/gunesis-game/pulsyflux-vault-stream@<verified-version>` (or the equivalent version resolution) and inspect the resulting `go.mod`/ `go.sum`.
4. Implement Auth's use of the vault lookup contract over the shared event channel. Subscribe to both found and not-found responses before publishing lookup requests. Use UUID request IDs consistently; verify the request/response fields and type identity match the vault package exactly.
5. Add integration tests for successful token lookup, missing token, multiple concurrent request IDs, cancellation/shutdown, and ensuring no secret leaks into Auth lifecycle event payloads. Use the actual stream API and avoid flaky sleeps; use bounded contexts/timeouts.
6. Update README with the integration API and ownership/lifecycle requirements. Update this steering file after implementation to record the final design and remaining limitations.
7. Run `gofmt -w` on changed Go files, `go mod tidy`, `go test -race ./...`, `go vet ./...`, and `go build ./...`. Report exact outcomes. Check CI workflow results if available.
8. Commit all intended changes to Auth `main`. Do not modify the Vault or generic stream repositories unless explicitly requested.

## Current status / hand-off

- Auth is a generic typed authentication-event stream, not yet integrated with Vault.
- Vault is now available as a Go module, and its three auth lookup event types are defined and committed on Vault `main`.
- The previous conversation ended before Auth integration was implemented. The assistant had only inspected Auth and Vault source and identified the construction-model mismatch described above.
- No Auth integration code, dependency update, or Auth integration tests should be assumed to exist yet.
- The immediate next action is design and implement the shared-channel bridge while preserving the closed `AuthEvent` union and existing API.

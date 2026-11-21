# AI Agent Steering — pulsyflux-auth-stream

Use this file as the project guidance for AI coding agents and automated contributors. Read it before planning or changing code. Keep changes focused, work directly on `main`, and preserve the design contract below unless the maintainer explicitly asks to change it.

## Purpose and ownership

Auth validates bearer tokens against the token held by `pulsyflux-vault-stream`.

- Upstream components report whether they received a bearer token; they do not validate it.
- Auth owns its input event definitions and credential-free validation outcomes.
- Vault owns the Vault lookup/request/response event types.
- Auth compares the presented token with the token returned by Vault.

## Event contract

Upstream publishes Auth-defined input events:

- `BearerTokenReceiveEvent{Token string}`
- `BearerTokenNotReceiveEvent{}`

Auth publishes only credential-free outcomes:

- `ValidBearerTokenEvent{}`
- `InvalidBearerTokenEvent{}`

Auth requests Vault using `VaultTokenRequestedEvent` and consumes `VaultTokenFoundEvent` / `VaultTokenNotFoundEvent`. These event types remain owned by the Vault package.

Use the shared `*stream.EventChannel` as the workflow boundary. Separate workflows must use separate channels. Do not introduce per-request IDs: the presented token is the Vault lookup key and pending lookup state handles concurrent/repeated requests for the same key. Keep one Auth validation consumer per workflow channel.

## Lifecycle and boundaries

- Upstream input streams and Vault must be registered/initialized on the shared channel before `authstream.New`.
- Auth subscribes to Vault responses before upstream inputs.
- Consumers retrieve Auth output streams with `stream.Get[T](channel)` and subscribe before publishing inputs.
- Auth owns only its subscriptions, workers, and output-stream registration. `Auth.Close()` must not close the shared channel or upstream/Vault streams.
- The application closes the EventChannel after its workflow components stop.

## Security and implementation rules

- Never log, trace, or expose bearer tokens or event payloads containing credentials.
- Auth outcome events must remain credential-free.
- Do not add unrelated event types (lifecycle, MFA, password reset), callbacks, reflection dispatch, or string-routed events.
- Keep typed events named with the `Event` suffix.
- Worker publish errors are reported through `Auth.Errors()`.
- Keep dependencies aligned with the current `pulsyflux-vault-stream` and `pulsyflux-stream` APIs; avoid stale tagged versions when upstream API changes require newer revisions.
- Maintain idiomatic Go, focused changes, and clear tests. Update README and this steering file when public API, setup, ownership, or architectural guidance changes.

## Verification

Before declaring work complete, run or verify CI for:

```sh
gofmt -w .
go mod tidy
go test -race ./...
go vet ./...
go build ./...
```

Tests should cover valid/mismatching tokens, Vault not-found, no-token input, repeated/concurrent lookups, channel isolation, and shutdown/cancellation. Do not claim checks passed unless a real run confirms it.

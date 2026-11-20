# Steering Guide — pulsyflux-auth-stream

This file carries project context and agreed engineering constraints into future sessions. Read it before changing code.

## Repository and relationship to other projects

- Repository: https://github.com/gunesis-game/pulsyflux-auth-stream
- Underlying transport: https://github.com/gunesis-game/pulsyflux-stream
- pulsyflux-auth-stream is the authentication implementation built on pulsyflux-stream. It is not a generic extension or fork of the generic stream package.
- Keep generic event delivery, buffering, subscriptions, and concurrency primitives in pulsyflux-stream. Keep authentication-specific behavior in this repository.

## Product scope: Bearer token only

This package supports **Bearer-token authentication only**.

Do not add or retain unrelated authentication mechanisms or workflows, including username/password authentication, MFA, password reset, session-authentication workflows, or non-Bearer authentication schemes. Do not silently expand scope.

## Public API and package boundaries

- The package should expose only its subscription functions, implemented in subscriptions.go.
- Do not expose internal types, stream wrappers, payloads, helpers, implementation structs, or details from authstream.go.
- Keep authstream.go and other implementation files private in the Go API sense: unexported identifiers only.
- Keep exported functions in subscriptions.go; avoid adding exported constants, variables, types, constructors, or helper functions elsewhere.
- Do not create a public generic handler abstraction.
- Do not introduce callbacks, anonymous functions, function-valued handlers, or JavaScript-style callback patterns.
- Use ordinary Go functions, explicit Subscribe() calls, contexts, and channel-based consumption.

## Subscription design agreed in the conversation

The original design principle remains important:

- Each authentication event/operation that needs to consume an event should have an explicit Subscribe() function or method in its function body.
- The subscription itself owns the relevant authentication behavior: consume its intended input, perform the Bearer-token-specific check, and publish the corresponding decision event.
- Use Go channel reads and for/select loops, not callbacks.
- Do not route all event types through one generic handler or a central type switch.
- Do not add separate Handle... functions as a handler layer.

The later scope decision narrowed the package to Bearer-token authentication only. When refining the exact subscription API, preserve the event-/operation-owned subscription principle where applicable while keeping the public surface limited to subscription functions in subscriptions.go.

## Bearer-token validation

- The current implementation uses an HTTPS RFC 7662-compatible token introspection endpoint. The intention is to support both JWT-formatted and opaque Bearer tokens through the trusted introspection service, without baking a token format into this package.
- The introspection service is responsible for deciding whether a token is active.
- Never log, publish, or expose raw Bearer tokens or other credential material.
- Publish only the authentication decision/result, not the token itself.
- Require HTTPS for remote introspection. Do not weaken this to plain HTTP.
- The supplied HTTP client must use trusted TLS configuration and whatever client authentication the introspection endpoint requires.
- Do not invent authentication policy or claim a token is valid based only on parsing its shape.
- Keep error messages free of token contents and other secrets.

## Current implementation snapshot

Work was made on branch: feat/event-owned-auth-subscriptions

Open draft pull request: https://github.com/gunesis-game/pulsyflux-auth-stream/pull/2

The branch currently contains a Bearer-only refactor:
- authstream.go was reduced to a package declaration/comment, with implementation details removed from its exported surface.
- subscriptions.go currently defines the exported SubscribeBearerToken function and private HTTPS introspection helpers.
- SubscribeBearerToken currently consumes Bearer token strings from a Go receive-only channel, introspects them, and publishes a boolean active/inactive result to pulsyflux-stream.Stream[bool].
- subscriptions_test.go contains tests for active decisions, blank tokens, cancellation, and rejecting non-HTTPS URLs.
- README.md documents the Bearer-only scope and narrow public API.

This snapshot is a work in progress, not a declaration that the design is fully finished. In particular, verify that the current channel input and boolean decision output correctly satisfy the intended event-specific Subscribe() architecture and the real pulsyflux-stream API. Do not preserve an incorrect API merely because it is already committed to the branch.

## Required next steps

1. Read this file and inspect the current repository branch and PR before making changes.
2. Inspect the actual public API of the current pulsyflux-stream dependency. Do not guess names, generic constraints, event envelope types, or method signatures.
3. Reconcile subscriptions.go with the agreed explicit, channel-based, event-/operation-owned Subscribe() design.
4. Ensure the only exported package symbols are the intended subscription functions. Check with Go tooling, including go doc and an external-package compile test.
5. Review all files and remove remnants of username/password, MFA, password-reset, session, and other non-Bearer workflows.
6. Verify HTTPS enforcement, context cancellation, closed-channel behavior, introspection response handling, and decision publishing.
7. Run gofmt, go test -race ./..., go vet ./..., and go build ./....
8. Fix compile/test/API issues before marking the PR ready. Do not merge unless explicitly asked.

## Collaboration rules

- Follow the owner's corrections literally. They have repeatedly emphasized: Go, not JavaScript; no callbacks; no anonymous functions; no generic handler layer; explicit Subscribe() logic in the relevant function body.
- Ask one concise clarification only when a material design decision cannot be determined from the repository or this guide. Do not repeatedly ask whether the package should support JWT versus opaque tokens: the current intended approach is introspection, which can support either.
- Do not claim code was tested, built, committed, pushed, or merged unless the corresponding action actually succeeded and the result was verified.
- Keep changes focused. Do not refactor pulsyflux-stream as part of this repository's work unless explicitly requested.

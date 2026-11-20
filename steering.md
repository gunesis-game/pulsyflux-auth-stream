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

The later scope decision narrowed the package to Bearer-token authentication only. Preserve the event-/operation-owned subscription principle while keeping the public surface limited to subscription functions in subscriptions.go.

## Bearer-token validation

- Use an HTTPS RFC 7662-compatible token introspection endpoint. This supports both JWT-formatted and opaque Bearer tokens through the trusted introspection service, without baking a token format into this package.
- The introspection service is responsible for deciding whether a token is active.
- Never log, publish, or expose raw Bearer tokens or other credential material.
- Publish only the authentication decision/result, not the token itself.
- Require HTTPS for remote introspection. Do not weaken this to plain HTTP.
- The supplied HTTP client must use trusted TLS configuration and whatever client authentication the introspection endpoint requires.
- Do not invent authentication policy or claim a token is valid based only on parsing its shape.
- Keep error messages free of token contents and other secrets.

## Current implementation snapshot

Work is on branch: feat/event-owned-auth-subscriptions

Open draft pull request: https://github.com/gunesis-game/pulsyflux-auth-stream/pull/2

The branch contains a Bearer-only implementation:
- authstream.go contains only the package declaration/comment.
- subscriptions.go exposes SubscribeBearerToken and keeps the introspection response and helper private.
- SubscribeBearerToken accepts a *pulsyflux-stream.Stream[string], explicitly calls Subscribe(ctx), consumes its Events() channel, introspects each token, and publishes only stream.Event[bool] decisions to *pulsyflux-stream.Stream[bool].
- Blank tokens produce an inactive decision without being sent to the introspection endpoint.
- The input stream is not closed by the subscription. Context cancellation returns the context error; closing the input stream ends the subscription.
- The introspection response must contain an active boolean. Non-200 responses and malformed/missing active values return errors without exposing token contents.
- subscriptions_test.go covers active decisions, blank tokens, cancellation, input-stream closure, malformed introspection responses, and HTTPS enforcement.
- The obsolete authstream_test.go for the removed generic event API has been deleted.
- README.md documents the stream-based API and Bearer-only scope.

The public API and dependency API were reconciled against the actual pulsyflux-stream dependency version in go.mod. Its Stream[T], Event[T], Subscribe(ctx), Subscription.Events(), and Publish(Event[T]) APIs are the ones used here.

## Required next steps

1. Inspect the current repository branch and PR before making changes.
2. Re-check the actual public API of the pulsyflux-stream dependency if its pinned version changes. Do not guess names, generic constraints, event envelope types, or method signatures.
3. Keep the explicit, channel-based, event-/operation-owned Subscribe() design. Do not replace it with callbacks or a generic handler.
4. Ensure the only exported package symbols are the intended subscription functions. Check with Go tooling, including go doc and an external-package compile test.
5. Review all files and remove remnants of username/password, MFA, password-reset, session, and other non-Bearer workflows.
6. Verify HTTPS enforcement, context cancellation, closed-stream behavior, introspection response handling, and decision publishing.
7. Run gofmt, go test -race ./..., go vet ./..., and go build ./....
8. Fix compile/test/API issues before marking the PR ready. Do not merge unless explicitly asked.

## Collaboration rules

- Follow the owner's corrections literally. They have repeatedly emphasized: Go, not JavaScript; no callbacks; no anonymous functions; no generic handler layer; explicit Subscribe() logic in the relevant function body.
- Ask one concise clarification only when a material design decision cannot be determined from the repository or this guide. Do not repeatedly ask whether the package should support JWT versus opaque tokens: the intended approach is introspection, which can support either.
- Do not claim code was tested, built, committed, pushed, or merged unless the corresponding action actually succeeded and the result was verified.
- Keep changes focused. Do not refactor pulsyflux-stream as part of this repository's work unless explicitly requested.

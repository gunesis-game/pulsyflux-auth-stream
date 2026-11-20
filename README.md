# pulsyflux-auth-stream

Bearer-token authentication subscriptions built on
[pulsyflux-stream](https://github.com/gunesis-game/pulsyflux-stream).

The package intentionally supports **Bearer tokens only**. Its public API
consists only of the subscription functions in `subscriptions.go`; stream
and authentication implementation details remain private.

## Bearer token validation

`SubscribeBearerToken` consumes tokens from a read-only Go channel, validates
each token through an RFC 7662-compatible introspection endpoint, and
publishes the endpoint's `active` result to a
`pulsyflux-stream.Stream[bool]`.

```go
tokens := make(chan string)
decisions := stream.New[bool](16)
defer decisions.Close()

go func() {
    err := authstream.SubscribeBearerToken(
        ctx,
        tokens,
        "https://identity.example.com/oauth2/introspect",
        httpClient,
        decisions,
    )
    if err != nil && !errors.Is(err, context.Canceled) {
        log.Printf("Bearer subscription stopped: %v", err)
    }
}()
```

Configure the supplied HTTP client with the introspection endpoint's required
client authentication (for example, HTTP Basic authentication) and trusted TLS
configuration. This package does not assume whether the token is a JWT or an
opaque token: the trusted introspection endpoint is responsible for validating
the token and returning its active state.

## API boundaries

- The only exported package function is `SubscribeBearerToken`.
- No authentication event structs, stream wrappers, payloads, or internal
  helpers are exported.
- No callbacks, anonymous functions, or handler interfaces are used.
- Bearer tokens are sent only to the configured introspection endpoint and
  are never published to the decision stream.
- The decision stream publishes `true` for active tokens and `false` for
  inactive tokens.

## Requirements

Go 1.27 or newer.

## Development

```sh
go test -race ./...
go vet ./...
go build ./...
```

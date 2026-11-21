# pulsyflux-auth-stream

Validate bearer tokens in a typed, event-driven Go workflow. This package connects your upstream token-receiving component to [pulsyflux-vault-stream](https://github.com/gunesis-game/pulsyflux-vault-stream), using [pulsyflux-stream](https://github.com/gunesis-game/pulsyflux-stream) as the shared event transport.

**Auth makes the decision.** Upstream only reports whether a bearer token was received. Auth asks Vault for the stored token and compares it with the presented value.

## How it works

1. An upstream component publishes either a token-received event or a no-token event.
2. Auth looks up the presented token in Vault (the token is the lookup key).
3. Auth publishes a valid or invalid result.

| Auth outcome | Meaning |
| --- | --- |
| `ValidBearerTokenEvent` | Vault returned a token matching the presented token. |
| `InvalidBearerTokenEvent` | No token was received, no Vault entry was found, or the token did not match. |

Outcome events contain no credentials.

## Install

```sh
go get github.com/gunesis-game/pulsyflux-auth-stream
```

## Integration

Create one shared EventChannel per workflow. Register the upstream input streams and initialize Vault on that channel before creating Auth. Subscribe to Auth outcomes before publishing token input events.

```go
channel := stream.NewEventChannel()
defer channel.Close()

// Register the Auth input streams on this channel first.
bearerReceived, err := stream.New[authstream.BearerTokenReceiveEvent](channel, 32)
if err != nil {
    return err
}
bearerNotReceived, err := stream.New[authstream.BearerTokenNotReceiveEvent](channel, 32)
if err != nil {
    return err
}

// Vault must use this same channel and be initialized before Auth.
vault, err := vaultstream.New(ctx, channel)
if err != nil {
    return err
}
defer vault.Close()

auth, err := authstream.New(ctx, channel)
if err != nil {
    return err
}
defer auth.Close()

// Subscribe to Auth outcomes before publishing input events.
validStream := stream.Get[authstream.ValidBearerTokenEvent](channel)
invalidStream := stream.Get[authstream.InvalidBearerTokenEvent](channel)
if validStream == nil || invalidStream == nil {
    return errors.New("Auth output streams not registered")
}
valid, err := validStream.Subscribe(ctx)
if err != nil {
    return err
}
defer valid.Close()
invalid, err := invalidStream.Subscribe(ctx)
if err != nil {
    return err
}
defer invalid.Close()

// Publish a received token using the current Event API.
event, err := stream.NewEvent[authstream.BearerTokenReceiveEvent](stream.NewRedactionPolicy())
if err != nil {
    return err
}
if err := event.SetData(authstream.BearerTokenReceiveEvent{Token: presentedToken}); err != nil {
    return err
}
if err := bearerReceived.Publish(event); err != nil {
    return err
}

// Read valid.Events() and invalid.Events() in your consumer logic.
_ = bearerNotReceived
_ = valid
_ = invalid
```

This example illustrates wiring; add your application's consumer loops, error handling, and token storage initialization. Vault must contain the presented token under the same key for the comparison to succeed.

## API and behavior

- **Inputs:** `BearerTokenReceiveEvent{Token: ...}`, `BearerTokenNotReceiveEvent{}`
- **Outputs:** `ValidBearerTokenEvent{}`, `InvalidBearerTokenEvent{}`
- **Errors:** `auth.Errors()` provides best-effort asynchronous publish errors.
- **Shutdown:** `auth.Close()` stops Auth's subscriptions and workers. It does not close the shared channel or other components' streams. The application owns channel shutdown.

Each workflow must use its own EventChannel. Sharing a channel is an in-process workflow boundary, not cross-process transport.

## Security notes

- Do not log bearer tokens or token-bearing event payloads.
- Auth's outcome events intentionally contain no credentials.
- The in-memory Vault is not persistent storage or a production secrets manager; consult the Vault project's documentation for storage limitations.

## Related packages

- [pulsyflux-stream](https://github.com/gunesis-game/pulsyflux-stream) — event transport and typed streams.
- [pulsyflux-vault-stream](https://github.com/gunesis-game/pulsyflux-vault-stream) — token lookup and storage workflow.

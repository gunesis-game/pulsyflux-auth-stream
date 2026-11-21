# pulsyflux-auth-stream

Auth-owned bearer-token validation events and a typed integration with
[pulsyflux-vault-stream](https://github.com/gunesis-game/pulsyflux-vault-stream)
over [pulsyflux-stream](https://github.com/gunesis-game/pulsyflux-stream).

Auth makes the validation decision. Upstream components only report whether a
bearer token was received; they do not validate it. Auth compares the presented
token with the token returned by Vault.

## Event contract

Auth defines the upstream input event types. Upstream packages import this
module and publish these events on the workflow's shared `EventChannel`:

- `BearerTokenReceiveEvent{Token: "..."}`
- `BearerTokenNotReceiveEvent{}`

Auth publishes two credential-free outcomes:

- `ValidBearerTokenEvent{}` — the Vault-returned token matched.
- `InvalidBearerTokenEvent{}` — no bearer token, no Vault entry, or mismatch.

Auth uses the presented bearer token itself as Vault's `Key` and publishes
`VaultTokenRequestedEvent`. It subscribes to
`VaultTokenFoundEvent` and `VaultTokenNotFoundEvent`. The Vault event
definitions and streams remain owned by the Vault package.

No `RequestID` is used. A shared `EventChannel` is the workflow boundary:
components in one workflow use the same channel, while separate workflows use
different channels. The Vault response `Key` identifies which pending token
lookup it belongs to within that workflow.

## Setup

Register the upstream input streams and initialize Vault on the same channel
before constructing Auth. Retrieve Auth's output streams with stream.Get[T](channel) and subscribe
before publishing any bearer input events.

```go
channel := stream.NewEventChannel()
defer channel.Close()

bearerReceived, err := stream.New[authstream.BearerTokenReceiveEvent](channel, 32)
if err != nil {
    return err
}
bearerNotReceived, err := stream.New[authstream.BearerTokenNotReceiveEvent](channel, 32)
if err != nil {
    return err
}
_ = bearerReceived
_ = bearerNotReceived

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

validEvents := stream.Get[authstream.ValidBearerTokenEvent](channel)
if validEvents == nil {
    return errors.New("Auth valid-token stream was not registered")
}
valid, err := validEvents.Subscribe(ctx)
if err != nil {
    return err
}
defer valid.Close()

invalidEvents := stream.Get[authstream.InvalidBearerTokenEvent](channel)
if invalidEvents == nil {
    return errors.New("Auth invalid-token stream was not registered")
}
invalid, err := invalidEvents.Subscribe(ctx)
if err != nil {
    return err
}
defer invalid.Close()

// Upstream publishes a token on the same channel:
event, err := stream.NewEvent[authstream.BearerTokenReceiveEvent](stream.NewRedactionPolicy())
if err != nil {
    return err
}
if err := event.SetData(authstream.BearerTokenReceiveEvent{Token: presentedToken}); err != nil {
    return err
}
err = bearerReceived.Publish(event)
```

The example omits application-specific error handling and token storage setup.
Vault must already contain the token under the same key for a successful
comparison. Use the Auth output event streams to consume the decision.
Auth.Close stops only Auth's own subscriptions and workers. It does not close
the shared channel or upstream/Vault streams; the application owns channel
shutdown.

## Security and limitations

- Bearer tokens are necessarily present in the upstream input and Vault lookup
  request/response events. Do not log event payloads or expose them in traces.
- Auth outcome events contain no credentials.
- The in-memory Vault is not persistent storage or a production secrets
  manager; see the Vault package documentation for its limitations.
- This is an in-process event transport. Sharing an EventChannel UUID does not
  provide cross-process messaging.

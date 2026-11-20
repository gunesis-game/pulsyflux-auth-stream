// Package authstream provides Bearer-token authentication subscriptions.
package authstream

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/url"
	"strings"

	stream "github.com/gunesis-game/pulsyflux-stream"
)

var (
	errNilBearerDependency    = errors.New("authstream: nil bearer-token subscription dependency")
	errInvalidIntrospectionURL = errors.New("authstream: invalid bearer-token introspection URL")
)

// bearerIntrospectionResponse is the RFC 7662 token introspection response.
type bearerIntrospectionResponse struct {
	Active *bool `json:"active"`
}

// SubscribeBearerToken subscribes directly to the token stream, validates each
// Bearer token using an RFC 7662-compatible HTTPS introspection endpoint, and
// publishes only the active decision. A true value means active; false means
// inactive. The HTTP client must provide the endpoint's required client
// authentication and trusted TLS configuration.
//
// The input stream is not closed by this function. The subscription ends when
// ctx is cancelled or the input stream is closed.
func SubscribeBearerToken(
	ctx context.Context,
	tokens *stream.Stream[string],
	introspectionURL string,
	client *http.Client,
	decisions *stream.Stream[bool],
) error {
	if tokens == nil || client == nil || decisions == nil {
		return errNilBearerDependency
	}
	if ctx == nil {
		ctx = context.Background()
	}
	parsed, err := url.ParseRequestURI(introspectionURL)
	if err != nil || parsed.Scheme != "https" || parsed.Host == "" {
		return errInvalidIntrospectionURL
	}

	tokenSubscription, err := tokens.Subscribe(ctx)
	if err != nil {
		return err
	}
	defer tokenSubscription.Close()

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case event, ok := <-tokenSubscription.Events():
			if !ok {
				return nil
			}
			active, validationErr := introspectBearerToken(ctx, client, introspectionURL, event.Data)
			if validationErr != nil {
				return validationErr
			}
			if err := decisions.Publish(stream.Event[bool]{Data: active}); err != nil {
				return err
			}
		}
	}
}

func introspectBearerToken(ctx context.Context, client *http.Client, endpoint, token string) (bool, error) {
	if strings.TrimSpace(token) == "" {
		return false, nil
	}
	form := url.Values{"token": []string{token}}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, strings.NewReader(form.Encode()))
	if err != nil {
		return false, err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Accept", "application/json")

	resp, err := client.Do(req)
	if err != nil {
		return false, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return false, errors.New("authstream: token introspection endpoint returned non-200 status")
	}
	var result bearerIntrospectionResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return false, err
	}
	if result.Active == nil {
		return false, errors.New("authstream: token introspection response omitted active")
	}
	return *result.Active, nil
}

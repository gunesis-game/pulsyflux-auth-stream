package authstream

import (
	"context"
	"errors"
)

// Authentication request events contain identifiers only. Credential material
// must be obtained from a trusted credential store or identity provider.
type UserAuthenticationRequested struct{ Payload }
type MFAVerificationRequested struct{ Payload }

type UserAuthenticationApproved struct{ Payload }
type UserAuthenticationDenied struct{ Payload }
type MFAVerificationApproved struct{ Payload }
type MFAVerificationDenied struct{ Payload }

func (UserAuthenticationRequested) authEvent() {}
func (MFAVerificationRequested) authEvent() {}
func (UserAuthenticationApproved) authEvent() {}
func (UserAuthenticationDenied) authEvent() {}
func (MFAVerificationApproved) authEvent() {}
func (MFAVerificationDenied) authEvent() {}

// UserAuthenticator performs the application's user-authentication check.
// Implementations may consult a credential store or external identity provider.
type UserAuthenticator interface {
	Authenticate(context.Context, UserAuthenticationRequested) (bool, error)
}

// MFAVerifier performs the application's MFA check.
type MFAVerifier interface {
	VerifyMFA(context.Context, MFAVerificationRequested) (bool, error)
}

var ErrNilAuthenticationDependency = errors.New("authstream: nil subscription dependency")

// Subscribe consumes only UserAuthenticationRequested events and publishes
// one typed decision event for each request. It contains no callback or
// central event-type dispatch.
func (UserAuthenticationRequested) Subscribe(
	ctx context.Context,
	requests *Stream[UserAuthenticationRequested],
	authenticator UserAuthenticator,
	decisions *Stream[AuthEvent],
) error {
	if requests == nil || authenticator == nil || decisions == nil {
		return ErrNilAuthenticationDependency
	}
	sub, err := requests.Subscribe(ctx)
	if err != nil {
		return err
	}
	defer sub.Close()

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case item, ok := <-sub.Events():
			if !ok {
				return nil
			}
			approved, authErr := authenticator.Authenticate(ctx, item.Data)
			payload := item.Data.Payload
			if authErr != nil {
				payload.Reason = "authentication_error"
				if err := decisions.Publish(UserAuthenticationDenied{Payload: payload}); err != nil {
					return err
				}
				continue
			}
			if approved {
				payload.Reason = ""
				err = decisions.Publish(UserAuthenticationApproved{Payload: payload})
			} else {
				payload.Reason = "authentication_denied"
				err = decisions.Publish(UserAuthenticationDenied{Payload: payload})
			}
			if err != nil {
				return err
			}
		}
	}
}

// Subscribe consumes only MFAVerificationRequested events and publishes
// one typed decision event for each request.
func (MFAVerificationRequested) Subscribe(
	ctx context.Context,
	requests *Stream[MFAVerificationRequested],
	verifier MFAVerifier,
	decisions *Stream[AuthEvent],
) error {
	if requests == nil || verifier == nil || decisions == nil {
		return ErrNilAuthenticationDependency
	}
	sub, err := requests.Subscribe(ctx)
	if err != nil {
		return err
	}
	defer sub.Close()

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case item, ok := <-sub.Events():
			if !ok {
				return nil
			}
			verified, verifyErr := verifier.VerifyMFA(ctx, item.Data)
			payload := item.Data.Payload
			if verifyErr != nil {
				payload.Reason = "mfa_verification_error"
				err = decisions.Publish(MFAVerificationDenied{Payload: payload})
			} else if verified {
				payload.Reason = ""
				err = decisions.Publish(MFAVerificationApproved{Payload: payload})
			} else {
				payload.Reason = "mfa_verification_denied"
				err = decisions.Publish(MFAVerificationDenied{Payload: payload})
			}
			if err != nil {
				return err
			}
		}
	}
}

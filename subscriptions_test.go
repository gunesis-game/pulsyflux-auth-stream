package authstream

import (
	"context"
	"testing"
	"time"
)

type testUserAuthenticator bool

func (a testUserAuthenticator) Authenticate(context.Context, UserAuthenticationRequested) (bool, error) {
	return bool(a), nil
}

type testMFAVerifier bool

func (v testMFAVerifier) VerifyMFA(context.Context, MFAVerificationRequested) (bool, error) {
	return bool(v), nil
}

func TestUserAuthenticationSubscribePublishesDecision(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	requests := New[UserAuthenticationRequested](1)
	decisions := New[AuthEvent](1)
	defer requests.Close()
	defer decisions.Close()

	decisionSub, err := decisions.Subscribe(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer decisionSub.Close()

	done := make(chan error, 1)
	go func() {
		done <- (UserAuthenticationRequested{}).Subscribe(ctx, requests, testUserAuthenticator(true), decisions)
	}()

	want := UserAuthenticationRequested{Payload: Payload{UserID: "u-1", RequestID: "r-1"}}
	if err := requests.Publish(want); err != nil {
		t.Fatal(err)
	}

	select {
	case got := <-decisionSub.Events():
		if got.Data != (UserAuthenticationApproved{Payload: want.Payload}) {
			t.Fatalf("got %#v, want approval for %#v", got.Data, want.Payload)
		}
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for authentication decision")
	}

	cancel()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("Subscribe did not stop after context cancellation")
	}
}

func TestMFASubscribePublishesDenial(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	requests := New[MFAVerificationRequested](1)
	decisions := New[AuthEvent](1)
	defer requests.Close()
	defer decisions.Close()

	sub, err := decisions.Subscribe(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer sub.Close()

	done := make(chan error, 1)
	go func() {
		done <- (MFAVerificationRequested{}).Subscribe(ctx, requests, testMFAVerifier(false), decisions)
	}()

	want := MFAVerificationRequested{Payload: Payload{UserID: "u-2", RequestID: "mfa-1"}}
	if err := requests.Publish(want); err != nil {
		t.Fatal(err)
	}

	select {
	case got := <-sub.Events():
		if got.Data != (MFAVerificationDenied{Payload: Payload{
			UserID: "u-2", RequestID: "mfa-1", Reason: "mfa_verification_denied",
		}}) {
			t.Fatalf("got %#v, want MFA denial", got.Data)
		}
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for MFA decision")
	}

	cancel()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("Subscribe did not stop after context cancellation")
	}
}

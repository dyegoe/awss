/*
Copyright © 2026 Dyego Alexandre Eugenio github@dyego.com.br

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

	http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package cleanup

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/sts"
	"github.com/aws/smithy-go"
)

// apiErr returns an AWS API error with code.
func apiErr(code string) error {
	return &smithy.GenericAPIError{Code: code, Message: code + " message"}
}

// fakeSTS is an STSAPI that answers account, or err.
type fakeSTS struct {
	account string
	err     error
}

// GetCallerIdentity returns the account of the fake.
func (f *fakeSTS) GetCallerIdentity(context.Context, *sts.GetCallerIdentityInput, ...func(*sts.Options)) (
	*sts.GetCallerIdentityOutput, error,
) {
	if f.err != nil {
		return nil, f.err
	}
	return &sts.GetCallerIdentityOutput{Account: aws.String(f.account)}, nil
}

// TestCallerAccount checks the account and the wrapped error.
func TestCallerAccount(t *testing.T) {
	got, err := CallerAccount(context.Background(), &fakeSTS{account: "111"})
	if got != "111" || err != nil {
		t.Errorf("CallerAccount() = %q, %v, want 111, nil", got, err)
	}
	boom := errors.New("expired")
	if _, err := CallerAccount(context.Background(), &fakeSTS{err: boom}); !errors.Is(err, boom) {
		t.Errorf("CallerAccount() error = %v, want it to wrap %v", err, boom)
	}
}

// TestErrorCode checks the code of an API error, wrapped or not, and of other errors.
func TestErrorCode(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want string
	}{
		{name: "nil", err: nil},
		{name: "plain", err: errors.New("boom")},
		{name: "api", err: apiErr("Boom"), want: "Boom"},
		{name: "wrapped", err: fmt.Errorf("x: %w", apiErr("Boom")), want: "Boom"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := ErrorCode(tt.err); got != tt.want {
				t.Errorf("ErrorCode(%v) = %q, want %q", tt.err, got, tt.want)
			}
		})
	}
}

// TestIsNotFound checks the error codes that mean a resource is already gone.
func TestIsNotFound(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want bool
	}{
		{name: "nil", err: nil},
		{name: "plain", err: errors.New("boom")},
		{name: "not found", err: apiErr("InvalidSubnetID.NotFound"), want: true},
		{name: "nat not found", err: apiErr("NatGatewayNotFound"), want: true},
		{name: "not attached", err: apiErr("Gateway.NotAttached"), want: true},
		{name: "dependency", err: apiErr("DependencyViolation")},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := IsNotFound(tt.err); got != tt.want {
				t.Errorf("IsNotFound(%v) = %v, want %v", tt.err, got, tt.want)
			}
		})
	}
}

// TestIgnoreNotFound checks that only the errors of a resource already gone are dropped.
func TestIgnoreNotFound(t *testing.T) {
	if err := IgnoreNotFound(apiErr("InvalidGroup.NotFound")); err != nil {
		t.Errorf("IgnoreNotFound(NotFound) = %v, want nil", err)
	}
	if err := IgnoreNotFound(nil); err != nil {
		t.Errorf("IgnoreNotFound(nil) = %v, want nil", err)
	}
	boom := apiErr("DependencyViolation")
	if err := IgnoreNotFound(boom); !errors.Is(err, boom) {
		t.Errorf("IgnoreNotFound(DependencyViolation) = %v, want it", err)
	}
}

// TestDryRunAllowed checks the three answers of a DryRun call.
func TestDryRunAllowed(t *testing.T) {
	tests := []struct {
		name        string
		err         error
		wantAllowed bool
		wantErr     bool
	}{
		{name: "allowed", err: apiErr("DryRunOperation"), wantAllowed: true},
		{name: "denied", err: apiErr("UnauthorizedOperation")},
		{name: "other", err: apiErr("RequestLimitExceeded"), wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			allowed, err := DryRunAllowed(tt.err)
			if allowed != tt.wantAllowed || (err != nil) != tt.wantErr {
				t.Errorf("DryRunAllowed(%v) = %v, %v, want %v, error %v", tt.err, allowed, err, tt.wantAllowed, tt.wantErr)
			}
		})
	}
}

// TestConciseError checks that the SDK operation error is cut down to the AWS code and message,
// and that other errors are kept as they are.
func TestConciseError(t *testing.T) {
	sdkErr := &smithy.OperationError{
		ServiceID: "EC2", OperationName: "ReleaseAddress", Err: apiErr("InvalidIPAddress.InUse"),
	}
	tests := []struct {
		name string
		err  error
		want string
	}{
		{name: "sdk error", err: fmt.Errorf("eipalloc-1: %w", sdkErr),
			want: "eipalloc-1: InvalidIPAddress.InUse: InvalidIPAddress.InUse message"},
		{name: "plain error", err: errors.New("boom"), want: "boom"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := ConciseError(tt.err); got != tt.want {
				t.Errorf("ConciseError(%v) = %q, want %q", tt.err, got, tt.want)
			}
		})
	}
}

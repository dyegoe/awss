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

// Package cleanup holds what the cleanup commands share: the statuses of a row, the account of
// the profile, and the reading of AWS errors. Each cleanup lives in its own subpackage
// (cleanup/defaultvpc, cleanup/dhcpoptions).
package cleanup

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/sts"
	"github.com/aws/smithy-go"
)

// The statuses every cleanup uses. A cleanup may add its own, such as blocked.
const (
	// StatusDeletable is a preview with nothing in the way: --apply deletes what the row lists.
	StatusDeletable = "deletable"

	// StatusDenied is a preview where a DryRun says the role may not delete.
	StatusDenied = "denied"

	// StatusDeleted is a resource that --apply deleted.
	StatusDeleted = "deleted"

	// StatusFailed is a deletion that failed; the details give the error. A new run resumes it.
	StatusFailed = "failed"
)

// STSAPI is the part of the STS client a cleanup uses, to learn the account of the profile.
type STSAPI interface {
	GetCallerIdentity(context.Context, *sts.GetCallerIdentityInput, ...func(*sts.Options)) (
		*sts.GetCallerIdentityOutput, error)
}

// CallerAccount returns the account ID of the profile.
func CallerAccount(ctx context.Context, s STSAPI) (string, error) {
	out, err := s.GetCallerIdentity(ctx, &sts.GetCallerIdentityInput{})
	if err != nil {
		return "", fmt.Errorf("error getting the account of the profile: %w", err)
	}
	return aws.ToString(out.Account), nil
}

// ErrorCode returns the AWS error code of err, or "" when err is nil or not an AWS API error.
func ErrorCode(err error) string {
	var apiErr smithy.APIError
	if errors.As(err, &apiErr) {
		return apiErr.ErrorCode()
	}
	return ""
}

// IsNotFound reports whether err says the resource is already gone, or already detached: its
// deletion is done.
func IsNotFound(err error) bool {
	code := ErrorCode(err)
	return strings.HasSuffix(code, "NotFound") || code == "Gateway.NotAttached"
}

// IgnoreNotFound returns err, or nil when it says the resource is already gone.
func IgnoreNotFound(err error) error {
	if err == nil || IsNotFound(err) {
		return nil
	}
	return err
}

// DryRunAllowed reads the error of a call made with DryRun: true when the role may make the call,
// false when it may not, and the error when AWS answered something else.
func DryRunAllowed(err error) (bool, error) {
	switch ErrorCode(err) {
	case "DryRunOperation":
		return true, nil
	case "UnauthorizedOperation":
		return false, nil
	default:
		return false, err
	}
}

// ConciseError returns the message of err with the SDK's operation error, which names the
// operation, the HTTP status and the request ID before the AWS error, cut down to "Code: message":
// the table has room for one line per detail.
func ConciseError(err error) string {
	msg := err.Error()
	var opErr *smithy.OperationError
	var apiErr smithy.APIError
	if errors.As(err, &opErr) && errors.As(err, &apiErr) {
		msg = strings.Replace(msg, opErr.Error(), apiErr.ErrorCode()+": "+apiErr.ErrorMessage(), 1)
	}
	return msg
}

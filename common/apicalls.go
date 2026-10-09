/*
Copyright © 2022 Dyego Alexandre Eugenio github@dyego.com.br

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

package common

import (
	"context"
	"fmt"
	"sync/atomic"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/aws/retry"
	"github.com/aws/smithy-go/middleware"
)

// APICalls counts the AWS API calls of every client built from AwsConfig.
var APICalls = &APICounter{}

// APICounter counts AWS API calls, their attempts and their throttled responses. It is safe for
// concurrent use: the searches of a run share one.
type APICounter struct {
	calls, retries, throttled atomic.Int64
}

// attemptsKey is the stack value that holds the attempt count of one call.
type attemptsKey struct{}

// APICallCounts is a snapshot of an APICounter.
type APICallCounts struct {
	// Calls is the operations called, one per call whatever its attempts.
	Calls int64

	// Retries is the attempts beyond the first of each call.
	Retries int64

	// Throttled is the attempts that AWS answered with a throttling error.
	Throttled int64
}

// Counts returns the counts so far.
func (c *APICounter) Counts() APICallCounts {
	return APICallCounts{Calls: c.calls.Load(), Retries: c.retries.Load(), Throttled: c.throttled.Load()}
}

// AddTo adds the counting middleware to the stack of an operation. It is an aws.Config APIOptions
// function, so every client built from that config is counted.
func (c *APICounter) AddTo(stack *middleware.Stack) error {
	// The initialize step runs once per call. The retries of a call are its attempts beyond the
	// first; a call that fails before its first attempt, such as without credentials, has none.
	err := stack.Initialize.Add(middleware.InitializeMiddlewareFunc("awssCountCalls",
		func(ctx context.Context, in middleware.InitializeInput, next middleware.InitializeHandler) (
			middleware.InitializeOutput, middleware.Metadata, error,
		) {
			c.calls.Add(1)
			attempts := new(atomic.Int64)
			out, md, err := next.HandleInitialize(middleware.WithStackValue(ctx, attemptsKey{}, attempts), in)
			c.retries.Add(max(attempts.Load()-1, 0))
			return out, md, err //nolint:wrapcheck // the middleware only counts; the client wraps the error
		}), middleware.Before)
	if err != nil {
		return fmt.Errorf("adding the API call counter: %w", err)
	}

	// The deserialize step comes after the retry middleware, so it runs once per attempt, and,
	// added first, it sees the attempt's error as the client returns it.
	err = stack.Deserialize.Add(middleware.DeserializeMiddlewareFunc("awssCountAttempts",
		func(ctx context.Context, in middleware.DeserializeInput, next middleware.DeserializeHandler) (
			middleware.DeserializeOutput, middleware.Metadata, error,
		) {
			out, md, err := next.HandleDeserialize(ctx, in)
			if attempts, ok := middleware.GetStackValue(ctx, attemptsKey{}).(*atomic.Int64); ok {
				attempts.Add(1)
			}
			if err != nil && retry.IsErrorThrottles(retry.DefaultThrottles).IsErrorThrottle(err) == aws.TrueTernary {
				c.throttled.Add(1)
			}
			return out, md, err //nolint:wrapcheck // the middleware only counts; the client wraps the error
		}), middleware.Before)
	if err != nil {
		return fmt.Errorf("adding the API attempt counter: %w", err)
	}
	return nil
}

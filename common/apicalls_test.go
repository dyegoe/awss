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
	"errors"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/aws/ratelimit"
	"github.com/aws/aws-sdk-go-v2/aws/retry"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	"github.com/aws/smithy-go/middleware"
)

// fakeResponse is one HTTP response of fakeHTTPClient.
type fakeResponse struct {
	status int
	body   string
}

// fakeHTTPClient answers each request with the next of its responses, so a test drives the SDK's
// retries without a network.
type fakeHTTPClient struct {
	responses []fakeResponse
	requests  int
}

func (c *fakeHTTPClient) Do(req *http.Request) (*http.Response, error) {
	r := c.responses[min(c.requests, len(c.responses)-1)]
	c.requests++
	return &http.Response{
		StatusCode: r.status,
		Header:     http.Header{"Content-Type": {"text/xml"}},
		Body:       io.NopCloser(strings.NewReader(r.body)),
		Request:    req,
	}, nil
}

// noBackoff retries at once, so the test does not wait.
type noBackoff struct{}

func (noBackoff) BackoffDelay(int, error) (time.Duration, error) { return 0, nil }

// failingCredentials fails to retrieve credentials, as an expired SSO session does.
type failingCredentials struct{}

func (failingCredentials) Retrieve(context.Context) (aws.Credentials, error) {
	return aws.Credentials{}, errors.New("the SSO session has expired")
}

// ec2Error returns the body of an EC2 error response with the given code.
func ec2Error(code string) string {
	return `<Response><Errors><Error><Code>` + code + `</Code><Message>m</Message></Error></Errors>` +
		`<RequestID>r</RequestID></Response>`
}

// countedClient returns an EC2 client counted by counter, with the given credentials (anonymous
// when nil), that gets responses from a fake HTTP client and retries without waiting.
func countedClient(
	t *testing.T, counter *APICounter, credentials aws.CredentialsProvider, responses []fakeResponse,
) *ec2.Client {
	t.Helper()
	if credentials == nil {
		credentials = aws.AnonymousCredentials{}
	}
	return ec2.NewFromConfig(aws.Config{
		Region:      "us-east-1",
		Credentials: credentials,
		HTTPClient:  &fakeHTTPClient{responses: responses},
		Retryer: func() aws.Retryer {
			return retry.NewStandard(func(o *retry.StandardOptions) {
				o.Backoff = noBackoff{}
				o.RateLimiter = ratelimit.None
			})
		},
		APIOptions: []func(*middleware.Stack) error{counter.AddTo},
	})
}

// TestAPICounter_AddTo checks that the middleware counts one call per operation, the attempts
// beyond the first as retries, and the throttling responses, through a real EC2 client whose
// HTTP client is fake. A call that fails before its first attempt has no retry.
func TestAPICounter_AddTo(t *testing.T) {
	const ok = `<DescribeRegionsResponse xmlns="http://ec2.amazonaws.com/doc/2016-11-15/">` +
		`<requestId>r</requestId><regionInfo/></DescribeRegionsResponse>`
	tests := []struct {
		name      string
		responses []fakeResponse
		// credentials are anonymous unless set.
		credentials aws.CredentialsProvider
		wantErr     bool
		want        APICallCounts
	}{
		{
			name:      "success at the first attempt",
			responses: []fakeResponse{{http.StatusOK, ok}},
			want:      APICallCounts{Calls: 1},
		},
		{
			name:      "throttled once, then success",
			responses: []fakeResponse{{http.StatusServiceUnavailable, ec2Error("RequestLimitExceeded")}, {http.StatusOK, ok}},
			want:      APICallCounts{Calls: 1, Retries: 1, Throttled: 1},
		},
		{
			name:      "throttled at every attempt",
			responses: []fakeResponse{{http.StatusServiceUnavailable, ec2Error("RequestLimitExceeded")}},
			wantErr:   true,
			want:      APICallCounts{Calls: 1, Retries: 2, Throttled: 3},
		},
		{
			name:      "an error that is not retried",
			responses: []fakeResponse{{http.StatusForbidden, ec2Error("UnauthorizedOperation")}},
			wantErr:   true,
			want:      APICallCounts{Calls: 1},
		},
		{
			name:        "credentials that fail before the first attempt",
			responses:   []fakeResponse{{http.StatusOK, ok}},
			credentials: failingCredentials{},
			wantErr:     true,
			want:        APICallCounts{Calls: 1},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			counter := &APICounter{}
			client := countedClient(t, counter, tt.credentials, tt.responses)

			_, err := client.DescribeRegions(context.Background(), &ec2.DescribeRegionsInput{})

			if (err != nil) != tt.wantErr {
				t.Fatalf("DescribeRegions() error = %v, wantErr %v", err, tt.wantErr)
			}
			if got := counter.Counts(); got != tt.want {
				t.Errorf("APICounter.Counts() = %+v, want %+v", got, tt.want)
			}
		})
	}
}

// TestAwsConfig_countsAPICalls checks that AwsConfig adds the APICalls middleware, so every client
// built from it is counted.
func TestAwsConfig_countsAPICalls(t *testing.T) {
	empty := filepath.Join(t.TempDir(), "config")
	if err := os.WriteFile(empty, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("AWS_CONFIG_FILE", empty)
	t.Setenv("AWS_SHARED_CREDENTIALS_FILE", empty)

	cfg, err := AwsConfig("", "us-east-1")
	if err != nil {
		t.Fatalf("AwsConfig() error = %v", err)
	}

	stack := middleware.NewStack("test", nil)
	for _, opt := range cfg.APIOptions {
		if err := opt(stack); err != nil {
			t.Fatalf("APIOptions error = %v", err)
		}
	}
	if _, ok := stack.Initialize.Get("awssCountCalls"); !ok {
		t.Error("AwsConfig() stack has no awssCountCalls middleware")
	}
	if _, ok := stack.Deserialize.Get("awssCountAttempts"); !ok {
		t.Error("AwsConfig() stack has no awssCountAttempts middleware")
	}
}

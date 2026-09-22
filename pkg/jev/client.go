package jev

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"net/http"
	"os"
	"strconv"
	"time"
)

const (
	// EnvAPIKey is the env var the api key is read from
	EnvAPIKey = "TYPESAFE_API_KEY"
	// DefaultBaseURL is the production api root
	DefaultBaseURL = "https://api.typesafe.ai"
	// DefaultModel is the alias for the latest stable release
	DefaultModel = "jev-latest"

	// Time allowed per attempt when none is given
	defaultTimeout = 30 * time.Second
	// Retries after the first attempt when none is given
	defaultMaxRetries = 2
	// The first backoff wait, doubles each retry
	baseBackoff = 500 * time.Millisecond
	// The most a backoff wait can grow to
	maxBackoff = 10 * time.Second
)

// Client talks to the System One api
type Client struct {
	// The bearer token
	apiKey string
	// The api root
	baseURL string
	// The model sent when a request doesn't name one
	model string
	// The http client used for every call
	http *http.Client
	// Retries after the first attempt
	maxRetries int
	// Time allowed per attempt
	timeout time.Duration
}

// New
// Builds a client from the options given, the api key falls back to the TYPESAFE_API_KEY env var
// @param opts {...Option} - the options to apply
// @return {*Client, error}
func New(opts ...Option) (*Client, error) {
	// The client with every default filled in
	c := &Client{
		apiKey:     os.Getenv(EnvAPIKey),
		baseURL:    DefaultBaseURL,
		model:      DefaultModel,
		http:       &http.Client{},
		maxRetries: defaultMaxRetries,
		timeout:    defaultTimeout,
	}

	// Apply each option on top of the defaults
	for _, o := range opts {
		o(c)
	}

	// Refuse to build a client with no key
	if c.apiKey == "" {
		return nil, fmt.Errorf("missing API key: set %s or pass WithAPIKey", EnvAPIKey)
	}

	// Return the built client
	return c, nil
}

// Model
// Gives back the model the client sends by default
// @return {string}
func (c *Client) Model() string { return c.model }

// SystemOne
// Sends the state and questions with the default model and returns the answers
// @param ctx {context.Context} - the context for the request
// @param state {any} - the content to evaluate
// @param questions {map[string]Question} - the questions keyed by id
// @return {*Response, error}
func (c *Client) SystemOne(ctx context.Context, state any, questions map[string]Question) (*Response, error) {
	// Wrap the pieces in a request and send it
	return c.Do(ctx, Request{Model: c.model, State: state, Questions: questions})
}

// Do
// Sends a full request and returns the answers
// @param ctx {context.Context} - the context for the request
// @param req {Request} - the request to send
// @return {*Response, error}
func (c *Client) Do(ctx context.Context, req Request) (*Response, error) {
	// Nothing to ask
	if len(req.Questions) == 0 {
		return nil, errors.New("request has no questions")
	}

	// Fill in the model when the request left it blank
	if req.Model == "" {
		req.Model = c.model
	}

	// Encode the request as json
	body, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("encode request: %w", err)
	}

	// The decoded response
	var resp Response

	// Post the body and decode the answer into resp
	if err := c.call(ctx, http.MethodPost, "/v1/systemone", body, &resp); err != nil {
		return nil, err
	}

	// Return the decoded response
	return &resp, nil
}

// Models
// Lists the model names the account can send
// @param ctx {context.Context} - the context for the request
// @return {[]ModelCard, error}
func (c *Client) Models(ctx context.Context) ([]ModelCard, error) {
	// The decoded list wrapper
	var out struct {
		Models []ModelCard `json:"models"`
	}

	// Get the list and decode it into out
	if err := c.call(ctx, http.MethodGet, "/v1/models", nil, &out); err != nil {
		return nil, err
	}

	// Return just the models
	return out.Models, nil
}

// call
// Runs one api call with retries on rate limits, overloads and server errors
// @param ctx {context.Context} - the context for the request
// @param method {string} - the http method
// @param path {string} - the path under the base url
// @param body {[]byte} - the json body, nil for none
// @param out {any} - where the response is decoded into
// @return {error}
func (c *Client) call(ctx context.Context, method, path string, body []byte, out any) error {
	// The error from the last attempt
	var lastErr error

	// Loop over the first attempt and every retry
	for attempt := 0; attempt <= c.maxRetries; attempt++ {
		// Wait before every attempt after the first
		if attempt > 0 {
			if err := sleepCtx(ctx, backoffFor(lastErr, attempt)); err != nil {
				return err
			}
		}

		// Run the attempt
		err := c.once(ctx, method, path, body, out)

		// Done when it worked
		if err == nil {
			return nil
		}

		// Remember the error for the backoff and the final return
		lastErr = err

		// The api error if that is what came back
		var apiErr *APIError

		// Stop on an api error that can't be retried
		if errors.As(err, &apiErr) && !apiErr.Retryable() {
			return err
		}

		// Stop when the context is done
		if ctx.Err() != nil {
			return err
		}
	}

	// Every attempt failed
	return lastErr
}

// once
// Runs a single http attempt
// @param ctx {context.Context} - the context for the request
// @param method {string} - the http method
// @param path {string} - the path under the base url
// @param body {[]byte} - the json body, nil for none
// @param out {any} - where the response is decoded into
// @return {error}
func (c *Client) once(ctx context.Context, method, path string, body []byte, out any) error {
	// Cap the attempt at the timeout
	ctx, cancel := context.WithTimeout(ctx, c.timeout)
	defer cancel()

	// The body reader, nil when there is no body
	var rdr io.Reader
	if body != nil {
		rdr = bytes.NewReader(body)
	}

	// Build the request
	req, err := http.NewRequestWithContext(ctx, method, c.baseURL+path, rdr)
	if err != nil {
		return err
	}

	// Set the auth and content headers
	req.Header.Set("Authorization", "Bearer "+c.apiKey)
	req.Header.Set("Accept", "application/json")
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	// Send it
	res, err := c.http.Do(req)
	if err != nil {
		return fmt.Errorf("request %s %s: %w", method, path, err)
	}
	defer res.Body.Close()

	// Read the whole body
	raw, err := io.ReadAll(res.Body)
	if err != nil {
		return fmt.Errorf("read response: %w", err)
	}

	// Turn a non 2xx into an api error carrying the retry after header
	if res.StatusCode < 200 || res.StatusCode >= 300 {
		return &retryHint{APIError: &APIError{Status: res.StatusCode, Body: string(raw)}, retryAfter: parseRetryAfter(res.Header.Get("Retry-After"))}
	}

	// Decode the body into out
	if err := json.Unmarshal(raw, out); err != nil {
		return fmt.Errorf("decode response: %w", err)
	}

	// Worked
	return nil
}

// retryHint is an api error that also carries the server's retry after wait
type retryHint struct {
	*APIError
	// The wait the server asked for, zero when it didn't
	retryAfter time.Duration
}

// Unwrap
// Exposes the api error underneath for errors.As
// @return {error}
func (r *retryHint) Unwrap() error { return r.APIError }

// backoffFor
// Picks the wait before the next attempt, the server's retry after if given else exponential
// @param err {error} - the error from the last attempt
// @param attempt {int} - the attempt number about to run
// @return {time.Duration}
func backoffFor(err error, attempt int) time.Duration {
	// The retry hint if the error carries one
	var hint *retryHint

	// Use the server's wait when it gave one
	if errors.As(err, &hint) && hint.retryAfter > 0 {
		return hint.retryAfter
	}

	// Double the base wait for each attempt
	d := time.Duration(float64(baseBackoff) * math.Pow(2, float64(attempt-1)))

	// Cap it
	if d > maxBackoff {
		d = maxBackoff
	}

	// Return the wait
	return d
}

// parseRetryAfter
// Reads a Retry-After header as seconds or an http date
// @param h {string} - the header value
// @return {time.Duration}
func parseRetryAfter(h string) time.Duration {
	// No header
	if h == "" {
		return 0
	}

	// A number of seconds
	if secs, err := strconv.ParseFloat(h, 64); err == nil {
		return time.Duration(secs * float64(time.Second))
	}

	// An http date
	if t, err := http.ParseTime(h); err == nil {
		return time.Until(t)
	}

	// Couldn't read it
	return 0
}

// sleepCtx
// Sleeps for d unless the context ends first
// @param ctx {context.Context} - the context to watch
// @param d {time.Duration} - how long to sleep
// @return {error}
func sleepCtx(ctx context.Context, d time.Duration) error {
	// Nothing to wait for
	if d <= 0 {
		return nil
	}

	// The timer for the wait
	t := time.NewTimer(d)
	defer t.Stop()

	// Wait for the timer or the context
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-t.C:
		return nil
	}
}

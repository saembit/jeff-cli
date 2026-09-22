package jev

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
)

// Checks the request body and headers going out and the three answer shapes coming back
func TestSystemOneSendsRequestAndParsesAnswers(t *testing.T) {
	// The body the fake server saw
	var got map[string]any

	// A fake server that checks the request and answers all three types
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// The path has to be the systemone endpoint
		if r.URL.Path != "/v1/systemone" {
			t.Errorf("path = %q", r.URL.Path)
		}

		// The key has to go out as a bearer token
		if r.Header.Get("Authorization") != "Bearer k" {
			t.Errorf("auth = %q", r.Header.Get("Authorization"))
		}

		// Keep the body
		_ = json.NewDecoder(r.Body).Decode(&got)

		// Answer with one of each type
		_, _ = w.Write([]byte(`{"model":"jev-1.13.0","answers":{
			"u":{"type":"noul","noul":0.9},
			"t":{"type":"choice","choice":"billing","probabilities":{"billing":0.8,"tech":0.2},"confidence":0.6},
			"f":{"type":"score","score":1.2,"legend":{"0":"calm","1":"mad"},"probabilities":{"0":0.4,"1":0.6},"confidence":0.2}
		},"usage":{"input_tokens":10,"output_tokens":3}}`))
	}))
	defer srv.Close()

	// A client pointed at the fake server
	c, err := New(WithAPIKey("k"), WithBaseURL(srv.URL))
	if err != nil {
		t.Fatal(err)
	}

	// Send one of each question type
	resp, err := c.SystemOne(context.Background(), "hi", map[string]Question{
		"u": Noul("urgent?", nil),
		"t": Choice("team?", map[string]any{"billing": nil, "tech": nil}),
		"f": Score("mad?", []any{"calm", "mad"}),
	})
	if err != nil {
		t.Fatal(err)
	}

	// The model and state went out
	if got["model"] != "jev-latest" || got["state"] != "hi" {
		t.Errorf("body = %v", got)
	}

	// The questions went out
	qs := got["questions"].(map[string]any)
	if qs["u"].(map[string]any)["type"] != "noul" {
		t.Errorf("noul question not sent: %v", qs)
	}

	// Nil criteria are left out of the json
	if _, ok := qs["u"].(map[string]any)["criteria"]; ok {
		t.Errorf("nil criteria must be omitted")
	}

	// The model and usage came back
	if resp.Model != "jev-1.13.0" || resp.Usage.InputTokens != 10 {
		t.Errorf("resp = %+v", resp)
	}

	// Each answer decoded into the right fields
	if *resp.Answers["u"].Noul != 0.9 {
		t.Errorf("noul = %v", resp.Answers["u"].Noul)
	}
	if resp.Answers["t"].Choice != "billing" || resp.Answers["t"].Probabilities["tech"] != 0.2 {
		t.Errorf("choice = %+v", resp.Answers["t"])
	}
	if *resp.Answers["f"].Score != 1.2 || resp.Answers["f"].Legend["1"] != "mad" {
		t.Errorf("score = %+v", resp.Answers["f"])
	}
}

// Checks a 429 is tried again and the retry after header is honored
func TestSystemOneRetriesOn429(t *testing.T) {
	// How many calls the fake server saw
	var calls int32

	// A fake server that rate limits the first call and answers the second
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if atomic.AddInt32(&calls, 1) == 1 {
			w.Header().Set("Retry-After", "0")
			w.WriteHeader(429)
			return
		}
		_, _ = w.Write([]byte(`{"model":"m","answers":{},"usage":{}}`))
	}))
	defer srv.Close()

	// A client with two retries
	c, _ := New(WithAPIKey("k"), WithBaseURL(srv.URL), WithMaxRetries(2))

	// The call should work on the second try
	if _, err := c.SystemOne(context.Background(), "x", map[string]Question{"q": Noul("?", nil)}); err != nil {
		t.Fatal(err)
	}

	// Exactly two calls went out
	if calls != 2 {
		t.Errorf("calls = %d, want 2", calls)
	}
}

// Checks a 422 comes back as an APIError and isn't retried
func TestSystemOneReturnsAPIError(t *testing.T) {
	// A fake server that always rejects
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(422)
		_, _ = w.Write([]byte(`{"detail":"bad"}`))
	}))
	defer srv.Close()

	// A client pointed at it
	c, _ := New(WithAPIKey("k"), WithBaseURL(srv.URL))

	// The call should fail
	_, err := c.SystemOne(context.Background(), "x", map[string]Question{"q": Noul("?", nil)})

	// The error should be an APIError with the status
	var apiErr *APIError
	if !errors.As(err, &apiErr) || apiErr.Status != 422 {
		t.Fatalf("err = %v", err)
	}
}

// Checks New refuses to build with no key anywhere
func TestNewRequiresKey(t *testing.T) {
	// Clear the env var
	t.Setenv("TYPESAFE_API_KEY", "")

	// New should fail
	if _, err := New(); err == nil {
		t.Fatal("expected error without key")
	}
}

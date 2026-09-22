package jev

import (
	"net/http"
	"time"
)

// Option changes one setting on a Client
type Option func(*Client)

// WithAPIKey
// Sets the bearer token, defaults to the TYPESAFE_API_KEY env var
// @param key {string} - the api key
// @return {Option}
func WithAPIKey(key string) Option { return func(c *Client) { c.apiKey = key } }

// WithBaseURL
// Sets the api root, defaults to https://api.typesafe.ai
// @param u {string} - the base url
// @return {Option}
func WithBaseURL(u string) Option { return func(c *Client) { c.baseURL = u } }

// WithModel
// Sets the model sent by default, defaults to jev-latest
// @param m {string} - the model alias or id
// @return {Option}
func WithModel(m string) Option { return func(c *Client) { c.model = m } }

// WithHTTPClient
// Swaps out the http client
// @param h {*http.Client} - the client to use
// @return {Option}
func WithHTTPClient(h *http.Client) Option { return func(c *Client) { c.http = h } }

// WithMaxRetries
// Sets how many times a 429, 529 or 5xx is tried again after the first attempt
// @param n {int} - the retry count
// @return {Option}
func WithMaxRetries(n int) Option { return func(c *Client) { c.maxRetries = n } }

// WithTimeout
// Sets the time allowed per attempt
// @param d {time.Duration} - the timeout
// @return {Option}
func WithTimeout(d time.Duration) Option { return func(c *Client) { c.timeout = d } }

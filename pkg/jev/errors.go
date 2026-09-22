package jev

import "fmt"

// APIError is a non 2xx response from the api
type APIError struct {
	// The http status code
	Status int
	// The response body as text
	Body string
}

// Error
// Formats the status and a cut down body as the error text
// @return {string}
func (e *APIError) Error() string {
	// Return the status and the first part of the body
	return fmt.Sprintf("typesafe api: HTTP %d: %s", e.Status, truncate(e.Body, 500))
}

// Retryable
// Says if the status is a rate limit, overload or server error
// @return {bool}
func (e *APIError) Retryable() bool {
	// Rate limited, overloaded or any 5xx can be tried again
	return e.Status == 429 || e.Status == 529 || e.Status >= 500
}

// truncate
// Cuts a string down to n chars and adds ... when it was longer
// @param s {string} - the string to cut
// @param n {int} - the max length
// @return {string}
func truncate(s string, n int) string {
	// Short enough already
	if len(s) <= n {
		return s
	}

	// Return the cut string with a marker
	return s[:n] + "..."
}

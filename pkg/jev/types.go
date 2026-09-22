// Package jev is a small client for the TypeSafe System One api
package jev

// Question is one typed question the model answers about the state
type Question struct {
	// The kind of question, noul, choice or score
	Type string `json:"type"`
	// The question itself, a string or a json object
	Instructions any `json:"instructions"`
	// The options, levels or yes/no meanings depending on the type
	Criteria any `json:"criteria,omitempty"`
}

// NoulCriteria says what a yes and a no mean for a noul question
type NoulCriteria struct {
	// What a yes means
	True any `json:"true,omitempty"`
	// What a no means
	False any `json:"false,omitempty"`
}

// Noul
// Builds a yes/no question
// @param instructions {any} - the question to ask
// @param criteria {*NoulCriteria} - what yes and no mean, nil to skip
// @return {Question}
func Noul(instructions any, criteria *NoulCriteria) Question {
	// The question with no criteria yet
	q := Question{Type: "noul", Instructions: instructions}

	// Only attach the criteria when the caller gave some
	if criteria != nil {
		q.Criteria = criteria
	}

	// Return the built question
	return q
}

// Choice
// Builds a pick one question over the options given
// @param instructions {any} - the question to ask
// @param options {map[string]any} - option name to description, nil for no description
// @return {Question}
func Choice(instructions any, options map[string]any) Question {
	// Return the question with the options as the criteria
	return Question{Type: "choice", Instructions: instructions, Criteria: options}
}

// Score
// Builds a rubric question over ordered levels, lowest first
// @param instructions {any} - the question to ask
// @param levels {[]any} - the level descriptions in order
// @return {Question}
func Score(instructions any, levels []any) Question {
	// Return the question with the levels as the criteria
	return Question{Type: "score", Instructions: instructions, Criteria: levels}
}

// Request is the body sent to the systemone endpoint
type Request struct {
	// The model alias or versioned id
	Model string `json:"model"`
	// The content the questions are about
	State any `json:"state"`
	// The questions keyed by the id the answer comes back under
	Questions map[string]Question `json:"questions"`
}

// Answer is one typed answer, the fields that are set depend on Type
type Answer struct {
	// The kind of answer, noul, choice or score
	Type string `json:"type"`
	// The probability of yes for a noul
	Noul *float64 `json:"noul,omitempty"`
	// The picked option for a choice
	Choice string `json:"choice,omitempty"`
	// The weighted level for a score
	Score *float64 `json:"score,omitempty"`
	// The level index to description for a score
	Legend map[string]string `json:"legend,omitempty"`
	// The probability per option or level
	Probabilities map[string]float64 `json:"probabilities,omitempty"`
	// How concentrated the probabilities are for a choice or score
	Confidence *float64 `json:"confidence,omitempty"`
}

// Usage is the token count for one request
type Usage struct {
	// Tokens sent
	InputTokens int `json:"input_tokens"`
	// Tokens returned
	OutputTokens int `json:"output_tokens"`
}

// Response is the body returned by the systemone endpoint
type Response struct {
	// The versioned model that answered
	Model string `json:"model"`
	// One answer per question id
	Answers map[string]Answer `json:"answers"`
	// The token usage for the request
	Usage Usage `json:"usage"`
}

// ModelCard is one row from the models endpoint
type ModelCard struct {
	// The name accepted by the model field
	Name string `json:"name"`
	// What the model is for
	Description string `json:"description"`
	// When it was released
	ReleaseDate string `json:"release_date"`
}

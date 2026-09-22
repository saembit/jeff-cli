// Package rank scores a list of items on weighted dimensions with Jev and ranks them in code
package rank

import (
	"errors"
	"fmt"
	"io"
	"sort"

	"github.com/saembit/jeff-cli/pkg/jev"
	"gopkg.in/yaml.v3"
)

const (
	// The fewest levels a score can have
	minLevels = 2
	// The most levels the api allows
	maxLevels = 10
)

// OverallID is the question id of the optional overall choice
const OverallID = "__overall"

// Dimension is one score question asked about every item
type Dimension struct {
	// How much this dimension counts in the composite
	Weight float64 `yaml:"weight"`
	// The question asked about each item
	Question string `yaml:"question"`
	// The level descriptions lowest first
	Levels []string `yaml:"levels"`
}

// Spec is one rank job, the items, the dimensions and some shared context
type Spec struct {
	// Shared facts every question can see
	Context map[string]any `yaml:"context"`
	// Item id to description
	Items map[string]any `yaml:"items"`
	// Dimension name to rubric
	Dimensions map[string]Dimension `yaml:"dimensions"`
	// When set, one choice over every item with this question
	Overall string `yaml:"overall"`
}

// Row is one ranked item
type Row struct {
	// The item id
	ID string `json:"id"`
	// The weighted sum of the scores
	Composite float64 `json:"composite"`
	// The score per dimension
	Scores map[string]float64 `json:"scores"`
}

// ParseSpec
// Reads a yaml spec and checks it
// @param r {io.Reader} - the yaml to read
// @return {*Spec, error}
func ParseSpec(r io.Reader) (*Spec, error) {
	// The decoded spec
	var s Spec

	// Decode the yaml
	if err := yaml.NewDecoder(r).Decode(&s); err != nil {
		return nil, fmt.Errorf("parse spec: %w", err)
	}

	// Check it
	if err := s.validate(); err != nil {
		return nil, err
	}

	// Return the spec
	return &s, nil
}

// validate
// Checks the spec has items and well formed dimensions
// @return {error}
func (s *Spec) validate() error {
	// Nothing to rank
	if len(s.Items) == 0 {
		return errors.New("spec has no items")
	}

	// Nothing to rank on
	if len(s.Dimensions) == 0 {
		return errors.New("spec has no dimensions")
	}

	// Check each dimension
	for name, d := range s.Dimensions {
		// A dimension needs a question
		if d.Question == "" {
			return fmt.Errorf("dimension %q has no question", name)
		}

		// A dimension needs between 2 and 10 levels
		if n := len(d.Levels); n < minLevels || n > maxLevels {
			return fmt.Errorf("dimension %q has %d levels; need %d to %d", name, n, minLevels, maxLevels)
		}

		// A weight can't be negative
		if d.Weight < 0 {
			return fmt.Errorf("dimension %q has negative weight", name)
		}
	}

	// All good
	return nil
}

// QuestionID
// Names the score question for one item and dimension
// @param item {string} - the item id
// @param dim {string} - the dimension name
// @return {string}
func QuestionID(item, dim string) string { return item + "__" + dim }

// Build
// Makes the state and the question map for one request
// @return {any, map[string]jev.Question}
func (s *Spec) Build() (any, map[string]jev.Question) {
	// The state with the items in it
	state := map[string]any{"items": s.Items}

	// Add the context when there is some
	if len(s.Context) > 0 {
		state["context"] = s.Context
	}

	// The question map sized for every item and dimension plus the overall
	qs := make(map[string]jev.Question, len(s.Items)*len(s.Dimensions)+1)

	// Loop over every item
	for item := range s.Items {
		// Loop over every dimension
		for name, d := range s.Dimensions {
			// Point the question at this item
			instr := fmt.Sprintf("Consider only `items.%s`", item)

			// Point it at the context too when there is some
			if len(s.Context) > 0 {
				instr += " given `context`"
			}

			// Add the dimension's question
			instr += ". " + d.Question

			// Store the score question under the item and dimension id
			qs[QuestionID(item, name)] = jev.Score(instr, toAny(d.Levels))
		}
	}

	// Add the overall choice when asked for
	if s.Overall != "" {
		// One option per item
		opts := make(map[string]any, len(s.Items))
		for item := range s.Items {
			opts[item] = nil
		}

		// The overall question pointed at the items
		instr := s.Overall + " Answer with one entry of `items`."

		// Point it at the context too when there is some
		if len(s.Context) > 0 {
			instr = "Given `context`: " + instr
		}

		// Store the choice
		qs[OverallID] = jev.Choice(instr, opts)
	}

	// Return the state and the questions
	return state, qs
}

// Composite
// Sums the weighted scores per item and sorts them highest first
// @param answers {map[string]jev.Answer} - the answers from the api
// @return {[]Row, error}
func (s *Spec) Composite(answers map[string]jev.Answer) ([]Row, error) {
	// One row per item
	rows := make([]Row, 0, len(s.Items))

	// Loop over every item
	for item := range s.Items {
		// The row for this item
		row := Row{ID: item, Scores: make(map[string]float64, len(s.Dimensions))}

		// Loop over every dimension
		for name, d := range s.Dimensions {
			// The answer for this item and dimension
			a, ok := answers[QuestionID(item, name)]

			// The api has to have answered with a score
			if !ok || a.Score == nil {
				return nil, fmt.Errorf("no score answer for item %q dimension %q", item, name)
			}

			// Keep the raw score
			row.Scores[name] = *a.Score

			// Add the weighted score to the composite
			row.Composite += d.Weight * *a.Score
		}

		// Add the row
		rows = append(rows, row)
	}

	// Sort highest composite first, ties by id so the order is stable
	sort.Slice(rows, func(i, j int) bool {
		if rows[i].Composite == rows[j].Composite {
			return rows[i].ID < rows[j].ID
		}
		return rows[i].Composite > rows[j].Composite
	})

	// Return the sorted rows
	return rows, nil
}

// DimensionNames
// Gives the dimension names sorted so output is stable
// @return {[]string}
func (s *Spec) DimensionNames() []string {
	// The names
	names := make([]string, 0, len(s.Dimensions))

	// Collect them
	for n := range s.Dimensions {
		names = append(names, n)
	}

	// Sort them
	sort.Strings(names)

	// Return them
	return names
}

// toAny
// Copies a string slice into an any slice for the api
// @param ss {[]string} - the strings
// @return {[]any}
func toAny(ss []string) []any {
	// The copy
	out := make([]any, len(ss))

	// Copy each string over
	for i, s := range ss {
		out[i] = s
	}

	// Return the copy
	return out
}

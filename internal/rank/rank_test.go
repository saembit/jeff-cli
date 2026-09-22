package rank

import (
	"math"
	"strings"
	"testing"

	"github.com/saembit/jeff-cli/pkg/jev"
)

// A small spec with two items and two dimensions
const specYAML = `
context:
  builder: solo dev
items:
  a: first thing
  b: second thing
dimensions:
  money:
    weight: 0.75
    question: How much money?
    levels: [none, some, lots]
  ease:
    weight: 0.25
    question: How easy?
    levels: [hard, easy]
`

// Checks a good spec parses with its weights
func TestParseSpec(t *testing.T) {
	// Parse the spec
	s, err := ParseSpec(strings.NewReader(specYAML))
	if err != nil {
		t.Fatal(err)
	}

	// Both items and both dimensions came through
	if len(s.Items) != 2 || len(s.Dimensions) != 2 {
		t.Fatalf("spec = %+v", s)
	}

	// The weight came through
	if s.Dimensions["money"].Weight != 0.75 {
		t.Errorf("weight = %v", s.Dimensions["money"].Weight)
	}
}

// Checks a dimension with one level is refused
func TestParseSpecRejectsBadLevels(t *testing.T) {
	// The spec with a single level
	bad := strings.Replace(specYAML, "levels: [hard, easy]", "levels: [only]", 1)

	// It should fail to parse
	if _, err := ParseSpec(strings.NewReader(bad)); err == nil {
		t.Fatal("expected error for single level")
	}
}

// Checks Build makes one score question per item and dimension pointed at the item
func TestBuildQuestionsOnePerItemDimension(t *testing.T) {
	// Parse the spec
	s, _ := ParseSpec(strings.NewReader(specYAML))

	// Build the request pieces
	state, qs := s.Build()

	// Two items times two dimensions
	if len(qs) != 4 {
		t.Fatalf("got %d questions", len(qs))
	}

	// The question for a on money
	q := qs[QuestionID("a", "money")]

	// It is a score
	if q.Type != "score" {
		t.Errorf("type = %s", q.Type)
	}

	// It points at the item by path
	if !strings.Contains(q.Instructions.(string), "`items.a`") {
		t.Errorf("instructions must point at the item: %v", q.Instructions)
	}

	// The state carries the context and the items
	st := state.(map[string]any)
	if st["context"] == nil || st["items"] == nil {
		t.Errorf("state = %v", st)
	}
}

// Checks the composite is weight times score summed and sorted highest first
func TestCompositeSortsByWeightedScore(t *testing.T) {
	// Parse the spec
	s, _ := ParseSpec(strings.NewReader(specYAML))

	// A helper to take a float's address
	f := func(v float64) *float64 { return &v }

	// Fake answers, a is 2 and 0, b is 1 and 1
	answers := map[string]jev.Answer{
		QuestionID("a", "money"): {Type: "score", Score: f(2)},
		QuestionID("a", "ease"):  {Type: "score", Score: f(0)},
		QuestionID("b", "money"): {Type: "score", Score: f(1)},
		QuestionID("b", "ease"):  {Type: "score", Score: f(1)},
	}

	// Add them up
	rows, err := s.Composite(answers)
	if err != nil {
		t.Fatal(err)
	}

	// a is 0.75*2 + 0.25*0 = 1.5 and comes first
	if rows[0].ID != "a" || math.Abs(rows[0].Composite-1.5) > 1e-9 {
		t.Errorf("rows = %+v", rows)
	}

	// b is 0.75*1 + 0.25*1 = 1.0 and comes second
	if rows[1].ID != "b" || math.Abs(rows[1].Composite-1.0) > 1e-9 {
		t.Errorf("rows = %+v", rows)
	}
}

// Checks a missing answer is an error instead of a zero
func TestCompositeMissingAnswerErrors(t *testing.T) {
	// Parse the spec
	s, _ := ParseSpec(strings.NewReader(specYAML))

	// No answers at all should fail
	if _, err := s.Composite(map[string]jev.Answer{}); err == nil {
		t.Fatal("expected error")
	}
}

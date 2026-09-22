package input

import (
	"strings"
	"testing"
)

// Checks auto sends json as json and text as text and the forced formats hold
func TestParseStateAutoDetectsJSON(t *testing.T) {
	// An object in auto mode decodes
	v, err := ParseState(`{"a":1}`, "auto")
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := v.(map[string]any); !ok {
		t.Errorf("want object, got %T", v)
	}

	// Plain words in auto mode stay a string
	v, _ = ParseState(`plain words`, "auto")
	if _, ok := v.(string); !ok {
		t.Errorf("want string, got %T", v)
	}

	// Text mode keeps json as a string
	v, _ = ParseState(`{"a":1}`, "text")
	if _, ok := v.(string); !ok {
		t.Errorf("text format must keep string, got %T", v)
	}

	// Json mode refuses text
	if _, err := ParseState(`nope`, "json"); err == nil {
		t.Error("json format with invalid json must error")
	}
}

// Checks - reads the reader and empty input is an error
func TestReadStateFromReader(t *testing.T) {
	// - reads and trims the reader
	s, err := ReadState("", "-", strings.NewReader("  from stdin \n"))
	if err != nil || s != "from stdin" {
		t.Errorf("s=%q err=%v", s, err)
	}

	// Nothing at all is an error
	if _, err := ReadState("", "", strings.NewReader("")); err == nil {
		t.Error("empty state must error")
	}
}

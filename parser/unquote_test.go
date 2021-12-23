package parser

import (
	"testing"

	"github.com/alecthomas/participle/v2/lexer"
)

func TestUnquote(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{`"\n\027"`, "\n\027"},
		{`"\?"`, "\x3f"},
		{`'\n\027'`, `\n\027`},
		{`"\n\x17"`, "\n\027"},
	}
	for _, test := range tests {
		actual, err := unquote(lexer.Token{Value: test.input})
		if err != nil {
			t.Fatal(err)
		}
		if actual.Value != test.expected {
			t.Fatalf("%q (actual) != %q (expected)", actual.Value, test.expected)
		}
	}
}

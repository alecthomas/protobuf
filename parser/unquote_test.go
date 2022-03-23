package parser

import (
	"testing"

	"github.com/alecthomas/participle/v2/lexer"
	"github.com/stretchr/testify/require"
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
		{`"hello\0world"`, "hello\000world"},
		{`"hello\x0world"`, "hello\000world"},
		{`"\0001"`, "\0001"},
		{`"\x001"`, "\x001"},
		{`"\341\210\264"`, "ሴ"},
	}
	for _, test := range tests {
		actual, err := unquote(lexer.Token{Value: test.input})
		require.NoError(t, err)
		require.Equal(t, test.expected, actual.Value)
	}
}

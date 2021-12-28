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
	}
	for _, test := range tests {
		actual, err := unquote(lexer.Token{Value: test.input})
		require.NoError(t, err)
		require.Equal(t, actual.Value, test.expected)
	}
}

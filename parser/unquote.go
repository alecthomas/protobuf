package parser

import (
	"fmt"
	"strconv"
	"strings"
	"unicode"

	"github.com/alecthomas/participle/v2/lexer"
)

var escapeTable = map[rune]rune{
	'0':  '\000',
	'a':  '\x07',
	'b':  '\x08',
	'e':  '\x1B',
	'f':  '\x0C',
	'n':  '\x0A',
	'r':  '\x0D',
	't':  '\x09',
	'v':  '\x0B',
	'\\': '\x5C',
	'\'': '\x27',
	'"':  '\x22',
	'?':  '\x3F',
}

var stateBase = map[unquoteState]int{
	uqHex: 16, uqOctal: 8,
}

type unquoteState int

const (
	uqDefault unquoteState = iota
	uqEscape
	uqHex
	uqOctal
)

// C-style unquoting - supports octal \DDD and hex \xDDD
func unquote(token lexer.Token) (lexer.Token, error) {
	kind := token.Value[0] // nolint: ifshort
	token.Value = token.Value[1 : len(token.Value)-1]
	// Single quoted, no escaping.
	if kind == '\'' {
		return token, nil
	}
	out := strings.Builder{}
	state := uqDefault
	// Digits being collected in hex/octal modes
	var digits string
	for _, rn := range token.Value {
		switch state {
		case uqEscape:
			if rn == 'x' { // nolint: gocritic
				state = uqHex
			} else if unicode.Is(unicode.Digit, rn) {
				state = uqOctal
				digits = string(rn)
			} else {
				escaped, ok := escapeTable[rn]
				if !ok {
					return token, fmt.Errorf("%s: %q: unknown escape sequence \"\\%c\"", token.Pos, token.Value, rn)
				}
				out.WriteRune(escaped)
				state = uqDefault
			}
			continue
		case uqHex:
			if unicode.Is(unicode.ASCII_Hex_Digit, rn) && len(digits) < 2 {
				digits += string(rn)
				continue
			}
			if err := flushDigits(digits, 16, &out); err != nil {
				return token, fmt.Errorf("%s: %w", token.Pos, err)
			}
			state = uqDefault
			digits = ""
		case uqOctal:
			if unicode.IsDigit(rn) && len(digits) < 3 {
				digits += string(rn)
				continue
			}
			if err := flushDigits(digits, 8, &out); err != nil {
				return token, fmt.Errorf("%s: %w", token.Pos, err)
			}
			state = uqDefault
			digits = ""
		case uqDefault:
		default:
			panic(state)
		}
		if rn == '\\' {
			state = uqEscape
		} else {
			out.WriteRune(rn)
		}
	}
	if digits != "" {
		if err := flushDigits(digits, stateBase[state], &out); err != nil {
			return token, fmt.Errorf("%s: %w", token.Pos, err)
		}
	}
	token.Value = out.String()
	return token, nil
}

func flushDigits(digits string, base int, out *strings.Builder) error {
	n, err := strconv.ParseUint(digits, base, 32)
	if err != nil {
		return fmt.Errorf("invalid base %d numeric value %s", base, digits)
	}
	if n > 255 {
		return fmt.Errorf("base %d value %s larger than 255", base, digits)
	}
	out.WriteByte(byte(n))
	return nil
}

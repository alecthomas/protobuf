package parser

import (
	"math/big"
	"os"
	"path/filepath"
	"reflect"
	"strconv"
	"testing"

	"github.com/alecthomas/participle/v2/lexer"
	"github.com/alecthomas/repr"
	"github.com/stretchr/testify/require"
)

func TestParserFromFixtures(t *testing.T) {
	files, err := filepath.Glob("../testdata/*.proto")
	require.NoError(t, err)
	for _, file := range files {
		t.Run(file, func(t *testing.T) {
			r, err := os.Open(file)
			require.NoError(t, err)
			_, err = Parse(file, r)
			require.NoError(t, err)
		})
	}
}

func TestParser(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected *Proto
	}{{
		name: "MessageOptions",
		input: `
			message VariousComplexOptions {
			  option (complex_opt2).bar.(protobuf_unittest.corge).qux = 2008;
			  option (complex_opt2).(protobuf_unittest.garply).(corge).qux = 2121;
			  option .(.ComplexOptionType2.ComplexOptionType4.complex_opt4).waldo = 1971;
			  option (strings) = "1" "2";
			}
			`,
		expected: &Proto{
			Entries: []*Entry{
				{Message: &Message{
					Name: "VariousComplexOptions",
					Entries: []*MessageEntry{
						{Option: &Option{
							Name:  []*OptionName{{Name: "(complex_opt2)"}, {Name: "bar"}, {Name: "(protobuf_unittest.corge)"}, {Name: "qux"}},
							Value: &Value{Number: toBig(2008)},
						}},
						{Option: &Option{
							Name:  []*OptionName{{Name: "(complex_opt2)"}, {Name: "(protobuf_unittest.garply)"}, {Name: "(corge)"}, {Name: "qux"}},
							Value: &Value{Number: toBig(2121)},
						}},
						{Option: &Option{
							Name:  []*OptionName{{Name: ".(.ComplexOptionType2.ComplexOptionType4.complex_opt4)"}, {Name: "waldo"}},
							Value: &Value{Number: toBig(1971)},
						}},
						{Option: &Option{
							Name:  []*OptionName{{Name: "(strings)"}},
							Value: &Value{String: strP("12")},
						}},
					},
				}},
			},
		}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			actual, err := ParseString(test.name, test.input)
			require.NoError(t, err)
			_ = Visit(actual, clearPos)
			expectedStr := repr.String(test.expected, repr.Indent("  "))
			actualStr := repr.String(actual, repr.Indent("  "))
			require.Equal(t, expectedStr, actualStr, actualStr)
		})
	}
}

func TestImports(t *testing.T) {
	tests := []struct {
		name   string
		source string
		want   []*Import
	}{{
		name:   "parses a single import correctly",
		source: `import 'foo/bar/test.proto'`,
		want:   []*Import{{Name: "foo/bar/test.proto", Public: false}},
	}, {
		name:   "parses public imports correctly",
		source: `import public "foo/bar/test.proto"`,
		want:   []*Import{{Name: "foo/bar/test.proto", Public: true}},
	}}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ParseString("test.proto", tt.source)
			require.NoError(t, err)

			result := imports(got)
			require.Equal(t, tt.want, result)
		})
	}
}

func imports(from *Proto) []*Import {
	var result []*Import
	for _, entity := range from.Entries {
		if entity.Import != nil {
			result = append(result, entity.Import)
		}
	}
	return result
}

var zeroPos = reflect.ValueOf(lexer.Position{})

func clearPos(node Node, next func() error) error {
	reflect.Indirect(reflect.ValueOf(node)).FieldByName("Pos").Set(zeroPos)
	return next()
}

func toBig(n int) *big.Float {
	f, _, _ := big.ParseFloat(strconv.Itoa(n), 10, 64, 0)
	return f
}

func strP(s string) *string {
	return &s
}

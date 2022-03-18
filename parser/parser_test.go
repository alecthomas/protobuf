package parser

import (
	"fmt"
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
			  option (.ComplexOptionType2.ComplexOptionType4.complex_opt4).waldo = 1971;
			  option (strings) = "1" "2";
			  option deprecated = true;
			  option deprecated = false;
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
							Name:  []*OptionName{{Name: "(.ComplexOptionType2.ComplexOptionType4.complex_opt4)"}, {Name: "waldo"}},
							Value: &Value{Number: toBig(1971)},
						}},
						{Option: &Option{
							Name:  []*OptionName{{Name: "(strings)"}},
							Value: &Value{String: strP("12")},
						}},
						{Option: &Option{
							Name:  []*OptionName{{Name: "deprecated"}},
							Value: &Value{Bool: boolP(true)},
						}},
						{Option: &Option{
							Name:  []*OptionName{{Name: "deprecated"}},
							Value: &Value{Bool: boolP(false)},
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

func TestProtoTextString(t *testing.T) {
	tests := []struct {
		name string
		in   Value
		want string
	}{{
		name: "string",
		in:   Value{String: strP("howdy")},
		want: `"howdy"`,
	}, {
		name: "bool",
		in:   Value{Bool: boolP(true)},
		want: "true",
	}, {
		name: "number",
		in:   Value{Number: toBig(2008)},
		want: "2008",
	}, {
		name: "array",
		in:   Value{Array: &Array{Elements: []*Value{{String: strP("1")}, {String: strP("2")}}}},
		want: `[ "1", "2" ]`,
	}, {
		name: "nested",
		in: Value{ProtoText: &ProtoText{
			Fields: []*ProtoTextField{
				{Name: "aString", Value: &Value{String: strP("abc")}},
				{Type: "aNum", Value: &Value{Number: toBig(12)}},
				{Name: "nest", Value: &Value{ProtoText: &ProtoText{Fields: []*ProtoTextField{
					{Name: "egg", Value: &Value{String: strP("chick")}},
					{Type: "ext", Value: &Value{String: strP("penguin")}},
				}}}},
			},
		}},
		want: `{
    aString: "abc"
    [aNum]: 12
    nest: {
      egg: "chick"
      [ext]: "penguin"
    }
  }`,
	}}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pt := ProtoText{Fields: []*ProtoTextField{{Name: "a_name", Value: &tt.in}}}
			got := pt.String()
			want := fmt.Sprintf("\n  %s: %s\n", "a_name", tt.want)
			require.Equal(t, want, got)
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

func boolP(b bool) *Boolean {
	bv := Boolean(b)
	return &bv
}

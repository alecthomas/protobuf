// Package parser contains a protobuf parser.
// nolint: govet, golint
package parser

import (
	"fmt"
	"io"
	"math/big"
	"strings"

	"github.com/alecthomas/participle/v2"
	"github.com/alecthomas/participle/v2/lexer"
)

type Proto struct {
	Pos lexer.Position

	Comments *Comments `@@?`

	Syntax  string   `("syntax" "=" @String ";")?`
	Entries []*Entry `{ @@ { ";" } }`
}

type Comments struct {
	Pos lexer.Position

	Comments []*Comment `@@+`
}

type Comment struct {
	Pos lexer.Position

	Comment string `@Comment`
}

type Entry struct {
	Pos lexer.Position

	Comments *Comments `@@?`

	Package string   `(   "package" @(Ident { "." Ident })`
	Import  *Import  `  | @@`
	Message *Message `  | @@`
	Service *Service `  | @@`
	Enum    *Enum    `  | @@`
	Option  *Option  `  | "option" @@`
	Extend  *Extend  `  | @@ )`
}

type Import struct {
	Public bool   `"import" @("public")?`
	Name   string `@String`
}

func (i *Import) children() (out []Node) { return nil }

type Option struct {
	Pos lexer.Position

	Comments *Comments `@@?`

	Name  []*OptionName `@@+`
	Attr  *string       `[ @("."? Ident { "." Ident }) ]`
	Value *Value        `"=" @@`
}

type OptionName struct {
	Pos lexer.Position

	Name string `( @("."? "(" ("."? Ident { "." Ident }) ")") | @Ident ) "."?`
}

func (o *OptionName) children() []Node { return nil }

type Value struct {
	Pos lexer.Position

	String    *string    `  @String+`
	Number    *big.Float `| ("-" | "+")? (@Float | @Int)`
	Bool      *Boolean   `| @("true"|"false")`
	Reference *string    `| @("."? Ident { "." Ident })`
	ProtoText *ProtoText `| "{" @@ "}"`
	Array     *Array     `| @@`
}

type Boolean bool

func (b *Boolean) Capture(v []string) error { *b = v[0] == "true"; return nil }

type ProtoText struct {
	Pos lexer.Position

	Fields []*ProtoTextField `( @@ ( "," | ";" )? )*`

	TrailingComments *Comments `@@?`
}

type ProtoTextField struct {
	Pos lexer.Position

	Comments *Comments `@@?`

	Name  string `(  @Ident`
	Type  string `  | "[" @("."? Ident { ("." | "/") Ident }) "]" )`
	Value *Value `( ":"? @@ )`
}

type Array struct {
	Pos lexer.Position

	Elements []*Value `"[" [ @@ { [ "," ] @@ } ] "]"`
}

type Extensions struct {
	Pos lexer.Position

	Extensions []*Range `"extensions" @@ { "," @@ }`
}

type Reserved struct {
	Pos lexer.Position

	Ranges     []*Range `@@ { "," @@ }`
	FieldNames []string `| @String { "," @String }`
}

type Range struct {
	Start   int     `@Int`
	End     *int    `  [ "to" ( @Int`
	Max     bool    `           | @"max" ) ]`
	Options Options `[ "[" @@ { "," @@ } "]" ]`
}

type Extend struct {
	Pos lexer.Position

	Reference string   `"extend" @("."? Ident { "." Ident })`
	Fields    []*Field `"{" { @@ [ ";" ] } "}"`
}

type Service struct {
	Pos lexer.Position

	Name  string          `"service" @Ident`
	Entry []*ServiceEntry `"{" { @@ [ ";" ] } "}"`
}

type ServiceEntry struct {
	Pos lexer.Position

	Option *Option `  "option" @@`
	Method *Method `| @@`
}

type Method struct {
	Pos lexer.Position

	Name              string  `"rpc" @Ident`
	StreamingRequest  bool    `"(" [ @"stream" ]`
	Request           *Type   `    @@ ")"`
	StreamingResponse bool    `"returns" "(" [ @"stream" ]`
	Response          *Type   `              @@ ")"`
	Options           Options `[ "{" { "option" @@ ";" } "}" ]`
}

type Enum struct {
	Pos lexer.Position

	Name   string       `"enum" @Ident`
	Values []*EnumEntry `"{" { @@ { ";" } }`

	TrailingComments Comments `@@* "}"`
}

type EnumEntry struct {
	Pos lexer.Position

	Comments *Comments `@@?`

	Value    *EnumValue `(   @@`
	Option   *Option    `  | "option" @@`
	Reserved *Reserved  `  | "reserved" @@ )`

	TrailingComments *Comments `@@?`
}

type Options []*Option

type EnumValue struct {
	Pos lexer.Position

	Key   string `@Ident`
	Value int    `"=" @( [ "-" ] Int )`

	Options Options `[ "[" @@ { "," @@ } "]" ]`
}

type Message struct {
	Pos lexer.Position

	Name    string          `"message" @Ident`
	Entries []*MessageEntry `"{" { @@ } "}"`
}

type MessageEntry struct {
	Pos lexer.Position

	Comments *Comments `@@?`

	Enum       *Enum       `( @@`
	Option     *Option     ` | "option" @@`
	Message    *Message    ` | @@`
	Oneof      *OneOf      ` | @@`
	Extend     *Extend     ` | @@`
	Reserved   *Reserved   ` | "reserved" @@`
	Extensions *Extensions ` | @@`
	Field      *Field      ` | @@ ) { ";" }`

	TrailingComments *Comments `@@?`
}

type OneOf struct {
	Pos lexer.Position

	Name    string        `"oneof" @Ident`
	Entries []*OneOfEntry `"{" { @@ { ";" } } "}"`
}

type OneOfEntry struct {
	Pos lexer.Position

	Comments *Comments `@@?`

	Field  *Field  `(  @@`
	Option *Option ` | "option" @@ )`
}

type Field struct {
	Pos lexer.Position

	Comments *Comments `@@?`

	Optional bool `[   @"optional"`
	Required bool `  | @"required"`
	Repeated bool `  | @"repeated" ]`

	Group  *Group  `( @@`
	Direct *Direct `| @@ ) ";"*`

	TrailingComments *Comments `@@?`
}

type Direct struct {
	Pos lexer.Position

	Type *Type  `@@`
	Name string `@Ident`
	Tag  int    `"=" @Int`

	Options Options `[ "[" @@ { "," @@ } "]" ]`
}

type Group struct {
	Pos lexer.Position

	Name    string          `"group" @Ident`
	Tag     int             `"=" @Int`
	Entries []*MessageEntry `"{" { @@ [ ";" ] } "}"`
}

type Scalar int

const (
	None Scalar = iota
	Double
	Float
	Int32
	Int64
	Uint32
	Uint64
	Sint32
	Sint64
	Fixed32
	Fixed64
	SFixed32
	SFixed64
	Bool
	String
	Bytes
)

var scalarToString = map[Scalar]string{
	None: "None", Double: "Double", Float: "Float", Int32: "Int32", Int64: "Int64", Uint32: "Uint32",
	Uint64: "Uint64", Sint32: "Sint32", Sint64: "Sint64", Fixed32: "Fixed32", Fixed64: "Fixed64",
	SFixed32: "SFixed32", SFixed64: "SFixed64", Bool: "Bool", String: "String", Bytes: "Bytes",
}

func (s Scalar) GoString() string { return scalarToString[s] }

var stringToScalar = map[string]Scalar{
	"double": Double, "float": Float, "int32": Int32, "int64": Int64, "uint32": Uint32, "uint64": Uint64,
	"sint32": Sint32, "sint64": Sint64, "fixed32": Fixed32, "fixed64": Fixed64, "sfixed32": SFixed32,
	"sfixed64": SFixed64, "bool": Bool, "string": String, "bytes": Bytes,
}

func (s *Scalar) Parse(lex *lexer.PeekingLexer) error {
	token, err := lex.Peek(0)
	if err != nil {
		return fmt.Errorf("failed to peek next token: %w", err)
	}
	scalar, ok := stringToScalar[token.Value]
	if !ok {
		return participle.NextMatch
	}
	_, err = lex.Next()
	if err != nil {
		return fmt.Errorf("failed to read next token: %w", err)
	}
	*s = scalar
	return nil
}

type Type struct {
	Pos lexer.Position

	Scalar    Scalar   `  @@`
	Map       *MapType `| @@`
	Reference *string  `| @("."? Ident { "." Ident })`
}

type MapType struct {
	Pos lexer.Position

	Key   *Type `"map" "<" @@`
	Value *Type `"," @@ ">"`
}

func (p *ProtoText) String() string {
	var b strings.Builder
	indent := "  "
	b.WriteString("\n")
	for _, f := range p.Fields {
		typ := f.Type
		if typ != "" {
			typ = "[" + typ + "]"
		}
		val := f.Value.indentString(indent)
		fmt.Fprintf(&b, "%s%s%s: %s\n", indent, f.Name, typ, val)
	}
	return b.String()
}

func (p *ProtoText) indentString(indent string) string {
	var b strings.Builder
	b.WriteString("{\n")
	for _, f := range p.Fields {
		indent2 := indent + "  "
		typ := f.Type
		if typ != "" {
			typ = "[" + typ + "]"
		}
		val := f.Value.indentString(indent2)
		fmt.Fprintf(&b, "%s%s%s: %s\n", indent2, f.Name, typ, val)
	}
	b.WriteString(indent + "}")
	return b.String()
}

func (v *Value) ToString() string {
	return v.indentString("")
}

func (v *Value) indentString(indent string) string {
	switch {
	case v.String != nil:
		return `"` + *v.String + `"`
	case v.Number != nil:
		return v.Number.String()
	case v.Bool != nil:
		return fmt.Sprintf("%t", *v.Bool)
	case v.Reference != nil:
		return *v.Reference
	case v.ProtoText != nil:
		return v.ProtoText.indentString(indent)
	case v.Array != nil:
		return v.Array.indentString(indent)
	default:
		return "UNKNOWN-VALUE"
	}
}

func (a *Array) indentString(indent string) string {
	s := make([]string, len(a.Elements))
	for i, e := range a.Elements {
		s[i] = e.indentString(indent)
	}
	return "[ " + strings.Join(s, ", ") + " ]"
}

var (
	lex = lexer.MustSimple([]lexer.Rule{
		{"String", `"(\\"|[^"])*"|'(\\'|[^'])*'`, nil},
		{"Ident", `[a-zA-Z_]([a-zA-Z_0-9])*`, nil},
		{"Float", `[-+]?(\d*\.\d+([eE][-+]?\d+)?|\d+[eE][-+]?\d+|inf)`, nil},
		{"Int", `[-+]?(0[xX][0-9A-Fa-f]+)|([-+]?\d+)`, nil},
		{"Whitespace", `[ \t\n\r\s]+`, nil},
		{"Comment", `(/\*([^*]|[\r\n]|(\*+([^*/]|[\r\n])))*\*+/)|(//(.*)[^\n]*\n)`, nil},
		{"Symbols", `[/={}\[\]()<>.,;:]`, nil},
	})

	parser = participle.MustBuild(
		&Proto{},
		participle.UseLookahead(2),
		participle.Map(unquote, "String"),
		participle.Lexer(lex),
		participle.Elide("Whitespace"),
	)
)

// Parse protobuf.
func Parse(filename string, r io.Reader) (*Proto, error) {
	p := &Proto{}
	err := parser.Parse(filename, r, p)
	if err != nil {
		return p, err
	}
	return p, nil
}

func ParseString(filename string, source string) (*Proto, error) {
	p := &Proto{}
	err := parser.ParseString(filename, source, p)
	if err != nil {
		return p, err
	}
	return p, nil
}

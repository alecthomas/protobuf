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

	Syntax  string   `("syntax" "=" @String ";")?`
	Entries []*Entry `{ @@ { ";" } }`
}

type Entry struct {
	Pos lexer.Position

	Package string   `"package" @(Ident { "." Ident })`
	Import  *Import  `| @@`
	Message *Message `| @@`
	Service *Service `| @@`
	Enum    *Enum    `| @@`
	Option  *Option  `| "option" @@`
	Extend  *Extend  `| @@`
}

type Import struct {
	Public bool   `"import" @("public")?`
	Name   string `@String`
}

func (i *Import) children() (out []Node) { return nil }

type Option struct {
	Pos lexer.Position

	Name  []*OptionName `@@+`
	Attr  *string       `[ @("."? Ident { "." Ident }) ]`
	Value *Value        `"=" @@`
}

type OptionName struct {
	Pos lexer.Position

	Name string `( @("."? "(" ("."? Ident { "." Ident }) ")") | @("."? Ident { "." Ident }) ) "."?`
}

func (o *OptionName) children() []Node { return nil }

type Value struct {
	Pos lexer.Position

	String    *string    `  @String`
	Number    *big.Float `| ("-" | "+")? (@Float | @Int)`
	Bool      *bool      `| (@"true" | "false")`
	Reference *string    `| @("."? Ident { "." Ident })`
	ProtoText *ProtoText `| "{" @@ "}"`
	Array     *Array     `| @@`
}

type ProtoText struct {
	Pos lexer.Position

	Fields []*ProtoTextField `( @@ ( "," | ";" )? )*`
}

type ProtoTextField struct {
	Pos lexer.Position

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
	Start int  `@Int`
	End   *int `  [ "to" ( @Int`
	Max   bool `           | @"max" ) ]`
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
	Values []*EnumEntry `"{" { @@ { ";" } } "}"`
}

type EnumEntry struct {
	Pos lexer.Position

	Value    *EnumValue `  @@`
	Option   *Option    `| "option" @@`
	Reserved *Reserved  `| "reserved" @@`
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

	Enum       *Enum       `( @@`
	Option     *Option     ` | "option" @@`
	Message    *Message    ` | @@`
	Oneof      *OneOf      ` | @@`
	Extend     *Extend     ` | @@`
	Reserved   *Reserved   ` | "reserved" @@`
	Extensions *Extensions ` | @@`
	Field      *Field      ` | @@ ) { ";" }`
}

type OneOf struct {
	Pos lexer.Position

	Name    string        `"oneof" @Ident`
	Entries []*OneOfEntry `"{" { @@ { ";" } } "}"`
}

type OneOfEntry struct {
	Pos lexer.Position

	Field  *Field  `@@`
	Option *Option `| "option" @@`
}

type Field struct {
	Pos lexer.Position

	Optional bool `[   @"optional"`
	Required bool `  | @"required"`
	Repeated bool `  | @"repeated" ]`

	Group  *Group  `( @@`
	Direct *Direct `| @@ )`
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

// Parse protobuf.
func Parse(filename string, r io.Reader) (*Proto, error) {
	p := &Proto{}

	l := lexer.MustSimple([]lexer.Rule{
		{"String", `"(\\"|[^"])*"|'(\\'|[^'])*'`, nil},
		{"Ident", `[a-zA-Z_]([a-zA-Z_0-9])*`, nil},
		{"Float", `[-+]?(\d*\.\d+([eE][-+]?\d+)?|\d+[eE][-+]?\d+|inf)`, nil},
		{"Int", `[-+]?(0[xX][0-9A-Fa-f]+)|([-+]?\d+)`, nil},
		{"Whitespace", `[ \t\n\r\s]+`, nil},
		{"BlockComment", `/\*([^*]|[\r\n]|(\*+([^*/]|[\r\n])))*\*+/`, nil},
		{"LineComment", `//(.*)[^\n]*\n`, nil},
		{"Symbols", `[/={}\[\]()<>.,;:]`, nil},
	})

	parser := participle.MustBuild(
		&Proto{},
		participle.UseLookahead(2),
		participle.Map(unquote, "String"),
		participle.Lexer(l),
		participle.Elide("Whitespace", "LineComment", "BlockComment"),
	)
	err := parser.Parse(filename, r, p)
	if err != nil {
		return p, err
	}
	return p, nil
}

func ParseString(filename string, source string) (*Proto, error) {
	return Parse(filename, strings.NewReader(source))
}

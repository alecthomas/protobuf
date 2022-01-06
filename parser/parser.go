// Package parser contains a protobuf parser.
// nolint: govet, golint
package parser

import (
	"fmt"
	"io"
	"math/big"
	"reflect"
	"strings"

	"github.com/alecthomas/participle/v2"
	"github.com/alecthomas/participle/v2/lexer"
)

type Node interface {
	children() []Node
}

// Visitor function.
type Visitor func(node Node, next func() error) error

// Visit all nodes in the AST.
func Visit(root Node, visit Visitor) error {
	return visit(root, func() error {
		for _, child := range root.children() {
			pv := reflect.ValueOf(child)
			if pv.Kind() != reflect.Struct && pv.IsNil() {
				continue
			}
			if err := Visit(child, visit); err != nil {
				return err
			}
		}
		return nil
	})
}

type Proto struct {
	Pos lexer.Position

	Syntax  string   `("syntax" "=" @String ";")?`
	Entries []*Entry `{ @@ { ";" } }`
}

func (p *Proto) children() (out []Node) {
	for _, child := range p.Entries {
		out = append(out, child)
	}
	return out
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

func (e *Entry) children() (out []Node) {
	return []Node{e.Import, e.Message, e.Service, e.Enum, e.Option, e.Extend}
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

func (o *Option) children() (out []Node) {
	for _, name := range o.Name {
		out = append(out, name)
	}
	out = append(out, o.Value)
	return
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

func (v *Value) children() (out []Node) {
	return []Node{v.ProtoText, v.Array}
}

type ProtoText struct {
	Pos lexer.Position

	Fields []*ProtoTextField `( @@ ( "," | ";" )? )*`
}

func (p *ProtoText) children() (out []Node) {
	for _, field := range p.Fields {
		out = append(out, field)
	}
	return
}

type ProtoTextField struct {
	Pos lexer.Position

	Name  string `(  @Ident`
	Type  string `  | "[" @("."? Ident { ("." | "/") Ident }) "]" )`
	Value *Value `( ":"? @@ )`
}

func (p *ProtoTextField) children() (out []Node) {
	out = append(out, p.Value)
	return out
}

type Array struct {
	Pos lexer.Position

	Elements []*Value `"[" [ @@ { [ "," ] @@ } ] "]"`
}

func (a *Array) children() (out []Node) {
	for _, element := range a.Elements {
		out = append(out, element)
	}
	return
}

type Extensions struct {
	Pos lexer.Position

	Extensions []*Range `"extensions" @@ { "," @@ }`
}

func (e *Extensions) children() (out []Node) {
	for _, rng := range e.Extensions {
		out = append(out, rng)
	}
	return
}

type Reserved struct {
	Pos lexer.Position

	Ranges     []*Range `@@ { "," @@ }`
	FieldNames []string `| @String { "," @String }`
}

func (r *Reserved) children() (out []Node) {
	for _, rng := range r.Ranges {
		out = append(out, rng)
	}
	return
}

type Range struct {
	Start int  `@Int`
	End   *int `  [ "to" ( @Int`
	Max   bool `           | @"max" ) ]`
}

func (r *Range) children() (out []Node) {
	return nil
}

type Extend struct {
	Pos lexer.Position

	Reference string   `"extend" @("."? Ident { "." Ident })`
	Fields    []*Field `"{" { @@ [ ";" ] } "}"`
}

func (e *Extend) children() (out []Node) {
	for _, field := range e.Fields {
		out = append(out, field)
	}
	return
}

type Service struct {
	Pos lexer.Position

	Name  string          `"service" @Ident`
	Entry []*ServiceEntry `"{" { @@ [ ";" ] } "}"`
}

func (s *Service) children() (out []Node) {
	for _, entry := range s.Entry {
		out = append(out, entry)
	}
	return
}

type ServiceEntry struct {
	Pos lexer.Position

	Option *Option `  "option" @@`
	Method *Method `| @@`
}

func (s *ServiceEntry) children() (out []Node) {
	return []Node{s.Option, s.Method}
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

func (m *Method) children() (out []Node) {
	out = []Node{m.Request, m.Response}
	out = append(out, m.Options.children()...)
	return
}

type Enum struct {
	Pos lexer.Position

	Name   string       `"enum" @Ident`
	Values []*EnumEntry `"{" { @@ { ";" } } "}"`
}

func (e *Enum) children() (out []Node) {
	for _, enum := range e.Values {
		out = append(out, enum)
	}
	return
}

type EnumEntry struct {
	Pos lexer.Position

	Value    *EnumValue `  @@`
	Option   *Option    `| "option" @@`
	Reserved *Reserved  `| "reserved" @@`
}

func (e *EnumEntry) children() (out []Node) {
	return []Node{e.Value, e.Option, e.Reserved}
}

type Options []*Option

func (o Options) children() (out []Node) {
	for _, option := range o {
		out = append(out, option)
	}
	return
}

type EnumValue struct {
	Pos lexer.Position

	Key   string `@Ident`
	Value int    `"=" @( [ "-" ] Int )`

	Options Options `[ "[" @@ { "," @@ } "]" ]`
}

func (e *EnumValue) children() (out []Node) {
	return e.Options.children()
}

type Message struct {
	Pos lexer.Position

	Name    string          `"message" @Ident`
	Entries []*MessageEntry `"{" { @@ } "}"`
}

func (m *Message) children() (out []Node) {
	for _, entry := range m.Entries {
		out = append(out, entry)
	}
	return
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

func (m *MessageEntry) children() (out []Node) {
	return []Node{m.Enum, m.Option, m.Message, m.Oneof, m.Extend, m.Reserved, m.Extensions, m.Field}
}

type OneOf struct {
	Pos lexer.Position

	Name    string        `"oneof" @Ident`
	Entries []*OneOfEntry `"{" { @@ { ";" } } "}"`
}

func (o *OneOf) children() (out []Node) {
	for _, entry := range o.Entries {
		out = append(out, entry)
	}
	return
}

type OneOfEntry struct {
	Pos lexer.Position

	Field  *Field  `@@`
	Option *Option `| "option" @@`
}

func (o *OneOfEntry) children() (out []Node) {
	return []Node{o.Field, o.Option}
}

type Field struct {
	Pos lexer.Position

	Optional bool `[   @"optional"`
	Required bool `  | @"required"`
	Repeated bool `  | @"repeated" ]`

	Group  *Group  `( @@`
	Direct *Direct `| @@ )`
}

func (f *Field) children() (out []Node) {
	return []Node{f.Group, f.Direct}
}

type Direct struct {
	Pos lexer.Position

	Type *Type  `@@`
	Name string `@Ident`
	Tag  int    `"=" @Int`

	Options Options `[ "[" @@ { "," @@ } "]" ]`
}

func (d *Direct) children() (out []Node) {
	out = []Node{d.Type}
	out = append(out, d.Options.children()...)
	return
}

type Group struct {
	Pos lexer.Position

	Name    string          `"group" @Ident`
	Tag     int             `"=" @Int`
	Entries []*MessageEntry `"{" { @@ [ ";" ] } "}"`
}

func (g *Group) children() (out []Node) {
	for _, entry := range g.Entries {
		out = append(out, entry)
	}
	return
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

func (t *Type) children() (out []Node) {
	return []Node{t.Map}
}

type MapType struct {
	Pos lexer.Position

	Key   *Type `"map" "<" @@`
	Value *Type `"," @@ ">"`
}

func (m *MapType) children() (out []Node) {
	return []Node{m.Key, m.Value}
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

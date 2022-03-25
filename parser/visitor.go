package parser

import "reflect"

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

func (p *Proto) children() (out []Node) {
	out = append(out, p.Comments.children()...)
	for _, child := range p.Entries {
		out = append(out, child)
	}
	return out
}

func (c *Comments) children() (out []Node) {
	if c == nil {
		return nil
	}
	for _, comment := range c.Comments {
		out = append(out, comment)
	}
	return
}

func (c *Comment) children() (out []Node) {
	return nil
}

func (e *Entry) children() (out []Node) {
	out = append(out, e.Comments.children()...)
	out = append(out, e.Import, e.Message, e.Service, e.Enum, e.Option, e.Extend)
	return
}

func (o *Option) children() (out []Node) {
	out = append(out, o.Comments.children()...)
	for _, name := range o.Name {
		out = append(out, name)
	}
	out = append(out, o.Value)
	return
}

func (v *Value) children() (out []Node) {
	return []Node{v.ProtoText, v.Array}
}

func (p *ProtoText) children() (out []Node) {
	for _, field := range p.Fields {
		out = append(out, field)
	}
	return
}

func (p *ProtoTextField) children() (out []Node) {
	out = append(out, p.Comments.children()...)
	out = append(out, p.Value)
	return out
}

func (a *Array) children() (out []Node) {
	for _, element := range a.Elements {
		out = append(out, element)
	}
	return
}

func (e *Extensions) children() (out []Node) {
	for _, rng := range e.Extensions {
		out = append(out, rng)
	}
	return
}

func (r *Reserved) children() (out []Node) {
	for _, rng := range r.Ranges {
		out = append(out, rng)
	}
	return
}

func (r *Range) children() (out []Node) {
	return nil
}

func (e *Extend) children() (out []Node) {
	for _, field := range e.Fields {
		out = append(out, field)
	}
	return
}

func (s *Service) children() (out []Node) {
	for _, entry := range s.Entries {
		out = append(out, entry)
	}
	return
}

func (s *ServiceEntry) children() (out []Node) {
	return []Node{s.Option, s.Method, s.Comment}
}

func (m *Method) children() (out []Node) {
	out = []Node{m.Request, m.Response}
	for _, entry := range m.Entries {
		out = append(out, entry)
	}
	return
}

func (m *MethodEntry) children() (out []Node) {
	return []Node{m.Option, m.Comment}
}

func (e *Enum) children() (out []Node) {
	for _, enum := range e.Values {
		out = append(out, enum)
	}
	out = append(out, e.TrailingComments.children()...)
	return
}

func (e *EnumEntry) children() (out []Node) {
	return []Node{e.Value, e.Option, e.Reserved}
}

func (o Options) children() (out []Node) {
	for _, option := range o {
		out = append(out, option)
	}
	return
}

func (e *EnumValue) children() (out []Node) {
	return e.Options.children()
}

func (m *Message) children() (out []Node) {
	for _, entry := range m.Entries {
		out = append(out, entry)
	}
	return
}

func (m *MessageEntry) children() (out []Node) {
	out = append(out, m.Comments.children()...)
	out = append(out, m.Enum, m.Option, m.Message, m.Oneof, m.Extend, m.Reserved, m.Extensions, m.Field)
	out = append(out, m.TrailingComments.children()...)
	return
}

func (o *OneOf) children() (out []Node) {
	for _, entry := range o.Entries {
		out = append(out, entry)
	}
	return
}

func (o *OneOfEntry) children() (out []Node) {
	out = append(out, o.Comments.children()...)
	out = append(out, o.Field, o.Option)
	return
}

func (f *Field) children() (out []Node) {
	out = append(out, f.Comments.children()...)
	out = append(out, f.Group, f.Direct)
	out = append(out, f.TrailingComments.children()...)
	return
}

func (d *Direct) children() (out []Node) {
	out = []Node{d.Type}
	out = append(out, d.Options.children()...)
	return
}

func (g *Group) children() (out []Node) {
	for _, entry := range g.Entries {
		out = append(out, entry)
	}
	return
}

func (t *Type) children() (out []Node) {
	return []Node{t.Map}
}

func (m *MapType) children() (out []Node) {
	return []Node{m.Key, m.Value}
}

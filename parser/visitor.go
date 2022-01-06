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
	for _, child := range p.Entries {
		out = append(out, child)
	}
	return out
}

func (e *Entry) children() (out []Node) {
	return []Node{e.Import, e.Message, e.Service, e.Enum, e.Option, e.Extend}
}

func (o *Option) children() (out []Node) {
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
	for _, entry := range s.Entry {
		out = append(out, entry)
	}
	return
}

func (s *ServiceEntry) children() (out []Node) {
	return []Node{s.Option, s.Method}
}

func (m *Method) children() (out []Node) {
	out = []Node{m.Request, m.Response}
	out = append(out, m.Options.children()...)
	return
}

func (e *Enum) children() (out []Node) {
	for _, enum := range e.Values {
		out = append(out, enum)
	}
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
	return []Node{m.Enum, m.Option, m.Message, m.Oneof, m.Extend, m.Reserved, m.Extensions, m.Field}
}

func (o *OneOf) children() (out []Node) {
	for _, entry := range o.Entries {
		out = append(out, entry)
	}
	return
}

func (o *OneOfEntry) children() (out []Node) {
	return []Node{o.Field, o.Option}
}

func (f *Field) children() (out []Node) {
	return []Node{f.Group, f.Direct}
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

// Package compiler creates FileDescriptorProtos from a parser.Parse()
// result. It is assumed that the parse result is semantically
// correct and would compile with protoc.
package compiler

import (
	"fmt"
	"strings"

	"github.com/alecthomas/protobuf/parser"
	pb "google.golang.org/protobuf/types/descriptorpb"
)

type fdBuilder struct {
	proto3   bool
	fileDesc *pb.FileDescriptorProto
	types    types
	pkg      string
}

type messageBuilder struct {
	proto3      bool
	messageDesc *pb.DescriptorProto
	types       types
	scope       []string
}

func NewFileDescriptor(file string, p *parser.Proto) *pb.FileDescriptorProto {
	b := &fdBuilder{
		proto3: p.Syntax == "proto3",
		pkg:    protoPkg(p),
	}
	b.types = analyseTypes(p)
	b.buildFileDescriptor(file, p)
	return b.fileDesc
}

func (b *fdBuilder) buildFileDescriptor(file string, p *parser.Proto) {
	var syntax *string
	if b.proto3 {
		s := "proto3"
		syntax = &s
	}
	b.fileDesc = &pb.FileDescriptorProto{
		Name:   &file,
		Syntax: syntax,
	}
	for _, e := range p.Entries {
		b.addEntry(e)
	}
}

func (b *fdBuilder) addEntry(e *parser.Entry) {
	switch {
	case e.Package != "":
		b.fileDesc.Package = &e.Package
	case e.Import != nil:
		b.buildImports(e.Import)
	case e.Message != nil:
		scope := []string{}
		if b.pkg != "" {
			scope = append(scope, b.pkg)
		}
		m := newMessage(e.Message, b.proto3, scope, b.types)
		b.fileDesc.MessageType = append(b.fileDesc.MessageType, m)
	case e.Enum != nil:
	case e.Service != nil:
	case e.Option != nil:
	case e.Extend != nil:
	default:
		panic(fmt.Sprintf("%s: cannot interpret Entry", e.Pos))
	}
}

func (b *fdBuilder) buildImports(imp *parser.Import) {
	b.fileDesc.Dependency = append(b.fileDesc.Dependency, imp.Name)
	if imp.Public {
		idx := int32(len(b.fileDesc.Dependency) - 1)
		b.fileDesc.PublicDependency = append(b.fileDesc.PublicDependency, idx)
	}
}

func newMessage(m *parser.Message, proto3 bool, scope []string, types types) *pb.DescriptorProto {
	b := &messageBuilder{
		proto3: proto3,
		scope:  append(scope, m.Name),
		types:  types,
	}
	b.buildMessageDescriptor(m)
	return b.messageDesc
}

func (b *messageBuilder) buildMessageDescriptor(pm *parser.Message) {
	b.messageDesc = &pb.DescriptorProto{Name: &pm.Name}
	for _, e := range pm.Entries {
		b.addEntry(e)
	}
}

func (b *messageBuilder) addEntry(e *parser.MessageEntry) {
	switch {
	case e.Field != nil:
		b.buildField(e.Field)
	case e.Message != nil:
		m := newMessage(e.Message, b.proto3, b.scope, b.types)
		b.messageDesc.NestedType = append(b.messageDesc.NestedType, m)
	case e.Enum != nil:
	case e.Option != nil:
	case e.Oneof != nil:
	case e.Extend != nil:
	case e.Reserved != nil:
	case e.Extensions != nil:
	default:
		panic(fmt.Sprintf("%s: cannot interpret MessageEntry", e.Pos))
	}
}

func (b *messageBuilder) buildField(pField *parser.Field) {
	fieldDesc := b.createField(pField)
	b.messageDesc.Field = append(b.messageDesc.Field, fieldDesc)

	if b.proto3 && fieldDesc.Proto3Optional != nil && *fieldDesc.Proto3Optional {
		idx := int32(len(b.messageDesc.Field)) - 1
		fieldDesc.OneofIndex = &idx
		name := "_" + *fieldDesc.Name
		dcl := &pb.OneofDescriptorProto{Name: &name}
		b.messageDesc.OneofDecl = append(b.messageDesc.OneofDecl, dcl)
	}
}

var scalars = map[parser.Scalar]pb.FieldDescriptorProto_Type{
	parser.Double:   pb.FieldDescriptorProto_TYPE_DOUBLE,
	parser.Float:    pb.FieldDescriptorProto_TYPE_FLOAT,
	parser.Int32:    pb.FieldDescriptorProto_TYPE_INT32,
	parser.Int64:    pb.FieldDescriptorProto_TYPE_INT64,
	parser.Uint32:   pb.FieldDescriptorProto_TYPE_UINT32,
	parser.Uint64:   pb.FieldDescriptorProto_TYPE_UINT64,
	parser.Sint32:   pb.FieldDescriptorProto_TYPE_SINT32,
	parser.Sint64:   pb.FieldDescriptorProto_TYPE_SINT64,
	parser.Fixed32:  pb.FieldDescriptorProto_TYPE_FIXED32,
	parser.Fixed64:  pb.FieldDescriptorProto_TYPE_FIXED64,
	parser.SFixed32: pb.FieldDescriptorProto_TYPE_SFIXED32,
	parser.SFixed64: pb.FieldDescriptorProto_TYPE_SFIXED64,
	parser.Bool:     pb.FieldDescriptorProto_TYPE_BOOL,
	parser.String:   pb.FieldDescriptorProto_TYPE_STRING,
	parser.Bytes:    pb.FieldDescriptorProto_TYPE_BYTES,
}

func (b *messageBuilder) createField(pf *parser.Field) *pb.FieldDescriptorProto {
	if pf.Direct == nil || pf.Direct.Type.Map != nil {
		panic(fmt.Sprintf("%s: non-direct not implemented", pf.Pos))
	}

	tag := int32(pf.Direct.Tag)
	df := &pb.FieldDescriptorProto{
		Name:           &pf.Direct.Name,
		Number:         &tag,
		JsonName:       jsonStr(pf.Direct.Name),
		Label:          fieldLabel(pf, b.proto3),
		Proto3Optional: proto3Optional(pf, b.proto3),
	}

	if pf.Direct.Type.Reference != nil {
		t := pb.FieldDescriptorProto_TYPE_MESSAGE
		df.Type = &t

		name := b.types.fullName(*pf.Direct.Type.Reference, b.scope)
		df.TypeName = &name
		return df
	}

	fieldType := scalars[pf.Direct.Type.Scalar]
	df.Type = &fieldType
	return df
}

func fieldLabel(pf *parser.Field, proto3 bool) *pb.FieldDescriptorProto_Label {
	var label pb.FieldDescriptorProto_Label
	switch {
	case pf.Required:
		label = pb.FieldDescriptorProto_LABEL_REQUIRED
	case pf.Repeated:
		label = pb.FieldDescriptorProto_LABEL_REPEATED
	case pf.Optional:
		label = pb.FieldDescriptorProto_LABEL_OPTIONAL
	case proto3:
		// unlabelled proto3 field
		label = pb.FieldDescriptorProto_LABEL_OPTIONAL
	default:
		panic(fmt.Sprintf("%s: invalid field label for syntax", pf.Pos))
	}
	return &label
}

func proto3Optional(pf *parser.Field, proto3 bool) *bool {
	if pf.Optional && proto3 {
		p3Optional := true
		return &p3Optional
	}
	return nil
}

func jsonStr(s string) *string {
	ss := strings.Split(s, "_")
	result := strings.ToLower(ss[0])
	for _, s := range ss[1:] {
		result += strings.Title(strings.ToLower(s))
	}
	return &result
}

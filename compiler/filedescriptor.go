package compiler

import (
	"fmt"
	"math"
	"strings"

	"github.com/alecthomas/protobuf/parser"
	pb "google.golang.org/protobuf/types/descriptorpb"
)

type messageBuilder struct {
	proto3      bool
	messageDesc *pb.DescriptorProto
	types       types
	scope       []string
}

func newFileDescriptor(ast *ast, types types) *pb.FileDescriptorProto {
	var proto3 bool
	var syntax *string
	if ast.syntax == "proto3" {
		proto3 = true
		syntax = &ast.syntax
	}
	fd := &pb.FileDescriptorProto{
		Name:             &ast.file,
		Syntax:           syntax,
		Options:          newOptions(ast.options),
		Dependency:       ast.imports,
		PublicDependency: ast.publicImports,

		WeakDependency: nil,
	}
	scope := []string{}
	if ast.pkg != "" {
		fd.Package = &ast.pkg
		scope = append(scope, ast.pkg)
	}
	for _, m := range ast.messages {
		md := newMessage(m, proto3, scope, types)
		fd.MessageType = append(fd.MessageType, md)
	}
	for _, s := range ast.services {
		sd := newService(s, scope, types)
		fd.Service = append(fd.Service, sd)
	}
	for _, e := range ast.enums {
		fd.EnumType = append(fd.EnumType, newEnum(e))
	}
	for _, e := range ast.extends {
		ed := newExtend(e, scope, types)
		fd.Extension = append(fd.Extension, ed)
	}

	return fd
}

func newMessage(m *parser.Message, proto3 bool, scope []string, types types) *pb.DescriptorProto {
	b := &messageBuilder{
		proto3: proto3,
		scope:  append(scope, m.Name),
		types:  types,
	}
	b.messageDesc = &pb.DescriptorProto{
		Name:           &m.Name,
		ExtensionRange: nil,
		ReservedRange:  nil,
		ReservedName:   nil,
	}
	for _, e := range m.Entries {
		b.addEntry(e)
	}
	return b.messageDesc
}

func (b *messageBuilder) addEntry(e *parser.MessageEntry) {
	switch {
	case e.Field != nil:
		b.buildField(e.Field)
	case e.Message != nil:
		m := newMessage(e.Message, b.proto3, b.scope, b.types)
		b.messageDesc.NestedType = append(b.messageDesc.NestedType, m)
	case e.Enum != nil:
		b.messageDesc.EnumType = append(b.messageDesc.EnumType, newEnum(e.Enum))
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
	typeEnum, typeName := newFieldDescriptorProtoType(pf.Direct.Type, b.scope, b.types)
	tag := int32(pf.Direct.Tag)
	df := &pb.FieldDescriptorProto{
		Name:           &pf.Direct.Name,
		Number:         &tag,
		JsonName:       jsonStr(pf.Direct.Name),
		Label:          fieldLabel(pf, b.proto3),
		Proto3Optional: proto3Optional(pf, b.proto3),

		Type:         &typeEnum,
		TypeName:     typeName,
		Extendee:     nil,
		DefaultValue: nil,
		OneofIndex:   nil,
		Options:      nil,
	}

	return df
}

func newFieldDescriptorProtoType(t *parser.Type, scope []string, types types) (pb.FieldDescriptorProto_Type, *string) {
	if t.Reference != nil {
		name, pbType := types.fullName(*t.Reference, scope)
		return pbType, &name
	}
	if t.Scalar != parser.None {
		return scalars[t.Scalar], nil
	}
	// maps
	panic("unimplemented type, probably map")
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

func newService(s *parser.Service, scope []string, types types) *pb.ServiceDescriptorProto {
	methods := []*pb.MethodDescriptorProto{}
	for _, e := range s.Entry {
		if e.Method != nil {
			method := newMethod(e.Method, scope, types)
			methods = append(methods, method)
		}
	}

	sd := &pb.ServiceDescriptorProto{
		Name:    &s.Name,
		Method:  methods,
		Options: nil,
	}

	return sd
}

func newMethod(m *parser.Method, scope []string, types types) *pb.MethodDescriptorProto {
	var clientStreaming, serverStreaming *bool
	if m.StreamingRequest {
		clientStreaming = &m.StreamingRequest
	}
	if m.StreamingResponse {
		serverStreaming = &m.StreamingResponse
	}
	inputEnum, inputType := newFieldDescriptorProtoType(m.Request, scope, types)
	if inputEnum != pb.FieldDescriptorProto_TYPE_MESSAGE {
		panic(fmt.Sprintf("%s: method %s should have Message as request type", m.Pos, m.Name))
	}
	outputEnum, outputType := newFieldDescriptorProtoType(m.Response, scope, types)
	if outputEnum != pb.FieldDescriptorProto_TYPE_MESSAGE {
		panic(fmt.Sprintf("%s: method %s should have Message as response type", m.Pos, m.Name))
	}
	md := &pb.MethodDescriptorProto{
		Name:            &m.Name,
		InputType:       inputType,
		OutputType:      outputType,
		Options:         nil,
		ClientStreaming: clientStreaming,
		ServerStreaming: serverStreaming,
	}

	return md
}

func newEnum(enum *parser.Enum) *pb.EnumDescriptorProto {
	var vals []*pb.EnumValueDescriptorProto
	var reservedNames []string
	var reservedRanges []*pb.EnumDescriptorProto_EnumReservedRange
	for _, e := range enum.Values {
		switch {
		case e.Value != nil:
			enumVal := newEnumValue(e.Value)
			vals = append(vals, enumVal)
		case e.Reserved != nil:
			er := newEnumRanges(e.Reserved)
			reservedRanges = append(reservedRanges, er...)
			reservedNames = append(reservedNames, e.Reserved.FieldNames...)
		case e.Option != nil:
			panic(fmt.Sprintf("%s: enum option not implemented", e.Pos))
		}
	}
	ed := &pb.EnumDescriptorProto{
		Name:          &enum.Name,
		Value:         vals,
		Options:       nil,
		ReservedRange: reservedRanges,
		ReservedName:  reservedNames,
	}
	return ed
}

func newEnumValue(e *parser.EnumValue) *pb.EnumValueDescriptorProto {
	val := int32(e.Value)
	ed := &pb.EnumValueDescriptorProto{
		Name:    &e.Key,
		Number:  &val,
		Options: nil,
	}
	return ed
}

func newEnumRanges(pr *parser.Reserved) []*pb.EnumDescriptorProto_EnumReservedRange {
	reservedRanges := make([]*pb.EnumDescriptorProto_EnumReservedRange, 0, len(pr.Ranges))
	for _, r := range pr.Ranges {
		start := int32(r.Start)
		er := &pb.EnumDescriptorProto_EnumReservedRange{
			Start: &start,
			End:   &start,
		}
		if r.End != nil {
			end := int32(*r.End)
			er.End = &end
		}
		if r.Max {
			var end int32 = math.MaxInt32
			er.End = &end
		}
		reservedRanges = append(reservedRanges, er)
	}
	return reservedRanges
}

func newOptions(o []*parser.Option) *pb.FileOptions {
	if o == nil {
		return nil
	}
	opts := &pb.FileOptions{}
	return opts
}

func newExtend(e *parser.Extend, scope []string, types types) *pb.FieldDescriptorProto {
	panic(fmt.Sprintf("not implemented: newExtend %v %v %v", e, scope, types))
}

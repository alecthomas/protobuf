package compiler

import (
	"fmt"
	"math"
	"math/big"
	"strconv"
	"strings"

	"github.com/alecthomas/protobuf/parser"
	pb "google.golang.org/protobuf/types/descriptorpb"
)

const maxReserved = int32(1 << 29)

type fileDescriptorBuilder struct {
	proto3   bool
	fileDesc *pb.FileDescriptorProto
	types    *types
	scope    []string
}

type messageBuilder struct {
	proto3      bool
	messageDesc *pb.DescriptorProto
	types       *types
	scope       []string

	proto3optionalFields []*pb.FieldDescriptorProto
}

type fieldBuilder struct {
	proto3 bool
	types  *types
	scope  []string

	oneofIndex *int32
	extendee   *string
}

func newFileDescriptor(ast *ast, types *types) *pb.FileDescriptorProto {
	var proto3 bool
	var syntax *string
	if ast.syntax == "proto3" {
		proto3 = true
		syntax = &ast.syntax
	}
	fd := &pb.FileDescriptorProto{
		Name:             &ast.file,
		Syntax:           syntax,
		Dependency:       ast.imports,
		PublicDependency: ast.publicImports,

		WeakDependency: nil,
	}
	b := fileDescriptorBuilder{
		proto3:   proto3,
		scope:    []string{},
		types:    types,
		fileDesc: fd,
	}

	if ast.pkg != "" {
		b.fileDesc.Package = &ast.pkg
		b.scope = append(b.scope, ast.pkg)
	}
	fd.Options = newFileOptions(ast.options, b.scope, b.types)

	for _, e := range ast.proto.Entries {
		b.addEntry(e)
	}
	return b.fileDesc
}

func newFileOptions(po []*parser.Option, scope []string, types *types) *pb.FileOptions {
	if len(po) == 0 {
		return nil
	}
	opts := &pb.FileOptions{}
	for _, o := range po {
		opt := newUninterpretedOption(o, scope, types)
		opts.UninterpretedOption = append(opts.UninterpretedOption, opt)
	}
	return opts
}

func (b *fileDescriptorBuilder) addEntry(e *parser.Entry) {
	fd := b.fileDesc
	switch {
	case e.Message != nil:
		md := newMessage(e.Message.Name, e.Message.Entries, b.proto3, b.scope, b.types)
		fd.MessageType = append(fd.MessageType, md)
	case e.Service != nil:
		sd := newService(e.Service, b.scope, b.types)
		fd.Service = append(fd.Service, sd)
	case e.Enum != nil:
		fd.EnumType = append(fd.EnumType, newEnum(e.Enum, b.scope, b.types))
	case e.Extend != nil:
		ed, groups := newExtend(e.Extend, b.proto3, b.scope, b.types)
		fd.Extension = append(fd.Extension, ed...)
		fd.MessageType = append(fd.MessageType, groups...)
	}
}

func newMessage(name string, entries []*parser.MessageEntry, proto3 bool, scope []string, types *types) *pb.DescriptorProto {
	b := &messageBuilder{
		proto3: proto3,
		scope:  append(scope, name),
		types:  types,
	}
	b.messageDesc = &pb.DescriptorProto{
		Name: &name,
	}
	for _, e := range entries {
		b.addEntry(e)
	}
	b.postProcessProto3Optional()
	return b.messageDesc
}

func (b *messageBuilder) addEntry(e *parser.MessageEntry) {
	md := b.messageDesc
	switch {
	case e.Field != nil:
		b.buildField(e.Field)
		if isMap(e.Field) {
			m := newMapEntry(e.Field, b.scope, b.types)
			md.NestedType = append(md.NestedType, m)
		}
	case e.Message != nil:
		m := newMessage(e.Message.Name, e.Message.Entries, b.proto3, b.scope, b.types)
		md.NestedType = append(md.NestedType, m)
	case e.Enum != nil:
		md.EnumType = append(md.EnumType, newEnum(e.Enum, b.scope, b.types))
	case e.Option != nil:
		b.buildOption(e.Option)
	case e.Oneof != nil:
		b.buildOneof(e.Oneof, md)
	case e.Extend != nil:
		extend, groups := newExtend(e.Extend, b.proto3, b.scope, b.types)
		md.Extension = append(md.Extension, extend...)
		md.NestedType = append(md.NestedType, groups...)
	case e.Reserved != nil:
		mr := newMessageRanges(e.Reserved)
		md.ReservedRange = append(md.ReservedRange, mr...)
		md.ReservedName = append(md.ReservedName, e.Reserved.FieldNames...)
	case e.Extensions != nil:
		er := newExtensionRanges(e.Extensions, b.scope, b.types)
		md.ExtensionRange = append(md.ExtensionRange, er...)
	default:
		panic(fmt.Sprintf("%s: cannot interpret MessageEntry", e.Pos))
	}
}

func (b *messageBuilder) buildOption(o *parser.Option) {
	if b.messageDesc.Options == nil {
		b.messageDesc.Options = &pb.MessageOptions{}
	}
	pbOpt := b.messageDesc.Options
	opt := newUninterpretedOption(o, b.scope, b.types)
	pbOpt.UninterpretedOption = append(pbOpt.UninterpretedOption, opt)
}

func newUninterpretedOption(o *parser.Option, scope []string, types *types) *pb.UninterpretedOption {
	opt := &pb.UninterpretedOption{}
	for _, optName := range o.Name {
		name := optName.Name
		isExtension := false
		if strings.HasPrefix(name, "(") {
			name = strings.TrimPrefix(name, "(")
			name = strings.TrimSuffix(name, ")")
			name = types.extensionName(name, scope)
			isExtension = true
		}
		o := &pb.UninterpretedOption_NamePart{NamePart: &name, IsExtension: &isExtension}
		opt.Name = append(opt.Name, o)
	}
	switch {
	case o.Value.String != nil:
		opt.StringValue = []byte(*o.Value.String)
	case o.Value.Number != nil && o.Value.Number.IsInt():
		if v, accuracy := o.Value.Number.Uint64(); accuracy == big.Exact {
			opt.PositiveIntValue = &v
		} else if v, accuracy := o.Value.Number.Int64(); accuracy == big.Exact {
			opt.NegativeIntValue = &v
		} else {
			panic(fmt.Sprintf("value to large for (u)int64: %v", *o.Value.Number))
		}
	case o.Value.Number != nil && !o.Value.Number.IsInt():
		v, _ := o.Value.Number.Float64()
		opt.DoubleValue = &v
	case o.Value.Bool != nil:
		v := strconv.FormatBool(bool(*o.Value.Bool))
		opt.IdentifierValue = &v
	case o.Value.Reference != nil:
		opt.IdentifierValue = o.Value.Reference
	case o.Value.ProtoText != nil:
		v := o.Value.ProtoText.String()
		opt.AggregateValue = &v
	default:
		// This includes o.Value.Array which does not appear to be valid.
		panic(fmt.Sprintf("Unknown option value form: %#v", o.Value))
	}
	return opt
}

func (b *messageBuilder) buildOneof(po *parser.OneOf, md *pb.DescriptorProto) {
	o := &pb.OneofDescriptorProto{
		Name:    &po.Name,
		Options: nil,
	}
	oneofIndex := int32(len(b.messageDesc.OneofDecl))
	fdBuilder := fieldBuilder{
		proto3:     b.proto3,
		scope:      b.scope,
		types:      b.types,
		oneofIndex: &oneofIndex,
	}
	b.messageDesc.OneofDecl = append(b.messageDesc.OneofDecl, o)
	for _, e := range po.Entries {
		switch {
		case e.Field != nil:
			fieldDesc := fdBuilder.createField(e.Field)
			b.messageDesc.Field = append(b.messageDesc.Field, fieldDesc)
			if group := e.Field.Group; group != nil {
				m := newMessage(group.Name, group.Entries, b.proto3, b.scope, b.types)
				md.NestedType = append(md.NestedType, m)
			}

		case e.Option != nil:
			if o.Options == nil {
				o.Options = &pb.OneofOptions{}
			}
			opt := newUninterpretedOption(e.Option, b.scope, b.types)
			o.Options.UninterpretedOption = append(o.Options.UninterpretedOption, opt)
		default:
			panic(fmt.Sprintf("%s: cannot interpret OneofEntry", e.Pos))
		}
	}
}

func newMessageRanges(pr *parser.Reserved) []*pb.DescriptorProto_ReservedRange {
	reservedRanges := make([]*pb.DescriptorProto_ReservedRange, 0, len(pr.Ranges))
	for _, r := range pr.Ranges {
		start, end := reservedRange(r)
		rr := &pb.DescriptorProto_ReservedRange{Start: &start, End: &end}
		reservedRanges = append(reservedRanges, rr)
	}
	return reservedRanges
}

func newExtensionRanges(er *parser.Extensions, scope []string, types *types) []*pb.DescriptorProto_ExtensionRange {
	extensionRanges := make([]*pb.DescriptorProto_ExtensionRange, 0, len(er.Extensions))
	ero := newExtensionRangeOptions(er.Options, scope, types)
	for _, r := range er.Extensions {
		start, end := reservedRange(r)
		rr := &pb.DescriptorProto_ExtensionRange{
			Start:   &start,
			End:     &end,
			Options: ero,
		}
		extensionRanges = append(extensionRanges, rr)
	}
	return extensionRanges
}

func reservedRange(r *parser.Range) (start int32, end int32) {
	start = int32(r.Start)
	end = int32(r.Start) + 1
	if r.End != nil {
		end = int32(*r.End) + 1
	}
	if r.Max {
		end = maxReserved
	}
	return start, end
}

func newExtensionRangeOptions(po []*parser.Option, scope []string, types *types) *pb.ExtensionRangeOptions {
	if len(po) == 0 {
		return nil
	}
	opts := &pb.ExtensionRangeOptions{}
	for _, o := range po {
		opt := newUninterpretedOption(o, scope, types)
		opts.UninterpretedOption = append(opts.UninterpretedOption, opt)
	}
	return opts
}

func (b *messageBuilder) buildField(pField *parser.Field) {
	fdBuilder := fieldBuilder{proto3: b.proto3, scope: b.scope, types: b.types}
	fieldDesc := fdBuilder.createField(pField)
	md := b.messageDesc
	md.Field = append(md.Field, fieldDesc)
	if group := pField.Group; group != nil {
		m := newMessage(group.Name, group.Entries, b.proto3, b.scope, b.types)
		md.NestedType = append(md.NestedType, m)
	}
	if b.proto3 && fieldDesc.Proto3Optional != nil && *fieldDesc.Proto3Optional {
		b.proto3optionalFields = append(b.proto3optionalFields, fieldDesc)
	}
}

func (b *messageBuilder) postProcessProto3Optional() {
	for _, fd := range b.proto3optionalFields {
		idx := int32(len(b.messageDesc.OneofDecl))
		fd.OneofIndex = &idx
		name := "_" + *fd.Name
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

func (b *fieldBuilder) createField(pField *parser.Field) *pb.FieldDescriptorProto {
	fType, typeName := fieldType(pField, b.scope, b.types)
	name := fieldName(pField)
	options, defaultValue := newFieldOptions(pField, b.scope, b.types)
	df := &pb.FieldDescriptorProto{
		Name:           &name,
		Number:         fieldTag(pField),
		JsonName:       jsonStr(name),
		Label:          fieldLabel(pField, b.proto3, b.oneofIndex != nil),
		Proto3Optional: proto3Optional(pField, b.proto3),

		Type:         &fType, // Message, Enum, Group, string, int32,...
		TypeName:     typeName,
		Extendee:     b.extendee,
		DefaultValue: defaultValue,
		OneofIndex:   b.oneofIndex,
		Options:      options,
	}

	return df
}

func fieldName(f *parser.Field) string {
	if f.Direct != nil {
		return f.Direct.Name
	}
	if f.Group != nil {
		return strings.ToLower(f.Group.Name)
	}
	panic(fmt.Sprintf("%s: fieldName: no direct or group", f.Pos))
}

func fieldTag(f *parser.Field) *int32 {
	var tag int32
	switch {
	case f.Direct != nil:
		tag = int32(f.Direct.Tag)
	case f.Group != nil:
		tag = int32(f.Group.Tag)
	default:
		panic(fmt.Sprintf("%s: fieldTag: no direct or group", f.Pos))
	}
	return &tag
}

func fieldType(f *parser.Field, scope []string, types *types) (pb.FieldDescriptorProto_Type, *string) {
	switch {
	case isMap(f):
		name, pbType := types.fullName(mapTypeStr(f.Direct.Name), scope)
		return pbType, &name
	case f.Direct != nil:
		fType, name := newFieldDescriptorProtoType(f.Direct.Type, scope, types)
		if fType == pb.FieldDescriptorProto_TYPE_GROUP {
			// references to groups are stored as Messages 🤷‍♀️
			fType = pb.FieldDescriptorProto_TYPE_MESSAGE
		}
		return fType, name
	case f.Group != nil:
		name, pbType := types.fullName(f.Group.Name, scope)
		return pbType, &name
	default:
		panic(fmt.Sprintf("%s: fieldType: no direct or group", f.Pos))
	}
}

func newFieldDescriptorProtoType(t *parser.Type, scope []string, types *types) (pb.FieldDescriptorProto_Type, *string) {
	if t.Scalar != parser.None {
		return scalars[t.Scalar], nil
	}
	if t.Reference != nil {
		name, pbType := types.fullName(*t.Reference, scope)
		return pbType, &name
	}
	panic("unimplemented type, probably map")
}

func newFieldOptions(field *parser.Field, scope []string, types *types) (*pb.FieldOptions, *string) {
	var options parser.Options
	if field.Direct != nil && len(field.Direct.Options) > 0 {
		options = field.Direct.Options
	} else if field.Group != nil && len(field.Group.Options) > 0 {
		options = field.Group.Options
	}
	if len(options) == 0 {
		return nil, nil
	}

	var defaultValue *string
	opts := &pb.FieldOptions{}
	for _, o := range options {
		// The "default" option for a field is handled differently than
		// other options - DefaultValue is a field in the FieldDescriptor
		if o.Name[0].Name == "default" {
			dv := o.Value.ToString()
			defaultValue = &dv
			continue
		}
		opt := newUninterpretedOption(o, scope, types)
		opts.UninterpretedOption = append(opts.UninterpretedOption, opt)
	}

	if len(opts.UninterpretedOption) == 0 {
		return nil, defaultValue
	}
	return opts, defaultValue
}

func newMapEntry(f *parser.Field, scope []string, types *types) *pb.DescriptorProto {
	keyField := MapEntryField("key", 1, f.Direct.Type.Map.Key, scope, types)
	valueField := MapEntryField("value", 2, f.Direct.Type.Map.Value, scope, types)
	name := mapTypeStr(f.Direct.Name)
	isMapEntry := true

	return &pb.DescriptorProto{
		Name:    &name,
		Options: &pb.MessageOptions{MapEntry: &isMapEntry},
		Field:   []*pb.FieldDescriptorProto{keyField, valueField},
	}
}

func MapEntryField(name string, number int32, t *parser.Type, scope []string, types *types) *pb.FieldDescriptorProto {
	label := pb.FieldDescriptorProto_LABEL_OPTIONAL
	fType, typeName := newFieldDescriptorProtoType(t, scope, types)
	return &pb.FieldDescriptorProto{
		Name:     &name,
		Number:   &number,
		JsonName: jsonStr(name),
		Label:    &label,
		Type:     &fType,
		TypeName: typeName,
	}
}

func isMap(f *parser.Field) bool {
	return f != nil && f.Direct != nil && f.Direct.Type != nil && f.Direct.Type.Map != nil
}

func fieldLabel(pf *parser.Field, proto3, oneof bool) *pb.FieldDescriptorProto_Label {
	var label pb.FieldDescriptorProto_Label
	switch {
	case pf.Required:
		label = pb.FieldDescriptorProto_LABEL_REQUIRED
	case pf.Repeated:
		label = pb.FieldDescriptorProto_LABEL_REPEATED
	case pf.Optional:
		label = pb.FieldDescriptorProto_LABEL_OPTIONAL
	case oneof:
		// oneof fields are unlabelled in proto2 and proto3
		label = pb.FieldDescriptorProto_LABEL_OPTIONAL
	case isMap(pf):
		// map<key, val> fields are unlabelled in proto2 and proto3
		label = pb.FieldDescriptorProto_LABEL_REPEATED
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
	result := ss[0]
	for _, s := range ss[1:] {
		result += strings.Title(s) //nolint:staticcheck // s is ascii not utf8
	}
	return &result
}

func mapTypeStr(s string) string {
	ss := strings.Split(s, "_")
	result := ""
	for _, s := range ss {
		result += strings.Title(s) //nolint:staticcheck // s is ascii not utf8
	}
	return result + "Entry"
}

func newService(s *parser.Service, scope []string, types *types) *pb.ServiceDescriptorProto {
	sd := &pb.ServiceDescriptorProto{Name: &s.Name}
	for _, e := range s.Entry {
		if e.Method != nil {
			sd.Method = append(sd.Method, newMethod(e.Method, scope, types))
		} else if e.Option != nil {
			buildServiceOption(sd, e.Option, scope, types)
		}
	}
	return sd
}

func newMethod(m *parser.Method, scope []string, types *types) *pb.MethodDescriptorProto {
	var clientStreaming, serverStreaming *bool
	if m.StreamingRequest {
		clientStreaming = &m.StreamingRequest
	}
	if m.StreamingResponse {
		serverStreaming = &m.StreamingResponse
	}
	inputType, inputTypeName := newFieldDescriptorProtoType(m.Request, scope, types)
	if inputType != pb.FieldDescriptorProto_TYPE_MESSAGE {
		panic(fmt.Sprintf("%s: method %s should have Message as request type", m.Pos, m.Name))
	}
	outputType, outputTypeName := newFieldDescriptorProtoType(m.Response, scope, types)
	if outputType != pb.FieldDescriptorProto_TYPE_MESSAGE {
		panic(fmt.Sprintf("%s: method %s should have Message as response type", m.Pos, m.Name))
	}
	md := &pb.MethodDescriptorProto{
		Name:            &m.Name,
		InputType:       inputTypeName,
		OutputType:      outputTypeName,
		Options:         newMethodOptions(m.Options, scope, types),
		ClientStreaming: clientStreaming,
		ServerStreaming: serverStreaming,
	}

	return md
}

func newMethodOptions(po []*parser.Option, scope []string, types *types) *pb.MethodOptions {
	if len(po) == 0 {
		return nil
	}
	opts := &pb.MethodOptions{}
	for _, o := range po {
		opt := newUninterpretedOption(o, scope, types)
		opts.UninterpretedOption = append(opts.UninterpretedOption, opt)
	}
	return opts
}

func buildServiceOption(sd *pb.ServiceDescriptorProto, o *parser.Option, scope []string, types *types) {
	if sd.Options == nil {
		sd.Options = &pb.ServiceOptions{}
	}
	opt := newUninterpretedOption(o, scope, types)
	sd.Options.UninterpretedOption = append(sd.Options.UninterpretedOption, opt)
}

func newEnum(enum *parser.Enum, scope []string, types *types) *pb.EnumDescriptorProto {
	ed := &pb.EnumDescriptorProto{Name: &enum.Name}
	for _, e := range enum.Values {
		switch {
		case e.Value != nil:
			ed.Value = append(ed.Value, newEnumValue(e.Value, scope, types))
		case e.Reserved != nil:
			er := newEnumRanges(e.Reserved)
			ed.ReservedRange = append(ed.ReservedRange, er...)
			ed.ReservedName = append(ed.ReservedName, e.Reserved.FieldNames...)
		case e.Option != nil:
			buildEnumOption(ed, e.Option, scope, types)
		}
	}
	return ed
}

func buildEnumOption(ed *pb.EnumDescriptorProto, o *parser.Option, scope []string, types *types) {
	if ed.Options == nil {
		ed.Options = &pb.EnumOptions{}
	}
	// allow_alias needs to be interpreted now as the uninterpreted option
	// unmarshaling needs to know if aliases on enums are allowed, otherwise
	// it will generate an error for aliases as allow_alias gets set too late.
	if len(o.Name) == 1 && o.Name[0].Name == "allow_alias" {
		ed.Options.AllowAlias = (*bool)(o.Value.Bool)
		return
	}
	opt := newUninterpretedOption(o, scope, types)
	ed.Options.UninterpretedOption = append(ed.Options.UninterpretedOption, opt)
}

func newEnumValue(e *parser.EnumValue, scope []string, types *types) *pb.EnumValueDescriptorProto {
	val := int32(e.Value)
	ed := &pb.EnumValueDescriptorProto{
		Name:   &e.Key,
		Number: &val,
	}
	if len(e.Options) != 0 {
		ed.Options = &pb.EnumValueOptions{}
		for _, o := range e.Options {
			opt := newUninterpretedOption(o, scope, types)
			ed.Options.UninterpretedOption = append(ed.Options.UninterpretedOption, opt)
		}
	}
	return ed
}

func newEnumRanges(pr *parser.Reserved) []*pb.EnumDescriptorProto_EnumReservedRange {
	reservedRanges := make([]*pb.EnumDescriptorProto_EnumReservedRange, 0, len(pr.Ranges))
	for _, r := range pr.Ranges {
		start := int32(r.Start)
		end := int32(r.Start)
		if r.End != nil {
			end = int32(*r.End)
		}
		if r.Max {
			end = math.MaxInt32
		}
		er := &pb.EnumDescriptorProto_EnumReservedRange{Start: &start, End: &end}
		reservedRanges = append(reservedRanges, er)
	}
	return reservedRanges
}

func newExtend(e *parser.Extend, proto3 bool, scope []string, types *types) (fields []*pb.FieldDescriptorProto, groups []*pb.DescriptorProto) {
	extendee, _ := types.fullName(e.Reference, scope)
	fdBuilder := fieldBuilder{proto3: proto3, scope: scope, types: types, extendee: &extendee}
	fds := make([]*pb.FieldDescriptorProto, len(e.Fields))
	var groupDescs []*pb.DescriptorProto
	for i, pf := range e.Fields {
		fds[i] = fdBuilder.createField(pf)
		if group := pf.Group; group != nil {
			groupDesc := newMessage(group.Name, group.Entries, proto3, scope, types)
			groupDescs = append(groupDescs, groupDesc)
		}
	}
	return fds, groupDescs
}

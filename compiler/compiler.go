// Package compiler creates FileDescriptorSets from a *.proto input and
// FileDescriptors for *parser.Proto input. It is assumed that the
// parse result is semantically correct and would compile with protoc.
package compiler

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"strings"

	"github.com/alecthomas/protobuf/parser"
	"google.golang.org/protobuf/encoding/prototext"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/reflect/protoregistry"
	pb "google.golang.org/protobuf/types/descriptorpb"
)

// ast is a convenience data structure on top of parser.Proto,
// classifying parser.Entry by their non-nil field and lifting out
// singular structures such as syntax or package.
type ast struct {
	proto         *parser.Proto
	file          string
	pkg           string
	imports       []string
	publicImports []int32
	syntax        string

	messages []*parser.Message
	services []*parser.Service
	enums    []*parser.Enum
	options  []*parser.Option
	extends  []*parser.Extend
}

// Compile creates a FileDescriptorSet similar to protoc:
//
// 		protoc -o filedescriptorset.pb -I importPath1 -I importPath2 --include_imports file1.proto file2.proto
//
// A FileDescriptorSet contains an array of FileDescriptorProtos, each
// of which a proto representation of the source proto files. The
// FileDescriptorSet is the intermediary representation typically
// passed to proto plugins.
func Compile(files, importPaths []string, includeImports bool) (*pb.FileDescriptorSet, error) {
	done := map[string]bool{}
	origFiles := map[string]bool{}
	for _, file := range files {
		origFiles[file] = true
	}
	asts, err := readProtos(files, importPaths, done)
	if err != nil {
		return nil, err
	}
	types := newTypes(asts)
	all := &pb.FileDescriptorSet{}
	filtered := &pb.FileDescriptorSet{}
	for _, a := range asts {
		fd := newFileDescriptor(a, types)
		all.File = append(all.File, fd)
		if includeImports || origFiles[a.file] {
			filtered.File = append(filtered.File, fd)
		}
	}
	err = resolveCustomOptions(all, filtered, types)
	if err != nil {
		return nil, err
	}
	return filtered, nil
}

func resolveCustomOptions(all, filtered *pb.FileDescriptorSet, types *types) error {
	reg, err := NewRegistry(all)
	if err != nil {
		return err
	}

	r := &scopedResolver{resolver: reg, types: types}

	for _, fd := range filtered.File {
		if err := resolveFileOptions(r, fd); err != nil {
			return err
		}
	}
	return nil
}

type resolver interface {
	protoregistry.ExtensionTypeResolver
	protoregistry.MessageTypeResolver
}

type scopedResolver struct {
	resolver
	scope []string
	types *types
}

func (sr *scopedResolver) FindExtensionByName(field protoreflect.FullName) (et protoreflect.ExtensionType, err error) {
	defer func() {
		if r := recover(); r != nil {
			et, err = nil, protoregistry.NotFound
		}
	}()
	// lookup full name and strip off leading "." (FindExtensionByName does
	// not want the leading dot)
	fullname := protoreflect.FullName(sr.types.extensionName(string(field), sr.scope)[1:])
	return sr.resolver.FindExtensionByName(fullname)
}

func (sr *scopedResolver) pushScopes(scopes ...string) {
	sr.scope = append(sr.scope, scopes...)
}

func (sr *scopedResolver) popScope() {
	sr.scope = sr.scope[:len(sr.scope)-1]
}

func (sr *scopedResolver) clearScopes() {
	sr.scope = []string{}
}

func resolveFileOptions(r *scopedResolver, fd *pb.FileDescriptorProto) error {
	if fd.GetPackage() != "" {
		r.pushScopes(strings.Split(fd.GetPackage(), ".")...)
	}

	if err := resolveUninterpretedOptions(r, fd.GetOptions()); err != nil {
		return err
	}
	for _, md := range fd.GetMessageType() {
		if err := resolveMessageOptions(r, md); err != nil {
			return err
		}
	}
	for _, ed := range fd.GetEnumType() {
		if err := resolveEnumOptions(r, ed); err != nil {
			return err
		}
	}
	for _, sd := range fd.GetService() {
		if err := resolveServiceOptions(r, sd); err != nil {
			return err
		}
	}
	for _, fd := range fd.GetExtension() {
		if err := resolveUninterpretedOptions(r, fd.GetOptions()); err != nil {
			return err
		}
	}
	r.clearScopes()
	return nil
}

func resolveMessageOptions(r *scopedResolver, md *pb.DescriptorProto) error {
	r.pushScopes(md.GetName())
	if err := resolveUninterpretedOptions(r, md.GetOptions()); err != nil {
		return err
	}
	for _, fd := range md.GetField() {
		if err := resolveUninterpretedOptions(r, fd.GetOptions()); err != nil {
			return err
		}
	}
	for _, fd := range md.GetExtension() {
		if err := resolveUninterpretedOptions(r, fd.GetOptions()); err != nil {
			return err
		}
	}
	for _, nestedMD := range md.GetNestedType() {
		if err := resolveMessageOptions(r, nestedMD); err != nil {
			return err
		}
	}
	for _, ed := range md.GetEnumType() {
		if err := resolveEnumOptions(r, ed); err != nil {
			return err
		}
	}
	for _, erd := range md.GetExtensionRange() {
		if err := resolveUninterpretedOptions(r, erd.GetOptions()); err != nil {
			return err
		}
	}
	for _, od := range md.GetOneofDecl() {
		if err := resolveUninterpretedOptions(r, od.GetOptions()); err != nil {
			return err
		}
	}
	r.popScope()
	return nil
}

func resolveEnumOptions(r resolver, ed *pb.EnumDescriptorProto) error {
	if err := resolveUninterpretedOptions(r, ed.GetOptions()); err != nil {
		return err
	}
	for _, evd := range ed.GetValue() {
		if err := resolveUninterpretedOptions(r, evd.GetOptions()); err != nil {
			return err
		}
	}
	return nil
}

func resolveServiceOptions(r resolver, sd *pb.ServiceDescriptorProto) error {
	if err := resolveUninterpretedOptions(r, sd.GetOptions()); err != nil {
		return err
	}
	for _, md := range sd.GetMethod() {
		if err := resolveUninterpretedOptions(r, md.GetOptions()); err != nil {
			return err
		}
	}
	return nil
}

type messageWithOptions interface {
	proto.Message
	GetUninterpretedOption() []*pb.UninterpretedOption
}

func resolveUninterpretedOptions(r resolver, opts messageWithOptions) error {
	if opts == nil || reflect.ValueOf(opts).IsNil() {
		return nil
	}

	for _, opt := range opts.GetUninterpretedOption() {
		msg, fd := getLastField(opts.ProtoReflect(), opt.GetName(), r)
		setField(msg, fd, opt, r)
	}

	// Use reflection to set the UninterpretedOption field to nil
	v := reflect.ValueOf(opts).Elem().FieldByName("UninterpretedOption")
	if !v.IsZero() {
		v.Set(reflect.Zero(v.Type()))
	}

	return nil
}

// getLastField returns a message and a field descriptor for the last field
// for an option value. Options are specified in a .proto file as
// option field1.(pkg.field2).field3.(field4) = <some-value>. getLastField
// will return the message of type field3 and a field descriptor for field4
// so that it can be set <some-value>.
func getLastField(msg protoreflect.Message, nameparts []*pb.UninterpretedOption_NamePart, r resolver) (protoreflect.Message, protoreflect.FieldDescriptor) {
	var fd protoreflect.FieldDescriptor

	for i, np := range nameparts {
		name := protoreflect.Name(np.GetNamePart())
		if np.GetIsExtension() {
			name := protoreflect.FullName(np.GetNamePart())
			et, err := r.FindExtensionByName(name[1:]) // does not like leading "."
			if err != nil {
				err := fmt.Errorf("unknown extension in option: %s", name)
				panic(err)
			}
			fd = et.TypeDescriptor()
		} else {
			if fd = msg.Descriptor().Fields().ByName(name); fd == nil {
				err := fmt.Errorf("unknown field name in option: %s", name)
				panic(err)
			}
		}
		// All but the last namepart must be a message, so get a mutable message
		// for the field (possibly from a list of messages) for the next level
		// of iteration.
		if i != len(nameparts)-1 {
			v := msg.Mutable(fd)
			if fd.IsList() {
				v = v.List().NewElement()
			}
			msg = v.Message()
		}
	}

	return msg, fd
}

func setField(msg protoreflect.Message, fd protoreflect.FieldDescriptor, val *pb.UninterpretedOption, r resolver) {
	var v protoreflect.Value
	switch fd.Kind() {
	case protoreflect.BoolKind:
		v = valueOfBool(val)
	case protoreflect.Int32Kind, protoreflect.Sint32Kind, protoreflect.Sfixed32Kind:
		v = valueOfInt32(val)
	case protoreflect.Int64Kind, protoreflect.Sint64Kind, protoreflect.Sfixed64Kind:
		v = valueOfInt64(val)
	case protoreflect.Uint32Kind, protoreflect.Fixed32Kind:
		v = valueOfUint32(val)
	case protoreflect.Uint64Kind, protoreflect.Fixed64Kind:
		v = valueOfUint64(val)
	case protoreflect.FloatKind:
		v = valueOfFloat32(val)
	case protoreflect.DoubleKind:
		v = valueOfFloat64(val)
	case protoreflect.StringKind:
		v = valueOfString(val)
	case protoreflect.BytesKind:
		v = valueOfBytes(val)
	case protoreflect.EnumKind:
		v = valueOfEnum(val, fd)
	case protoreflect.MessageKind, protoreflect.GroupKind:
		mval := msg.NewField(fd)
		if fd.IsList() {
			mval = mval.List().NewElement()
		}
		v = valueOfMessage(val, mval.Message().Interface(), r)
	}

	if !v.IsValid() {
		err := fmt.Errorf("%s: cannot make %s from %s", fd.FullName(), fd.Kind(), val)
		panic(err)
	}

	// We don't need to worry about maps as they cannot be extension fields.
	if fd.IsList() {
		msg.Mutable(fd).List().Append(v)
	} else {
		msg.Set(fd, v)
	}
}

func valueOfBool(val *pb.UninterpretedOption) protoreflect.Value {
	var v bool
	switch {
	case val.IdentifierValue != nil && *val.IdentifierValue == "true":
		v = true
	case val.IdentifierValue != nil && *val.IdentifierValue == "false":
		v = false
	default:
		return protoreflect.Value{}
	}
	return protoreflect.ValueOfBool(v)
}

func valueOfInt32(val *pb.UninterpretedOption) protoreflect.Value {
	var v int32
	switch {
	case val.PositiveIntValue != nil:
		v = int32(*val.PositiveIntValue)
	case val.NegativeIntValue != nil:
		v = int32(*val.NegativeIntValue)
	default:
		return protoreflect.Value{}
	}
	return protoreflect.ValueOfInt32(v)
}

func valueOfInt64(val *pb.UninterpretedOption) protoreflect.Value {
	var v int64
	switch {
	case val.PositiveIntValue != nil:
		v = int64(*val.PositiveIntValue)
	case val.NegativeIntValue != nil:
		v = *val.NegativeIntValue
	default:
		return protoreflect.Value{}
	}
	return protoreflect.ValueOfInt64(v)
}

func valueOfUint32(val *pb.UninterpretedOption) protoreflect.Value {
	var v uint32
	switch {
	case val.PositiveIntValue != nil:
		v = uint32(*val.PositiveIntValue)
	case val.NegativeIntValue != nil:
		v = uint32(*val.NegativeIntValue)
	default:
		return protoreflect.Value{}
	}
	return protoreflect.ValueOfUint32(v)
}

func valueOfUint64(val *pb.UninterpretedOption) protoreflect.Value {
	var v uint64
	switch {
	case val.PositiveIntValue != nil:
		v = *val.PositiveIntValue
	case val.NegativeIntValue != nil:
		v = uint64(*val.NegativeIntValue)
	default:
		return protoreflect.Value{}
	}
	return protoreflect.ValueOfUint64(v)
}

func valueOfFloat32(val *pb.UninterpretedOption) protoreflect.Value {
	var v float32
	switch {
	case val.DoubleValue != nil:
		v = float32(*val.DoubleValue)
	case val.PositiveIntValue != nil:
		v = float32(*val.PositiveIntValue)
	case val.NegativeIntValue != nil:
		v = float32(*val.NegativeIntValue)
	default:
		return protoreflect.Value{}
	}
	return protoreflect.ValueOfFloat32(v)
}

func valueOfFloat64(val *pb.UninterpretedOption) protoreflect.Value {
	var v float64
	switch {
	case val.DoubleValue != nil:
		v = *val.DoubleValue
	case val.PositiveIntValue != nil:
		v = float64(*val.PositiveIntValue)
	case val.NegativeIntValue != nil:
		v = float64(*val.NegativeIntValue)
	default:
		return protoreflect.Value{}
	}
	return protoreflect.ValueOfFloat64(v)
}

func valueOfString(val *pb.UninterpretedOption) protoreflect.Value {
	if val.StringValue == nil {
		return protoreflect.Value{}
	}
	return protoreflect.ValueOfString(string(val.StringValue))
}

func valueOfBytes(val *pb.UninterpretedOption) protoreflect.Value {
	if val.StringValue == nil {
		return protoreflect.Value{}
	}
	return protoreflect.ValueOfBytes(val.StringValue)
}

func valueOfEnum(val *pb.UninterpretedOption, fd protoreflect.FieldDescriptor) protoreflect.Value {
	var v protoreflect.EnumNumber
	switch {
	case val.PositiveIntValue != nil:
		v = protoreflect.EnumNumber(*val.PositiveIntValue)
	case val.NegativeIntValue != nil:
		v = protoreflect.EnumNumber(*val.NegativeIntValue)
	case val.IdentifierValue != nil:
		e := fd.Enum().Values().ByName(protoreflect.Name(*val.IdentifierValue))
		if e == nil {
			return protoreflect.Value{}
		}
		v = e.Number()
	default:
		return protoreflect.Value{}
	}
	return protoreflect.ValueOfEnum(v)
}

func valueOfMessage(val *pb.UninterpretedOption, m proto.Message, r resolver) protoreflect.Value {
	if val.AggregateValue == nil {
		return protoreflect.Value{}
	}
	o := prototext.UnmarshalOptions{Resolver: r}
	if err := o.Unmarshal([]byte(*val.AggregateValue), m); err != nil {
		panic(err)
	}
	return protoreflect.ValueOfMessage(m.ProtoReflect())
}

// readProtos creates ASTs for given files and their dependencies in
// order of the original files slice. By contrast, readProto creates an
// AST for a given file and the ASTs of its dependencies, listed before
// the file AST. Multiple dependencies(imports) listed in the same
// proto file are again processed in order by readProtos(). Already
// processed files are not double listed.
//
// For example:
// * f1 imports f2 which imports f3
// * g1 imports g2, g3, f3
// readProtos([]string{f1, g1}, ...)
// results in the following order: f3, f2, f1, g2, g3, g1
func readProtos(files, importPaths []string, done map[string]bool) ([]*ast, error) {
	asts := make([]*ast, 0, len(files))
	for _, file := range files {
		nextASTs, err := readProto(file, importPaths, done)
		if err != nil {
			return nil, err
		}
		asts = append(asts, nextASTs...)
	}
	return asts, nil
}

// readProto creates an AST for a given file and the ASTs of its
// dependencies, listed before the file AST.
//
// see readProtos for more details.
func readProto(file string, importPaths []string, done map[string]bool) ([]*ast, error) {
	if done[file] {
		return nil, nil
	}
	done[file] = true
	ast, err := newASTFromPath(file, importPaths)
	if err != nil {
		return nil, err
	}
	importedASTs, err := readProtos(ast.imports, importPaths, done)
	if err != nil {
		return nil, err
	}
	importedASTs = append(importedASTs, ast)
	return importedASTs, nil
}

func newAST(file string, r io.Reader) (*ast, error) {
	proto, err := parser.Parse(file, r)
	if err != nil {
		return nil, fmt.Errorf("compile %s: %w", file, err)
	}
	a := &ast{
		file:   file,
		proto:  proto,
		syntax: proto.Syntax,
	}
	for _, e := range proto.Entries {
		switch {
		case e.Package != "":
			a.pkg = e.Package
		case e.Import != nil:
			a.imports = append(a.imports, e.Import.Name)
			if e.Import.Public {
				a.publicImports = append(a.publicImports, int32(len(a.imports))-1)
			}
		case e.Message != nil:
			a.messages = append(a.messages, e.Message)
		case e.Enum != nil:
			a.enums = append(a.enums, e.Enum)
		case e.Service != nil:
			a.services = append(a.services, e.Service)
		case e.Option != nil:
			a.options = append(a.options, e.Option)
		case e.Extend != nil:
			a.extends = append(a.extends, e.Extend)
		case e.Comment != nil:
			// Ignore comments for now
		default:
			return nil, fmt.Errorf("%s: cannot interpret Entry", e.Pos)
		}
	}
	return a, nil
}

func newASTFromPath(file string, importPaths []string) (*ast, error) {
	r, err := search(file, importPaths)
	if err != nil {
		return nil, err
	}
	defer r.Close()
	return newAST(file, r)
}

func search(file string, importPaths []string) (io.ReadCloser, error) {
	for _, path := range importPaths {
		fname := filepath.Join(path, file)
		f, err := os.Open(fname)
		if err == nil {
			return f, nil
		}
		if !errors.Is(err, os.ErrNotExist) {
			return nil, fmt.Errorf("unexpected error trying to open %q: %w", file, err)
		}
	}
	return nil, fmt.Errorf("cannot find %q on import paths", file)
}

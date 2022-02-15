// Package compiler creates FileDescriptorSets from a *.proto input and
// FileDescriptors for *parser.Proto input. It is assumed that the
// parse result is semantically correct and would compile with protoc.
package compiler

import (
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/alecthomas/protobuf/parser"
	"google.golang.org/protobuf/encoding/prototext"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/reflect/protoregistry"
	pb "google.golang.org/protobuf/types/descriptorpb"
	"google.golang.org/protobuf/types/dynamicpb"
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

// NewFileDescriptorSet creates a FileDescriptorSet similar to protoc:
//
// 		protoc -o filedescriptorset.pb -I importPath1 -I importPath2 --include_imports file1.proto file2.proto
//
// A FileDescriptorSet contains an array of FileDescriptorProtos, each
// of which a proto representation of the source proto files. The
// FileDescriptorSet is the intermediary representation typically
// passed to proto plugins.
func NewFileDescriptorSet(files, importPaths []string, includeImports bool) (*pb.FileDescriptorSet, error) {
	all, filtered, err := compileFileDescriptorSets(files, importPaths, includeImports)
	if err != nil {
		return nil, err
	}
	err = resolveCustomOptions(all, filtered)
	if err != nil {
		return nil, err
	}
	return filtered, nil
}

// compileFileDescriptorSets returns a FileDescriptorSet of all given
// proto files _and_ their transitive dependencies as well as a
// FileDescriptorSet of only the given proto files without their
// transitive dependencies if includeImports is if false. If
// includeImports is true all and filter FileDescriptorSets are identical.
func compileFileDescriptorSets(files, importPaths []string, includeImports bool) (all *pb.FileDescriptorSet, filtered *pb.FileDescriptorSet, err error) {
	done := map[string]bool{}
	origFiles := map[string]bool{}
	for _, file := range files {
		origFiles[file] = true
	}
	asts, err := readProtos(files, importPaths, done)
	if err != nil {
		return nil, nil, err
	}
	types := newTypes(asts)
	all = &pb.FileDescriptorSet{}
	filtered = &pb.FileDescriptorSet{}
	for _, a := range asts {
		fd := newFileDescriptor(a, types)
		all.File = append(all.File, fd)
		if includeImports || origFiles[a.file] {
			filtered.File = append(filtered.File, fd)
		}
	}
	return all, filtered, nil
}

func resolveCustomOptions(all, filtered *pb.FileDescriptorSet) error {
	files, err := NewFiles(all)
	if err != nil {
		return err
	}
	for _, fd := range filtered.File {
		for _, md := range fd.GetMessageType() {
			opts := md.GetOptions()
			if opts == nil {
				continue
			}
			for _, opt := range opts.GetUninterpretedOption() {
				name := fullName(opt.Name)
				desc, err := files.FindDescriptorByName(name)
				if err != nil {
					return err
				}
				ed, ok := desc.(protoreflect.ExtensionDescriptor)
				if !ok {
					return fmt.Errorf("expected ExtensionDescriptor, got %T :%w", desc, protoregistry.NotFound)
				}
				// TODO: extends for extension fields that are not Messages (e.g. scalars)
				message := dynamicpb.NewMessage(ed.Message())

				m := prototext.UnmarshalOptions{
					Resolver: files,
				}
				err = m.Unmarshal([]byte(*opt.AggregateValue), message)
				if err != nil {
					return err
				}
				proto.SetExtension(opts, dynamicpb.NewExtensionType(ed), message)
			}
			opts.UninterpretedOption = nil
		}
	}
	return nil
}

func fullName(names []*pb.UninterpretedOption_NamePart) protoreflect.FullName {
	// TODO: extend for field references and extensions of extensions.
	str := protoreflect.FullName((*names[0].NamePart)[1:])
	return str
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
	return newAST(file, r)
}

func search(file string, importPaths []string) (io.ReadCloser, error) {
	for _, path := range importPaths {
		fname := filepath.Join(path, file)
		if f, err := os.Open(fname); err == nil {
			return f, nil
		}
	}
	return nil, fmt.Errorf("cannot find %q on import paths", file)
}

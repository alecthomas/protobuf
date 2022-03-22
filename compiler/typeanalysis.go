package compiler

import (
	"fmt"
	"strings"

	"github.com/alecthomas/protobuf/parser"
	pb "google.golang.org/protobuf/types/descriptorpb"
)

// types contains all known proto custom types with their fully
// qualified name (fullname) and serves as a lookup table to get
// fully qualified names for relative names and given scope.
//
//   fullname := .[pkgpart.]*[type.]*[type]
//
// For example:
//
//    package pkg1.pkg2;
//    message Nest {
//      repeated Egg eggs = 1;
//      repeated Egg2 eggs2 = 2;
//      message Egg {
//        optional string chick = 1;
//      }
//    }
//    message Egg2 {
//      optional string duckling = 1;
//    }
//
// The fullname type of eggs and eggs2 would be returned by
//
//     types.fullName("Egg", {"pkg1.pkg2", "Nest"}): .pkg1.pkg2.Nest.Egg
//     types.fullName("Egg2", {"pkg1.pkg2", "Nest"}): .pkg1.pkg2.Egg2
//
// types.fullName returns an empty string if no type can be found.
type types struct {
	types      map[string]pb.FieldDescriptorProto_Type
	extensions map[string]bool
}

func (t *types) fullName(typeName string, scope []string) (string, pb.FieldDescriptorProto_Type) {
	if strings.HasPrefix(typeName, ".") {
		if pbType, ok := t.types[typeName]; ok {
			return typeName, pbType
		}
		panic(fmt.Sprintf("typeanalysis: not found: %s, %v", typeName, scope))
	}
	for i := len(scope); i >= 0; i-- {
		sn := scopedName(typeName, scope[:i])
		if pbType, ok := t.types[sn]; ok {
			return sn, pbType
		}
	}
	panic(fmt.Sprintf("typeanalysis: not found: %s, %v, %v", typeName, scope, t))
}

func (t *types) extensionName(name string, scope []string) string {
	if strings.HasPrefix(name, ".") {
		if t.extensions[name] {
			return name
		}
		panic(fmt.Sprintf("typeanalysis: not found: %s, %v", name, scope))
	}
	for i := len(scope); i >= 0; i-- {
		sn := scopedName(name, scope[:i])
		if t.extensions[sn] {
			return sn
		}
	}
	panic(fmt.Sprintf("typeanalysis: not found: %s, %v", name, scope))
}

func (t *types) addName(relTypeName string, pbType pb.FieldDescriptorProto_Type, scope []string) {
	sn := scopedName(relTypeName, scope)
	if _, ok := t.types[sn]; ok {
		panic(fmt.Sprintf("typeanalysis: duplicate type name: %s", sn))
	}
	t.types[sn] = pbType
}

func (t *types) addExtension(relName string, scope []string) {
	sn := scopedName(relName, scope)
	if _, ok := t.extensions[sn]; ok {
		panic(fmt.Sprintf("typeanalysis: duplicate extension: %s", sn))
	}
	t.extensions[sn] = true
}

func scopedName(name string, scope []string) string {
	sn := strings.Join(scope, ".") + "." + name
	if len(scope) > 0 {
		sn = "." + sn
	}
	return sn
}

func newTypes(asts []*ast) *types {
	t := &types{
		types:      map[string]pb.FieldDescriptorProto_Type{},
		extensions: map[string]bool{},
	}
	for _, ast := range asts {
		analyseTypes(ast, t)
	}
	return t
}

func analyseTypes(ast *ast, t *types) {
	scope := []string{}
	if ast.pkg != "" {
		scope = append(scope, ast.pkg)
	}

	for _, m := range ast.messages {
		analyseMessage(m, scope, t)
	}
	for _, e := range ast.enums {
		t.addName(e.Name, pb.FieldDescriptorProto_TYPE_ENUM, scope)
	}
	for _, e := range ast.extends {
		analyseExtend(e, scope, t)
	}
}

func analyseMessage(m *parser.Message, scope []string, t *types) {
	name := m.Name
	t.addName(name, pb.FieldDescriptorProto_TYPE_MESSAGE, scope)
	scope = append(scope, name)
	analyseMessageEntries(m.Entries, scope, t)
}

func analyseGroup(g *parser.Group, scope []string, t *types) {
	name := g.Name
	t.addName(name, pb.FieldDescriptorProto_TYPE_GROUP, scope)
	scope = append(scope, name)
	analyseMessageEntries(g.Entries, scope, t)
}

func analyseExtend(e *parser.Extend, scope []string, t *types) {
	for _, f := range e.Fields {
		if f.Group != nil {
			analyseGroup(f.Group, scope, t)
			t.addExtension(strings.ToLower(f.Group.Name), scope)
		} else if f.Direct != nil {
			t.addExtension(f.Direct.Name, scope)
		}
	}
}

func analyseField(f *parser.Field, scope []string, t *types) {
	if f.Group != nil {
		analyseGroup(f.Group, scope, t)
		return
	}
	if f.Direct != nil && f.Direct.Type != nil && f.Direct.Type.Map != nil {
		mapType := mapTypeStr(f.Direct.Name)
		t.addName(mapType, pb.FieldDescriptorProto_TYPE_MESSAGE, scope)
	}
}

func analyseMessageEntries(messageEntries []*parser.MessageEntry, scope []string, t *types) {
	for _, me := range messageEntries {
		switch {
		case me.Message != nil:
			analyseMessage(me.Message, scope, t)
		case me.Enum != nil:
			t.addName(me.Enum.Name, pb.FieldDescriptorProto_TYPE_ENUM, scope)
		case me.Extend != nil:
			analyseExtend(me.Extend, scope, t)
		case me.Field != nil:
			analyseField(me.Field, scope, t)
		case me.Oneof != nil:
			analyseOneof(me.Oneof, scope, t)
		}
	}
}

func analyseOneof(oneof *parser.OneOf, scope []string, t *types) {
	for _, oe := range oneof.Entries {
		if oe.Field != nil {
			analyseField(oe.Field, scope, t)
		}
	}
}

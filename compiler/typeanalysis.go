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
type types map[string]pb.FieldDescriptorProto_Type

func (t types) fullName(typeName string, scope []string) (string, pb.FieldDescriptorProto_Type) {
	if strings.HasPrefix(typeName, ".") {
		if t[typeName] != 0 {
			return typeName, t[typeName]
		}
		panic(fmt.Sprintf("typeanalysis: not found: %s, %v", typeName, scope))
	}
	for i := len(scope); i >= 0; i-- {
		parts := make([]string, i+1)
		copy(parts, scope[:i])
		parts[i] = typeName
		name := "." + strings.Join(parts, ".")
		if t[name] != 0 {
			return name, t[name]
		}
	}
	panic(fmt.Sprintf("typeanalysis: not found: %s, %v, %v", typeName, scope, t))
}

func (t types) addName(relTypeName string, pbType pb.FieldDescriptorProto_Type, scope []string) {
	parts := make([]string, len(scope)+1)
	copy(parts, scope)
	parts[len(parts)-1] = relTypeName
	name := "." + strings.Join(parts, ".")
	t[name] = pbType
}

func newTypes(asts []*ast) types {
	t := types{}
	for _, ast := range asts {
		analyseTypes(ast, t)
	}
	return t
}

func analyseTypes(ast *ast, t types) {
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

func analyseMessage(m *parser.Message, scope []string, t types) {
	name := m.Name
	t.addName(name, pb.FieldDescriptorProto_TYPE_MESSAGE, scope)
	scope = append(scope, name)
	analyseMessageEntries(m.Entries, scope, t)
}

func analyseGroup(g *parser.Group, scope []string, t types) {
	name := g.Name
	t.addName(name, pb.FieldDescriptorProto_TYPE_GROUP, scope)
	scope = append(scope, name)
	analyseMessageEntries(g.Entries, scope, t)
}

func analyseExtend(e *parser.Extend, scope []string, t types) {
	for _, f := range e.Fields {
		if f.Group != nil {
			analyseGroup(f.Group, scope, t)
		}
	}
}

func analyseField(f *parser.Field, scope []string, t types) {
	if f.Group != nil {
		analyseGroup(f.Group, scope, t)
		return
	}
	if f.Direct != nil && f.Direct.Type != nil && f.Direct.Type.Map != nil {
		mapType := mapTypeStr(f.Direct.Name)
		t.addName(mapType, pb.FieldDescriptorProto_TYPE_MESSAGE, scope)
	}
}

func analyseMessageEntries(messageEntries []*parser.MessageEntry, scope []string, t types) {
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
		}
	}
}

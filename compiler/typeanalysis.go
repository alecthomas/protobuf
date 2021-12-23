package compiler

import (
	"fmt"
	"strings"

	"github.com/alecthomas/protobuf/parser"
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

type types map[string]bool

func (t types) fullName(typeName string, scope []string) string {
	if strings.HasPrefix(typeName, ".") {
		if t[typeName] {
			return typeName
		}
		panic(fmt.Sprintf("typeanalysis: not found: %s, %v", typeName, scope))
	}
	for i := len(scope); i >= 0; i-- {
		parts := make([]string, i+1)
		copy(parts, scope[:i])
		parts[i] = typeName
		name := "." + strings.Join(parts, ".")
		if t[name] {
			return name
		}
	}
	panic(fmt.Sprintf("typeanalysis: not found: %s, %v", typeName, scope))
}

func (t types) addName(relTypeName string, scope []string) {
	parts := make([]string, len(scope)+1)
	copy(parts, scope)
	parts[len(parts)-1] = relTypeName
	name := "." + strings.Join(parts, ".")
	t[name] = true
}

func analyseTypes(p *parser.Proto) types {
	t := types{}
	scope := []string{}
	if pkg := protoPkg(p); pkg != "" {
		scope = append(scope, pkg)
	}

	for _, e := range p.Entries {
		if e.Message != nil {
			analyseMessage(e.Message, scope, t)
		}
	}
	return t
}

func analyseMessage(m *parser.Message, scope []string, t types) {
	name := m.Name
	t.addName(name, scope)
	scope = append(scope, name)
	for _, me := range m.Entries {
		if me.Message != nil {
			analyseMessage(me.Message, scope, t)
		}
	}
}

func protoPkg(p *parser.Proto) string {
	for _, e := range p.Entries {
		if e.Package != "" {
			return e.Package
		}
	}
	return ""
}

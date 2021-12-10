package protoparser

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestParser(t *testing.T) {
	files, err := filepath.Glob("testdata/*.proto")
	if err != nil {
		t.Fatal(err)
	}
	for _, file := range files {
		t.Run(file, func(t *testing.T) {
			r, err := os.Open(file)
			if err != nil {
				t.Fatal(err)
			}
			_, err = Parse(file, r)
			if err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestImports(t *testing.T) {
	tests := []struct {
		name   string
		source string
		want   []string
	}{{
		name:   "parses a single import correctly",
		source: `import "foo/bar/test.proto"`,
		want:   []string{"foo/bar/test.proto"},
	}}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ParseString("test.proto", tt.source)
			if err != nil {
				t.Fatalf("got unexpected error: %s", err)
			}
			result := imports(got)
			if !reflect.DeepEqual(result, tt.want) {
				t.Errorf("ParseString() got = %v, want %v", result, tt.want)
			}
		})
	}
}

func imports(from *Proto) []string {
	var result []string
	for _, entity := range from.Entries {
		if entity.Import != "" {
			result = append(result, entity.Import)
		}
	}
	return result
}

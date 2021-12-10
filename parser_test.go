package protoparser

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestParser(t *testing.T) {
	files, err := filepath.Glob("testdata/*.proto")
	if err != nil {
		t.Fatal(err)
	}
	for _, file := range files {
		name := strings.TrimPrefix(strings.TrimSuffix(file, ".proto"), "testdata/")
		t.Run(name, func(t *testing.T) {
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

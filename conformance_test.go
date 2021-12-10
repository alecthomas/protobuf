//go:build conformance

package protoparser

import (
	"os"
	"path/filepath"
	"testing"
)

func TestConformance(t *testing.T) {
	files, err := filepath.Glob("testdata/conformance/*.proto")
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

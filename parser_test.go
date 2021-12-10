package protoparser_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/alecthomas/protoparser"
)

func TestParser(t *testing.T) {
	t.Parallel()
	files, err := filepath.Glob("testdata/*.proto")
	if err != nil {
		t.Fatal(err)
	}
	for _, file := range files { // nolint: paralleltest
		t.Run(file, func(t *testing.T) {
			t.Parallel()
			r, err := os.Open(file)
			if err != nil {
				t.Fatal(err)
			}
			_, err = protoparser.Parse(file, r)
			if err != nil {
				t.Fatal(err)
			}
		})
	}
}

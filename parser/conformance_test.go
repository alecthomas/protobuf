package parser

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestConformance(t *testing.T) {
	files, err := filepath.Glob("../testdata/conformance/*.proto")
	require.NoError(t, err)
	for _, file := range files {
		t.Run(file, func(t *testing.T) {
			r, err := os.Open(file)
			require.NoError(t, err)
			_, err = Parse(file, r)
			require.NoError(t, err)
		})
	}
}

package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/alecthomas/protobuf/compiler"
	"github.com/google/go-cmp/cmp"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/testing/protocmp"
	pb "google.golang.org/protobuf/types/descriptorpb"
)

func TestFiledescriptorSetConformance(t *testing.T) {
	files, err := filepath.Glob("testdata/conformance/*.proto")
	require.NoError(t, err)
	tmpDir := t.TempDir()
	// skip test 15 out of 47 conformance tests:
	//    5 deprecated
	//    10 with compiler issues to be fixed
	skip := map[string]bool{
		// deprecated in Go
		"testdata/conformance/unittest_lite.proto":                 true, // golib deprecation proto1 featurea
		"testdata/conformance/unittest_lite_imports_nonlite.proto": true, // golib deprecation proto1 featurea
		"testdata/conformance/unittest_mset.proto":                 true, // golib deprecation proto1 featurea
		"testdata/conformance/unittest_mset_wire_format.proto":     true, // golib deprecation proto1 featurea
		"testdata/conformance/map_lite_unittest.proto":             true, // golib deprecation proto1 featurea
		// compiler issues to be worked out
		"testdata/conformance/map_unittest.proto":                 true, // groups panic
		"testdata/conformance/test_messages_proto2.proto":         true, // has invalid default: could not parse value for int64: "-9.123456789e+18"
		"testdata/conformance/unittest.proto":                     true, // groups panic
		"testdata/conformance/unittest_custom_options.proto":      true, // panic
		"testdata/conformance/unittest_embed_optimize_for.proto":  true, // panic
		"testdata/conformance/unittest_enormous_descriptor.proto": true, // not equal
		"testdata/conformance/unittest_lazy_dependencies.proto":   true, // not equal
		"testdata/conformance/unittest_no_field_presence.proto":   true, // panic
		"testdata/conformance/unittest_optimize_for.proto":        true, // panic
		"testdata/conformance/unittest_proto3_optional.proto":     true, // not equal
	}
	require.NoError(t, err)
	for _, file := range files {
		if skip[file] {
			continue
		}
		t.Run(file, func(t *testing.T) {
			fdName := filepath.Base(file)
			importPaths := []string{"testdata/conformance/"}

			includeImports := true
			name := strings.TrimSuffix(fdName, ".proto")

			fds, err := compiler.Compile([]string{fdName}, importPaths, includeImports)
			require.NoError(t, err)
			// Deterministic forces maps to be ordered by keys which comes
			// to bear for option value `{s1: "1"  s2: "2"};`
			m := proto.MarshalOptions{Deterministic: true}
			b, err := m.Marshal(fds)
			require.NoError(t, err)
			gotFile := filepath.Join(tmpDir, name+".pb")
			require.NoError(t, os.WriteFile(gotFile, b, 0600))

			wantFile := "testdata/conformance/pb/" + name + ".pb"
			want := loadPB(t, wantFile)
			got := loadPB(t, gotFile)
			require.Equal(t, len(got.File), len(want.File))
			requireProtoEqual(t, want, got)
		})
	}
}

func requireProtoEqual(t *testing.T, want, got proto.Message) {
	t.Helper()
	if proto.Equal(want, got) {
		return
	}
	if diff := cmp.Diff(want, got, protocmp.Transform()); diff != "" {
		t.Errorf("not equal (-want +got):\n%s", diff)
	}
}

func loadPB(t *testing.T, file string) *pb.FileDescriptorSet {
	t.Helper()
	pbBytes, err := os.ReadFile(file)
	require.NoError(t, err)
	fds := &pb.FileDescriptorSet{}
	err = proto.Unmarshal(pbBytes, fds)
	require.NoError(t, err)
	return fds
}

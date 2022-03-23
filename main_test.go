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
	for _, file := range files {
		t.Run(file, func(t *testing.T) {
			fdName := filepath.Base(file)
			importPaths := []string{"testdata/conformance/"}

			includeImports := true
			name := strings.TrimSuffix(fdName, ".proto")

			got, err := compiler.Compile([]string{fdName}, importPaths, includeImports)
			require.NoError(t, err)
			want := loadPB(t, "testdata/conformance/pb/"+name+".pb")
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

	// Unmarshal again using the fds we just created as a resolver registry
	// so that extensions can be resolved properly. Without this, the
	// extension fields appear as RawFields which can have different
	// representations of the same field due to ordering, and then they
	// dont compare as equal when they semantically are. Properly resolving
	// extensions makes the comparison work.
	reg, err := compiler.NewRegistry(fds)
	require.NoError(t, err)
	err = proto.UnmarshalOptions{Resolver: reg}.Unmarshal(pbBytes, fds)
	require.NoError(t, err)
	return fds
}

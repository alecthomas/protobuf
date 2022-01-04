package compiler

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/testing/protocmp"
	pb "google.golang.org/protobuf/types/descriptorpb"
)

func requireProtoEqual(t *testing.T, want, got proto.Message) {
	t.Helper()
	if diff := cmp.Diff(want, got, protocmp.Transform()); diff != "" {
		t.Errorf("not equal (-want +got):\n%s", diff)
	}
}

func TestFiledescriptorSet(t *testing.T) {
	files, err := filepath.Glob("testdata/*.proto")
	require.NoError(t, err)
	for _, file := range files {
		t.Run(file, func(t *testing.T) {
			fdName := strings.TrimPrefix(file, "testdata/")
			importPaths := []string{"testdata"}

			includeImports := true
			name := strings.TrimSuffix(filepath.Base(file), ".proto")
			if strings.HasSuffix(name, "_no_include") {
				includeImports = false
			}

			got, err := NewFileDescriptorSet([]string{fdName}, importPaths, includeImports)
			require.NoError(t, err)

			pbFile := "testdata/pb/" + name + ".pb"
			want := loadPB(t, pbFile)
			require.Equal(t, len(got.File), len(want.File))
			requireProtoEqual(t, want, got)
		})
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

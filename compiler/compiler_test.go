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
	// skip test 17 as we don't interpret custom options yet.
	skip := map[string]bool{"testdata/17_proto2_custom_options.proto": true}
	require.NoError(t, err)
	for _, file := range files {
		if skip[file] {
			continue
		}
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

func TestUninterpretedOptions(t *testing.T) {
	fdName := "17_proto2_custom_options.proto"
	importPaths := []string{"testdata"}
	fds, err := NewFileDescriptorSet([]string{fdName}, importPaths, false)
	require.NoError(t, err)
	require.Equal(t, 1, len(fds.File))
	fd := fds.File[0]
	require.Equal(t, 2, len(fd.MessageType))
	message := fd.MessageType[1]
	require.NotNil(t, message)
	require.Equal(t, "User", message.GetName())
	uOpts := message.Options.UninterpretedOption
	require.Equal(t, 6, len(uOpts))

	// option (.pkg.opt1).s1 = "opt1-s1";
	require.Equal(t, 2, len(uOpts[0].Name))
	require.Equal(t, ".pkg.opt1", uOpts[0].Name[0].GetNamePart())
	require.True(t, uOpts[0].Name[0].GetIsExtension())
	require.Equal(t, "s1", uOpts[0].Name[1].GetNamePart())
	require.False(t, uOpts[0].Name[1].GetIsExtension())
}

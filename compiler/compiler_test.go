package compiler

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/testing/protocmp"
	pb "google.golang.org/protobuf/types/descriptorpb"
)

func requireNoError(t *testing.T, err error) {
	t.Helper()
	if err != nil {
		t.Fatalf("want: no error, got: %v", err)
	}
}

func requireEqual(t *testing.T, want, got interface{}) {
	t.Helper()
	var opts []cmp.Option
	var prefix string
	_, wantOK := want.(proto.Message) //nolint:ifshort
	_, gotOK := got.(proto.Message)   //nolint:ifshort
	if wantOK && gotOK {
		opts = append(opts, protocmp.Transform())
		prefix = "protos "
	}
	if diff := cmp.Diff(want, got, opts...); diff != "" {
		t.Errorf(prefix+"not equal (-want +got):\n%s", diff)
	}
}

func TestFiledescriptorSet(t *testing.T) {
	files, err := filepath.Glob("testdata/*.proto")
	requireNoError(t, err)
	for _, file := range files {
		t.Run(file, func(t *testing.T) {
			fdName := strings.TrimPrefix(file, "testdata/")
			importPaths := []string{"testdata"}
			includeImports := true
			got, err := NewFileDescriptorSet([]string{fdName}, importPaths, includeImports)
			requireNoError(t, err)

			name := strings.TrimSuffix(filepath.Base(file), ".proto")
			pbFile := "testdata/pb/" + name + ".pb"
			want := loadPB(t, pbFile)
			requireEqual(t, len(got.File), len(want.File))
			requireEqual(t, want, got)
		})
	}
}

func TestNoIncludeImports(t *testing.T) {
	importPaths := []string{"testdata"}
	includeImports := false
	got, err := NewFileDescriptorSet([]string{"06_proto3_import_transitive.proto"}, importPaths, includeImports)
	requireNoError(t, err)

	want := loadPB(t, "testdata/pb/06_proto3_import_transitive_no_include.pb")
	requireEqual(t, len(got.File), len(want.File))
	requireEqual(t, want, got)
}

func loadPB(t *testing.T, file string) *pb.FileDescriptorSet {
	t.Helper()
	pbBytes, err := os.ReadFile(file)
	requireNoError(t, err)
	fds := &pb.FileDescriptorSet{}
	err = proto.Unmarshal(pbBytes, fds)
	requireNoError(t, err)
	return fds
}

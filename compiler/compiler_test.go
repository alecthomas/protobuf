package compiler

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/alecthomas/protobuf/parser"
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

func requireProtoEqual(t *testing.T, want, got proto.Message) {
	t.Helper()
	opts := cmp.Options{protocmp.Transform()}
	if diff := cmp.Diff(want, got, opts); diff != "" {
		t.Errorf("protos not equal (-want +got):\n%s", diff)
	}
}

func requireEqual(t *testing.T, want, got interface{}) {
	t.Helper()
	opts := cmp.Options{protocmp.Transform()}
	if diff := cmp.Diff(want, got, opts); diff != "" {
		t.Errorf("not equal (-want +got):\n%s", diff)
	}
}

func TestFiledescriptors(t *testing.T) {
	files, err := filepath.Glob("testdata/*.proto")
	requireNoError(t, err)
	skipFile := map[string]bool{
		"testdata/03_proto2_nested.proto": true,
	}
	for _, file := range files {
		t.Run(file, func(t *testing.T) {
			if skipFile[file] {
				return
			}
			r, err := os.Open(file)
			requireNoError(t, err)
			p, err := parser.Parse(file, r)
			requireNoError(t, err)
			fdName := strings.TrimPrefix(file, "testdata/")
			got := NewFileDescriptor(fdName, p)

			name := strings.TrimSuffix(filepath.Base(file), ".proto")
			pbFile := "testdata/pb/" + name + ".pb"
			pbBytes, err := os.ReadFile(pbFile)
			requireNoError(t, err)
			fds := &pb.FileDescriptorSet{}
			err = proto.Unmarshal(pbBytes, fds)
			requireNoError(t, err)
			requireEqual(t, 1, len(fds.File))
			want := fds.File[0]
			requireProtoEqual(t, want, got)
		})
	}
}

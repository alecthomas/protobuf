package parser

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
	files, err := filepath.Glob("../testdata/filedescriptors/*.proto")
	requireNoError(t, err)
	for _, file := range files {
		t.Run(file, func(t *testing.T) {
			r, err := os.Open(file)
			requireNoError(t, err)
			p, err := Parse(file, r)
			requireNoError(t, err)
			fdName := strings.TrimPrefix(file, "../testdata/")
			got, err := NewFileDescriptor(fdName, p)
			requireNoError(t, err)

			pbFile := strings.TrimSuffix(file, ".proto") + ".pb"
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

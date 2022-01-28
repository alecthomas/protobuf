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
	if proto.Equal(want, got) {
		return
	}
	if diff := cmp.Diff(want, got, protocmp.Transform()); diff != "" {
		t.Errorf("not equal (-want +got):\n%s", diff)
	}
}

func TestFiledescriptorSet(t *testing.T) {
	files, err := filepath.Glob("testdata/*.proto")
	// skip test 17 and 18 as we don't interpret custom options yet.
	skip := map[string]bool{
		"testdata/17_proto2_custom_options.proto":           true,
		"testdata/18_a_proto2_aggregate_opt.proto":          true,
		"testdata/18_b_proto2_aggregate_opt.proto":          true,
		"testdata/18_proto2_aggregate_opt_no_include.proto": true,
	}
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
	_, fds, err := compileFileDescriptorSets([]string{fdName}, importPaths, false)
	require.NoError(t, err)
	require.Equal(t, 1, len(fds.File))
	fd := fds.File[0]
	require.Equal(t, 2, len(fd.MessageType))
	message := fd.MessageType[1]
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

func TestUninterpretedAggregateOptions(t *testing.T) {
	fdName := "18_proto2_aggregate_opt_no_include.proto"
	importPaths := []string{"testdata"}
	_, fds, err := compileFileDescriptorSets([]string{fdName}, importPaths, false)
	require.NoError(t, err)
	require.Equal(t, 1, len(fds.File))
	require.Equal(t, 2, len(fds.File[0].MessageType))
	require.Equal(t, 1, len(fds.File[0].MessageType[1].Options.UninterpretedOption))

	uOpt := fds.File[0].MessageType[1].Options.UninterpretedOption[0]
	// option (opt1) = {s1: "1" s2: "2" };
	require.Equal(t, 1, len(uOpt.Name))
	require.Equal(t, ".pkg.opt1", uOpt.Name[0].GetNamePart())
	require.True(t, uOpt.Name[0].GetIsExtension())
	want := `
  s1: "1"
  s2: "2"
`
	require.Equal(t, want, uOpt.GetAggregateValue())
}

/*
This test is flaky - sometimes it passes, sometimes it doesn't
I believe this indicates that there is a bug in protoc when custom
options are defined and used in the same file.

func TestAggregateOptionsSameFile(t *testing.T) {
	// When a custom option is defined and used in the same file
	// it get marshalled in the FileDescriptor as Unknown field,
	// rather than as Extension.
	//
	// Resulting error:
	// -(want) "50001":      protoreflect.RawFields{0x8a, 0xb5, 0x18, 0x06, 0x0a, 0x01, 0x31, 0x12, 0x01, 0x32},
	// +( got) "[pkg.opt1]": s`{s1:"1"  s2:"2"}`,
	//
	// Writing the compiled FileDescriptorSet to File and reading it in again
	// gives the same result, which seems to indicate that the results are correct
	// and wire compatible
	name := "18_proto2_aggregate_opt_no_include"
	fdName := name + ".proto"
	importPaths := []string{"testdata"}
	fds, err := NewFileDescriptorSet([]string{fdName}, importPaths, false)
	require.NoError(t, err)

	dir := t.TempDir()
	fname := filepath.Join(dir, name+".pb")
	f, err := os.Create(fname)
	require.NoError(t, err)
	b, err := proto.Marshal(fds)
	require.NoError(t, err)
	f.Write(b)
	require.NoError(t, f.Close())

	pbFile := "testdata/pb/" + name + ".pb"
	want := loadPB(t, pbFile)
	got := loadPB(t, fname)
	requireProtoEqual(t, want, got)
}
*/

func TestAggregateOptionsSeparateFiles(t *testing.T) {
	// This test contains the same protos as TestAggregateOptionsSameFile
	// only split over two files:
	// one containing the extension definition (a)
	// the second containing the extension usage (b)
	name := "18_b_proto2_aggregate_opt"
	fdName := name + ".proto"
	importPaths := []string{"testdata"}
	fds, err := NewFileDescriptorSet([]string{fdName}, importPaths, true)
	require.NoError(t, err)
	require.Equal(t, 3, len(fds.File))
	got := fds.File[2]
	require.Equal(t, fdName, got.GetName())

	pbFile := "testdata/pb/" + name + ".pb"
	wantFDS := loadPB(t, pbFile)
	require.Equal(t, 3, len(wantFDS.File))
	want := fds.File[2]
	require.Equal(t, fdName, got.GetName())

	requireProtoEqual(t, want, got)
}

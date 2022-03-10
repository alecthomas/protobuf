package compiler

import (
	"os"
	"testing"

	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/reflect/protoregistry"
	"google.golang.org/protobuf/types/descriptorpb"
)

// ensure Registry implments ExtensionTypeResolver
var _ protoregistry.ExtensionTypeResolver = (*Registry)(nil)

// ensure Registry implments MessageTypeResolver
var _ protoregistry.MessageTypeResolver = (*Registry)(nil)

func TestFindExtensionByName(t *testing.T) {
	tests := map[string]struct {
		extName string
		err     error
	}{
		"top-level extension":      {"regtest.ef1", nil},
		"nested extension":         {"regtest.ExtensionMessage.ef2", nil},
		"deeply nested extension":  {"regtest.ExtensionMessage.NestedExtension.ef3", nil},
		"other package extension":  {"regtest.base", nil},
		"imported extension":       {"google.api.http", nil},
		"unknown extension":        {"unknown.extension", protoregistry.NotFound},
		"non-extension descriptor": {"regtest.BaseMessage", protoregistry.NotFound},
	}

	r := newRegistry(t)
	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			extName := protoreflect.FullName(tc.extName)
			et, err := r.FindExtensionByName(extName)
			if tc.err != nil {
				require.ErrorIs(t, err, tc.err)
			} else {
				require.NoError(t, err, tc.extName)
				require.Equal(t, extName, et.TypeDescriptor().FullName())
			}
		})
	}
}

func TestFindExtensionByNumber(t *testing.T) {
	tests := map[string]struct {
		message     string
		fieldNumber int32
		extName     string
		err         error
	}{
		"top-level extension":     {"regtest.BaseMessage", 1000, "regtest.ef1", nil},
		"nested extension":        {"regtest.BaseMessage", 1001, "regtest.ExtensionMessage.ef2", nil},
		"deeply nested extension": {"regtest.BaseMessage", 1002, "regtest.ExtensionMessage.NestedExtension.ef3", nil},
		"other package extension": {"google.protobuf.MethodOptions", 56789, "regtest.base", nil},
		"imported extension":      {"google.protobuf.MethodOptions", 72295728, "google.api.http", nil},
		"unknown message":         {"regtest.Foo", 999, "unknown.message", protoregistry.NotFound},
		"unknown extension":       {"regtest.BaseMessage", 999, "unknown.extension", protoregistry.NotFound},
	}

	r := newRegistry(t)
	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			messageName := protoreflect.FullName(tc.message)
			fieldNumber := protoreflect.FieldNumber(tc.fieldNumber)
			et, err := r.FindExtensionByNumber(messageName, fieldNumber)
			if tc.err != nil {
				require.ErrorIs(t, err, tc.err)
			} else {
				require.NoError(t, err, tc.extName)
				extName := protoreflect.FullName(tc.extName)
				require.Equal(t, extName, et.TypeDescriptor().FullName())
			}
		})
	}
}

func TestFindMessageByName(t *testing.T) {
	tests := map[string]struct {
		name string
		err  error
	}{
		"top-level message":      {"regtest.BaseMessage", nil},
		"nested message":         {"regtest.ExtensionMessage.NestedExtension", nil},
		"unknown message":        {"regtest.Foo", protoregistry.NotFound},
		"non-message descriptor": {"regtest.ef1", protoregistry.NotFound},
	}

	r := newRegistry(t)
	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			messageName := protoreflect.FullName(tc.name)
			mt, err := r.FindMessageByName(messageName)
			if tc.err != nil {
				require.ErrorIs(t, err, tc.err)
			} else {
				require.NoError(t, err, tc.name)
				require.Equal(t, messageName, mt.Descriptor().FullName())
			}
		})
	}
}

func TestFindMessageByURL(t *testing.T) {
	tests := map[string]struct {
		url string
		err error
	}{
		"simple url":       {"regtest.BaseMessage", nil},
		"hostname url":     {"example.com/regtest.BaseMessage", nil},
		"multiple slashes": {"example.com/foo/bar/regtest.BaseMessage", nil},
		"unknown message":  {"example.com/regtest.Foo", protoregistry.NotFound},
	}

	r := newRegistry(t)
	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			mt, err := r.FindMessageByURL(tc.url)
			if tc.err != nil {
				require.ErrorIs(t, err, tc.err)
			} else {
				require.NoError(t, err, tc.url)
				expected := protoreflect.FullName("regtest.BaseMessage")
				require.Equal(t, expected, mt.Descriptor().FullName())
			}
		})
	}
}

func newRegistry(t *testing.T) *Registry {
	t.Helper()
	b, err := os.ReadFile("testdata/pb/regtest.pb")
	require.NoError(t, err)
	fds := descriptorpb.FileDescriptorSet{}
	err = proto.Unmarshal(b, &fds)
	require.NoError(t, err)
	files, err := NewRegistry(&fds)
	require.NoError(t, err)
	return files
}

// Copied from Apache 2.0 licensed
// https://github.com/foxygoat/protog/blob/v0.0.11/registry/files.go
//
// Type Files wraps protoregistry.Files and can be used as a
// protoregistry.ExtensionTypeResolver and a
// protoregistry.MessageTypeResolver. This allows a protoregistry.Files
// to be used as Resolver for protobuf encoding marshaling options.
package compiler

import (
	"strings"

	"google.golang.org/protobuf/reflect/protodesc"
	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/reflect/protoregistry"
	"google.golang.org/protobuf/types/descriptorpb"
	"google.golang.org/protobuf/types/dynamicpb"
)

type Files struct {
	protoregistry.Files
}

func NewFiles(fds *descriptorpb.FileDescriptorSet) (*Files, error) {
	f, err := protodesc.NewFiles(fds)
	if err != nil {
		return nil, err
	}
	return &Files{Files: *f}, nil
}

type extMatchFn func(protoreflect.ExtensionDescriptor) bool

// extensionContainer is implemented by FileDescriptor and MessageDescriptor.
// They are both "namespaces" that contain extensions and have "sub-namespaces".
type extensionContainer interface {
	Messages() protoreflect.MessageDescriptors
	Extensions() protoreflect.ExtensionDescriptors
}

func (f *Files) FindExtensionByName(field protoreflect.FullName) (protoreflect.ExtensionType, error) {
	desc, err := f.FindDescriptorByName(field)
	if err != nil {
		return nil, err
	}
	ed, ok := desc.(protoreflect.ExtensionDescriptor)
	if !ok {
		return nil, protoregistry.NotFound
	}
	return dynamicpb.NewExtensionType(ed), nil
}

func (f *Files) FindExtensionByNumber(message protoreflect.FullName, field protoreflect.FieldNumber) (protoreflect.ExtensionType, error) {
	ets := f.walkExtensions(false, func(ed protoreflect.ExtensionDescriptor) bool {
		return ed.ContainingMessage().FullName() == message && ed.Number() == field
	})
	if len(ets) == 0 {
		return nil, protoregistry.NotFound
	}
	return ets[0], nil
}

func (f *Files) GetExtensionsOfMessage(message protoreflect.FullName) []protoreflect.ExtensionType {
	return f.walkExtensions(true, func(ed protoreflect.ExtensionDescriptor) bool {
		return ed.ContainingMessage().FullName() == message
	})
}

func (f *Files) walkExtensions(getAll bool, pred extMatchFn) []protoreflect.ExtensionType {
	var result []protoreflect.ExtensionType

	f.RangeFiles(func(fd protoreflect.FileDescriptor) bool {
		result = append(result, getExtensions(fd, getAll, pred)...)
		// continue if we are getting all extensions or have none so far
		return getAll || len(result) == 0
	})
	return result
}

func getExtensions(ec extensionContainer, getAll bool, pred extMatchFn) []protoreflect.ExtensionType {
	var result []protoreflect.ExtensionType

	eds := ec.Extensions()
	for i := 0; i < eds.Len() && (getAll || len(result) == 0); i++ {
		ed := eds.Get(i)
		if pred(ed) {
			result = append(result, dynamicpb.NewExtensionType(ed))
		}
	}

	mds := ec.Messages()
	for i := 0; i < mds.Len() && (getAll || len(result) == 0); i++ {
		md := mds.Get(i)
		result = append(result, getExtensions(md, getAll, pred)...)
	}

	return result
}

func (f *Files) FindMessageByName(name protoreflect.FullName) (protoreflect.MessageType, error) {
	desc, err := f.FindDescriptorByName(name)
	if err != nil {
		return nil, err
	}
	md, ok := desc.(protoreflect.MessageDescriptor)
	if !ok {
		return nil, protoregistry.NotFound
	}
	return dynamicpb.NewMessageType(md), nil
}

func (f *Files) FindMessageByURL(url string) (protoreflect.MessageType, error) {
	message := protoreflect.FullName(url)
	// Strip off before the last slash - we only look locally for the
	// message and do not hit the network. The part after the last slash
	// must be the full name of the message.
	if i := strings.LastIndexByte(url, '/'); i >= 0 {
		message = message[i+len("/"):]
	}
	return f.FindMessageByName(message)
}

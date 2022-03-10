package compiler

import (
	"strings"

	"google.golang.org/protobuf/reflect/protodesc"
	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/reflect/protoregistry"
	"google.golang.org/protobuf/types/descriptorpb"
	"google.golang.org/protobuf/types/dynamicpb"
)

type Registry struct {
	protoregistry.Files
}

func NewRegistry(fds *descriptorpb.FileDescriptorSet) (*Registry, error) {
	f, err := protodesc.NewFiles(fds)
	if err != nil {
		return nil, err
	}
	return &Registry{Files: *f}, nil
}

// FindExtensionByName implements protoregistry.ExtensionTypeResolver.
func (f *Registry) FindExtensionByName(field protoreflect.FullName) (protoreflect.ExtensionType, error) {
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

// FindExtensionByNumber implements protoregistry.ExtensionTypeResolver.
func (f *Registry) FindExtensionByNumber(message protoreflect.FullName, field protoreflect.FieldNumber) (protoreflect.ExtensionType, error) {
	// FileDescriptor and MessageDescriptor implement this interface.
	type extensionContainer interface {
		Messages() protoreflect.MessageDescriptors
		Extensions() protoreflect.ExtensionDescriptors
	}

	var getExtension func(extensionContainer) protoreflect.ExtensionType
	getExtension = func(ec extensionContainer) protoreflect.ExtensionType {
		eds := ec.Extensions()
		for i := 0; i < eds.Len(); i++ {
			ed := eds.Get(i)
			if ed.ContainingMessage().FullName() == message && ed.Number() == field {
				return dynamicpb.NewExtensionType(ed)
			}
		}

		mds := ec.Messages()
		for i := 0; i < mds.Len(); i++ {
			if et := getExtension(mds.Get(i)); et != nil {
				return et
			}
		}

		return nil
	}

	var et protoreflect.ExtensionType
	f.RangeFiles(func(fd protoreflect.FileDescriptor) bool {
		et = getExtension(fd)
		return et == nil
	})
	if et == nil {
		return nil, protoregistry.NotFound
	}
	return et, nil
}

// FindMessageByName implements protoregistry.MessageTypeResolver.
func (f *Registry) FindMessageByName(name protoreflect.FullName) (protoreflect.MessageType, error) {
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

// FindMessageByURL implements protoregistry.MessageTypeResolver.
func (f *Registry) FindMessageByURL(url string) (protoreflect.MessageType, error) {
	message := protoreflect.FullName(url)
	// Strip off before the last slash - we only look locally for the
	// message and do not hit the network. The part after the last slash
	// must be the full name of the message.
	if i := strings.LastIndexByte(url, '/'); i >= 0 {
		message = message[i+len("/"):]
	}
	return f.FindMessageByName(message)
}

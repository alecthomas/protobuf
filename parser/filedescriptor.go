package parser

import (
	"errors"
	"fmt"
	"strings"

	pb "google.golang.org/protobuf/types/descriptorpb"
)

func NewFileDescriptor(file string, p *Proto) (*pb.FileDescriptorProto, error) {
	fd := &pb.FileDescriptorProto{
		Name: &file,
	}
	for _, e := range p.Entries {
		switch {
		case e.Syntax != "":
			if fd.Syntax != nil {
				return nil, errors.New("found second syntax entries")
			}
			fd.Syntax = &e.Syntax
		case e.Package != "":
			if fd.Package != nil {
				return nil, errors.New("found second Package entries")
			}
			fd.Package = &e.Package
		case e.Import != nil:
			fd.Dependency = append(fd.Dependency, e.Import.Name)
			if e.Import.Public {
				idx := int32(len(fd.Dependency) - 1)
				fd.PublicDependency = append(fd.PublicDependency, idx)
			}
		case e.Message != nil:
			m, err := message(e.Message)
			if err != nil {
				return nil, err
			}
			fd.MessageType = append(fd.MessageType, m)
		case e.Enum != nil:
			e, err := enum(e.Enum)
			if err != nil {
				return nil, err
			}
			fd.EnumType = append(fd.EnumType, e)
		case e.Service != nil:
		case e.Option != nil:
		case e.Extend != nil:
		default:
			return nil, errors.New("cannot interpret Entry")
		}
	}

	return fd, nil
}

func enum(pe *Enum) (*pb.EnumDescriptorProto, error) {
	e := &pb.EnumDescriptorProto{
		Name: &pe.Name,
	}
	for _, pev := range pe.Values {
		switch {
		case pev.Value != nil:
			ev, err := enumValue(pev.Value)
			if err != nil {
				return nil, err
			}
			e.Value = append(e.Value, ev)
		case pev.Option != nil: // TODO
		case pev.Reserved != nil:
			reservedRanges, reservedNames, err := reserved(pev.Reserved)
			if err != nil {
				return nil, err
			}
			e.ReservedRange = append(e.ReservedRange, reservedRanges...)
			e.ReservedName = append(e.ReservedName, reservedNames...)
		default:
			return nil, errors.New("cannot interpret EnumEntry")
		}
	}
	return e, nil
}

func reserved(pr *Reserved) ([]*pb.EnumDescriptorProto_EnumReservedRange, []string, error) {
	var reservedRanges []*pb.EnumDescriptorProto_EnumReservedRange
	for _, r := range pr.Ranges {
		start := int32(r.Start)
		er := &pb.EnumDescriptorProto_EnumReservedRange{
			Start: &start,
			End:   &start,
		}
		if r.End != nil {
			end := int32(*r.End)
			er.End = &end
		}
		if r.Max {
			var end int32 = 2147483647 // tested with protoc 🤷‍♀️
			er.End = &end
		}
		reservedRanges = append(reservedRanges, er)
	}
	return reservedRanges, pr.FieldNames, nil
}

func enumValue(pev *EnumValue) (*pb.EnumValueDescriptorProto, error) {
	val := int32(pev.Value)
	e := &pb.EnumValueDescriptorProto{
		Name:    &pev.Key,
		Number:  &val,
		Options: nil, // TODO
	}
	return e, nil
}

func message(pm *Message) (*pb.DescriptorProto, error) {
	dp := &pb.DescriptorProto{
		Name: &pm.Name,
	}
	for _, e := range pm.Entries {
		switch {
		case e.Enum != nil:
			et, err := enum(e.Enum)
			if err != nil {
				return nil, err
			}
			dp.EnumType = append(dp.EnumType, et)
		case e.Option != nil:
		case e.Message != nil:
			m, err := message(e.Message)
			if err != nil {
				return nil, err
			}
			dp.NestedType = append(dp.NestedType, m)
		case e.Oneof != nil:
		case e.Extend != nil:
		case e.Reserved != nil:
		case e.Extensions != nil:
		case e.Field != nil:
			df, err := field(e.Field)
			if err != nil {
				return nil, err
			}
			idx := int32(len(dp.Field))
			df.OneofIndex = &idx
			dp.Field = append(dp.Field, df)
			name := "_" + *df.Name
			dcl := &pb.OneofDescriptorProto{Name: &name}
			dp.OneofDecl = append(dp.OneofDecl, dcl)
		default:
			return nil, errors.New("cannot interpret MessageEntry")
		}
	}

	return dp, nil
}

var scalars = map[Scalar]pb.FieldDescriptorProto_Type{
	Double:   pb.FieldDescriptorProto_TYPE_DOUBLE,
	Float:    pb.FieldDescriptorProto_TYPE_FLOAT,
	Int32:    pb.FieldDescriptorProto_TYPE_INT32,
	Int64:    pb.FieldDescriptorProto_TYPE_INT64,
	Uint32:   pb.FieldDescriptorProto_TYPE_UINT32,
	Uint64:   pb.FieldDescriptorProto_TYPE_UINT64,
	Sint32:   pb.FieldDescriptorProto_TYPE_SINT32,
	Sint64:   pb.FieldDescriptorProto_TYPE_SINT64,
	Fixed32:  pb.FieldDescriptorProto_TYPE_FIXED32,
	Fixed64:  pb.FieldDescriptorProto_TYPE_FIXED64,
	SFixed32: pb.FieldDescriptorProto_TYPE_SFIXED32,
	SFixed64: pb.FieldDescriptorProto_TYPE_SFIXED64,
	Bool:     pb.FieldDescriptorProto_TYPE_BOOL,
	String:   pb.FieldDescriptorProto_TYPE_STRING,
	Bytes:    pb.FieldDescriptorProto_TYPE_BYTES,
}

func field(pf *Field) (*pb.FieldDescriptorProto, error) {
	if pf.Direct == nil {
		return nil, errors.New("non-direct not implemented")
	}
	if pf.Direct.Type.Map != nil {
		return nil, errors.New("map types not implemented")
	}

	df := &pb.FieldDescriptorProto{}
	var label pb.FieldDescriptorProto_Label
	// TODO: if proto2, a label is necessary.
	if pf.Required {
		// TODO: return error in proto3. required not valid.
		label = pb.FieldDescriptorProto_LABEL_REQUIRED
	} else if pf.Repeated {
		label = pb.FieldDescriptorProto_LABEL_REPEATED
	} else {
		// TODO: Add proto3 optional. Dont have syntax here now.
		label = pb.FieldDescriptorProto_LABEL_OPTIONAL
	}

	tag := int32(pf.Direct.Tag)
	df.Name = &pf.Direct.Name
	df.Number = &tag
	proto3Optional := true // TODO fudged.
	df.Proto3Optional = &proto3Optional
	df.JsonName = jsonStr(pf.Direct.Name)
	df.Label = &label
	if pf.Direct.Type.Reference != nil {
		// TODO: Determine if enum or message reference.
		// TODO: Resolve relative references
		t := pb.FieldDescriptorProto_TYPE_MESSAGE
		df.Type = &t
		df.TypeName = pf.Direct.Type.Reference
		return df, nil
	}

	fieldType, ok := scalars[pf.Direct.Type.Scalar]
	if !ok {
		return nil, fmt.Errorf("unknown scalar type: %d", pf.Direct.Type.Scalar)
	}
	df.Type = &fieldType

	return df, nil
}

//todo very incomplete
func jsonStr(s string) *string {
	ss := strings.Split(s, "_")
	result := strings.ToLower(ss[0])
	for _, s := range ss[1:] {
		result += strings.Title(strings.ToLower(s))
	}
	return &result
}

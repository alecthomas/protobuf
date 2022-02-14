package compiler

import (
	"github.com/alecthomas/protobuf/parser"
	pb "google.golang.org/protobuf/types/descriptorpb"
)

func newFileOptions(po []*parser.Option) *pb.FileOptions {
	if len(po) == 0 {
		return nil
	}
	opts := &pb.FileOptions{}
	for _, o := range po {
		if len(o.Name) != 1 {
			opts.UninterpretedOption = append(opts.UninterpretedOption, newUninterpretedOption(o))
			continue
		}
		name := o.Name[0].Name
		switch name {
		case "java_package":
			opts.JavaPackage = o.Value.String
		case "java_outer_classname":
			opts.JavaOuterClassname = o.Value.String
		case "go_package":
			opts.GoPackage = o.Value.String
		case "objc_class_prefix":
			opts.ObjcClassPrefix = o.Value.String
		case "csharp_namespace":
			opts.CsharpNamespace = o.Value.String
		case "swift_prefix":
			opts.SwiftPrefix = o.Value.String
		case "php_class_prefix":
			opts.PhpClassPrefix = o.Value.String
		case "php_namespace":
			opts.PhpNamespace = o.Value.String
		case "php_metadata_namespace":
			opts.PhpMetadataNamespace = o.Value.String
		case "ruby_package":
			opts.RubyPackage = o.Value.String
		case "java_multiple_files":
			opts.JavaMultipleFiles = (*bool)(o.Value.Bool)
		case "java_generate_equals_and_hash":
			opts.JavaGenerateEqualsAndHash = (*bool)(o.Value.Bool) // nolint:staticcheck // SA1019: opts.JavaGenerateEqualsAndHash is deprecated: Do not use. - we want to try and be maximum backwards compatible.
		case "java_string_check_utf8":
			opts.JavaStringCheckUtf8 = (*bool)(o.Value.Bool)
		case "cc_generic_services":
			opts.CcGenericServices = (*bool)(o.Value.Bool)
		case "java_generic_services":
			opts.JavaGenericServices = (*bool)(o.Value.Bool)
		case "py_generic_services":
			opts.PyGenericServices = (*bool)(o.Value.Bool)
		case "php_generic_services":
			opts.PhpGenericServices = (*bool)(o.Value.Bool)
		case "deprecated":
			opts.Deprecated = (*bool)(o.Value.Bool)
		case "cc_enable_arenas":
			opts.CcEnableArenas = (*bool)(o.Value.Bool)
		case "optimize_for":
			v := pb.FileOptions_OptimizeMode(pb.FileOptions_OptimizeMode_value[*o.Value.Reference])
			opts.OptimizeFor = &v
		default:
			opts.UninterpretedOption = append(opts.UninterpretedOption, newUninterpretedOption(o))
		}
	}
	return opts
}

func newFieldOptions(field *parser.Field) (*pb.FieldOptions, *string) {
	if field.Direct == nil || len(field.Direct.Options) == 0 {
		return nil, nil
	}
	po := field.Direct.Options
	if len(po) == 1 && len(po[0].Name) == 1 && po[0].Name[0].Name == "default" {
		defaultValue := po[0].Value.ToString()
		return nil, &defaultValue
	}
	var defaultValue *string
	opts := &pb.FieldOptions{}
	for _, o := range po {
		if len(o.Name) != 1 {
			opts.UninterpretedOption = append(opts.UninterpretedOption, newUninterpretedOption(o))
			continue
		}
		name := o.Name[0].Name
		switch name {
		case "default":
			dv := po[0].Value.ToString()
			defaultValue = &dv
		case "ctype":
			v := pb.FieldOptions_CType(pb.FieldOptions_CType_value[*o.Value.Reference])
			opts.Ctype = &v
		case "packed":
			opts.Packed = (*bool)(o.Value.Bool)
		case "jstype":
			v := pb.FieldOptions_JSType(pb.FieldOptions_JSType_value[*o.Value.Reference])
			opts.Jstype = &v
		case "lazy":
			opts.Lazy = (*bool)(o.Value.Bool)
		case "deprecated":
			opts.Deprecated = (*bool)(o.Value.Bool)
		// case "weak": Deprecated in protodescriptor code
		// A weak field is a legacy proto1 feature that is no longer supported
		// https://github.com/protocolbuffers/protobuf-go/blob/e5db2960ed1380681b571cdf4648230beefaf58b/reflect/protodesc/desc_validate.go#L163
		default:
			opts.UninterpretedOption = append(opts.UninterpretedOption, newUninterpretedOption(o))
		}
	}
	return opts, defaultValue
}

func newMethodOptions(po []*parser.Option) *pb.MethodOptions {
	if len(po) == 0 {
		return nil
	}
	opts := &pb.MethodOptions{}
	for _, o := range po {
		if len(o.Name) != 1 {
			opts.UninterpretedOption = append(opts.UninterpretedOption, newUninterpretedOption(o))
			continue
		}
		name := o.Name[0].Name
		switch name {
		case "idempotency_level":
			v := pb.MethodOptions_IdempotencyLevel(pb.MethodOptions_IdempotencyLevel_value[*o.Value.Reference])
			opts.IdempotencyLevel = &v
		case "deprecated":
			opts.Deprecated = (*bool)(o.Value.Bool)
		default:
			opts.UninterpretedOption = append(opts.UninterpretedOption, newUninterpretedOption(o))
		}
	}
	return opts
}

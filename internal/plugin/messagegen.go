package plugin

import (
	"fmt"
	"strconv"

	"github.com/go-kratos/protoc-gen-typescript-http/internal/codegen"
	"google.golang.org/protobuf/reflect/protoreflect"
)

type messageGenerator struct {
	pkg     protoreflect.FullName
	message protoreflect.MessageDescriptor
}

func (m messageGenerator) Generate(f *codegen.File) {
	commentGenerator{descriptor: m.message}.generateLeading(f, 0)
	oneofs := collectOneofs(m.message)
	if len(oneofs) > 0 {
		m.generateOneofUnion(f, oneofs)
	} else {
		m.generateMessageStruct(f)
	}
}

// generateMessageStruct emits the standard flat message struct.
func (m messageGenerator) generateMessageStruct(f *codegen.File) {
	f.P("export type ", scopedDescriptorTypeName(m.pkg, m.message), " = {")
	rangeFields(m.message, func(field protoreflect.FieldDescriptor) {
		commentGenerator{descriptor: field}.generateLeading(f, 1)
		fieldType, err := typeFromField(m.pkg, field)
		if err != nil {
			panic(fmt.Errorf("generate type for field %s: %w", field.FullName(), err))
		}
		if field.ContainingOneof() == nil && !field.HasOptionalKeyword() {
			f.P(t(1), field.JSONName(), ": ", fieldType.Reference(), " | undefined;")
		} else {
			f.P(t(1), field.JSONName(), "?: ", fieldType.Reference(), ";")
		}
	})
	f.P("};")
	f.P()
}

// collectOneofs returns the oneof descriptors that contain at least one
// non-map-entry field.
func collectOneofs(message protoreflect.MessageDescriptor) []protoreflect.OneofDescriptor {
	var oneofs []protoreflect.OneofDescriptor
	for i := 0; i < message.Oneofs().Len(); i++ {
		oneofs = append(oneofs, message.Oneofs().Get(i))
	}
	return oneofs
}

// generateOneofUnion emits a discriminated union message type.
// Shape:
//   export type MsgName =
//     | { $case: "fieldA"; fieldA: TypeA }
//     | { $case: "fieldB"; fieldB: TypeB }
// Then at runtime callers narrow via msg.$case.
func (m messageGenerator) generateOneofUnion(f *codegen.File, oneofs []protoreflect.OneofDescriptor) {
	typeName := scopedDescriptorTypeName(m.pkg, m.message)
	f.P("export type ", typeName, " =")
	for i, oo := range oneofs {
		fields := oneofFields(oo)
		if len(fields) == 0 {
			continue
		}
		for j, fld := range fields {
			fieldType, err := typeFromField(m.pkg, fld)
			if err != nil {
				panic(fmt.Errorf("generate oneof field %s: %w", fld.FullName(), err))
			}
			commentGenerator{descriptor: fld}.generateLeading(f, 1)
			unionFieldName := fld.JSONName()
			if j > 0 {
				f.P(t(1), "|")
			}
			f.P(t(1), "{ $case: ", strconv.Quote(unionFieldName), "; ", unionFieldName, ": ", fieldType.Reference(), " }")
		}
		_ = i
	}
	f.P()
}

// oneofFields returns the scalar/enum/message fields of a oneof (in field order).
func oneofFields(oo protoreflect.OneofDescriptor) []protoreflect.FieldDescriptor {
	var fields []protoreflect.FieldDescriptor
	for i := 0; i < oo.Fields().Len(); i++ {
		fields = append(fields, oo.Fields().Get(i))
	}
	return fields
}

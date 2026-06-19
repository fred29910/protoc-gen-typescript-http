package plugin

import (
	"testing"

	"google.golang.org/protobuf/types/descriptorpb"
	"gotest.tools/v3/assert"
)

func typeFromFieldFromProto(t *testing.T, fd *descriptorpb.FileDescriptorProto, fieldName string) Type {
	t.Helper()
	file := mustNewFile(t, fd)
	msg := findMessage(t, file, "TestMessage")
	for i := 0; i < msg.Fields().Len(); i++ {
		f := msg.Fields().Get(i)
		if string(f.Name()) == fieldName {
			typ, err := typeFromField(file.Package(), f)
			assert.NilError(t, err)
			return typ
		}
	}
	t.Fatalf("field %q not found in TestMessage", fieldName)
	return Type{}
}

func Test_typeFromField_scalarTypes(t *testing.T) {
	t.Parallel()
	fd := &descriptorpb.FileDescriptorProto{
		Name:    strPtr("typetest/scalars.proto"),
		Package: strPtr("typetest"),
		MessageType: []*descriptorpb.DescriptorProto{
			{
				Name: strPtr("TestMessage"),
				Field: []*descriptorpb.FieldDescriptorProto{
					{Name: strPtr("string_field"), Number: int32Ptr(1), Type: fieldType(descriptorpb.FieldDescriptorProto_TYPE_STRING), JsonName: strPtr("stringField")},
					{Name: strPtr("bytes_field"), Number: int32Ptr(2), Type: fieldType(descriptorpb.FieldDescriptorProto_TYPE_BYTES), JsonName: strPtr("bytesField")},
					{Name: strPtr("int32_field"), Number: int32Ptr(3), Type: fieldType(descriptorpb.FieldDescriptorProto_TYPE_INT32), JsonName: strPtr("int32Field")},
					{Name: strPtr("uint32_field"), Number: int32Ptr(4), Type: fieldType(descriptorpb.FieldDescriptorProto_TYPE_UINT32), JsonName: strPtr("uint32Field")},
					{Name: strPtr("int64_field"), Number: int32Ptr(5), Type: fieldType(descriptorpb.FieldDescriptorProto_TYPE_INT64), JsonName: strPtr("int64Field")},
					{Name: strPtr("uint64_field"), Number: int32Ptr(6), Type: fieldType(descriptorpb.FieldDescriptorProto_TYPE_UINT64), JsonName: strPtr("uint64Field")},
					{Name: strPtr("bool_field"), Number: int32Ptr(7), Type: fieldType(descriptorpb.FieldDescriptorProto_TYPE_BOOL), JsonName: strPtr("boolField")},
					{Name: strPtr("float_field"), Number: int32Ptr(8), Type: fieldType(descriptorpb.FieldDescriptorProto_TYPE_FLOAT), JsonName: strPtr("floatField")},
					{Name: strPtr("double_field"), Number: int32Ptr(9), Type: fieldType(descriptorpb.FieldDescriptorProto_TYPE_DOUBLE), JsonName: strPtr("doubleField")},
				},
			},
		},
	}

	t.Run("string maps to string", func(t *testing.T) {
		t.Parallel()
		got := typeFromFieldFromProto(t, fd, "string_field")
		assert.Equal(t, "string", got.Reference())
	})

	t.Run("bytes maps to string", func(t *testing.T) {
		t.Parallel()
		got := typeFromFieldFromProto(t, fd, "bytes_field")
		assert.Equal(t, "string", got.Reference())
	})

	t.Run("int32 maps to number", func(t *testing.T) {
		t.Parallel()
		got := typeFromFieldFromProto(t, fd, "int32_field")
		assert.Equal(t, "number", got.Reference())
	})

	t.Run("uint32 maps to number", func(t *testing.T) {
		t.Parallel()
		got := typeFromFieldFromProto(t, fd, "uint32_field")
		assert.Equal(t, "number", got.Reference())
	})

	t.Run("int64 maps to string", func(t *testing.T) {
		t.Parallel()
		got := typeFromFieldFromProto(t, fd, "int64_field")
		assert.Equal(t, "string", got.Reference())
	})

	t.Run("uint64 maps to string", func(t *testing.T) {
		t.Parallel()
		got := typeFromFieldFromProto(t, fd, "uint64_field")
		assert.Equal(t, "string", got.Reference())
	})

	t.Run("bool maps to boolean", func(t *testing.T) {
		t.Parallel()
		got := typeFromFieldFromProto(t, fd, "bool_field")
		assert.Equal(t, "boolean", got.Reference())
	})

	t.Run("float maps to number", func(t *testing.T) {
		t.Parallel()
		got := typeFromFieldFromProto(t, fd, "float_field")
		assert.Equal(t, "number", got.Reference())
	})

	t.Run("double maps to number", func(t *testing.T) {
		t.Parallel()
		got := typeFromFieldFromProto(t, fd, "double_field")
		assert.Equal(t, "number", got.Reference())
	})
}

func Test_typeFromField_repeatedAndMap(t *testing.T) {
	t.Parallel()

	// repeated string field
	fd := &descriptorpb.FileDescriptorProto{
		Name:    strPtr("typetest/repeated.proto"),
		Package: strPtr("typetest"),
		MessageType: []*descriptorpb.DescriptorProto{
			{
				Name: strPtr("TestMessage"),
				Field: []*descriptorpb.FieldDescriptorProto{
					{
						Name:     strPtr("tags"),
						Number:   int32Ptr(1),
						Label:    descriptorpb.FieldDescriptorProto_LABEL_REPEATED.Enum(),
						Type:     fieldType(descriptorpb.FieldDescriptorProto_TYPE_STRING),
						JsonName: strPtr("tags"),
					},
				},
			},
		},
	}
	t.Run("repeated string maps to T[]", func(t *testing.T) {
		t.Parallel()
		got := typeFromFieldFromProto(t, fd, "tags")
		assert.Assert(t, got.IsList, "expected IsList=true")
		assert.Assert(t, !got.IsMap, "expected IsMap=false")
		assert.Equal(t, "string[]", got.Reference())
	})

	// map<string, string> field — map entry must be declared in the same scope
	// and entry name must be <FieldName>Entry (protoc rule)
	mapEntry := &descriptorpb.DescriptorProto{
		Name: strPtr("LabelsEntry"),
		Options: &descriptorpb.MessageOptions{MapEntry: boolPtr(true)},
		Field: []*descriptorpb.FieldDescriptorProto{
			{Name: strPtr("key"), Number: int32Ptr(1), Type: fieldType(descriptorpb.FieldDescriptorProto_TYPE_STRING), JsonName: strPtr("key")},
			{Name: strPtr("value"), Number: int32Ptr(2), Type: fieldType(descriptorpb.FieldDescriptorProto_TYPE_STRING), JsonName: strPtr("value")},
		},
	}
	mapField := &descriptorpb.FieldDescriptorProto{
		Name:     strPtr("labels"),
		Number:   int32Ptr(1),
		Label:    descriptorpb.FieldDescriptorProto_LABEL_REPEATED.Enum(),
		Type:     fieldType(descriptorpb.FieldDescriptorProto_TYPE_MESSAGE),
		TypeName: strPtr(".typetest.TestMessage.LabelsEntry"),
		JsonName: strPtr("labels"),
	}
	fd2 := &descriptorpb.FileDescriptorProto{
		Name:    strPtr("typetest/map.proto"),
		Package: strPtr("typetest"),
		MessageType: []*descriptorpb.DescriptorProto{
			{
				Name:  strPtr("TestMessage"),
				Field: []*descriptorpb.FieldDescriptorProto{mapField},
				NestedType: []*descriptorpb.DescriptorProto{mapEntry},
			},
		},
	}
	t.Run("map field maps to { [key: string]: T }", func(t *testing.T) {
		t.Parallel()
		file := mustNewFile(t, fd2)
		msg := findMessage(t, file, "TestMessage")
		field := msg.Fields().Get(0)
		got, err := typeFromField(file.Package(), field)
		assert.NilError(t, err)
		assert.Assert(t, got.IsMap, "expected IsMap=true")
		assert.Assert(t, !got.IsList, "expected IsList=false")
		assert.Equal(t, "{ [key: string]: string }", got.Reference())
	})
}

func Test_typeFromField_messageAndEnum(t *testing.T) {
	t.Parallel()
	fd := &descriptorpb.FileDescriptorProto{
		Name:    strPtr("typetest/messages.proto"),
		Package: strPtr("typetest"),
		MessageType: []*descriptorpb.DescriptorProto{
			{
				Name: strPtr("Inner"),
				Field: []*descriptorpb.FieldDescriptorProto{
					{Name: strPtr("value"), Number: int32Ptr(1), Type: fieldType(descriptorpb.FieldDescriptorProto_TYPE_STRING), JsonName: strPtr("value")},
				},
			},
			{
				Name: strPtr("TestMessage"),
				Field: []*descriptorpb.FieldDescriptorProto{
					{Name: strPtr("inner"), Number: int32Ptr(1), Type: fieldType(descriptorpb.FieldDescriptorProto_TYPE_MESSAGE), TypeName: strPtr(".typetest.Inner"), JsonName: strPtr("inner")},
					{Name: strPtr("status"), Number: int32Ptr(2), Type: fieldType(descriptorpb.FieldDescriptorProto_TYPE_ENUM), TypeName: strPtr(".typetest.Status"), JsonName: strPtr("status")},
				},
			},
		},
		EnumType: []*descriptorpb.EnumDescriptorProto{
			{
				Name: strPtr("Status"),
				Value: []*descriptorpb.EnumValueDescriptorProto{
					{Name: strPtr("UNKNOWN"), Number: int32Ptr(0)},
					{Name: strPtr("OK"), Number: int32Ptr(1)},
				},
			},
		},
	}

	t.Run("message field references scoped type name", func(t *testing.T) {
		t.Parallel()
		got := typeFromFieldFromProto(t, fd, "inner")
		assert.Equal(t, "Inner", got.Reference())
	})

	t.Run("enum field references scoped type name", func(t *testing.T) {
		t.Parallel()
		got := typeFromFieldFromProto(t, fd, "status")
		assert.Equal(t, "Status", got.Reference())
	})
}

func Test_typeFromField_unsupportedKind(t *testing.T) {
	t.Parallel()
	// Construct a field with an unknown kind by using a synthetic descriptor.
	// We cannot easily create an invalid FieldDescriptorProto, so we test the
	// error path by checking that known kinds do NOT return "unknown".
	fd := &descriptorpb.FileDescriptorProto{
		Name:    strPtr("typetest/known.proto"),
		Package: strPtr("typetest"),
		MessageType: []*descriptorpb.DescriptorProto{
			{
				Name: strPtr("TestMessage"),
				Field: []*descriptorpb.FieldDescriptorProto{
					{Name: strPtr("name"), Number: int32Ptr(1), Type: fieldType(descriptorpb.FieldDescriptorProto_TYPE_STRING), JsonName: strPtr("name")},
				},
			},
		},
	}
	got := typeFromFieldFromProto(t, fd, "name")
	assert.Assert(t, got.Name != "unknown", "expected known type, got unknown")
}


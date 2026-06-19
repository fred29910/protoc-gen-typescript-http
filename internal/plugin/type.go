package plugin

import (
	"fmt"

	"google.golang.org/protobuf/reflect/protoreflect"
)

type Type struct {
	IsNamed bool
	Name    string

	IsList     bool
	IsMap      bool
	Underlying *Type
}

func (t Type) Reference() string {
	switch {
	case t.IsMap:
		return "{ [key: string]: " + t.Underlying.Reference() + " }"
	case t.IsList:
		return t.Underlying.Reference() + "[]"
	default:
		return t.Name
	}
}

func typeFromField(pkg protoreflect.FullName, field protoreflect.FieldDescriptor) (Type, error) {
	switch {
	case field.IsMap():
		underlying, err := namedTypeFromField(pkg, field.MapValue())
		if err != nil {
			return Type{}, err
		}
		return Type{
			IsMap:      true,
			Underlying: &underlying,
		}, nil
	case field.IsList():
		underlying, err := namedTypeFromField(pkg, field)
		if err != nil {
			return Type{}, err
		}
		return Type{
			IsList:     true,
			Underlying: &underlying,
		}, nil
	default:
		return namedTypeFromField(pkg, field)
	}
}

func namedTypeFromField(pkg protoreflect.FullName, field protoreflect.FieldDescriptor) (Type, error) {
	switch field.Kind() {
	case protoreflect.StringKind, protoreflect.BytesKind:
		return Type{IsNamed: true, Name: "string"}, nil
	case protoreflect.BoolKind:
		return Type{IsNamed: true, Name: "boolean"}, nil
	case
		protoreflect.Int32Kind,
		protoreflect.Uint32Kind,
		protoreflect.DoubleKind,
		protoreflect.Fixed32Kind,
		protoreflect.Sfixed32Kind,
		protoreflect.FloatKind:
		return Type{IsNamed: true, Name: "number"}, nil
	case
		protoreflect.Int64Kind,
		protoreflect.Uint64Kind,
		protoreflect.Fixed64Kind,
		protoreflect.Sfixed64Kind,
		protoreflect.Sint64Kind:
		// 64-bit integers map to string to avoid precision loss beyond Number.MAX_SAFE_INTEGER.
		// Per Protobuf JSON specification, int64/uint64 values should be encoded as strings.
		return Type{IsNamed: true, Name: "string"}, nil
	case protoreflect.MessageKind:
		return typeFromMessage(pkg, field.Message())
	case protoreflect.EnumKind:
		desc := field.Enum()
		if wkt, ok := WellKnownType(field.Enum()); ok {
			return Type{IsNamed: true, Name: wkt.Name()}, nil
		}
		return Type{IsNamed: true, Name: scopedDescriptorTypeName(pkg, desc)}, nil
	default:
		return Type{}, fmt.Errorf("unsupported field kind %v for field %s in message %s", field.Kind(), field.Name(), field.ContainingMessage().FullName())
	}
}

func typeFromMessage(pkg protoreflect.FullName, message protoreflect.MessageDescriptor) (Type, error) {
	if wkt, ok := WellKnownType(message); ok {
		return Type{IsNamed: true, Name: wkt.Name()}, nil
	}
	return Type{IsNamed: true, Name: scopedDescriptorTypeName(pkg, message)}, nil
}

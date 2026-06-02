package plugin

import (
	"fmt"

	"github.com/go-kratos/protoc-gen-typescript-http/internal/httprule"
	"google.golang.org/protobuf/reflect/protoreflect"
)

type jsonLeafWalkFunc func(path httprule.FieldPath, field protoreflect.FieldDescriptor)

func walkJSONLeafFields(message protoreflect.MessageDescriptor, f jsonLeafWalkFunc) {
	var w jsonWalker
	w.walkMessage(nil, message, f)
}

type jsonWalker struct {
	ancestors map[protoreflect.FullName]struct{}
}

func (w *jsonWalker) enter(name protoreflect.FullName) bool {
	if _, ok := w.ancestors[name]; ok {
		return false
	}
	if w.ancestors == nil {
		w.ancestors = make(map[protoreflect.FullName]struct{})
	}
	w.ancestors[name] = struct{}{}
	return true
}

func (w *jsonWalker) leave(name protoreflect.FullName) {
	delete(w.ancestors, name)
}

func (w *jsonWalker) walkMessage(path httprule.FieldPath, message protoreflect.MessageDescriptor, f jsonLeafWalkFunc) {
	if !w.enter(message.FullName()) {
		return
	}
	defer w.leave(message.FullName())
	for i := 0; i < message.Fields().Len(); i++ {
		field := message.Fields().Get(i)
		p := append(httprule.FieldPath{}, path...)
		p = append(p, string(field.Name()))
		switch {
		case !field.IsMap() && field.Kind() == protoreflect.MessageKind:
			if IsWellKnownType(field.Message()) {
				f(p, field)
			} else {
				w.walkMessage(p, field.Message(), f)
			}
		default:
			f(p, field)
		}
	}
}

// repeatedMessageLeaf describes a single leaf inside a repeated message field
// for query serialization purposes. e.g., for `repeated Item items` where
// Item has a `name` field, one Leaf is {PathPrefix: ["items"], LeafPath: ["name"]}.
type repeatedMessageLeaf struct {
	PathPrefix    httprule.FieldPath
	RepeatedField protoreflect.FieldDescriptor
	Element       protoreflect.MessageDescriptor
	LeafPath      httprule.FieldPath
	LeafField     protoreflect.FieldDescriptor
}

type repeatedMessageWalkFunc func(repeatedMessageLeaf)

// walkJSONRepeatedMessageLeaves walks a message descriptor and emits one
// repeatedMessageLeaf per (repeated message field, leaf in element) pair.
// Nested single-message subtrees are recursed into so that repeated message
// fields at any depth are discovered. Nested repeated message or
// map-of-message inside an element returns an error (not supported in MVP).
func walkJSONRepeatedMessageLeaves(
	message protoreflect.MessageDescriptor,
	f repeatedMessageWalkFunc,
) error {
	return walkRepeatedLeaves(message, nil, f)
}

func walkRepeatedLeaves(
	message protoreflect.MessageDescriptor,
	prefix httprule.FieldPath,
	f repeatedMessageWalkFunc,
) error {
	for i := 0; i < message.Fields().Len(); i++ {
		field := message.Fields().Get(i)
		p := append(httprule.FieldPath{}, prefix...)
		p = append(p, field.JSONName())
		switch {
		case field.IsMap():
			// maps are handled by walkJSONLeafFields; this walker only handles repeated message
			continue
		case !field.IsList() && field.Kind() == protoreflect.MessageKind && !IsWellKnownType(field.Message()):
			// Recurse into single-message subtrees to find repeated-message fields at deeper paths.
			if err := walkRepeatedLeaves(field.Message(), p, f); err != nil {
				return err
			}
		case field.IsList() && field.Kind() == protoreflect.MessageKind && !IsWellKnownType(field.Message()):
			if err := emitRepeatedMessageLeaves(field, p, f); err != nil {
				return err
			}
		}
	}
	return nil
}

func emitRepeatedMessageLeaves(
	repeatedField protoreflect.FieldDescriptor,
	prefix httprule.FieldPath,
	f repeatedMessageWalkFunc,
) error {
	element := repeatedField.Message()
	// Validate: element must not contain nested repeated message, map-of-message, or nested single message.
	for i := 0; i < element.Fields().Len(); i++ {
		ef := element.Fields().Get(i)
		if ef.IsList() && ef.Kind() == protoreflect.MessageKind && !IsWellKnownType(ef.Message()) {
			return fmt.Errorf("repeated message field %q: nested repeated message %s.%s is not supported in query serialization", repeatedField.FullName(), element.FullName(), ef.Name())
		}
		if ef.IsMap() && ef.Kind() == protoreflect.MessageKind {
			vm := ef.MapValue()
			if vm.Kind() == protoreflect.MessageKind && !IsWellKnownType(vm.Message()) {
				return fmt.Errorf("repeated message field %q: nested map-of-message %s.%s is not supported in query serialization", repeatedField.FullName(), element.FullName(), ef.Name())
			}
		}
		if !ef.IsMap() && !ef.IsList() && ef.Kind() == protoreflect.MessageKind && !IsWellKnownType(ef.Message()) {
			return fmt.Errorf("repeated message field %q: nested single message %s.%s is not supported in query serialization", repeatedField.FullName(), element.FullName(), ef.Name())
		}
	}
	// Emit a leaf for each scalar/enum/WKT field in the element.
	for i := 0; i < element.Fields().Len(); i++ {
		ef := element.Fields().Get(i)
		f(repeatedMessageLeaf{
			PathPrefix:    append(httprule.FieldPath{}, prefix...),
			RepeatedField: repeatedField,
			Element:       element,
			LeafPath:      httprule.FieldPath{ef.JSONName()},
			LeafField:     ef,
		})
	}
	return nil
}

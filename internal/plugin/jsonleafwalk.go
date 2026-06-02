package plugin

import (
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

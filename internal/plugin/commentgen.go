package plugin

import (
	"strings"

	"github.com/go-kratos/protoc-gen-typescript-http/internal/codegen"
	"google.golang.org/genproto/googleapis/api/annotations"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protoreflect"
)

type commentGenerator struct {
	descriptor protoreflect.Descriptor
}

func (c commentGenerator) generateLeading(f *codegen.File, indent int) {
	loc := c.descriptor.ParentFile().SourceLocations().ByDescriptor(c.descriptor)
	leading := strings.TrimSpace(loc.LeadingComments)
	var behavior string
	if field, ok := c.descriptor.(protoreflect.FieldDescriptor); ok {
		behavior = fieldBehaviorComment(field)
	}

	if leading == "" && behavior == "" {
		return
	}

	f.P(t(indent), "/**")
	if leading != "" {
		lines := strings.Split(leading, "\n")
		for _, line := range lines {
			f.P(t(indent), " * ", strings.TrimSpace(line))
		}
	}
	if behavior != "" {
		if leading != "" {
			f.P(t(indent), " *")
		}
		f.P(t(indent), " * ", behavior)
	}
	f.P(t(indent), " */")
}

func fieldBehaviorComment(field protoreflect.FieldDescriptor) string {
	behaviors := getFieldBehaviors(field)
	if len(behaviors) == 0 {
		return ""
	}

	behaviorStrings := make([]string, 0, len(behaviors))
	for _, b := range behaviors {
		behaviorStrings = append(behaviorStrings, b.String())
	}
	return "Behaviors: " + strings.Join(behaviorStrings, ", ")
}

func getFieldBehaviors(field protoreflect.FieldDescriptor) []annotations.FieldBehavior {
	if behaviors, ok := proto.GetExtension(
		field.Options(), annotations.E_FieldBehavior,
	).([]annotations.FieldBehavior); ok {
		return behaviors
	}
	return nil
}

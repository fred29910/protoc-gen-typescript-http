package plugin

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/go-kratos/protoc-gen-typescript-http/internal/codegen"
	"github.com/go-kratos/protoc-gen-typescript-http/internal/httprule"
	"google.golang.org/protobuf/reflect/protoreflect"
)

type serviceGenerator struct {
	pkg        protoreflect.FullName
	genHandler bool
	service    protoreflect.ServiceDescriptor
}

func (s serviceGenerator) Generate(f *codegen.File) error {
	s.generateInterface(f)
	if s.genHandler {
		s.generateHandler(f)
	}
	return s.generateClient(f)
}

func (s serviceGenerator) generateInterface(f *codegen.File) {
	commentGenerator{descriptor: s.service}.generateLeading(f, 0)
	f.P("export interface ", descriptorTypeName(s.service), " {")
	rangeMethods(s.service.Methods(), func(method protoreflect.MethodDescriptor) {
		if !supportedMethod(method) {
			return
		}
		commentGenerator{descriptor: method}.generateLeading(f, 1)
		input := typeFromMessage(s.pkg, method.Input())
		output := typeFromMessage(s.pkg, method.Output())
		f.P(t(1), method.Name(), "(request: ", input.Reference(), "): Promise<", output.Reference(), ">;")
	})
	f.P("}")
	f.P()
}

func (s serviceGenerator) generateHandler(f *codegen.File) {
	f.P("type RequestType = {")
	f.P(t(1), "path: string;")
	f.P(t(1), "method: string;")
	f.P(t(1), "body: string | null;")
	f.P("};")
	f.P()
	f.P("type RequestHandler = (request: RequestType, meta: { service: string, method: string }) => Promise<unknown>;")
	f.P()
}

func (s serviceGenerator) generateClient(f *codegen.File) error {
	f.P(
		"export function create",
		descriptorTypeName(s.service),
		"Client(",
		"\n",
		t(1),
		"handler: RequestHandler",
		"\n",
		"): ",
		descriptorTypeName(s.service),
		" {",
	)
	f.P(t(1), "return {")
	var methodErr error
	rangeMethods(s.service.Methods(), func(method protoreflect.MethodDescriptor) {
		if err := s.generateMethod(f, method); err != nil {
			methodErr = fmt.Errorf("generate method %s: %w", method.Name(), err)
		}
	})
	if methodErr != nil {
		return methodErr
	}
	f.P(t(1), "};")
	f.P("}")
	return nil
}

func (s serviceGenerator) generateMethod(f *codegen.File, method protoreflect.MethodDescriptor) error {
	if !supportedMethod(method) {
		return nil
	}
	outputType := typeFromMessage(s.pkg, method.Output())
	r, ok := httprule.Get(method)
	if !ok {
		return nil
	}
	rule, err := httprule.ParseRule(r)
	if err != nil {
		return fmt.Errorf("parse http rule: %w", err)
	}
	f.P(t(2), method.Name(), "(request) { // eslint-disable-line @typescript-eslint/no-unused-vars")
	if err := s.generateMethodPathValidation(f, method.Input(), rule); err != nil {
		return fmt.Errorf("path validation: %w", err)
	}
	if err := s.generateMethodPath(f, method.Input(), rule); err != nil {
		return fmt.Errorf("path: %w", err)
	}
	if err := s.generateMethodBody(f, method.Input(), rule); err != nil {
		return fmt.Errorf("body: %w", err)
	}
	if err := s.generateMethodQuery(f, method.Input(), rule); err != nil {
		return fmt.Errorf("query: %w", err)
	}
	f.P(t(3), "let uri = path;")
	f.P(t(3), "if (queryParams.length > 0) {")
	f.P(t(4), "uri += `?${queryParams.join(\"&\")}`")
	f.P(t(3), "}")
	f.P(t(3), "return handler({")
	f.P(t(4), "path: uri,")
	f.P(t(4), "method: ", strconv.Quote(rule.Method), ",")
	f.P(t(4), "body,")
	f.P(t(3), "}, {")
	f.P(t(4), "service: \"", method.Parent().Name(), "\",")
	f.P(t(4), "method: \"", method.Name(), "\",")
	f.P(t(3), "}) as Promise<", outputType.Reference(), ">;")
	f.P(t(2), "},")
	return nil
}

func (s serviceGenerator) generateMethodPathValidation(
	f *codegen.File,
	input protoreflect.MessageDescriptor,
	rule httprule.Rule,
) error {
	for _, seg := range rule.Template.Segments {
		if seg.Kind != httprule.SegmentKindVariable {
			continue
		}
		fp := seg.Variable.FieldPath
		nullPath, err := nullPropagationPath(fp, input)
		if err != nil {
			return fmt.Errorf("path validation null propagation: %w", err)
		}
		protoPath := strings.Join(fp, ".")
		errMsg := "missing required field request." + protoPath
		f.P(t(3), "if (!request.", nullPath, ") {")
		f.P(t(4), "throw new Error(", strconv.Quote(errMsg), ");")
		f.P(t(3), "}")
	}
	return nil
}

func (s serviceGenerator) generateMethodPath(
	f *codegen.File,
	input protoreflect.MessageDescriptor,
	rule httprule.Rule,
) error {
	pathParts := make([]string, 0, len(rule.Template.Segments))
	for _, seg := range rule.Template.Segments {
		switch seg.Kind {
		case httprule.SegmentKindVariable:
			fieldPath, err := jsonPath(seg.Variable.FieldPath, input)
			if err != nil {
				return fmt.Errorf("method path json path: %w", err)
			}
			pathParts = append(pathParts, "${request."+fieldPath+"}")
		case httprule.SegmentKindLiteral:
			pathParts = append(pathParts, seg.Literal)
		case httprule.SegmentKindMatchSingle: // TODO: Double check this and following case
			pathParts = append(pathParts, "*")
		case httprule.SegmentKindMatchMultiple:
			pathParts = append(pathParts, "**")
		}
	}
	path := strings.Join(pathParts, "/")
	if rule.Template.Verb != "" {
		path += ":" + rule.Template.Verb
	}
	f.P(t(3), "const path = `", path, "`; // eslint-disable-line quotes")
	return nil
}

func (s serviceGenerator) generateMethodBody(
	f *codegen.File,
	input protoreflect.MessageDescriptor,
	rule httprule.Rule,
) error {
	switch {
	case rule.Body == "":
		f.P(t(3), "const body = null;")
	case rule.Body == "*":
		f.P(t(3), "const body = JSON.stringify(request);")
	default:
		nullPath, err := nullPropagationPath(httprule.FieldPath{rule.Body}, input)
		if err != nil {
			return fmt.Errorf("method body null propagation: %w", err)
		}
		f.P(t(3), "const body = JSON.stringify(request?.", nullPath, " ?? {});")
	}
	return nil
}

func (s serviceGenerator) generateMethodQuery(
	f *codegen.File,
	input protoreflect.MessageDescriptor,
	rule httprule.Rule,
) error {
	f.P(t(3), "const queryParams: string[] = [];")
	// nothing in query
	if rule.Body == "*" {
		return nil
	}
	// fields covered by path
	pathCovered := make(map[string]struct{})
	for _, segment := range rule.Template.Segments {
		if segment.Kind != httprule.SegmentKindVariable {
			continue
		}
		pathCovered[segment.Variable.FieldPath.String()] = struct{}{}
	}
	var queryErr error
	walkJSONLeafFields(input, func(path httprule.FieldPath, field protoreflect.FieldDescriptor) {
		if queryErr != nil {
			return
		}
		if _, ok := pathCovered[path.String()]; ok {
			return
		}
		if rule.Body != "" && path[0] == rule.Body {
			return
		}
		presenceExpr, err := queryPresenceExpr(path, input)
		if err != nil {
			queryErr = fmt.Errorf("query presence expr: %w", err)
			return
		}
		jp, err := jsonPath(path, input)
		if err != nil {
			queryErr = fmt.Errorf("query json path: %w", err)
			return
		}
		f.P(t(3), "if (", presenceExpr, ") {")
		switch {
		case field.IsList():
			f.P(t(4), "request.", jp, ".forEach((x) => {")
			f.P(t(5), "queryParams.push(`", jp, "=${encodeURIComponent(x.toString())}`)")
			f.P(t(4), "})")
		case field.IsMap():
			f.P(t(4), "Object.keys(request.", jp, ").sort().forEach((key) => {")
			f.P(t(5), "const value = request.", jp, "[key];")
			f.P(t(5), "queryParams.push(`", jp, "[${encodeURIComponent(key)}]=${encodeURIComponent(value.toString())}`)")
			f.P(t(4), "})")
		default:
			f.P(t(4), "queryParams.push(`", jp, "=${encodeURIComponent(request.", jp, ".toString())}`)")
		}
		f.P(t(3), "}")
	})
	return queryErr
}

func supportedMethod(method protoreflect.MethodDescriptor) bool {
	_, ok := httprule.Get(method)
	return ok && !method.IsStreamingClient() && !method.IsStreamingServer()
}

// queryPresenceExpr generates a TypeScript nullish check expression for a query field.
// Returns something like "request.pageSize !== undefined && request.pageSize !== null".
// For nested fields, uses optional chaining: "request.nested?.string !== undefined && request.nested?.string !== null".
func queryPresenceExpr(path httprule.FieldPath, message protoreflect.MessageDescriptor) (string, error) {
	np, err := nullPropagationPath(path, message)
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("request.%s !== undefined && request.%s !== null", np, np), nil
}

// queryValueExpr generates a TypeScript value access expression for a query field.
// Returns something like "request.pageSize" for direct access.
// For nested fields, uses direct dot access: "request.nested.string".
func queryValueExpr(path httprule.FieldPath, message protoreflect.MessageDescriptor) (string, error) {
	jp, err := jsonPath(path, message)
	if err != nil {
		return "", err
	}
	return "request." + jp, nil
}

func jsonPath(path httprule.FieldPath, message protoreflect.MessageDescriptor) (string, error) {
	segs, err := jsonPathSegments(path, message)
	if err != nil {
		return "", err
	}
	return strings.Join(segs, "."), nil
}

func nullPropagationPath(path httprule.FieldPath, message protoreflect.MessageDescriptor) (string, error) {
	segs, err := jsonPathSegments(path, message)
	if err != nil {
		return "", err
	}
	return strings.Join(segs, "?."), nil
}

func jsonPathSegments(path httprule.FieldPath, message protoreflect.MessageDescriptor) ([]string, error) {
	segs := make([]string, len(path))
	for i, p := range path {
		field := message.Fields().ByName(protoreflect.Name(p))
		if field == nil {
			return nil, fmt.Errorf("field %q not found in message %s", p, message.FullName())
		}
		segs[i] = field.JSONName()
		if i < len(path) {
			message = field.Message()
		}
	}
	return segs, nil
}

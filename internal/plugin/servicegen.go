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
		input, err := typeFromMessage(s.pkg, method.Input())
		if err != nil {
			panic(fmt.Errorf("service interface input %s: %w", method.Input().FullName(), err))
		}
		output, err := typeFromMessage(s.pkg, method.Output())
		if err != nil {
			panic(fmt.Errorf("service interface output %s: %w", method.Output().FullName(), err))
		}
		f.P(t(1), method.Name(), "(request: ", input.Reference(), "): Promise<", output.Reference(), ">;")
	})
	f.P("}")
	f.P()
}

func (s serviceGenerator) generateHandler(f *codegen.File) {
	f.P("export type RequestType = {")
	f.P(t(1), "path: string;")
	f.P(t(1), "method: string;")
	f.P(t(1), "body: string | null;")
	f.P("};")
	f.P()
	f.P("export type RequestHandler = (request: RequestType, meta: { service: string, method: string }) => Promise<unknown>;")
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

// generateMethodBinding generates the per-binding TS body: path validation,
// path construction, body construction, and query construction. It does NOT
// write the `return handler(...)` line — the caller wraps each block in an
// if-statement and decides where the return goes.
func (s serviceGenerator) generateMethodBinding(
	f *codegen.File,
	input protoreflect.MessageDescriptor,
	rule httprule.Rule,
) error {
	if err := s.generateMethodPathValidation(f, input, rule); err != nil {
		return fmt.Errorf("path validation: %w", err)
	}
	if err := s.generateMethodPath(f, input, rule); err != nil {
		return fmt.Errorf("path: %w", err)
	}
	if err := s.generateMethodBody(f, input, rule); err != nil {
		return fmt.Errorf("body: %w", err)
	}
	if err := s.generateMethodQuery(f, input, rule); err != nil {
		return fmt.Errorf("query: %w", err)
	}
	return nil
}

func (s serviceGenerator) generateMethod(f *codegen.File, method protoreflect.MethodDescriptor) error {
	if !supportedMethod(method) {
		return nil
	}
	r, ok := httprule.Get(method)
	if !ok {
		return nil
	}
	rule, err := httprule.ParseRule(r)
	if err != nil {
		return fmt.Errorf("parse http rule: %w", err)
	}
	rules := append([]httprule.Rule{rule}, rule.AdditionalRules...)
	return s.writeMethodDispatchBody(f, method, rules)
}

// writeMethodDispatchBody writes the per-method TypeScript body, dispatching
// across the parsed rules. The single-binding fast path emits the body
// without an if/else/throw wrapper so the generated output for RPCs without
// additional_bindings stays byte-equivalent to the pre-multi-binding output.
// The two branches MUST stay in sync — any change to the multi-binding body
// below should be mirrored here, otherwise golden file diffs will appear for
// every RPC.
//
// Extracted from generateMethod so tests can drive it with pre-parsed rules
// without needing to attach google.api.http annotations via protodesc.
func (s serviceGenerator) writeMethodDispatchBody(
	f *codegen.File,
	method protoreflect.MethodDescriptor,
	rules []httprule.Rule,
) error {
	f.P(t(2), method.Name(), "(request) { // eslint-disable-line @typescript-eslint/no-unused-vars")
	if len(rules) == 1 {
		sub := rules[0]
		input := method.Input()
		if err := s.generateMethodBinding(f, input, sub); err != nil {
			return fmt.Errorf("binding: %w", err)
		}
		if err := s.writeMethodHandlerCall(f, method, sub); err != nil {
			return fmt.Errorf("handler: %w", err)
		}
	} else {
		for i, sub := range rules {
			input := method.Input()
			cond, err := bindingPathVarsPresentExpr(sub, input)
			if err != nil {
				return fmt.Errorf("binding %d presence: %w", i, err)
			}
			if i == 0 {
				f.P(t(3), "if (", cond, ") {")
			} else {
				f.P(t(3), "} else if (", cond, ") {")
			}
			if err := s.generateMethodBinding(f, input, sub); err != nil {
				return fmt.Errorf("binding %d: %w", i, err)
			}
			if err := s.writeMethodHandlerCall(f, method, sub); err != nil {
				return fmt.Errorf("binding %d handler: %w", i, err)
			}
		}
		f.P(t(3), "} else {")
		f.P(t(4), "throw new Error(", strconv.Quote("no matching binding for "+string(method.Name())), ");")
		f.P(t(3), "}")
	}
	f.P(t(2), "},")
	return nil
}

// bindingPathVarsPresentExpr returns a TS nullish AND-chain for the path
// variables of the given rule. e.g. "request.id !== undefined && request.id !== null".
// Returns "true" when the rule has no path variables.
func bindingPathVarsPresentExpr(rule httprule.Rule, message protoreflect.MessageDescriptor) (string, error) {
	var parts []string
	for _, seg := range rule.Template.Segments {
		if seg.Kind != httprule.SegmentKindVariable {
			continue
		}
		np, err := nullPropagationPath(seg.Variable.FieldPath, message)
		if err != nil {
			return "", err
		}
		parts = append(parts, fmt.Sprintf("request.%s !== undefined && request.%s !== null", np, np))
	}
	if len(parts) == 0 {
		return "true", nil
	}
	return strings.Join(parts, " && "), nil
}

// writeMethodHandlerCall writes the `return handler({...}, {...})` block for
// a single binding. The method body's `path`, `body`, and `queryParams`
// variables must already be defined by generateMethodBinding in the current
// scope.
func (s serviceGenerator) writeMethodHandlerCall(
	f *codegen.File,
	method protoreflect.MethodDescriptor,
	rule httprule.Rule,
) error {
	outputType, err := typeFromMessage(s.pkg, method.Output())
	if err != nil {
		panic(fmt.Errorf("service method output %s: %w", method.Output().FullName(), err))
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
				if isWildcardVariable(seg) {
					// Preserve structural slashes — split, encode each segment, join
					// e.g., request.name.split('/').map(p => encode(p)).join('/')
					pathParts = append(pathParts,
						"${request."+fieldPath+".split('/').map(p => encode(p)).join('/')}")
				} else {
					// Standard variable — full encode
					pathParts = append(pathParts,
						"${encode(request."+fieldPath+")}")
				}
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
		bodyPath := httprule.FieldPath(strings.Split(rule.Body, "."))
		nullPath, err := nullPropagationPath(bodyPath, input)
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
		if rule.Body != "" && rule.Body != "*" {
			bodySegments := strings.Split(rule.Body, ".")
			if pathStartsWith(path, bodySegments) {
				return
			}
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
			f.P(t(4), "queryParams.push(`", jp, "=${encode(x.toString())}`)")
				f.P(t(4), "})")
			case field.IsMap():
				f.P(t(4), "Object.keys(request.", jp, ").sort().forEach((key) => {")
				f.P(t(5), "const value = request.", jp, "[key];")
				f.P(t(5), "queryParams.push(`", jp, "[${encode(key)}]=${encode(value.toString())}`)")
				f.P(t(4), "})")
			default:
				f.P(t(4), "queryParams.push(`", jp, "=${encode(request.", jp, ".toString())}`)")
		}
		f.P(t(3), "}")
	})
	return queryErr
}

// isWildcardVariable returns true if the variable segment uses a sub-template
// pattern (e.g., {name=shippers/*} or {name=**}), which means slashes within
// the value are semantically significant and must be preserved.
func isWildcardVariable(seg httprule.Segment) bool {
	if seg.Kind != httprule.SegmentKindVariable {
		return false
	}
	// If the variable has only a single * segment, it's a standard variable
	if len(seg.Variable.Segments) == 1 && seg.Variable.Segments[0].Kind == httprule.SegmentKindMatchSingle {
		return false // simple {id} — no sub-template
	}
	return true // has sub-template like {name=shippers/*} or {name=**}
}

// pathStartsWith returns true if path starts with the given prefix segments.
func pathStartsWith(path httprule.FieldPath, prefix []string) bool {
	if len(path) < len(prefix) {
		return false
	}
	for i, seg := range prefix {
		if string(path[i]) != seg {
			return false
		}
	}
	return true
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

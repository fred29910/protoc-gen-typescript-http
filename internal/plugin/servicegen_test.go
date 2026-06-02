package plugin

import (
	"strings"
	"testing"

	"github.com/go-kratos/protoc-gen-typescript-http/internal/codegen"
	"github.com/go-kratos/protoc-gen-typescript-http/internal/httprule"
	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/types/descriptorpb"
	"gotest.tools/v3/assert"
)

func Test_isWildcardVariable(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		segment  httprule.Segment
		expected bool
	}{
		{
			name: "standard variable {id}",
			segment: httprule.Segment{
				Kind: httprule.SegmentKindVariable,
				Variable: httprule.VariableSegment{
					FieldPath: httprule.FieldPath{"id"},
					Segments: []httprule.Segment{
						{Kind: httprule.SegmentKindMatchSingle},
					},
				},
			},
			expected: false,
		},
		{
			name: "nested field variable {message.id}",
			segment: httprule.Segment{
				Kind: httprule.SegmentKindVariable,
				Variable: httprule.VariableSegment{
					FieldPath: httprule.FieldPath{"message", "id"},
					Segments: []httprule.Segment{
						{Kind: httprule.SegmentKindMatchSingle},
					},
				},
			},
			expected: false,
		},
		{
			name: "wildcard sub-template {name=shippers/*}",
			segment: httprule.Segment{
				Kind: httprule.SegmentKindVariable,
				Variable: httprule.VariableSegment{
					FieldPath: httprule.FieldPath{"name"},
					Segments: []httprule.Segment{
						{Kind: httprule.SegmentKindLiteral, Literal: "shippers"},
						{Kind: httprule.SegmentKindMatchSingle},
					},
				},
			},
			expected: true,
		},
		{
			name: "catch-all {name=**}",
			segment: httprule.Segment{
				Kind: httprule.SegmentKindVariable,
				Variable: httprule.VariableSegment{
					FieldPath: httprule.FieldPath{"name"},
					Segments: []httprule.Segment{
						{Kind: httprule.SegmentKindMatchMultiple},
					},
				},
			},
			expected: true,
		},
		{
			name: "non-variable literal segment",
			segment: httprule.Segment{
				Kind:    httprule.SegmentKindLiteral,
				Literal: "static",
			},
			expected: false,
		},
		{
			name: "non-variable match-single segment",
			segment: httprule.Segment{
				Kind: httprule.SegmentKindMatchSingle,
			},
			expected: false,
		},
		{
			name: "non-variable match-multiple segment",
			segment: httprule.Segment{
				Kind: httprule.SegmentKindMatchMultiple,
			},
			expected: false,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := isWildcardVariable(tt.segment)
			assert.Equal(t, tt.expected, got)
		})
	}
}

func Test_pathStartsWith(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		path     httprule.FieldPath
		prefix   []string
		expected bool
	}{
		{
			name:     "path starts with prefix",
			path:     httprule.FieldPath{"nested", "field", "sub"},
			prefix:   []string{"nested", "field"},
			expected: true,
		},
		{
			name:     "path does not start with prefix",
			path:     httprule.FieldPath{"other", "field"},
			prefix:   []string{"nested", "field"},
			expected: false,
		},
		{
			name:     "path shorter than prefix",
			path:     httprule.FieldPath{"nested"},
			prefix:   []string{"nested", "field"},
			expected: false,
		},
		{
			name:     "exact match",
			path:     httprule.FieldPath{"nested"},
			prefix:   []string{"nested"},
			expected: true,
		},
		{
			name:     "empty prefix matches any path",
			path:     httprule.FieldPath{"a", "b", "c"},
			prefix:   []string{},
			expected: true,
		},
		{
			name:     "empty path with empty prefix",
			path:     httprule.FieldPath{},
			prefix:   []string{},
			expected: true,
		},
		{
			name:     "empty path does not match non-empty prefix",
			path:     httprule.FieldPath{},
			prefix:   []string{"something"},
			expected: false,
		},
		{
			name:     "first segment differs",
			path:     httprule.FieldPath{"nested", "other"},
			prefix:   []string{"nested", "field"},
			expected: false,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := pathStartsWith(tt.path, tt.prefix)
			assert.Equal(t, tt.expected, got)
		})
	}
}

func Test_bindingPathVarsPresentExpr(t *testing.T) {
	t.Parallel()
	// Build a minimal message tree:
	//   message Inner { string child = 1; }
	//   message Req   { string id = 1; Inner parent = 2; }
	// The `parent` field is a message (not a string) so that the
	// `parent.child` path can be exercised through nullPropagationPath.
	fd := &descriptorpb.FileDescriptorProto{
		Name:    strPtr("bindtest/present.proto"),
		Package: strPtr("bindtest"),
		MessageType: []*descriptorpb.DescriptorProto{
			{
				Name: strPtr("Inner"),
				Field: []*descriptorpb.FieldDescriptorProto{
					{Name: strPtr("child"), Number: int32Ptr(1), Type: fieldType(descriptorpb.FieldDescriptorProto_TYPE_STRING), JsonName: strPtr("child")},
				},
			},
			{
				Name: strPtr("Req"),
				Field: []*descriptorpb.FieldDescriptorProto{
					{Name: strPtr("id"), Number: int32Ptr(1), Type: fieldType(descriptorpb.FieldDescriptorProto_TYPE_STRING), JsonName: strPtr("id")},
					{Name: strPtr("parent"), Number: int32Ptr(2), Type: fieldType(descriptorpb.FieldDescriptorProto_TYPE_MESSAGE), TypeName: strPtr(".bindtest.Inner"), JsonName: strPtr("parent")},
				},
			},
		},
	}
	file := mustNewFile(t, fd)
	msg := findMessage(t, file, "Req")

	tests := []struct {
		name string
		rule httprule.Rule
		want string
	}{
		{
			name: "no path variables returns true",
			rule: httprule.Rule{
				Template: httprule.Template{
					Segments: []httprule.Segment{
						{Kind: httprule.SegmentKindLiteral, Literal: "v1"},
						{Kind: httprule.SegmentKindLiteral, Literal: "items"},
					},
				},
			},
			want: "true",
		},
		{
			name: "single path variable",
			rule: httprule.Rule{
				Template: httprule.Template{
					Segments: []httprule.Segment{
						{Kind: httprule.SegmentKindLiteral, Literal: "v1"},
						{
							Kind: httprule.SegmentKindVariable,
							Variable: httprule.VariableSegment{
								FieldPath: httprule.FieldPath{"id"},
								Segments:  []httprule.Segment{{Kind: httprule.SegmentKindMatchSingle}},
							},
						},
					},
				},
			},
			want: "request.id !== undefined && request.id !== null",
		},
		{
			name: "nested path variable uses optional chaining",
			rule: httprule.Rule{
				Template: httprule.Template{
					Segments: []httprule.Segment{
						{
							Kind: httprule.SegmentKindVariable,
							Variable: httprule.VariableSegment{
								FieldPath: httprule.FieldPath{"parent", "child"},
								Segments:  []httprule.Segment{{Kind: httprule.SegmentKindMatchSingle}},
							},
						},
					},
				},
			},
			want: "request.parent?.child !== undefined && request.parent?.child !== null",
		},
		{
			name: "multiple path variables joined with &&",
			rule: httprule.Rule{
				Template: httprule.Template{
					Segments: []httprule.Segment{
						{
							Kind: httprule.SegmentKindVariable,
							Variable: httprule.VariableSegment{
								FieldPath: httprule.FieldPath{"parent"},
								Segments:  []httprule.Segment{{Kind: httprule.SegmentKindMatchSingle}},
							},
						},
						{Kind: httprule.SegmentKindLiteral, Literal: "items"},
						{
							Kind: httprule.SegmentKindVariable,
							Variable: httprule.VariableSegment{
								FieldPath: httprule.FieldPath{"id"},
								Segments:  []httprule.Segment{{Kind: httprule.SegmentKindMatchSingle}},
							},
						},
					},
				},
			},
			want: "request.parent !== undefined && request.parent !== null && request.id !== undefined && request.id !== null",
		},
	}
	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got, err := bindingPathVarsPresentExpr(tt.rule, msg)
			assert.NilError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}
}

// dispatchTestFile builds a minimal service file descriptor with one input
// message (Req), one empty output message (Out), and one service (Svc) that
// exposes a single method (M). It is used by the writeMethodDispatchBody
// tests below to drive the multi-binding dispatch seam without having to
// attach google.api.http annotations via protodesc.
func dispatchTestFile(t *testing.T) (protoreflect.ServiceDescriptor, protoreflect.MethodDescriptor) {
	t.Helper()
	fd := &descriptorpb.FileDescriptorProto{
		Name:    strPtr("bindtest/dispatch.proto"),
		Package: strPtr("bindtest"),
		MessageType: []*descriptorpb.DescriptorProto{
			{
				Name: strPtr("Req"),
				Field: []*descriptorpb.FieldDescriptorProto{
					{Name: strPtr("id"), Number: int32Ptr(1), Type: fieldType(descriptorpb.FieldDescriptorProto_TYPE_STRING), JsonName: strPtr("id")},
				},
			},
			{Name: strPtr("Out")},
		},
		Service: []*descriptorpb.ServiceDescriptorProto{
			{
				Name: strPtr("Svc"),
				Method: []*descriptorpb.MethodDescriptorProto{
					{
						Name:       strPtr("M"),
						InputType:  strPtr(".bindtest.Req"),
						OutputType: strPtr(".bindtest.Out"),
					},
				},
			},
		},
	}
	file := mustNewFile(t, fd)
	return file.Services().Get(0), file.Services().Get(0).Methods().Get(0)
}

func Test_writeMethodDispatchBody_singleBindingFastPath(t *testing.T) {
	t.Parallel()
	svc, method := dispatchTestFile(t)
	gen := serviceGenerator{pkg: "bindtest", genHandler: false, service: svc}

	// Single binding: should NOT wrap in an if/else if/else chain.
	rules := []httprule.Rule{
		{
			Method: "GET",
			Template: httprule.Template{
				Segments: []httprule.Segment{
					{Kind: httprule.SegmentKindLiteral, Literal: "v1"},
				},
			},
		},
	}

	f := &codegen.File{}
	err := gen.writeMethodDispatchBody(f, method, rules)
	assert.NilError(t, err)
	out := string(f.Content())

	// Method body was opened.
	assert.Assert(t, strings.Contains(out, "M(request) {"), "expected method opener in: %s", out)
	// Fast path: no multi-binding dispatch (no `} else if` chain, no
	// trailing throw). Inner query generation can still emit its own
	// `if (...)` for nullable query fields — that's the pre-existing
	// single-binding output, not the new dispatch.
	assert.Assert(t, !strings.Contains(out, "} else if ("), "single-binding fast path should not emit `} else if (...)`, got: %s", out)
	assert.Assert(t, !strings.Contains(out, "throw new Error"), "single-binding fast path should not emit throw, got: %s", out)
	// HTTP method still flows through writeMethodHandlerCall.
	assert.Assert(t, strings.Contains(out, "method: \"GET\""), "expected method GET, got: %s", out)
}

func Test_writeMethodDispatchBody_multiBindingStructure(t *testing.T) {
	t.Parallel()
	svc, method := dispatchTestFile(t)
	gen := serviceGenerator{pkg: "bindtest", genHandler: false, service: svc}

	// Two bindings:
	//   1. /v1/{id}  — guards on request.id
	//   2. /list     — no path vars, so bindingPathVarsPresentExpr returns "true"
	rules := []httprule.Rule{
		{
			Method: "GET",
			Template: httprule.Template{
				Segments: []httprule.Segment{
					{Kind: httprule.SegmentKindLiteral, Literal: "v1"},
					{
						Kind: httprule.SegmentKindVariable,
						Variable: httprule.VariableSegment{
							FieldPath: httprule.FieldPath{"id"},
							Segments:  []httprule.Segment{{Kind: httprule.SegmentKindMatchSingle}},
						},
					},
				},
			},
		},
		{
			Method: "POST",
			Template: httprule.Template{
				Segments: []httprule.Segment{
					{Kind: httprule.SegmentKindLiteral, Literal: "list"},
				},
			},
		},
	}

	f := &codegen.File{}
	err := gen.writeMethodDispatchBody(f, method, rules)
	assert.NilError(t, err)
	out := string(f.Content())

	// Multi-binding dispatch emits an if / else if / else chain.
	assert.Assert(t, strings.Contains(out, "if (request.id !== undefined && request.id !== null) {"), "expected first binding guard, got: %s", out)
	assert.Assert(t, strings.Contains(out, "} else if (true) {"), "expected second binding guard, got: %s", out)
	assert.Assert(t, strings.Contains(out, "} else {"), "expected trailing else, got: %s", out)
	// The trailing else throws a meaningful error mentioning the method name.
	assert.Assert(t, strings.Contains(out, "throw new Error(\"no matching binding for M\")"), "expected throw, got: %s", out)
	// Both HTTP methods flow through writeMethodHandlerCall.
	assert.Assert(t, strings.Contains(out, "method: \"GET\""), "expected first binding method, got: %s", out)
	assert.Assert(t, strings.Contains(out, "method: \"POST\""), "expected second binding method, got: %s", out)
}

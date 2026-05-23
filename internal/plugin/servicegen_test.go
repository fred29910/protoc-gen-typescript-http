package plugin

import (
	"testing"

	"github.com/go-kratos/protoc-gen-typescript-http/internal/httprule"
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

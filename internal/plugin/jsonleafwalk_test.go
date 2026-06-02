package plugin

import (
	"testing"
	"time"

	"github.com/go-kratos/protoc-gen-typescript-http/internal/httprule"
	"google.golang.org/protobuf/reflect/protodesc"
	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/types/descriptorpb"
	"gotest.tools/v3/assert"
)

func timeAfter() <-chan time.Time {
	return time.After(2 * time.Second)
}

func strPtr(s string) *string { return &s }

func int32Ptr(i int32) *int32 { return &i }

func fieldType(t descriptorpb.FieldDescriptorProto_Type) *descriptorpb.FieldDescriptorProto_Type {
	v := t
	return &v
}

func mustNewFile(t *testing.T, fd *descriptorpb.FileDescriptorProto) protoreflect.FileDescriptor {
	t.Helper()
	file, err := protodesc.NewFile(fd, nil)
	assert.NilError(t, err)
	return file
}

func findMessage(t *testing.T, file protoreflect.FileDescriptor, name string) protoreflect.MessageDescriptor {
	t.Helper()
	msgs := file.Messages()
	for i := 0; i < msgs.Len(); i++ {
		if string(msgs.Get(i).Name()) == name {
			return msgs.Get(i)
		}
	}
	t.Fatalf("message %q not found in file %s", name, file.Path())
	return nil
}

func collectLeaves(message protoreflect.MessageDescriptor) []string {
	var paths []string
	walkJSONLeafFields(message, func(path httprule.FieldPath, _ protoreflect.FieldDescriptor) {
		paths = append(paths, path.String())
	})
	return paths
}

func Test_walkJSONLeafFields_sameMessageTypeAtMultipleSiblingFields(t *testing.T) {
	t.Parallel()
	// Reproduces the P0 bug:
	//   message Address { string city = 1; string street = 2; }
	//   message RouteRequest { Address source = 1; Address destination = 2; }
	// Both source and destination reference Address, so every leaf under each
	// must appear. The current global-seen implementation drops destination.
	fd := &descriptorpb.FileDescriptorProto{
		Name:    strPtr("walktest/siblings.proto"),
		Package: strPtr("walktest"),
		MessageType: []*descriptorpb.DescriptorProto{
			{
				Name: strPtr("Address"),
				Field: []*descriptorpb.FieldDescriptorProto{
					{Name: strPtr("city"), Number: int32Ptr(1), Type: fieldType(descriptorpb.FieldDescriptorProto_TYPE_STRING), JsonName: strPtr("city")},
					{Name: strPtr("street"), Number: int32Ptr(2), Type: fieldType(descriptorpb.FieldDescriptorProto_TYPE_STRING), JsonName: strPtr("street")},
				},
			},
			{
				Name: strPtr("RouteRequest"),
				Field: []*descriptorpb.FieldDescriptorProto{
					{Name: strPtr("source"), Number: int32Ptr(1), Type: fieldType(descriptorpb.FieldDescriptorProto_TYPE_MESSAGE), TypeName: strPtr(".walktest.Address"), JsonName: strPtr("source")},
					{Name: strPtr("destination"), Number: int32Ptr(2), Type: fieldType(descriptorpb.FieldDescriptorProto_TYPE_MESSAGE), TypeName: strPtr(".walktest.Address"), JsonName: strPtr("destination")},
				},
			},
		},
	}
	file := mustNewFile(t, fd)
	request := findMessage(t, file, "RouteRequest")

	got := collectLeaves(request)
	want := []string{
		"source.city",
		"source.street",
		"destination.city",
		"destination.street",
	}
	assert.DeepEqual(t, got, want)
}

func Test_walkJSONLeafFields_sameMessageTypeInSeparateBranches(t *testing.T) {
	t.Parallel()
	// Diamond-like layout where the same message type appears through
	// different containers at different depths:
	//   A { string a; }
	//   B { A x; }
	//   C { A y; }
	//   D { B b; C c; }
	// Both b.x.a and c.y.a must be visited.
	fd := &descriptorpb.FileDescriptorProto{
		Name:    strPtr("walktest/diamond.proto"),
		Package: strPtr("walktest"),
		MessageType: []*descriptorpb.DescriptorProto{
			{
				Name: strPtr("A"),
				Field: []*descriptorpb.FieldDescriptorProto{
					{Name: strPtr("a"), Number: int32Ptr(1), Type: fieldType(descriptorpb.FieldDescriptorProto_TYPE_STRING), JsonName: strPtr("a")},
				},
			},
			{
				Name: strPtr("B"),
				Field: []*descriptorpb.FieldDescriptorProto{
					{Name: strPtr("x"), Number: int32Ptr(1), Type: fieldType(descriptorpb.FieldDescriptorProto_TYPE_MESSAGE), TypeName: strPtr(".walktest.A"), JsonName: strPtr("x")},
				},
			},
			{
				Name: strPtr("C"),
				Field: []*descriptorpb.FieldDescriptorProto{
					{Name: strPtr("y"), Number: int32Ptr(1), Type: fieldType(descriptorpb.FieldDescriptorProto_TYPE_MESSAGE), TypeName: strPtr(".walktest.A"), JsonName: strPtr("y")},
				},
			},
			{
				Name: strPtr("D"),
				Field: []*descriptorpb.FieldDescriptorProto{
					{Name: strPtr("b"), Number: int32Ptr(1), Type: fieldType(descriptorpb.FieldDescriptorProto_TYPE_MESSAGE), TypeName: strPtr(".walktest.B"), JsonName: strPtr("b")},
					{Name: strPtr("c"), Number: int32Ptr(2), Type: fieldType(descriptorpb.FieldDescriptorProto_TYPE_MESSAGE), TypeName: strPtr(".walktest.C"), JsonName: strPtr("c")},
				},
			},
		},
	}
	file := mustNewFile(t, fd)
	d := findMessage(t, file, "D")

	got := collectLeaves(d)
	want := []string{"b.x.a", "c.y.a"}
	assert.DeepEqual(t, got, want)
}

func Test_walkJSONLeafFields_selfReferenceDoesNotLoop(t *testing.T) {
	t.Parallel()
	// The original motivation for the seen map was to prevent infinite
	// recursion on self-referencing types like:
	//   message TreeNode { string name = 1; TreeNode child = 2; }
	// After the fix this must still terminate and produce a finite leaf list
	// that contains name and child (with the cycle broken at child).
	fd := &descriptorpb.FileDescriptorProto{
		Name:    strPtr("walktest/tree.proto"),
		Package: strPtr("walktest"),
		MessageType: []*descriptorpb.DescriptorProto{
			{
				Name: strPtr("TreeNode"),
				Field: []*descriptorpb.FieldDescriptorProto{
					{Name: strPtr("name"), Number: int32Ptr(1), Type: fieldType(descriptorpb.FieldDescriptorProto_TYPE_STRING), JsonName: strPtr("name")},
					{Name: strPtr("child"), Number: int32Ptr(2), Type: fieldType(descriptorpb.FieldDescriptorProto_TYPE_MESSAGE), TypeName: strPtr(".walktest.TreeNode"), JsonName: strPtr("child")},
				},
			},
		},
	}
	file := mustNewFile(t, fd)
	tree := findMessage(t, file, "TreeNode")

	done := make(chan []string, 1)
	go func() {
		done <- collectLeaves(tree)
	}()
	var got []string
	select {
	case got = <-done:
	case <-timeAfter():
		t.Fatal("walkJSONLeafFields did not terminate on self-referencing message")
	}
	want := []string{"name"}
	assert.DeepEqual(t, got, want)
}

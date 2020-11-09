<<<<<<< HEAD
// Copyright 2016 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package ptypes

import (
	"fmt"
	"strings"

	"github.com/golang/protobuf/proto"
	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/reflect/protoregistry"

	anypb "github.com/golang/protobuf/ptypes/any"
)

const urlPrefix = "type.googleapis.com/"

// AnyMessageName returns the message name contained in an anypb.Any message.
// Most type assertions should use the Is function instead.
func AnyMessageName(any *anypb.Any) (string, error) {
	name, err := anyMessageName(any)
	return string(name), err
}
func anyMessageName(any *anypb.Any) (protoreflect.FullName, error) {
	if any == nil {
		return "", fmt.Errorf("message is nil")
	}
	name := protoreflect.FullName(any.TypeUrl)
	if i := strings.LastIndex(any.TypeUrl, "/"); i >= 0 {
		name = name[i+len("/"):]
	}
	if !name.IsValid() {
		return "", fmt.Errorf("message type url %q is invalid", any.TypeUrl)
	}
	return name, nil
}

// MarshalAny marshals the given message m into an anypb.Any message.
func MarshalAny(m proto.Message) (*anypb.Any, error) {
	switch dm := m.(type) {
	case DynamicAny:
		m = dm.Message
	case *DynamicAny:
		if dm == nil {
			return nil, proto.ErrNil
		}
		m = dm.Message
	}
	b, err := proto.Marshal(m)
	if err != nil {
		return nil, err
	}
	return &anypb.Any{TypeUrl: urlPrefix + proto.MessageName(m), Value: b}, nil
}

// Empty returns a new message of the type specified in an anypb.Any message.
// It returns protoregistry.NotFound if the corresponding message type could not
// be resolved in the global registry.
func Empty(any *anypb.Any) (proto.Message, error) {
	name, err := anyMessageName(any)
	if err != nil {
		return nil, err
	}
	mt, err := protoregistry.GlobalTypes.FindMessageByName(name)
	if err != nil {
		return nil, err
	}
	return proto.MessageV1(mt.New().Interface()), nil
}

// UnmarshalAny unmarshals the encoded value contained in the anypb.Any message
// into the provided message m. It returns an error if the target message
// does not match the type in the Any message or if an unmarshal error occurs.
//
// The target message m may be a *DynamicAny message. If the underlying message
// type could not be resolved, then this returns protoregistry.NotFound.
func UnmarshalAny(any *anypb.Any, m proto.Message) error {
	if dm, ok := m.(*DynamicAny); ok {
		if dm.Message == nil {
			var err error
			dm.Message, err = Empty(any)
=======
// Go support for Protocol Buffers - Google's data interchange format
//
// Copyright 2016 The Go Authors.  All rights reserved.
// https://github.com/golang/protobuf
//
// Redistribution and use in source and binary forms, with or without
// modification, are permitted provided that the following conditions are
// met:
//
//     * Redistributions of source code must retain the above copyright
// notice, this list of conditions and the following disclaimer.
//     * Redistributions in binary form must reproduce the above
// copyright notice, this list of conditions and the following disclaimer
// in the documentation and/or other materials provided with the
// distribution.
//     * Neither the name of Google Inc. nor the names of its
// contributors may be used to endorse or promote products derived from
// this software without specific prior written permission.
//
// THIS SOFTWARE IS PROVIDED BY THE COPYRIGHT HOLDERS AND CONTRIBUTORS
// "AS IS" AND ANY EXPRESS OR IMPLIED WARRANTIES, INCLUDING, BUT NOT
// LIMITED TO, THE IMPLIED WARRANTIES OF MERCHANTABILITY AND FITNESS FOR
// A PARTICULAR PURPOSE ARE DISCLAIMED. IN NO EVENT SHALL THE COPYRIGHT
// OWNER OR CONTRIBUTORS BE LIABLE FOR ANY DIRECT, INDIRECT, INCIDENTAL,
// SPECIAL, EXEMPLARY, OR CONSEQUENTIAL DAMAGES (INCLUDING, BUT NOT
// LIMITED TO, PROCUREMENT OF SUBSTITUTE GOODS OR SERVICES; LOSS OF USE,
// DATA, OR PROFITS; OR BUSINESS INTERRUPTION) HOWEVER CAUSED AND ON ANY
// THEORY OF LIABILITY, WHETHER IN CONTRACT, STRICT LIABILITY, OR TORT
// (INCLUDING NEGLIGENCE OR OTHERWISE) ARISING IN ANY WAY OUT OF THE USE
// OF THIS SOFTWARE, EVEN IF ADVISED OF THE POSSIBILITY OF SUCH DAMAGE.

package ptypes

// This file implements functions to marshal proto.Message to/from
// google.protobuf.Any message.

import (
	"fmt"
	"reflect"
	"strings"

	"github.com/golang/protobuf/proto"
	"github.com/golang/protobuf/ptypes/any"
)

const googleApis = "type.googleapis.com/"

// AnyMessageName returns the name of the message contained in a google.protobuf.Any message.
//
// Note that regular type assertions should be done using the Is
// function. AnyMessageName is provided for less common use cases like filtering a
// sequence of Any messages based on a set of allowed message type names.
func AnyMessageName(any *any.Any) (string, error) {
	if any == nil {
		return "", fmt.Errorf("message is nil")
	}
	slash := strings.LastIndex(any.TypeUrl, "/")
	if slash < 0 {
		return "", fmt.Errorf("message type url %q is invalid", any.TypeUrl)
	}
	return any.TypeUrl[slash+1:], nil
}

// MarshalAny takes the protocol buffer and encodes it into google.protobuf.Any.
func MarshalAny(pb proto.Message) (*any.Any, error) {
	value, err := proto.Marshal(pb)
	if err != nil {
		return nil, err
	}
	return &any.Any{TypeUrl: googleApis + proto.MessageName(pb), Value: value}, nil
}

// DynamicAny is a value that can be passed to UnmarshalAny to automatically
// allocate a proto.Message for the type specified in a google.protobuf.Any
// message. The allocated message is stored in the embedded proto.Message.
//
// Example:
//
//   var x ptypes.DynamicAny
//   if err := ptypes.UnmarshalAny(a, &x); err != nil { ... }
//   fmt.Printf("unmarshaled message: %v", x.Message)
type DynamicAny struct {
	proto.Message
}

// Empty returns a new proto.Message of the type specified in a
// google.protobuf.Any message. It returns an error if corresponding message
// type isn't linked in.
func Empty(any *any.Any) (proto.Message, error) {
	aname, err := AnyMessageName(any)
	if err != nil {
		return nil, err
	}

	t := proto.MessageType(aname)
	if t == nil {
		return nil, fmt.Errorf("any: message type %q isn't linked in", aname)
	}
	return reflect.New(t.Elem()).Interface().(proto.Message), nil
}

// UnmarshalAny parses the protocol buffer representation in a google.protobuf.Any
// message and places the decoded result in pb. It returns an error if type of
// contents of Any message does not match type of pb message.
//
// pb can be a proto.Message, or a *DynamicAny.
func UnmarshalAny(any *any.Any, pb proto.Message) error {
	if d, ok := pb.(*DynamicAny); ok {
		if d.Message == nil {
			var err error
			d.Message, err = Empty(any)
>>>>>>> cbc9bb05... fixup add vendor back
			if err != nil {
				return err
			}
		}
<<<<<<< HEAD
		m = dm.Message
	}

	anyName, err := AnyMessageName(any)
	if err != nil {
		return err
	}
	msgName := proto.MessageName(m)
	if anyName != msgName {
		return fmt.Errorf("mismatched message type: got %q want %q", anyName, msgName)
	}
	return proto.Unmarshal(any.Value, m)
}

// Is reports whether the Any message contains a message of the specified type.
func Is(any *anypb.Any, m proto.Message) bool {
	if any == nil || m == nil {
		return false
	}
	name := proto.MessageName(m)
	if !strings.HasSuffix(any.TypeUrl, name) {
		return false
	}
	return len(any.TypeUrl) == len(name) || any.TypeUrl[len(any.TypeUrl)-len(name)-1] == '/'
}

// DynamicAny is a value that can be passed to UnmarshalAny to automatically
// allocate a proto.Message for the type specified in an anypb.Any message.
// The allocated message is stored in the embedded proto.Message.
//
// Example:
//   var x ptypes.DynamicAny
//   if err := ptypes.UnmarshalAny(a, &x); err != nil { ... }
//   fmt.Printf("unmarshaled message: %v", x.Message)
type DynamicAny struct{ proto.Message }

func (m DynamicAny) String() string {
	if m.Message == nil {
		return "<nil>"
	}
	return m.Message.String()
}
func (m DynamicAny) Reset() {
	if m.Message == nil {
		return
	}
	m.Message.Reset()
}
func (m DynamicAny) ProtoMessage() {
	return
}
func (m DynamicAny) ProtoReflect() protoreflect.Message {
	if m.Message == nil {
		return nil
	}
	return dynamicAny{proto.MessageReflect(m.Message)}
}

type dynamicAny struct{ protoreflect.Message }

func (m dynamicAny) Type() protoreflect.MessageType {
	return dynamicAnyType{m.Message.Type()}
}
func (m dynamicAny) New() protoreflect.Message {
	return dynamicAnyType{m.Message.Type()}.New()
}
func (m dynamicAny) Interface() protoreflect.ProtoMessage {
	return DynamicAny{proto.MessageV1(m.Message.Interface())}
}

type dynamicAnyType struct{ protoreflect.MessageType }

func (t dynamicAnyType) New() protoreflect.Message {
	return dynamicAny{t.MessageType.New()}
}
func (t dynamicAnyType) Zero() protoreflect.Message {
	return dynamicAny{t.MessageType.Zero()}
=======
		return UnmarshalAny(any, d.Message)
	}

	aname, err := AnyMessageName(any)
	if err != nil {
		return err
	}

	mname := proto.MessageName(pb)
	if aname != mname {
		return fmt.Errorf("mismatched message type: got %q want %q", aname, mname)
	}
	return proto.Unmarshal(any.Value, pb)
}

// Is returns true if any value contains a given message type.
func Is(any *any.Any, pb proto.Message) bool {
	// The following is equivalent to AnyMessageName(any) == proto.MessageName(pb),
	// but it avoids scanning TypeUrl for the slash.
	if any == nil {
		return false
	}
	name := proto.MessageName(pb)
	prefix := len(any.TypeUrl) - len(name)
	return prefix >= 1 && any.TypeUrl[prefix-1] == '/' && any.TypeUrl[prefix:] == name
>>>>>>> cbc9bb05... fixup add vendor back
}

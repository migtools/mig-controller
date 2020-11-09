<<<<<<< HEAD
// Copyright 2010 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package proto

import (
	"fmt"
	"reflect"
	"strconv"
	"strings"
	"sync"

	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/runtime/protoimpl"
)

// StructProperties represents protocol buffer type information for a
// generated protobuf message in the open-struct API.
//
// Deprecated: Do not use.
type StructProperties struct {
	// Prop are the properties for each field.
	//
	// Fields belonging to a oneof are stored in OneofTypes instead, with a
	// single Properties representing the parent oneof held here.
	//
	// The order of Prop matches the order of fields in the Go struct.
	// Struct fields that are not related to protobufs have a "XXX_" prefix
	// in the Properties.Name and must be ignored by the user.
	Prop []*Properties

	// OneofTypes contains information about the oneof fields in this message.
	// It is keyed by the protobuf field name.
	OneofTypes map[string]*OneofProperties
}

// Properties represents the type information for a protobuf message field.
//
// Deprecated: Do not use.
type Properties struct {
	// Name is a placeholder name with little meaningful semantic value.
	// If the name has an "XXX_" prefix, the entire Properties must be ignored.
	Name string
	// OrigName is the protobuf field name or oneof name.
	OrigName string
	// JSONName is the JSON name for the protobuf field.
	JSONName string
	// Enum is a placeholder name for enums.
	// For historical reasons, this is neither the Go name for the enum,
	// nor the protobuf name for the enum.
	Enum string // Deprecated: Do not use.
	// Weak contains the full name of the weakly referenced message.
	Weak string
	// Wire is a string representation of the wire type.
	Wire string
	// WireType is the protobuf wire type for the field.
	WireType int
	// Tag is the protobuf field number.
	Tag int
	// Required reports whether this is a required field.
	Required bool
	// Optional reports whether this is a optional field.
	Optional bool
	// Repeated reports whether this is a repeated field.
	Repeated bool
	// Packed reports whether this is a packed repeated field of scalars.
	Packed bool
	// Proto3 reports whether this field operates under the proto3 syntax.
	Proto3 bool
	// Oneof reports whether this field belongs within a oneof.
	Oneof bool

	// Default is the default value in string form.
	Default string
	// HasDefault reports whether the field has a default value.
	HasDefault bool

	// MapKeyProp is the properties for the key field for a map field.
	MapKeyProp *Properties
	// MapValProp is the properties for the value field for a map field.
	MapValProp *Properties
}

// OneofProperties represents the type information for a protobuf oneof.
//
// Deprecated: Do not use.
type OneofProperties struct {
	// Type is a pointer to the generated wrapper type for the field value.
	// This is nil for messages that are not in the open-struct API.
	Type reflect.Type
	// Field is the index into StructProperties.Prop for the containing oneof.
	Field int
	// Prop is the properties for the field.
	Prop *Properties
=======
// Go support for Protocol Buffers - Google's data interchange format
//
// Copyright 2010 The Go Authors.  All rights reserved.
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

package proto

/*
 * Routines for encoding data into the wire format for protocol buffers.
 */

import (
	"fmt"
	"log"
	"os"
	"reflect"
	"sort"
	"strconv"
	"strings"
	"sync"
)

const debug bool = false

// Constants that identify the encoding of a value on the wire.
const (
	WireVarint     = 0
	WireFixed64    = 1
	WireBytes      = 2
	WireStartGroup = 3
	WireEndGroup   = 4
	WireFixed32    = 5
)

// tagMap is an optimization over map[int]int for typical protocol buffer
// use-cases. Encoded protocol buffers are often in tag order with small tag
// numbers.
type tagMap struct {
	fastTags []int
	slowTags map[int]int
}

// tagMapFastLimit is the upper bound on the tag number that will be stored in
// the tagMap slice rather than its map.
const tagMapFastLimit = 1024

func (p *tagMap) get(t int) (int, bool) {
	if t > 0 && t < tagMapFastLimit {
		if t >= len(p.fastTags) {
			return 0, false
		}
		fi := p.fastTags[t]
		return fi, fi >= 0
	}
	fi, ok := p.slowTags[t]
	return fi, ok
}

func (p *tagMap) put(t int, fi int) {
	if t > 0 && t < tagMapFastLimit {
		for len(p.fastTags) < t+1 {
			p.fastTags = append(p.fastTags, -1)
		}
		p.fastTags[t] = fi
		return
	}
	if p.slowTags == nil {
		p.slowTags = make(map[int]int)
	}
	p.slowTags[t] = fi
}

// StructProperties represents properties for all the fields of a struct.
// decoderTags and decoderOrigNames should only be used by the decoder.
type StructProperties struct {
	Prop             []*Properties  // properties for each field
	reqCount         int            // required count
	decoderTags      tagMap         // map from proto tag to struct field number
	decoderOrigNames map[string]int // map from original name to struct field number
	order            []int          // list of struct field numbers in tag order

	// OneofTypes contains information about the oneof fields in this message.
	// It is keyed by the original name of a field.
	OneofTypes map[string]*OneofProperties
}

// OneofProperties represents information about a specific field in a oneof.
type OneofProperties struct {
	Type  reflect.Type // pointer to generated struct type for this oneof field
	Field int          // struct field number of the containing oneof in the message
	Prop  *Properties
}

// Implement the sorting interface so we can sort the fields in tag order, as recommended by the spec.
// See encode.go, (*Buffer).enc_struct.

func (sp *StructProperties) Len() int { return len(sp.order) }
func (sp *StructProperties) Less(i, j int) bool {
	return sp.Prop[sp.order[i]].Tag < sp.Prop[sp.order[j]].Tag
}
func (sp *StructProperties) Swap(i, j int) { sp.order[i], sp.order[j] = sp.order[j], sp.order[i] }

// Properties represents the protocol-specific behavior of a single struct field.
type Properties struct {
	Name     string // name of the field, for error messages
	OrigName string // original name before protocol compiler (always set)
	JSONName string // name to use for JSON; determined by protoc
	Wire     string
	WireType int
	Tag      int
	Required bool
	Optional bool
	Repeated bool
	Packed   bool   // relevant for repeated primitives only
	Enum     string // set for enum types only
	proto3   bool   // whether this is known to be a proto3 field
	oneof    bool   // whether this is a oneof field

	Default    string // default value
	HasDefault bool   // whether an explicit default was provided

	stype reflect.Type      // set for struct types only
	sprop *StructProperties // set for struct types only

	mtype      reflect.Type // set for map types only
	MapKeyProp *Properties  // set for map types only
	MapValProp *Properties  // set for map types only
>>>>>>> cbc9bb05... fixup add vendor back
}

// String formats the properties in the protobuf struct field tag style.
func (p *Properties) String() string {
	s := p.Wire
<<<<<<< HEAD
	s += "," + strconv.Itoa(p.Tag)
=======
	s += ","
	s += strconv.Itoa(p.Tag)
>>>>>>> cbc9bb05... fixup add vendor back
	if p.Required {
		s += ",req"
	}
	if p.Optional {
		s += ",opt"
	}
	if p.Repeated {
		s += ",rep"
	}
	if p.Packed {
		s += ",packed"
	}
	s += ",name=" + p.OrigName
<<<<<<< HEAD
	if p.JSONName != "" {
		s += ",json=" + p.JSONName
	}
	if len(p.Enum) > 0 {
		s += ",enum=" + p.Enum
	}
	if len(p.Weak) > 0 {
		s += ",weak=" + p.Weak
	}
	if p.Proto3 {
		s += ",proto3"
	}
	if p.Oneof {
		s += ",oneof"
	}
=======
	if p.JSONName != p.OrigName {
		s += ",json=" + p.JSONName
	}
	if p.proto3 {
		s += ",proto3"
	}
	if p.oneof {
		s += ",oneof"
	}
	if len(p.Enum) > 0 {
		s += ",enum=" + p.Enum
	}
>>>>>>> cbc9bb05... fixup add vendor back
	if p.HasDefault {
		s += ",def=" + p.Default
	}
	return s
}

// Parse populates p by parsing a string in the protobuf struct field tag style.
<<<<<<< HEAD
func (p *Properties) Parse(tag string) {
	// For example: "bytes,49,opt,name=foo,def=hello!"
	for len(tag) > 0 {
		i := strings.IndexByte(tag, ',')
		if i < 0 {
			i = len(tag)
		}
		switch s := tag[:i]; {
		case strings.HasPrefix(s, "name="):
			p.OrigName = s[len("name="):]
		case strings.HasPrefix(s, "json="):
			p.JSONName = s[len("json="):]
		case strings.HasPrefix(s, "enum="):
			p.Enum = s[len("enum="):]
		case strings.HasPrefix(s, "weak="):
			p.Weak = s[len("weak="):]
		case strings.Trim(s, "0123456789") == "":
			n, _ := strconv.ParseUint(s, 10, 32)
			p.Tag = int(n)
		case s == "opt":
			p.Optional = true
		case s == "req":
			p.Required = true
		case s == "rep":
			p.Repeated = true
		case s == "varint" || s == "zigzag32" || s == "zigzag64":
			p.Wire = s
			p.WireType = WireVarint
		case s == "fixed32":
			p.Wire = s
			p.WireType = WireFixed32
		case s == "fixed64":
			p.Wire = s
			p.WireType = WireFixed64
		case s == "bytes":
			p.Wire = s
			p.WireType = WireBytes
		case s == "group":
			p.Wire = s
			p.WireType = WireStartGroup
		case s == "packed":
			p.Packed = true
		case s == "proto3":
			p.Proto3 = true
		case s == "oneof":
			p.Oneof = true
		case strings.HasPrefix(s, "def="):
			// The default tag is special in that everything afterwards is the
			// default regardless of the presence of commas.
			p.HasDefault = true
			p.Default, i = tag[len("def="):], len(tag)
		}
		tag = strings.TrimPrefix(tag[i:], ",")
	}
}

// Init populates the properties from a protocol buffer struct tag.
//
// Deprecated: Do not use.
func (p *Properties) Init(typ reflect.Type, name, tag string, f *reflect.StructField) {
=======
func (p *Properties) Parse(s string) {
	// "bytes,49,opt,name=foo,def=hello!"
	fields := strings.Split(s, ",") // breaks def=, but handled below.
	if len(fields) < 2 {
		fmt.Fprintf(os.Stderr, "proto: tag has too few fields: %q\n", s)
		return
	}

	p.Wire = fields[0]
	switch p.Wire {
	case "varint":
		p.WireType = WireVarint
	case "fixed32":
		p.WireType = WireFixed32
	case "fixed64":
		p.WireType = WireFixed64
	case "zigzag32":
		p.WireType = WireVarint
	case "zigzag64":
		p.WireType = WireVarint
	case "bytes", "group":
		p.WireType = WireBytes
		// no numeric converter for non-numeric types
	default:
		fmt.Fprintf(os.Stderr, "proto: tag has unknown wire type: %q\n", s)
		return
	}

	var err error
	p.Tag, err = strconv.Atoi(fields[1])
	if err != nil {
		return
	}

outer:
	for i := 2; i < len(fields); i++ {
		f := fields[i]
		switch {
		case f == "req":
			p.Required = true
		case f == "opt":
			p.Optional = true
		case f == "rep":
			p.Repeated = true
		case f == "packed":
			p.Packed = true
		case strings.HasPrefix(f, "name="):
			p.OrigName = f[5:]
		case strings.HasPrefix(f, "json="):
			p.JSONName = f[5:]
		case strings.HasPrefix(f, "enum="):
			p.Enum = f[5:]
		case f == "proto3":
			p.proto3 = true
		case f == "oneof":
			p.oneof = true
		case strings.HasPrefix(f, "def="):
			p.HasDefault = true
			p.Default = f[4:] // rest of string
			if i+1 < len(fields) {
				// Commas aren't escaped, and def is always last.
				p.Default += "," + strings.Join(fields[i+1:], ",")
				break outer
			}
		}
	}
}

var protoMessageType = reflect.TypeOf((*Message)(nil)).Elem()

// setFieldProps initializes the field properties for submessages and maps.
func (p *Properties) setFieldProps(typ reflect.Type, f *reflect.StructField, lockGetProp bool) {
	switch t1 := typ; t1.Kind() {
	case reflect.Ptr:
		if t1.Elem().Kind() == reflect.Struct {
			p.stype = t1.Elem()
		}

	case reflect.Slice:
		if t2 := t1.Elem(); t2.Kind() == reflect.Ptr && t2.Elem().Kind() == reflect.Struct {
			p.stype = t2.Elem()
		}

	case reflect.Map:
		p.mtype = t1
		p.MapKeyProp = &Properties{}
		p.MapKeyProp.init(reflect.PtrTo(p.mtype.Key()), "Key", f.Tag.Get("protobuf_key"), nil, lockGetProp)
		p.MapValProp = &Properties{}
		vtype := p.mtype.Elem()
		if vtype.Kind() != reflect.Ptr && vtype.Kind() != reflect.Slice {
			// The value type is not a message (*T) or bytes ([]byte),
			// so we need encoders for the pointer to this type.
			vtype = reflect.PtrTo(vtype)
		}
		p.MapValProp.init(vtype, "Value", f.Tag.Get("protobuf_val"), nil, lockGetProp)
	}

	if p.stype != nil {
		if lockGetProp {
			p.sprop = GetProperties(p.stype)
		} else {
			p.sprop = getPropertiesLocked(p.stype)
		}
	}
}

var (
	marshalerType = reflect.TypeOf((*Marshaler)(nil)).Elem()
)

// Init populates the properties from a protocol buffer struct tag.
func (p *Properties) Init(typ reflect.Type, name, tag string, f *reflect.StructField) {
	p.init(typ, name, tag, f, true)
}

func (p *Properties) init(typ reflect.Type, name, tag string, f *reflect.StructField, lockGetProp bool) {
	// "bytes,49,opt,def=hello!"
>>>>>>> cbc9bb05... fixup add vendor back
	p.Name = name
	p.OrigName = name
	if tag == "" {
		return
	}
	p.Parse(tag)
<<<<<<< HEAD

	if typ != nil && typ.Kind() == reflect.Map {
		p.MapKeyProp = new(Properties)
		p.MapKeyProp.Init(nil, "Key", f.Tag.Get("protobuf_key"), nil)
		p.MapValProp = new(Properties)
		p.MapValProp.Init(nil, "Value", f.Tag.Get("protobuf_val"), nil)
	}
}

var propertiesCache sync.Map // map[reflect.Type]*StructProperties

// GetProperties returns the list of properties for the type represented by t,
// which must be a generated protocol buffer message in the open-struct API,
// where protobuf message fields are represented by exported Go struct fields.
//
// Deprecated: Use protobuf reflection instead.
func GetProperties(t reflect.Type) *StructProperties {
	if p, ok := propertiesCache.Load(t); ok {
		return p.(*StructProperties)
	}
	p, _ := propertiesCache.LoadOrStore(t, newProperties(t))
	return p.(*StructProperties)
}

func newProperties(t reflect.Type) *StructProperties {
	if t.Kind() != reflect.Struct {
		panic(fmt.Sprintf("%v is not a generated message in the open-struct API", t))
	}

	var hasOneof bool
	prop := new(StructProperties)

	// Construct a list of properties for each field in the struct.
	for i := 0; i < t.NumField(); i++ {
		p := new(Properties)
		f := t.Field(i)
		tagField := f.Tag.Get("protobuf")
		p.Init(f.Type, f.Name, tagField, &f)

		tagOneof := f.Tag.Get("protobuf_oneof")
		if tagOneof != "" {
			hasOneof = true
			p.OrigName = tagOneof
		}

		// Rename unrelated struct fields with the "XXX_" prefix since so much
		// user code simply checks for this to exclude special fields.
		if tagField == "" && tagOneof == "" && !strings.HasPrefix(p.Name, "XXX_") {
			p.Name = "XXX_" + p.Name
			p.OrigName = "XXX_" + p.OrigName
		} else if p.Weak != "" {
			p.Name = p.OrigName // avoid possible "XXX_" prefix on weak field
		}

		prop.Prop = append(prop.Prop, p)
	}

	// Construct a mapping of oneof field names to properties.
	if hasOneof {
		var oneofWrappers []interface{}
		if fn, ok := reflect.PtrTo(t).MethodByName("XXX_OneofFuncs"); ok {
			oneofWrappers = fn.Func.Call([]reflect.Value{reflect.Zero(fn.Type.In(0))})[3].Interface().([]interface{})
		}
		if fn, ok := reflect.PtrTo(t).MethodByName("XXX_OneofWrappers"); ok {
			oneofWrappers = fn.Func.Call([]reflect.Value{reflect.Zero(fn.Type.In(0))})[0].Interface().([]interface{})
		}
		if m, ok := reflect.Zero(reflect.PtrTo(t)).Interface().(protoreflect.ProtoMessage); ok {
			if m, ok := m.ProtoReflect().(interface{ ProtoMessageInfo() *protoimpl.MessageInfo }); ok {
				oneofWrappers = m.ProtoMessageInfo().OneofWrappers
			}
		}

		prop.OneofTypes = make(map[string]*OneofProperties)
		for _, wrapper := range oneofWrappers {
			p := &OneofProperties{
				Type: reflect.ValueOf(wrapper).Type(), // *T
				Prop: new(Properties),
			}
			f := p.Type.Elem().Field(0)
			p.Prop.Name = f.Name
			p.Prop.Parse(f.Tag.Get("protobuf"))

			// Determine the struct field that contains this oneof.
			// Each wrapper is assignable to exactly one parent field.
			var foundOneof bool
			for i := 0; i < t.NumField() && !foundOneof; i++ {
				if p.Type.AssignableTo(t.Field(i).Type) {
					p.Field = i
					foundOneof = true
				}
			}
			if !foundOneof {
				panic(fmt.Sprintf("%v is not a generated message in the open-struct API", t))
			}
			prop.OneofTypes[p.Prop.OrigName] = p
		}
	}

	return prop
}

func (sp *StructProperties) Len() int           { return len(sp.Prop) }
func (sp *StructProperties) Less(i, j int) bool { return false }
func (sp *StructProperties) Swap(i, j int)      { return }
=======
	p.setFieldProps(typ, f, lockGetProp)
}

var (
	propertiesMu  sync.RWMutex
	propertiesMap = make(map[reflect.Type]*StructProperties)
)

// GetProperties returns the list of properties for the type represented by t.
// t must represent a generated struct type of a protocol message.
func GetProperties(t reflect.Type) *StructProperties {
	if t.Kind() != reflect.Struct {
		panic("proto: type must have kind struct")
	}

	// Most calls to GetProperties in a long-running program will be
	// retrieving details for types we have seen before.
	propertiesMu.RLock()
	sprop, ok := propertiesMap[t]
	propertiesMu.RUnlock()
	if ok {
		return sprop
	}

	propertiesMu.Lock()
	sprop = getPropertiesLocked(t)
	propertiesMu.Unlock()
	return sprop
}

type (
	oneofFuncsIface interface {
		XXX_OneofFuncs() (func(Message, *Buffer) error, func(Message, int, int, *Buffer) (bool, error), func(Message) int, []interface{})
	}
	oneofWrappersIface interface {
		XXX_OneofWrappers() []interface{}
	}
)

// getPropertiesLocked requires that propertiesMu is held.
func getPropertiesLocked(t reflect.Type) *StructProperties {
	if prop, ok := propertiesMap[t]; ok {
		return prop
	}

	prop := new(StructProperties)
	// in case of recursive protos, fill this in now.
	propertiesMap[t] = prop

	// build properties
	prop.Prop = make([]*Properties, t.NumField())
	prop.order = make([]int, t.NumField())

	for i := 0; i < t.NumField(); i++ {
		f := t.Field(i)
		p := new(Properties)
		name := f.Name
		p.init(f.Type, name, f.Tag.Get("protobuf"), &f, false)

		oneof := f.Tag.Get("protobuf_oneof") // special case
		if oneof != "" {
			// Oneof fields don't use the traditional protobuf tag.
			p.OrigName = oneof
		}
		prop.Prop[i] = p
		prop.order[i] = i
		if debug {
			print(i, " ", f.Name, " ", t.String(), " ")
			if p.Tag > 0 {
				print(p.String())
			}
			print("\n")
		}
	}

	// Re-order prop.order.
	sort.Sort(prop)

	var oots []interface{}
	switch m := reflect.Zero(reflect.PtrTo(t)).Interface().(type) {
	case oneofFuncsIface:
		_, _, _, oots = m.XXX_OneofFuncs()
	case oneofWrappersIface:
		oots = m.XXX_OneofWrappers()
	}
	if len(oots) > 0 {
		// Interpret oneof metadata.
		prop.OneofTypes = make(map[string]*OneofProperties)
		for _, oot := range oots {
			oop := &OneofProperties{
				Type: reflect.ValueOf(oot).Type(), // *T
				Prop: new(Properties),
			}
			sft := oop.Type.Elem().Field(0)
			oop.Prop.Name = sft.Name
			oop.Prop.Parse(sft.Tag.Get("protobuf"))
			// There will be exactly one interface field that
			// this new value is assignable to.
			for i := 0; i < t.NumField(); i++ {
				f := t.Field(i)
				if f.Type.Kind() != reflect.Interface {
					continue
				}
				if !oop.Type.AssignableTo(f.Type) {
					continue
				}
				oop.Field = i
				break
			}
			prop.OneofTypes[oop.Prop.OrigName] = oop
		}
	}

	// build required counts
	// build tags
	reqCount := 0
	prop.decoderOrigNames = make(map[string]int)
	for i, p := range prop.Prop {
		if strings.HasPrefix(p.Name, "XXX_") {
			// Internal fields should not appear in tags/origNames maps.
			// They are handled specially when encoding and decoding.
			continue
		}
		if p.Required {
			reqCount++
		}
		prop.decoderTags.put(p.Tag, i)
		prop.decoderOrigNames[p.OrigName] = i
	}
	prop.reqCount = reqCount

	return prop
}

// A global registry of enum types.
// The generated code will register the generated maps by calling RegisterEnum.

var enumValueMaps = make(map[string]map[string]int32)

// RegisterEnum is called from the generated code to install the enum descriptor
// maps into the global table to aid parsing text format protocol buffers.
func RegisterEnum(typeName string, unusedNameMap map[int32]string, valueMap map[string]int32) {
	if _, ok := enumValueMaps[typeName]; ok {
		panic("proto: duplicate enum registered: " + typeName)
	}
	enumValueMaps[typeName] = valueMap
}

// EnumValueMap returns the mapping from names to integers of the
// enum type enumType, or a nil if not found.
func EnumValueMap(enumType string) map[string]int32 {
	return enumValueMaps[enumType]
}

// A registry of all linked message types.
// The string is a fully-qualified proto name ("pkg.Message").
var (
	protoTypedNils = make(map[string]Message)      // a map from proto names to typed nil pointers
	protoMapTypes  = make(map[string]reflect.Type) // a map from proto names to map types
	revProtoTypes  = make(map[reflect.Type]string)
)

// RegisterType is called from generated code and maps from the fully qualified
// proto name to the type (pointer to struct) of the protocol buffer.
func RegisterType(x Message, name string) {
	if _, ok := protoTypedNils[name]; ok {
		// TODO: Some day, make this a panic.
		log.Printf("proto: duplicate proto type registered: %s", name)
		return
	}
	t := reflect.TypeOf(x)
	if v := reflect.ValueOf(x); v.Kind() == reflect.Ptr && v.Pointer() == 0 {
		// Generated code always calls RegisterType with nil x.
		// This check is just for extra safety.
		protoTypedNils[name] = x
	} else {
		protoTypedNils[name] = reflect.Zero(t).Interface().(Message)
	}
	revProtoTypes[t] = name
}

// RegisterMapType is called from generated code and maps from the fully qualified
// proto name to the native map type of the proto map definition.
func RegisterMapType(x interface{}, name string) {
	if reflect.TypeOf(x).Kind() != reflect.Map {
		panic(fmt.Sprintf("RegisterMapType(%T, %q); want map", x, name))
	}
	if _, ok := protoMapTypes[name]; ok {
		log.Printf("proto: duplicate proto type registered: %s", name)
		return
	}
	t := reflect.TypeOf(x)
	protoMapTypes[name] = t
	revProtoTypes[t] = name
}

// MessageName returns the fully-qualified proto name for the given message type.
func MessageName(x Message) string {
	type xname interface {
		XXX_MessageName() string
	}
	if m, ok := x.(xname); ok {
		return m.XXX_MessageName()
	}
	return revProtoTypes[reflect.TypeOf(x)]
}

// MessageType returns the message type (pointer to struct) for a named message.
// The type is not guaranteed to implement proto.Message if the name refers to a
// map entry.
func MessageType(name string) reflect.Type {
	if t, ok := protoTypedNils[name]; ok {
		return reflect.TypeOf(t)
	}
	return protoMapTypes[name]
}

// A registry of all linked proto files.
var (
	protoFiles = make(map[string][]byte) // file name => fileDescriptor
)

// RegisterFile is called from generated code and maps from the
// full file name of a .proto file to its compressed FileDescriptorProto.
func RegisterFile(filename string, fileDescriptor []byte) {
	protoFiles[filename] = fileDescriptor
}

// FileDescriptor returns the compressed FileDescriptorProto for a .proto file.
func FileDescriptor(filename string) []byte { return protoFiles[filename] }
>>>>>>> cbc9bb05... fixup add vendor back

package reference

import (
	kapi "k8s.io/api/core/v1"
	"reflect"
	"strings"
)

func RefSet(ref *kapi.ObjectReference) bool {
	return ref != nil &&
		ref.Namespace != "" &&
		ref.Name != ""
}

func RefEquals(refA, refB *kapi.ObjectReference) bool {
	if refA == nil || refB == nil {
		return false
	}
	return reflect.DeepEqual(refA, refB)
}

func ToKind(resource interface{}) string {
	t := reflect.TypeOf(resource).String()
	p := strings.SplitN(t, ".", 2)
	return string(p[len(p)-1])
}

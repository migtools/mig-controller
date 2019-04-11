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

func ToKind(resource interface{}) string {
	t := reflect.TypeOf(resource).String()
	p := strings.SplitN(t, ".", 2)
	return string(p[len(p)-1])
}

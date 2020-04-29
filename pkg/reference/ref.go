package reference

import (
	"reflect"
	"regexp"
	"strings"

	kapi "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func ObjectKey(obj metav1.Object) client.ObjectKey {
	return client.ObjectKey{
		Name:      obj.GetName(),
		Namespace: obj.GetNamespace(),
	}
}

func TruncateName(name string) string {
	r := regexp.MustCompile(`(-+)`)
	name = r.ReplaceAllString(name, "-")
	name = strings.TrimRight(name, "-")
	if len(name) > 57 {
		name = name[:57]
	}
	return name
}

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

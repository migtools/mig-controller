package v1alpha1

import (
	migref "github.com/fusor/mig-controller/pkg/reference"
	"k8s.io/apimachinery/pkg/types"
	"strings"
)

// Labels
const (
	PartOfLabel = "app.kubernetes.io/part-of" // = Application
	Application = "openshift-migration"
)

// Build label (key, value) used to correlate CRs.
// Format: <kind>: <uid>.  The <uid> should be the ObjectMeta.UID
func CorrelationLabel(r interface{}, uid types.UID) (key, value string) {
	return labelKey(r), string(uid)
}

// Get a label (key) for the specified CR kind.
func labelKey(r interface{}) string {
	return strings.ToLower(migref.ToKind(r))
}

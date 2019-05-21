package v1alpha1

import (
	migref "github.com/fusor/mig-controller/pkg/reference"
	"k8s.io/apimachinery/pkg/types"
	"strings"
)

// All known correlation labels.
var KnownLabels = map[string]bool{
	label(MigPlan{}):      true,
	label(MigCluster{}):   true,
	label(MigStorage{}):   true,
	label(MigMigration{}): true,
	label(MigStage{}):     true,
}

// Build  labels used to correlate CRs.
// Format: <kind>: <uid>.  The <uid> should be the ObjectMeta.UID
func buildCorrelationLabels(r interface{}, uid types.UID) map[string]string {
	return map[string]string{
		label(r): string(uid),
	}
}

// Get a label (key) for the specified CR kind.
func label(r interface{}) string {
	return strings.ToLower(migref.ToKind(r))
}

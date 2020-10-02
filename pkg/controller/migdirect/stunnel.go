package migdirect

import (
//"context"
//  liberr "github.com/konveyor/controller/pkg/error"
//migapi "github.com/konveyor/mig-controller/pkg/apis/migration/v1alpha1"
//corev1 "k8s.io/api/core/v1"
//"k8s.io/apimachinery/pkg/types"
//k8sclient "sigs.k8s.io/controller-runtime/pkg/client"
)

func (t *Task) createStunnelCerts() error {
	// Get client for destination
	/*destClient, err := t.getDestinationClient()
	if err != nil {
		return err
	}

	// Get client for source
	srcClient, err := t.getSourceClient()
	if err != nil {
		return err
	}*/

	// Generate stunnel certs
	// openssl library? to generate new certs

	// Create same stunnel configmap with certs on both source+destination
	// https://github.com/konveyor/pvc-migrate/blob/master/3_run_rsync/templates/stunnel.yml.j2

	// Stunnel configmap consumption can follow 2 approaches:
	// On destination stunnel is sidecar container for rsync, stunnel pod is
	// exposed via route On source, can do sidecar approach; can also do
	// deployment of stunnel and use it for all migrations

	// For source stunnel pod, must mount certs into /etc/stunnel (see
	// https://github.com/konveyor/pvc-migrate/blob/master/3_run_rsync/tasks/rsync.yml#L54)
	// and write stunnel conf to /etc/stunnel/stunnel.conf

	// For source configmap: see https://github.com/konveyor/pvc-migrate/blob/master/3_run_rsync/tasks/rsync.yml#L47
	// For destination configmap: see https://github.com/konveyor/pvc-migrate/blob/master/3_run_rsync/templates/stunnel.yml.j2#L10

	// Create 1 rsync client pod per PVC
	// Create 1 stunnel pod per namespace

	// SOURCE
	// Create 1 rsync client pod per PVC and 1 stunnel pod per namespace
	// Create 1 stunnel svc - rsync client talks to stunnel svc

	// DESTINATION
	// Create 1 rsync transfer+stunnel pod per namespace
	// Create 1 stunnel svc
	return nil
}

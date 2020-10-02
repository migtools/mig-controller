package migdirect

import (
	"bytes"
	"context"
	"gopkg.in/yaml.v2"
	"text/template"
	//  liberr "github.com/konveyor/controller/pkg/error"
	corev1 "k8s.io/api/core/v1"
	k8serror "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	//"k8s.io/apimachinery/pkg/types"
)

type pvc struct {
	Name string
}

type rsyncConfig struct {
	SshUser    string
	ConfigName string
	Namespace  string
	PVCList    []pvc
}

// TODO: Parameterize this more to support custom
// user/pass/networking configs from migdirect spec
const rsyncConfigTemplate = `apiVersion: v1
kind: ConfigMap
metadata:
  name: {{ .ConfigName }}
  namespace: {{ .Namespace }}
  labels:
    purpose: rsync
data:
  rsyncd.conf: |
    syslog facility = local7
    read only = no
    list = yes
    max = 3
    auth users = {{ .SshUser }}
    secrets file = /etc/rsyncd.secrets
    hosts allow = ::1, 127.0.0.1, localhost
    uid = root
    gid = root
    {{ range $i, $pvc := .PVCList }}
    [{{ $pvc.Name }}]
        comment = archive for {{ $pvc.Name }}
        path = /mnt/{{ $.Namespace }}/{{ $pvc.Name }}
        uid = root
        gid = root
        list = yes
        hosts allow = ::1, 127.0.0.1, localhost
        auth users = {{ $.SshUser }}
        secrets file = /etc/rsyncd.secrets
        read only = false
   {{ end }}
`

func (t *Task) ensureRsyncPods() error {
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
	return nil

	// Create rsync transfer pod on destination

	// Create rsync client pod on source
}

func (t *Task) createRsyncConfig() error {
	// Get client for destination
	destClient, err := t.getDestinationClient()
	if err != nil {
		return err
	}

	// Get client for source
	srcClient, err := t.getSourceClient()
	if err != nil {
		return err
	}

	// Create rsync configmap/secret on source + destination
	// Create rsync secret (which contains user/pass for rsync transfer pod) in
	// each namespace being migrated
	// Needs to go in every namespace where a PVC is being migrated
	pvcMap := t.getPVCNamespaceMap()

	for ns, vols := range pvcMap {
		t.Log.Info("namespace", ns)
		pvcList := []pvc{}
		for _, vol := range vols {
			pvcList = append(pvcList, pvc{Name: vol})
		}
		// Generate template
		rsyncConf := rsyncConfig{
			SshUser:    "root",
			ConfigName: "migdirect-rsync-config",
			Namespace:  ns,
			PVCList:    pvcList,
		}
		var tpl bytes.Buffer
		temp, err := template.New("config").Parse(rsyncConfigTemplate)
		if err != nil {
			return err
		}
		err = temp.Execute(&tpl, rsyncConf)
		if err != nil {
			return err
		}

		configMap := corev1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: ns,
				Name:      "migdirect-rsync-config",
			},
		}
		err = yaml.Unmarshal(tpl.Bytes(), &configMap)
		if err != nil {
			return err
		}

		// Create configmap on source + dest
		// Note: when this configmap changes the rsync pod
		// needs to restart
		// Need to launch new pod when configmap changes
		err = destClient.Create(context.TODO(), &configMap)
		if k8serror.IsAlreadyExists(err) {
			t.Log.Info("Configmap already exists on destination", "namespace", configMap.Namespace)
		} else if err != nil {
			return err
		}
	}

	// Before starting rsync transfer pod, must generate rsync password in a
	// secret and pass it into the transfer pod

	// Format user:password
	// Put this string into /etc/rsyncd.secrets in rsync transfer pod
	// Rsyncd configmap references this file as "secrets file":
	// https://github.com/konveyor/pvc-migrate/blob/master/3_run_rsync/templates/rsyncd.yml.j2#L17
	// This configmap also takes in the user name as an "auth user". (root)
	// Make this user configurable on CR spec?

	// For source side, create secret with user/password and
	// mount as environment variables into rsync client pod

	// One rsync transfer pod per namespace
	// One rsync client pod per PVC

	// Also in this rsyncd configmap, include all PVC mount paths, see:
	// https://github.com/konveyor/pvc-migrate/blob/master/3_run_rsync/templates/rsyncd.yml.j2#L23

	return nil
}

// Transfer pod which runs rsyncd
func (t *Task) CreateRsyncTransferPod() {
	// one transfer pod should be created per namespace and should mount all
	// PVCs that are being written to in that namespace

	// Transfer pod contains 2 containers, this is the stunnel container +
	// rsyncd

	// Transfer pod should also mount the stunnel configmap, the rsync secret
	// (contains creds), and add appropiate health checks for both stunnel +
	// rsyncd containers.
}

func (t *Task) getPVCNamespaceMap() map[string][]string {
	nsMap := map[string][]string{}
	for _, pvc := range t.Owner.Spec.PersistentVolumeClaims {
		if vols, exists := nsMap[pvc.Namespace]; exists {
			vols = append(vols, pvc.Name)
			nsMap[pvc.Namespace] = vols
		} else {
			nsMap[pvc.Namespace] = []string{pvc.Name}
		}
	}
	return nsMap
}

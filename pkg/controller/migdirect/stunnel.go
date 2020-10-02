package migdirect

import (
	"bytes"
	"context"
	"gopkg.in/yaml.v2"
	"text/template"

	corev1 "k8s.io/api/core/v1"
	//"k8s.io/apimachinery/pkg/types"
	k8serror "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type stunnelConfig struct {
	Name        string
	Namespace   string
	StunnelPort int32
	RsyncRoute  string
	RsyncPort   int32
}

// TODO: Parameterize this more to support custom
// networking configs from migdirect spec
const stunnelClientConfigTemplate = `apiVersion: v1
kind: ConfigMap
metadata:
  labels:
    purpose: stunnel
data:
  stunnel.conf: |
    foreground = no
    pid =
    sslVersion = TLSv1.2
    client = yes
    syslog = no
    [rsync]
    accept = {{ .StunnelPort}}
    CAFile = /etc/stunnel/tls.crt
    cert = /etc/stunnel/tls.crt
    connect = {{ .RsyncRoute }}:443
    verify = 2
    key = /etc/stunnel/tls.key
`

const stunnelDestinationConfigTemplate = `apiVersion: v1
kind: ConfigMap
metadata:
  labels:
    purpose: stunnel-config
data:
  stunnel.conf: |
    foreground = yes
    pid =
    socket = l:TCP_NODELAY=1
    socket = r:TCP_NODELAY=1
    sslVersion = TLSv1.2

    [rsync]
    accept = {{ .StunnelPort }}
    connect = {{ .RsyncPort }}
    key = /etc/stunnel/tls.key
    cert = /etc/stunnel/tls.crt
    TIMEOUTclose = 0
`

func (t *Task) createStunnelConfig() error {
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
	pvcMap := t.getPVCNamespaceMap()

	for ns, _ := range pvcMap {
		// Declare config
		stunnelConf := stunnelConfig{
			Namespace:   ns,
			StunnelPort: 2222,
			RsyncPort:   22,
			RsyncRoute:  t.getRsyncRoute(),
		}

		// Generate templates
		var clientTpl bytes.Buffer
		var destTpl bytes.Buffer
		clientTemp, err := template.New("config").Parse(stunnelClientConfigTemplate)
		if err != nil {
			return err
		}
		destTemp, err := template.New("config").Parse(stunnelDestinationConfigTemplate)
		if err != nil {
			return err
		}

		// Execute templates
		err = clientTemp.Execute(&clientTpl, stunnelConf)
		if err != nil {
			return err
		}
		err = destTemp.Execute(&destTpl, stunnelConf)
		if err != nil {
			return err
		}

		// Generate configmaps
		clientConfigMap := corev1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: ns,
				Name:      "migdirect-stunnel-config",
			},
		}
		err = yaml.Unmarshal(clientTpl.Bytes(), &clientConfigMap)
		if err != nil {
			return err
		}

		destConfigMap := corev1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: ns,
				Name:      "migdirect-stunnel-config",
			},
		}
		err = yaml.Unmarshal(destTpl.Bytes(), &destConfigMap)
		if err != nil {
			return err
		}

		// Create configmaps on source + dest
		err = srcClient.Create(context.TODO(), &clientConfigMap)
		if k8serror.IsAlreadyExists(err) {
			t.Log.Info("Configmap already exists on destination", "namespace", clientConfigMap.Namespace)
		} else if err != nil {
			return err
		}

		err = destClient.Create(context.TODO(), &destConfigMap)
		if k8serror.IsAlreadyExists(err) {
			t.Log.Info("Configmap already exists on destination", "namespace", destConfigMap.Namespace)
		} else if err != nil {
			return err
		}
	}
	return nil
}

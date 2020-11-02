package directvolumemigration

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/rsa"
	"fmt"
	"github.com/konveyor/mig-controller/pkg/apis/migration/v1alpha1"
	"golang.org/x/crypto/ssh"
	"gopkg.in/yaml.v2"
	"text/template"
	//  liberr "github.com/konveyor/controller/pkg/error"
	"github.com/konveyor/mig-controller/pkg/compat"
	routev1 "github.com/openshift/api/route/v1"
	corev1 "k8s.io/api/core/v1"
	k8serror "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/intstr"
	k8sclient "sigs.k8s.io/controller-runtime/pkg/client"
	//"k8s.io/apimachinery/pkg/types"
)

type pvc struct {
	Name string
}

type rsyncConfig struct {
	SshUser   string
	Namespace string
	Password  string
	PVCList   []pvc
}

// TODO: Parameterize this more to support custom
// user/pass/networking configs from directvolumemigration spec
const rsyncConfigTemplate = `apiVersion: v1
kind: ConfigMap
metadata:
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

func (t *Task) areRsyncTransferPodsRunning() (bool, error) {
	// Get client for destination
	destClient, err := t.getDestinationClient()
	if err != nil {
		return false, err
	}

	pvcMap := t.getPVCNamespaceMap()

	selector := labels.SelectorFromSet(map[string]string{
		"app":     "directvolumemigration-rsync-transfer",
		"owner":   "directvolumemigration",
		"purpose": "rsync",
	})
	for ns, _ := range pvcMap {
		pods := corev1.PodList{}
		err = destClient.List(
			context.TODO(),
			&k8sclient.ListOptions{
				Namespace:     ns,
				LabelSelector: selector,
			},
			&pods)
		if err != nil {
			return false, err
		}
		for _, pod := range pods.Items {
			if pod.Status.Phase != corev1.PodRunning {
				return false, nil
			}
		}
	}

	return true, nil

	// Create rsync transfer pod on destination

	// Create rsync client pod on source
}

// Generate SSH keys to be used
// TODO: Need to determine if this has already been generated and
// not to regenerate
func (t *Task) generateSSHKeys() error {
	// Check if already generated
	if t.SSHKeys != nil {
		return nil
	}
	// Private Key generation
	privateKey, err := rsa.GenerateKey(rand.Reader, 4096)
	if err != nil {
		return err
	}

	// Validate Private Key
	err = privateKey.Validate()
	if err != nil {
		return err
	}

	t.SSHKeys = &sshKeys{
		PublicKey:  &privateKey.PublicKey,
		PrivateKey: privateKey,
	}
	return nil
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
		pvcList := []pvc{}
		for _, vol := range vols {
			pvcList = append(pvcList, pvc{Name: vol})
		}
		// Generate template
		rsyncConf := rsyncConfig{
			SshUser:   "root",
			Namespace: ns,
			PVCList:   pvcList,
			Password:  "changeme",
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
				Name:      "directvolumemigration-rsync-config",
				Labels: map[string]string{
					"app": "directvolumemigration-rsync-transfer",
				},
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
		srcSecret := corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: ns,
				Name:      "directvolumemigration-rsync-creds",
				Labels: map[string]string{
					"app": "directvolumemigration-rsync-transfer",
				},
			},
			Data: map[string][]byte{
				"RSYNC_PASSWORD": []byte("changeme"),
			},
		}
		err = srcClient.Create(context.TODO(), &srcSecret)
		if k8serror.IsAlreadyExists(err) {
			t.Log.Info("Secret already exists on source", "namespace", srcSecret.Namespace)
		} else if err != nil {
			return err
		}
		destSecret := corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: ns,
				Name:      "directvolumemigration-rsync-creds",
				Labels: map[string]string{
					"app": "directvolumemigration-rsync-transfer",
				},
			},
			Data: map[string][]byte{
				"credentials": []byte("root:changeme"),
			},
		}
		err = destClient.Create(context.TODO(), &destSecret)
		if k8serror.IsAlreadyExists(err) {
			t.Log.Info("Secret already exists on destination", "namespace", destSecret.Namespace)
		} else if err != nil {
			return err
		}
	}

	// One rsync transfer pod per namespace
	// One rsync client pod per PVC

	// Also in this rsyncd configmap, include all PVC mount paths, see:
	// https://github.com/konveyor/pvc-migrate/blob/master/3_run_rsync/templates/rsyncd.yml.j2#L23

	return nil
}

// Create rsync transfer route
func (t *Task) createRsyncTransferRoute() error {
	// Get client for destination
	destClient, err := t.getDestinationClient()
	if err != nil {
		return err
	}
	pvcMap := t.getPVCNamespaceMap()
	for ns, _ := range pvcMap {
		svc := corev1.Service{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "directvolumemigration-rsync-transfer-svc",
				Namespace: ns,
				Labels: map[string]string{
					"app": "directvolumemigration-rsync-transfer",
				},
			},
			Spec: corev1.ServiceSpec{
				Ports: []corev1.ServicePort{
					{
						Name:       "stunnel",
						Protocol:   corev1.ProtocolTCP,
						Port:       int32(2222),
						TargetPort: intstr.IntOrString{Type: intstr.Int, IntVal: 2222},
					},
				},
				Selector: map[string]string{
					"app":     "directvolumemigration-rsync-transfer",
					"purpose": "rsync",
					"owner":   "directvolumemigration",
				},
				Type: corev1.ServiceTypeClusterIP,
			},
		}
		err = destClient.Create(context.TODO(), &svc)
		if k8serror.IsAlreadyExists(err) {
			t.Log.Info("Rsync transfer svc already exists on destination", "namespace", ns)
		} else if err != nil {
			return err
		}
		route := routev1.Route{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "directvolumemigration-rsync-transfer-route",
				Namespace: ns,
				Labels: map[string]string{
					"app": "directvolumemigration-rsync-transfer",
				},
			},
			Spec: routev1.RouteSpec{
				To: routev1.RouteTargetReference{
					Kind: "Service",
					Name: "directvolumemigration-rsync-transfer-svc",
				},
				Port: &routev1.RoutePort{
					TargetPort: intstr.IntOrString{Type: intstr.Int, IntVal: 2222},
				},
				TLS: &routev1.TLSConfig{
					Termination: routev1.TLSTerminationPassthrough,
				},
			},
		}
		err = destClient.Create(context.TODO(), &route)
		if k8serror.IsAlreadyExists(err) {
			t.Log.Info("Rsync transfer route already exists on destination", "namespace", ns)
		} else if err != nil {
			return err
		}
		t.RsyncRoutes[ns] = route.Spec.Host
	}
	return nil
}

// Transfer pod which runs rsyncd
func (t *Task) createRsyncTransferPods() error {
	// Ensure SSH Keys exist
	err := t.generateSSHKeys()
	if err != nil {
		return err
	}

	// Get client for destination
	destClient, err := t.getDestinationClient()
	if err != nil {
		return err
	}
	// one transfer pod should be created per namespace and should mount all
	// PVCs that are being written to in that namespace

	// Transfer pod contains 2 containers, this is the stunnel container +
	// rsyncd

	// Transfer pod should also mount the stunnel configmap, the rsync secret
	// (contains creds), and add appropiate health checks for both stunnel +
	// rsyncd containers.

	// Generate pubkey bytes
	// TODO: Use a secret for this so we aren't regenerating every time
	publicRsaKey, err := ssh.NewPublicKey(t.SSHKeys.PublicKey)
	if err != nil {
		return err
	}

	pubKeyBytes := ssh.MarshalAuthorizedKey(publicRsaKey)
	mode := int32(0600)

	// Loop through namespaces and create transfer pod
	pvcMap := t.getPVCNamespaceMap()
	for ns, vols := range pvcMap {
		volumeMounts := []corev1.VolumeMount{}
		volumes := []corev1.Volume{
			{
				Name: "stunnel-conf",
				VolumeSource: corev1.VolumeSource{
					ConfigMap: &corev1.ConfigMapVolumeSource{
						LocalObjectReference: corev1.LocalObjectReference{
							Name: "directvolumemigration-stunnel-config",
						},
					},
				},
			},
			{
				Name: "stunnel-certs",
				VolumeSource: corev1.VolumeSource{
					Secret: &corev1.SecretVolumeSource{
						SecretName: "directvolumemigration-stunnel-certs",
						Items: []corev1.KeyToPath{
							{
								Key:  "tls.crt",
								Path: "tls.crt",
							},
							{
								Key:  "ca.crt",
								Path: "ca.crt",
							},
							{
								Key:  "tls.key",
								Path: "tls.key",
							},
						},
					},
				},
			},
			{
				Name: "rsync-creds",
				VolumeSource: corev1.VolumeSource{
					Secret: &corev1.SecretVolumeSource{
						SecretName:  "directvolumemigration-rsync-creds",
						DefaultMode: &mode,
						Items: []corev1.KeyToPath{
							{
								Key:  "credentials",
								Path: "rsyncd.secrets",
							},
						},
					},
				},
			},
			{
				Name: "rsyncd-conf",
				VolumeSource: corev1.VolumeSource{
					ConfigMap: &corev1.ConfigMapVolumeSource{
						LocalObjectReference: corev1.LocalObjectReference{
							Name: "directvolumemigration-rsync-config",
						},
					},
				},
			},
		}
		trueBool := true
		runAsUser := int64(0)

		// Add PVC volume mounts
		for _, vol := range vols {
			volumeMounts = append(volumeMounts, corev1.VolumeMount{
				Name:      vol,
				MountPath: fmt.Sprintf("/mnt/%s/%s", ns, vol),
			})
			volumes = append(volumes, corev1.Volume{
				Name: vol,
				VolumeSource: corev1.VolumeSource{
					PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{
						ClaimName: vol,
					},
				},
			})
		}
		// Add rsyncd config mount
		volumeMounts = append(volumeMounts, corev1.VolumeMount{
			Name:      "rsyncd-conf",
			MountPath: "/etc/rsyncd.conf",
			SubPath:   "rsyncd.conf",
		})
		// Add rsync creds to volumeMounts
		volumeMounts = append(volumeMounts, corev1.VolumeMount{
			Name:      "rsync-creds",
			MountPath: "/etc/rsyncd.secrets",
			SubPath:   "rsyncd.secrets",
		})

		labels := map[string]string{
			"app":     "directvolumemigration-rsync-transfer",
			"owner":   "directvolumemigration",
			"purpose": "rsync",
		}

		transferPod := corev1.Pod{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "directvolumemigration-rsync-transfer",
				Namespace: ns,
				Labels:    labels,
			},
			Spec: corev1.PodSpec{
				Volumes: volumes,
				Containers: []corev1.Container{
					{
						Name:  "rsyncd",
						Image: "quay.io/konveyor/rsync-transfer:latest",
						Env: []corev1.EnvVar{
							{
								Name:  "SSH_PUBLIC_KEY",
								Value: string(pubKeyBytes),
							},
						},
						Command: []string{"/usr/bin/rsync", "--daemon", "--no-detach", "--port=22", "-vvv"},
						Ports: []corev1.ContainerPort{
							{
								Name:          "rsyncd",
								Protocol:      corev1.ProtocolTCP,
								ContainerPort: int32(22),
							},
						},
						VolumeMounts: volumeMounts,
						SecurityContext: &corev1.SecurityContext{
							Privileged:             &trueBool,
							RunAsUser:              &runAsUser,
							ReadOnlyRootFilesystem: &trueBool,
						},
					},
					{
						Name:    "stunnel",
						Image:   "quay.io/konveyor/rsync-transfer:latest",
						Command: []string{"/bin/stunnel", "/etc/stunnel/stunnel.conf"},
						Ports: []corev1.ContainerPort{
							{
								Name:          "stunnel",
								Protocol:      corev1.ProtocolTCP,
								ContainerPort: int32(2222),
							},
						},
						VolumeMounts: []corev1.VolumeMount{
							{
								Name:      "stunnel-conf",
								MountPath: "/etc/stunnel/stunnel.conf",
								SubPath:   "stunnel.conf",
							},
							{
								Name:      "stunnel-certs",
								MountPath: "/etc/stunnel/certs",
							},
						},
						SecurityContext: &corev1.SecurityContext{
							Privileged:             &trueBool,
							RunAsUser:              &runAsUser,
							ReadOnlyRootFilesystem: &trueBool,
						},
					},
				},
			},
		}
		err = destClient.Create(context.TODO(), &transferPod)
		if k8serror.IsAlreadyExists(err) {
			t.Log.Info("Rsync transfer pod already exists on destination", "namespace", transferPod.Namespace)
		} else if err != nil {
			return err
		}
		t.Log.Info("Rsync transfer pod created", "name", transferPod.Name, "namespace", transferPod.Namespace)

	}
	return nil
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

func (t *Task) getRsyncRoute(namespace string) (string, error) {
	// Get client for destination
	destClient, err := t.getDestinationClient()
	if err != nil {
		return "", err
	}
	route := routev1.Route{}

	key := types.NamespacedName{Name: "directvolumemigration-rsync-transfer-route", Namespace: namespace}
	err = destClient.Get(context.TODO(), key, &route)
	if err != nil {
		return "", err
	}
	return route.Spec.Host, nil
}

// Create rsync client pods
func (t *Task) createRsyncClientPods() error {
	// Get client for destination
	srcClient, err := t.getSourceClient()
	if err != nil {
		return err
	}

	pvcMap := t.getPVCNamespaceMap()
	for ns, vols := range pvcMap {
		// Get stunnel svc IP
		svc := corev1.Service{}
		key := types.NamespacedName{Name: "directvolumemigration-rsync-transfer-svc", Namespace: ns}
		srcClient.Get(context.TODO(), key, &svc)
		ip := svc.Spec.ClusterIP

		trueBool := true
		runAsUser := int64(0)

		// Add PVC volume mounts
		for _, vol := range vols {
			volumes := []corev1.Volume{}
			volumeMounts := []corev1.VolumeMount{}
			containers := []corev1.Container{}
			volumeMounts = append(volumeMounts, corev1.VolumeMount{
				Name:      vol,
				MountPath: fmt.Sprintf("/mnt/%s/%s", ns, vol),
			})
			volumes = append(volumes, corev1.Volume{
				Name: vol,
				VolumeSource: corev1.VolumeSource{
					PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{
						ClaimName: vol,
					},
				},
			})
			containers = append(containers, corev1.Container{
				Name:  "rsync-client",
				Image: "quay.io/konveyor/rsync-transfer:latest",
				Env: []corev1.EnvVar{
					{
						Name:  "RSYNC_PASSWORD",
						Value: "changeme",
					},
				},
				Command: []string{"rsync", "-aPvvHh", "--delete", "--port", "2222", "--log-file", "/dev/stdout", fmt.Sprintf("/mnt/%s/%s/", ns, vol), fmt.Sprintf("rsync://root@%s/%s", ip, vol)},
				Ports: []corev1.ContainerPort{
					{
						Name:          "rsync-client",
						Protocol:      corev1.ProtocolTCP,
						ContainerPort: int32(22),
					},
				},
				VolumeMounts: volumeMounts,
				SecurityContext: &corev1.SecurityContext{
					Privileged:             &trueBool,
					RunAsUser:              &runAsUser,
					ReadOnlyRootFilesystem: &trueBool,
				},
			})
			clientPod := corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Name:      fmt.Sprintf("directvolumemigration-rsync-transfer-%s", vol),
					Namespace: ns,
					Labels: map[string]string{
						"app":                   "directvolumemigration-rsync-transfer",
						"directvolumemigration": "rsync-client",
					},
				},
				Spec: corev1.PodSpec{
					RestartPolicy: corev1.RestartPolicyNever,
					Volumes:       volumes,
					Containers:    containers,
				},
			}
			err = srcClient.Create(context.TODO(), &clientPod)
			if k8serror.IsAlreadyExists(err) {
				t.Log.Info("Rsync client pod already exists on source", "namespace", clientPod.Namespace)
			} else if err != nil {
				return err
			}
			t.Log.Info("Rsync client pod created", "name", clientPod.Name, "namespace", clientPod.Namespace)
		}

	}
	return nil
}

// Create rsync PV progress CR on destination cluster
func (t *Task) createPVProgressCR() error {
	// Get client for destination
	dstClient, err := t.getDestinationClient()
	if err != nil {
		return err
	}

	pvcMap := t.getPVCNamespaceMap()
	for ns, vols := range pvcMap {
		for _, vol := range vols {
			dvp := v1alpha1.DirectPVMigrationProgress{
				ObjectMeta: metav1.ObjectMeta{
					Name:      fmt.Sprintf("directvolumemigration-rsync-transfer-%s", vol),
					Namespace: ns,
					// TODO @alpatel, add owner references
				},
				Spec: v1alpha1.DirectPVMigrationProgressSpec{
					ClusterRef: t.Owner.Spec.SrcMigClusterRef,
					PodRef: &corev1.ObjectReference{
						Namespace: ns,
						Name:      fmt.Sprintf("directvolumemigration-rsync-transfer-%s", vol),
					},
				},
				Status: v1alpha1.DirectPVMigrationProgressStatus{},
			}
			err = dstClient.Create(context.TODO(), &dvp)
			if k8serror.IsAlreadyExists(err) {
				t.Log.Info("Rsync client progress CR already exists on destination", "namespace", dvp.Namespace, "name", dvp.Name)
			} else if err != nil {
				return err
			}
			t.Log.Info("Rsync client progress CR created", "name", dvp.Name, "namespace", dvp.Namespace)
		}

	}
	return nil
}

func (t *Task) haveRsyncClientPodsCompletedOrFailed() (bool, bool, error) {
	// Get client for destination
	dstClient, err := t.getDestinationClient()
	if err != nil {
		return false, false, err
	}

	t.Owner.Status.RunningPods = []*corev1.ObjectReference{}
	t.Owner.Status.FailedPods = []*corev1.ObjectReference{}
	t.Owner.Status.SuccessfulPods = []*corev1.ObjectReference{}

	pvcMap := t.getPVCNamespaceMap()
	for ns, vols := range pvcMap {
		for _, vol := range vols {
			dvp := v1alpha1.DirectPVMigrationProgress{}
			err = dstClient.Get(context.TODO(), types.NamespacedName{
				Name:      fmt.Sprintf("directvolumemigration-rsync-transfer-%s", vol),
				Namespace: ns,
			}, &dvp)
			if err != nil {
				// todo, need to start thinking about collecting this error and reporting other CR's progress
				return false, false, err
			}
			objRef := &corev1.ObjectReference{
				Namespace: ns,
				Name:      fmt.Sprintf("directvolumemigration-rsync-transfer-%s", vol),
			}
			switch {
			case dvp.Status.PodPhase == corev1.PodRunning:
				t.Owner.Status.RunningPods = append(t.Owner.Status.RunningPods, objRef)
			case dvp.Status.PodPhase == corev1.PodFailed:
				t.Owner.Status.FailedPods = append(t.Owner.Status.FailedPods, objRef)
			case dvp.Status.PodPhase == corev1.PodSucceeded:
				t.Owner.Status.SuccessfulPods = append(t.Owner.Status.SuccessfulPods, objRef)
			}
		}
	}

	// wait for all the running pods to completed before returning failures
	if len(t.Owner.Status.RunningPods) > 0 {
		return false, false, nil
	}

	if len(t.Owner.Status.FailedPods) > 0 {
		return false, true, nil
	}

	return true, false, nil
}

// Delete rsync resources
func (t *Task) deleteRsyncResources() error {
	// Get client for source + destination
	srcClient, err := t.getSourceClient()
	if err != nil {
		return err
	}
	destClient, err := t.getDestinationClient()
	if err != nil {
		return err
	}

	err = t.findAndDeleteResources(srcClient)
	if err != nil {
		return err
	}

	err = t.findAndDeleteResources(destClient)
	if err != nil {
		return err
	}

	if !t.Owner.Spec.DeleteProgressReportingCRs {
		return nil
	}

	err = t.deleteProgressReportingDRs(destClient)
	if err != nil {
		return err
	}

	return nil
}

func (t *Task) findAndDeleteResources(client compat.Client) error {
	// Find all resources with the app label
	selector := labels.SelectorFromSet(map[string]string{
		"app": "directvolumemigration-rsync-transfer",
	})
	pvcMap := t.getPVCNamespaceMap()
	for ns, _ := range pvcMap {
		podList := corev1.PodList{}
		cmList := corev1.ConfigMapList{}
		svcList := corev1.ServiceList{}
		secretList := corev1.SecretList{}
		routeList := routev1.RouteList{}

		// Get Pod list
		err := client.List(
			context.TODO(),
			&k8sclient.ListOptions{
				Namespace:     ns,
				LabelSelector: selector,
			},
			&podList)
		if err != nil {
			return err
		}
		// Get Secret list
		err = client.List(
			context.TODO(),
			&k8sclient.ListOptions{
				Namespace:     ns,
				LabelSelector: selector,
			},
			&secretList)
		if err != nil {
			return err
		}

		// Get configmap list
		err = client.List(
			context.TODO(),
			&k8sclient.ListOptions{
				Namespace:     ns,
				LabelSelector: selector,
			},
			&cmList)
		if err != nil {
			return err
		}

		// Get svc list
		err = client.List(
			context.TODO(),
			&k8sclient.ListOptions{
				Namespace:     ns,
				LabelSelector: selector,
			},
			&svcList)
		if err != nil {
			return err
		}

		// Get route list
		err = client.List(
			context.TODO(),
			&k8sclient.ListOptions{
				Namespace:     ns,
				LabelSelector: selector,
			},
			&routeList)
		if err != nil {
			return err
		}

		// Delete pods
		for _, pod := range podList.Items {
			err = client.Delete(context.TODO(), &pod, k8sclient.PropagationPolicy(metav1.DeletePropagationForeground))
			if err != nil && !k8serror.IsNotFound(err) {
				return err
			}
		}

		// Delete secrets
		for _, secret := range secretList.Items {
			err = client.Delete(context.TODO(), &secret, k8sclient.PropagationPolicy(metav1.DeletePropagationForeground))
			if err != nil && !k8serror.IsNotFound(err) {
				return err
			}
		}

		// Delete routes
		for _, route := range routeList.Items {
			err = client.Delete(context.TODO(), &route, k8sclient.PropagationPolicy(metav1.DeletePropagationForeground))
			if err != nil && !k8serror.IsNotFound(err) {
				return err
			}
		}

		// Delete svcs
		for _, svc := range svcList.Items {
			err = client.Delete(context.TODO(), &svc, k8sclient.PropagationPolicy(metav1.DeletePropagationForeground))
			if err != nil && !k8serror.IsNotFound(err) {
				return err
			}
		}

		// Delete configmaps
		for _, cm := range cmList.Items {
			err = client.Delete(context.TODO(), &cm, k8sclient.PropagationPolicy(metav1.DeletePropagationForeground))
			if err != nil && !k8serror.IsNotFound(err) {
				return err
			}
		}
	}
	return nil
}

func (t *Task) deleteProgressReportingDRs(client compat.Client) error {
	pvcMap := t.getPVCNamespaceMap()

	for ns, vols := range pvcMap {
		for _, vol := range vols {
			err := client.Delete(context.TODO(), &v1alpha1.DirectPVMigrationProgress{
				ObjectMeta: metav1.ObjectMeta{
					Name:      fmt.Sprintf("directvolumemigration-rsync-transfer-%s", vol),
					Namespace: ns,
				},
			}, k8sclient.PropagationPolicy(metav1.DeletePropagationForeground))
			if err != nil && !k8serror.IsNotFound(err) {
				return err
			}
		}
	}
	return nil
}

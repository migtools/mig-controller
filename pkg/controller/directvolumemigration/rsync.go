package directvolumemigration

import (
	"bytes"
	"context"
	"crypto/md5"
	"crypto/rand"
	"crypto/rsa"
	"encoding/hex"
	"fmt"
	random "math/rand"
	"path"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"text/template"
	"time"
	"unicode/utf8"

	liberr "github.com/konveyor/controller/pkg/error"
	migapi "github.com/konveyor/mig-controller/pkg/apis/migration/v1alpha1"
	"github.com/konveyor/mig-controller/pkg/compat"
	migevent "github.com/konveyor/mig-controller/pkg/event"
	"github.com/konveyor/mig-controller/pkg/settings"
	routev1 "github.com/openshift/api/route/v1"
	"golang.org/x/crypto/ssh"
	"gopkg.in/yaml.v2"
	corev1 "k8s.io/api/core/v1"
	k8serror "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/intstr"
	k8sclient "sigs.k8s.io/controller-runtime/pkg/client"
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

const (
	TRANSFER_POD_CPU_LIMIT      = "TRANSFER_POD_CPU_LIMIT"
	TRANSFER_POD_MEMORY_LIMIT   = "TRANSFER_POD_MEMORY_LIMIT"
	TRANSFER_POD_CPU_REQUEST    = "TRANSFER_POD_CPU_REQUEST"
	TRANSFER_POD_MEMORY_REQUEST = "TRANSFER_POD_MEMORY_REQUEST"
	CLIENT_POD_CPU_LIMIT        = "CLIENT_POD_CPU_LIMIT"
	CLIENT_POD_MEMORY_LIMIT     = "CLIENT_POD_MEMORY_LIMIT"
	CLIENT_POD_CPU_REQUEST      = "CLIENT_POD_CPU_REQUEST"
	CLIENT_POD_MEMORY_REQUEST   = "CLIENT_POD_MEMORY_REQUEST"
	STUNNEL_POD_CPU_LIMIT       = "STUNNEL_POD_CPU_LIMIT"
	STUNNEL_POD_MEMORY_LIMIT    = "STUNNEL_POD_MEMORY_LIMIT"
	STUNNEL_POD_CPU_REQUEST     = "STUNNEL_POD_CPU_REQUEST"
	STUNNEL_POD_MEMORY_REQUEST  = "STUNNEL_POD_MEMORY_REQUEST"

	// DefaultStunnelTimout is when stunnel timesout on establishing connection from source to destination.
	//  When this timeout is reached, the rsync client will still see "connection reset by peer". It is a red-herring
	// it does not conclusively mean the destination rsyncd is unhealthy but stunnel is dropping this in between
	DefaultStunnelTimeout = 20
	// DefaultRsyncBackOffLimit defines default limit on number of retries on Rsync Pods
	DefaultRsyncBackOffLimit = 20
	// DefaultRsyncOperationConcurrency defines number of Rsync operations that can be processed concurrently
	DefaultRsyncOperationConcurrency = 5
	// PendingPodWarningTimeLimit time threshold for Rsync Pods in Pending state to show warning
	PendingPodWarningTimeLimit = 10 * time.Minute
)

// labels
const (
	// RsyncAttemptLabel is used to associate an Rsync Pod with the attempts
	RsyncAttemptLabel = "migration.openshift.io/rsync-attempt"
)

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
    log file = /dev/stdout
    max verbosity = 4
    auth users = {{ .SshUser }}
    secrets file = /etc/rsyncd.secrets
    hosts allow = ::1, 127.0.0.1, localhost
    uid = root
    gid = root
    {{ range $i, $pvc := .PVCList }}
    [{{ $pvc.Name }}]
        comment = archive for {{ $pvc.Name }}
        path = /mnt/{{ $.Namespace }}/{{ $pvc.Name }}
        use chroot = no
        munge symlinks = no
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
	dvmLabels := t.buildDVMLabels()
	dvmLabels["purpose"] = DirectVolumeMigrationRsync
	selector := labels.SelectorFromSet(dvmLabels)

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
		if len(pods.Items) != 1 {
			t.Log.Info("Unexpected number of DVM Rsync Pods found.",
				"podExpected", 1, "podsFound", len(pods.Items))
			return false, nil
		}
		for _, pod := range pods.Items {
			if pod.Status.Phase != corev1.PodRunning {
				// Log abnormal events for Rsync transfer Pod if any are found
				migevent.LogAbnormalEventsForResource(
					destClient, t.Log,
					"Found abnormal event for Rsync transfer Pod on destination cluster",
					types.NamespacedName{Namespace: pod.Namespace, Name: pod.Name}, "Pod")

				for _, podCond := range pod.Status.Conditions {
					if podCond.Reason == corev1.PodReasonUnschedulable {
						t.Log.Info("Found UNSCHEDULABLE Rsync Transfer Pod on destination cluster",
							"pod", path.Join(pod.Namespace, pod.Name),
							"podPhase", pod.Status.Phase,
							"podConditionMessage", podCond.Message)
						return false, nil
					}
				}
				t.Log.Info("Found non-running Rsync Transfer Pod on destination cluster.",
					"pod", path.Join(pod.Namespace, pod.Name),
					"podPhase", pod.Status.Phase)
				return false, nil
			}
		}
	}
	return true, nil
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

	password, err := t.getRsyncPassword()
	if err != nil {
		return err
	}
	if password == "" {
		password, err = t.createRsyncPassword()
		if err != nil {
			return err
		}
	}

	// Create rsync configmap/secret on source + destination
	// Create rsync secret (which contains user/pass for rsync transfer pod) in
	// each namespace being migrated
	// Needs to go in every namespace where a PVC is being migrated
	pvcMap := t.getPVCNamespaceMap()

	for ns, vols := range pvcMap {
		pvcList := []pvc{}
		for _, vol := range vols {
			dnsSafeName, err := getDNSSafeName(vol.Name)
			if err != nil {
				return err
			}
			pvcList = append(pvcList, pvc{Name: dnsSafeName})
		}
		// Generate template
		rsyncConf := rsyncConfig{
			SshUser:   "root",
			Namespace: ns,
			PVCList:   pvcList,
			Password:  password,
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
				Name:      DirectVolumeMigrationRsyncConfig,
			},
		}
		configMap.Labels = t.Owner.GetCorrelationLabels()
		configMap.Labels["app"] = DirectVolumeMigrationRsyncTransfer

		err = yaml.Unmarshal(tpl.Bytes(), &configMap)
		if err != nil {
			return err
		}

		// Create configmap on source + dest
		// Note: when this configmap changes the rsync pod
		// needs to restart
		// Need to launch new pod when configmap changes
		t.Log.Info("Creating Rsync Transfer Pod ConfigMap on destination cluster",
			"configMap", path.Join(configMap.Namespace, configMap.Name))
		err = destClient.Create(context.TODO(), &configMap)
		if k8serror.IsAlreadyExists(err) {
			t.Log.Info("Rsync Transfer Pod ConfigMap already exists on destination",
				"configMap", path.Join(configMap.Namespace, configMap.Name))
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
				Name:      DirectVolumeMigrationRsyncCreds,
			},
			Data: map[string][]byte{
				"RSYNC_PASSWORD": []byte(password),
			},
		}
		srcSecret.Labels = t.Owner.GetCorrelationLabels()
		srcSecret.Labels["app"] = DirectVolumeMigrationRsyncTransfer

		t.Log.Info("Creating Rsync Password Secret on source cluster",
			"secret", path.Join(srcSecret.Namespace, srcSecret.Name))
		err = srcClient.Create(context.TODO(), &srcSecret)
		if k8serror.IsAlreadyExists(err) {
			t.Log.Info("Rsync Password Secret already exists on source cluster", "namespace", srcSecret.Namespace)
		} else if err != nil {
			return err
		}
		destSecret := corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: ns,
				Name:      DirectVolumeMigrationRsyncCreds,
			},
			Data: map[string][]byte{
				"credentials": []byte("root:" + password),
			},
		}
		destSecret.Labels = t.Owner.GetCorrelationLabels()
		destSecret.Labels["app"] = DirectVolumeMigrationRsyncTransfer

		t.Log.Info("Creating Rsync Password Secret on destination cluster",
			"secret", path.Join(destSecret.Namespace, destSecret.Name))
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
	dvmLabels := t.buildDVMLabels()
	dvmLabels["purpose"] = DirectVolumeMigrationRsync

	for ns, _ := range pvcMap {
		svc := corev1.Service{
			ObjectMeta: metav1.ObjectMeta{
				Name:      DirectVolumeMigrationRsyncTransferSvc,
				Namespace: ns,
			},
			Spec: corev1.ServiceSpec{
				Ports: []corev1.ServicePort{
					{
						Name:       DirectVolumeMigrationStunnel,
						Protocol:   corev1.ProtocolTCP,
						Port:       int32(2222),
						TargetPort: intstr.IntOrString{Type: intstr.Int, IntVal: 2222},
					},
				},
				Selector: dvmLabels,
				Type:     corev1.ServiceTypeClusterIP,
			},
		}
		svc.Labels = t.Owner.GetCorrelationLabels()
		svc.Labels["app"] = DirectVolumeMigrationRsyncTransfer

		t.Log.Info("Creating Rsync Transfer Service for Stunnel connection "+
			"on destination MigCluster ",
			"service", path.Join(svc.Namespace, svc.Name))
		err = destClient.Create(context.TODO(), &svc)
		if k8serror.IsAlreadyExists(err) {
			t.Log.Info("Rsync transfer svc already exists on destination",
				"service", path.Join(svc.Namespace, svc.Name))
		} else if err != nil {
			return err
		}
		route := routev1.Route{
			ObjectMeta: metav1.ObjectMeta{
				Name:      DirectVolumeMigrationRsyncTransferRoute,
				Namespace: ns,
			},
			Spec: routev1.RouteSpec{
				To: routev1.RouteTargetReference{
					Kind: "Service",
					Name: DirectVolumeMigrationRsyncTransferSvc,
				},
				Port: &routev1.RoutePort{
					TargetPort: intstr.IntOrString{Type: intstr.Int, IntVal: 2222},
				},
				TLS: &routev1.TLSConfig{
					Termination: routev1.TLSTerminationPassthrough,
				},
			},
		}
		route.Labels = t.Owner.GetCorrelationLabels()
		route.Labels["app"] = DirectVolumeMigrationRsyncTransfer

		// Get cluster subdomain if it exists
		cluster, err := t.Owner.GetDestinationCluster(t.Client)
		if err != nil {
			return err
		}
		// Ignore error since this is optional config and won't break
		// anything if it doesn't exist
		subdomain, _ := cluster.GetClusterSubdomain(t.Client)

		// This is a backdoor setting to help guarantee DVM can still function if a
		// user is migrating namespaces that are 60+ characters
		// NOTE: We do no validation of this subdomain value. User is expected to
		// set this properly and it's only used for the unlikely case a user needs
		// to migrate namespaces with very long names.
		if subdomain != "" {
			// Ensure that route prefix will not exceed 63 chars
			// Route gen will add `-` between name + ns so need to ensure below is <62 chars
			// NOTE: only do this if we actually get a configured subdomain,
			// otherwise just use the name and hope for the best
			prefix := fmt.Sprintf("%s-%s", DirectVolumeMigrationRsyncTransferRoute, getMD5Hash(ns))
			if len(prefix) > 62 {
				prefix = prefix[0:62]
			}
			host := fmt.Sprintf("%s.%s", prefix, subdomain)
			route.Spec.Host = host
		}
		t.Log.Info("Creating Rsync Transfer Route for Stunnel connection "+
			"on destination MigCluster ",
			"route", path.Join(route.Namespace, route.Name))
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

	// Get transfer image
	cluster, err := t.Owner.GetDestinationCluster(t.Client)
	if err != nil {
		return err
	}
	t.Log.Info("Getting Rsync Transfer Pod image from ConfigMap.")
	transferImage, err := cluster.GetRsyncTransferImage(t.Client)
	if err != nil {
		return err
	}
	t.Log.Info("Getting Rsync Transfer Pod limits and requests from ConfigMap.")
	limits, requests, err := t.getPodResourceLists(TRANSFER_POD_CPU_LIMIT, TRANSFER_POD_MEMORY_LIMIT, TRANSFER_POD_CPU_REQUEST, TRANSFER_POD_MEMORY_REQUEST)
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
	t.Log.Info("Generating SSH public key for Rsync Transfer Pod")
	publicRsaKey, err := ssh.NewPublicKey(t.SSHKeys.PublicKey)
	if err != nil {
		return err
	}
	pubKeyBytes := ssh.MarshalAuthorizedKey(publicRsaKey)
	mode := int32(0600)

	isRsyncPrivileged, err := isRsyncPrivileged(destClient)
	if err != nil {
		return err
	}
	t.Log.Info(fmt.Sprintf("Rsync Transfer Pod will be created with privileged=[%v]",
		isRsyncPrivileged))
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
							Name: DirectVolumeMigrationStunnelConfig,
						},
					},
				},
			},
			{
				Name: "stunnel-certs",
				VolumeSource: corev1.VolumeSource{
					Secret: &corev1.SecretVolumeSource{
						SecretName: DirectVolumeMigrationStunnelCerts,
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
						SecretName:  DirectVolumeMigrationRsyncCreds,
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
							Name: DirectVolumeMigrationRsyncConfig,
						},
					},
				},
			},
		}
		trueBool := true
		runAsUser := int64(0)

		// Add PVC volume mounts
		for _, vol := range vols {
			dnsSafeName, err := getDNSSafeName(vol.Name)
			if err != nil {
				return err
			}
			volumeMounts = append(volumeMounts, corev1.VolumeMount{
				Name:      dnsSafeName,
				MountPath: fmt.Sprintf("/mnt/%s/%s", ns, dnsSafeName),
			})
			volumes = append(volumes, corev1.Volume{
				Name: dnsSafeName,
				VolumeSource: corev1.VolumeSource{
					PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{
						ClaimName: vol.Name,
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

		dvmLabels := t.buildDVMLabels()
		dvmLabels["purpose"] = DirectVolumeMigrationRsync

		transferPod := corev1.Pod{
			ObjectMeta: metav1.ObjectMeta{
				Name:      DirectVolumeMigrationRsyncTransfer,
				Namespace: ns,
				Labels:    dvmLabels,
			},
			Spec: corev1.PodSpec{
				Volumes: volumes,
				Containers: []corev1.Container{
					{
						Name:  "rsyncd",
						Image: transferImage,
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
							Privileged:             &isRsyncPrivileged,
							RunAsUser:              &runAsUser,
							ReadOnlyRootFilesystem: &trueBool,
							Capabilities: &corev1.Capabilities{
								Drop: []corev1.Capability{"MKNOD", "SETPCAP"},
							},
						},
						Resources: corev1.ResourceRequirements{
							Limits:   limits,
							Requests: requests,
						},
					},
					{
						Name:    DirectVolumeMigrationStunnel,
						Image:   transferImage,
						Command: []string{"/bin/stunnel", "/etc/stunnel/stunnel.conf"},
						Ports: []corev1.ContainerPort{
							{
								Name:          DirectVolumeMigrationStunnel,
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
							Privileged:             &isRsyncPrivileged,
							RunAsUser:              &runAsUser,
							ReadOnlyRootFilesystem: &trueBool,
							Capabilities: &corev1.Capabilities{
								Drop: []corev1.Capability{"MKNOD", "SETPCAP"},
							},
						},
					},
				},
			},
		}
		t.Log.Info("Creating Rsync Transfer Pod with containers [rsyncd, stunnel] on destination cluster.",
			"pod", path.Join(transferPod.Namespace, transferPod.Name))
		err = destClient.Create(context.TODO(), &transferPod)
		if k8serror.IsAlreadyExists(err) {
			t.Log.Info("Rsync transfer pod already exists on destination",
				"pod", path.Join(transferPod.Namespace, transferPod.Name))
		} else if err != nil {
			return err
		}
		t.Log.Info("Rsync transfer pod created",
			"pod", path.Join(transferPod.Namespace, transferPod.Name))

	}
	return nil
}

func (t *Task) getPodResourceLists(cpuLimit string, memoryLimit string, cpuRequests string, memRequests string) (corev1.ResourceList, corev1.ResourceList, error) {
	podConfigMap := &corev1.ConfigMap{}
	err := t.Client.Get(context.TODO(), types.NamespacedName{Name: "migration-controller", Namespace: migapi.OpenshiftMigrationNamespace}, podConfigMap)
	if err != nil {
		return nil, nil, err
	}
	limits := corev1.ResourceList{
		corev1.ResourceMemory: resource.MustParse("1Gi"),
		corev1.ResourceCPU:    resource.MustParse("1"),
	}
	if _, exists := podConfigMap.Data[cpuLimit]; exists {
		cpu := resource.MustParse(podConfigMap.Data[cpuLimit])
		limits[corev1.ResourceCPU] = cpu
	}
	if _, exists := podConfigMap.Data[memoryLimit]; exists {
		memory := resource.MustParse(podConfigMap.Data[memoryLimit])
		limits[corev1.ResourceMemory] = memory
	}
	requests := corev1.ResourceList{
		corev1.ResourceMemory: resource.MustParse("1Gi"),
		corev1.ResourceCPU:    resource.MustParse("400m"),
	}
	if _, exists := podConfigMap.Data[cpuRequests]; exists {
		cpu := resource.MustParse(podConfigMap.Data[cpuRequests])
		requests[corev1.ResourceCPU] = cpu
	}
	if _, exists := podConfigMap.Data[memRequests]; exists {
		memory := resource.MustParse(podConfigMap.Data[memRequests])
		requests[corev1.ResourceMemory] = memory
	}
	return limits, requests, nil
}

type pvcMapElement struct {
	Name   string
	Verify bool
}

func (t *Task) getPVCNamespaceMap() map[string][]pvcMapElement {
	nsMap := map[string][]pvcMapElement{}
	for _, pvc := range t.Owner.Spec.PersistentVolumeClaims {
		if vols, exists := nsMap[pvc.Namespace]; exists {
			vols = append(vols, pvcMapElement{Name: pvc.Name, Verify: pvc.Verify})
			nsMap[pvc.Namespace] = vols
		} else {
			nsMap[pvc.Namespace] = []pvcMapElement{{Name: pvc.Name, Verify: pvc.Verify}}
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

	key := types.NamespacedName{Name: DirectVolumeMigrationRsyncTransferRoute, Namespace: namespace}
	err = destClient.Get(context.TODO(), key, &route)
	if err != nil {
		return "", err
	}
	return route.Spec.Host, nil
}

func (t *Task) areRsyncRoutesAdmitted() (bool, []string, error) {
	messages := []string{}
	// Get client for destination
	destClient, err := t.getDestinationClient()
	if err != nil {
		return false, messages, err
	}
	nsMap := t.getPVCNamespaceMap()
	for namespace, _ := range nsMap {
		route := routev1.Route{}

		key := types.NamespacedName{Name: DirectVolumeMigrationRsyncTransferRoute, Namespace: namespace}
		err = destClient.Get(context.TODO(), key, &route)
		if err != nil {
			return false, messages, err
		}
		// Logs abnormal events related to route if any are found
		migevent.LogAbnormalEventsForResource(
			destClient, t.Log,
			"Found abnormal event for Rsync Route on destination cluster",
			types.NamespacedName{Namespace: route.Namespace, Name: route.Name}, "Route")

		admitted := false
		message := "no status condition available for the route"
		// Check if we can find the admitted condition for the route
		for _, ingress := range route.Status.Ingress {
			for _, condition := range ingress.Conditions {
				if condition.Type == routev1.RouteAdmitted && condition.Status == corev1.ConditionFalse {
					t.Log.Info("Rsync Transfer Route has not been admitted.",
						"route", path.Join(route.Namespace, route.Name))
					admitted = false
					message = condition.Message
					break
				}
				if condition.Type == routev1.RouteAdmitted && condition.Status == corev1.ConditionTrue {
					t.Log.Info("Rsync Transfer Route has been admitted successfully.",
						"route", path.Join(route.Namespace, route.Name))
					admitted = true
					break
				}
			}
		}
		if !admitted {
			messages = append(messages, message)
		}
	}
	if len(messages) > 0 {
		return false, messages, nil
	}
	return true, []string{}, nil
}

func (t *Task) createRsyncPassword() (string, error) {
	var letters = []byte("abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ")
	random.Seed(time.Now().UnixNano())
	password := make([]byte, 6)
	for i := range password {
		password[i] = letters[random.Intn(len(letters))]
	}

	secret := corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: migapi.OpenshiftMigrationNamespace,
			Name:      DirectVolumeMigrationRsyncPass,
		},
		StringData: map[string]string{
			corev1.BasicAuthPasswordKey: string(password),
		},
		Type: corev1.SecretTypeBasicAuth,
	}
	// Correlation labels for discovery service tree view
	secret.Labels = t.Owner.GetCorrelationLabels()
	secret.Labels["app"] = DirectVolumeMigrationRsyncTransfer

	t.Log.Info("Creating Rsync Password Secret on host cluster",
		"secret", path.Join(secret.Namespace, secret.Name))
	err := t.Client.Create(context.TODO(), &secret)
	if k8serror.IsAlreadyExists(err) {
		t.Log.Info("Secret already exists on host cluster",
			"secret", path.Join(secret.Namespace, secret.Name))
	} else if err != nil {
		return "", err
	}
	return string(password), nil
}

func (t *Task) getRsyncPassword() (string, error) {
	rsyncSecret := corev1.Secret{}
	key := types.NamespacedName{Name: DirectVolumeMigrationRsyncPass, Namespace: migapi.OpenshiftMigrationNamespace}
	t.Log.Info("Getting Rsync Password from Secret on host MigCluster",
		"secret", path.Join(rsyncSecret.Namespace, rsyncSecret.Name))
	err := t.Client.Get(context.TODO(), key, &rsyncSecret)
	if k8serror.IsNotFound(err) {
		t.Log.Info("Rsync Password Secret is not found on host MigCluster",
			"secret", path.Join(rsyncSecret.Namespace, rsyncSecret.Name))
		return "", nil
	}
	if err != nil {
		return "", err
	}
	if pass, ok := rsyncSecret.Data[corev1.BasicAuthPasswordKey]; ok {
		return string(pass), nil
	}
	return "", nil
}

func (t *Task) deleteRsyncPassword() error {
	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: migapi.OpenshiftMigrationNamespace,
			Name:      DirectVolumeMigrationRsyncPass,
		},
	}
	t.Log.Info("Deleting Rsync password Secret on host MigCluster",
		"secret", path.Join(secret.Namespace, secret.Name))
	err := t.Client.Delete(context.TODO(), secret, k8sclient.PropagationPolicy(metav1.DeletePropagationBackground))
	if k8serror.IsNotFound(err) {
		t.Log.Info("Rsync Password Secret not found",
			"secret", path.Join(secret.Namespace, secret.Name))
	} else if err != nil {
		return err
	}
	return nil
}

//Returns a map of PVCNamespacedName to the pod.NodeName
func (t *Task) getPVCNodeNameMap() (map[string]string, error) {
	nodeNameMap := map[string]string{}
	pvcMap := t.getPVCNamespaceMap()

	srcClient, err := t.getSourceClient()
	if err != nil {
		return nil, err
	}

	for ns, _ := range pvcMap {

		nsPodList := corev1.PodList{}
		err = srcClient.List(context.TODO(), k8sclient.InNamespace(ns), &nsPodList)
		if err != nil {
			return nil, err
		}

		for _, pod := range nsPodList.Items {
			if pod.Status.Phase == corev1.PodRunning {
				for _, vol := range pod.Spec.Volumes {
					if vol.PersistentVolumeClaim != nil {
						pvcNsName := pod.ObjectMeta.Namespace + "/" + vol.PersistentVolumeClaim.ClaimName
						nodeNameMap[pvcNsName] = pod.Spec.NodeName
					}
				}
			}
		}
	}

	return nodeNameMap, nil
}

// validates extra Rsync options set by user
// only returns options identified as valid
func (t *Task) filterRsyncExtraOptions(options []string) (validatedOptions []string) {
	for _, opt := range options {
		if valid, _ := regexp.Match(`^\-{1,2}[\w-]+?\w$`, []byte(opt)); valid {
			validatedOptions = append(validatedOptions, opt)
		} else {
			t.Log.Info(fmt.Sprintf("Invalid Rsync extra option passed: %s", opt))
		}
	}
	return
}

// generates Rsync options based on custom options provided by the user in MigrationController CR
func (t *Task) getRsyncOptions() []string {
	var rsyncOpts []string
	defaultInfoOpts := "COPY2,DEL2,REMOVE2,SKIP2,FLIST2,PROGRESS2,STATS2"
	defaultExtraOpts := []string{
		"--human-readable",
		"--port", "2222",
		"--log-file", "/dev/stdout",
	}
	rsyncOptions := settings.Settings.DvmOpts.RsyncOpts
	if rsyncOptions.BwLimit != -1 {
		rsyncOpts = append(rsyncOpts,
			fmt.Sprintf("--bwlimit=%d", rsyncOptions.BwLimit))
	}
	if rsyncOptions.Archive {
		rsyncOpts = append(rsyncOpts, "--archive")
	}
	if rsyncOptions.Delete {
		rsyncOpts = append(rsyncOpts, "--delete")
		// --delete option does not work without --recursive
		rsyncOpts = append(rsyncOpts, "--recursive")
	}
	if rsyncOptions.HardLinks {
		rsyncOpts = append(rsyncOpts, "--hard-links")
	}
	if rsyncOptions.Partial {
		rsyncOpts = append(rsyncOpts, "--partial")
	}
	if valid, _ := regexp.Match(`^\w[\w,]*?\w$`, []byte(rsyncOptions.Info)); valid {
		rsyncOpts = append(rsyncOpts,
			fmt.Sprintf("--info=%s", rsyncOptions.Info))
	} else {
		rsyncOpts = append(rsyncOpts,
			fmt.Sprintf("--info=%s", defaultInfoOpts))
	}
	rsyncOpts = append(rsyncOpts, defaultExtraOpts...)
	rsyncOpts = append(rsyncOpts,
		t.filterRsyncExtraOptions(rsyncOptions.Extras)...)
	return rsyncOpts
}

type PVCWithSecurityContext struct {
	name               string
	dnsSafeName        string
	fsGroup            *int64
	supplementalGroups []int64
	seLinuxOptions     *corev1.SELinuxOptions
	verify             bool

	// TODO:
	// add capabilities for dvm controller to handle case the source
	// application pods is privileged with the following flags from
	// PodSecurityContext and Containers' SecurityContext
	// We need to
	// 1. go through the pod's volume.
	// 2. find the container where it is volume mounted.
	//    i. if the container's fields are non-nil, use that
	//    ii. if the container's fields are nil, use the pods fields.

	// RunAsUser  *int64
	// RunAsGroup *int64
}

// Get fsGroup per PVC
func (t *Task) getfsGroupMapForNamespace() (map[string][]PVCWithSecurityContext, error) {
	pvcMap := t.getPVCNamespaceMap()
	pvcSecurityContextMap := map[string][]PVCWithSecurityContext{}
	for ns, _ := range pvcMap {
		pvcSecurityContextMap[ns] = []PVCWithSecurityContext{}
	}
	for ns, pvcs := range pvcMap {
		srcClient, err := t.getSourceClient()
		if err != nil {
			return nil, err
		}
		podList := &corev1.PodList{}
		err = srcClient.List(context.TODO(), &k8sclient.ListOptions{Namespace: ns}, podList)
		if err != nil {
			return nil, err
		}

		// for each namespace, have a pvc->SCC map to look up in the pvc loop later
		// we will use the scc of the last pod in the list mounting the pvc
		pvcSecurityContextMapForNamespace := map[string]PVCWithSecurityContext{}
		for _, pod := range podList.Items {
			for _, vol := range pod.Spec.Volumes {
				if vol.PersistentVolumeClaim != nil {
					dnsSafeName, err := getDNSSafeName(vol.PersistentVolumeClaim.ClaimName)
					if err != nil {
						return nil, err
					}
					pvcSecurityContextMapForNamespace[vol.PersistentVolumeClaim.ClaimName] = PVCWithSecurityContext{
						name:               vol.PersistentVolumeClaim.ClaimName,
						dnsSafeName:        dnsSafeName,
						fsGroup:            pod.Spec.SecurityContext.FSGroup,
						supplementalGroups: pod.Spec.SecurityContext.SupplementalGroups,
						seLinuxOptions:     pod.Spec.SecurityContext.SELinuxOptions,
					}
				}
			}
		}

		for _, claim := range pvcs {
			pss, exists := pvcSecurityContextMapForNamespace[claim.Name]
			if exists {
				pss.verify = claim.Verify
				pvcSecurityContextMap[ns] = append(pvcSecurityContextMap[ns], pss)
				continue
			}
			dnsSafeName, err := getDNSSafeName(claim.Name)
			if err != nil {
				return nil, err
			}
			// pvc not used by any pod
			pvcSecurityContextMap[ns] = append(pvcSecurityContextMap[ns], PVCWithSecurityContext{
				name:               claim.Name,
				dnsSafeName:        dnsSafeName,
				fsGroup:            nil,
				supplementalGroups: nil,
				seLinuxOptions:     nil,
				verify:             claim.Verify,
			})
		}
	}
	return pvcSecurityContextMap, nil
}

func isClaimUsedByPod(claimName string, p *corev1.Pod) bool {
	for _, vol := range p.Spec.Volumes {
		if vol.PersistentVolumeClaim != nil && vol.PersistentVolumeClaim.ClaimName == claimName {
			return true
		}
	}
	return false
}

func isRsyncPrivileged(client compat.Client) (bool, error) {
	cm := &corev1.ConfigMap{}
	err := client.Get(context.TODO(), k8sclient.ObjectKey{Name: migapi.ClusterConfigMapName, Namespace: migapi.OpenshiftMigrationNamespace}, cm)
	if err != nil {
		return false, err
	}
	if cm.Data != nil {
		isRsyncPrivileged, exists := cm.Data["RSYNC_PRIVILEGED"]
		if !exists {
			return false, fmt.Errorf("RSYNC_PRIVILEGED boolean does not exist. Verify source and destination clusters operators are up to date")
		}
		parsed, err := strconv.ParseBool(isRsyncPrivileged)
		if err != nil {
			return false, err
		}
		return parsed, nil
	}
	return false, fmt.Errorf("configmap %s of source cluster has empty data", k8sclient.ObjectKey{Name: migapi.ClusterConfigMapName, Namespace: migapi.OpenshiftMigrationNamespace}.String())
}

// deleteInvalidPVProgressCR deletes an existing CR which doesn't have expected fields
// used to delete CRs created pre MTCv1.4.3
func (t *Task) deleteInvalidPVProgressCR(dvmpName string, dvmpNamespace string) error {
	existingDvmp := migapi.DirectVolumeMigrationProgress{}
	// Make sure existing DVMP CRs which don't have required fields are deleted
	err := t.Client.Get(context.TODO(), types.NamespacedName{Name: dvmpName, Namespace: dvmpNamespace}, &existingDvmp)
	if err != nil {
		if !k8serror.IsNotFound(err) {
			return err
		}
	}
	if existingDvmp.Name != "" && existingDvmp.Namespace != "" {
		shouldDelete := false
		// if any of podNamespace or podSelector is missing, delete the CR
		if existingDvmp.Spec.PodNamespace == "" || existingDvmp.Spec.PodSelector == nil {
			shouldDelete = true
		}
		// if podSelector doesn't have a required label, delete the CR
		if existingDvmp.Spec.PodSelector != nil {
			if _, exists := existingDvmp.Spec.PodSelector[migapi.RsyncPodIdentityLabel]; !exists {
				shouldDelete = true
			}
		}
		if shouldDelete {
			err := t.Client.Delete(context.TODO(), &existingDvmp)
			if err != nil {
				return err
			}
			t.Log.Info("Deleted DVMP as it was missing required fields", "DVMP", path.Join(dvmpNamespace, dvmpName))
		}
	}
	return nil
}

// Create rsync PV progress CR on destination cluster
func (t *Task) createPVProgressCR() error {
	pvcMap := t.getPVCNamespaceMap()
	labels := t.Owner.GetCorrelationLabels()
	for ns, vols := range pvcMap {
		for _, vol := range vols {
			dvmp := migapi.DirectVolumeMigrationProgress{
				ObjectMeta: metav1.ObjectMeta{
					Name:      getMD5Hash(t.Owner.Name + vol.Name + ns),
					Labels:    labels,
					Namespace: migapi.OpenshiftMigrationNamespace,
				},
				Spec: migapi.DirectVolumeMigrationProgressSpec{
					ClusterRef:   t.Owner.Spec.SrcMigClusterRef,
					PodNamespace: ns,
					PodSelector:  GetRsyncPodSelector(vol.Name),
				},
			}
			// make sure existing CRs that don't have required fields are deleted
			err := t.deleteInvalidPVProgressCR(dvmp.Name, dvmp.Namespace)
			if err != nil {
				return liberr.Wrap(err)
			}
			migapi.SetOwnerReference(t.Owner, t.Owner, &dvmp)
			t.Log.Info("Creating DVMP on host MigCluster to track Rsync Pod completion on MigCluster",
				"dvmp", path.Join(dvmp.Namespace, dvmp.Name),
				"srcNamespace", dvmp.Spec.PodNamespace,
				"selector", dvmp.Spec.PodSelector,
				"migCluster", path.Join(t.Owner.Spec.SrcMigClusterRef.Namespace,
					t.Owner.Spec.SrcMigClusterRef.Name))
			err = t.Client.Create(context.TODO(), &dvmp)
			if k8serror.IsAlreadyExists(err) {
				t.Log.Info("DVMP already exists on destination cluster",
					"dvmp", path.Join(dvmp.Namespace, dvmp.Name))
			} else if err != nil {
				return err
			}
			t.Log.Info("Rsync client progress CR created", "dvmp", path.Join(dvmp.Name, "namespace", dvmp.Namespace))
		}

	}
	return nil
}

func getMD5Hash(s string) string {
	hash := md5.Sum([]byte(s))
	return hex.EncodeToString(hash[:])
}

// hasAllProgressReportingCompleted reads DVMP CR and status of Rsync Operations present for all PVCs and generates progress information in CR status
// returns True when progress reporting for all Rsync Pods is complete
func (t *Task) hasAllProgressReportingCompleted() (bool, error) {
	t.Owner.Status.RunningPods = []*migapi.PodProgress{}
	t.Owner.Status.FailedPods = []*migapi.PodProgress{}
	t.Owner.Status.SuccessfulPods = []*migapi.PodProgress{}
	t.Owner.Status.PendingPods = []*migapi.PodProgress{}
	var pendingSinceTimeLimitPods []string
	pvcMap := t.getPVCNamespaceMap()
	for ns, vols := range pvcMap {
		for _, vol := range vols {
			operation := t.Owner.Status.GetRsyncOperationStatusForPVC(&corev1.ObjectReference{
				Namespace: ns,
				Name:      vol.Name,
			})
			dvmp := migapi.DirectVolumeMigrationProgress{}
			err := t.Client.Get(context.TODO(), types.NamespacedName{
				Name:      getMD5Hash(t.Owner.Name + vol.Name + ns),
				Namespace: migapi.OpenshiftMigrationNamespace,
			}, &dvmp)
			if err != nil {
				return false, err
			}
			podProgress := &migapi.PodProgress{
				ObjectReference: &corev1.ObjectReference{
					Namespace: ns,
					Name:      dvmp.Status.PodName,
				},
				PVCReference: &corev1.ObjectReference{
					Namespace: ns,
					Name:      vol.Name,
				},
				LastObservedProgressPercent: dvmp.Status.TotalProgressPercentage,
				LastObservedTransferRate:    dvmp.Status.LastObservedTransferRate,
				TotalElapsedTime:            dvmp.Status.RsyncElapsedTime,
			}
			switch {
			case dvmp.Status.PodPhase == corev1.PodRunning:
				t.Owner.Status.RunningPods = append(t.Owner.Status.RunningPods, podProgress)
			case operation.Failed:
				t.Owner.Status.FailedPods = append(t.Owner.Status.FailedPods, podProgress)
			case dvmp.Status.PodPhase == corev1.PodSucceeded:
				t.Owner.Status.SuccessfulPods = append(t.Owner.Status.SuccessfulPods, podProgress)
			case dvmp.Status.PodPhase == corev1.PodPending:
				t.Owner.Status.PendingPods = append(t.Owner.Status.PendingPods, podProgress)
				if dvmp.Status.CreationTimestamp != nil {
					if time.Now().UTC().Sub(dvmp.Status.CreationTimestamp.Time.UTC()) > PendingPodWarningTimeLimit {
						pendingSinceTimeLimitPods = append(pendingSinceTimeLimitPods, fmt.Sprintf("%s/%s", podProgress.Namespace, podProgress.Name))
					}
				}
			case !operation.Failed:
				t.Owner.Status.RunningPods = append(t.Owner.Status.RunningPods, podProgress)
			}
		}
	}

	isCompleted := len(t.Owner.Status.SuccessfulPods)+len(t.Owner.Status.FailedPods) == len(t.Owner.Spec.PersistentVolumeClaims)
	isAnyPending := len(t.Owner.Status.PendingPods) > 0
	isAnyRunning := len(t.Owner.Status.RunningPods) > 0
	if len(pendingSinceTimeLimitPods) > 0 {
		pendingMessage := fmt.Sprintf("Rsync Client Pods [%s] are stuck in Pending state for more than 10 mins", strings.Join(pendingSinceTimeLimitPods[:], ", "))
		t.Log.Info(pendingMessage)
		t.Owner.Status.SetCondition(migapi.Condition{
			Type:     RsyncClientPodsPending,
			Status:   migapi.True,
			Reason:   "PodStuckInContainerCreating",
			Category: migapi.Warn,
			Message:  pendingMessage,
		})
	}
	return !isAnyRunning && !isAnyPending && isCompleted, nil
}

func (t *Task) hasAllRsyncClientPodsTimedOut() (bool, error) {
	for ns, vols := range t.getPVCNamespaceMap() {
		for _, vol := range vols {
			dvmp := migapi.DirectVolumeMigrationProgress{}
			err := t.Client.Get(context.TODO(), types.NamespacedName{
				Name:      getMD5Hash(t.Owner.Name + vol.Name + ns),
				Namespace: migapi.OpenshiftMigrationNamespace,
			}, &dvmp)
			if err != nil {
				return false, err
			}
			if dvmp.Status.PodPhase != corev1.PodFailed ||
				dvmp.Status.ContainerElapsedTime == nil ||
				(dvmp.Status.ContainerElapsedTime != nil &&
					dvmp.Status.ContainerElapsedTime.Duration.Round(time.Second).Seconds() != float64(DefaultStunnelTimeout)) {
				return false, nil
			}
		}
	}
	return true, nil
}

func (t *Task) isAllRsyncClientPodsNoRouteToHost() (bool, error) {
	for ns, vols := range t.getPVCNamespaceMap() {
		for _, vol := range vols {
			dvmp := migapi.DirectVolumeMigrationProgress{}
			err := t.Client.Get(context.TODO(), types.NamespacedName{
				Name:      getMD5Hash(t.Owner.Name + vol.Name + ns),
				Namespace: migapi.OpenshiftMigrationNamespace,
			}, &dvmp)
			if err != nil {
				return false, err
			}

			if dvmp.Status.PodPhase != corev1.PodFailed ||
				dvmp.Status.ContainerElapsedTime == nil ||
				(dvmp.Status.ContainerElapsedTime != nil &&
					dvmp.Status.ContainerElapsedTime.Duration.Seconds() > float64(5)) || *dvmp.Status.ExitCode != int32(10) || !strings.Contains(dvmp.Status.LogMessage, "No route to host") {
				return false, nil
			}
		}
	}
	return true, nil
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

	t.Log.Info("Checking for stale Rsync resources on source MigCluster",
		"migCluster",
		path.Join(t.Owner.Spec.SrcMigClusterRef.Namespace, t.Owner.Spec.SrcMigClusterRef.Name))
	err = t.findAndDeleteResources(srcClient)
	if err != nil {
		return err
	}

	t.Log.Info("Checking for stale Rsync resources on destination MigCluster",
		"migCluster",
		path.Join(t.Owner.Spec.DestMigClusterRef.Namespace, t.Owner.Spec.DestMigClusterRef.Name))
	err = t.findAndDeleteResources(destClient)
	if err != nil {
		return err
	}

	err = t.deleteRsyncPassword()
	if err != nil {
		return err
	}

	if !t.Owner.Spec.DeleteProgressReportingCRs {
		return nil
	}

	t.Log.Info("Checking for stale DVMP resources on host MigCluster",
		"migCluster", "host")
	err = t.deleteProgressReportingCRs(t.Client)
	if err != nil {
		return err
	}

	return nil
}

func (t *Task) waitForRsyncResourcesDeleted() (error, bool) {
	srcClient, err := t.getSourceClient()
	if err != nil {
		return err, false
	}
	destClient, err := t.getDestinationClient()
	if err != nil {
		return err, false
	}
	t.Log.Info("Checking if Rsync resource deletion has completed on source MigCluster")
	err, deleted := t.areRsyncResourcesDeleted(srcClient)
	if err != nil {
		return err, false
	}
	if !deleted {
		return nil, false
	}
	t.Log.Info("Checking if Rsync resource deletion has completed on destination MigCluster")
	err, deleted = t.areRsyncResourcesDeleted(destClient)
	if err != nil {
		return err, false
	}
	if !deleted {
		return nil, false
	}
	return nil, true
}

func (t *Task) areRsyncResourcesDeleted(client compat.Client) (error, bool) {
	selector := labels.SelectorFromSet(map[string]string{
		"app": DirectVolumeMigrationRsyncTransfer,
	})
	pvcMap := t.getPVCNamespaceMap()
	for ns, _ := range pvcMap {
		t.Log.Info("Searching namespace for leftover Rsync Pods, ConfigMaps, "+
			"Services, Secrets, Routes with label.",
			"searchNamespace", ns,
			"labelSelector", selector)
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
			return err, false
		}
		if len(podList.Items) > 0 {
			t.Log.Info("Found stale Rsync Pod.",
				"pod", path.Join(podList.Items[0].Namespace, podList.Items[0].Name),
				"podPhase", podList.Items[0].Status.Phase)
			return nil, false
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
			return err, false
		}
		if len(secretList.Items) > 0 {
			t.Log.Info("Found stale Rsync Secret.",
				"secret", path.Join(secretList.Items[0].Namespace, secretList.Items[0].Name))
			return nil, false
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
			return err, false
		}
		if len(cmList.Items) > 0 {
			t.Log.Info("Found stale Rsync ConfigMap.",
				"configMap", path.Join(cmList.Items[0].Namespace, cmList.Items[0].Name))
			return nil, false
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
			return err, false
		}
		if len(svcList.Items) > 0 {
			t.Log.Info("Found stale Rsync Service.",
				"service", path.Join(svcList.Items[0].Namespace, svcList.Items[0].Name))
			return nil, false
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
			return err, false
		}
		if len(routeList.Items) > 0 {
			t.Log.Info("Found stale Rsync Route.",
				"route", path.Join(routeList.Items[0].Namespace, routeList.Items[0].Name))
			return nil, false
		}
	}
	return nil, true

}

func (t *Task) findAndDeleteResources(client compat.Client) error {
	// Find all resources with the app label
	// TODO: This label set should include a DVM run-specific UID.
	selector := labels.SelectorFromSet(map[string]string{
		"app": DirectVolumeMigrationRsyncTransfer,
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
			t.Log.Info("Deleting stale DVM Pod",
				"pod", path.Join(pod.Namespace, pod.Name))
			err = client.Delete(context.TODO(), &pod, k8sclient.PropagationPolicy(metav1.DeletePropagationBackground))
			if err != nil && !k8serror.IsNotFound(err) {
				return err
			}
		}

		// Delete secrets
		for _, secret := range secretList.Items {
			t.Log.Info("Deleting stale DVM Secret",
				"secret", path.Join(secret.Namespace, secret.Name))
			err = client.Delete(context.TODO(), &secret, k8sclient.PropagationPolicy(metav1.DeletePropagationBackground))
			if err != nil && !k8serror.IsNotFound(err) {
				return err
			}
		}

		// Delete routes
		for _, route := range routeList.Items {
			t.Log.Info("Deleting stale DVM Route",
				"route", path.Join(route.Namespace, route.Name))
			err = client.Delete(context.TODO(), &route, k8sclient.PropagationPolicy(metav1.DeletePropagationBackground))
			if err != nil && !k8serror.IsNotFound(err) {
				return err
			}
		}

		// Delete svcs
		for _, svc := range svcList.Items {
			t.Log.Info("Deleting stale DVM Service",
				"service", path.Join(svc.Namespace, svc.Name))
			err = client.Delete(context.TODO(), &svc, k8sclient.PropagationPolicy(metav1.DeletePropagationBackground))
			if err != nil && !k8serror.IsNotFound(err) {
				return err
			}
		}

		// Delete configmaps
		for _, cm := range cmList.Items {
			t.Log.Info("Deleting stale DVM ConfigMap",
				"configMap", path.Join(cm.Namespace, cm.Name))
			err = client.Delete(context.TODO(), &cm, k8sclient.PropagationPolicy(metav1.DeletePropagationBackground))
			if err != nil && !k8serror.IsNotFound(err) {
				return err
			}
		}
	}
	return nil
}

func (t *Task) deleteProgressReportingCRs(client k8sclient.Client) error {
	pvcMap := t.getPVCNamespaceMap()

	for ns, vols := range pvcMap {
		for _, vol := range vols {
			dvmpName := getMD5Hash(t.Owner.Name + vol.Name + ns)
			t.Log.Info("Deleting stale DVMP CR.",
				"dvmp", path.Join(dvmpName, ns))
			err := client.Delete(context.TODO(), &migapi.DirectVolumeMigrationProgress{
				ObjectMeta: metav1.ObjectMeta{
					Name:      dvmpName,
					Namespace: ns,
				},
			}, k8sclient.PropagationPolicy(metav1.DeletePropagationBackground))
			if err != nil && !k8serror.IsNotFound(err) {
				return err
			}
		}
	}
	return nil
}

// rsyncClientPodRequirements represents information required to create a client Rsync Pod
type rsyncClientPodRequirements struct {
	// pvInfo stuctured PVC info for which Pod will be created
	pvInfo PVCWithSecurityContext
	// namespace ns in which Rsync Pod will be created
	namespace string
	// image image used by the Rsync Pod
	image string
	// password password used by startup Rsync command
	password string
	// privileged whether Rsync Pod will run privileged
	privileged bool
	// resourceReq resource requirements for the Pod
	resourceReq corev1.ResourceRequirements
	// nodeName node on which Rsync Pod will be launched
	nodeName string
	// destIP destination IP address for Stunnel route
	destIP string
	// rsyncOptions rsync command to execute
	rsyncOptions []string
}

// getRsyncClientPodTemplate given RsyncClientPodRequirements, returns a Pod template
func (req rsyncClientPodRequirements) getRsyncClientPodTemplate() corev1.Pod {
	runAsUser := int64(0)
	trueBool := true
	isPrivileged := req.privileged
	volumes := []corev1.Volume{}
	volumeMounts := []corev1.VolumeMount{}
	containers := []corev1.Container{}
	volumeMounts = append(volumeMounts, corev1.VolumeMount{
		Name:      req.pvInfo.dnsSafeName,
		MountPath: fmt.Sprintf("/mnt/%s/%s", req.namespace, req.pvInfo.dnsSafeName),
	})
	volumes = append(volumes, corev1.Volume{
		Name: req.pvInfo.dnsSafeName,
		VolumeSource: corev1.VolumeSource{
			PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{
				ClaimName: req.pvInfo.name,
			},
		},
	})
	rsyncCommand := []string{"rsync"}
	rsyncCommand = append(rsyncCommand, req.rsyncOptions...)
	rsyncCommand = append(rsyncCommand, fmt.Sprintf("/mnt/%s/%s/", req.namespace, req.pvInfo.dnsSafeName))
	rsyncCommand = append(rsyncCommand, fmt.Sprintf("rsync://root@%s/%s", req.destIP, req.pvInfo.dnsSafeName))
	labels := map[string]string{
		"app":                   DirectVolumeMigrationRsyncTransfer,
		"directvolumemigration": DirectVolumeMigrationRsyncClient,
		migapi.PartOfLabel:      migapi.Application,
	}
	labels = Union(labels, GetRsyncPodSelector(req.pvInfo.name))
	containers = append(containers, corev1.Container{
		Name:  DirectVolumeMigrationRsyncClient,
		Image: req.image,
		Env: []corev1.EnvVar{
			{
				Name:  "RSYNC_PASSWORD",
				Value: req.password,
			},
		},
		TerminationMessagePolicy: corev1.TerminationMessageFallbackToLogsOnError,
		Command:                  rsyncCommand,
		Ports: []corev1.ContainerPort{
			{
				Name:          DirectVolumeMigrationRsyncClient,
				Protocol:      corev1.ProtocolTCP,
				ContainerPort: int32(22),
			},
		},
		VolumeMounts: volumeMounts,
		SecurityContext: &corev1.SecurityContext{
			Privileged:             &isPrivileged,
			RunAsUser:              &runAsUser,
			ReadOnlyRootFilesystem: &trueBool,
		},
		Resources: req.resourceReq,
	})
	clientPod := corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			GenerateName: "dvm-rsync-",
			Namespace:    req.namespace,
			Labels:       labels,
		},
		Spec: corev1.PodSpec{
			RestartPolicy: corev1.RestartPolicyNever,
			Volumes:       volumes,
			Containers:    containers,
			NodeName:      req.nodeName,
			SecurityContext: &corev1.PodSecurityContext{
				SupplementalGroups: req.pvInfo.supplementalGroups,
				FSGroup:            req.pvInfo.fsGroup,
				SELinuxOptions:     req.pvInfo.seLinuxOptions,
			},
		},
	}
	return clientPod
}

func (t *Task) prepareRsyncPodRequirements(srcClient compat.Client) ([]rsyncClientPodRequirements, error) {
	req := []rsyncClientPodRequirements{}
	cluster, err := t.Owner.GetSourceCluster(t.Client)
	if err != nil {
		return req, liberr.Wrap(err)
	}
	t.Log.V(4).Info("Getting image for Rsync client Pods that will be created on source MigCluster")
	transferImage, err := cluster.GetRsyncTransferImage(t.Client)
	if err != nil {
		return req, liberr.Wrap(err)
	}
	t.Log.V(4).Info("Getting [NS => PVCWithSecurityContext] mappings for PVCs to be migrated")
	pvcMap, err := t.getfsGroupMapForNamespace()
	if err != nil {
		return req, liberr.Wrap(err)
	}
	t.Log.V(4).Info("Getting Rsync password for Rsync client Pods")
	password, err := t.getRsyncPassword()
	if err != nil {
		return req, liberr.Wrap(err)
	}
	t.Log.V(4).Info("Getting [PVC => NodeName] mapping for PVCs to be migrated")
	pvcNodeMap, err := t.getPVCNodeNameMap()
	if err != nil {
		return req, liberr.Wrap(err)
	}
	t.Log.V(4).Info("Getting limits and requests for Rsync client Pods")
	limits, requests, err := t.getPodResourceLists(CLIENT_POD_CPU_LIMIT, CLIENT_POD_MEMORY_LIMIT, CLIENT_POD_CPU_REQUEST, CLIENT_POD_MEMORY_REQUEST)
	if err != nil {
		return req, liberr.Wrap(err)
	}
	isPrivileged, _ := isRsyncPrivileged(srcClient)
	t.Log.V(4).Info(fmt.Sprintf("Rsync client Pods will be created with privileged=[%v]", isPrivileged))
	for ns, vols := range pvcMap {
		// Add PVC volume mounts
		for _, vol := range vols {
			rsyncOptions := t.getRsyncOptions()
			if vol.verify {
				rsyncOptions = append(rsyncOptions, "--checksum")
			}
			podRequirements := rsyncClientPodRequirements{
				pvInfo:    vol,
				namespace: ns,
				image:     transferImage,
				password:  password,
				resourceReq: corev1.ResourceRequirements{
					Limits:   limits,
					Requests: requests,
				},
				privileged:   isPrivileged,
				nodeName:     pvcNodeMap[ns+"/"+vol.name],
				destIP:       fmt.Sprintf("%s.%s.svc", DirectVolumeMigrationRsyncTransferSvc, ns),
				rsyncOptions: rsyncOptions,
			}
			req = append(req, podRequirements)
		}
	}
	return req, nil
}

func (t *Task) getRsyncOperationsRequirements() (compat.Client, []rsyncClientPodRequirements, error) {
	srcClient, err := t.getSourceClient()
	if err != nil {
		return nil, nil, liberr.Wrap(err)
	}
	podRequirements, err := t.prepareRsyncPodRequirements(srcClient)
	if err != nil {
		return nil, nil, liberr.Wrap(err)
	}
	return srcClient, podRequirements, nil
}

func GetRsyncPodBackOffLimit(dvm migapi.DirectVolumeMigration) int {
	overriddenBackOffLimit := settings.Settings.DvmOpts.RsyncOpts.BackOffLimit
	// when both the spec and the overridden backoff limits are not set, use default
	if dvm.Spec.BackOffLimit == 0 && overriddenBackOffLimit == 0 {
		return DefaultRsyncBackOffLimit
	}
	// whenever set, prefer overridden limit over the one set through Spec
	if overriddenBackOffLimit != 0 {
		return overriddenBackOffLimit
	}
	return dvm.Spec.BackOffLimit
}

// runRsyncOperations creates pod requirements for Rsync pods for all PVCs present in the spec
// runs Rsync operations for all PVCs concurrently, processes outputs of each operation
// returns whether or not all operations are completed, whether any of the operation is failed, and a list of failure reasons
func (t *Task) runRsyncOperations() (bool, bool, []string, error) {
	var failureReasons []string
	srcClient, podRequirements, err := t.getRsyncOperationsRequirements()
	if err != nil {
		return false, false, failureReasons, liberr.Wrap(err)
	}
	status, garbageCollectionErrors := t.ensureRsyncOperations(srcClient, podRequirements)
	if err != nil {
		return false, false, failureReasons, liberr.Wrap(err)
	}
	// report progress of pods
	progressCompleted, err := t.hasAllProgressReportingCompleted()
	if err != nil {
		return false, false, failureReasons, liberr.Wrap(err)
	}
	operationsCompleted, anyFailed, failureReasons, err := t.processRsyncOperationStatus(status, garbageCollectionErrors)
	if err != nil {
		return false, false, failureReasons, liberr.Wrap(err)
	}
	return operationsCompleted && progressCompleted, anyFailed, failureReasons, nil
}

// processRsyncOperationStatus processes status of Rsync operations by reading the status list
// returns whether all operations are completed and whether any of the operation is failed
func (t *Task) processRsyncOperationStatus(status rsyncClientOperationStatusList, garbageCollectionErrors []error) (bool, bool, []string, error) {
	isComplete, anyFailed, failureReasons := false, false, make([]string, 0)
	if status.AllCompleted() {
		isComplete = true
		// we are done running rsync, we can move on
		// need to check whether there are any permanent failures
		if status.Failed() > 0 {
			anyFailed = true
			// attempt to categorize failures in any of the special failure categories we defined
			failureReasons, err := t.reportAdvancedErrorHeuristics()
			if err != nil {
				return isComplete, anyFailed, failureReasons, liberr.Wrap(err)
			}
		}
		return isComplete, anyFailed, failureReasons, nil
	}
	if status.AnyErrored() {
		// check if we are seeing errors running any of the operation for over 5 minutes
		// if yes, set a warning condition
		runningCondition := t.Owner.Status.Conditions.FindCondition(Running)
		if runningCondition != nil &&
			time.Now().Add(time.Minute*-5).After(runningCondition.LastTransitionTime.Time) {
			t.Owner.Status.SetCondition(migapi.Condition{
				Category: Warn,
				Type:     FailedCreatingRsyncPods,
				Message:  "Repeated errors occurred when attempting to create one or more Rsync pods in the source cluster. Please check controller logs for details.",
				Reason:   Failed,
				Status:   True,
			})
		}
		t.Log.Info("encountered repeated errors attempting to create Rsync Pods")
	}
	if len(garbageCollectionErrors) > 0 {
		// check if we are seeing errors running any of the operation for over 5 minutes
		// if yes, set a warning condition
		runningCondition := t.Owner.Status.Conditions.FindCondition(Running)
		if runningCondition != nil &&
			time.Now().Add(time.Minute*-5).After(runningCondition.LastTransitionTime.Time) {
			t.Owner.Status.SetCondition(migapi.Condition{
				Category: Warn,
				Type:     FailedDeletingRsyncPods,
				Message:  "Repeated errors occurred when attempting to delete one or more Rsync pods in the source cluster. Please check controller logs for details.",
				Reason:   Failed,
				Status:   True,
			})
		}
		t.Log.Info("encountered repeated errors attempting to garbage clean Rsync Pods")
	}
	return isComplete, anyFailed, failureReasons, nil
}

// reportAdvancedErrorHeuristics processes DVMP CRs for all PVCs,
// for all errored pods, attempts to determine whether the errors fall into any
// of the special categories we can identify and reports them as conditions
// returns reasons and error for reconcile decisions
func (t *Task) reportAdvancedErrorHeuristics() ([]string, error) {
	reasons := make([]string, 0)
	// check if the pods are failing due to a network misconfiguration causing Stunnel to timeout
	isStunnelTimeout, err := t.hasAllRsyncClientPodsTimedOut()
	if err != nil {
		return reasons, liberr.Wrap(err)
	}
	if isStunnelTimeout {
		t.Owner.Status.SetCondition(migapi.Condition{
			Type:     SourceToDestinationNetworkError,
			Status:   True,
			Reason:   RsyncTimeout,
			Category: migapi.Critical,
			Message: "All the rsync client pods on source are timing out at 20 seconds, " +
				"please check your network configuration (like egressnetworkpolicy) that would block traffic from " +
				"source namespace to destination",
			Durable: true,
		})
		t.Log.Info("Timeout error observed in all Rsync Pods")
		reasons = append(reasons, "All the source cluster Rsync Pods have timed out, look at error condition for more details")
		return reasons, nil
	}
	// check if the pods are failing due to 'No route to host' error
	isNoRouteToHost, err := t.isAllRsyncClientPodsNoRouteToHost()
	if err != nil {
		return reasons, liberr.Wrap(err)
	}
	if isNoRouteToHost {
		t.Owner.Status.SetCondition(migapi.Condition{
			Type:     SourceToDestinationNetworkError,
			Status:   True,
			Reason:   RsyncNoRouteToHost,
			Category: migapi.Critical,
			Message: "All Rsync client Pods on Source Cluster are failing because of \"no route to host\" error," +
				"please check your network configuration",
			Durable: true,
		})
		t.Log.Info("'No route to host' error observed in all Rsync Pods")
		reasons = append(reasons, "All the source cluster Rsync Pods have timed out, look at error condition for more details")
	}
	return reasons, nil
}

// rsyncClientOperationStatus defines status of one Rsync operation
type rsyncClientOperationStatus struct {
	operation *migapi.RsyncOperation
	// When set,.means that all attempts have been exhausted resulting in a failure
	failed bool
	// When set, means that one out of all attempts succeeded
	succeeded bool
	// When set, means that the operation is waiting for pod to become ready, will retry in next reconcile
	pending bool
	// When set, means that the operation is waiting for pod to finish, will retry in next reconcile
	running bool
	// List of errors encountered when reconciling one operation
	errors []error
}

// HasErrors Checks whether there were errors in processing this operation
// presence of errors indicates that the status information may not be accurate, demands a retry
func (e *rsyncClientOperationStatus) HasErrors() bool {
	return len(e.errors) > 0
}

func (e *rsyncClientOperationStatus) AddError(err error) {
	if e.errors == nil {
		e.errors = make([]error, 0)
	}
	e.errors = append(e.errors, err)
}

// rsyncClientOperationStatusList managed list of all ongoing Rsync operations
type rsyncClientOperationStatusList struct {
	// ops list of operations
	ops []rsyncClientOperationStatus
}

func (r *rsyncClientOperationStatusList) Add(s rsyncClientOperationStatus) {
	if r.ops == nil {
		r.ops = make([]rsyncClientOperationStatus, 0)
	}
	r.ops = append(r.ops, s)
}

// AllCompleted checks whether all of the Rsync attempts are in a terminal state
// If true, reconcile can move to next phase
func (r *rsyncClientOperationStatusList) AllCompleted() bool {
	for _, attempt := range r.ops {
		if attempt.pending || attempt.running || attempt.HasErrors() {
			return false
		}
	}
	return true
}

// AnyErrored checks whether any of the operation is resulting in an error
func (r *rsyncClientOperationStatusList) AnyErrored() bool {
	for _, attempt := range r.ops {
		if attempt.HasErrors() {
			return true
		}
	}
	return false
}

// Failed returns number of failed operations
func (r *rsyncClientOperationStatusList) Failed() int {
	i := 0
	for _, attempt := range r.ops {
		if attempt.failed {
			i += 1
		}
	}
	return i
}

// Succeeded returns number of failed operations
func (r *rsyncClientOperationStatusList) Succeeded() int {
	i := 0
	for _, attempt := range r.ops {
		if attempt.succeeded {
			i += 1
		}
	}
	return i
}

// Pending returns number of pending operations
func (r *rsyncClientOperationStatusList) Pending() int {
	i := 0
	for _, attempt := range r.ops {
		if attempt.pending {
			i += 1
		}
	}
	return i
}

// Running returns number of running operations
func (r *rsyncClientOperationStatusList) Running() int {
	i := 0
	for _, attempt := range r.ops {
		if attempt.pending {
			i += 1
		}
	}
	return i
}

// ensureRsyncOperations orchestrates all attempts of Rsync, updates owner status with observed state of all ongoing Rsync operations
// returns structured status of all operations, the return value should be used to make decisions about whether to retry in next reconcile
func (t *Task) ensureRsyncOperations(client compat.Client, podRequirements []rsyncClientPodRequirements) (rsyncClientOperationStatusList, []error) {
	statusList := rsyncClientOperationStatusList{}
	// rateLimiter defines maximum concurrent operations that can be reconciled in one go
	operationsRateLimiter, garbageCollectionRateLimiter := make(chan bool, DefaultRsyncOperationConcurrency), make(chan bool, 2)
	// outputChan streamlines output of concurrent reconcile operations
	operationOutputChan := make(chan rsyncClientOperationStatus, DefaultRsyncOperationConcurrency)
	garbageCollectionOutputChan := make(chan []error, 2)
	finishOperationChan, finishGarbageCollectionChan := make(chan bool), make(chan bool)
	garbageCollectionErrors := []error{}
	waitGroup := &sync.WaitGroup{}
	mutex := &sync.Mutex{}
	for i := range podRequirements {
		req := &podRequirements[i]
		lastObservedOperationStatus := t.Owner.Status.GetRsyncOperationStatusForPVC(&corev1.ObjectReference{
			Name:      req.pvInfo.name,
			Namespace: req.namespace,
		})
		// if the Rsync operation is already completed, do nothing
		if lastObservedOperationStatus.IsComplete() {
			statusList.Add(rsyncClientOperationStatus{
				failed:    lastObservedOperationStatus.Failed,
				succeeded: lastObservedOperationStatus.Succeeded,
				operation: lastObservedOperationStatus,
			})
			continue
		}
		// from this point onwards, do not mutate the original reference, create a copy and use it
		threadSafeOperationStatus := *lastObservedOperationStatus.DeepCopy()
		t.garbageCollectPodsForRequirements(
			client,
			threadSafeOperationStatus,
			garbageCollectionRateLimiter,
			garbageCollectionOutputChan,
			waitGroup)
		t.reconcileRsyncOperationState(
			client,
			req,
			threadSafeOperationStatus,
			operationsRateLimiter,
			operationOutputChan,
			waitGroup)
	}
	// consume output from concurrent operations
	go func() {
		for incomingOutput := range operationOutputChan {
			mutex.Lock()
			statusList.Add(incomingOutput)
			t.Owner.Status.AddRsyncOperation(incomingOutput.operation)
			mutex.Unlock()
		}
		finishOperationChan <- true
	}()
	// consume output from garbage collector
	go func() {
		for incomingOutput := range garbageCollectionOutputChan {
			mutex.Lock()
			garbageCollectionErrors = append(garbageCollectionErrors, incomingOutput...)
			mutex.Unlock()
		}
		finishGarbageCollectionChan <- true
	}()
	waitGroup.Wait()
	close(operationsRateLimiter)
	close(garbageCollectionRateLimiter)
	close(operationOutputChan)
	close(garbageCollectionOutputChan)
	<-finishOperationChan
	<-finishGarbageCollectionChan
	return statusList, garbageCollectionErrors
}

// garbageCollectRsyncPods garbage collection routine
// will run in background, sends list of errors on a channel, logs deletion
func (t *Task) garbageCollectPodsForRequirements(client compat.Client, op migapi.RsyncOperation, rateLimiter chan bool, outputChan chan<- []error, wg *sync.WaitGroup) {
	wg.Add(1)
	go func() {
		defer wg.Done()
		rateLimiter <- true
		errors := []error{}
		podList, err := t.getAllPodsForOperation(client, op)
		if err != nil {
			errors = append(errors, liberr.Wrap(err))
			outputChan <- errors
			<-rateLimiter
			return
		}
		for i := range podList.Items {
			pod := podList.Items[i]
			shouldDelete := false
			if label, exists := pod.Labels[RsyncAttemptLabel]; exists {
				if podAttempt, err := strconv.Atoi(label); err == nil {
					// Delete the pod when it was created for a past attempt and DVMP is done gathering info from this pod
					// keep a safety window of 1 attempt to avoid race conditions
					if _, present := pod.Labels[migapi.DVMPDoneLabelKey]; present && podAttempt < op.CurrentAttempt {
						shouldDelete = true
					}
				} else {
					// Delete the pod when the attempt label cannot be parsed as int, we never consider this pod anyway
					shouldDelete = true
				}
			} else {
				// Delete the pod when the pod attempt label is missing, we never consider this pod anyway
				shouldDelete = true
			}
			if shouldDelete {
				// Delete the pod when the pod attempt label is missing, we never consider this pod anyway
				err := client.Delete(context.TODO(), &pod)
				if err != nil {
					t.Log.Error(err, "failed deleting garbage Rsync pods for operation", "pvc", op)
					errors = append(errors, liberr.Wrap(err))
				}
				t.Log.Info("garbage cleaned pod", "pod", path.Join(pod.Namespace, pod.Name))
			}
		}
		outputChan <- errors
		<-rateLimiter
	}()
}

// reconcileRsyncOperationState reconciles observed state of 1 Rsync operation
// takes relevant actions wherever required such as launching a new pod when previous one fails
func (t *Task) reconcileRsyncOperationState(client compat.Client, req *rsyncClientPodRequirements,
	operation migapi.RsyncOperation, rateLimiter chan bool, outputChan chan<- rsyncClientOperationStatus, wg *sync.WaitGroup) {
	wg.Add(1)
	go func() {
		defer wg.Done()
		rateLimiter <- true
		currentStatus := rsyncClientOperationStatus{
			operation: &operation,
		}
		t.Log.Info("Reconciling Rsync operation for PVC", "pvc", operation)
		pod, err := t.getLatestPodForOperation(client, operation)
		if err != nil {
			currentStatus.AddError(err)
			outputChan <- currentStatus
			<-rateLimiter
			return
		}
		// when pod exists, analyze its status
		// when pod doesn't exist, start fresh
		if pod != nil {
			operation.CurrentAttempt, _ = strconv.Atoi(pod.Labels[RsyncAttemptLabel])
			currentStatus.failed, currentStatus.succeeded, currentStatus.running, currentStatus.pending = t.analyzeRsyncPodStatus(pod)
			// when pod failed and backoff limit is not reached, create a new pod
			if currentStatus.failed && operation.CurrentAttempt < GetRsyncPodBackOffLimit(*t.Owner) {
				err := t.createNewPodForOperation(client, req, operation)
				if err != nil {
					currentStatus.AddError(err)
				}
				// increment current attempt
				operation.CurrentAttempt += 1
				// indicate that the operation is not yet completely failed, we will retry
				currentStatus.pending = true
				t.Log.Info("Previous attempt of Rsync failed, created a new Rsync Pod", "pvc", operation, "attempt", operation.CurrentAttempt)
			} else {
				operation.Failed = currentStatus.failed
				operation.Succeeded = currentStatus.succeeded
				if operation.IsComplete() {
					t.Log.Info(
						fmt.Sprintf("Rsync operation completed after %d attempts", operation.CurrentAttempt),
						"pvc", operation, "failed", operation.Failed, "succeded", operation.Succeeded)
				} else {
					t.Log.Info("Rsync operation is still running. Waiting for completion",
						"pod", path.Join(pod.Namespace, pod.Name),
						"pvc", operation,
					)
				}
			}
		} else {
			operation.CurrentAttempt = 0
			err := t.createNewPodForOperation(client, req, operation)
			if err != nil {
				currentStatus.AddError(err)
			} else {
				// increment current attempt
				operation.CurrentAttempt += 1
				// indicate that pod is being created in this round of reconcile, need to come back
				currentStatus.pending = true
			}
			t.Log.Info("Started a new Rsync operation", "pvc", operation, "attempt", operation.CurrentAttempt)
		}
		outputChan <- currentStatus
		<-rateLimiter
	}()
}

// getAllPodsForOperation returns all pods matching given Rsync operation
func (t *Task) getAllPodsForOperation(client compat.Client, operation migapi.RsyncOperation) (*corev1.PodList, error) {
	podList := corev1.PodList{}
	pvcNamespace, pvcName := operation.GetPVDetails()
	labels := GetRsyncPodSelector(pvcName)
	err := client.List(context.TODO(),
		k8sclient.InNamespace(pvcNamespace).MatchingLabels(labels), &podList)
	if err != nil {
		t.Log.Error(err,
			"failed to list all Rsync Pods for PVC", "pvc", operation)
		return nil, err
	}
	return &podList, nil
}

// getLatestPodForOperation given an RsyncOperation, returns latest pod for that operator
func (t *Task) getLatestPodForOperation(client compat.Client, operation migapi.RsyncOperation) (*corev1.Pod, error) {
	podList, err := t.getAllPodsForOperation(client, operation)
	if err != nil {
		return nil, liberr.Wrap(err)
	}
	// if no existing pods found, it probably means we need to start fresh
	if len(podList.Items) < 1 {
		return nil, nil
	}
	var mostRecentPod corev1.Pod
	for _, pod := range podList.Items {
		// if expected attempt label is not found on the pod or its value is not an integer,
		// there is no way to associate this pod with an Rsync attempt we made, we skip this pod
		if val, exists := pod.Labels[RsyncAttemptLabel]; !exists {
			continue
		} else if _, err := strconv.Atoi(val); err != nil {
			continue
		}
		if pod.CreationTimestamp.After(mostRecentPod.CreationTimestamp.Time) {
			mostRecentPod = pod
		}
	}
	return &mostRecentPod, nil
}

// createNewPodForOperation creates a new pod for given RsyncOperation
func (t *Task) createNewPodForOperation(client compat.Client, req *rsyncClientPodRequirements, operation migapi.RsyncOperation) error {
	podTemplate := req.getRsyncClientPodTemplate()
	nextAttempt := operation.CurrentAttempt + 1
	existingLabels := podTemplate.Labels
	attemptLabel := map[string]string{
		RsyncAttemptLabel: fmt.Sprintf("%d", nextAttempt)}
	podTemplate.Labels = Union(existingLabels, attemptLabel)
	if len(podTemplate.Spec.Containers) > 0 {
		t.Log.Info(
			"creating new Pod with Rsync command", "cmd", strings.Join(podTemplate.Spec.Containers[0].Command, " "))
	}
	err := client.Create(context.TODO(), podTemplate.DeepCopy())
	if k8serror.IsAlreadyExists(err) {
		t.Log.Info(
			"Rsync Pod for given attempt already exists", "pvc", operation, "attempt", nextAttempt)
	} else if err != nil {
		t.Log.Error(err,
			"failed creating a new Rsync Pod for pvc", "pvc", operation, "attempt", nextAttempt)
		return err
	}
	operation.CurrentAttempt = nextAttempt
	return nil
}

// analyzeRsyncPodStatus looks at Rsync Pod and determines whether the Rsync attempt was successful, failed or the pod is pending
// returns whether running, succeeded, failed, pending
func (t *Task) analyzeRsyncPodStatus(pod *corev1.Pod) (failed bool, succeeded bool, running bool, pending bool) {
	switch pod.Status.Phase {
	case corev1.PodFailed:
		failed = true
	case corev1.PodSucceeded:
		succeeded = true
	case corev1.PodRunning:
		running = true
	case corev1.PodPending, corev1.PodUnknown:
		pending = true
	}
	return
}

// GetRsyncPodSelector returns pod selector used to identify sibling Rsync pods
func GetRsyncPodSelector(pvcName string) map[string]string {
	selector := make(map[string]string, 1)
	selector[migapi.RsyncPodIdentityLabel] = pvcName
	return selector
}

func Union(m1 map[string]string, m2 map[string]string) map[string]string {
	m3 := make(map[string]string, len(m1)+len(m2))
	for k, v := range m1 {
		m3[k] = v
	}
	for k, v := range m2 {
		m3[k] = v
	}
	return m3
}

func getDNSSafeName(name string) (string, error) {
	// TODO: this will probably introduce some non-trivial memory consumption.
	//   investigate if we can make a thread safe global variable and use it.
	re, err := regexp.Compile(`(\.+|\%+|\/+)`)
	if err != nil {
		return "", err
	}
	if utf8.RuneCountInString(name) > 63 {
		return re.ReplaceAllString(name[:63], "-"), nil
	}
	return re.ReplaceAllString(name, "-"), nil
}

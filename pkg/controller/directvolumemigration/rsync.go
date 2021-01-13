package directvolumemigration

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/rsa"
	"fmt"
	random "math/rand"
	"regexp"
	"strings"
	"text/template"
	"time"

	migapi "github.com/konveyor/mig-controller/pkg/apis/migration/v1alpha1"
	"github.com/konveyor/mig-controller/pkg/compat"
	migsettings "github.com/konveyor/mig-controller/pkg/settings"
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
			pvcList = append(pvcList, pvc{Name: vol})
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
				Labels: map[string]string{
					"app": DirectVolumeMigrationRsyncTransfer,
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
				Name:      DirectVolumeMigrationRsyncCreds,
				Labels: map[string]string{
					"app": DirectVolumeMigrationRsyncTransfer,
				},
			},
			Data: map[string][]byte{
				"RSYNC_PASSWORD": []byte(password),
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
				Name:      DirectVolumeMigrationRsyncCreds,
				Labels: map[string]string{
					"app": DirectVolumeMigrationRsyncTransfer,
				},
			},
			Data: map[string][]byte{
				"credentials": []byte("root:" + password),
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
	dvmLabels := t.buildDVMLabels()
	dvmLabels["purpose"] = DirectVolumeMigrationRsync

	for ns, _ := range pvcMap {
		svc := corev1.Service{
			ObjectMeta: metav1.ObjectMeta{
				Name:      DirectVolumeMigrationRsyncTransferSvc,
				Namespace: ns,
				Labels: map[string]string{
					"app": DirectVolumeMigrationRsyncTransfer,
				},
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
		err = destClient.Create(context.TODO(), &svc)
		if k8serror.IsAlreadyExists(err) {
			t.Log.Info("Rsync transfer svc already exists on destination", "namespace", ns)
		} else if err != nil {
			return err
		}
		route := routev1.Route{
			ObjectMeta: metav1.ObjectMeta{
				Name:      DirectVolumeMigrationRsyncTransferRoute,
				Namespace: ns,
				Labels: map[string]string{
					"app": DirectVolumeMigrationRsyncTransfer,
				},
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
	transferImage, err := cluster.GetRsyncTransferImage(t.Client)
	if err != nil {
		return err
	}
	limits, requests, err := getPodResourceLists(t.Client, TRANSFER_POD_CPU_LIMIT, TRANSFER_POD_MEMORY_LIMIT, TRANSFER_POD_CPU_REQUEST, TRANSFER_POD_MEMORY_REQUEST)
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
							Privileged:             &trueBool,
							RunAsUser:              &runAsUser,
							ReadOnlyRootFilesystem: &trueBool,
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

func getPodResourceLists(client k8sclient.Client, cpu_limit string, memory_limit string, cpu_request string, memory_request string) (corev1.ResourceList, corev1.ResourceList, error) {
	podConfigMap := &corev1.ConfigMap{}
	err := client.Get(context.TODO(), types.NamespacedName{Name: "migration-controller", Namespace: migapi.OpenshiftMigrationNamespace}, podConfigMap)
	if err != nil {
		return nil, nil, err
	}
	limits := corev1.ResourceList{
		corev1.ResourceMemory: resource.MustParse("1Gi"),
		corev1.ResourceCPU:    resource.MustParse("1"),
	}
	if _, exists := podConfigMap.Data[cpu_limit]; exists {
		cpu := resource.MustParse(podConfigMap.Data[cpu_limit])
		limits[corev1.ResourceCPU] = cpu
	}
	if _, exists := podConfigMap.Data[memory_limit]; exists {
		memory := resource.MustParse(podConfigMap.Data[memory_limit])
		limits[corev1.ResourceMemory] = memory
	}
	requests := corev1.ResourceList{
		corev1.ResourceMemory: resource.MustParse("1Gi"),
		corev1.ResourceCPU:    resource.MustParse("400m"),
	}
	if _, exists := podConfigMap.Data[cpu_request]; exists {
		cpu := resource.MustParse(podConfigMap.Data[cpu_request])
		requests[corev1.ResourceCPU] = cpu
	}
	if _, exists := podConfigMap.Data[memory_request]; exists {
		memory := resource.MustParse(podConfigMap.Data[memory_request])
		requests[corev1.ResourceMemory] = memory
	}
	return limits, requests, nil
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

	key := types.NamespacedName{Name: DirectVolumeMigrationRsyncTransferRoute, Namespace: namespace}
	err = destClient.Get(context.TODO(), key, &route)
	if err != nil {
		return "", err
	}
	return route.Spec.Host, nil
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
			Labels: map[string]string{
				"app": DirectVolumeMigrationRsyncTransfer,
			},
		},
		StringData: map[string]string{
			corev1.BasicAuthPasswordKey: string(password),
		},
		Type: corev1.SecretTypeBasicAuth,
	}
	err := t.Client.Create(context.TODO(), &secret)
	if k8serror.IsAlreadyExists(err) {
		t.Log.Info("Secret already exists on host", "name", "directvolumemigration-rsync-pass", "namespace", migapi.OpenshiftMigrationNamespace)
	} else if err != nil {
		return "", err
	}
	return string(password), nil
}

func (t *Task) getRsyncPassword() (string, error) {
	rsyncSecret := corev1.Secret{}
	key := types.NamespacedName{Name: DirectVolumeMigrationRsyncPass, Namespace: migapi.OpenshiftMigrationNamespace}
	err := t.Client.Get(context.TODO(), key, &rsyncSecret)
	if k8serror.IsNotFound(err) {
		t.Log.Info("Secret is not found", "name", DirectVolumeMigrationRsyncPass, "namespace", migapi.OpenshiftMigrationNamespace)
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
	err := t.Client.Delete(context.TODO(), secret, k8sclient.PropagationPolicy(metav1.DeletePropagationBackground))
	if k8serror.IsNotFound(err) {
		t.Log.Info("Secret is not found", "name", DirectVolumeMigrationRsyncPass, "namespace", migapi.OpenshiftMigrationNamespace)
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
	rsyncOptions := migsettings.Settings.RsyncOpts
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
	fsGroup            *int64
	supplementalGroups []int64
	seLinuxOptions     *corev1.SELinuxOptions

	// TODO:
	// add capabilities for dvm controller to handle case the source
	// application pods is privileged with the following flags from
	// PodSecurityContext and Containers' SecurityContext
	// We need to
	// 1. go through the pod's volume.
	// 2. find the container where it is volume mounted.
	//    i. if the container's fields are non-nil, use that
	//    ii. if the container's fields are nil, use the pods fields.

	// RunAsUser    *int64
	// RunAsGroup   *int64
	// RunAsNonRoot *bool
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
		for _, claimName := range pvcs {
			for _, p := range podList.Items {
				if !isClaimUsedByPod(claimName, &p) {
					continue
				}
				pvcSecurityContextMap[ns] = append(pvcSecurityContextMap[ns], PVCWithSecurityContext{
					name:               claimName,
					fsGroup:            p.Spec.SecurityContext.FSGroup,
					supplementalGroups: p.Spec.SecurityContext.SupplementalGroups,
					seLinuxOptions:     p.Spec.SecurityContext.SELinuxOptions,
				})
				// get the first lucky pod's fsgroup to avoid selection problem.
				break
			}
			// pvc not used by any pod
			pvcSecurityContextMap[ns] = append(pvcSecurityContextMap[ns], PVCWithSecurityContext{
				name:               claimName,
				fsGroup:            nil,
				supplementalGroups: nil,
				seLinuxOptions:     nil,
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

// Create rsync client pods
func (t *Task) createRsyncClientPods() error {
	// Get client for destination
	srcClient, err := t.getSourceClient()
	if err != nil {
		return err
	}

	// Get transfer image for cluster
	cluster, err := t.Owner.GetSourceCluster(t.Client)
	if err != nil {
		return err
	}
	transferImage, err := cluster.GetRsyncTransferImage(t.Client)
	if err != nil {
		return err
	}

	pvcMap, err := t.getfsGroupMapForNamespace()
	if err != nil {
		return err
	}

	password, err := t.getRsyncPassword()
	if err != nil {
		return err
	}
	pvcNodeMap, err := t.getPVCNodeNameMap()
	if err != nil {
		return err
	}

	limits, requests, err := getPodResourceLists(t.Client, CLIENT_POD_CPU_LIMIT, CLIENT_POD_MEMORY_LIMIT, CLIENT_POD_CPU_REQUEST, CLIENT_POD_MEMORY_REQUEST)
	if err != nil {
		return err
	}

	for ns, vols := range pvcMap {
		// Get stunnel svc IP
		svc := corev1.Service{}
		key := types.NamespacedName{Name: DirectVolumeMigrationRsyncTransferSvc, Namespace: ns}
		err := srcClient.Get(context.TODO(), key, &svc)
		if err != nil {
			return err
		}
		ip := svc.Spec.ClusterIP

		trueBool := true
		runAsUser := int64(0)

		// Add PVC volume mounts
		for _, vol := range vols {
			volumes := []corev1.Volume{}
			volumeMounts := []corev1.VolumeMount{}
			containers := []corev1.Container{}
			volumeMounts = append(volumeMounts, corev1.VolumeMount{
				Name:      vol.name,
				MountPath: fmt.Sprintf("/mnt/%s/%s", ns, vol.name),
			})
			volumes = append(volumes, corev1.Volume{
				Name: vol.name,
				VolumeSource: corev1.VolumeSource{
					PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{
						ClaimName: vol.name,
					},
				},
			})
			rsyncCommand := []string{"rsync"}
			rsyncCommand = append(rsyncCommand, t.getRsyncOptions()...)
			rsyncCommand = append(rsyncCommand, fmt.Sprintf("/mnt/%s/%s/", ns, vol.name))
			rsyncCommand = append(rsyncCommand, fmt.Sprintf("rsync://root@%s/%s", ip, vol.name))
			t.Log.Info(fmt.Sprintf("Using Rsync command [%s]", strings.Join(rsyncCommand, " ")))
			containers = append(containers, corev1.Container{
				Name:  DirectVolumeMigrationRsyncClient,
				Image: transferImage,
				Env: []corev1.EnvVar{
					{
						Name:  "RSYNC_PASSWORD",
						Value: password,
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
					Privileged:             &trueBool,
					RunAsUser:              &runAsUser,
					ReadOnlyRootFilesystem: &trueBool,
				},
				Resources: corev1.ResourceRequirements{
					Limits:   limits,
					Requests: requests,
				},
			})
			//var fsGroup *int64
			//sgs := []int64{}
			//if vol.fsGroup != nil {
			//	fsGroup = vol.fsGroup
			//}
			//if vol.supplementalGroups != nil && len(vol.supplementalGroups) != 0 {
			//	sgs = vol.supplementalGroups
			//}
			clientPod := corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Name:      fmt.Sprintf("directvolumemigration-rsync-transfer-%s", vol.name),
					Namespace: ns,
					Labels: map[string]string{
						"app":                   DirectVolumeMigrationRsyncTransfer,
						"directvolumemigration": DirectVolumeMigrationRsyncClient,
					},
				},
				Spec: corev1.PodSpec{
					RestartPolicy: corev1.RestartPolicyNever,
					Volumes:       volumes,
					Containers:    containers,
					NodeName:      pvcNodeMap[ns+"/"+vol.name],
					SecurityContext: &corev1.PodSecurityContext{
						SupplementalGroups: vol.supplementalGroups,
						FSGroup:            vol.fsGroup,
						SELinuxOptions:     vol.seLinuxOptions,
					},
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
	pvcMap := t.getPVCNamespaceMap()
	for ns, vols := range pvcMap {
		for _, vol := range vols {
			dvmp := migapi.DirectVolumeMigrationProgress{
				ObjectMeta: metav1.ObjectMeta{
					Name:      fmt.Sprintf("directvolumemigration-rsync-transfer-%s", vol),
					Namespace: migapi.OpenshiftMigrationNamespace,
				},
				Spec: migapi.DirectVolumeMigrationProgressSpec{
					ClusterRef: t.Owner.Spec.SrcMigClusterRef,
					PodRef: &corev1.ObjectReference{
						Namespace: ns,
						Name:      fmt.Sprintf("directvolumemigration-rsync-transfer-%s", vol),
					},
				},
				Status: migapi.DirectVolumeMigrationProgressStatus{},
			}
			migapi.SetOwnerReference(t.Owner, t.Owner, &dvmp)
			err := t.Client.Create(context.TODO(), &dvmp)
			if k8serror.IsAlreadyExists(err) {
				t.Log.Info("Rsync client progress CR already exists on destination", "namespace", dvmp.Namespace, "name", dvmp.Name)
			} else if err != nil {
				return err
			}
			t.Log.Info("Rsync client progress CR created", "name", dvmp.Name, "namespace", dvmp.Namespace)
		}

	}
	return nil
}

func (t *Task) haveRsyncClientPodsCompletedOrFailed() (bool, bool, error) {
	t.Owner.Status.RunningPods = []*migapi.PodProgress{}
	t.Owner.Status.FailedPods = []*migapi.PodProgress{}
	t.Owner.Status.SuccessfulPods = []*migapi.PodProgress{}

	pvcMap := t.getPVCNamespaceMap()
	for ns, vols := range pvcMap {
		for _, vol := range vols {
			dvmp := migapi.DirectVolumeMigrationProgress{}
			err := t.Client.Get(context.TODO(), types.NamespacedName{
				Name:      fmt.Sprintf("directvolumemigration-rsync-transfer-%s", vol),
				Namespace: migapi.OpenshiftMigrationNamespace,
			}, &dvmp)
			if err != nil {
				// todo, need to start thinking about collecting this error and reporting other CR's progress
				return false, false, err
			}
			objRef := &corev1.ObjectReference{
				Namespace: ns,
				Name:      fmt.Sprintf("directvolumemigration-rsync-transfer-%s", vol),
			}
			switch {
			case dvmp.Status.PodPhase == corev1.PodRunning:
				t.Owner.Status.RunningPods = append(t.Owner.Status.RunningPods, &migapi.PodProgress{
					ObjectReference:             objRef,
					LastObservedProgressPercent: dvmp.Status.LastObservedProgressPercent,
					LastObservedTransferRate:    dvmp.Status.LastObservedTransferRate,
				})
			case dvmp.Status.PodPhase == corev1.PodFailed:
				t.Owner.Status.FailedPods = append(t.Owner.Status.FailedPods, &migapi.PodProgress{
					ObjectReference:             objRef,
					LastObservedProgressPercent: dvmp.Status.LastObservedProgressPercent,
					LastObservedTransferRate:    dvmp.Status.LastObservedTransferRate,
				})
			case dvmp.Status.PodPhase == corev1.PodSucceeded:
				t.Owner.Status.SuccessfulPods = append(t.Owner.Status.SuccessfulPods, &migapi.PodProgress{
					ObjectReference:             objRef,
					LastObservedProgressPercent: dvmp.Status.LastObservedProgressPercent,
					LastObservedTransferRate:    dvmp.Status.LastObservedTransferRate,
				})
			}
		}
	}

	isCompleted := len(t.Owner.Status.SuccessfulPods)+len(t.Owner.Status.FailedPods) == len(t.Owner.Spec.PersistentVolumeClaims)
	hasAnyFailed := len(t.Owner.Status.FailedPods) > 0

	return isCompleted, hasAnyFailed, nil
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

	err = t.deleteRsyncPassword()
	if err != nil {
		return err
	}

	if !t.Owner.Spec.DeleteProgressReportingCRs {
		return nil
	}

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
	err, deleted := t.areRsyncResourcesDeleted(srcClient)
	if err != nil {
		return err, false
	}
	if !deleted {
		return nil, false
	}
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
		if len(routeList.Items) > 0 || len(svcList.Items) > 0 || len(cmList.Items) > 0 || len(secretList.Items) > 0 || len(podList.Items) > 0 {
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
			err = client.Delete(context.TODO(), &pod, k8sclient.PropagationPolicy(metav1.DeletePropagationBackground))
			if err != nil && !k8serror.IsNotFound(err) {
				return err
			}
		}

		// Delete secrets
		for _, secret := range secretList.Items {
			err = client.Delete(context.TODO(), &secret, k8sclient.PropagationPolicy(metav1.DeletePropagationBackground))
			if err != nil && !k8serror.IsNotFound(err) {
				return err
			}
		}

		// Delete routes
		for _, route := range routeList.Items {
			err = client.Delete(context.TODO(), &route, k8sclient.PropagationPolicy(metav1.DeletePropagationBackground))
			if err != nil && !k8serror.IsNotFound(err) {
				return err
			}
		}

		// Delete svcs
		for _, svc := range svcList.Items {
			err = client.Delete(context.TODO(), &svc, k8sclient.PropagationPolicy(metav1.DeletePropagationBackground))
			if err != nil && !k8serror.IsNotFound(err) {
				return err
			}
		}

		// Delete configmaps
		for _, cm := range cmList.Items {
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
			err := client.Delete(context.TODO(), &migapi.DirectVolumeMigrationProgress{
				ObjectMeta: metav1.ObjectMeta{
					Name:      fmt.Sprintf("directvolumemigration-rsync-transfer-%s", vol),
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

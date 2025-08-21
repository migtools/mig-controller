package directvolumemigration

import (
	"context"
	"crypto/md5"
	"encoding/hex"
	"fmt"
	"math/rand"
	random "math/rand"
	"path"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/go-logr/logr"
	liberr "github.com/konveyor/controller/pkg/error"
	"github.com/konveyor/crane-lib/state_transfer/endpoint"
	routeendpoint "github.com/konveyor/crane-lib/state_transfer/endpoint/route"
	svcendpoint "github.com/konveyor/crane-lib/state_transfer/endpoint/service"
	cranemeta "github.com/konveyor/crane-lib/state_transfer/meta"
	transfer "github.com/konveyor/crane-lib/state_transfer/transfer"
	blockrsynctransfer "github.com/konveyor/crane-lib/state_transfer/transfer/blockrsync"
	rsynctransfer "github.com/konveyor/crane-lib/state_transfer/transfer/rsync"
	"github.com/konveyor/crane-lib/state_transfer/transport"
	stunneltransport "github.com/konveyor/crane-lib/state_transfer/transport/stunnel"
	migapi "github.com/konveyor/mig-controller/pkg/apis/migration/v1alpha1"
	"github.com/konveyor/mig-controller/pkg/compat"
	migevent "github.com/konveyor/mig-controller/pkg/event"
	"github.com/konveyor/mig-controller/pkg/settings"
	configv1 "github.com/openshift/api/config/v1"
	routev1 "github.com/openshift/api/route/v1"
	corev1 "k8s.io/api/core/v1"
	k8serror "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/selection"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/utils/set"
	virtv1 "kubevirt.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	k8sclient "sigs.k8s.io/controller-runtime/pkg/client"
)

const (
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
	// SuperPrivilegedContainerType is the selinux SPC type string
	SuperPrivilegedContainerType = "spc_t"
)

// labels
const (
	// RsyncAttemptLabel is used to associate an Rsync Pod with the attempts
	RsyncAttemptLabel = "migration.openshift.io/rsync-attempt"
)

// ensureRsyncEndpoints ensures that new Endpoints are created for Rsync and Blockrsync Transfers
func (t *Task) ensureRsyncEndpoints() error {
	dvmLabels := t.buildDVMLabels()
	dvmLabels["purpose"] = DirectVolumeMigrationRsync
	blockdvmLabels := t.buildDVMLabels()
	blockdvmLabels["app"] = DirectVolumeMigrationRsyncTransferBlock
	blockdvmLabels["purpose"] = DirectVolumeMigrationRsync

	hostnames := []string{}
	if t.EndpointType == migapi.NodePort {
		var err error
		hostnames, err = getWorkerNodeHostnames(t.destinationClient)
		if err != nil {
			return liberr.Wrap(err)
		}
	}

	for bothNs := range t.getPVCNamespaceMap() {
		ns := getDestNs(bothNs)

		var endpoint, blockEndpoint endpoint.Endpoint

		switch t.EndpointType {
		case migapi.ClusterIP, migapi.NodePort:
			endpoint = svcendpoint.NewEndpoint(
				types.NamespacedName{
					Namespace: ns,
					Name:      DirectVolumeMigrationRsyncTransferSvc,
				},
				dvmLabels,
				getNodeHostnameAtRandom(hostnames),
				t.getServiceType(),
			)
			blockEndpoint = svcendpoint.NewEndpoint(
				types.NamespacedName{
					Namespace: ns,
					Name:      DirectVolumeMigrationRsyncTransferSvcBlock,
				},
				blockdvmLabels,
				getNodeHostnameAtRandom(hostnames),
				t.getServiceType(),
			)
		default:
			// Get cluster subdomain if it exists
			cluster, err := t.Owner.GetDestinationCluster(t.Client)
			if err != nil {
				return err
			}

			// Get the user provided subdomain, if empty we'll attempt to
			// get the cluster's subdomain from destination client directly
			subdomain, err := cluster.GetClusterSubdomain(t.Client)
			if err != nil {
				t.Log.Info("failed to get cluster_subdomain, attempting to get cluster's ingress domain", "error", err)
				ingressConfig := &configv1.Ingress{}
				err = t.destinationClient.Get(context.TODO(), types.NamespacedName{Name: "cluster"}, ingressConfig)
				if err != nil {
					t.Log.Error(err, "failed to retrieve cluster's ingress domain, extremely long namespace names will cause route creation failure")
				} else {
					subdomain = ingressConfig.Spec.Domain
				}
			}

			endpoint = routeendpoint.NewEndpoint(
				types.NamespacedName{
					Namespace: ns,
					Name:      DirectVolumeMigrationRsyncTransferRoute,
				},
				routeendpoint.EndpointTypePassthrough,
				dvmLabels,
				subdomain,
			)
			blockEndpoint = routeendpoint.NewEndpoint(
				types.NamespacedName{
					Namespace: ns,
					Name:      DirectVolumeMigrationRsyncTransferRouteBlock,
				},
				routeendpoint.EndpointTypePassthrough,
				blockdvmLabels,
				subdomain,
			)
		}

		if err := endpoint.Create(t.destinationClient); err != nil {
			return liberr.Wrap(err)
		}
		if err := blockEndpoint.Create(t.destinationClient); err != nil {
			return liberr.Wrap(err)
		}
	}
	return nil
}

// getFSRsyncTransferOptions returns Rsync transfer options
func (t *Task) getFSRsyncTransferOptions() ([]rsynctransfer.TransferOption, error) {
	// prepare rsync command options
	o := settings.Settings.DvmOpts.RsyncOpts
	rsyncPassword, err := t.getRsyncPassword()
	if err != nil {
		return nil, liberr.Wrap(err)
	}
	transferOptions := []rsynctransfer.TransferOption{
		rsynctransfer.StandardProgress(true),
		rsynctransfer.ArchiveFiles(o.Archive),
		rsynctransfer.DeleteDestination(o.Delete),
		HardLinks(o.HardLinks),
		Partial(o.Partial),
		ExtraOpts(o.Extras),
		rsynctransfer.Username("root"),
		rsynctransfer.Password(rsyncPassword),
		rsynctransfer.ExcludeFiles{
			"lost+found",
		},
	}
	srcCluster, err := t.Owner.GetSourceCluster(t.Client)
	if err != nil {
		return nil, liberr.Wrap(err)
	}
	if srcCluster != nil {
		srcTransferImage, err := srcCluster.GetRsyncTransferImage(t.Client)
		if err != nil {
			return nil, liberr.Wrap(err)
		}
		transferOptions = append(transferOptions,
			rsynctransfer.RsyncClientImage(srcTransferImage))
	}
	destCluster, err := t.Owner.GetDestinationCluster(t.Client)
	if err != nil {
		return nil, liberr.Wrap(err)
	}
	if destCluster != nil {
		destTransferImage, err := destCluster.GetRsyncTransferImage(t.Client)
		if err != nil {
			return nil, liberr.Wrap(err)
		}
		transferOptions = append(transferOptions,
			rsynctransfer.RsyncServerImage(destTransferImage))
	}
	if o.BwLimit > 0 {
		transferOptions = append(transferOptions,
			RsyncBwLimit(o.BwLimit))
	}
	return transferOptions, nil
}

// getBlockRsyncTransferOptions returns Rsync transfer options
func (t *Task) getBlockRsyncTransferOptions() (*blockrsynctransfer.TransferOptions, error) {
	// prepare rsync command options
	transferOptions := &blockrsynctransfer.TransferOptions{}
	srcCluster, err := t.Owner.GetSourceCluster(t.Client)
	if err != nil {
		return nil, liberr.Wrap(err)
	}
	if srcCluster != nil {
		srcTransferImage, err := srcCluster.GetRsyncTransferImage(t.Client)
		if err != nil {
			return nil, liberr.Wrap(err)
		}
		blockrsynctransfer.RsyncClientImage(srcTransferImage).ApplyTo(transferOptions)
	}
	destCluster, err := t.Owner.GetDestinationCluster(t.Client)
	if err != nil {
		return nil, liberr.Wrap(err)
	}
	if destCluster != nil {
		destTransferImage, err := destCluster.GetRsyncTransferImage(t.Client)
		if err != nil {
			return nil, liberr.Wrap(err)
		}
		blockrsynctransfer.RsyncServerImage(destTransferImage).ApplyTo(transferOptions)
	}
	return transferOptions, nil
}

// getRsyncClientMutations get Rsync container mutations for source Rsync Pod
func (t *Task) getRsyncClientMutations(srcClient compat.Client, destClient compat.Client, namespace string) ([]rsynctransfer.TransferOption, error) {
	transferOptions := []rsynctransfer.TransferOption{}
	containerMutation := &corev1.Container{}

	migration, err := t.Owner.GetMigrationForDVM(t.Client)
	if err != nil {
		return nil, liberr.Wrap(err)
	}
	if migration == nil {
		return transferOptions, nil
	}

	containerMutation.SecurityContext, err = t.getSecurityContext(srcClient, namespace, migration)
	if err != nil {
		return nil, liberr.Wrap(err)
	}
	resourceRequirements, err := t.getRsyncClientResourceRequirements(namespace, srcClient)
	if err != nil {
		return nil, liberr.Wrap(err)
	}
	containerMutation.Resources = resourceRequirements
	transferOptions = append(transferOptions,
		rsynctransfer.SourceContainerMutation{
			C: containerMutation,
		})
	return transferOptions, nil
}

// getRsyncTransferServerMutations get Rsync container & pod mutations for target rsync pod
func (t *Task) getRsyncTransferServerMutations(client compat.Client, namespace string) ([]rsynctransfer.TransferOption, error) {
	transferOptions := []rsynctransfer.TransferOption{}
	containerMutation := &corev1.Container{}

	resourceRequirements, err := t.getRsyncServerResourceRequirements(namespace, client)
	if err != nil {
		return nil, liberr.Wrap(err)
	}
	containerMutation.Resources = resourceRequirements
	migration, err := t.Owner.GetMigrationForDVM(t.Client)
	if err != nil {
		return nil, liberr.Wrap(err)
	}
	if migration == nil {
		return transferOptions, nil
	}

	containerMutation.SecurityContext, err = t.getSecurityContext(client, namespace, migration)
	if err != nil {
		return nil, liberr.Wrap(err)
	}
	transferOptions = append(transferOptions,
		rsynctransfer.DestinationContainerMutation{
			C: containerMutation,
		})
	// add supplemental groups for the rsync transfer server pod
	podSecurityContext := &rsynctransfer.DestinationPodSpecMutation{}
	if len(settings.Settings.DvmOpts.DestinationSupplementalGroups) > 0 {
		podSecurityContext.Spec = &corev1.PodSpec{
			SecurityContext: &corev1.PodSecurityContext{
				SupplementalGroups: settings.Settings.DvmOpts.DestinationSupplementalGroups,
			},
		}
	}
	transferOptions = append(transferOptions, podSecurityContext)
	return transferOptions, nil
}

// getSecurityContext returns the appropriate pod security context based on user input on migmigration
func (t *Task) getSecurityContext(client compat.Client, namespace string, migration *migapi.MigMigration) (*corev1.SecurityContext, error) {
	securityContext := &corev1.SecurityContext{}
	var selinuxOptions *corev1.SELinuxOptions

	// check if user explicitely asked to run Rsync Pods as root
	isPrivileged, err := isRsyncPrivileged(client)
	if err != nil {
		return securityContext, liberr.Wrap(err)
	}

	isSuperPrivileged, err := isRsyncSuperPrivileged(client)
	if err != nil {
		return securityContext, liberr.Wrap(err)
	}

	trueBool := true
	falseBool := false
	rootUser := int64(0)

	if isSuperPrivileged {
		isPrivileged = trueBool
		selinuxOptions = &corev1.SELinuxOptions{
			Type: SuperPrivilegedContainerType,
		}
	}

	if migration.Spec.RunAsRoot == nil {
		migration.Spec.RunAsRoot = &isPrivileged
	}

	// check if the cluster has restricted PSA enforced
	// OpenShift 4.12+ versions have it enforced by default
	psaEnforced := isPSAEnforced(client)

	// if PSA is not enforced, we can safely run Rsync Pod using the rsync-anyuid SCC
	// if PSA is enforced, we run Rsync Pod with least privileges by default
	if !psaEnforced {
		securityContext = &corev1.SecurityContext{
			Privileged:             &isPrivileged,
			ReadOnlyRootFilesystem: &trueBool,
			RunAsUser:              &rootUser,
			SELinuxOptions:         selinuxOptions,
			Capabilities: &corev1.Capabilities{
				Drop: []corev1.Capability{"MKNOD", "SETPCAP"},
			},
		}
	} else {
		privilegedLabelPresent := false
		if *migration.Spec.RunAsRoot {
			// if PSA is enforced, and if the user asked to run Rsync Pod as root explicitely
			// we check if the namespace has required exception labels set
			privilegedLabelPresent, err = isPrivilegedLabelPresent(client, namespace)
			if err != nil {
				return securityContext, liberr.Wrap(err)
			}
			if !privilegedLabelPresent {
				// warning in DVM since user wants to run rsync as root but the namespace is missing needed labels, so we are running rsync as non root
				rsyncMessage := "missing required labels on namespace to run rsync as root, running as non root"
				t.Log.Info(rsyncMessage)
				t.Owner.Status.SetCondition(migapi.Condition{
					Type:     RsyncServerPodsRunningAsNonRoot,
					Status:   migapi.True,
					Reason:   "RsyncOperationsAreRunningAsNonRoot",
					Category: Warn,
					Message:  rsyncMessage,
				})
			}
		}
		if privilegedLabelPresent {
			securityContext = &corev1.SecurityContext{
				Privileged:             &isPrivileged,
				ReadOnlyRootFilesystem: &trueBool,
				RunAsUser:              &rootUser,
				SELinuxOptions:         selinuxOptions,
				Capabilities: &corev1.Capabilities{
					Drop: []corev1.Capability{"MKNOD", "SETPCAP"},
				},
			}
		} else {
			securityContext = &corev1.SecurityContext{
				RunAsUser:    migration.Spec.RunAsUser,
				RunAsGroup:   migration.Spec.RunAsGroup,
				RunAsNonRoot: &trueBool,
				Capabilities: &corev1.Capabilities{
					Drop: []corev1.Capability{"ALL"},
				},
				AllowPrivilegeEscalation: &falseBool,
				SeccompProfile: &corev1.SeccompProfile{
					Type: "RuntimeDefault",
				},
			}
		}
	}
	return securityContext, nil
}

// ensureRsyncTransferServer ensures that server component of the Transfer is created
func (t *Task) ensureRsyncTransferServer() error {
	nsMap, err := t.getNamespacedPVCPairs()
	if err != nil {
		return liberr.Wrap(err)
	}

	err = t.buildDestinationLimitRangeMap(nsMap, t.destinationClient)
	if err != nil {
		return liberr.Wrap(err)
	}

	transportOptions, err := t.getStunnelOptions()
	if err != nil {
		return liberr.Wrap(err)
	}

	if err := t.ensureFilesystemRsyncTransferServer(nsMap, transportOptions); err != nil {
		return err
	}
	if err := t.ensureBlockRsyncTransferServer(nsMap, transportOptions); err != nil {
		return err
	}
	return nil
}

func (t *Task) ensureFilesystemRsyncTransferServer(nsMap map[string][]transfer.PVCPair, transportOptions *transport.Options) error {
	for bothNs, pvcPairs := range nsMap {
		srcNs := getSourceNs(bothNs)
		destNs := getDestNs(bothNs)
		nnPair := cranemeta.NewNamespacedPair(
			types.NamespacedName{Name: DirectVolumeMigrationRsyncTransfer, Namespace: srcNs},
			types.NamespacedName{Name: DirectVolumeMigrationRsyncTransfer, Namespace: destNs},
		)
		endpoints, err := t.getEndpoints(t.destinationClient, destNs)
		if err != nil {
			return liberr.Wrap(err)
		}
		stunnelTransport, err := stunneltransport.GetTransportFromKubeObjects(
			t.sourceClient, t.destinationClient, "fs", nnPair, endpoints[0], transportOptions)
		if err != nil {
			return liberr.Wrap(err)
		}
		filesystemPvcList, err := transfer.NewFilesystemPVCPairList(pvcPairs...)
		if err != nil {
			return liberr.Wrap(err)
		}
		if len(filesystemPvcList) > 0 {
			labels := t.buildDVMLabels()
			labels["purpose"] = DirectVolumeMigrationRsync
			rsyncOptions, err := t.getFSRsyncTransferOptions()
			if err != nil {
				return liberr.Wrap(err)
			}
			mutations, err := t.getRsyncTransferServerMutations(t.destinationClient, destNs)
			if err != nil {
				return liberr.Wrap(err)
			}
			rsyncOptions = append(rsyncOptions, mutations...)
			rsyncOptions = append(rsyncOptions, rsynctransfer.WithDestinationPodLabels(labels))
			transfer, err := rsynctransfer.NewTransfer(
				stunnelTransport, endpoints[0], t.sourceClient, t.destinationClient, filesystemPvcList, t.Log, rsyncOptions...)
			if err != nil {
				return liberr.Wrap(err)
			}
			if transfer == nil {
				return fmt.Errorf("transfer %s/%s not found", nnPair.Source().Namespace, nnPair.Source().Name)
			}
			err = transfer.CreateServer(t.destinationClient)
			if err != nil {
				return liberr.Wrap(err)
			}
		}
	}
	return nil
}

func (t *Task) ensureBlockRsyncTransferServer(nsMap map[string][]transfer.PVCPair, transportOptions *transport.Options) error {
	for bothNs, pvcPairs := range nsMap {
		srcNs := getSourceNs(bothNs)
		destNs := getDestNs(bothNs)
		nnPair := cranemeta.NewNamespacedPair(
			types.NamespacedName{Name: DirectVolumeMigrationRsyncTransfer, Namespace: srcNs},
			types.NamespacedName{Name: DirectVolumeMigrationRsyncTransfer, Namespace: destNs},
		)
		endpoints, err := t.getEndpoints(t.destinationClient, destNs)
		if err != nil {
			return liberr.Wrap(err)
		}
		stunnelTransports, err := stunneltransport.GetTransportFromKubeObjects(
			t.sourceClient, t.destinationClient, "block", nnPair, endpoints[1], transportOptions)
		if err != nil {
			return liberr.Wrap(err)
		}

		blockOrVMPvcList, err := transfer.NewBlockOrVMDiskPVCPairList(pvcPairs...)
		if err != nil {
			return liberr.Wrap(err)
		}
		blockOrVMPvcList = t.filterCompletedPairs(blockOrVMPvcList)
		if t.PlanResources.MigPlan.LiveMigrationChecked() {
			blockOrVMPvcList, err = t.filterRunningVMs(blockOrVMPvcList)
			if err != nil {
				return liberr.Wrap(err)
			}
		}
		if len(blockOrVMPvcList) > 0 {
			labels := t.buildDVMLabels()
			labels["app"] = DirectVolumeMigrationRsyncTransferBlock
			labels["purpose"] = DirectVolumeMigrationRsync
			transferOptions, err := t.getBlockRsyncTransferOptions()
			if err != nil {
				return liberr.Wrap(err)
			}
			transferOptions.SourcePodMeta = transfer.ResourceMetadata{
				Labels: labels,
			}

			transfer, err := blockrsynctransfer.NewTransfer(
				stunnelTransports, endpoints[1], t.sourceClient, t.destinationClient, blockOrVMPvcList, t.Log, transferOptions)
			if err != nil {
				return liberr.Wrap(err)
			}
			if transfer == nil {
				return fmt.Errorf("transfer %s/%s not found", nnPair.Source().Namespace, nnPair.Source().Name)
			}
			err = transfer.CreateServer(t.destinationClient)
			if err != nil && !k8serror.IsAlreadyExists(err) {
				return liberr.Wrap(err)
			}
		}
	}
	return nil
}

// Return only PVCPairs that are NOT completed yet
func (t *Task) filterCompletedPairs(unfilteredPVCPairs []transfer.PVCPair) []transfer.PVCPair {
	runningVMPairs := []transfer.PVCPair{}
	successfulSet := set.New[string]()

	for _, successfulPod := range t.Owner.Status.SuccessfulPods {
		successfulSet.Insert(fmt.Sprintf("%s/%s", successfulPod.PVCReference.Namespace, successfulPod.PVCReference.Name))
	}
	successfulList := successfulSet.UnsortedList()
	for _, pvcPair := range unfilteredPVCPairs {
		if !slices.Contains(successfulList, fmt.Sprintf("%s/%s", pvcPair.Source().Claim().Namespace, pvcPair.Source().Claim().Name)) {
			runningVMPairs = append(runningVMPairs, pvcPair)
		}
	}
	return runningVMPairs
}

// Return only PVCPairs that are NOT associated with a running VM
func (t *Task) filterRunningVMs(unfilteredPVCPairs []transfer.PVCPair) ([]transfer.PVCPair, error) {
	runningVMPairs := []transfer.PVCPair{}
	ns := set.New[string]()
	for _, pvcPair := range unfilteredPVCPairs {
		ns.Insert(pvcPair.Source().Claim().Namespace)
	}
	nsVolumes, err := t.getRunningVMVolumes(ns.SortedList())
	if err != nil {
		return nil, err
	}

	for _, pvcPair := range unfilteredPVCPairs {
		if !slices.Contains(nsVolumes, fmt.Sprintf("%s/%s", pvcPair.Source().Claim().Namespace, pvcPair.Source().Claim().Name)) {
			runningVMPairs = append(runningVMPairs, pvcPair)
		}
	}
	return runningVMPairs, nil
}

func (t *Task) createRsyncTransferClients(srcClient compat.Client,
	destClient compat.Client, nsMap map[string][]transfer.PVCPair) (*migrationOperationStatusList, error) {
	statusList := &migrationOperationStatusList{}

	pvcNodeMap, err := t.getPVCNodeNameMap(srcClient)
	if err != nil {
		return statusList, liberr.Wrap(err)
	}

	secInfo, err := t.getSourceSecurityGroupInfo(srcClient, nsMap)
	if err != nil {
		return statusList, liberr.Wrap(err)
	}

	rsyncOptions, err := t.getFSRsyncTransferOptions()
	if err != nil {
		return statusList, liberr.Wrap(err)
	}

	transportOptions, err := t.getStunnelOptions()
	if err != nil {
		return statusList, liberr.Wrap(err)
	}

	migration, err := t.Owner.GetMigrationForDVM(t.Client)
	if err != nil {
		return nil, liberr.Wrap(err)
	}

	checkLabels := isPSAEnforced(destClient)

	for bothNs, pvcPairs := range nsMap {
		srcNs := getSourceNs(bothNs)
		destNs := getDestNs(bothNs)
		mutations, err := t.getRsyncClientMutations(srcClient, destClient, srcNs)
		if err != nil {
			return statusList, liberr.Wrap(err)
		}
		rsyncOptions = append(rsyncOptions, mutations...)
		nnPair := cranemeta.NewNamespacedPair(
			types.NamespacedName{Name: DirectVolumeMigrationRsyncClient, Namespace: srcNs},
			types.NamespacedName{Name: DirectVolumeMigrationRsyncClient, Namespace: destNs},
		)
		endpoints, err := t.getEndpoints(destClient, destNs)
		if err != nil {
			return statusList, liberr.Wrap(err)
		}
		fsStunnelTransport, err := stunneltransport.GetTransportFromKubeObjects(
			srcClient, destClient, "fs", nnPair, endpoints[0], transportOptions)
		if err != nil {
			return statusList, liberr.Wrap(err)
		}
		blockStunnelTransport, err := stunneltransport.GetTransportFromKubeObjects(
			srcClient, destClient, "block", nnPair, endpoints[1], transportOptions)
		if err != nil {
			return statusList, liberr.Wrap(err)
		}

		labels := t.buildDVMLabels()

		privilegedLabelPresent := true
		if checkLabels {
			privilegedLabelPresent, err = isPrivilegedLabelPresent(destClient, destNs)
			if err != nil {
				return nil, liberr.Wrap(err)
			}
		}

		isPrivileged, err := isRsyncPrivileged(destClient)
		if migration.Spec.RunAsRoot == nil {
			migration.Spec.RunAsRoot = &isPrivileged
		}
		if err != nil {
			return nil, liberr.Wrap(err)
		}
		if err := t.deleteOfflineMigrationServerPod(destNs, pvcPairs); err != nil {
			return nil, liberr.Wrap(err)
		}

		for _, pvc := range pvcPairs {
			optionsForPvc := []rsynctransfer.TransferOption{}
			// ensure that the Rsync operation for this PVC is not already complete
			lastObservedOperationStatus := t.Owner.Status.GetRsyncOperationStatusForPVC(
				&corev1.ObjectReference{
					Name:      pvc.Source().Claim().Name,
					Namespace: pvc.Source().Claim().Namespace,
				},
			)
			if lastObservedOperationStatus.IsComplete() {
				statusList.Add(
					migrationOperationStatus{
						failed:    lastObservedOperationStatus.Failed,
						succeeded: lastObservedOperationStatus.Succeeded,
						operation: lastObservedOperationStatus,
					},
				)
				continue
			}

			newOperation := lastObservedOperationStatus
			currentStatus := migrationOperationStatus{
				operation: newOperation,
			}
			pod, err := t.getLatestPodForOperation(srcClient, *lastObservedOperationStatus)
			if err != nil {
				t.Log.Error(err, "failed getting latest rsync client pod", "pvc", newOperation)
				currentStatus.AddError(err)
				statusList.Add(currentStatus)
				continue
			}

			filesystemPVCList, err := transfer.NewFilesystemPVCPairList(pvc)
			if err != nil {
				t.Log.Error(err, "failed creating PVC pair", "pvc", newOperation)
				currentStatus.AddError(err)
				statusList.Add(currentStatus)
				continue
			}
			uncompletedBlockOrVMPvcList, err := transfer.NewBlockOrVMDiskPVCPairList(pvc)
			if err != nil {
				t.Log.Error(err, "failed creating block PVC pair", "pvc", newOperation)
				currentStatus.AddError(err)
				statusList.Add(currentStatus)
				continue
			}
			uncompletedBlockOrVMPvcList = t.filterCompletedPairs(uncompletedBlockOrVMPvcList)
			unfilteredUncompletedBlockOrVMPvcList := transfer.PVCPairList{}
			unfilteredUncompletedBlockOrVMPvcList = append(unfilteredUncompletedBlockOrVMPvcList, uncompletedBlockOrVMPvcList...)

			if t.PlanResources.MigPlan.LiveMigrationChecked() {
				uncompletedBlockOrVMPvcList, err = t.filterRunningVMs(uncompletedBlockOrVMPvcList)
				if err != nil {
					t.Log.Error(err, "failed filtering block PVC pairs", "pvc", newOperation)
					currentStatus.AddError(err)
					statusList.Add(currentStatus)
					continue
				}
			}

			// Force schedule Rsync Pod on the application node
			nodeName := pvcNodeMap[fmt.Sprintf("%s/%s", srcNs, pvc.Source().Claim().Name)]
			clientPodMutation := rsynctransfer.SourcePodSpecMutation{
				Spec: &corev1.PodSpec{
					NodeName: nodeName,
				},
			}
			if len(settings.Settings.DvmOpts.SourceSupplementalGroups) > 0 {
				clientPodMutation.Spec.SecurityContext = &corev1.PodSecurityContext{
					SupplementalGroups: settings.Settings.DvmOpts.SourceSupplementalGroups,
				}
			}
			optionsForPvc = append(optionsForPvc, &clientPodMutation)
			if info, exists := secInfo.Get(
				pvc.Source().Claim().Name, pvc.Source().Claim().Namespace); exists {
				if info.verify {
					optionsForPvc = append(optionsForPvc, ExtraOpts{"--checksum"})
				}
			}

			val, exists := t.SparseFileMap[fmt.Sprintf("%s/%s", pvc.Source().Claim().Namespace, pvc.Source().Claim().Name)]
			if exists && val {
				sparseFileOption := ExtraOpts{
					"--sparse",
					"--no-inplace",
				}
				optionsForPvc = append(optionsForPvc, sparseFileOption)
			}

			if !*migration.Spec.RunAsRoot || !privilegedLabelPresent {
				optionsForPvc = append(optionsForPvc, ExtraOpts{"--omit-dir-times"})
			}

			// Add identification label for Rsync Pod that keep them associated with a pvc
			labels[migapi.RsyncPodIdentityLabel] = pvc.Source().LabelSafeName()

			if pod != nil {
				newOperation.CurrentAttempt, _ = strconv.Atoi(pod.Labels[RsyncAttemptLabel])
				updateOperationStatus(&currentStatus, pod)
				if currentStatus.failed && currentStatus.operation.CurrentAttempt < GetRsyncPodBackOffLimit(t.Owner) {
					// since we have not yet attempted all retries,
					// reset the failed status and set the pending status
					currentStatus.failed = false
					currentStatus.pending = true
					labels[RsyncAttemptLabel] = fmt.Sprintf("%d", currentStatus.operation.CurrentAttempt+1)
					optionsForPvc = append(optionsForPvc, rsynctransfer.WithSourcePodLabels(labels))
					if len(filesystemPVCList) > 0 {
						optionsForPvc = append(optionsForPvc, rsynctransfer.WithSourcePodLabels(labels))
						transfer, err := rsynctransfer.NewTransfer(
							fsStunnelTransport, endpoints[0], srcClient, destClient, filesystemPVCList, t.Log, append(rsyncOptions, optionsForPvc...)...)
						if err != nil {
							t.Log.Error(err, "failed creating filesystem transfer", "pvc", currentStatus.operation)
							currentStatus.AddError(err)
							continue
						}
						if currentStatus := t.createClientPodForTransfer(nnPair.Source().Name,
							nnPair.Source().Namespace, transfer, &currentStatus); currentStatus != nil {
							statusList.Add(*currentStatus)
							continue
						}
					}
					if len(uncompletedBlockOrVMPvcList) > 0 {
						transferOptions := blockrsynctransfer.TransferOptions{
							SourcePodMeta: transfer.ResourceMetadata{
								Labels: labels,
							},
							NodeName: nodeName,
						}
						transferOptions.SourcePodMeta.Labels["app"] = DirectVolumeMigrationRsyncTransferBlock
						transfer, err := blockrsynctransfer.NewTransfer(
							blockStunnelTransport, endpoints[1], srcClient, destClient, uncompletedBlockOrVMPvcList, t.Log, &transferOptions)
						if err != nil {
							t.Log.Error(err, "failed creating block transfer", "pvc", currentStatus.operation)
							currentStatus.AddError(err)
							continue
						}

						if currentStatus := t.createClientPodForTransfer(nnPair.Source().Name,
							nnPair.Source().Namespace, transfer, &currentStatus); currentStatus != nil {
							statusList.Add(*currentStatus)
							continue
						}
					}

					t.Log.Info("previous attempt of Rsync failed for pvc, created a new pod", "pvc", newOperation)
					err = srcClient.Delete(context.TODO(), pod)
					if err != nil {
						t.Log.Error(err, "failed deleting rsync pod of previous attempt for pvc", "pvc", newOperation)
						currentStatus.AddError(err)
						statusList.Add(currentStatus)
						continue
					}
				} else {
					// Check if PVC is now part of the running VMs PVs, if so we need to cancel the in progress rsync, and
					// that should trigger a live migration or a new rsync operation after it stops the VM.
					for _, pvcPair := range unfilteredUncompletedBlockOrVMPvcList {
						if t.Owner.IsCutover() && pvcPair.Source().Claim().Name == newOperation.PVCReference.Name && pvcPair.Source().Claim().Namespace == newOperation.PVCReference.Namespace {
							runningVMVolumes, err := t.getRunningVMVolumes([]string{newOperation.PVCReference.Namespace})
							if err != nil {
								currentStatus.AddError(err)
								continue
							}
							for _, runningVMVolume := range runningVMVolumes {
								if runningVMVolume == fmt.Sprintf("%s/%s", newOperation.PVCReference.Namespace, newOperation.PVCReference.Name) {
									t.Log.Info("Cancelling rsync operation for pvc", "pvc", newOperation.PVCReference.Name, "namespace", newOperation.PVCReference.Namespace, "label safe name", pvc.Source().LabelSafeName())
									if err := t.cancelOfflineMigration(pvcPair.Source(), newOperation.PVCReference.Namespace); err != nil {
										currentStatus.AddError(err)
									}
								}
							}
							continue
						}
					}
					t.Log.Info("previous attempt of Rsync did not fail", "pvc", newOperation)
					newOperation.Failed = currentStatus.failed
					newOperation.Succeeded = currentStatus.succeeded
					if newOperation.IsComplete() {
						t.Log.Info(
							fmt.Sprintf("Rsync operation completed after %d attempts", newOperation.CurrentAttempt),
							"pvc", newOperation, "failed", newOperation.Failed, "succeded", newOperation.Succeeded)
					} else {
						t.Log.Info("Rsync operation is still running. Waiting for completion",
							"pod", path.Join(pod.Namespace, pod.Name),
							"pvc", newOperation,
						)
					}
				}
			} else {
				newOperation.CurrentAttempt = 0
				labels[RsyncAttemptLabel] = fmt.Sprintf("%d", currentStatus.operation.CurrentAttempt+1)
				if len(filesystemPVCList) > 0 {
					optionsForPvc = append(optionsForPvc, rsynctransfer.WithSourcePodLabels(labels))
					transfer, err := rsynctransfer.NewTransfer(
						fsStunnelTransport, endpoints[0], srcClient, destClient, filesystemPVCList, t.Log, append(rsyncOptions, optionsForPvc...)...)
					if err != nil {
						t.Log.Error(err, "failed creating filesystem transfer", "pvc", currentStatus.operation)
						currentStatus.AddError(err)
						continue
					}
					if currentStatus := t.createClientPodForTransfer(nnPair.Source().Name,
						nnPair.Source().Namespace, transfer, &currentStatus); currentStatus != nil {
						statusList.Add(*currentStatus)
						continue
					}
				}
				if len(uncompletedBlockOrVMPvcList) > 0 {
					destinationName := pvc.Destination().Claim().Name
					serverPodsRunning, _, err := t.areRsyncTransferPodsRunning(destinationName)
					if err != nil {
						currentStatus.AddError(err)
					}
					if serverPodsRunning {
						if err := t.createPVProgressCR(); err != nil {
							// createPVProgressCR doesn't return an error if it already exists
							currentStatus.AddError(err)
							continue
						}
						transferOptions, err := t.getBlockRsyncTransferOptions()
						if err != nil {
							t.Log.Error(err, "failed creating block transfer", "pvc", currentStatus.operation)
							currentStatus.AddError(err)
							continue
						}
						labels["app"] = DirectVolumeMigrationRsyncTransferBlock
						transferOptions.SourcePodMeta = transfer.ResourceMetadata{
							Labels: labels,
						}
						transferOptions.NodeName = nodeName

						transfer, err := blockrsynctransfer.NewTransfer(
							blockStunnelTransport, endpoints[1], srcClient, destClient, uncompletedBlockOrVMPvcList, t.Log, transferOptions)
						if err != nil {
							t.Log.Error(err, "failed creating block transfer", "pvc", currentStatus.operation)
							currentStatus.AddError(err)
							continue
						}

						if currentStatus := t.createClientPodForTransfer(nnPair.Source().Name,
							nnPair.Source().Namespace, transfer, &currentStatus); currentStatus != nil {
							statusList.Add(*currentStatus)
							continue
						}
					}
				}
			}
			statusList.Add(currentStatus)
		}
	}
	return statusList, nil
}

// Cancel offline migration for a single PVC. This will delete the source pod, not the destination pod.
func (t *Task) cancelOfflineMigration(source transfer.PVC, namespace string) error {
	pvcMd5 := getMD5Hash(t.Owner.Name + source.Claim().Name + namespace)
	labels := make(map[string]string)
	labels["app"] = DirectVolumeMigrationRsyncTransferBlock
	labels[migapi.RsyncPodIdentityLabel] = source.LabelSafeName()
	podList := corev1.PodList{}
	if err := t.sourceClient.List(context.TODO(), &podList,
		k8sclient.InNamespace(namespace),
		k8sclient.MatchingLabels(labels)); err != nil {
		return err
	}

	for _, pod := range podList.Items {
		for _, volumes := range pod.Spec.Volumes {
			if volumes.PersistentVolumeClaim != nil && volumes.PersistentVolumeClaim.ClaimName == source.Claim().Name {
				if (pod.Status.Phase == corev1.PodRunning && pod.DeletionTimestamp == nil) || pod.Status.Phase == corev1.PodSucceeded {
					t.Log.V(3).Info("Deleting block rsync pod", "pod", path.Join(pod.Namespace, pod.Name), "pvc", path.Join(namespace, source.Claim().Name))
					if err := t.sourceClient.Delete(context.TODO(), &pod); err != nil && !k8serror.IsNotFound(err) {
						return err
					}
					t.Log.V(3).Info("Deleting DVM progress resource for PVC", "pvc", path.Join(namespace, source.Claim().Name))
					if err := t.deleteDirectVolumeMigrationProgress(pvcMd5); err != nil {
						return err
					}
				}
			}
		}
	}

	return nil
}

func (t *Task) deleteDirectVolumeMigrationProgress(pvcMd5 string) error {
	dvmp := migapi.DirectVolumeMigrationProgress{}
	err := t.Client.Get(context.TODO(), types.NamespacedName{
		Name:      pvcMd5,
		Namespace: migapi.OpenshiftMigrationNamespace,
	}, &dvmp)
	if err != nil && !k8serror.IsNotFound(err) {
		return err
	} else if err == nil {
		return t.Client.Delete(context.TODO(), &dvmp)
	}
	return nil
}

// The offline blockrsync transfer server pod should only be deleted when all the transfers are complete
// Or when one or more of the transfers have failed or when one or more of the transfers have switched to
// live migration.
func (t *Task) deleteOfflineMigrationServerPod(namespace string, pvcPairs []transfer.PVCPair) error {
	labels := t.buildDVMLabels()
	labels["app"] = DirectVolumeMigrationRsyncTransferBlock
	serverPodList := corev1.PodList{}
	if err := t.destinationClient.List(context.TODO(), &serverPodList, k8sclient.InNamespace(namespace), k8sclient.MatchingLabels(labels)); err != nil {
		return err
	}
	destToSourcePVCMap := map[string]string{}
	for _, pvcPair := range pvcPairs {
		destToSourcePVCMap[pvcPair.Destination().Claim().Name] = pvcPair.Source().Claim().Name
	}
	for _, pod := range serverPodList.Items {
		if strings.Contains(pod.Name, "blockrsync-server") && pod.DeletionTimestamp == nil {
			completedBlockRsyncPvSet := getCompletedBlockRsyncPvSet(&t.Owner.Status)
			allCompleted := true
			for _, volumes := range pod.Spec.Volumes {
				if volumes.PersistentVolumeClaim != nil {
					if sourcePVCName, ok := destToSourcePVCMap[volumes.PersistentVolumeClaim.ClaimName]; ok {
						allCompleted = allCompleted && completedBlockRsyncPvSet.Has(sourcePVCName)
					} else if !ok {
						allCompleted = false
					}
				}
			}
			if allCompleted {
				t.Log.Info("Deleting block rsync server pod", "pod", path.Join(pod.Namespace, pod.Name))
				if err := t.destinationClient.Delete(context.TODO(), &pod); err != nil {
					return err
				}
			}
		}
	}
	t.Log.V(3).Info("No running block rsync server pod found")
	return nil
}

// getCompletedBlockRsyncPvSet returns a map of PVCs that have completed. Either successfully or failed
// Live migration pending PVCs are also considered completed from the blockrsync server perspective
func getCompletedBlockRsyncPvSet(ds *migapi.DirectVolumeMigrationStatus) sets.Set[string] {
	completedSet := sets.New[string]()
	for _, progress := range ds.SuccessfulPods {
		if progress.PVCReference != nil {
			completedSet.Insert(progress.PVCReference.Name)
		}
	}
	for _, progress := range ds.FailedPods {
		if progress.PVCReference != nil {
			completedSet.Insert(progress.PVCReference.Name)
		}
	}
	for _, progress := range ds.SuccessfulLiveMigrations {
		if progress.PVCReference != nil {
			completedSet.Insert(progress.PVCReference.Name)
		}
	}
	for _, progress := range ds.FailedLiveMigrations {
		if progress.PVCReference != nil {
			completedSet.Insert(progress.PVCReference.Name)
		}
	}
	for _, progress := range ds.PendingLiveMigrations {
		if progress.PVCReference != nil {
			completedSet.Insert(progress.PVCReference.Name)
		}
	}
	for _, progress := range ds.RunningLiveMigrations {
		if progress.PVCReference != nil {
			completedSet.Insert(progress.PVCReference.Name)
		}
	}
	return completedSet
}

func (t *Task) createClientPodForTransfer(
	name, namespace string,
	tr transfer.Transfer,
	currentStatus *migrationOperationStatus) *migrationOperationStatus {
	if tr == nil {
		currentStatus.AddError(
			fmt.Errorf("transfer %s/%s not found", namespace, name))
		return currentStatus
	}
	if err := transfer.CreateClient(tr); err != nil {
		t.Log.Error(err, "failed creating client", "pvc", currentStatus.operation)
		currentStatus.AddError(err)
		return currentStatus
	}
	return nil
}

func (t *Task) areRsyncTransferPodsRunning(volumeNames ...string) (arePodsRunning bool, nonRunningPods []*corev1.Pod, e error) {
	pvcMap := t.getPVCNamespaceMap()
	containsAllVolumes := true
	dvmLabels := t.buildDVMLabels()
	dvmLabels["purpose"] = DirectVolumeMigrationRsync
	delete(dvmLabels, "app")
	selector := labels.SelectorFromSet(dvmLabels)
	appRequirement, _ := labels.NewRequirement("app", selection.In, []string{DirectVolumeMigrationRsyncTransfer, DirectVolumeMigrationRsyncTransferBlock})
	selector.Add(*appRequirement)

	pods := corev1.PodList{}
	hasOfflineTransfer := false
	for bothNs, pvcMapElements := range pvcMap {
		ns := getDestNs(bothNs)
		if offlineTransfer, err := t.hasOfflineMigration(ns, pvcMapElements); err != nil {
			return false, nil, err
		} else if offlineTransfer {
			hasOfflineTransfer = true
		}
		err := t.destinationClient.List(
			context.TODO(),
			&pods,
			&k8sclient.ListOptions{
				Namespace:     ns,
				LabelSelector: selector,
			})
		if err != nil {
			return false, nil, err
		}
		for _, pod := range pods.Items {
			if pod.Status.Phase != corev1.PodRunning {
				// Log abnormal events for Rsync transfer Pod if any are found
				migevent.LogAbnormalEventsForResource(
					t.destinationClient, t.Log,
					"Found abnormal event for Rsync transfer Pod on destination cluster",
					types.NamespacedName{Namespace: pod.Namespace, Name: pod.Name},
					pod.UID, "Pod")

				isUnschedulable := false
				for _, podCond := range pod.Status.Conditions {
					if podCond.Reason == corev1.PodReasonUnschedulable {
						t.Log.Info("Found UNSCHEDULABLE Rsync Transfer Pod on destination cluster",
							"pod", path.Join(pod.Namespace, pod.Name),
							"podPhase", pod.Status.Phase,
							"podConditionMessage", podCond.Message)
						nonRunningPods = append(nonRunningPods, &pod)
						isUnschedulable = true
						break
					}
				}
				if isUnschedulable {
					continue
				}
				t.Log.Info("Found non-running Rsync Transfer Pod on destination cluster.",
					"pod", path.Join(pod.Namespace, pod.Name),
					"podPhase", pod.Status.Phase)
				nonRunningPods = append(nonRunningPods, &pod)
			} else {
				// Ensure the pod contains the expected volumes.
				for _, volumeName := range volumeNames {
					hasVolume := false
					for _, podVolume := range pod.Spec.Volumes {
						if podVolume.PersistentVolumeClaim != nil && podVolume.PersistentVolumeClaim.ClaimName == volumeName {
							hasVolume = true
							break
						}
					}
					if !hasVolume {
						// Someone stopped a running VM, and we have to wait until the block rsync completes, so we can create
						// a new rsync block server pod.
						t.Log.Info("Running rsync server pod does not contain expected volume", "volume name", volumeName)
					}
					containsAllVolumes = containsAllVolumes && hasVolume
				}
			}
		}
	}
	if len(nonRunningPods) > 0 {
		return false, nonRunningPods, nil
	}
	return (len(pods.Items) > 0 && containsAllVolumes) || !hasOfflineTransfer, nil, nil
}

func (t *Task) hasOfflineMigration(namespace string, pvcMapElements []pvcMapElement) (bool, error) {
	ns := set.New[string]()
	ns.Insert(namespace)
	nsVolumes, err := t.getRunningVMVolumes(ns.SortedList())
	if err != nil {
		return false, err
	}
	for _, pvc := range pvcMapElements {
		if !slices.Contains(nsVolumes, fmt.Sprintf("%s/%s", namespace, pvc.Name)) {
			return true, nil
		}
	}
	return false, nil
}

func (t *Task) createRsyncConfig() error {
	password, err := t.getRsyncPassword()
	if err != nil {
		return err
	}
	if password == "" {
		_, err = t.createRsyncPassword()
		if err != nil {
			return err
		}
	}
	return nil
}

type pvcMapElement struct {
	Name   string
	Verify bool
}

// With namespace mapping, the destination cluster namespace may be different than that in the source cluster.
// This function maps PVCs to the appropriate src:dest namespace pairs.
func (t *Task) getPVCNamespaceMap() map[string][]pvcMapElement {
	nsMap := map[string][]pvcMapElement{}
	for _, pvc := range t.Owner.Spec.PersistentVolumeClaims {
		srcNs := pvc.Namespace
		destNs := srcNs
		if pvc.TargetNamespace != "" {
			destNs = pvc.TargetNamespace
		}
		bothNs := srcNs + ":" + destNs
		if vols, exists := nsMap[bothNs]; exists {
			vols = append(vols, pvcMapElement{Name: pvc.Name, Verify: pvc.Verify})
			nsMap[bothNs] = vols
		} else {
			nsMap[bothNs] = []pvcMapElement{{Name: pvc.Name, Verify: pvc.Verify}}
		}
	}
	return nsMap
}

type securityContextInfo struct {
	fsGroup            *int64
	supplementalGroups []int64
	seLinuxOptions     *corev1.SELinuxOptions
	verify             bool
}

type pvcWithSecurityContextInfo map[string]securityContextInfo

func (p pvcWithSecurityContextInfo) Add(srcClaimName string, srcClaimNamespace string, info securityContextInfo) {
	if p == nil {
		p = make(pvcWithSecurityContextInfo)
	}
	key := fmt.Sprintf("%s/%s", srcClaimNamespace, srcClaimName)
	p[key] = info
}

func (p pvcWithSecurityContextInfo) Get(srcClaimName string, srcClaimNamespace string) (securityContextInfo, bool) {
	key := fmt.Sprintf("%s/%s", srcClaimNamespace, srcClaimName)
	val, exists := p[key]
	return val, exists
}

// With namespace mapping, the destination cluster namespace may be different than that in the source cluster.
// This function maps PVCs to the appropriate src:dest namespace pairs.
func (t *Task) getNamespacedPVCPairs() (map[string][]transfer.PVCPair, error) {
	nsMap := map[string][]transfer.PVCPair{}
	for _, pvc := range t.Owner.Spec.PersistentVolumeClaims {
		srcNs := pvc.Namespace
		destNs := srcNs
		if pvc.TargetNamespace != "" {
			destNs = pvc.TargetNamespace
		}
		srcPvc := corev1.PersistentVolumeClaim{}
		err := t.sourceClient.Get(context.TODO(), types.NamespacedName{Name: pvc.Name, Namespace: srcNs}, &srcPvc)
		if err != nil {
			return nil, err
		}
		destPvc := corev1.PersistentVolumeClaim{}
		err = t.destinationClient.Get(context.TODO(), types.NamespacedName{Name: pvc.TargetName, Namespace: destNs}, &destPvc)
		if err != nil {
			return nil, err
		}
		newPVCPair := transfer.NewPVCPair(&srcPvc, &destPvc)
		bothNs := srcNs + ":" + destNs
		if vols, exists := nsMap[bothNs]; exists {
			vols = append(vols, newPVCPair)
			nsMap[bothNs] = vols
		} else {
			nsMap[bothNs] = []transfer.PVCPair{newPVCPair}
		}
	}
	return nsMap, nil
}

func (t *Task) getSourceSecurityGroupInfo(srcClient compat.Client, pvcPairMap map[string][]transfer.PVCPair) (pvcWithSecurityContextInfo, error) {
	pvcInfo := make(pvcWithSecurityContextInfo)

	for bothNS := range pvcPairMap {
		srcNs := getSourceNs(bothNS)

		podList := &corev1.PodList{}
		err := srcClient.List(context.TODO(), podList, &k8sclient.ListOptions{Namespace: srcNs})
		if err != nil {
			return nil, err
		}

		// for each namespace, have a pvc->SCC map to look up in the pvc loop later
		// we will use the scc of the last pod in the list mounting the pvc
		for _, pod := range podList.Items {
			for _, vol := range pod.Spec.Volumes {
				if vol.PersistentVolumeClaim != nil {
					info := securityContextInfo{
						fsGroup:            pod.Spec.SecurityContext.FSGroup,
						supplementalGroups: pod.Spec.SecurityContext.SupplementalGroups,
						seLinuxOptions:     pod.Spec.SecurityContext.SELinuxOptions,
					}
					pvcInfo.Add(vol.PersistentVolumeClaim.ClaimName, pod.Namespace, info)
				}
			}
		}
	}

	// process verify values and PVCs not attached with any pod
	for _, pvc := range t.Owner.Spec.PersistentVolumeClaims {
		secInfo, exists := pvcInfo.Get(pvc.Name, pvc.Namespace)
		if exists {
			secInfo.verify = pvc.Verify
		} else {
			secInfo = securityContextInfo{
				fsGroup:            nil,
				supplementalGroups: nil,
				seLinuxOptions:     nil,
				verify:             pvc.Verify,
			}
		}
		pvcInfo.Add(pvc.Name, pvc.Namespace, secInfo)
	}
	return pvcInfo, nil
}

func getSourceNs(bothNs string) string {
	nsNames := strings.Split(bothNs, ":")
	return nsNames[0]
}

func getDestNs(bothNs string) string {
	nsNames := strings.Split(bothNs, ":")
	if len(nsNames) > 1 {
		return nsNames[1]
	} else {
		return nsNames[0]
	}
}

func (t *Task) areRsyncRoutesAdmitted() (bool, []string, error) {
	messages := []string{}
	// Get client for destination
	nsMap := t.getPVCNamespaceMap()
	for bothNs := range nsMap {
		namespace := getDestNs(bothNs)

		switch t.EndpointType {
		case migapi.Route:
			route := routev1.Route{}

			key := types.NamespacedName{Name: DirectVolumeMigrationRsyncTransferRoute, Namespace: namespace}
			if err := t.destinationClient.Get(context.TODO(), key, &route); err != nil {
				return false, messages, err
			}
			// Logs abnormal events related to route if any are found
			migevent.LogAbnormalEventsForResource(
				t.destinationClient, t.Log,
				"Found abnormal event for Rsync Route on destination cluster",
				types.NamespacedName{Namespace: route.Namespace, Name: route.Name},
				route.UID, "Route")

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
		default:
			if _, err := t.getEndpoints(t.destinationClient, namespace); err != nil {
				t.Log.Info("rsync transfer service is not healthy", "namespace", namespace)
				messages = append(messages, fmt.Sprintf("rsync transfer service is not healthy in namespace %s", namespace))
			}

		}
	}
	if len(messages) > 0 {
		return false, messages, nil
	}
	return true, []string{}, nil
}

// getRsyncPasswordSecretName returns a unique name for Secret object created to store Rsync password
func (t *Task) getRsyncPasswordSecretName() string {
	return getMD5Hash(fmt.Sprintf("%s-%s", DirectVolumeMigrationRsyncPass, t.Owner.Name))
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
			Name:      t.getRsyncPasswordSecretName(),
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
	key := types.NamespacedName{
		Name:      t.getRsyncPasswordSecretName(),
		Namespace: migapi.OpenshiftMigrationNamespace,
	}
	t.Log.Info("Getting Rsync Password from Secret on host MigCluster",
		"secret", path.Join(key.Namespace, key.Name))
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
			Name:      t.getRsyncPasswordSecretName(),
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

// Returns a map of PVCNamespacedName to the pod.NodeName
func (t *Task) getPVCNodeNameMap(srcClient compat.Client) (map[string]string, error) {
	nodeNameMap := map[string]string{}
	pvcMap := t.getPVCNamespaceMap()

	for bothNs, _ := range pvcMap {
		ns := getSourceNs(bothNs)

		nsPodList := corev1.PodList{}
		err := srcClient.List(context.TODO(), &nsPodList, k8sclient.InNamespace(ns))
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

func isRsyncSuperPrivileged(client compat.Client) (bool, error) {
	cm := &corev1.ConfigMap{}
	err := client.Get(context.TODO(), k8sclient.ObjectKey{Name: migapi.ClusterConfigMapName, Namespace: migapi.OpenshiftMigrationNamespace}, cm)
	if err != nil {
		return false, err
	}
	if cm.Data != nil {
		isRsyncSuperPrivileged, exists := cm.Data["RSYNC_SUPER_PRIVILEGED"]
		if !exists {
			return false, fmt.Errorf("RSYNC_SUPER_PRIVILEGED boolean does not exist. Verify source and destination clusters operators are up to date")
		}
		parsed, err := strconv.ParseBool(isRsyncSuperPrivileged)
		if err != nil {
			return false, err
		}
		return parsed, nil
	}
	return false, fmt.Errorf("configmap %s of source cluster has empty data", k8sclient.ObjectKey{Name: migapi.ClusterConfigMapName, Namespace: migapi.OpenshiftMigrationNamespace}.String())
}

// deleteInvalidPVProgressCR deletes an existing CR which doesn't have expected fields
// used to delete CRs created pre MTCv1.4.3
func (t *Task) deleteInvalidPVProgressCR(dvmp *migapi.DirectVolumeMigrationProgress) error {
	existingDvmp := migapi.DirectVolumeMigrationProgress{}
	// Make sure existing DVMP CRs which don't have required fields are deleted
	err := t.Client.Get(context.TODO(), types.NamespacedName{Name: dvmp.Name, Namespace: dvmp.Namespace}, &existingDvmp)
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
			_, exists := existingDvmp.Spec.PodSelector[migapi.RsyncPodIdentityLabel]
			if !exists {
				shouldDelete = true
			}
		}
		if shouldDelete {
			err := t.Client.Delete(context.TODO(), &existingDvmp)
			if err != nil {
				return err
			}
			t.Log.Info("Deleted DVMP as it was missing required fields", "DVMP", path.Join(dvmp.Namespace, dvmp.Name))
		}
	}
	return nil
}

// Create rsync PV progress CR on destination cluster
func (t *Task) createPVProgressCR() error {
	pvcMap := t.getPVCNamespaceMap()
	labels := t.Owner.GetCorrelationLabels()
	for bothNs, vols := range pvcMap {
		ns := getSourceNs(bothNs)
		volumeNames, err := t.getRunningVMVolumes([]string{ns})
		if err != nil {
			return liberr.Wrap(err)
		}
		for _, vol := range vols {
			matchString := fmt.Sprintf("%s/%s", ns, vol.Name)
			if t.PlanResources.MigPlan.LiveMigrationChecked() &&
				slices.Contains(volumeNames, matchString) {
				t.Log.V(3).Info("Skipping Rsync Progress CR creation for running VM", "volume", matchString)
				continue
			}
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
			err := t.deleteInvalidPVProgressCR(&dvmp)
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
	// Keep current progress in case looking up progress fails, this way we don't wipe out
	// the progress until next time the update succeeds.
	currentProgress := t.getCurrentLiveMigrationProgress()
	t.resetProgressCounters()
	pvcMap := t.getPVCNamespaceMap()
	for bothNs, vols := range pvcMap {
		ns := getSourceNs(bothNs)
		if err := t.populateVMMappings(ns); err != nil {
			return false, err
		}
		t.Owner.Status.SkippedVolumes = []string{}
		for _, vol := range vols {
			matchString := fmt.Sprintf("%s/%s", ns, vol.Name)
			if t.PlanResources.MigPlan.LiveMigrationChecked() &&
				slices.Contains(t.VirtualMachineMappings.runningVMVolumeNames, matchString) {
				// Only count skipped during staging. During cutover we need to live migrate
				// PVCs for running VMs. For reporting purposes we won't count skipped PVCs
				// here since they will get reported with the live migration status.
				if !(t.Owner.IsCutover() || t.Owner.IsRollback()) {
					t.Owner.SkipVolume(vol.Name, ns)
				} else {
					if err := t.updateVolumeLiveMigrationProgressStatus(vol.Name, ns, currentProgress); err != nil {
						return false, err
					}
				}
			} else {
				// On rollback we are only interested in live migration volumes, skip the rest
				if t.Owner.IsRollback() {
					t.Owner.SkipVolume(vol.Name, ns)
				}
				if err := t.updateRsyncProgressStatus(vol.Name, ns); err != nil {
					return false, err
				}
			}
		}
	}
	return t.Owner.AllReportingCompleted(), nil
}

func (t *Task) updateVolumeLiveMigrationProgressStatus(volumeName, namespace string, currentProgress map[string]*migapi.LiveMigrationProgress) error {
	matchString := fmt.Sprintf("%s/%s", namespace, volumeName)

	liveMigrationProgress := &migapi.LiveMigrationProgress{
		VMName:      "",
		VMNamespace: namespace,
		PVCReference: &corev1.ObjectReference{
			Namespace: namespace,
			Name:      volumeName,
		},
		TotalElapsedTime:            nil,
		LastObservedProgressPercent: "",
	}
	// Look up VirtualMachineInstanceMigration CR to get the status of the migration
	if vmim, exists := t.VirtualMachineMappings.volumeVMIMMap[matchString]; exists && vmim != nil && vmim.Status.MigrationState != nil {
		liveMigrationProgress.VMName = vmim.Spec.VMIName
		elapsedTime := getVMIMElapsedTime(vmim)
		liveMigrationProgress.TotalElapsedTime = &elapsedTime
		if vmim.Status.MigrationState.FailureReason != "" {
			liveMigrationProgress.Message = vmim.Status.MigrationState.FailureReason
		}
		switch vmim.Status.Phase {
		case virtv1.MigrationSucceeded:
			liveMigrationProgress.LastObservedProgressPercent = "100%"
			t.Owner.Status.SuccessfulLiveMigrations = append(t.Owner.Status.SuccessfulLiveMigrations, liveMigrationProgress)
		case virtv1.MigrationFailed:
			// Only set to failed if the VMI is there and not terminating
			vmName := t.VirtualMachineMappings.volumeVMNameMap[volumeName]
			vmi := &virtv1.VirtualMachineInstance{}
			err := t.sourceClient.Get(context.TODO(), k8sclient.ObjectKey{Namespace: namespace, Name: vmName}, vmi)
			if err != nil && !k8serror.IsNotFound(err) {
				return err
			}
			if !k8serror.IsNotFound(err) && vmi.DeletionTimestamp == nil && vmi.Status.Phase == virtv1.Running {
				t.Owner.Status.FailedLiveMigrations = append(t.Owner.Status.FailedLiveMigrations, liveMigrationProgress)
			}
		case virtv1.MigrationRunning:
			progressPercent, err := t.getLastObservedProgressPercent(vmim.Spec.VMIName, namespace, currentProgress)
			if err != nil {
				return err
			}
			liveMigrationProgress.LastObservedProgressPercent = progressPercent
			t.Owner.Status.RunningLiveMigrations = append(t.Owner.Status.RunningLiveMigrations, liveMigrationProgress)
		case virtv1.MigrationPending:
			t.Owner.Status.PendingLiveMigrations = append(t.Owner.Status.PendingLiveMigrations, liveMigrationProgress)
		}
	} else {
		// VMIM doesn't exist, check if the VMI is in error.
		vmName := t.VirtualMachineMappings.volumeVMNameMap[volumeName]
		message, err := virtualMachineMigrationStatus(t.sourceClient, vmName, namespace, t.Log)
		if err != nil {
			if k8serror.IsNotFound(err) {
				t.Log.V(3).Info("VirtualMachineInstance not found", "vm", vmName, "namespace", namespace, "error", err)
				// VMI doesn't exist, skip, so that an offline migration can start
				return nil
			}
			return err
		}
		liveMigrationProgress.VMName = vmName
		if message == vmiNotFound {
			return nil
		}
		if message != "" {
			vmMatchString := fmt.Sprintf("%s/%s", namespace, vmName)
			liveMigrationProgress.Message = message
			if currentProgress[vmMatchString] != nil && currentProgress[vmMatchString].TotalElapsedTime != nil {
				liveMigrationProgress.TotalElapsedTime = currentProgress[vmMatchString].TotalElapsedTime
			} else {
				if t.Owner.Status.StartTimestamp != nil {
					dvmStart := *t.Owner.Status.StartTimestamp
					liveMigrationProgress.TotalElapsedTime = &metav1.Duration{
						Duration: time.Since(dvmStart.Time),
					}
				}
			}
			t.Owner.Status.FailedLiveMigrations = append(t.Owner.Status.FailedLiveMigrations, liveMigrationProgress)
		} else {
			t.Owner.Status.PendingLiveMigrations = append(t.Owner.Status.PendingLiveMigrations, liveMigrationProgress)
		}
	}
	return nil
}

func (t *Task) updateRsyncProgressStatus(volumeName, namespace string) error {
	operation := t.Owner.Status.GetRsyncOperationStatusForPVC(&corev1.ObjectReference{
		Namespace: namespace,
		Name:      volumeName,
	})
	dvmp := migapi.DirectVolumeMigrationProgress{}
	err := t.Client.Get(context.TODO(), types.NamespacedName{
		Name:      getMD5Hash(t.Owner.Name + volumeName + namespace),
		Namespace: migapi.OpenshiftMigrationNamespace,
	}, &dvmp)
	if err != nil && !k8serror.IsNotFound(err) {
		return err
	} else if k8serror.IsNotFound(err) {
		t.Log.Info("Rsync Progress CR not found", "volume", volumeName, "namespace", namespace, "operation", operation)
		return nil
	}
	podProgress := &migapi.PodProgress{
		ObjectReference: &corev1.ObjectReference{
			Namespace: namespace,
			Name:      dvmp.Status.PodName,
		},
		PVCReference: &corev1.ObjectReference{
			Namespace: namespace,
			Name:      volumeName,
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
				t.Owner.Status.PendingSinceTimeLimitPods = append(t.Owner.Status.PendingSinceTimeLimitPods, podProgress)
			}
		}
	case dvmp.Status.PodPhase == "":
		t.Owner.Status.UnknownPods = append(t.Owner.Status.UnknownPods, podProgress)
	case !operation.Failed:
		t.Owner.Status.RunningPods = append(t.Owner.Status.RunningPods, podProgress)
	}
	return nil
}

func (t *Task) resetProgressCounters() {
	t.Owner.Status.RunningPods = []*migapi.PodProgress{}
	t.Owner.Status.FailedPods = []*migapi.PodProgress{}
	t.Owner.Status.SuccessfulPods = []*migapi.PodProgress{}
	t.Owner.Status.PendingPods = []*migapi.PodProgress{}
	t.Owner.Status.UnknownPods = []*migapi.PodProgress{}
	t.Owner.Status.RunningLiveMigrations = []*migapi.LiveMigrationProgress{}
	t.Owner.Status.FailedLiveMigrations = []*migapi.LiveMigrationProgress{}
	t.Owner.Status.SuccessfulLiveMigrations = []*migapi.LiveMigrationProgress{}
	t.Owner.Status.PendingLiveMigrations = []*migapi.LiveMigrationProgress{}
}

func (t *Task) getCurrentLiveMigrationProgress() map[string]*migapi.LiveMigrationProgress {
	currentProgress := make(map[string]*migapi.LiveMigrationProgress)
	for _, progress := range t.Owner.Status.RunningLiveMigrations {
		currentProgress[fmt.Sprintf("%s/%s", progress.VMNamespace, progress.VMName)] = progress
	}
	for _, progress := range t.Owner.Status.FailedLiveMigrations {
		currentProgress[fmt.Sprintf("%s/%s", progress.VMNamespace, progress.VMName)] = progress
	}
	for _, progress := range t.Owner.Status.SuccessfulLiveMigrations {
		currentProgress[fmt.Sprintf("%s/%s", progress.VMNamespace, progress.VMName)] = progress
	}
	for _, progress := range t.Owner.Status.PendingLiveMigrations {
		currentProgress[fmt.Sprintf("%s/%s", progress.VMNamespace, progress.VMName)] = progress
	}
	return currentProgress
}

func (t *Task) hasAllRsyncClientPodsTimedOut() (bool, error) {
	for bothNs, vols := range t.getPVCNamespaceMap() {
		ns := getSourceNs(bothNs)
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
	for bothNs, vols := range t.getPVCNamespaceMap() {
		ns := getSourceNs(bothNs)
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
	t.Log.Info("Checking for stale Rsync resources on source MigCluster",
		"migCluster",
		path.Join(t.Owner.Spec.SrcMigClusterRef.Namespace, t.Owner.Spec.SrcMigClusterRef.Name))
	t.Log.Info("Checking for stale Rsync resources on destination MigCluster",
		"migCluster",
		path.Join(t.Owner.Spec.DestMigClusterRef.Namespace, t.Owner.Spec.DestMigClusterRef.Name))
	if err := t.findAndDeleteResources(t.sourceClient, t.destinationClient, t.getPVCNamespaceMap()); err != nil {
		return err
	}

	if err := t.deleteRsyncPassword(); err != nil {
		return err
	}

	if !t.Owner.Spec.DeleteProgressReportingCRs {
		return nil
	}

	t.Log.Info("Checking for stale DVMP resources on host MigCluster",
		"migCluster", "host")
	if err := t.deleteProgressReportingCRs(t.Client); err != nil {
		return err
	}

	return nil
}

func (t *Task) waitForRsyncResourcesDeleted() (bool, error) {
	t.Log.Info("Checking if Rsync resource deletion has completed on source and destination MigClusters")
	pvcMap := t.getPVCNamespaceMap()
	// Find all resources with the app label
	// TODO: This label set should include a DVM run-specific UID.
	req, err := labels.NewRequirement("app", selection.In, []string{DirectVolumeMigrationRsyncTransfer, DirectVolumeMigrationRsyncTransferBlock})
	if err != nil {
		return false, err
	}
	selector := labels.NewSelector().Add(*req)
	for bothNs := range pvcMap {
		srcNs := getSourceNs(bothNs)
		destNs := getDestNs(bothNs)
		t.Log.Info("Searching source namespace for leftover Rsync Pods, ConfigMaps, "+
			"Services, Secrets, Routes with label.",
			"searchNamespace", srcNs,
			"labelSelector", selector)
		err, areDeleted := areRsyncNsResourcesDeleted(t.sourceClient, srcNs, selector, t.Log)
		if err != nil {
			return false, err
		}
		if !areDeleted {
			return false, nil
		}
		t.Log.Info("Searching destination namespace for leftover Rsync Pods, ConfigMaps, "+
			"Services, Secrets, Routes with label.",
			"searchNamespace", destNs,
			"labelSelector", selector)
		err, areDeleted = areRsyncNsResourcesDeleted(t.destinationClient, destNs, selector, t.Log)
		if err != nil {
			return false, err
		}
		if !areDeleted {
			return false, nil
		}
	}
	return true, nil
}

func areRsyncNsResourcesDeleted(client compat.Client, ns string, selector labels.Selector, log logr.Logger) (error, bool) {
	podList := corev1.PodList{}
	cmList := corev1.ConfigMapList{}
	svcList := corev1.ServiceList{}
	secretList := corev1.SecretList{}
	routeList := routev1.RouteList{}

	// Get Pod list
	err := client.List(
		context.TODO(),
		&podList,
		&k8sclient.ListOptions{
			Namespace:     ns,
			LabelSelector: selector,
		})
	if err != nil {
		return err, false
	}
	if len(podList.Items) > 0 {
		log.Info("Found stale Rsync Pod.",
			"pod", path.Join(podList.Items[0].Namespace, podList.Items[0].Name),
			"podPhase", podList.Items[0].Status.Phase)
		return nil, false
	}
	// Get Secret list
	err = client.List(
		context.TODO(),
		&secretList,
		&k8sclient.ListOptions{
			Namespace:     ns,
			LabelSelector: selector,
		})
	if err != nil {
		return err, false
	}
	if len(secretList.Items) > 0 {
		log.Info("Found stale Rsync Secret.",
			"secret", path.Join(secretList.Items[0].Namespace, secretList.Items[0].Name))
		return nil, false
	}
	// Get configmap list
	err = client.List(
		context.TODO(),
		&cmList,
		&k8sclient.ListOptions{
			Namespace:     ns,
			LabelSelector: selector,
		})
	if err != nil {
		return err, false
	}
	if len(cmList.Items) > 0 {
		log.Info("Found stale Rsync ConfigMap.",
			"configMap", path.Join(cmList.Items[0].Namespace, cmList.Items[0].Name))
		return nil, false
	}
	// Get svc list
	err = client.List(
		context.TODO(),
		&svcList,
		&k8sclient.ListOptions{
			Namespace:     ns,
			LabelSelector: selector,
		})
	if err != nil {
		return err, false
	}
	if len(svcList.Items) > 0 {
		log.Info("Found stale Rsync Service.",
			"service", path.Join(svcList.Items[0].Namespace, svcList.Items[0].Name))
		return nil, false
	}

	// Get route list
	err = client.List(
		context.TODO(),
		&routeList,
		&k8sclient.ListOptions{
			Namespace:     ns,
			LabelSelector: selector,
		})
	if err != nil {
		return err, false
	}
	if len(routeList.Items) > 0 {
		log.Info("Found stale Rsync Route.",
			"route", path.Join(routeList.Items[0].Namespace, routeList.Items[0].Name))
		return nil, false
	}
	return nil, true
}

func (t *Task) findAndDeleteResources(srcClient, destClient compat.Client, pvcMap map[string][]pvcMapElement) error {
	// Find all resources with the app label
	// TODO: This label set should include a DVM run-specific UID.
	req, err := labels.NewRequirement("app", selection.In, []string{DirectVolumeMigrationRsyncTransfer, DirectVolumeMigrationRsyncTransferBlock})
	if err != nil {
		return err
	}
	selector := labels.NewSelector().Add(*req)
	for bothNs := range pvcMap {
		srcNs := getSourceNs(bothNs)
		destNs := getDestNs(bothNs)
		err = findAndDeleteNsResources(srcClient, srcNs, selector, t.Log)
		if err != nil {
			return err
		}
		err = findAndDeleteNsResources(destClient, destNs, selector, t.Log)
		if err != nil {
			return err
		}
	}
	return nil
}

func findCompletedDestinationVolumes(sourceClient compat.Client, srcNs string, pvcPairs []transfer.PVCPair, selector labels.Selector, log logr.Logger) (sets.Set[string], error) {
	podList := corev1.PodList{}
	completedDestinationVolumes := sets.New[string]()
	err := sourceClient.List(context.TODO(), &podList, &k8sclient.ListOptions{
		Namespace:     srcNs,
		LabelSelector: selector,
	})
	if err != nil {
		return completedDestinationVolumes, err
	}
	for _, pod := range podList.Items {
		if pod.Status.Phase == corev1.PodSucceeded {
			for _, vol := range pvcPairs {
				for _, podVolumes := range pod.Spec.Volumes {
					log.Info("Checking pod volumes", "podVolumes", podVolumes, "source", vol.Source().Claim().Name)
					if podVolumes.PersistentVolumeClaim != nil && podVolumes.PersistentVolumeClaim.ClaimName == vol.Source().Claim().Name {
						completedDestinationVolumes.Insert(vol.Destination().Claim().Name)
					}
				}
			}
		}
	}
	return completedDestinationVolumes, nil
}

func (t *Task) deleteCompletedSourceRsyncPods(completedSourceVolumeNames sets.Set[string], srcNs string, selector labels.Selector) error {
	podList := corev1.PodList{}
	err := t.sourceClient.List(context.TODO(), &podList, &k8sclient.ListOptions{
		Namespace:     srcNs,
		LabelSelector: selector,
	})
	if err != nil {
		return err
	}
	for _, pod := range podList.Items {
		if pod.Status.Phase == corev1.PodSucceeded {
			for _, volume := range pod.Spec.Volumes {
				if volume.PersistentVolumeClaim != nil && completedSourceVolumeNames.Has(volume.PersistentVolumeClaim.ClaimName) {
					//only delete pod once the DVMP is completely updated.
					if complete, err := t.isDirectVolumeMigrationProgressCompleteForVolume(volume.PersistentVolumeClaim.ClaimName, srcNs); err != nil {
						return err
					} else if !complete {
						continue
					}
					// All progress is updated, free to delete the source rsync pod
					log.Info("Deleting completed Rsync Pod on source MigCluster", "pod", pod.Name)
					err = t.sourceClient.Delete(context.TODO(), &pod)
					if err != nil && !k8serror.IsNotFound(err) {
						return err
					}
				}
			}
		}
	}
	return nil
}

func (t *Task) isDirectVolumeMigrationProgressCompleteForVolume(volumeName, srcNs string) (bool, error) {
	dvmp := migapi.DirectVolumeMigrationProgress{}
	md5Hash := getMD5Hash(t.Owner.Name + volumeName + srcNs)
	if err := t.Client.Get(context.TODO(), types.NamespacedName{
		Name:      md5Hash,
		Namespace: migapi.OpenshiftMigrationNamespace,
	}, &dvmp); err != nil {
		if k8serror.IsNotFound(err) {
			return true, nil
		}
		return false, err
	}
	return dvmp.Status.PodPhase == corev1.PodSucceeded, nil
}

func (t *Task) findAndDeleteCompletedRsyncClientPods(srcNs string, pvcPairs []transfer.PVCPair, selector labels.Selector) error {
	destToSourceMap := map[string]string{}
	for _, pvcPair := range pvcPairs {
		destToSourceMap[pvcPair.Destination().Claim().Name] = pvcPair.Source().Claim().Name
	}
	completedDestinationVolumes, err := findCompletedDestinationVolumes(t.sourceClient, srcNs, pvcPairs, selector, log)
	if err != nil {
		return err
	}

	for _, completedDestinationVolume := range completedDestinationVolumes.UnsortedList() {
		if value, exists := destToSourceMap[completedDestinationVolume]; exists {
			if err := t.updateRsyncProgressStatus(value, srcNs); err != nil {
				return err
			}
		}
	}

	completedSourceVolumes := sets.New[string]()
	for _, v := range completedDestinationVolumes.UnsortedList() {
		if _, exists := destToSourceMap[v]; exists {
			completedSourceVolumes.Insert(destToSourceMap[v])
		}
	}
	err = t.deleteCompletedSourceRsyncPods(completedSourceVolumes, srcNs, selector)
	if err != nil {
		return err
	}
	return nil
}

func findAndDeleteNsResources(client compat.Client, ns string, selector labels.Selector, log logr.Logger) error {
	podList := corev1.PodList{}
	cmList := corev1.ConfigMapList{}
	svcList := corev1.ServiceList{}
	secretList := corev1.SecretList{}
	routeList := routev1.RouteList{}

	// Get Pod list
	err := client.List(
		context.TODO(),
		&podList,
		&k8sclient.ListOptions{
			Namespace:     ns,
			LabelSelector: selector,
		})
	if err != nil {
		return err
	}
	// Get Secret list
	err = client.List(
		context.TODO(),
		&secretList,
		&k8sclient.ListOptions{
			Namespace:     ns,
			LabelSelector: selector,
		})
	if err != nil {
		return err
	}

	// Get configmap list
	err = client.List(
		context.TODO(),
		&cmList,
		&k8sclient.ListOptions{
			Namespace:     ns,
			LabelSelector: selector,
		})
	if err != nil {
		return err
	}

	// Get svc list
	err = client.List(
		context.TODO(),
		&svcList,
		&k8sclient.ListOptions{
			Namespace:     ns,
			LabelSelector: selector,
		})
	if err != nil {
		return err
	}

	// Get route list
	err = client.List(
		context.TODO(),
		&routeList,
		&k8sclient.ListOptions{
			Namespace:     ns,
			LabelSelector: selector,
		})
	if err != nil {
		return err
	}

	// Delete pods
	for _, pod := range podList.Items {
		log.Info("Deleting stale DVM Pod",
			"pod", path.Join(pod.Namespace, pod.Name))
		err = client.Delete(context.TODO(), &pod, k8sclient.PropagationPolicy(metav1.DeletePropagationBackground))
		if err != nil && !k8serror.IsNotFound(err) {
			return err
		}
	}

	// Delete secrets
	for _, secret := range secretList.Items {
		log.Info("Deleting stale DVM Secret",
			"secret", path.Join(secret.Namespace, secret.Name))
		err = client.Delete(context.TODO(), &secret, k8sclient.PropagationPolicy(metav1.DeletePropagationBackground))
		if err != nil && !k8serror.IsNotFound(err) {
			return err
		}
	}

	// Delete routes
	for _, route := range routeList.Items {
		log.Info("Deleting stale DVM Route",
			"route", path.Join(route.Namespace, route.Name))
		err = client.Delete(context.TODO(), &route, k8sclient.PropagationPolicy(metav1.DeletePropagationBackground))
		if err != nil && !k8serror.IsNotFound(err) {
			return err
		}
	}

	// Delete svcs
	for _, svc := range svcList.Items {
		log.Info("Deleting stale DVM Service",
			"service", path.Join(svc.Namespace, svc.Name))
		err = client.Delete(context.TODO(), &svc, k8sclient.PropagationPolicy(metav1.DeletePropagationBackground))
		if err != nil && !k8serror.IsNotFound(err) {
			return err
		}
	}

	// Delete configmaps
	for _, cm := range cmList.Items {
		log.Info("Deleting stale DVM ConfigMap",
			"configMap", path.Join(cm.Namespace, cm.Name))
		err = client.Delete(context.TODO(), &cm, k8sclient.PropagationPolicy(metav1.DeletePropagationBackground))
		if err != nil && !k8serror.IsNotFound(err) {
			return err
		}
	}
	return nil
}

func (t *Task) deleteProgressReportingCRs(client k8sclient.Client) error {
	pvcMap := t.getPVCNamespaceMap()

	for bothNs, vols := range pvcMap {
		ns := getSourceNs(bothNs)
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

// checking privileged labels in destination namespace
func isPrivilegedLabelPresent(client compat.Client, namespace string) (bool, error) {
	ns := &corev1.Namespace{}
	err := client.Get(context.TODO(), types.NamespacedName{Name: namespace}, ns)
	if err != nil {
		return false, liberr.Wrap(err)
	}

	if ns.Labels != nil {
		if ns.Labels["pod-security.kubernetes.io/enforce"] == "privileged" {
			return true, nil
		}
		return false, nil
	}
	return false, nil
}

func GetRsyncPodBackOffLimit(dvm *migapi.DirectVolumeMigration) int {
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
	pvcMap, err := t.getNamespacedPVCPairs()
	if err != nil {
		return false, false, failureReasons, err
	}
	err = t.buildSourceLimitRangeMap(pvcMap, t.sourceClient)
	if err != nil {
		return false, false, failureReasons, err
	}
	var (
		status *migrationOperationStatusList
		progressCompleted,
		rsyncOperationsCompleted,
		anyRsyncFailed bool
	)

	for bothNs, pvcPairs := range pvcMap {
		dvmLabels := t.buildDVMLabels()
		dvmLabels["app"] = DirectVolumeMigrationRsyncTransferBlock
		selector := labels.SelectorFromSet(dvmLabels)
		srcNs := getSourceNs(bothNs)
		if err := t.findAndDeleteCompletedRsyncClientPods(srcNs, pvcPairs, selector); err != nil {
			return false, false, failureReasons, err
		}
	}

	if t.Owner.IsLiveMigrate() && t.Owner.IsCutover() {
		transportOptions, err := t.getStunnelOptions()
		if err != nil {
			return false, false, failureReasons, err
		}
		nsMap, err := t.getNamespacedPVCPairs()
		if err != nil {
			return false, false, failureReasons, err
		}
		if err := t.ensureBlockRsyncTransferServer(nsMap, transportOptions); err != nil {
			return false, false, failureReasons, err
		}
	}
	if !t.Owner.IsRollback() {
		t.Log.V(3).Info("Creating Rsync Transfer Clients")
		status, err = t.createRsyncTransferClients(t.sourceClient, t.destinationClient, pvcMap)
		if err != nil {
			return false, false, failureReasons, err
		}
		t.podPendingSinceTimeLimit()
	}
	// report progress of pods
	progressCompleted, err = t.hasAllProgressReportingCompleted()
	if err != nil {
		return false, false, failureReasons, err
	}
	migrationOperationsCompleted := true
	anyMigrationFailed := false
	if t.Owner.IsCutover() || t.Owner.IsRollback() {
		liveMigrationPVCMap := pvcMap
		if t.Owner.IsRollback() {
			// Swap source and destination PVCs for rollback
			liveMigrationPVCMap = swapSourceDestination(pvcMap)
		}
		var migrationFailureReasons []string
		if t.Owner.IsLiveMigrate() {
			// Doing a cutover or rollback, start any live migrations if needed.
			failureReasons, err = t.startLiveMigrations(liveMigrationPVCMap)
			if err != nil {
				return false, len(failureReasons) > 0, failureReasons, err
			}
			migrationOperationsCompleted, anyMigrationFailed, migrationFailureReasons, err = t.processMigrationOperationStatus(pvcMap, t.sourceClient)
			if err != nil {
				return false, len(migrationFailureReasons) > 0, migrationFailureReasons, err
			}
			failureReasons = append(failureReasons, migrationFailureReasons...)
		}
	}

	if !t.Owner.IsRollback() {
		var rsyncFailureReasons []string
		rsyncOperationsCompleted, anyRsyncFailed, rsyncFailureReasons, err = t.processRsyncOperationStatus(status, []error{})
		if err != nil {
			return false, len(failureReasons) > 0, failureReasons, err
		}
		failureReasons = append(failureReasons, rsyncFailureReasons...)
	} else {
		rsyncOperationsCompleted = true
	}
	t.Log.V(3).Info("Migration Operations Completed", "MigrationOperationsCompleted", migrationOperationsCompleted, "RsyncOperationsCompleted", rsyncOperationsCompleted, "ProgressCompleted", progressCompleted)
	return migrationOperationsCompleted && rsyncOperationsCompleted && progressCompleted, anyRsyncFailed || anyMigrationFailed, failureReasons, nil
}

func swapSourceDestination(pvcMap map[string][]transfer.PVCPair) map[string][]transfer.PVCPair {
	swappedMap := make(map[string][]transfer.PVCPair)
	for bothNs, volumes := range pvcMap {
		swappedVolumes := make([]transfer.PVCPair, 0)
		for _, volume := range volumes {
			swappedVolumes = append(swappedVolumes, transfer.NewPVCPair(volume.Destination().Claim(), volume.Source().Claim()))
		}
		ns := getSourceNs(bothNs)
		destNs := getDestNs(bothNs)
		swappedMap[fmt.Sprintf("%s:%s", destNs, ns)] = swappedVolumes
	}
	return swappedMap
}

func (t *Task) podPendingSinceTimeLimit() {
	if len(t.Owner.Status.PendingSinceTimeLimitPods) > 0 {
		pendingPods := make([]string, 0)
		for _, pod := range t.Owner.Status.PendingSinceTimeLimitPods {
			pendingPods = append(pendingPods, fmt.Sprintf("%s/%s", pod.Namespace, pod.Name))
		}

		pendingMessage := fmt.Sprintf("Rsync Client Pods [%s] are stuck in Pending state for more than 10 mins", strings.Join(pendingPods[:], ", "))
		t.Owner.Status.SetCondition(migapi.Condition{
			Type:     RsyncClientPodsPending,
			Status:   migapi.True,
			Reason:   "PodStuckInContainerCreating",
			Category: migapi.Warn,
			Message:  pendingMessage,
		})
	}
}

// Count the number of VMs, and the number of migrations in error/completed. If they match total isComplete needs to be true
// returns:
// isComplete: whether all migrations are completed, false if no pvc pairs are found
// anyFailed: whether any of the migrations failed
// failureReasons: list of failure reasons
// error: error if any
func (t *Task) processMigrationOperationStatus(nsMap map[string][]transfer.PVCPair, sourceClient k8sclient.Client) (bool, bool, []string, error) {
	isComplete, anyFailed, failureReasons := false, false, make([]string, 0)
	vmVolumeMap := make(map[string]vmVolumes)

	for k, v := range nsMap {
		namespace, err := getNamespace(k)
		if err != nil {
			failureReasons = append(failureReasons, err.Error())
			return isComplete, anyFailed, failureReasons, err
		}
		volumeVmMap, err := getRunningVmVolumeMap(sourceClient, namespace)
		if err != nil {
			failureReasons = append(failureReasons, err.Error())
			return isComplete, anyFailed, failureReasons, err
		}
		for _, pvcPair := range v {
			if vmName, found := volumeVmMap[pvcPair.Source().Claim().Name]; found {
				vmVolumeMap[vmName] = vmVolumes{
					sourceVolumes: append(vmVolumeMap[vmName].sourceVolumes, pvcPair.Source().Claim().Name),
					targetVolumes: append(vmVolumeMap[vmName].targetVolumes, pvcPair.Destination().Claim().Name),
				}
			}
		}
		isComplete = true
		completeCount := 0
		failedCount := 0
		for vmName := range vmVolumeMap {
			message, err := virtualMachineMigrationStatus(sourceClient, vmName, namespace, t.Log)
			if err != nil {
				if !k8serror.IsNotFound(err) {
					failedCount++
					anyFailed = true
					failureReasons = append(failureReasons, message)
				} else {
					t.Log.V(3).Info("Unable to update progress, VirtualMachineInstance not found", "vm", vmName, "namespace", namespace, "error", err)
				}
				return isComplete, anyFailed, failureReasons, err
			}
			if message == "" {
				// Completed
				completeCount++
			} else {
				// Failed
				failedCount++
				anyFailed = true
				failureReasons = append(failureReasons, message)
			}
		}
		isComplete = true
	}
	return isComplete, anyFailed, failureReasons, nil
}

// processRsyncOperationStatus processes status of Rsync operations by reading the status list
// returns whether all operations are completed and whether any of the operation is failed
func (t *Task) processRsyncOperationStatus(status *migrationOperationStatusList, garbageCollectionErrors []error) (bool, bool, []string, error) {
	isComplete, anyFailed, failureReasons := false, false, make([]string, 0)
	if status.AllCompleted() {
		isComplete = true
		// we are done running rsync, we can move on
		// need to check whether there are any permanent failures
		if status.Failed() > 0 {
			anyFailed = true
			// attempt to categorize failures in any of the special failure categories we defined
			failureReasons, err := t.reportAdvancedRsyncErrorHeuristics()
			if err != nil {
				return isComplete, anyFailed, failureReasons, liberr.Wrap(err)
			}
		}
		return isComplete, anyFailed, failureReasons, nil
	}
	if status.AnyErrored() {
		// check if we are seeing errors running any of the operation for over 5 minutes
		// if yes, set a warning condition
		t.Owner.Status.StageCondition(Running)
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
		t.Owner.Status.StageCondition(Running)
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
func (t *Task) reportAdvancedRsyncErrorHeuristics() ([]string, error) {
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

// migrationOperationStatus defines status of one Rsync operation
type migrationOperationStatus struct {
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
func (e *migrationOperationStatus) HasErrors() bool {
	return len(e.errors) > 0
}

func (e *migrationOperationStatus) AddError(err error) {
	if e.errors == nil {
		e.errors = make([]error, 0)
	}
	e.errors = append(e.errors, err)
}

// migrationOperationStatusList managed list of all ongoing Rsync operations
type migrationOperationStatusList struct {
	// ops list of operations
	ops []migrationOperationStatus
}

func (r *migrationOperationStatusList) Add(s migrationOperationStatus) {
	if r.ops == nil {
		r.ops = make([]migrationOperationStatus, 0)
	}
	r.ops = append(r.ops, s)
}

// AllCompleted checks whether all of the Rsync attempts are in a terminal state
// If true, reconcile can move to next phase
func (r *migrationOperationStatusList) AllCompleted() bool {
	for _, attempt := range r.ops {
		if attempt.pending || attempt.running || attempt.HasErrors() {
			return false
		}
	}
	return true
}

// AnyErrored checks whether any of the operation is resulting in an error
func (r *migrationOperationStatusList) AnyErrored() bool {
	for _, attempt := range r.ops {
		if attempt.HasErrors() {
			return true
		}
	}
	return false
}

// Failed returns number of failed operations
func (r *migrationOperationStatusList) Failed() int {
	i := 0
	for _, attempt := range r.ops {
		if attempt.failed {
			i += 1
		}
	}
	return i
}

// Succeeded returns number of failed operations
func (r *migrationOperationStatusList) Succeeded() int {
	i := 0
	for _, attempt := range r.ops {
		if attempt.succeeded {
			i += 1
		}
	}
	return i
}

// Pending returns number of pending operations
func (r *migrationOperationStatusList) Pending() int {
	i := 0
	for _, attempt := range r.ops {
		if attempt.pending {
			i += 1
		}
	}
	return i
}

// Running returns number of running operations
func (r *migrationOperationStatusList) Running() int {
	i := 0
	for _, attempt := range r.ops {
		if attempt.running {
			i += 1
		}
	}
	return i
}

// getAllPodsForOperation returns all pods matching given Rsync operation
func (t *Task) getAllPodsForOperation(client compat.Client, operation migapi.RsyncOperation) (*corev1.PodList, error) {
	podList := corev1.PodList{}
	pvcNamespace, pvcName := operation.GetPVDetails()
	labels := GetRsyncPodSelector(pvcName)
	err := client.List(context.TODO(),
		&podList,
		k8sclient.InNamespace(pvcNamespace),
		k8sclient.MatchingLabels(labels),
	)
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
	var mostRecentPod *corev1.Pod = nil
	for i := range podList.Items {
		// if expected attempt label is not found on the pod or its value is not an integer,
		// there is no way to associate this pod with an Rsync attempt we made, we skip this pod
		pod := podList.Items[i]
		if val, exists := pod.Labels[RsyncAttemptLabel]; !exists {
			continue
		} else if _, err := strconv.Atoi(val); err != nil {
			continue
		}
		if mostRecentPod == nil {
			mostRecentPod = &pod
		} else if pod.CreationTimestamp.After(mostRecentPod.CreationTimestamp.Time) {
			mostRecentPod = &pod
		}
	}
	return mostRecentPod, nil
}

// updateOperationStatus given a Rsync Pod and operation status, updates operation status with pod status
func updateOperationStatus(status *migrationOperationStatus, pod *corev1.Pod) {
	switch pod.Status.Phase {
	case corev1.PodFailed:
		status.failed = true
	case corev1.PodSucceeded:
		status.succeeded = true
	case corev1.PodRunning:
		status.running = true
	case corev1.PodPending, corev1.PodUnknown:
		status.pending = true
	}
}

// GetRsyncPodSelector returns pod selector used to identify sibling Rsync pods
func GetRsyncPodSelector(pvcName string) map[string]string {
	selector := make(map[string]string, 1)
	selector[migapi.RsyncPodIdentityLabel] = getMD5Hash(pvcName)
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

type RsyncBwLimit int

func (r RsyncBwLimit) ApplyTo(opts *rsynctransfer.TransferOptions) error {
	val := int(r)
	if val < 0 {
		opts.BwLimit = nil
	}
	opts.BwLimit = &val
	return nil
}

type HardLinks bool

func (h HardLinks) ApplyTo(opts *rsynctransfer.TransferOptions) error {
	opts.HardLinks = bool(h)
	return nil
}

type Partial bool

func (p Partial) ApplyTo(opts *rsynctransfer.TransferOptions) error {
	opts.Partial = bool(p)
	return nil
}

type ExtraOpts []string

func (e ExtraOpts) ApplyTo(opts *rsynctransfer.TransferOptions) error {
	validatedOptions := []string{}
	for _, opt := range e {
		r := regexp.MustCompile(`^\-{1,2}([a-z0-9]+\-){0,}?[a-z0-9]+(=.*)?$`)
		if r.MatchString(opt) {
			validatedOptions = append(validatedOptions, opt)
		} else {
			log.Info("Invalid Rsync extra option passed", "option", opt)
		}
	}
	opts.Extras = append(opts.Extras, validatedOptions...)
	return nil
}

func isPSAEnforced(client compat.Client) bool {
	return client.MinorVersion() >= 24
}

// getEndpoints returns correct endpoint objects as per app settings
func (t *Task) getEndpoints(client client.Client, namespace string) ([]endpoint.Endpoint, error) {
	res := []endpoint.Endpoint{}
	switch t.EndpointType {
	case migapi.ClusterIP, migapi.NodePort:
		endpoint, err := svcendpoint.GetEndpointFromKubeObjects(client, types.NamespacedName{
			Name:      DirectVolumeMigrationRsyncTransferSvc,
			Namespace: namespace,
		})
		if err != nil {
			return nil, liberr.Wrap(err)
		}
		res = append(res, endpoint)
		endpoint, err = svcendpoint.GetEndpointFromKubeObjects(client, types.NamespacedName{
			Name:      DirectVolumeMigrationRsyncTransferSvcBlock,
			Namespace: namespace,
		})
		if err != nil {
			return nil, liberr.Wrap(err)
		}
		res = append(res, endpoint)
		return res, nil
	default:
		endpoint, err := routeendpoint.GetEndpointFromKubeObjects(client, types.NamespacedName{
			Name:      DirectVolumeMigrationRsyncTransferRoute,
			Namespace: namespace,
		})
		if err != nil {
			return nil, liberr.Wrap(err)
		}
		res = append(res, endpoint)
		endpoint, err = routeendpoint.GetEndpointFromKubeObjects(client, types.NamespacedName{
			Name:      DirectVolumeMigrationRsyncTransferRouteBlock,
			Namespace: namespace,
		})
		if err != nil {
			return nil, liberr.Wrap(err)
		}
		res = append(res, endpoint)
		return res, nil
	}
}

// getServiceType returns endpoint service type based on settings
func (t *Task) getServiceType() corev1.ServiceType {
	switch t.EndpointType {
	case migapi.NodePort:
		return corev1.ServiceTypeNodePort
	case migapi.ClusterIP:
		return corev1.ServiceTypeClusterIP
	}
	return corev1.ServiceTypeNodePort
}

// getWorkerNodeHostnames returns hostnames of worker nodes
func getWorkerNodeHostnames(client client.Client) ([]string, error) {
	nodeList := corev1.NodeList{}
	hostnames := []string{}
	err := client.List(context.TODO(), &nodeList, k8sclient.MatchingLabels{
		"node-role.kubernetes.io/worker": "",
	})
	if err != nil {
		return hostnames, liberr.Wrap(err)
	}
	for _, node := range nodeList.Items {
		hostname := ""
		for _, address := range node.Status.Addresses {
			switch address.Type {
			case corev1.NodeInternalIP, corev1.NodeInternalDNS, corev1.NodeHostName:
				hostname = address.Address
			}
			if hostname != "" {
				hostnames = append(hostnames, hostname)
				break
			}
		}
	}
	return hostnames, nil
}

// getNodeHostnameAtRandom returns a random hostname from a list of hostnames
func getNodeHostnameAtRandom(hostnames []string) string {
	if len(hostnames) == 0 {
		return ""
	}
	return hostnames[rand.Int()%len(hostnames)]
}

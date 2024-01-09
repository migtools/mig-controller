package directvolumemigration

import (
	"context"
	"fmt"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/go-logr/logr"
	testlog "github.com/go-logr/logr/testr"
	transferservice "github.com/konveyor/crane-lib/state_transfer/endpoint/service"
	transfer "github.com/konveyor/crane-lib/state_transfer/transfer"
	"github.com/konveyor/crane-lib/state_transfer/transport"
	migapi "github.com/konveyor/mig-controller/pkg/apis/migration/v1alpha1"
	"github.com/konveyor/mig-controller/pkg/compat"
	fakecompat "github.com/konveyor/mig-controller/pkg/compat/fake"
	configv1 "github.com/openshift/api/config/v1"
	routev1 "github.com/openshift/api/route/v1"
	corev1 "k8s.io/api/core/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/apimachinery/pkg/util/rand"
	"sigs.k8s.io/controller-runtime/pkg/client"
	k8sclient "sigs.k8s.io/controller-runtime/pkg/client"
)

func getTestRsyncPodForPVC(podName string, pvcName string, ns string, attemptNo string, timestamp time.Time) *corev1.Pod {
	return &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			CreationTimestamp: metav1.Time{
				Time: timestamp,
			},
			Name:      podName,
			Namespace: ns,
			Labels: map[string]string{
				migapi.RsyncPodIdentityLabel: getMD5Hash(pvcName),
				RsyncAttemptLabel:            attemptNo,
			},
		},
	}
}

func getTestRsyncPodWithStatusForPVC(podName string, pvcName string, ns string, attemptNo string, phase corev1.PodPhase, tstamp time.Time) *corev1.Pod {
	pod := getTestRsyncPodForPVC(podName, pvcName, ns, attemptNo, tstamp)
	pod.Status = corev1.PodStatus{
		Phase: phase,
	}
	return pod
}

func getTestRsyncOperationStatus(pvcName string, ns string, attemptNo int, succeeded bool, failed bool) *migapi.RsyncOperation {
	return &migapi.RsyncOperation{
		PVCReference: &corev1.ObjectReference{
			Name:      pvcName,
			Namespace: ns,
		},
		CurrentAttempt: attemptNo,
		Failed:         failed,
		Succeeded:      succeeded,
	}
}

func arePodsEqual(p1 *corev1.Pod, p2 *corev1.Pod) bool {
	if p1 == nil && p2 == nil {
		return true
	}
	if p1 == nil || p2 == nil {
		return false
	}
	if p1.Name == p2.Name && p1.Namespace == p2.Namespace {
		return true
	}
	return false
}

func getFakeCompatClientWithSubdomain(obj ...k8sclient.Object) compat.Client {
	clusterConfig := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{Name: "migration-cluster-config", Namespace: migapi.OpenshiftMigrationNamespace},
		Data: map[string]string{
			"RSYNC_PRIVILEGED":       "false",
			"RSYNC_SUPER_PRIVILEGED": "false",
			"CLUSTER_SUBDOMAIN":      "test.domain",
		},
	}
	controllerConfig := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{Name: "migration-controller", Namespace: migapi.OpenshiftMigrationNamespace},
	}
	obj = append(obj, clusterConfig)
	obj = append(obj, controllerConfig)
	client, _ := fakecompat.NewFakeClient(obj...)
	return client
}

func getFakeCompatClient(obj ...k8sclient.Object) compat.Client {
	clusterConfig := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{Name: "migration-cluster-config", Namespace: migapi.OpenshiftMigrationNamespace},
		Data: map[string]string{
			"RSYNC_PRIVILEGED":       "false",
			"RSYNC_SUPER_PRIVILEGED": "false",
		},
	}
	controllerConfig := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{Name: "migration-controller", Namespace: migapi.OpenshiftMigrationNamespace},
	}
	obj = append(obj, clusterConfig)
	obj = append(obj, controllerConfig)
	client, _ := fakecompat.NewFakeClient(obj...)
	return client
}

func getFakeCompatClientWithVersion(major int, minor int, obj ...k8sclient.Object) compat.Client {
	clusterConfig := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{Name: "migration-cluster-config", Namespace: migapi.OpenshiftMigrationNamespace},
		Data:       map[string]string{"RSYNC_PRIVILEGED": "false", "RSYNC_SUPER_PRIVILEGED": "false"},
	}
	controllerConfig := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{Name: "migration-controller", Namespace: migapi.OpenshiftMigrationNamespace},
	}
	obj = append(obj, clusterConfig)
	obj = append(obj, controllerConfig)
	client, _ := fakecompat.NewFakeClientWithVersion(major, minor, obj...)
	return client
}

func Test_hasAllRsyncClientPodsTimedOut(t *testing.T) {
	type fields struct {
		Client k8sclient.Client
		Owner  *migapi.DirectVolumeMigration
	}
	tests := []struct {
		name    string
		fields  fields
		want    bool
		wantErr bool
	}{
		{
			name: "both rsync clients pods succeeded",
			fields: fields{
				Client: getFakeCompatClient(&migapi.DirectVolumeMigrationProgress{
					ObjectMeta: metav1.ObjectMeta{
						Name:      getMD5Hash("test" + "pvc-0" + "foo"),
						Namespace: migapi.OpenshiftMigrationNamespace,
					},
					Spec:   migapi.DirectVolumeMigrationProgressSpec{},
					Status: migapi.DirectVolumeMigrationProgressStatus{RsyncPodStatus: migapi.RsyncPodStatus{PodPhase: corev1.PodSucceeded, ContainerElapsedTime: &metav1.Duration{Duration: time.Second * 20}}},
				}, &migapi.DirectVolumeMigrationProgress{
					ObjectMeta: metav1.ObjectMeta{
						Name:      getMD5Hash("test" + "pvc-1" + "foo"),
						Namespace: migapi.OpenshiftMigrationNamespace,
					},
					Spec:   migapi.DirectVolumeMigrationProgressSpec{},
					Status: migapi.DirectVolumeMigrationProgressStatus{RsyncPodStatus: migapi.RsyncPodStatus{PodPhase: corev1.PodSucceeded, ContainerElapsedTime: &metav1.Duration{Duration: time.Second * 20}}},
				}),
				Owner: &migapi.DirectVolumeMigration{
					ObjectMeta: metav1.ObjectMeta{Name: "test"},
					Spec: migapi.DirectVolumeMigrationSpec{
						PersistentVolumeClaims: []migapi.PVCToMigrate{
							{ObjectReference: &corev1.ObjectReference{Namespace: "foo", Name: "pvc-0"}},
							{ObjectReference: &corev1.ObjectReference{Namespace: "foo", Name: "pvc-1"}},
						},
					},
				},
			},
			want:    false,
			wantErr: false,
		},
		{
			name: "both rsync clients failed with 20 seconds",
			fields: fields{
				Client: getFakeCompatClient(&migapi.DirectVolumeMigrationProgress{
					ObjectMeta: metav1.ObjectMeta{
						Name:      getMD5Hash("test" + "pvc-0" + "foo"),
						Namespace: migapi.OpenshiftMigrationNamespace,
					},
					Spec:   migapi.DirectVolumeMigrationProgressSpec{},
					Status: migapi.DirectVolumeMigrationProgressStatus{RsyncPodStatus: migapi.RsyncPodStatus{PodPhase: corev1.PodFailed, ContainerElapsedTime: &metav1.Duration{Duration: time.Second * 20}}},
				}, &migapi.DirectVolumeMigrationProgress{
					ObjectMeta: metav1.ObjectMeta{
						Name:      getMD5Hash("test" + "pvc-1" + "foo"),
						Namespace: migapi.OpenshiftMigrationNamespace,
					},
					Spec:   migapi.DirectVolumeMigrationProgressSpec{},
					Status: migapi.DirectVolumeMigrationProgressStatus{RsyncPodStatus: migapi.RsyncPodStatus{PodPhase: corev1.PodFailed, ContainerElapsedTime: &metav1.Duration{Duration: time.Second * 20}}},
				}),
				Owner: &migapi.DirectVolumeMigration{
					ObjectMeta: metav1.ObjectMeta{Name: "test"},
					Spec: migapi.DirectVolumeMigrationSpec{
						PersistentVolumeClaims: []migapi.PVCToMigrate{
							{ObjectReference: &corev1.ObjectReference{Namespace: "foo", Name: "pvc-0"}},
							{ObjectReference: &corev1.ObjectReference{Namespace: "foo", Name: "pvc-1"}},
						},
					},
				},
			},
			want:    true,
			wantErr: false,
		},
		{
			name: "one rsync client pod failed with 20 second timeout other succeeded",
			fields: fields{
				Client: getFakeCompatClient(&migapi.DirectVolumeMigrationProgress{
					ObjectMeta: metav1.ObjectMeta{
						Name:      getMD5Hash("test" + "pvc-0" + "foo"),
						Namespace: migapi.OpenshiftMigrationNamespace,
					},
					Spec:   migapi.DirectVolumeMigrationProgressSpec{},
					Status: migapi.DirectVolumeMigrationProgressStatus{RsyncPodStatus: migapi.RsyncPodStatus{PodPhase: corev1.PodSucceeded, ContainerElapsedTime: &metav1.Duration{Duration: time.Second * 20}}},
				}, &migapi.DirectVolumeMigrationProgress{
					ObjectMeta: metav1.ObjectMeta{
						Name:      getMD5Hash("test" + "pvc-1" + "foo"),
						Namespace: migapi.OpenshiftMigrationNamespace,
					},
					Spec:   migapi.DirectVolumeMigrationProgressSpec{},
					Status: migapi.DirectVolumeMigrationProgressStatus{RsyncPodStatus: migapi.RsyncPodStatus{PodPhase: corev1.PodFailed, ContainerElapsedTime: &metav1.Duration{Duration: time.Second * 20}}},
				}),
				Owner: &migapi.DirectVolumeMigration{
					ObjectMeta: metav1.ObjectMeta{Name: "test"},
					Spec: migapi.DirectVolumeMigrationSpec{
						PersistentVolumeClaims: []migapi.PVCToMigrate{
							{ObjectReference: &corev1.ObjectReference{Namespace: "foo", Name: "pvc-0"}},
							{ObjectReference: &corev1.ObjectReference{Namespace: "foo", Name: "pvc-1"}},
						},
					},
				},
			},
			want:    false,
			wantErr: false,
		},
		{
			name: "one rsync client pod failed with 20.49 and other with 20.50 second, expect false",
			fields: fields{
				Client: getFakeCompatClient(&migapi.DirectVolumeMigrationProgress{
					ObjectMeta: metav1.ObjectMeta{
						Name:      getMD5Hash("test" + "pvc-0" + "foo"),
						Namespace: migapi.OpenshiftMigrationNamespace,
					},
					Spec:   migapi.DirectVolumeMigrationProgressSpec{},
					Status: migapi.DirectVolumeMigrationProgressStatus{RsyncPodStatus: migapi.RsyncPodStatus{PodPhase: corev1.PodFailed, ContainerElapsedTime: &metav1.Duration{Duration: time.Millisecond * 20490}}},
				}, &migapi.DirectVolumeMigrationProgress{
					ObjectMeta: metav1.ObjectMeta{
						Name:      getMD5Hash("test" + "pvc-1" + "foo"),
						Namespace: migapi.OpenshiftMigrationNamespace,
					},
					Spec:   migapi.DirectVolumeMigrationProgressSpec{},
					Status: migapi.DirectVolumeMigrationProgressStatus{RsyncPodStatus: migapi.RsyncPodStatus{PodPhase: corev1.PodFailed, ContainerElapsedTime: &metav1.Duration{Duration: time.Millisecond * 20500}}},
				}),
				Owner: &migapi.DirectVolumeMigration{
					ObjectMeta: metav1.ObjectMeta{Name: "test"},
					Spec: migapi.DirectVolumeMigrationSpec{
						PersistentVolumeClaims: []migapi.PVCToMigrate{
							{ObjectReference: &corev1.ObjectReference{Namespace: "foo", Name: "pvc-0"}},
							{ObjectReference: &corev1.ObjectReference{Namespace: "foo", Name: "pvc-1"}},
						},
					},
				},
			},
			want:    false,
			wantErr: false,
		},
		{
			name: "one rsync client pod failed with 20.49 and other with 20.45 second, expect true",
			fields: fields{
				Client: getFakeCompatClient(&migapi.DirectVolumeMigrationProgress{
					ObjectMeta: metav1.ObjectMeta{
						Name:      getMD5Hash("test" + "pvc-0" + "foo"),
						Namespace: migapi.OpenshiftMigrationNamespace,
					},
					Spec:   migapi.DirectVolumeMigrationProgressSpec{},
					Status: migapi.DirectVolumeMigrationProgressStatus{RsyncPodStatus: migapi.RsyncPodStatus{PodPhase: corev1.PodFailed, ContainerElapsedTime: &metav1.Duration{Duration: time.Millisecond * 20490}}},
				}, &migapi.DirectVolumeMigrationProgress{
					ObjectMeta: metav1.ObjectMeta{
						Name:      getMD5Hash("test" + "pvc-1" + "foo"),
						Namespace: migapi.OpenshiftMigrationNamespace,
					},
					Spec:   migapi.DirectVolumeMigrationProgressSpec{},
					Status: migapi.DirectVolumeMigrationProgressStatus{RsyncPodStatus: migapi.RsyncPodStatus{PodPhase: corev1.PodFailed, ContainerElapsedTime: &metav1.Duration{Duration: time.Millisecond * 20450}}},
				}),
				Owner: &migapi.DirectVolumeMigration{
					ObjectMeta: metav1.ObjectMeta{Name: "test"},
					Spec: migapi.DirectVolumeMigrationSpec{
						PersistentVolumeClaims: []migapi.PVCToMigrate{
							{ObjectReference: &corev1.ObjectReference{Namespace: "foo", Name: "pvc-0"}},
							{ObjectReference: &corev1.ObjectReference{Namespace: "foo", Name: "pvc-1"}},
						},
					},
				},
			},
			want:    true,
			wantErr: false,
		},
		{
			name: "both rsync client pods failed with nil elapsad time",
			fields: fields{
				Client: getFakeCompatClient(&migapi.DirectVolumeMigrationProgress{
					ObjectMeta: metav1.ObjectMeta{
						Name:      getMD5Hash("test" + "pvc-0" + "foo"),
						Namespace: migapi.OpenshiftMigrationNamespace,
					},
					Spec:   migapi.DirectVolumeMigrationProgressSpec{},
					Status: migapi.DirectVolumeMigrationProgressStatus{RsyncPodStatus: migapi.RsyncPodStatus{PodPhase: corev1.PodFailed}},
				}, &migapi.DirectVolumeMigrationProgress{
					ObjectMeta: metav1.ObjectMeta{
						Name:      getMD5Hash("test" + "pvc-1" + "foo"),
						Namespace: migapi.OpenshiftMigrationNamespace,
					},
					Spec:   migapi.DirectVolumeMigrationProgressSpec{},
					Status: migapi.DirectVolumeMigrationProgressStatus{RsyncPodStatus: migapi.RsyncPodStatus{PodPhase: corev1.PodFailed}},
				}),
				Owner: &migapi.DirectVolumeMigration{
					ObjectMeta: metav1.ObjectMeta{Name: "test"},
					Spec: migapi.DirectVolumeMigrationSpec{
						PersistentVolumeClaims: []migapi.PVCToMigrate{
							{ObjectReference: &corev1.ObjectReference{Namespace: "foo", Name: "pvc-0"}},
							{ObjectReference: &corev1.ObjectReference{Namespace: "foo", Name: "pvc-1"}},
						},
					},
				},
			},
			want:    false,
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			task := Task{
				Client: tt.fields.Client,
				Owner:  tt.fields.Owner,
			}
			got, err := task.hasAllRsyncClientPodsTimedOut()
			if (err != nil) != tt.wantErr {
				t.Errorf("hasAllRsyncClientPodsTimedOut() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if got != tt.want {
				t.Errorf("hasAllRsyncClientPodsTimedOut() got = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_isAllRsyncClientPodsNoRouteToHost(t *testing.T) {
	ten := int32(10)
	twelve := int32(12)
	type fields struct {
		Owner  *migapi.DirectVolumeMigration
		Client k8sclient.Client
	}
	tests := []struct {
		name    string
		fields  fields
		want    bool
		wantErr bool
	}{
		{
			name: "all client pods failed with no route to host",
			fields: fields{
				Owner: &migapi.DirectVolumeMigration{
					ObjectMeta: metav1.ObjectMeta{Name: "test"},
					Spec: migapi.DirectVolumeMigrationSpec{
						PersistentVolumeClaims: []migapi.PVCToMigrate{
							{ObjectReference: &corev1.ObjectReference{Namespace: "foo", Name: "pvc-0"}},
							{ObjectReference: &corev1.ObjectReference{Namespace: "foo", Name: "pvc-1"}},
						},
					},
				},
				Client: getFakeCompatClient(&migapi.DirectVolumeMigrationProgress{
					ObjectMeta: metav1.ObjectMeta{
						Name:      getMD5Hash("test" + "pvc-0" + "foo"),
						Namespace: migapi.OpenshiftMigrationNamespace,
					},
					Spec: migapi.DirectVolumeMigrationProgressSpec{},
					Status: migapi.DirectVolumeMigrationProgressStatus{
						RsyncPodStatus: migapi.RsyncPodStatus{
							PodPhase:             corev1.PodFailed,
							ContainerElapsedTime: &metav1.Duration{Duration: time.Second * 2},
							ExitCode:             &ten,
							LogMessage: strings.Join([]string{
								"      2021/02/10 23:31:18 [1] rsync: failed to connect to 172.30.12.121 (172.30.12.121): No route to host (113)",
								"      2021/02/10 23:31:18 [1] rsync error: error in socket IO (code 10) at clientserver.c(127) [sender=3.1.3]",
								"      rsync: failed to connect to 172.30.12.121 (172.30.12.121): No route to host (113)",
								"rsync, error: error in socket IO (code 10) at clientserver.c(127) [sender = 3.1.3]"}, "\n"),
						}},
				}, &migapi.DirectVolumeMigrationProgress{
					ObjectMeta: metav1.ObjectMeta{
						Name:      getMD5Hash("test" + "pvc-1" + "foo"),
						Namespace: migapi.OpenshiftMigrationNamespace,
					},
					Spec: migapi.DirectVolumeMigrationProgressSpec{},
					Status: migapi.DirectVolumeMigrationProgressStatus{
						RsyncPodStatus: migapi.RsyncPodStatus{
							PodPhase:             corev1.PodFailed,
							ContainerElapsedTime: &metav1.Duration{Duration: time.Second * 2},
							ExitCode:             &ten,
							LogMessage: strings.Join([]string{
								"      2021/02/10 23:31:18 [1] rsync: failed to connect to 172.30.12.121 (172.30.12.121): No route to host (113)",
								"      2021/02/10 23:31:18 [1] rsync error: error in socket IO (code 10) at clientserver.c(127) [sender=3.1.3]",
								"      rsync: failed to connect to 172.30.12.121 (172.30.12.121): No route to host (113)",
								"rsync, error: error in socket IO (code 10) at clientserver.c(127) [sender = 3.1.3]"}, "\n"),
						},
					}}),
			},
			want:    true,
			wantErr: false,
		},
		{
			name: "one client fails with exit code 10, one with 12",
			fields: fields{
				Owner: &migapi.DirectVolumeMigration{
					ObjectMeta: metav1.ObjectMeta{Name: "test"},
					Spec: migapi.DirectVolumeMigrationSpec{
						PersistentVolumeClaims: []migapi.PVCToMigrate{
							{ObjectReference: &corev1.ObjectReference{Namespace: "foo", Name: "pvc-0"}},
							{ObjectReference: &corev1.ObjectReference{Namespace: "foo", Name: "pvc-1"}},
						},
					},
				},
				Client: getFakeCompatClient(&migapi.DirectVolumeMigrationProgress{
					ObjectMeta: metav1.ObjectMeta{
						Name:      getMD5Hash("test" + "pvc-0" + "foo"),
						Namespace: migapi.OpenshiftMigrationNamespace,
					},
					Spec: migapi.DirectVolumeMigrationProgressSpec{},
					Status: migapi.DirectVolumeMigrationProgressStatus{
						RsyncPodStatus: migapi.RsyncPodStatus{
							PodPhase:             corev1.PodFailed,
							ContainerElapsedTime: &metav1.Duration{Duration: time.Second * 2},
							ExitCode:             &ten,
							LogMessage: strings.Join([]string{
								"      2021/02/10 23:31:18 [1] rsync: failed to connect to 172.30.12.121 (172.30.12.121): No route to host (113)",
								"      2021/02/10 23:31:18 [1] rsync error: error in socket IO (code 10) at clientserver.c(127) [sender=3.1.3]",
								"      rsync: failed to connect to 172.30.12.121 (172.30.12.121): No route to host (113)",
								"rsync, error: error in socket IO (code 10) at clientserver.c(127) [sender = 3.1.3]"}, "\n"),
						}},
				}, &migapi.DirectVolumeMigrationProgress{
					ObjectMeta: metav1.ObjectMeta{
						Name:      getMD5Hash("test" + "pvc-1" + "foo"),
						Namespace: migapi.OpenshiftMigrationNamespace,
					},
					Spec: migapi.DirectVolumeMigrationProgressSpec{},
					Status: migapi.DirectVolumeMigrationProgressStatus{
						RsyncPodStatus: migapi.RsyncPodStatus{
							PodPhase:             corev1.PodFailed,
							ContainerElapsedTime: &metav1.Duration{Duration: time.Second * 2},
							ExitCode:             &twelve,
							LogMessage: strings.Join([]string{
								"      2021/02/10 23:31:18 [1] rsync: failed to connect to 172.30.12.121 (172.30.12.121): No route to host (113)",
								"      2021/02/10 23:31:18 [1] rsync error: error in socket IO (code 10) at clientserver.c(127) [sender=3.1.3]",
								"      rsync: failed to connect to 172.30.12.121 (172.30.12.121): No route to host (113)",
								"rsync, error: error in socket IO (code 10) at clientserver.c(127) [sender = 3.1.3]"}, "\n"),
						}},
				}),
			},
			want:    false,
			wantErr: false,
		},
		{
			name: "one client fails with exit code 10, and connection reset error",
			fields: fields{
				Owner: &migapi.DirectVolumeMigration{
					ObjectMeta: metav1.ObjectMeta{Name: "test"},
					Spec: migapi.DirectVolumeMigrationSpec{
						PersistentVolumeClaims: []migapi.PVCToMigrate{
							{ObjectReference: &corev1.ObjectReference{Namespace: "foo", Name: "pvc-0"}},
							{ObjectReference: &corev1.ObjectReference{Namespace: "foo", Name: "pvc-1"}},
						},
					},
				},
				Client: getFakeCompatClient(&migapi.DirectVolumeMigrationProgress{
					ObjectMeta: metav1.ObjectMeta{
						Name:      getMD5Hash("test" + "pvc-0" + "foo"),
						Namespace: migapi.OpenshiftMigrationNamespace,
					},
					Spec: migapi.DirectVolumeMigrationProgressSpec{},
					Status: migapi.DirectVolumeMigrationProgressStatus{
						RsyncPodStatus: migapi.RsyncPodStatus{
							PodPhase:             corev1.PodFailed,
							ContainerElapsedTime: &metav1.Duration{Duration: time.Second * 2},
							ExitCode:             &ten,
							LogMessage: strings.Join([]string{
								"      2021/02/10 23:31:18 [1] rsync: failed to connect to: connection reset error",
								"      2021/02/10 23:31:18 [1] rsync error: error in socket IO (code 10) at clientserver.c(127) [sender=3.1.3]",
								"      rsync: failed to connect to 172.30.12.121 (172.30.12.121): connection reset error (113)",
								"rsync, error: error in socket IO (code 10) at clientserver.c(127) [sender = 3.1.3]"}, "\n"),
						}},
				}, &migapi.DirectVolumeMigrationProgress{
					ObjectMeta: metav1.ObjectMeta{
						Name:      getMD5Hash("test" + "pvc-1" + "foo"),
						Namespace: migapi.OpenshiftMigrationNamespace,
					},
					Spec: migapi.DirectVolumeMigrationProgressSpec{},
					Status: migapi.DirectVolumeMigrationProgressStatus{
						RsyncPodStatus: migapi.RsyncPodStatus{
							PodPhase:             corev1.PodFailed,
							ContainerElapsedTime: &metav1.Duration{Duration: time.Second * 2},
							ExitCode:             &ten,
							LogMessage: strings.Join([]string{
								"      2021/02/10 23:31:18 [1] rsync: failed to connect to 172.30.12.121 (172.30.12.121): No route to host (113)",
								"      2021/02/10 23:31:18 [1] rsync error: error in socket IO (code 10) at clientserver.c(127) [sender=3.1.3]",
								"      rsync: failed to connect to 172.30.12.121 (172.30.12.121): No route to host (113)",
								"rsync, error: error in socket IO (code 10) at clientserver.c(127) [sender = 3.1.3]"}, "\n"),
						}},
				}),
			},
			want:    false,
			wantErr: false,
		},
		{
			name: "both rsync client pods failed with nil elapsad time",
			fields: fields{
				Owner: &migapi.DirectVolumeMigration{
					ObjectMeta: metav1.ObjectMeta{Name: "test"},
					Spec: migapi.DirectVolumeMigrationSpec{
						PersistentVolumeClaims: []migapi.PVCToMigrate{
							{ObjectReference: &corev1.ObjectReference{Namespace: "foo", Name: "pvc-0"}},
							{ObjectReference: &corev1.ObjectReference{Namespace: "foo", Name: "pvc-1"}},
						},
					},
				},
				Client: getFakeCompatClient(&migapi.DirectVolumeMigrationProgress{
					ObjectMeta: metav1.ObjectMeta{
						Name:      getMD5Hash("test" + "pvc-0" + "foo"),
						Namespace: migapi.OpenshiftMigrationNamespace,
					},
					Spec:   migapi.DirectVolumeMigrationProgressSpec{},
					Status: migapi.DirectVolumeMigrationProgressStatus{RsyncPodStatus: migapi.RsyncPodStatus{PodPhase: corev1.PodFailed}},
				}, &migapi.DirectVolumeMigrationProgress{
					ObjectMeta: metav1.ObjectMeta{
						Name:      getMD5Hash("test" + "pvc-1" + "foo"),
						Namespace: migapi.OpenshiftMigrationNamespace,
					},
					Spec:   migapi.DirectVolumeMigrationProgressSpec{},
					Status: migapi.DirectVolumeMigrationProgressStatus{RsyncPodStatus: migapi.RsyncPodStatus{PodPhase: corev1.PodFailed}},
				}),
			},
			want:    false,
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			task := Task{
				Owner:  tt.fields.Owner,
				Client: tt.fields.Client,
			}
			got, err := task.isAllRsyncClientPodsNoRouteToHost()
			if (err != nil) != tt.wantErr {
				t.Errorf("isAllRsyncClientPodsNoRouteToHost() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if got != tt.want {
				t.Errorf("isAllRsyncClientPodsNoRouteToHost() got = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestTask_getLatestPodForOperation(t *testing.T) {
	type fields struct {
		Log logr.Logger
	}
	type args struct {
		client    compat.Client
		operation *migapi.RsyncOperation
	}
	tests := []struct {
		name    string
		fields  fields
		args    args
		want    *corev1.Pod
		wantErr bool
	}{
		{
			name: "given zero rsync pods in the source namespace, should return nil",
			fields: fields{
				Log: testlog.New(t).WithName("rsync-operation-test"),
			},
			args: args{
				client:    getFakeCompatClient(),
				operation: getTestRsyncOperationStatus("test-1", "test-ns", 1, false, false),
			},
			want:    nil,
			wantErr: false,
		},
		{
			name: "given one rsync pod for given pvc, should return that one pod",
			args: args{
				client: getFakeCompatClient(
					getTestRsyncPodForPVC("pod-1", "pvc-1", "ns", "1", time.Now())),
				operation: getTestRsyncOperationStatus("pvc-1", "ns", 1, false, false),
			},
			want:    getTestRsyncPodForPVC("pod-1", "pvc-1", "ns", "1", time.Now()),
			wantErr: false,
		},
		{
			name: "given more than one pods present for given pvc, should return the most recent pod",
			fields: fields{
				Log: testlog.New(t).WithName("rsync-operation-test"),
			},
			args: args{
				client: getFakeCompatClient(
					getTestRsyncPodForPVC("pod-1", "pvc-1", "ns", "1", time.Now()),
					getTestRsyncPodForPVC("pod-2", "pvc-1", "ns", "2", time.Now().Add(time.Second*20)),
					getTestRsyncPodForPVC("pod-3", "pvc-1", "ns", "2", time.Now().Add(time.Second*30)),
					getTestRsyncPodForPVC("pod-4", "pvc-1", "ns", "2", time.Now().Add(time.Second*40))),
				operation: getTestRsyncOperationStatus("pvc-1", "ns", 1, false, false),
			},
			want:    getTestRsyncPodForPVC("pod-4", "pvc-1", "ns", "2", time.Now()),
			wantErr: false,
		},
		{
			name: "given more than one pods present for given pvc with some of them with non-integer attempt labels, should return the most recent pod with the correct label",
			fields: fields{
				Log: testlog.New(t).WithName("rsync-operation-test"),
			},
			args: args{
				client: getFakeCompatClient(
					getTestRsyncPodForPVC("pod-1", "pvc-1", "ns", "1", time.Now()),
					getTestRsyncPodForPVC("pod-2", "pvc-1", "ns", "2", time.Now().Add(time.Second*20)),
					getTestRsyncPodForPVC("pod-3", "pvc-1", "ns", "2", time.Now().Add(time.Second*30)),
					getTestRsyncPodForPVC("pod-4", "pvc-1", "ns", "ab", time.Now().Add(time.Second*40))),
				operation: getTestRsyncOperationStatus("pvc-1", "ns", 1, false, false),
			},
			want:    getTestRsyncPodForPVC("pod-3", "pvc-1", "ns", "2", time.Now()),
			wantErr: false,
		},
		{
			name: "given more than one pods present for different PVCs, should return pod corresponding to the most recent attempt for the correct PVC",
			fields: fields{
				Log: testlog.New(t).WithName("rsync-operation-test"),
			},
			args: args{
				client: getFakeCompatClient(
					getTestRsyncPodForPVC("pod-1", "pvc-1", "ns", "1", time.Now()),
					getTestRsyncPodForPVC("pod-2", "pvc-1", "ns", "2", time.Now().Add(time.Second*20)),
					getTestRsyncPodForPVC("pod-3", "pvc-2", "ns", "2", time.Now()),
					getTestRsyncPodForPVC("pod-4", "pvc-2", "ns", "2", time.Now().Add(time.Second*20))),
				operation: getTestRsyncOperationStatus("pvc-1", "ns", 1, false, false),
			},
			want:    getTestRsyncPodForPVC("pod-2", "pvc-1", "ns", "1", time.Now()),
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tr := &Task{
				Log: tt.fields.Log,
			}
			got, err := tr.getLatestPodForOperation(tt.args.client, *tt.args.operation)
			if (err != nil) != tt.wantErr {
				t.Errorf("Task.getLatestPodForOperation() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !arePodsEqual(got, tt.want) {
				t.Errorf("Task.getLatestPodForOperation() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestTask_processRsyncOperationStatus(t *testing.T) {
	type fields struct {
		Log    logr.Logger
		Client k8sclient.Client
		Owner  *migapi.DirectVolumeMigration
	}
	type args struct {
		status                  rsyncClientOperationStatusList
		garbageCollectionErrors []error
	}
	tests := []struct {
		name              string
		fields            fields
		args              args
		wantAllCompleted  bool
		wantAnyFailed     bool
		wantErr           bool
		wantCondition     *migapi.Condition
		dontWantCondition *migapi.Condition
	}{
		{
			name: "when there are errors within less than 5 minutes of running the phase, warning should not be present",
			fields: fields{
				Log:    log.WithName("test-logger"),
				Client: getFakeCompatClient(),
				Owner: &migapi.DirectVolumeMigration{
					ObjectMeta: metav1.ObjectMeta{
						Name: "test-dvm", Namespace: "openshift-migration",
					},
					Status: migapi.DirectVolumeMigrationStatus{
						Conditions: migapi.Conditions{
							List: []migapi.Condition{
								{Type: Running, Category: Advisory, Status: True, LastTransitionTime: metav1.Time{Time: time.Now().Add(-1 * time.Minute)}},
							},
						},
					},
				},
			},
			args: args{
				status: rsyncClientOperationStatusList{
					ops: []rsyncClientOperationStatus{
						{errors: []error{fmt.Errorf("failed creating pod")}},
					},
				},
			},
			wantAllCompleted:  false,
			wantAnyFailed:     false,
			wantCondition:     nil,
			dontWantCondition: &migapi.Condition{Type: FailedCreatingRsyncPods, Status: True, Category: Warn},
		},
		{
			name: "when there are errors for over 5 minutes of running the phase, warning should be present",
			fields: fields{
				Log:    log.WithName("test-logger"),
				Client: getFakeCompatClient(),
				Owner: &migapi.DirectVolumeMigration{
					ObjectMeta: metav1.ObjectMeta{
						Name: "test-dvm", Namespace: "openshift-migration",
					},
					Status: migapi.DirectVolumeMigrationStatus{
						Conditions: migapi.Conditions{
							List: []migapi.Condition{
								{Type: Running, Category: Advisory, Status: True, LastTransitionTime: metav1.Time{Time: time.Now().Add(-6 * time.Minute)}},
							},
						},
					},
				},
			},
			args: args{
				status: rsyncClientOperationStatusList{
					ops: []rsyncClientOperationStatus{
						{errors: []error{fmt.Errorf("failed creating pod")}},
					},
				},
			},
			wantAllCompleted:  false,
			wantAnyFailed:     false,
			wantCondition:     &migapi.Condition{Type: FailedCreatingRsyncPods, Status: True, Category: Warn},
			dontWantCondition: nil,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tr := &Task{
				Log:    tt.fields.Log,
				Client: tt.fields.Client,
				Owner:  tt.fields.Owner,
			}
			gotAllCompleted, gotAnyFailed, _, err := tr.processRsyncOperationStatus(&tt.args.status, tt.args.garbageCollectionErrors)
			if (err != nil) != tt.wantErr {
				t.Errorf("Task.processRsyncOperationStatus() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if gotAllCompleted != tt.wantAllCompleted {
				t.Errorf("Task.processRsyncOperationStatus() gotAllCompleted = %v, want %v", gotAllCompleted, tt.wantAllCompleted)
			}
			if gotAnyFailed != tt.wantAnyFailed {
				t.Errorf("Task.processRsyncOperationStatus() gotAnyFailed = %v, want %v", gotAnyFailed, tt.wantAnyFailed)
			}
			if tt.wantCondition != nil && !tt.fields.Owner.Status.HasCondition(tt.wantCondition.Type) {
				t.Errorf("Task.processRsyncOperationStatus() didn't find expected condition of type %s", tt.wantCondition.Type)
			}
			if tt.dontWantCondition != nil && tt.fields.Owner.Status.HasCondition(tt.dontWantCondition.Type) {
				t.Errorf("Task.processRsyncOperationStatus() found unexpected condition of type %s", tt.dontWantCondition.Type)
			}
		})
	}
}

func getPVCPair(name string, namespace string, volumeMode corev1.PersistentVolumeMode) transfer.PVCPair {
	pvcPair := transfer.NewPVCPair(
		&corev1.PersistentVolumeClaim{
			ObjectMeta: metav1.ObjectMeta{
				Name:      name,
				Namespace: namespace,
			},
			Spec: corev1.PersistentVolumeClaimSpec{
				AccessModes: []corev1.PersistentVolumeAccessMode{corev1.ReadWriteOnce},
				VolumeMode:  &volumeMode,
			},
		},
		nil)
	return pvcPair
}

type invalidPVCPair struct {
}

func (i invalidPVCPair) Source() transfer.PVC {
	return nil
}

func (i invalidPVCPair) Destination() transfer.PVC {
	return nil
}

func getDependencies(ns string, ownerName string) []k8sclient.Object {
	deps := []k8sclient.Object{}
	fsRoute := &routev1.Route{
		ObjectMeta: metav1.ObjectMeta{
			Name:      DirectVolumeMigrationRsyncTransferRoute,
			Namespace: ns,
		},
		Spec: routev1.RouteSpec{
			Host: "test.domain.com",
			Port: &routev1.RoutePort{
				TargetPort: intstr.IntOrString{IntVal: 80}},
			TLS: &routev1.TLSConfig{
				Termination: routev1.TLSTerminationEdge,
			},
		},
		Status: routev1.RouteStatus{
			Ingress: []routev1.RouteIngress{
				{Conditions: []routev1.RouteIngressCondition{
					{Type: routev1.RouteAdmitted, Status: corev1.ConditionTrue}}}}},
	}
	deps = append(deps, fsRoute)
	blockRoute := &routev1.Route{
		ObjectMeta: metav1.ObjectMeta{
			Name:      DirectVolumeMigrationRsyncTransferRouteBlock,
			Namespace: ns,
		},
		Spec: routev1.RouteSpec{
			Host: "test.domain.com",
			Port: &routev1.RoutePort{
				TargetPort: intstr.IntOrString{IntVal: 80}},
			TLS: &routev1.TLSConfig{
				Termination: routev1.TLSTerminationEdge,
			},
		},
		Status: routev1.RouteStatus{
			Ingress: []routev1.RouteIngress{
				{Conditions: []routev1.RouteIngressCondition{
					{Type: routev1.RouteAdmitted, Status: corev1.ConditionTrue}}}}},
	}
	deps = append(deps, blockRoute)
	stunnelConf := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{Name: "fs-crane2-stunnel-server-config", Namespace: ns},
	}
	deps = append(deps, stunnelConf)
	stunnelConf = &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{Name: "block-crane2-stunnel-server-config", Namespace: ns},
	}
	deps = append(deps, stunnelConf)
	rsyncConf := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{Name: "fs-crane2-rsync-server-config", Namespace: ns},
	}
	deps = append(deps, rsyncConf)
	stunnelSecret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{Name: "fs-crane2-stunnel-server-secret", Namespace: ns},
		Data:       map[string][]byte{"tls.key": []byte{}, "tls.crt": []byte{}},
	}
	deps = append(deps, stunnelSecret)
	stunnelSecret = &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{Name: "block-crane2-stunnel-server-secret", Namespace: ns},
		Data:       map[string][]byte{"tls.key": []byte{}, "tls.crt": []byte{}},
	}
	deps = append(deps, stunnelSecret)
	rsyncSecret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{Name: "crane2-rsync-server-secret", Namespace: ns},
		Data:       map[string][]byte{"tls.key": []byte{}, "tls.crt": []byte{}},
	}
	stunnelConf = &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{Name: "fs-crane2-stunnel-client-config", Namespace: ns},
	}
	deps = append(deps, stunnelConf)
	stunnelConf = &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{Name: "block-crane2-stunnel-client-config", Namespace: ns},
	}
	deps = append(deps, stunnelConf)
	rsyncConf = &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{Name: "fs-crane2-rsync-client-config", Namespace: ns},
	}
	deps = append(deps, rsyncConf)
	stunnelSecret = &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{Name: "fs-crane2-stunnel-client-secret", Namespace: ns},
		Data:       map[string][]byte{"tls.key": []byte{}, "tls.crt": []byte{}},
	}
	deps = append(deps, stunnelSecret)
	stunnelSecret = &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{Name: "block-crane2-stunnel-client-secret", Namespace: ns},
		Data:       map[string][]byte{"tls.key": []byte{}, "tls.crt": []byte{}},
	}
	deps = append(deps, stunnelSecret)
	rsyncSecret = &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{Name: "fs-crane2-rsync-client-secret", Namespace: ns},
		Data:       map[string][]byte{"tls.key": []byte{}, "tls.crt": []byte{}},
	}
	deps = append(deps, rsyncSecret)
	rsyncPass := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{Name: getMD5Hash(fmt.Sprintf("%s-%s", DirectVolumeMigrationRsyncPass, ownerName)), Namespace: ns},
		Data:       map[string][]byte{corev1.BasicAuthPasswordKey: []byte{}},
	}
	deps = append(deps, rsyncPass)
	return deps
}

func TestTask_createRsyncTransferClients(t *testing.T) {
	type fields struct {
		SrcClient  compat.Client
		DestClient compat.Client
		Owner      *migapi.DirectVolumeMigration
		PVCPairMap map[string][]transfer.PVCPair
	}
	migration := &migapi.MigMigration{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "migmigration",
			Namespace: migapi.OpenshiftMigrationNamespace,
		},
	}
	tests := []struct {
		name         string
		fields       fields
		wantPods     []*corev1.Pod
		wantErr      bool
		wantReturn   rsyncClientOperationStatusList
		wantCRStatus []*migapi.RsyncOperation
	}{
		{
			name: "when there are 0 existing Rsync Pods in source and no PVCs to migrate, status list should be empty",
			fields: fields{
				SrcClient:  getFakeCompatClient(getDependencies("test-ns", "test-dvm")...),
				DestClient: getFakeCompatClient(append(getDependencies("test-ns", "test-dvm"), migration)...),
				Owner: &migapi.DirectVolumeMigration{
					ObjectMeta: metav1.ObjectMeta{Name: "test-dvm", Namespace: migapi.OpenshiftMigrationNamespace, OwnerReferences: []metav1.OwnerReference{{Name: "migmigration"}}},
					Spec:       migapi.DirectVolumeMigrationSpec{BackOffLimit: 2},
				},
			},
			wantPods:     []*corev1.Pod{},
			wantErr:      false,
			wantReturn:   rsyncClientOperationStatusList{},
			wantCRStatus: []*migapi.RsyncOperation{},
		},
		{
			name: "when there are 0 existing Rsync Pods in source and 1 new fs PVC is provided as input, 1 Rsync Pod must be created in source namespace",
			fields: fields{
				SrcClient:  getFakeCompatClient(getDependencies("test-ns", "test-dvm")...),
				DestClient: getFakeCompatClient(append(getDependencies("test-ns", "test-dvm"), migration)...),
				Owner: &migapi.DirectVolumeMigration{
					ObjectMeta: metav1.ObjectMeta{Name: "test-dvm", Namespace: migapi.OpenshiftMigrationNamespace, OwnerReferences: []metav1.OwnerReference{{Name: "migmigration"}}},
					Spec:       migapi.DirectVolumeMigrationSpec{BackOffLimit: 2},
				},
				PVCPairMap: map[string][]transfer.PVCPair{
					"test-ns": {
						getPVCPair("pvc-0", "test-ns", corev1.PersistentVolumeFilesystem),
					},
				},
			},
			wantPods: []*corev1.Pod{
				getTestRsyncPodForPVC("pod-0", "pvc-0", "test-ns", "1", metav1.Now().Time),
			},
			wantErr:      false,
			wantReturn:   rsyncClientOperationStatusList{},
			wantCRStatus: []*migapi.RsyncOperation{},
		},
		{
			name: "when there are 0 existing Rsync Pods in source and 1 new block PVC is provided as input, 1 Rsync Pod must be created in source namespace",
			fields: fields{
				SrcClient:  getFakeCompatClient(getDependencies("test-ns", "test-dvm")...),
				DestClient: getFakeCompatClient(append(getDependencies("test-ns", "test-dvm"), migration)...),
				Owner: &migapi.DirectVolumeMigration{
					ObjectMeta: metav1.ObjectMeta{Name: "test-dvm", Namespace: migapi.OpenshiftMigrationNamespace, OwnerReferences: []metav1.OwnerReference{{Name: "migmigration"}}},
					Spec:       migapi.DirectVolumeMigrationSpec{BackOffLimit: 2},
				},
				PVCPairMap: map[string][]transfer.PVCPair{
					"test-ns": {
						getPVCPair("pvc-0", "test-ns", corev1.PersistentVolumeBlock),
					},
				},
			},
			wantPods: []*corev1.Pod{
				getTestRsyncPodForPVC("pod-0", "pvc-0", "test-ns", "1", metav1.Now().Time),
			},
			wantErr:      false,
			wantReturn:   rsyncClientOperationStatusList{},
			wantCRStatus: []*migapi.RsyncOperation{},
		},
		{
			name: "when there are 0 existing Rsync Pods in source and 2 new fs PVCs and one block PVC are provided as input, 3 Rsync Pod must be created in source namespace",
			fields: fields{
				SrcClient:  getFakeCompatClient(getDependencies("test-ns", "test-dvm")...),
				DestClient: getFakeCompatClient(append(getDependencies("test-ns", "test-dvm"), migration)...),
				Owner: &migapi.DirectVolumeMigration{
					ObjectMeta: metav1.ObjectMeta{Name: "test-dvm", Namespace: migapi.OpenshiftMigrationNamespace, OwnerReferences: []metav1.OwnerReference{{Name: "migmigration"}}},
					Spec:       migapi.DirectVolumeMigrationSpec{BackOffLimit: 2},
				},
				PVCPairMap: map[string][]transfer.PVCPair{
					"test-ns": {
						getPVCPair("pvc-0", "ns-00", corev1.PersistentVolumeFilesystem),
						getPVCPair("pvc-0", "ns-01", corev1.PersistentVolumeBlock),
						getPVCPair("pvc-0", "ns-02", corev1.PersistentVolumeFilesystem),
					},
				},
			},
			wantPods: []*corev1.Pod{
				getTestRsyncPodForPVC("pod-0", "pvc-0", "ns-00", "1", metav1.Now().Time),
				getTestRsyncPodForPVC("pod-0", "pvc-0", "ns-01", "1", metav1.Now().Time),
				getTestRsyncPodForPVC("pod-0", "pvc-0", "ns-02", "1", metav1.Now().Time),
			},
			wantErr:      false,
			wantReturn:   rsyncClientOperationStatusList{},
			wantCRStatus: []*migapi.RsyncOperation{},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tr := &Task{
				Log:    log.WithName("test-logger"),
				Client: tt.fields.DestClient,
				Owner:  tt.fields.Owner,
			}
			got, err := tr.createRsyncTransferClients(tt.fields.SrcClient, tt.fields.DestClient, tt.fields.PVCPairMap)
			if (err != nil) != tt.wantErr {
				t.Errorf("Task.createRsyncTransferClients() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if got.Succeeded() != tt.wantReturn.Succeeded() {
				t.Errorf("Task.createRsyncTransferClients() = got %d succeded operations, want %d", got.Succeeded(), tt.wantReturn.Succeeded())
			}
			if got.Pending() != tt.wantReturn.Pending() {
				t.Errorf("Task.createRsyncTransferClients() = got %d pending operations, want %d", got.Pending(), tt.wantReturn.Pending())
			}
			if got.Failed() != tt.wantReturn.Failed() {
				t.Errorf("Task.createRsyncTransferClients() = got %d failed operations, want %d", got.Failed(), tt.wantReturn.Failed())
			}
			if got.Running() != tt.wantReturn.Running() {
				t.Errorf("Task.createRsyncTransferClients() = got %d running operations, want %d", got.Running(), tt.wantReturn.Running())
			}

			for _, s := range tt.wantCRStatus {
				got := tt.fields.Owner.Status.GetRsyncOperationStatusForPVC(&corev1.ObjectReference{
					Name:      s.PVCReference.Name,
					Namespace: s.PVCReference.Namespace,
				})
				if !reflect.DeepEqual(*got, *s) {
					t.Errorf("Task.createRsyncTransferClients() expected operation status doesnt match actual, want %v got %v",
						*s, *got)
				}
			}

			podExistsInSource := func(pod *corev1.Pod) bool {
				srcPods := corev1.PodList{}
				errs := tt.fields.SrcClient.List(
					context.TODO(),
					&srcPods,
					k8sclient.InNamespace(pod.Namespace),
					k8sclient.MatchingLabels(pod.Labels),
				)
				if errs != nil {
					t.Errorf("Task.createRsyncTransferClients() failed getting pods in source namespace")
				}
				return len(srcPods.Items) > 0
			}
			// check whether expected pods are present in the source ns
			for _, pod := range tt.wantPods {
				if !podExistsInSource(pod) {
					t.Errorf("Task.createRsyncTransferClients() expected pod with labels %v not found in the source ns", pod.Labels)
				}
			}
		})
	}
}

func Test_getSecurityContext(t *testing.T) {
	type fields struct {
		client    compat.Client
		Owner     *migapi.DirectVolumeMigration
		Migration *migapi.MigMigration
	}
	trueBool := bool(true)
	falseBool := bool(false)
	runAsUser := int64(100000)
	runAsGroup := int64(2)
	tests := []struct {
		name       string
		fields     fields
		wantErr    bool
		wantReturn *corev1.SecurityContext
	}{
		{
			name: "run rsync as user and group",
			fields: fields{
				client: getFakeCompatClientWithVersion(1, 24),
				Owner: &migapi.DirectVolumeMigration{
					ObjectMeta: metav1.ObjectMeta{Name: "test-dvm", Namespace: migapi.OpenshiftMigrationNamespace, OwnerReferences: []metav1.OwnerReference{{Name: "migmigration"}}},
					Spec:       migapi.DirectVolumeMigrationSpec{BackOffLimit: 2},
				},
				Migration: &migapi.MigMigration{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "migmigration",
						Namespace: migapi.OpenshiftMigrationNamespace,
					},
					Spec: migapi.MigMigrationSpec{
						RunAsUser:  &runAsUser,
						RunAsGroup: &runAsGroup,
					},
				},
			},
			wantErr: false,
			wantReturn: &corev1.SecurityContext{
				RunAsUser:    &runAsUser,
				RunAsGroup:   &runAsGroup,
				RunAsNonRoot: &trueBool,
				Capabilities: &corev1.Capabilities{
					Drop: []corev1.Capability{"ALL"},
				},
				AllowPrivilegeEscalation: &falseBool,
				SeccompProfile: &corev1.SeccompProfile{
					Type: "RuntimeDefault",
				},
			},
		},
		{
			name: "run rsync with user",
			fields: fields{
				client: getFakeCompatClientWithVersion(1, 24),
				Owner: &migapi.DirectVolumeMigration{
					ObjectMeta: metav1.ObjectMeta{Name: "test-dvm", Namespace: migapi.OpenshiftMigrationNamespace, OwnerReferences: []metav1.OwnerReference{{Name: "migmigration"}}},
					Spec:       migapi.DirectVolumeMigrationSpec{BackOffLimit: 2},
				},
				Migration: &migapi.MigMigration{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "migmigration",
						Namespace: migapi.OpenshiftMigrationNamespace,
					},
					Spec: migapi.MigMigrationSpec{
						RunAsUser: &runAsUser,
					},
				},
			},
			wantErr: false,
			wantReturn: &corev1.SecurityContext{
				RunAsUser:    &runAsUser,
				RunAsNonRoot: &trueBool,
				Capabilities: &corev1.Capabilities{
					Drop: []corev1.Capability{"ALL"},
				},
				AllowPrivilegeEscalation: &falseBool,
				SeccompProfile: &corev1.SeccompProfile{
					Type: "RuntimeDefault",
				},
			},
		},
		{
			name: "run rsync with group",
			fields: fields{
				client: getFakeCompatClientWithVersion(1, 24),
				Owner: &migapi.DirectVolumeMigration{
					ObjectMeta: metav1.ObjectMeta{Name: "test-dvm", Namespace: migapi.OpenshiftMigrationNamespace, OwnerReferences: []metav1.OwnerReference{{Name: "migmigration"}}},
					Spec:       migapi.DirectVolumeMigrationSpec{BackOffLimit: 2},
				},
				Migration: &migapi.MigMigration{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "migmigration",
						Namespace: migapi.OpenshiftMigrationNamespace,
					},
					Spec: migapi.MigMigrationSpec{
						RunAsGroup: &runAsGroup,
					},
				},
			},
			wantErr: false,
			wantReturn: &corev1.SecurityContext{
				RunAsGroup:   &runAsGroup,
				RunAsNonRoot: &trueBool,
				Capabilities: &corev1.Capabilities{
					Drop: []corev1.Capability{"ALL"},
				},
				AllowPrivilegeEscalation: &falseBool,
				SeccompProfile: &corev1.SeccompProfile{
					Type: "RuntimeDefault",
				},
			},
		},
		{
			name: "run rsync with default",
			fields: fields{
				client: getFakeCompatClientWithVersion(1, 24),
				Owner: &migapi.DirectVolumeMigration{
					ObjectMeta: metav1.ObjectMeta{Name: "test-dvm", Namespace: migapi.OpenshiftMigrationNamespace, OwnerReferences: []metav1.OwnerReference{{Name: "migmigration"}}},
					Spec:       migapi.DirectVolumeMigrationSpec{BackOffLimit: 2},
				},
				Migration: &migapi.MigMigration{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "migmigration",
						Namespace: migapi.OpenshiftMigrationNamespace,
					},
				},
			},
			wantErr: false,
			wantReturn: &corev1.SecurityContext{
				RunAsNonRoot: &trueBool,
				Capabilities: &corev1.Capabilities{
					Drop: []corev1.Capability{"ALL"},
				},
				AllowPrivilegeEscalation: &falseBool,
				SeccompProfile: &corev1.SeccompProfile{
					Type: "RuntimeDefault",
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tr := &Task{
				Log:    log.WithName("test-logger"),
				Client: tt.fields.client,
				Owner:  tt.fields.Owner,
			}
			got, err := tr.getSecurityContext(tt.fields.client, "test-ns", tt.fields.Migration)
			if (err != nil) != tt.wantErr {
				t.Errorf("Task.getSecurityContext() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.wantReturn) {
				t.Errorf("Task.getSecurityContext() expected secutiryContext doesn't match actual, want %v got %v",
					tt.wantReturn, got)
			}
		})
	}
}

func Test_ensureRsyncEndpoints(t *testing.T) {
	type fields struct {
		srcClient    compat.Client
		destClient   compat.Client
		endpointType migapi.EndpointType
		owner        *migapi.DirectVolumeMigration
	}
	tests := []struct {
		name         string
		fields       fields
		wantErr      bool
		checkService bool
		checkRoute   bool
	}{
		{
			name:    "error getting destination client",
			fields:  fields{},
			wantErr: true,
		},
		{
			name: "get rsync service endpoints",
			fields: fields{
				srcClient:    getFakeCompatClient(),
				destClient:   getFakeCompatClient(createNode("worker1"), createNode("worker2")),
				endpointType: migapi.NodePort,
				owner: &migapi.DirectVolumeMigration{
					ObjectMeta: metav1.ObjectMeta{Name: "test-dvm", Namespace: migapi.OpenshiftMigrationNamespace, OwnerReferences: []metav1.OwnerReference{{Name: "migmigration"}}},
					Spec: migapi.DirectVolumeMigrationSpec{
						BackOffLimit: 2,
						PersistentVolumeClaims: []migapi.PVCToMigrate{
							{ObjectReference: &corev1.ObjectReference{Namespace: "foo", Name: "pvc-0"}},
							{ObjectReference: &corev1.ObjectReference{Namespace: "foo", Name: "pvc-1"}},
						},
					},
				},
			},
			wantErr:      false,
			checkService: true,
		},
		{
			name: "get rsync ingress endpoints",
			fields: fields{
				srcClient:    getFakeCompatClient(),
				destClient:   getFakeCompatClient(createMigCluster("test-cluster"), createClusterIngress()),
				endpointType: migapi.Route,
				owner: &migapi.DirectVolumeMigration{
					ObjectMeta: metav1.ObjectMeta{Name: "test-dvm", Namespace: migapi.OpenshiftMigrationNamespace, OwnerReferences: []metav1.OwnerReference{{Name: "migmigration"}}},
					Spec: migapi.DirectVolumeMigrationSpec{
						BackOffLimit: 2,
						PersistentVolumeClaims: []migapi.PVCToMigrate{
							{ObjectReference: &corev1.ObjectReference{Namespace: "foo", Name: "pvc-0"}},
							{ObjectReference: &corev1.ObjectReference{Namespace: "foo", Name: "pvc-1"}},
						},
						DestMigClusterRef: &corev1.ObjectReference{
							Name:      "test-cluster",
							Namespace: migapi.OpenshiftMigrationNamespace,
						},
					},
				},
			},
			wantErr: false,
		},
		{
			name: "fail to get rsync ingress endpoints",
			fields: fields{
				srcClient:    getFakeCompatClient(),
				destClient:   getFakeCompatClient(createMigCluster("test-cluster")),
				endpointType: migapi.Route,
				owner: &migapi.DirectVolumeMigration{
					ObjectMeta: metav1.ObjectMeta{Name: "test-dvm", Namespace: migapi.OpenshiftMigrationNamespace, OwnerReferences: []metav1.OwnerReference{{Name: "migmigration"}}},
					Spec: migapi.DirectVolumeMigrationSpec{
						BackOffLimit: 2,
						PersistentVolumeClaims: []migapi.PVCToMigrate{
							{ObjectReference: &corev1.ObjectReference{Namespace: "foo", Name: "pvc-0"}},
							{ObjectReference: &corev1.ObjectReference{Namespace: "foo", Name: "pvc-1"}},
						},
						DestMigClusterRef: &corev1.ObjectReference{
							Name:      "test-cluster",
							Namespace: migapi.OpenshiftMigrationNamespace,
						},
					},
				},
			},
			wantErr: true,
		},
		{
			name: "get rsync route endpoints",
			fields: fields{
				srcClient:    getFakeCompatClient(),
				destClient:   getFakeCompatClientWithSubdomain(createMigCluster("test-cluster")),
				endpointType: migapi.Route,
				owner: &migapi.DirectVolumeMigration{
					ObjectMeta: metav1.ObjectMeta{Name: "test-dvm", Namespace: migapi.OpenshiftMigrationNamespace, OwnerReferences: []metav1.OwnerReference{{Name: "migmigration"}}},
					Spec: migapi.DirectVolumeMigrationSpec{
						BackOffLimit: 2,
						PersistentVolumeClaims: []migapi.PVCToMigrate{
							{ObjectReference: &corev1.ObjectReference{Namespace: "foo", Name: "pvc-0"}},
							{ObjectReference: &corev1.ObjectReference{Namespace: "foo", Name: "pvc-1"}},
						},
						DestMigClusterRef: &corev1.ObjectReference{
							Name:      "test-cluster",
							Namespace: migapi.OpenshiftMigrationNamespace,
						},
					},
				},
			},
			wantErr:    false,
			checkRoute: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if !migapi.Settings.EnableCachedClient {
				migapi.Settings.EnableCachedClient = true
				defer func() {
					migapi.Settings.EnableCachedClient = false
				}()
			}
			tr := &Task{
				Log:               log.WithName("test-logger"),
				destinationClient: tt.fields.destClient,
				Owner:             tt.fields.owner,
				EndpointType:      tt.fields.endpointType,
				Client:            tt.fields.destClient,
			}
			if err := tr.ensureRsyncEndpoints(); err != nil && !tt.wantErr {
				t.Fatalf("ensureRsyncEndpoints() error = %v, wantErr %v", err, tt.wantErr)
			}
			if tt.checkService {
				// verify the endpoints are created
				verifyServiceHealthy(DirectVolumeMigrationRsyncTransferSvc, "foo", DirectVolumeMigrationRsyncTransfer, tt.fields.destClient, t)
				verifyServiceHealthy(DirectVolumeMigrationRsyncTransferSvcBlock, "foo", DirectVolumeMigrationRsyncTransferBlock, tt.fields.destClient, t)
			} else if tt.checkRoute {
				verifyRouteHealthy(DirectVolumeMigrationRsyncTransferRoute, "foo", DirectVolumeMigrationRsyncTransfer, tt.fields.destClient, t)
				verifyRouteHealthy(DirectVolumeMigrationRsyncTransferRouteBlock, "foo", DirectVolumeMigrationRsyncTransferBlock, tt.fields.destClient, t)
			}
		})
	}
}

func verifyServiceHealthy(name, namespace, appLabel string, c client.Client, t *testing.T) {
	service := &corev1.Service{}
	if err := c.Get(context.TODO(), types.NamespacedName{Namespace: namespace, Name: name}, service); err != nil {
		t.Fatalf("ensureRsyncEndpoints() failed to get rsync service in namespace %s", namespace)
	}
	if service.Labels["app"] != appLabel {
		t.Fatalf("ensureRsyncEndpoints() service app label is incorrect, [%s]", service.Labels["app"])
	}
	_, err := transferservice.GetEndpointFromKubeObjects(c, types.NamespacedName{Namespace: namespace, Name: name})
	if err != nil {
		t.Fatalf("ensureRsyncEndpoints() failed to get endpoint from service")
	}
}

func verifyRouteHealthy(name, namespace, appLabel string, c client.Client, t *testing.T) {
	route := &routev1.Route{}
	if err := c.Get(context.TODO(), types.NamespacedName{Namespace: namespace, Name: name}, route); err != nil {
		if !k8serrors.IsNotFound(err) {
			t.Fatalf("ensureRsyncEndpoints() failed to get rsync route in namespace %s", namespace)
		}
	}
	if route.Labels["app"] != appLabel {
		t.Fatalf("ensureRsyncEndpoints() route app label is incorrect, [%s]", route.Labels["app"])
	}
	if !strings.HasSuffix(route.Spec.Host, "test.domain") {
		t.Fatalf("ensureRsyncEndpoints() route host doesn't have right suffix %s", route.Spec.Host)
	}

	verifyServiceHealthy(name, namespace, appLabel, c, t)
}

func Test_ensureFilesystemRsyncTransferServer(t *testing.T) {
	type fields struct {
		srcClient        compat.Client
		destClient       compat.Client
		Owner            *migapi.DirectVolumeMigration
		PVCPairMap       map[string][]transfer.PVCPair
		transportOptions *transport.Options
	}
	migration := &migapi.MigMigration{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "migmigration",
			Namespace: migapi.OpenshiftMigrationNamespace,
		},
	}
	tests := []struct {
		name    string
		fields  fields
		wantErr bool
		wantPod bool
	}{
		{
			name: "get single pvc filesystem server pod",
			fields: fields{
				srcClient:  getFakeCompatClient(getDependencies("test-ns", "test-dvm")...),
				destClient: getFakeCompatClient(append(getDependencies("test-ns", "test-dvm"), migration)...),
				PVCPairMap: map[string][]transfer.PVCPair{
					"test-ns": {
						getPVCPair("pvc", "test-ns", corev1.PersistentVolumeFilesystem),
					},
				},
				Owner: &migapi.DirectVolumeMigration{
					ObjectMeta: metav1.ObjectMeta{Name: "test-dvm", Namespace: migapi.OpenshiftMigrationNamespace, OwnerReferences: []metav1.OwnerReference{{Name: "migmigration"}}},
					Spec:       migapi.DirectVolumeMigrationSpec{BackOffLimit: 2},
				},
			},
			wantPod: true,
		},
		{
			name: "no filesystem pod available, no server pods",
			fields: fields{
				srcClient:  getFakeCompatClient(getDependencies("test-ns", "test-dvm")...),
				destClient: getFakeCompatClient(append(getDependencies("test-ns", "test-dvm"), migration)...),
				PVCPairMap: map[string][]transfer.PVCPair{
					"test-ns": {
						getPVCPair("pvc", "test-ns", corev1.PersistentVolumeBlock),
					},
				},
				Owner: &migapi.DirectVolumeMigration{
					ObjectMeta: metav1.ObjectMeta{Name: "test-dvm", Namespace: migapi.OpenshiftMigrationNamespace, OwnerReferences: []metav1.OwnerReference{{Name: "migmigration"}}},
					Spec:       migapi.DirectVolumeMigrationSpec{BackOffLimit: 2},
				},
			},
		},
		{
			name: "error with invalid PVCPair",
			fields: fields{
				srcClient:  getFakeCompatClient(getDependencies("test-ns", "test-dvm")...),
				destClient: getFakeCompatClient(append(getDependencies("test-ns", "test-dvm"), migration)...),
				PVCPairMap: map[string][]transfer.PVCPair{
					"test-ns": {
						invalidPVCPair{},
					},
				},
				Owner: &migapi.DirectVolumeMigration{
					ObjectMeta: metav1.ObjectMeta{Name: "test-dvm", Namespace: migapi.OpenshiftMigrationNamespace, OwnerReferences: []metav1.OwnerReference{{Name: "migmigration"}}},
					Spec:       migapi.DirectVolumeMigrationSpec{BackOffLimit: 2},
				},
			},
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tr := &Task{
				Log:               log.WithName("test-logger"),
				destinationClient: tt.fields.destClient,
				Owner:             tt.fields.Owner,
				Client:            tt.fields.destClient,
			}
			if err := tr.ensureFilesystemRsyncTransferServer(tt.fields.srcClient, tt.fields.destClient, tt.fields.PVCPairMap, tt.fields.transportOptions); err != nil && !tt.wantErr {
				t.Fatalf("ensureFilesystemRsyncTransferServer() error = %v, wantErr %v", err, tt.wantErr)
			}
			// Verify the server pod is created in the destination namespace
			pod := &corev1.Pod{}
			if err := tt.fields.destClient.Get(context.TODO(), types.NamespacedName{Namespace: "test-ns", Name: "rsync-server"}, pod); err != nil {
				if tt.wantPod && k8serrors.IsNotFound(err) {
					t.Fatalf("ensureFilesystemRsyncTransferServer() wanted pod and not found %s", "test-ns")
				} else if !k8serrors.IsNotFound(err) {
					t.Fatalf("ensureFilesystemRsyncTransferServer() failed to get rsync server pod in namespace %s", "test-ns")
				}
			}
		})
	}
}

func Test_ensureBlockRsyncTransferServer(t *testing.T) {
	type fields struct {
		srcClient        compat.Client
		destClient       compat.Client
		Owner            *migapi.DirectVolumeMigration
		PVCPairMap       map[string][]transfer.PVCPair
		transportOptions *transport.Options
	}
	migration := &migapi.MigMigration{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "migmigration",
			Namespace: migapi.OpenshiftMigrationNamespace,
		},
	}
	tests := []struct {
		name    string
		fields  fields
		wantErr bool
		wantPod bool
	}{
		{
			name: "get single pvc block server pod",
			fields: fields{
				srcClient:  getFakeCompatClient(getDependencies("test-ns", "test-dvm")...),
				destClient: getFakeCompatClient(append(getDependencies("test-ns", "test-dvm"), migration)...),
				PVCPairMap: map[string][]transfer.PVCPair{
					"test-ns": {
						getPVCPair("pvc", "test-ns", corev1.PersistentVolumeBlock),
					},
				},
				Owner: &migapi.DirectVolumeMigration{
					ObjectMeta: metav1.ObjectMeta{Name: "test-dvm", Namespace: migapi.OpenshiftMigrationNamespace, OwnerReferences: []metav1.OwnerReference{{Name: "migmigration"}}},
					Spec:       migapi.DirectVolumeMigrationSpec{BackOffLimit: 2},
				},
			},
			wantPod: true,
		},
		{
			name: "no block pod available, no server pods",
			fields: fields{
				srcClient:  getFakeCompatClient(getDependencies("test-ns", "test-dvm")...),
				destClient: getFakeCompatClient(append(getDependencies("test-ns", "test-dvm"), migration)...),
				PVCPairMap: map[string][]transfer.PVCPair{
					"test-ns": {
						getPVCPair("pvc", "test-ns", corev1.PersistentVolumeFilesystem),
					},
				},
				Owner: &migapi.DirectVolumeMigration{
					ObjectMeta: metav1.ObjectMeta{Name: "test-dvm", Namespace: migapi.OpenshiftMigrationNamespace, OwnerReferences: []metav1.OwnerReference{{Name: "migmigration"}}},
					Spec:       migapi.DirectVolumeMigrationSpec{BackOffLimit: 2},
				},
			},
		},
		{
			name: "error with invalid PVCPair",
			fields: fields{
				srcClient:  getFakeCompatClient(getDependencies("test-ns", "test-dvm")...),
				destClient: getFakeCompatClient(append(getDependencies("test-ns", "test-dvm"), migration)...),
				PVCPairMap: map[string][]transfer.PVCPair{
					"test-ns": {
						invalidPVCPair{},
					},
				},
				Owner: &migapi.DirectVolumeMigration{
					ObjectMeta: metav1.ObjectMeta{Name: "test-dvm", Namespace: migapi.OpenshiftMigrationNamespace, OwnerReferences: []metav1.OwnerReference{{Name: "migmigration"}}},
					Spec:       migapi.DirectVolumeMigrationSpec{BackOffLimit: 2},
				},
			},
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tr := &Task{
				Log:               log.WithName("test-logger"),
				destinationClient: tt.fields.destClient,
				Owner:             tt.fields.Owner,
				Client:            tt.fields.destClient,
			}
			if err := tr.ensureBlockRsyncTransferServer(tt.fields.srcClient, tt.fields.destClient, tt.fields.PVCPairMap, tt.fields.transportOptions); err != nil && !tt.wantErr {
				t.Fatalf("ensureBlockRsyncTransferServer() error = %v, wantErr %v", err, tt.wantErr)
			}
			// Verify the server pod is created in the destination namespace
			pod := &corev1.Pod{}
			if err := tt.fields.destClient.Get(context.TODO(), types.NamespacedName{Namespace: "test-ns", Name: "blockrsync-server"}, pod); err != nil {
				if tt.wantPod && k8serrors.IsNotFound(err) {
					t.Fatalf("ensureBlockRsyncTransferServer() wanted pod and not found %s", "test-ns")
				} else if !k8serrors.IsNotFound(err) {
					t.Fatalf("ensureBlockRsyncTransferServer() failed to get rsync server pod in namespace %s", "test-ns")
				}
			}
		})
	}
}

func createNode(name string) *corev1.Node {
	return &corev1.Node{
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
			Labels: map[string]string{
				"node-role.kubernetes.io/worker": "",
			},
		},
		Status: corev1.NodeStatus{
			Addresses: []corev1.NodeAddress{
				{
					Type:    corev1.NodeInternalDNS,
					Address: fmt.Sprintf("%s.internal", name),
				},
			},
		},
	}
}

func createMigCluster(name string) *migapi.MigCluster {
	return &migapi.MigCluster{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: migapi.OpenshiftMigrationNamespace,
			UID:       types.UID(rand.String(16)),
		},
		Spec: migapi.MigClusterSpec{
			IsHostCluster: true,
		},
	}
}

func createClusterIngress() *configv1.Ingress {
	return &configv1.Ingress{
		ObjectMeta: metav1.ObjectMeta{
			Name: "cluster",
		},
		Spec: configv1.IngressSpec{
			Domain: "ingress.domain",
		},
	}
}

// FILEPATH: /home/awels/go/src/github.com/awels/mig-controller/pkg/controller/migmigration/rollback_test.go

package migmigration

import (
	"context"
	"testing"

	"github.com/konveyor/mig-controller/pkg/apis/migration/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func TestTask_DeleteLiveMigrationCompletedPods(t *testing.T) {
	tests := []struct {
		name         string
		objects      []client.Object
		liveMigrate  bool
		expectedPods []*corev1.Pod
		deletedPods  []*corev1.Pod
	}{
		{
			name:        "live migrate is not checked",
			liveMigrate: false,
		},
		{
			name:        "live migrate is checked, no running pods",
			liveMigrate: true,
		},
		{
			name:        "live migrate is checked, running and completed pods, should delete completed pods",
			liveMigrate: true,
			objects: []client.Object{
				createVirtlauncherPodWithStatus("pod1", "ns1", corev1.PodRunning),
				createVirtlauncherPodWithStatus("pod1", "ns2", corev1.PodSucceeded),
				createVirtlauncherPod("pod2", "ns1"),
			},
			expectedPods: []*corev1.Pod{
				{
					ObjectMeta: metav1.ObjectMeta{
						Namespace: "ns1",
						Name:      "pod1-virt-launcher",
					},
				},
			},
			deletedPods: []*corev1.Pod{
				{
					ObjectMeta: metav1.ObjectMeta{
						Namespace: "ns2",
						Name:      "pod1-virt-launcher",
					},
				},
			},
		},
		{
			name:        "live migrate is checked, running and completed pods, but non matching volumes, should delete completed pods",
			liveMigrate: true,
			objects: []client.Object{
				createVirtlauncherPodWithStatus("pod1", "ns1", corev1.PodRunning),
				createVirtlauncherPodWithExtraVolume("pod1", "ns2", corev1.PodSucceeded),
				createVirtlauncherPod("pod2", "ns1"),
			},
			expectedPods: []*corev1.Pod{
				{
					ObjectMeta: metav1.ObjectMeta{
						Namespace: "ns1",
						Name:      "pod1-virt-launcher",
					},
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := getFakeClientWithObjs(tt.objects...)
			task := &Task{
				PlanResources: &v1alpha1.PlanResources{
					MigPlan: &v1alpha1.MigPlan{
						Spec: v1alpha1.MigPlanSpec{
							Namespaces:  []string{"ns1:ns1", "ns2"},
							LiveMigrate: ptr.To[bool](tt.liveMigrate),
							PersistentVolumes: v1alpha1.PersistentVolumes{
								List: []v1alpha1.PV{
									{
										PVC: v1alpha1.PVC{
											Namespace: "ns1",
											Name:      "pvc1",
										},
										Selection: v1alpha1.Selection{
											Action:       v1alpha1.PvCopyAction,
											CopyMethod:   v1alpha1.PvBlockCopyMethod,
											StorageClass: "sc2",
											AccessMode:   "ReadWriteOnce",
										},
										StorageClass: "sc1",
									},
									{
										PVC: v1alpha1.PVC{
											Namespace: "ns2",
											Name:      "pvc1",
										},
										Selection: v1alpha1.Selection{
											Action:       v1alpha1.PvCopyAction,
											CopyMethod:   v1alpha1.PvFilesystemCopyMethod,
											StorageClass: "sc2",
											AccessMode:   "ReadWriteOnce",
										},
										StorageClass: "sc1",
									},
								},
							},
						},
					},
				},
				destinationClient: c,
			}
			err := task.deleteLiveMigrationCompletedPods()
			if err != nil {
				t.Errorf("Task.deleteLiveMigrationCompletedPods() error = %v", err)
			}
			for _, pod := range tt.expectedPods {
				res := &corev1.Pod{}
				err := c.Get(context.TODO(), client.ObjectKeyFromObject(pod), res)
				if err != nil {
					t.Errorf("Task.deleteLiveMigrationCompletedPods() pod not found, while it should remain: %s/%s, %v", pod.Namespace, pod.Name, err)
				}
			}
			for _, pod := range tt.deletedPods {
				res := &corev1.Pod{}
				err := c.Get(context.TODO(), client.ObjectKeyFromObject(pod), res)
				if err == nil {
					t.Errorf("Task.deleteLiveMigrationCompletedPods() pod %s/%s found, while it should be deleted", pod.Namespace, pod.Name)
				}
			}
		})
	}
}

func createVirtlauncherPodWithStatus(name, namespace string, phase corev1.PodPhase) *corev1.Pod {
	pod := createVirtlauncherPod(name, namespace)
	pod.Spec.Volumes = append(pod.Spec.Volumes, corev1.Volume{
		Name: "volume",
		VolumeSource: corev1.VolumeSource{
			PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{
				ClaimName: "pvc1",
			},
		},
	})
	pod.Status.Phase = phase
	return pod
}

func createVirtlauncherPodWithExtraVolume(name, namespace string, phase corev1.PodPhase) *corev1.Pod {
	pod := createVirtlauncherPodWithStatus(name, namespace, phase)
	pod.Spec.Volumes = append(pod.Spec.Volumes, corev1.Volume{
		Name: "extra-volume",
		VolumeSource: corev1.VolumeSource{
			PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{
				ClaimName: "pvc2",
			},
		},
	})
	return pod
}

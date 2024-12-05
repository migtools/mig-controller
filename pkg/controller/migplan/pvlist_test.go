package migplan

import (
	"fmt"
	"strings"
	"testing"

	migapi "github.com/konveyor/mig-controller/pkg/apis/migration/v1alpha1"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func Test_getStatefulSetVolumeName(t *testing.T) {
	suffix := "abcd"
	tests := []struct {
		name    string
		pvcName string
		setName string
		migPlan *migapi.MigPlan
		want    string
	}{
		{
			name:    "given a statefulset volume with single ordinal value, and no suffix specified in plan must return correctly formatted name",
			pvcName: "example-set-0",
			setName: "set",
			migPlan: &migapi.MigPlan{
				Status: migapi.MigPlanStatus{},
			},
			want: fmt.Sprintf("example-mig-%s-set-0", migapi.StorageConversionPVCNamePrefix),
		},
		{
			name:    "given a statefulset volume with single ordinal value, and suffix specified in plan must return correctly formatted name",
			pvcName: "example-set-0",
			setName: "set",
			migPlan: &migapi.MigPlan{
				Status: migapi.MigPlanStatus{
					Suffix: &suffix,
				},
			},
			want: fmt.Sprintf("example-mig-%s-set-0", suffix),
		},
		{
			name:    "given a statefulset volume with double ordinal value, must return correctly formatted name",
			pvcName: "example-set-10",
			setName: "set",
			migPlan: &migapi.MigPlan{
				Status: migapi.MigPlanStatus{
					Suffix: &suffix,
				},
			},
			want: fmt.Sprintf("example-mig-%s-set-10", suffix),
		},
		{
			name:    "given a volume with no ordinal, must return original name",
			pvcName: "example-set",
			migPlan: &migapi.MigPlan{
				Status: migapi.MigPlanStatus{
					Suffix: &suffix,
				},
			},
			want: "example-set-mig-abcd",
		},
		{
			name:    "given a statefulset volume with double ordinal value, must return correctly formatted name",
			pvcName: "example-mig-defg-set-10",
			setName: "set",
			migPlan: &migapi.MigPlan{
				Status: migapi.MigPlanStatus{
					Suffix: &suffix,
				},
			},
			want: fmt.Sprintf("example-mig-%s-set-10", suffix),
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := getStatefulSetVolumeName(tt.pvcName, tt.setName, tt.migPlan); got != tt.want {
				t.Errorf("getStatefulSetVolumeName() = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_isStatefulSetVolume(t *testing.T) {
	type args struct {
		pvcRef  *corev1.ObjectReference
		podList []corev1.Pod
	}
	tests := []struct {
		name string
		args args
		want bool
	}{
		{
			name: "when none of the pods are using the volume, should return false",
			args: args{
				pvcRef:  &corev1.ObjectReference{Name: "test", Namespace: "test"},
				podList: []corev1.Pod{},
			},
			want: false,
		},
		{
			name: "when one or more pods are using the volume but are not owned by statefulsets, should return false",
			args: args{
				pvcRef: &corev1.ObjectReference{Name: "test", Namespace: "test"},
				podList: []corev1.Pod{
					{
						ObjectMeta: metav1.ObjectMeta{Name: "test-0", Namespace: "test"},
						Spec: corev1.PodSpec{
							Volumes: []corev1.Volume{
								{
									Name: "vol-0",
									VolumeSource: corev1.VolumeSource{
										PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{
											ClaimName: "test",
										},
									},
								},
							},
						},
					},
				},
			},
			want: false,
		},
		{
			name: "when one or more pods are using the volume and are not owned by statefulsets, should return true",
			args: args{
				pvcRef: &corev1.ObjectReference{Name: "test", Namespace: "test"},
				podList: []corev1.Pod{
					{
						ObjectMeta: metav1.ObjectMeta{
							Name:      "test-0",
							Namespace: "test",
							OwnerReferences: []metav1.OwnerReference{
								{
									Kind: "StatefulSet",
								}}},
						Spec: corev1.PodSpec{
							Volumes: []corev1.Volume{
								{
									Name: "vol-0",
									VolumeSource: corev1.VolumeSource{
										PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{
											ClaimName: "test",
										},
									},
								},
							},
						},
					},
				},
			},
			want: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got, _ := isStatefulSetVolume(tt.args.pvcRef, tt.args.podList); got != tt.want {
				t.Errorf("isStatefulSetVolume() = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_getMappedNameForPVC(t *testing.T) {
	suffix := "abcd"
	type args struct {
		pvcRef  *corev1.ObjectReference
		podList []corev1.Pod
		migPlan *migapi.MigPlan
	}
	tests := []struct {
		name string
		args args
		want string
	}{
		{
			name: "Migration plan is not a storage conversion plan, should return original PVC name",
			args: args{
				pvcRef:  &corev1.ObjectReference{Name: "test", Namespace: "test"},
				migPlan: &migapi.MigPlan{},
			},
			want: "test",
		},
		{
			name: "Migration plan is not a storage conversion plan, but existing name, should return original PVC name:existing name",
			args: args{
				pvcRef: &corev1.ObjectReference{Name: "test", Namespace: "test"},
				migPlan: &migapi.MigPlan{
					Spec: migapi.MigPlanSpec{
						PersistentVolumes: migapi.PersistentVolumes{
							List: []migapi.PV{
								{
									PVC: migapi.PVC{
										Name:      "test:existing-name",
										Namespace: "test",
									},
								},
							},
						},
					},
				},
			},
			want: "test:existing-name",
		},
		{
			name: "Migration plan is a storage conversion plan",
			args: args{
				pvcRef: &corev1.ObjectReference{Name: "test", Namespace: "test"},
				migPlan: &migapi.MigPlan{
					Spec: migapi.MigPlanSpec{
						PersistentVolumes: migapi.PersistentVolumes{
							List: []migapi.PV{},
						},
					},
					Status: migapi.MigPlanStatus{
						Suffix: &suffix,
						Conditions: migapi.Conditions{
							List: []migapi.Condition{
								{
									Type:   migapi.MigrationTypeIdentified,
									Reason: string(migapi.StorageConversionPlan),
								},
							},
						},
					},
				},
			},
			want: "test:test-mig-abcd",
		},
		{
			name: "Migration plan is a storage conversion plan, with 252 length pvc name",
			args: args{
				pvcRef: &corev1.ObjectReference{Name: strings.Repeat("b", 252), Namespace: "test"},
				migPlan: &migapi.MigPlan{
					Spec: migapi.MigPlanSpec{
						PersistentVolumes: migapi.PersistentVolumes{
							List: []migapi.PV{},
						},
					},
					Status: migapi.MigPlanStatus{
						Suffix: &suffix,
						Conditions: migapi.Conditions{
							List: []migapi.Condition{
								{
									Type:   migapi.MigrationTypeIdentified,
									Reason: string(migapi.StorageConversionPlan),
								},
							},
						},
					},
				},
			},
			want: strings.Repeat("b", 252) + ":" + strings.Repeat("b", 247) + "-mig-a",
		},
		{
			name: "Migration plan is a storage conversion plan, with 247 length pvc name",
			args: args{
				pvcRef: &corev1.ObjectReference{Name: strings.Repeat("b", 247), Namespace: "test"},
				migPlan: &migapi.MigPlan{
					Spec: migapi.MigPlanSpec{
						PersistentVolumes: migapi.PersistentVolumes{
							List: []migapi.PV{},
						},
					},
					Status: migapi.MigPlanStatus{
						Suffix: &suffix,
						Conditions: migapi.Conditions{
							List: []migapi.Condition{
								{
									Type:   migapi.MigrationTypeIdentified,
									Reason: string(migapi.StorageConversionPlan),
								},
							},
						},
					},
				},
			},
			want: strings.Repeat("b", 247) + ":" + strings.Repeat("b", 247) + "-mig-a",
		},
		{
			name: "Migration plan is a storage conversion plan, with a statefulset volume",
			args: args{
				pvcRef: &corev1.ObjectReference{Name: "test-set-2", Namespace: "test"},
				podList: []corev1.Pod{
					{
						ObjectMeta: metav1.ObjectMeta{
							Name:      "test-0",
							Namespace: "test",
							OwnerReferences: []metav1.OwnerReference{
								{
									Kind: "StatefulSet",
									Name: "set",
								},
							},
						},
						Spec: corev1.PodSpec{
							Volumes: []corev1.Volume{
								{
									Name: "vol-0",
									VolumeSource: corev1.VolumeSource{
										PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{
											ClaimName: "test-set-2",
										},
									},
								},
							},
						},
					},
				},
				migPlan: &migapi.MigPlan{
					Spec: migapi.MigPlanSpec{
						PersistentVolumes: migapi.PersistentVolumes{
							List: []migapi.PV{},
						},
					},
					Status: migapi.MigPlanStatus{
						Suffix: &suffix,
						Conditions: migapi.Conditions{
							List: []migapi.Condition{
								{
									Type:   migapi.MigrationTypeIdentified,
									Reason: string(migapi.StorageConversionPlan),
								},
							},
						},
					},
				},
			},
			want: "test-set-2:test-mig-abcd-set-2",
		},
		{
			name: "Migration plan is a storage conversion plan, with -new name",
			args: args{
				pvcRef: &corev1.ObjectReference{Name: "test-new", Namespace: "test"},
				migPlan: &migapi.MigPlan{
					Spec: migapi.MigPlanSpec{
						PersistentVolumes: migapi.PersistentVolumes{
							List: []migapi.PV{},
						},
					},
					Status: migapi.MigPlanStatus{
						Suffix: &suffix,
						Conditions: migapi.Conditions{
							List: []migapi.Condition{
								{
									Type:   migapi.MigrationTypeIdentified,
									Reason: string(migapi.StorageConversionPlan),
								},
							},
						},
					},
				},
			},
			want: "test-new:test-mig-abcd",
		},
		{
			name: "Migration plan is a storage conversion plan, with a statefulset volume",
			args: args{
				pvcRef: &corev1.ObjectReference{Name: "test-new-set-2", Namespace: "test"},
				podList: []corev1.Pod{
					{
						ObjectMeta: metav1.ObjectMeta{
							Name:      "test-0",
							Namespace: "test",
							OwnerReferences: []metav1.OwnerReference{
								{
									Kind: "StatefulSet",
									Name: "set",
								},
							},
						},
						Spec: corev1.PodSpec{
							Volumes: []corev1.Volume{
								{
									Name: "vol-0",
									VolumeSource: corev1.VolumeSource{
										PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{
											ClaimName: "test-new-set-2",
										},
									},
								},
							},
						},
					},
				},
				migPlan: &migapi.MigPlan{
					Spec: migapi.MigPlanSpec{
						PersistentVolumes: migapi.PersistentVolumes{
							List: []migapi.PV{},
						},
					},
					Status: migapi.MigPlanStatus{
						Suffix: &suffix,
						Conditions: migapi.Conditions{
							List: []migapi.Condition{
								{
									Type:   migapi.MigrationTypeIdentified,
									Reason: string(migapi.StorageConversionPlan),
								},
							},
						},
					},
				},
			},
			want: "test-new-set-2:test-mig-abcd-set-2",
		},
		{
			name: "Migration plan is a storage conversion plan, with -new name",
			args: args{
				pvcRef: &corev1.ObjectReference{Name: "test-mig-vwxd", Namespace: "test"},
				migPlan: &migapi.MigPlan{
					Spec: migapi.MigPlanSpec{
						PersistentVolumes: migapi.PersistentVolumes{
							List: []migapi.PV{},
						},
					},
					Status: migapi.MigPlanStatus{
						Suffix: &suffix,
						Conditions: migapi.Conditions{
							List: []migapi.Condition{
								{
									Type:   migapi.MigrationTypeIdentified,
									Reason: string(migapi.StorageConversionPlan),
								},
							},
						},
					},
				},
			},
			want: "test-mig-vwxd:test-mig-abcd",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := getMappedNameForPVC(tt.args.pvcRef, tt.args.podList, tt.args.migPlan)
			gotSplit := strings.Split(got, ":")
			gotTarget := gotSplit[0]
			if len(gotSplit) > 1 {
				gotTarget = gotSplit[1]
			}
			if len(gotTarget) > 253 {
				t.Errorf("getMappedNameForPVC() = %v, want %v", len(got), 253)
			}
			if got != tt.want {
				t.Errorf("getMappedNameForPVC() = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_createPVCToOwnerTypeMap(t *testing.T) {
	type args struct {
		podList []corev1.Pod
		migPlan ReconcileMigPlan
	}
	tests := []struct {
		name    string
		args    args
		want    map[string]migapi.OwnerType
		wantErr bool
	}{
		{
			name: "empty podlist",
			args: args{
				podList: []corev1.Pod{},
				migPlan: ReconcileMigPlan{},
			},
			want: map[string]migapi.OwnerType{},
		},
		{
			name: "pod with no owner, and single pvc",
			args: args{
				podList: []corev1.Pod{
					{
						ObjectMeta: metav1.ObjectMeta{
							Name: "pod-0",
						},
						Spec: corev1.PodSpec{
							Volumes: createPodVolumes([]string{"pvc-0"}),
						},
					},
				},
				migPlan: ReconcileMigPlan{},
			},
			want: map[string]migapi.OwnerType{
				"pvc-0": migapi.Unknown,
			},
		},
		{
			name: "pod with stateful set owner, and single pvc",
			args: args{
				podList: []corev1.Pod{
					{
						ObjectMeta: metav1.ObjectMeta{
							Name: "pod-0",
							OwnerReferences: []metav1.OwnerReference{
								{
									Kind:       "StatefulSet",
									APIVersion: "apps/v1",
								},
							},
						},
						Spec: corev1.PodSpec{
							Volumes: createPodVolumes([]string{"pvc-0"}),
						},
					},
				},
				migPlan: ReconcileMigPlan{},
			},
			want: map[string]migapi.OwnerType{
				"pvc-0": migapi.StatefulSet,
			},
		},
		{
			name: "pod with deployment through replicaset owner, and single pvc",
			args: args{
				podList: []corev1.Pod{
					{
						ObjectMeta: metav1.ObjectMeta{
							Name:      "pod-0",
							Namespace: "default",
							OwnerReferences: []metav1.OwnerReference{
								{
									Kind:       "ReplicaSet",
									APIVersion: "apps/v1",
									Name:       "rs-0",
								},
							},
						},
						Spec: corev1.PodSpec{
							Volumes: createPodVolumes([]string{"pvc-0"}),
						},
					},
				},
				migPlan: ReconcileMigPlan{
					Client: fake.NewFakeClient(&appsv1.ReplicaSet{
						ObjectMeta: metav1.ObjectMeta{
							Name:      "rs-0",
							Namespace: "default",
							OwnerReferences: []metav1.OwnerReference{
								{
									Kind:       "Deployment",
									APIVersion: "apps/v1",
								},
							},
						},
					}),
				},
			},
			want: map[string]migapi.OwnerType{
				"pvc-0": migapi.Deployment,
			},
		},
		{
			name: "pod with replicaset owner, replicaset not found, and single pvc",
			args: args{
				podList: []corev1.Pod{
					{
						ObjectMeta: metav1.ObjectMeta{
							Name:      "pod-0",
							Namespace: "default",
							OwnerReferences: []metav1.OwnerReference{
								{
									Kind:       "ReplicaSet",
									APIVersion: "apps/v1",
									Name:       "rs-0",
								},
							},
						},
						Spec: corev1.PodSpec{
							Volumes: createPodVolumes([]string{"pvc-0"}),
						},
					},
				},
				migPlan: ReconcileMigPlan{
					Client: fake.NewFakeClient(),
				},
			},
			want: map[string]migapi.OwnerType{
				"pvc-0": migapi.ReplicaSet,
			},
		},
		{
			name: "pod with replicaset owner, and single pvc",
			args: args{
				podList: []corev1.Pod{
					{
						ObjectMeta: metav1.ObjectMeta{
							Name:      "pod-0",
							Namespace: "default",
							OwnerReferences: []metav1.OwnerReference{
								{
									Kind:       "ReplicaSet",
									APIVersion: "apps/v1",
									Name:       "rs-0",
								},
							},
						},
						Spec: corev1.PodSpec{
							Volumes: createPodVolumes([]string{"pvc-0"}),
						},
					},
				},
				migPlan: ReconcileMigPlan{
					Client: fake.NewFakeClient(&appsv1.ReplicaSet{
						ObjectMeta: metav1.ObjectMeta{
							Name:      "rs-0",
							Namespace: "default",
						},
					}),
				},
			},
			want: map[string]migapi.OwnerType{
				"pvc-0": migapi.ReplicaSet,
			},
		},
		{
			name: "pod with deployment owner, and single pvc",
			args: args{
				podList: []corev1.Pod{
					{
						ObjectMeta: metav1.ObjectMeta{
							Name:      "pod-0",
							Namespace: "default",
							OwnerReferences: []metav1.OwnerReference{
								{
									Kind:       "Deployment",
									APIVersion: "apps/v1",
								},
							},
						},
						Spec: corev1.PodSpec{
							Volumes: createPodVolumes([]string{"pvc-0"}),
						},
					},
				},
				migPlan: ReconcileMigPlan{},
			},
			want: map[string]migapi.OwnerType{
				"pvc-0": migapi.Deployment,
			},
		},
		{
			name: "pod with daemonset owner, and single pvc",
			args: args{
				podList: []corev1.Pod{
					{
						ObjectMeta: metav1.ObjectMeta{
							Name:      "pod-0",
							Namespace: "default",
							OwnerReferences: []metav1.OwnerReference{
								{
									Kind:       "DaemonSet",
									APIVersion: "apps/v1",
								},
							},
						},
						Spec: corev1.PodSpec{
							Volumes: createPodVolumes([]string{"pvc-0"}),
						},
					},
				},
				migPlan: ReconcileMigPlan{},
			},
			want: map[string]migapi.OwnerType{
				"pvc-0": migapi.DaemonSet,
			},
		},
		{
			name: "pod with job owner, and single pvc",
			args: args{
				podList: []corev1.Pod{
					{
						ObjectMeta: metav1.ObjectMeta{
							Name:      "pod-0",
							Namespace: "default",
							OwnerReferences: []metav1.OwnerReference{
								{
									Kind:       "Job",
									APIVersion: "batch/v1",
								},
							},
						},
						Spec: corev1.PodSpec{
							Volumes: createPodVolumes([]string{"pvc-0"}),
						},
					},
				},
				migPlan: ReconcileMigPlan{},
			},
			want: map[string]migapi.OwnerType{
				"pvc-0": migapi.Job,
			},
		},
		{
			name: "pod with cron job owner, and single pvc",
			args: args{
				podList: []corev1.Pod{
					{
						ObjectMeta: metav1.ObjectMeta{
							Name:      "pod-0",
							Namespace: "default",
							OwnerReferences: []metav1.OwnerReference{
								{
									Kind:       "CronJob",
									APIVersion: "batch/v1",
								},
							},
						},
						Spec: corev1.PodSpec{
							Volumes: createPodVolumes([]string{"pvc-0"}),
						},
					},
				},
				migPlan: ReconcileMigPlan{},
			},
			want: map[string]migapi.OwnerType{
				"pvc-0": migapi.CronJob,
			},
		},
		{
			name: "pod with VMI owner, and single pvc",
			args: args{
				podList: []corev1.Pod{
					{
						ObjectMeta: metav1.ObjectMeta{
							Name:      "pod-0",
							Namespace: "default",
							OwnerReferences: []metav1.OwnerReference{
								{
									Kind:       "VirtualMachineInstance",
									APIVersion: "kubevirt.io/v1",
								},
							},
						},
						Spec: corev1.PodSpec{
							Volumes: createPodVolumes([]string{"pvc-0"}),
						},
					},
				},
				migPlan: ReconcileMigPlan{},
			},
			want: map[string]migapi.OwnerType{
				"pvc-0": migapi.VirtualMachine,
			},
		},
		{
			name: "pod with VMI owner, and multiple pvcs",
			args: args{
				podList: []corev1.Pod{
					{
						ObjectMeta: metav1.ObjectMeta{
							Name:      "pod-0",
							Namespace: "default",
							OwnerReferences: []metav1.OwnerReference{
								{
									Kind:       "VirtualMachineInstance",
									APIVersion: "kubevirt.io/v1",
								},
							},
						},
						Spec: corev1.PodSpec{
							Volumes: createPodVolumes([]string{"pvc-0", "pvc-1", "pvc-2"}),
						},
					},
				},
				migPlan: ReconcileMigPlan{},
			},
			want: map[string]migapi.OwnerType{
				"pvc-0": migapi.VirtualMachine,
				"pvc-1": migapi.VirtualMachine,
				"pvc-2": migapi.VirtualMachine,
			},
		},
		{
			name: "hotplug pod with pod owner, and single pvc",
			args: args{
				podList: []corev1.Pod{
					{
						ObjectMeta: metav1.ObjectMeta{
							Name:      "hp-test",
							Namespace: "default",
							OwnerReferences: []metav1.OwnerReference{
								{
									Kind:       "Pod",
									APIVersion: "",
								},
							},
						},
						Spec: corev1.PodSpec{
							Volumes: createPodVolumes([]string{"pvc-0"}),
						},
					},
				},
				migPlan: ReconcileMigPlan{},
			},
			want: map[string]migapi.OwnerType{
				"pvc-0": migapi.VirtualMachine,
			},
		},
		{
			name: "pod with unknown owner, and single pvc",
			args: args{
				podList: []corev1.Pod{
					{
						ObjectMeta: metav1.ObjectMeta{
							Name:      "hp-test",
							Namespace: "default",
							OwnerReferences: []metav1.OwnerReference{
								{
									Kind:       "Something",
									APIVersion: "unknown",
								},
							},
						},
						Spec: corev1.PodSpec{
							Volumes: createPodVolumes([]string{"pvc-0"}),
						},
					},
				},
				migPlan: ReconcileMigPlan{},
			},
			want: map[string]migapi.OwnerType{
				"pvc-0": migapi.Unknown,
			},
		},
		{
			name: "single pvc, owned by multiple pods with different types",
			args: args{
				podList: []corev1.Pod{
					{
						ObjectMeta: metav1.ObjectMeta{
							Name:      "pod-0",
							Namespace: "default",
							OwnerReferences: []metav1.OwnerReference{
								{
									Kind:       "VirtualMachineInstance",
									APIVersion: "kubevirt.io/v1",
								},
							},
						},
						Spec: corev1.PodSpec{
							Volumes: createPodVolumes([]string{"pvc-0"}),
						},
					},
					{
						ObjectMeta: metav1.ObjectMeta{
							Name:      "pod-0",
							Namespace: "default",
							OwnerReferences: []metav1.OwnerReference{
								{
									Kind:       "DaemonSet",
									APIVersion: "apps/v1",
								},
							},
						},
						Spec: corev1.PodSpec{
							Volumes: createPodVolumes([]string{"pvc-0"}),
						},
					},
				},
				migPlan: ReconcileMigPlan{},
			},
			want: map[string]migapi.OwnerType{
				"pvc-0": migapi.Unknown,
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := tt.args.migPlan.createPVCToOwnerTypeMap(tt.args.podList)
			if (err != nil) != tt.wantErr {
				t.Errorf("createPVCToOwnerTypeMap() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			for k, v := range got {
				if tt.want[k] != v {
					t.Errorf("createPVCToOwnerTypeMap() = %v, want %v", got, tt.want)
				}
			}
		})
	}
}

func createPodVolumes(pvcNames []string) []corev1.Volume {
	volumes := make([]corev1.Volume, len(pvcNames))
	for _, pvcName := range pvcNames {
		volumes = append(volumes, corev1.Volume{
			Name: "pvcName",
			VolumeSource: corev1.VolumeSource{
				PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{
					ClaimName: pvcName,
				},
			},
		})
	}
	return volumes
}

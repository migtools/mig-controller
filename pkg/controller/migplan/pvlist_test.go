package migplan

import (
	"fmt"
	"strings"
	"testing"

	migapi "github.com/konveyor/mig-controller/pkg/apis/migration/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
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
			want: strings.Repeat("b", 252),
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
			want: fmt.Sprintf("%s:%s", strings.Repeat("b", 247), strings.Repeat("b", 247)+"-mig-a"),
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
			if got := getMappedNameForPVC(tt.args.pvcRef, tt.args.podList, tt.args.migPlan); got != tt.want {
				t.Errorf("getMappedNameForPVC() = %v, want %v", got, tt.want)
			}
		})
	}
}

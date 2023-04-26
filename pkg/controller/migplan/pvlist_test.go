package migplan

import (
	"fmt"
	"testing"

	migapi "github.com/konveyor/mig-controller/pkg/apis/migration/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func Test_getStatefulSetVolumeName(t *testing.T) {
	tests := []struct {
		name    string
		pvcName string
		setName string
		want    string
	}{
		{
			name:    "given a statefulset volume with single ordinal value, must return correctly formatted name",
			pvcName: "example-set-0",
			setName: "set",
			want:    fmt.Sprintf("example-%s-set-0", migapi.StorageConversionPVCNamePrefix),
		},
		{
			name:    "given a statefulset volume with double ordinal value, must return correctly formatted name",
			pvcName: "example-set-10",
			setName: "set",
			want:    fmt.Sprintf("example-%s-set-10", migapi.StorageConversionPVCNamePrefix),
		},
		{
			name:    "given a volume with no ordinal, must return original name",
			pvcName: "example-set",
			want:    "example-set-new",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := getStatefulSetVolumeName(tt.pvcName, tt.setName); got != tt.want {
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

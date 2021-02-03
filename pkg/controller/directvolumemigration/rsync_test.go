package directvolumemigration

import (
	migapi "github.com/konveyor/mig-controller/pkg/apis/migration/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"testing"
	"time"
)

func Test_hasAllRsyncClientPodsTimedOut(t *testing.T) {
	type args struct {
		pvcMap  map[string][]pvcMapElement
		client  client.Client
		dvmName string
	}
	tests := []struct {
		name    string
		args    args
		want    bool
		wantErr bool
	}{
		{
			name: "both rsync clients pods succeeded",
			args: args{
				pvcMap: map[string][]pvcMapElement{"foo": {{
					Name:   "pvc-0",
					Verify: false,
				}, {
					Name:   "pvc-1",
					Verify: false,
				}}},
				client: fake.NewFakeClient(&migapi.DirectVolumeMigrationProgress{
					ObjectMeta: metav1.ObjectMeta{
						Name:      getMD5Hash("test" + "pvc-0" + "foo"),
						Namespace: migapi.OpenshiftMigrationNamespace,
					},
					Spec:   migapi.DirectVolumeMigrationProgressSpec{},
					Status: migapi.DirectVolumeMigrationProgressStatus{PodPhase: corev1.PodSucceeded, ContainerElapsedTime: &metav1.Duration{Duration: time.Second * 20}},
				}, &migapi.DirectVolumeMigrationProgress{
					ObjectMeta: metav1.ObjectMeta{
						Name:      getMD5Hash("test" + "pvc-1" + "foo"),
						Namespace: migapi.OpenshiftMigrationNamespace,
					},
					Spec:   migapi.DirectVolumeMigrationProgressSpec{},
					Status: migapi.DirectVolumeMigrationProgressStatus{PodPhase: corev1.PodSucceeded, ContainerElapsedTime: &metav1.Duration{Duration: time.Second * 20}},
				}),
				dvmName: "test",
			},
			want:    false,
			wantErr: false,
		},
		{
			name: "both rsync clients failed with 20 seconds",
			args: args{
				pvcMap: map[string][]pvcMapElement{"foo": {{
					Name:   "pvc-0",
					Verify: false,
				}, {
					Name:   "pvc-1",
					Verify: false,
				}}},
				client: fake.NewFakeClient(&migapi.DirectVolumeMigrationProgress{
					ObjectMeta: metav1.ObjectMeta{
						Name:      getMD5Hash("test" + "pvc-0" + "foo"),
						Namespace: migapi.OpenshiftMigrationNamespace,
					},
					Spec:   migapi.DirectVolumeMigrationProgressSpec{},
					Status: migapi.DirectVolumeMigrationProgressStatus{PodPhase: corev1.PodFailed, ContainerElapsedTime: &metav1.Duration{Duration: time.Second * 20}},
				}, &migapi.DirectVolumeMigrationProgress{
					ObjectMeta: metav1.ObjectMeta{
						Name:      getMD5Hash("test" + "pvc-1" + "foo"),
						Namespace: migapi.OpenshiftMigrationNamespace,
					},
					Spec:   migapi.DirectVolumeMigrationProgressSpec{},
					Status: migapi.DirectVolumeMigrationProgressStatus{PodPhase: corev1.PodFailed, ContainerElapsedTime: &metav1.Duration{Duration: time.Second * 20}},
				}),
				dvmName: "test",
			},
			want:    true,
			wantErr: false,
		},
		{
			name: "one rsync client pod failed with 20 second timeout other succeeded",
			args: args{
				pvcMap: map[string][]pvcMapElement{"foo": {{
					Name:   "pvc-0",
					Verify: false,
				}, {
					Name:   "pvc-1",
					Verify: false,
				}}},
				client: fake.NewFakeClient(&migapi.DirectVolumeMigrationProgress{
					ObjectMeta: metav1.ObjectMeta{
						Name:      getMD5Hash("test" + "pvc-0" + "foo"),
						Namespace: migapi.OpenshiftMigrationNamespace,
					},
					Spec:   migapi.DirectVolumeMigrationProgressSpec{},
					Status: migapi.DirectVolumeMigrationProgressStatus{PodPhase: corev1.PodSucceeded, ContainerElapsedTime: &metav1.Duration{Duration: time.Second * 20}},
				}, &migapi.DirectVolumeMigrationProgress{
					ObjectMeta: metav1.ObjectMeta{
						Name:      getMD5Hash("test" + "pvc-1" + "foo"),
						Namespace: migapi.OpenshiftMigrationNamespace,
					},
					Spec:   migapi.DirectVolumeMigrationProgressSpec{},
					Status: migapi.DirectVolumeMigrationProgressStatus{PodPhase: corev1.PodFailed, ContainerElapsedTime: &metav1.Duration{Duration: time.Second * 20}},
				}),
				dvmName: "test",
			},
			want:    false,
			wantErr: false,
		},
		{
			name: "one rsync client pod failed with 20.49 and other with 20.50 second, expect false",
			args: args{
				pvcMap: map[string][]pvcMapElement{"foo": {{
					Name:   "pvc-0",
					Verify: false,
				}, {
					Name:   "pvc-1",
					Verify: false,
				}}},
				client: fake.NewFakeClient(&migapi.DirectVolumeMigrationProgress{
					ObjectMeta: metav1.ObjectMeta{
						Name:      getMD5Hash("test" + "pvc-0" + "foo"),
						Namespace: migapi.OpenshiftMigrationNamespace,
					},
					Spec:   migapi.DirectVolumeMigrationProgressSpec{},
					Status: migapi.DirectVolumeMigrationProgressStatus{PodPhase: corev1.PodFailed, ContainerElapsedTime: &metav1.Duration{Duration: time.Millisecond * 20490}},
				}, &migapi.DirectVolumeMigrationProgress{
					ObjectMeta: metav1.ObjectMeta{
						Name:      getMD5Hash("test" + "pvc-1" + "foo"),
						Namespace: migapi.OpenshiftMigrationNamespace,
					},
					Spec:   migapi.DirectVolumeMigrationProgressSpec{},
					Status: migapi.DirectVolumeMigrationProgressStatus{PodPhase: corev1.PodFailed, ContainerElapsedTime: &metav1.Duration{Duration: time.Millisecond * 20500}},
				}),
				dvmName: "test",
			},
			want:    false,
			wantErr: false,
		},
		{
			name: "one rsync client pod failed with 20.49 and other with 20.45 second, expect true",
			args: args{
				pvcMap: map[string][]pvcMapElement{"foo": {{
					Name:   "pvc-0",
					Verify: false,
				}, {
					Name:   "pvc-1",
					Verify: false,
				}}},
				client: fake.NewFakeClient(&migapi.DirectVolumeMigrationProgress{
					ObjectMeta: metav1.ObjectMeta{
						Name:      getMD5Hash("test" + "pvc-0" + "foo"),
						Namespace: migapi.OpenshiftMigrationNamespace,
					},
					Spec:   migapi.DirectVolumeMigrationProgressSpec{},
					Status: migapi.DirectVolumeMigrationProgressStatus{PodPhase: corev1.PodFailed, ContainerElapsedTime: &metav1.Duration{Duration: time.Millisecond * 20490}},
				}, &migapi.DirectVolumeMigrationProgress{
					ObjectMeta: metav1.ObjectMeta{
						Name:      getMD5Hash("test" + "pvc-1" + "foo"),
						Namespace: migapi.OpenshiftMigrationNamespace,
					},
					Spec:   migapi.DirectVolumeMigrationProgressSpec{},
					Status: migapi.DirectVolumeMigrationProgressStatus{PodPhase: corev1.PodFailed, ContainerElapsedTime: &metav1.Duration{Duration: time.Millisecond * 20450}},
				}),
				dvmName: "test",
			},
			want:    true,
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := hasAllRsyncClientPodsTimedOut(tt.args.pvcMap, tt.args.client, tt.args.dvmName)
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

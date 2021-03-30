package directvolumemigrationprogress

import (
	"testing"

	migapi "github.com/konveyor/mig-controller/pkg/apis/migration/v1alpha1"
	"github.com/konveyor/mig-controller/pkg/compat"
	fakecompat "github.com/konveyor/mig-controller/pkg/compat/fake"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestReconcileDirectVolumeMigrationProgress_validateSpec(t *testing.T) {
	type args struct {
		srcClient  compat.Client
		pvProgress *migapi.DirectVolumeMigrationProgress
	}
	tests := []struct {
		name              string
		args              args
		wantCondition     *migapi.Condition
		dontWantCondition *migapi.Condition
		wantErr           bool
	}{
		{
			name: "when podRef is set and podselector is not set, should not have blocker condition",
			args: args{
				srcClient: fakecompat.NewFakeClient(&corev1.Pod{ObjectMeta: metav1.ObjectMeta{Name: "pod-1", Namespace: "ns-1"}, Status: corev1.PodStatus{Phase: corev1.PodSucceeded}}),
				pvProgress: &migapi.DirectVolumeMigrationProgress{
					ObjectMeta: metav1.ObjectMeta{Name: "dvmp", Namespace: "openshift-migration"},
					Spec: migapi.DirectVolumeMigrationProgressSpec{
						PodRef: &corev1.ObjectReference{Name: "pod-1", Namespace: "ns-1"},
					},
				},
			},
			wantErr:           false,
			wantCondition:     nil,
			dontWantCondition: &migapi.Condition{Type: InvalidPod, Category: migapi.Critical},
		},
		{
			name: "when podRef is not set and podselector is set but podNamespace is not set, should have blocker condition",
			args: args{
				srcClient: fakecompat.NewFakeClient(&corev1.Pod{ObjectMeta: metav1.ObjectMeta{Name: "pod-1", Namespace: "ns-1"}, Status: corev1.PodStatus{Phase: corev1.PodSucceeded}}),
				pvProgress: &migapi.DirectVolumeMigrationProgress{
					ObjectMeta: metav1.ObjectMeta{Name: "dvmp", Namespace: "openshift-migration"},
					Spec: migapi.DirectVolumeMigrationProgressSpec{
						PodSelector: map[string]string{migapi.RsyncPodIdentityLabel: "pod-1"},
					},
				},
			},
			wantErr:           false,
			wantCondition:     &migapi.Condition{Type: InvalidSpec, Category: migapi.Critical},
			dontWantCondition: nil,
		},
		{
			name: "when podRef is not set and podselector and podNamespace are set, should not have blocker condition",
			args: args{
				srcClient: fakecompat.NewFakeClient(&corev1.Pod{ObjectMeta: metav1.ObjectMeta{Name: "pod-1", Namespace: "ns-1"}, Status: corev1.PodStatus{Phase: corev1.PodSucceeded}}),
				pvProgress: &migapi.DirectVolumeMigrationProgress{
					ObjectMeta: metav1.ObjectMeta{Name: "dvmp", Namespace: "openshift-migration"},
					Spec: migapi.DirectVolumeMigrationProgressSpec{
						PodSelector: map[string]string{migapi.RsyncPodIdentityLabel: "pod-1"},
					},
				},
			},
			wantErr:           false,
			wantCondition:     nil,
			dontWantCondition: &migapi.Condition{Type: InvalidPod, Category: migapi.Critical},
		},
		{
			name: "when required specs are missing, should have blocker condition",
			args: args{
				srcClient: fakecompat.NewFakeClient(&corev1.Pod{ObjectMeta: metav1.ObjectMeta{Name: "pod-1", Namespace: "ns-1"}, Status: corev1.PodStatus{Phase: corev1.PodSucceeded}}),
				pvProgress: &migapi.DirectVolumeMigrationProgress{
					ObjectMeta: metav1.ObjectMeta{Name: "dvmp", Namespace: "openshift-migration"},
					Spec: migapi.DirectVolumeMigrationProgressSpec{
						PodRef:      &corev1.ObjectReference{Namespace: "ns"},
						PodSelector: map[string]string{"invalidLabel": "val"},
					},
				},
			},
			wantErr:           false,
			wantCondition:     &migapi.Condition{Type: InvalidSpec, Category: migapi.Critical},
			dontWantCondition: nil,
		},
		{
			name: "when podselector is set but doesn't have pod identity label in it, should have blocker condition",
			args: args{
				srcClient: fakecompat.NewFakeClient(&corev1.Pod{ObjectMeta: metav1.ObjectMeta{Name: "pod-1", Namespace: "ns-1"}, Status: corev1.PodStatus{Phase: corev1.PodSucceeded}}),
				pvProgress: &migapi.DirectVolumeMigrationProgress{
					ObjectMeta: metav1.ObjectMeta{Name: "dvmp", Namespace: "openshift-migration"},
					Spec: migapi.DirectVolumeMigrationProgressSpec{
						PodSelector:  map[string]string{},
						PodNamespace: "ns",
					},
				},
			},
			wantErr:           false,
			wantCondition:     &migapi.Condition{Type: InvalidPodSelector, Category: migapi.Critical},
			dontWantCondition: nil,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := &ReconcileDirectVolumeMigrationProgress{}
			if err := r.validateSpec(tt.args.srcClient, tt.args.pvProgress); (err != nil) != tt.wantErr {
				t.Errorf("ReconcileDirectVolumeMigrationProgress.validateSpec() error = %v, wantErr %v", err, tt.wantErr)
			}
			if tt.wantCondition != nil && !tt.args.pvProgress.Status.HasCondition(tt.wantCondition.Type) {
				t.Errorf("ReconcileDirectVolumeMigrationProgress.validateSpec() expected condition of type %s not found", tt.wantCondition.Type)
			}
			if tt.dontWantCondition != nil && tt.args.pvProgress.Status.HasCondition(tt.dontWantCondition.Type) {
				t.Errorf("ReconcileDirectVolumeMigrationProgress.validateSpec() unexpected condition of type %s found", tt.wantCondition.Type)
			}
		})
	}
}

package migmigration

import (
	"reflect"
	"testing"

	migapi "github.com/konveyor/mig-controller/pkg/apis/migration/v1alpha1"
	dvmc "github.com/konveyor/mig-controller/pkg/controller/directvolumemigration"
	v1 "k8s.io/api/core/v1"
)

func TestTask_hasDirectVolumeMigrationCompleted(t1 *testing.T) {
	type args struct {
		dvm *migapi.DirectVolumeMigration
	}
	tests := []struct {
		name               string
		args               args
		wantCompleted      bool
		wantFailureReasons []string
		wantProgress       []string
	}{
		{
			name: "system wide network failure",
			args: args{dvm: &migapi.DirectVolumeMigration{
				Spec: migapi.DirectVolumeMigrationSpec{
					PersistentVolumeClaims: []migapi.PVCToMigrate{
						{
							ObjectReference: &v1.ObjectReference{
								Namespace: "ns",
								Name:      "foo",
							},
						},
					},
				},
				Status: migapi.DirectVolumeMigrationStatus{
					Conditions: migapi.Conditions{
						List: []migapi.Condition{
							{
								Type:   dvmc.Failed,
								Status: True,
							},
						},
					},
					Phase: dvmc.MigrationFailed,
					FailedPods: []*migapi.PodProgress{
						{
							PVCReference: &v1.ObjectReference{
								Namespace: "ns",
								Name:      "pvc-0",
							},
							ObjectReference: &v1.ObjectReference{
								Namespace: "ns",
								Name:      "foo",
							},
						},
					},
				},
			}},
			wantProgress:       []string{"[pvc-0] ns/foo: Failed"},
			wantFailureReasons: []string{"direct volume migration failed. 1 total volumes; 0 successful; 0 running; 1 failed"},
			wantCompleted:      true,
		},
		{
			name: "all client pods succeeded",
			args: args{dvm: &migapi.DirectVolumeMigration{
				Spec: migapi.DirectVolumeMigrationSpec{
					PersistentVolumeClaims: []migapi.PVCToMigrate{
						{
							ObjectReference: &v1.ObjectReference{
								Namespace: "ns",
								Name:      "foo",
							},
						},
					},
				},
				Status: migapi.DirectVolumeMigrationStatus{
					Conditions: migapi.Conditions{
						List: []migapi.Condition{
							{
								Type:   dvmc.Succeeded,
								Status: True,
							},
						},
					},
					Itinerary: dvmc.VolumeMigration.Name,
					Phase:     dvmc.Completed,
					SuccessfulPods: []*migapi.PodProgress{
						{
							PVCReference: &v1.ObjectReference{
								Namespace: "ns",
								Name:      "pvc-0",
							},
							ObjectReference: &v1.ObjectReference{
								Namespace: "ns",
								Name:      "foo",
							},
							LastObservedProgressPercent: "100%",
						},
					},
				},
			}},
			wantProgress:       []string{"[pvc-0] ns/foo:  100% completed"},
			wantFailureReasons: nil,
			wantCompleted:      true,
		},
	}
	for _, tt := range tests {
		t1.Run(tt.name, func(t1 *testing.T) {
			t := &Task{}
			gotCompleted, gotFailureReasons, gotProgress := t.hasDirectVolumeMigrationCompleted(tt.args.dvm)
			if gotCompleted != tt.wantCompleted {
				t1.Errorf("hasDirectVolumeMigrationCompleted() gotCompleted = %v, want %v", gotCompleted, tt.wantCompleted)
			}
			if !reflect.DeepEqual(gotFailureReasons, tt.wantFailureReasons) {
				t1.Errorf("hasDirectVolumeMigrationCompleted() gotFailureReasons = %v, want %v", gotFailureReasons, tt.wantFailureReasons)
			}
			if !reflect.DeepEqual(gotProgress, tt.wantProgress) {
				t1.Errorf("hasDirectVolumeMigrationCompleted() gotProgress = %v, want %v", gotProgress, tt.wantProgress)
			}
		})
	}
}

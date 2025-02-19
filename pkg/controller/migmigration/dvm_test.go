package migmigration

import (
	"reflect"
	"slices"
	"testing"

	migapi "github.com/konveyor/mig-controller/pkg/apis/migration/v1alpha1"
	dvmc "github.com/konveyor/mig-controller/pkg/controller/directvolumemigration"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
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
					Errors: []string{"direct volume migration failed. 1 total volumes; 0 successful; 0 running; 1 failed"},
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
			wantProgress:       []string{"[pvc-0] ns/foo: Completed 100%"},
			wantFailureReasons: nil,
			wantCompleted:      true,
		},
		{
			name: "when PVCReference is not present on the PodProgress, pre-MTC-1.4.3 message should be shown",
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
							ObjectReference: &v1.ObjectReference{
								Namespace: "ns",
								Name:      "foo",
							},
							LastObservedProgressPercent: "100%",
						},
					},
				},
			}},
			wantProgress:       []string{"Rsync Pod ns/foo: Completed 100%"},
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

func TestTask_getDVMLiveMigrationProgress(t *testing.T) {
	tests := []struct {
		name             string
		dvm              *migapi.DirectVolumeMigration
		expectedProgress []string
	}{
		{
			name: "no dvm",
		},
		{
			name: "dvm with running live migrations, no message, no progress",
			dvm: &migapi.DirectVolumeMigration{
				Status: migapi.DirectVolumeMigrationStatus{
					RunningLiveMigrations: []*migapi.LiveMigrationProgress{
						{
							VMName:      "vm-0",
							VMNamespace: "ns",
							PVCReference: &v1.ObjectReference{
								Name:       "pvc-0",
								Kind:       "PersistentVolumeClaim",
								APIVersion: "",
							},
						},
					},
				},
			},
			expectedProgress: []string{"[pvc-0] Live Migration ns/vm-0: Running"},
		},
		{
			name: "dvm with failed live migrations, message, no progress",
			dvm: &migapi.DirectVolumeMigration{
				Status: migapi.DirectVolumeMigrationStatus{
					FailedLiveMigrations: []*migapi.LiveMigrationProgress{
						{
							VMName:      "vm-0",
							VMNamespace: "ns",
							PVCReference: &v1.ObjectReference{
								Name:       "pvc-0",
								Kind:       "PersistentVolumeClaim",
								APIVersion: "",
							},
							Message: "Failed because of test",
						},
					},
				},
			},
			expectedProgress: []string{"[pvc-0] Live Migration ns/vm-0: Failed Failed because of test"},
		},
		{
			name: "dvm with completed live migrations, message, progress",
			dvm: &migapi.DirectVolumeMigration{
				Status: migapi.DirectVolumeMigrationStatus{
					SuccessfulLiveMigrations: []*migapi.LiveMigrationProgress{
						{
							VMName:      "vm-0",
							VMNamespace: "ns",
							PVCReference: &v1.ObjectReference{
								Name:       "pvc-0",
								Kind:       "PersistentVolumeClaim",
								APIVersion: "",
							},
							Message:                     "Successfully completed",
							LastObservedProgressPercent: "100%",
						},
					},
				},
			},
			expectedProgress: []string{"[pvc-0] Live Migration ns/vm-0: Completed Successfully completed 100%"},
		},
		{
			name: "dvm with pending live migrations, message, blank progress",
			dvm: &migapi.DirectVolumeMigration{
				Status: migapi.DirectVolumeMigrationStatus{
					PendingLiveMigrations: []*migapi.LiveMigrationProgress{
						{
							VMName:      "vm-0",
							VMNamespace: "ns",
							PVCReference: &v1.ObjectReference{
								Name:       "pvc-0",
								Kind:       "PersistentVolumeClaim",
								APIVersion: "",
							},
							Message:                     "Pending",
							LastObservedProgressPercent: "",
						},
					},
				},
			},
			expectedProgress: []string{"[pvc-0] Live Migration ns/vm-0: Pending Pending"},
		},
		{
			name: "dvm with running live migrations, message, progress, transferrate, and elapsed time",
			dvm: &migapi.DirectVolumeMigration{
				Status: migapi.DirectVolumeMigrationStatus{
					RunningLiveMigrations: []*migapi.LiveMigrationProgress{
						{
							VMName:      "vm-0",
							VMNamespace: "ns",
							PVCReference: &v1.ObjectReference{
								Name:       "pvc-0",
								Kind:       "PersistentVolumeClaim",
								APIVersion: "",
							},
							Message:                     "Running",
							LastObservedProgressPercent: "50%",
							LastObservedTransferRate:    "10MB/s",
							TotalElapsedTime: &metav1.Duration{
								Duration: 1000,
							},
						},
					},
				},
			},
			expectedProgress: []string{"[pvc-0] Live Migration ns/vm-0: Running Running 50% (Transfer rate 10MB/s) (0s)"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			res := getDVMLiveMigrationProgress(tt.dvm)
			if len(res) != len(tt.expectedProgress) {
				t.Errorf("getDVMLiveMigrationProgress() = %v, want %v", res, tt.expectedProgress)
			}
			for _, p := range tt.expectedProgress {
				if !slices.Contains(res, p) {
					t.Errorf("getDVMLiveMigrationProgress() = %v, want %v", res, tt.expectedProgress)
				}
			}
		})
	}
}

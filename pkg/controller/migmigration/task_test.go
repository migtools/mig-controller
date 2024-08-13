package migmigration

import (
	"reflect"
	"testing"

	"github.com/go-logr/logr"
	migapi "github.com/konveyor/mig-controller/pkg/apis/migration/v1alpha1"
	dvmc "github.com/konveyor/mig-controller/pkg/controller/directvolumemigration"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestTask_getStagePVs(t *testing.T) {
	type fields struct {
		Log           logr.Logger
		PlanResources *migapi.PlanResources
		Phase         string
		Step          string
	}
	tests := []struct {
		name   string
		fields fields
		want   migapi.PersistentVolumes
	}{
		{
			name: "one move action pvc",
			fields: fields{
				Log: log.WithName("test_getStagePVs"),
				PlanResources: &migapi.PlanResources{
					MigPlan: &migapi.MigPlan{
						Spec: migapi.MigPlanSpec{
							IndirectVolumeMigration: false,
							PersistentVolumes: migapi.PersistentVolumes{List: []migapi.PV{
								{
									Name: "pvc-0",
									Selection: migapi.Selection{
										Action: migapi.PvMoveAction,
									},
								},
							}},
						},
						Status: migapi.MigPlanStatus{},
					},
				},
				Phase: "foo",
				Step:  "bar",
			},
			want: migapi.PersistentVolumes{List: []migapi.PV{
				{
					Name: "pvc-0",
					Selection: migapi.Selection{
						Action: migapi.PvMoveAction,
					},
				},
			}},
		},
		{
			name: "one move action pvc, one skip action pvc with restic copy",
			fields: fields{
				Log: log.WithName("test_getStagePVs"),
				PlanResources: &migapi.PlanResources{
					MigPlan: &migapi.MigPlan{
						Spec: migapi.MigPlanSpec{
							IndirectVolumeMigration: true,
							PersistentVolumes: migapi.PersistentVolumes{List: []migapi.PV{
								{
									Name: "pvc-0",
									Selection: migapi.Selection{
										Action: migapi.PvMoveAction,
									},
								},
								{
									Name: "pvc-1",
									Selection: migapi.Selection{
										Action:     migapi.PvCopyAction,
										CopyMethod: migapi.PvFilesystemCopyMethod,
									},
								},
							}},
						},
						Status: migapi.MigPlanStatus{},
					},
				},
				Phase: "foo",
				Step:  "bar",
			},
			want: migapi.PersistentVolumes{List: []migapi.PV{
				{
					Name: "pvc-0",
					Selection: migapi.Selection{
						Action: migapi.PvMoveAction,
					},
				},
				{
					Name: "pvc-1",
					Selection: migapi.Selection{
						Action:     migapi.PvCopyAction,
						CopyMethod: migapi.PvFilesystemCopyMethod,
					},
				},
			}},
		},
		{
			name: "one move action pvc, one copy action pvc with DVM",
			fields: fields{
				Log: log.WithName("test_getStagePVs"),
				PlanResources: &migapi.PlanResources{
					MigPlan: &migapi.MigPlan{
						Spec: migapi.MigPlanSpec{
							IndirectVolumeMigration: false,
							PersistentVolumes: migapi.PersistentVolumes{List: []migapi.PV{
								{
									Name: "pvc-0",
									Selection: migapi.Selection{
										Action: migapi.PvMoveAction,
									},
								},
								{
									Name: "pvc-1",
									Selection: migapi.Selection{
										Action:     migapi.PvCopyAction,
										CopyMethod: migapi.PvFilesystemCopyMethod,
									},
								},
							}},
						},
						Status: migapi.MigPlanStatus{},
					},
				},
				Phase: "foo",
				Step:  "bar",
			},
			want: migapi.PersistentVolumes{List: []migapi.PV{
				{
					Name: "pvc-0",
					Selection: migapi.Selection{
						Action: migapi.PvMoveAction,
					},
				},
			}},
		},
		{
			name: "one move action pvc, one skip action",
			fields: fields{
				Log: log.WithName("test_getStagePVs"),
				PlanResources: &migapi.PlanResources{
					MigPlan: &migapi.MigPlan{
						Spec: migapi.MigPlanSpec{
							IndirectVolumeMigration: false,
							PersistentVolumes: migapi.PersistentVolumes{List: []migapi.PV{
								{
									Name: "pvc-0",
									Selection: migapi.Selection{
										Action: migapi.PvMoveAction,
									},
								},
								{
									Name: "pvc-1",
									Selection: migapi.Selection{
										Action: migapi.PvSkipAction,
									},
								},
							}},
						},
						Status: migapi.MigPlanStatus{},
					},
				},
				Phase: "foo",
				Step:  "bar",
			},
			want: migapi.PersistentVolumes{List: []migapi.PV{
				{
					Name: "pvc-0",
					Selection: migapi.Selection{
						Action: migapi.PvMoveAction,
					},
				},
			}},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			task := &Task{
				Log:           tt.fields.Log,
				PlanResources: tt.fields.PlanResources,
				Phase:         tt.fields.Phase,
				Step:          tt.fields.Step,
			}
			if got := task.getStagePVs(); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("getStagePVs() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestTask_waitForDMVToComplete(t *testing.T) {
	tests := []struct {
		name               string
		step               string
		dvm                *migapi.DirectVolumeMigration
		initialConditions  []migapi.Condition
		expectedConditions []migapi.Condition
		wantErr            bool
	}{
		{
			name: "dvm uncompleted, no warnings",
			dvm:  &migapi.DirectVolumeMigration{},
			initialConditions: []migapi.Condition{
				{
					Type: DirectVolumeMigrationBlocked,
				},
			},
			expectedConditions: []migapi.Condition{},
			wantErr:            false,
		},
		{
			name: "dvm uncompleted, warnings",
			dvm: &migapi.DirectVolumeMigration{
				Status: migapi.DirectVolumeMigrationStatus{
					Conditions: migapi.Conditions{
						List: []migapi.Condition{
							{
								Category: migapi.Warn,
								Message:  "warning",
							},
						},
					},
				},
			},
			initialConditions: []migapi.Condition{
				{
					Type: DirectVolumeMigrationBlocked,
				},
			},
			expectedConditions: []migapi.Condition{
				{
					Type:     DirectVolumeMigrationBlocked,
					Status:   True,
					Reason:   migapi.NotReady,
					Category: migapi.Warn,
					Message:  "warning",
				},
			},
			wantErr: false,
		},
		{
			name: "dvm completed, no warnings",
			step: "test",
			dvm: &migapi.DirectVolumeMigration{
				Status: migapi.DirectVolumeMigrationStatus{
					Phase:     dvmc.Completed,
					Itinerary: dvmc.VolumeMigrationItinerary,
					Conditions: migapi.Conditions{
						List: []migapi.Condition{
							{
								Type:   dvmc.Succeeded,
								Status: True,
							},
						},
					},
				},
			},
			initialConditions:  []migapi.Condition{},
			expectedConditions: []migapi.Condition{},
			wantErr:            false,
		},
		{
			name: "dvm completed, invalid next step",
			step: "invalid",
			dvm: &migapi.DirectVolumeMigration{
				Status: migapi.DirectVolumeMigrationStatus{
					Phase:     dvmc.Completed,
					Itinerary: dvmc.VolumeMigrationItinerary,
					Conditions: migapi.Conditions{
						List: []migapi.Condition{
							{
								Type:   dvmc.Succeeded,
								Status: True,
							},
						},
					},
				},
			},
			initialConditions:  []migapi.Condition{},
			expectedConditions: []migapi.Condition{},
			wantErr:            true,
		},
		{
			name: "dvm completed, warnings",
			step: "test",
			dvm: &migapi.DirectVolumeMigration{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test",
					Namespace: "test",
				},
				Status: migapi.DirectVolumeMigrationStatus{
					Phase:     dvmc.Completed,
					Itinerary: dvmc.VolumeMigrationItinerary,
					Conditions: migapi.Conditions{
						List: []migapi.Condition{
							{
								Type:   dvmc.Failed,
								Reason: "test failure",
								Status: True,
							},
						},
					},
				},
			},
			initialConditions: []migapi.Condition{},
			expectedConditions: []migapi.Condition{
				{
					Type:     DirectVolumeMigrationFailed,
					Status:   True,
					Category: migapi.Warn,
					Message:  "DirectVolumeMigration (dvm): test/test failed. See in dvm status.Errors",
					Durable:  true,
				},
			},
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			task := &Task{
				Step: tt.step,
				Owner: &migapi.MigMigration{
					Status: migapi.MigMigrationStatus{
						Pipeline: []*migapi.Step{
							{
								Name: "test",
							},
						},
						Conditions: migapi.Conditions{
							List: tt.initialConditions,
						},
					},
				},
			}
			err := task.waitForDVMToComplete(tt.dvm)
			if (err != nil) != tt.wantErr {
				t.Errorf("waitForDMVToComplete() error = %v, wantErr %v", err, tt.wantErr)
				t.FailNow()
			}
			if len(task.Owner.Status.Conditions.List) != len(tt.expectedConditions) {
				t.Errorf("waitForDMVToComplete() = %v, want %v", task.Owner.Status.Conditions.List, tt.expectedConditions)
				t.FailNow()
			}
			for i, c := range task.Owner.Status.Conditions.List {
				if c.Category != tt.expectedConditions[i].Category {
					t.Errorf("category = %s, want %s", c.Category, tt.expectedConditions[i].Category)
				}
				if c.Type != tt.expectedConditions[i].Type {
					t.Errorf("type = %s, want %s", c.Type, tt.expectedConditions[i].Type)
				}
				if c.Status != tt.expectedConditions[i].Status {
					t.Errorf("status = %s, want %s", c.Status, tt.expectedConditions[i].Status)
				}
				if c.Reason != tt.expectedConditions[i].Reason {
					t.Errorf("reason = %s, want %s", c.Reason, tt.expectedConditions[i].Reason)
				}
				if c.Message != tt.expectedConditions[i].Message {
					t.Errorf("message = %s, want %s", c.Message, tt.expectedConditions[i].Message)
				}
			}
		})
	}
}

package migmigration

import (
	"github.com/go-logr/logr"
	migapi "github.com/konveyor/mig-controller/pkg/apis/migration/v1alpha1"
	"reflect"
	"testing"
)

func TestTask_getStagePVs(t1 *testing.T) {
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
		t1.Run(tt.name, func(t1 *testing.T) {
			t := &Task{
				Log:           tt.fields.Log,
				PlanResources: tt.fields.PlanResources,
				Phase:         tt.fields.Phase,
				Step:          tt.fields.Step,
			}
			if got := t.getStagePVs(); !reflect.DeepEqual(got, tt.want) {
				t1.Errorf("getStagePVs() = %v, want %v", got, tt.want)
			}
		})
	}
}

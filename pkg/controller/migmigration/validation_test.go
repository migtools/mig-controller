package migmigration

import (
	"context"
	"strings"
	"testing"

	migapi "github.com/konveyor/mig-controller/pkg/apis/migration/v1alpha1"
	"github.com/konveyor/mig-controller/pkg/compat"
	fakecompat "github.com/konveyor/mig-controller/pkg/compat/fake"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	k8sclient "sigs.k8s.io/controller-runtime/pkg/client"
)

func TestReconcileMigMigration_validateMigrationType(t *testing.T) {
	getFakeClientWithObjs := func(obj ...k8sclient.Object) compat.Client {
		client, _ := fakecompat.NewFakeClient(obj...)
		return client
	}
	getTestMigCluster := func(name string, url string) *migapi.MigCluster {
		return &migapi.MigCluster{
			ObjectMeta: metav1.ObjectMeta{
				Name:      name,
				Namespace: migapi.OpenshiftMigrationNamespace,
			},
			Spec: migapi.MigClusterSpec{
				URL: url,
			},
		}
	}
	getTestMigPlan := func(srcCluster string, destCluster string, ns []string, pvMappings []string) *migapi.MigPlan {
		PVs := []migapi.PV{}
		for _, pvMapping := range pvMappings {
			mapping := strings.Split(pvMapping, "/")
			ns := mapping[0]
			name := mapping[1]
			PVs = append(PVs, migapi.PV{PVC: migapi.PVC{Name: name, Namespace: ns}})
		}
		return &migapi.MigPlan{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "migplan",
				Namespace: migapi.OpenshiftMigrationNamespace,
			},
			Spec: migapi.MigPlanSpec{
				SrcMigClusterRef:  &v1.ObjectReference{Name: srcCluster, Namespace: migapi.OpenshiftMigrationNamespace},
				DestMigClusterRef: &v1.ObjectReference{Name: destCluster, Namespace: migapi.OpenshiftMigrationNamespace},
				Namespaces:        ns,
				PersistentVolumes: migapi.PersistentVolumes{List: PVs},
			},
		}
	}
	type args struct {
		ctx       context.Context
		plan      *migapi.MigPlan
		migration *migapi.MigMigration
	}
	tests := []struct {
		name               string
		client             k8sclient.Client
		args               args
		wantErr            bool
		wantConditions     []migapi.Condition
		dontWantConditions []migapi.Condition
	}{
		{
			name: "given a MigMigration with only stage set to false, should not have a critical condition",
			client: getFakeClientWithObjs(
				getTestMigCluster("test-cluster", "http://test-api.com:6444"),
			),
			args: args{
				ctx:  context.TODO(),
				plan: getTestMigPlan("test-cluster", "test-cluster", []string{}, []string{}),
				migration: &migapi.MigMigration{
					Spec: migapi.MigMigrationSpec{
						Stage: false,
					},
				},
			},
			wantErr: false,
			dontWantConditions: []migapi.Condition{{
				Type: InvalidSpec,
			}},
		},
		{
			name: "given a MigMigration with only stage set to true, should not have a critical condition",
			client: getFakeClientWithObjs(
				getTestMigCluster("test-cluster", "http://test-api.com:6444"),
			),
			args: args{
				ctx:  context.TODO(),
				plan: getTestMigPlan("test-cluster", "test-cluster", []string{}, []string{}),
				migration: &migapi.MigMigration{
					Spec: migapi.MigMigrationSpec{
						Stage: true,
					},
				},
			},
			wantErr: false,
			dontWantConditions: []migapi.Condition{{
				Type: InvalidSpec,
			}},
		},
		{
			name: "given a MigMigration with stage set to false & rollback set to true, should not have a critical condition",
			client: getFakeClientWithObjs(
				getTestMigCluster("test-cluster", "http://test-api.com:6444"),
			),
			args: args{
				ctx:  context.TODO(),
				plan: getTestMigPlan("test-cluster", "test-cluster", []string{}, []string{}),
				migration: &migapi.MigMigration{
					Spec: migapi.MigMigrationSpec{
						Stage:    false,
						Rollback: true,
					},
				},
			},
			wantErr: false,
			dontWantConditions: []migapi.Condition{{
				Type: InvalidSpec,
			}},
		},
		{
			name: "given a MigMigration with stage set to false & migrateState set to true, should not have a critical condition",
			client: getFakeClientWithObjs(
				getTestMigCluster("test-cluster", "http://test-api.com:6444"),
			),
			args: args{
				ctx:  context.TODO(),
				plan: getTestMigPlan("test-cluster", "test-cluster", []string{}, []string{}),
				migration: &migapi.MigMigration{
					Spec: migapi.MigMigrationSpec{
						Stage:        false,
						MigrateState: true,
					},
				},
			},
			wantErr: false,
			dontWantConditions: []migapi.Condition{{
				Type: InvalidSpec,
			}},
		},
		{
			name: "given a MigMigration with stage & migrateState set to true, should have a critical condition",
			client: getFakeClientWithObjs(
				getTestMigCluster("test-cluster", "http://test-api.com:6444"),
			),
			args: args{
				ctx:  context.TODO(),
				plan: getTestMigPlan("test-cluster", "test-cluster", []string{}, []string{}),
				migration: &migapi.MigMigration{
					Spec: migapi.MigMigrationSpec{
						MigrateState: true,
						Stage:        true,
					},
				},
			},
			wantErr: false,
			wantConditions: []migapi.Condition{{
				Type:   InvalidSpec,
				Reason: NotSupported,
			}},
		},
		{
			name: "given a MigMigration with stage & rollback set to true, should have a critical condition",
			client: getFakeClientWithObjs(
				getTestMigCluster("test-cluster", "http://test-api.com:6444"),
			),
			args: args{
				ctx:  context.TODO(),
				plan: getTestMigPlan("test-cluster", "test-cluster", []string{}, []string{}),
				migration: &migapi.MigMigration{
					Spec: migapi.MigMigrationSpec{
						Rollback: true,
						Stage:    true,
					},
				},
			},
			wantErr: false,
			wantConditions: []migapi.Condition{{
				Type:   InvalidSpec,
				Reason: NotSupported,
			}},
		},
		{
			name: "given a MigMigration with stage & rollback & migrateState set to true, should have a critical condition",
			client: getFakeClientWithObjs(
				getTestMigCluster("test-cluster", "http://test-api.com:6444"),
			),
			args: args{
				ctx:  context.TODO(),
				plan: getTestMigPlan("test-cluster", "test-cluster", []string{}, []string{}),
				migration: &migapi.MigMigration{
					Spec: migapi.MigMigrationSpec{
						Rollback:     true,
						Stage:        true,
						MigrateState: true,
					},
				},
			},
			wantErr: false,
			wantConditions: []migapi.Condition{{
				Type:   InvalidSpec,
				Reason: NotSupported,
			}},
		},
		{
			name: "given a intra-cluster MigMigration with no conflicting PVCs, should not have a critical condition",
			client: getFakeClientWithObjs(
				getTestMigCluster("test-cluster", "http://test-api.com:6444"),
			),
			args: args{
				ctx: context.TODO(),
				plan: getTestMigPlan("test-cluster", "test-cluster", []string{
					"ns-00:ns-00",
					"ns-01:ns-01",
				}, []string{
					"ns-00/pv-00:pv-01",
					"ns-00/pv-02:pv-03",
					"ns-01/pv-01:pv-02",
				}),
				migration: &migapi.MigMigration{
					Spec: migapi.MigMigrationSpec{
						MigrateState: true,
					},
				},
			},
			wantErr: false,
			dontWantConditions: []migapi.Condition{{
				Type:   migapi.Failed,
				Reason: PvNameConflict,
			}},
		},
		{
			name: "given a intra-cluster MigMigration with conflicting PVCs, should have a critical condition",
			client: getFakeClientWithObjs(
				getTestMigCluster("test-cluster", "http://test-api.com:6444"),
			),
			args: args{
				ctx: context.TODO(),
				plan: getTestMigPlan("test-cluster", "test-cluster", []string{
					"ns-00:ns-00",
				}, []string{
					"ns-00/pv-00:pv-02",
					"ns-00/pv-01:pv-02",
				}),
				migration: &migapi.MigMigration{
					Spec: migapi.MigMigrationSpec{
						MigrateState: true,
					},
				},
			},
			wantErr: false,
			wantConditions: []migapi.Condition{{
				Type:   ConflictingPVCMappings,
				Reason: PvNameConflict,
			}},
		},
		{
			name: "given a intra-cluster final MigMigration with conflicting namespaces, should have a critical condition",
			client: getFakeClientWithObjs(
				getTestMigCluster("test-cluster", "http://test-api.com:6444"),
			),
			args: args{
				ctx: context.TODO(),
				plan: getTestMigPlan("test-cluster", "test-cluster", []string{
					"ns-00:ns-00",
					"ns-01:ns-01",
				}, []string{}),
				migration: &migapi.MigMigration{
					Spec: migapi.MigMigrationSpec{
						MigrateState: false,
					},
				},
			},
			wantErr: false,
			wantConditions: []migapi.Condition{{
				Type:   migapi.Failed,
				Reason: NotSupported,
			}},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := ReconcileMigMigration{
				Client: tt.client,
			}
			if err := r.validateMigrationType(tt.args.ctx, tt.args.plan, tt.args.migration); (err != nil) != tt.wantErr {
				t.Errorf("ReconcileMigMigration.validateMigrationType() error = %v, wantErr %v", err, tt.wantErr)
			}
			for _, wantCond := range tt.wantConditions {
				foundCond := tt.args.migration.Status.FindCondition(wantCond.Type)
				if foundCond == nil {
					t.Errorf("ReconcileMigPlan.validatePossibleMigrationTypes() wantCondition = %s, found nil", wantCond.Type)
				}
				if foundCond != nil && foundCond.Reason != wantCond.Reason {
					t.Errorf("ReconcileMigPlan.validatePossibleMigrationTypes() want reason = %s, found %s", wantCond.Reason, foundCond.Reason)
				}
			}
			for _, dontWantCond := range tt.dontWantConditions {
				foundCond := tt.args.migration.Status.FindCondition(dontWantCond.Type)
				if foundCond != nil {
					t.Errorf("ReconcileMigPlan.validatePossibleMigrationTypes() dontWantCondition = %s, found = %s", dontWantCond.Type, foundCond.Type)
				}
			}
		})
	}
}

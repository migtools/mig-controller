package migplan

import (
	"context"
	"testing"
	"time"

	migapi "github.com/konveyor/mig-controller/pkg/apis/migration/v1alpha1"
	"github.com/konveyor/mig-controller/pkg/compat"
	fakecompat "github.com/konveyor/mig-controller/pkg/compat/fake"
	"github.com/opentracing/opentracing-go/mocktracer"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	k8sclient "sigs.k8s.io/controller-runtime/pkg/client"
)

func TestReconcileMigPlan_validatePossibleMigrationTypes(t *testing.T) {
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
	getTestMigPlan := func(srcCluster string, destCluster string, ns []string, conds []migapi.Condition) *migapi.MigPlan {
		return &migapi.MigPlan{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "migplan",
				Namespace: migapi.OpenshiftMigrationNamespace,
			},
			Spec: migapi.MigPlanSpec{
				SrcMigClusterRef:  &v1.ObjectReference{Name: srcCluster, Namespace: migapi.OpenshiftMigrationNamespace},
				DestMigClusterRef: &v1.ObjectReference{Name: destCluster, Namespace: migapi.OpenshiftMigrationNamespace},
				Namespaces:        ns,
			},
			Status: migapi.MigPlanStatus{
				Conditions: migapi.Conditions{
					List: conds,
				},
			},
		}
	}
	tests := []struct {
		name               string
		client             k8sclient.Client
		plan               *migapi.MigPlan
		wantErr            bool
		wantConditions     []migapi.Condition
		dontWantConditions []migapi.Condition
	}{
		{
			name: "given a plan with zero state migrations associated, should not have any identification condition",
			client: getFakeClientWithObjs(
				getTestMigCluster("test-cluster", "http://test-api.com:6444"),
			),
			plan:           getTestMigPlan("test-cluster", "test-cluster", []string{}, []migapi.Condition{}),
			wantErr:        false,
			wantConditions: []migapi.Condition{},
		},
		{
			name: "given an intra-cluster plan with one state migrations associated, should have the correct identification condition",
			client: getFakeClientWithObjs(
				getTestMigCluster("test-cluster", "http://test-api.com:6444"),
				&migapi.MigMigration{
					ObjectMeta: metav1.ObjectMeta{Name: "test", Namespace: migapi.OpenshiftMigrationNamespace},
					Spec:       migapi.MigMigrationSpec{MigrateState: true},
				},
			),
			plan:    getTestMigPlan("test-cluster", "test-cluster", []string{"ns-00:ns-01"}, []migapi.Condition{}),
			wantErr: false,
			wantConditions: []migapi.Condition{
				{
					Type:   migapi.MigrationTypeIdentified,
					Reason: string(migapi.StateMigrationPlan),
				},
			},
		},
		{
			name: "given a plan referencing two different clusters with one state migrations associated, should have the correct identification condition",
			client: getFakeClientWithObjs(
				getTestMigCluster("test-cluster", "http://test-api.com:6444"),
				getTestMigCluster("test-cluster-2", "http://test-api-2.com:6444"),
				&migapi.MigMigration{
					ObjectMeta: metav1.ObjectMeta{
						Name: "test", Namespace: migapi.OpenshiftMigrationNamespace,
					},
					Spec: migapi.MigMigrationSpec{
						MigrateState: true,
						MigPlanRef: &v1.ObjectReference{
							Name:      "migplan",
							Namespace: migapi.OpenshiftMigrationNamespace,
						},
					},
				},
			),
			plan:    getTestMigPlan("test-cluster", "test-cluster-2", []string{}, []migapi.Condition{}),
			wantErr: false,
			wantConditions: []migapi.Condition{
				{
					Type:   migapi.MigrationTypeIdentified,
					Reason: string(migapi.StateMigrationPlan),
				},
			},
		},
		{
			name: "given a plan referencing two different clusters with one state migration & a successful rollback associated with it, should not have the identification condition",
			client: getFakeClientWithObjs(
				getTestMigCluster("test-cluster", "http://test-api.com:6444"),
				getTestMigCluster("test-cluster-2", "http://test-api-2.com:6444"),
				&migapi.MigMigration{
					ObjectMeta: metav1.ObjectMeta{
						CreationTimestamp: metav1.Now(),
						Name:              "stage-00", Namespace: migapi.OpenshiftMigrationNamespace,
					},
					Spec: migapi.MigMigrationSpec{MigrateState: true},
				},
				&migapi.MigMigration{
					ObjectMeta: metav1.ObjectMeta{
						CreationTimestamp: metav1.Time{Time: time.Now().Add(time.Second * 3)},
						Name:              "rollback-01", Namespace: migapi.OpenshiftMigrationNamespace,
					},
					Spec: migapi.MigMigrationSpec{
						Rollback: true,
						MigPlanRef: &v1.ObjectReference{
							Name:      "migplan",
							Namespace: migapi.OpenshiftMigrationNamespace,
						},
					},
					Status: migapi.MigMigrationStatus{
						Conditions: migapi.Conditions{
							List: []migapi.Condition{
								{
									Type:   migapi.Succeeded,
									Status: True,
								},
							},
						},
					},
				},
			),
			plan: getTestMigPlan("test-cluster", "test-cluster-2", []string{}, []migapi.Condition{{
				Type:   migapi.MigrationTypeIdentified,
				Reason: string(migapi.StateMigrationPlan),
			}}),
			wantErr: false,
			dontWantConditions: []migapi.Condition{
				{
					Type:   migapi.MigrationTypeIdentified,
					Reason: string(migapi.StateMigrationPlan),
				},
			},
		},
		{
			name: "given a plan referencing two different clusters with one state migration & a unsuccessful rollback associated with it, should have the identification condition",
			client: getFakeClientWithObjs(
				getTestMigCluster("test-cluster", "http://test-api.com:6444"),
				getTestMigCluster("test-cluster-2", "http://test-api-2.com:6444"),
				&migapi.MigMigration{
					ObjectMeta: metav1.ObjectMeta{
						CreationTimestamp: metav1.Now(),
						Name:              "stage-00", Namespace: migapi.OpenshiftMigrationNamespace,
					},
					Spec: migapi.MigMigrationSpec{MigrateState: true},
				},
				&migapi.MigMigration{
					ObjectMeta: metav1.ObjectMeta{
						CreationTimestamp: metav1.Time{Time: time.Now().Add(time.Second * 3)},
						Name:              "rollback-01", Namespace: migapi.OpenshiftMigrationNamespace,
					},
					Spec: migapi.MigMigrationSpec{
						Rollback:     true,
						MigrateState: true,
						MigPlanRef: &v1.ObjectReference{
							Name:      "migplan",
							Namespace: migapi.OpenshiftMigrationNamespace,
						},
					},
				},
			),
			plan:    getTestMigPlan("test-cluster", "test-cluster-2", []string{}, []migapi.Condition{}),
			wantErr: false,
			wantConditions: []migapi.Condition{
				{
					Type:   migapi.MigrationTypeIdentified,
					Reason: string(migapi.StateMigrationPlan),
				},
			},
		},
		{
			name: "given an intra-cluster plan with some namespaces mapped & some unmapped, should have a critical condition",
			client: getFakeClientWithObjs(
				getTestMigCluster("test-cluster", "http://test-api.com:6444"),
			),
			plan: getTestMigPlan("test-cluster", "test-cluster", []string{
				"ns-00:ns-00",
				"ns-01:ns-02",
			}, []migapi.Condition{}),
			wantErr: false,
			wantConditions: []migapi.Condition{
				{
					Type:   IntraClusterMigration,
					Reason: ConflictingNamespaces,
				},
			},
		},
		{
			name: "given an intra-cluster plan with zero mapped namespace, should have a storage conversion identification condition",
			client: getFakeClientWithObjs(
				getTestMigCluster("test-cluster", "http://test-api.com:6444"),
			),
			plan: getTestMigPlan("test-cluster", "test-cluster", []string{
				"ns-00:ns-00",
				"ns-01:ns-01",
			}, []migapi.Condition{}),
			wantErr: false,
			wantConditions: []migapi.Condition{
				{
					Type:   migapi.MigrationTypeIdentified,
					Reason: string(migapi.StorageConversionPlan),
				},
			},
		},
		{
			name: "given an intra-cluster plan with all mapped namespace, should have a state migration condition",
			client: getFakeClientWithObjs(
				getTestMigCluster("test-cluster", "http://test-api.com:6444"),
			),
			plan: getTestMigPlan("test-cluster", "test-cluster", []string{
				"ns-00:ns-03",
				"ns-01:ns-02",
			}, []migapi.Condition{}),
			wantErr: false,
			wantConditions: []migapi.Condition{
				{
					Type:   migapi.MigrationTypeIdentified,
					Reason: string(migapi.StateMigrationPlan),
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := ReconcileMigPlan{
				Client: tt.client,
				tracer: mocktracer.New(),
			}
			if err := r.validatePossibleMigrationTypes(context.TODO(), tt.plan); (err != nil) != tt.wantErr {
				t.Errorf("ReconcileMigPlan.validatePossibleMigrationTypes() error = %v, wantErr %v", err, tt.wantErr)
			}
			for _, wantCond := range tt.wantConditions {
				foundCond := tt.plan.Status.FindCondition(wantCond.Type)
				if foundCond == nil {
					t.Errorf("ReconcileMigPlan.validatePossibleMigrationTypes() wantCondition = %s, found nil", wantCond.Type)
				}
				if foundCond != nil && foundCond.Reason != wantCond.Reason {
					t.Errorf("ReconcileMigPlan.validatePossibleMigrationTypes() want reason = %s, found %s", wantCond.Reason, foundCond.Reason)
				}
			}
			for _, dontWantCond := range tt.dontWantConditions {
				foundCond := tt.plan.Status.FindCondition(dontWantCond.Type)
				if foundCond != nil {
					t.Errorf("ReconcileMigPlan.validatePossibleMigrationTypes() dontWantCondition = %s, found = %s", dontWantCond.Type, foundCond.Type)
				}
			}
		})
	}
}

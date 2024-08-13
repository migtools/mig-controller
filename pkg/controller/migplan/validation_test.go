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
	"k8s.io/utils/ptr"
	virtv1 "kubevirt.io/api/core/v1"
	k8sclient "sigs.k8s.io/controller-runtime/pkg/client"
)

func getFakeClientWithObjs(obj ...k8sclient.Object) compat.Client {
	client, _ := fakecompat.NewFakeClient(obj...)
	return client
}

func getTestMigCluster(name string, url string) *migapi.MigCluster {
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

func getTestMigPlan(srcCluster string, destCluster string, ns []string, conds []migapi.Condition) *migapi.MigPlan {
	return &migapi.MigPlan{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "migplan",
			Namespace: migapi.OpenshiftMigrationNamespace,
		},
		Spec: migapi.MigPlanSpec{
			SrcMigClusterRef:  &v1.ObjectReference{Name: srcCluster, Namespace: migapi.OpenshiftMigrationNamespace},
			DestMigClusterRef: &v1.ObjectReference{Name: destCluster, Namespace: migapi.OpenshiftMigrationNamespace},
			Namespaces:        ns,
			LiveMigrate:       ptr.To[bool](true),
		},
		Status: migapi.MigPlanStatus{
			Conditions: migapi.Conditions{
				List: conds,
			},
		},
	}
}

func TestReconcileMigPlan_validatePossibleMigrationTypes(t *testing.T) {
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

func TestReconcileMigPlan_validateparseKubeVirtOperatorSemver(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		wantErr bool
		major   int
		minor   int
		bugfix  int
	}{
		{
			name:    "given a valid semver string, should not return an error",
			input:   "v0.0.0",
			wantErr: false,
			major:   0,
			minor:   0,
			bugfix:  0,
		},
		{
			name:    "given a valid semver string with extra info, should not return an error",
			input:   "v1.2.3-rc1",
			wantErr: false,
			major:   1,
			minor:   2,
			bugfix:  3,
		},
		{
			name:    "given a valid semver string with extra info, should not return an error",
			input:   "v1.2.3-rc1.debug.1",
			wantErr: false,
			major:   1,
			minor:   2,
			bugfix:  3,
		},
		{
			name:    "given a semver string with two dots, should return an error",
			input:   "v0.0",
			wantErr: true,
		},
		{
			name:    "given a semver string without a v should not return an error",
			input:   "1.1.1",
			wantErr: false,
			major:   1,
			minor:   1,
			bugfix:  1,
		},
		{
			name:    "given a semver with an invalid major version, should return an error",
			input:   "va.1.1",
			wantErr: true,
		},
		{
			name:    "given a semver with an invalid minor version, should return an error",
			input:   "v4.b.1",
			wantErr: true,
		},
		{
			name:    "given a semver with an invalid bugfix version, should return an error",
			input:   "v2.1.c",
			wantErr: true,
		},
		{
			name:    "given a semver with an invalid bugfix version with dash, should return an error",
			input:   "v2.1.-",
			wantErr: true,
		},
		{
			name:    "given a semver with a dot instead of a valid bugfix version, should return an error",
			input:   "v2.1.-",
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			major, minor, patch, err := parseKubeVirtOperatorSemver(tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("parseKubeVirtOperatorSemver() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if err == nil {
				if major != tt.major {
					t.Errorf("parseKubeVirtOperatorSemver() major = %v, want %v", major, tt.major)
				}
				if minor != tt.minor {
					t.Errorf("parseKubeVirtOperatorSemver() minor = %v, want %v", minor, tt.minor)
				}
				if patch != tt.bugfix {
					t.Errorf("parseKubeVirtOperatorSemver() patch = %v, want %v", patch, tt.bugfix)
				}
			}
		})
	}
}

func TestReconcileMigPlan_validateKubeVirtInstalled(t *testing.T) {
	plan := getTestMigPlan("test-cluster", "test-cluster", []string{
		"ns-00:ns-00",
		"ns-01:ns-02",
	}, []migapi.Condition{})
	noLiveMigratePlan := plan.DeepCopy()
	noLiveMigratePlan.Spec.LiveMigrate = ptr.To[bool](false)
	tests := []struct {
		name               string
		client             compat.Client
		plan               *migapi.MigPlan
		wantErr            bool
		wantConditions     []migapi.Condition
		dontWantConditions []migapi.Condition
	}{
		{
			name:    "given a cluster without kubevirt installed, should return a warning condition",
			client:  getFakeClientWithObjs(),
			plan:    plan.DeepCopy(),
			wantErr: false,
			wantConditions: []migapi.Condition{
				{
					Type:     KubeVirtNotInstalledSourceCluster,
					Status:   True,
					Reason:   NotFound,
					Category: Advisory,
					Message:  KubeVirtNotInstalledSourceClusterMessage,
				},
			},
		},
		{
			name: "given a cluster with multiple kubevirt CRs, should return a warning condition",
			client: getFakeClientWithObjs(
				&virtv1.KubeVirt{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test-kubevirt",
						Namespace: "kubevirt",
					},
				},
				&virtv1.KubeVirt{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test-kubevirt-two",
						Namespace: "openshift-cnv",
					},
				},
			),
			plan:    plan.DeepCopy(),
			wantErr: false,
			wantConditions: []migapi.Condition{
				{
					Type:     KubeVirtNotInstalledSourceCluster,
					Status:   True,
					Reason:   NotFound,
					Category: Advisory,
					Message:  KubeVirtNotInstalledSourceClusterMessage,
				},
			},
		},
		{
			name: "given a cluster with kubevirt installed, but invalid version, should return a warning condition",
			client: getFakeClientWithObjs(
				&virtv1.KubeVirt{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test-kubevirt",
						Namespace: "kubevirt",
					},
					Status: virtv1.KubeVirtStatus{
						OperatorVersion: "a.b.c",
					},
				},
			),
			plan:    plan.DeepCopy(),
			wantErr: false,
			wantConditions: []migapi.Condition{
				{
					Type:     KubeVirtVersionNotSupported,
					Status:   True,
					Reason:   NotSupported,
					Category: Warn,
					Message:  KubeVirtVersionNotSupportedMessage,
				},
			},
		},
		{
			name: "given a cluster with kubevirt installed, but older version, should return a warning condition",
			client: getFakeClientWithObjs(
				&virtv1.KubeVirt{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test-kubevirt",
						Namespace: "kubevirt",
					},
					Status: virtv1.KubeVirtStatus{
						OperatorVersion: "0.4.3",
					},
				},
			),
			plan:    plan.DeepCopy(),
			wantErr: false,
			wantConditions: []migapi.Condition{
				{
					Type:     KubeVirtVersionNotSupported,
					Status:   True,
					Reason:   NotSupported,
					Category: Warn,
					Message:  KubeVirtVersionNotSupportedMessage,
				},
			},
		},
		{
			name: "given a cluster with kubevirt installed, but plan has no live migration, should not return a warning condition",
			client: getFakeClientWithObjs(
				&virtv1.KubeVirt{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test-kubevirt",
						Namespace: "kubevirt",
					},
					Status: virtv1.KubeVirtStatus{
						OperatorVersion: "1.4.3",
					},
				},
			),
			plan:    noLiveMigratePlan,
			wantErr: false,
			dontWantConditions: []migapi.Condition{
				{
					Type:     KubeVirtStorageLiveMigrationNotEnabled,
					Status:   True,
					Reason:   NotSupported,
					Category: Warn,
					Message:  KubeVirtStorageLiveMigrationNotEnabledMessage,
				},
				{
					Type:     KubeVirtVersionNotSupported,
					Status:   True,
					Reason:   NotSupported,
					Category: Warn,
					Message:  KubeVirtVersionNotSupportedMessage,
				},
				{
					Type:     KubeVirtNotInstalledSourceCluster,
					Status:   True,
					Reason:   NotFound,
					Category: Advisory,
					Message:  KubeVirtNotInstalledSourceClusterMessage,
				},
			},
		},
		{
			name: "given a cluster with new enough kubevirt installed, should not return a warning condition",
			client: getFakeClientWithObjs(
				&virtv1.KubeVirt{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test-kubevirt",
						Namespace: "kubevirt",
					},
					Spec: virtv1.KubeVirtSpec{
						Configuration: virtv1.KubeVirtConfiguration{
							VMRolloutStrategy: ptr.To[virtv1.VMRolloutStrategy](virtv1.VMRolloutStrategyLiveUpdate),
							DeveloperConfiguration: &virtv1.DeveloperConfiguration{
								FeatureGates: []string{
									VMLiveUpdateFeatures,
									VolumeMigrationConfig,
									VolumesUpdateStrategy,
								},
							},
						},
					},
					Status: virtv1.KubeVirtStatus{
						OperatorVersion: "1.4.3",
					},
				},
			),
			plan:    plan.DeepCopy(),
			wantErr: false,
			dontWantConditions: []migapi.Condition{
				{
					Type:     KubeVirtStorageLiveMigrationNotEnabled,
					Status:   True,
					Reason:   NotSupported,
					Category: Warn,
					Message:  KubeVirtStorageLiveMigrationNotEnabledMessage,
				},
			},
		},
		{
			name: "given a cluster with new enough kubevirt installed, but not all featuregates should a warning condition",
			client: getFakeClientWithObjs(
				&virtv1.KubeVirt{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test-kubevirt",
						Namespace: "kubevirt",
					},
					Spec: virtv1.KubeVirtSpec{
						Configuration: virtv1.KubeVirtConfiguration{
							VMRolloutStrategy: ptr.To[virtv1.VMRolloutStrategy](virtv1.VMRolloutStrategyLiveUpdate),
							DeveloperConfiguration: &virtv1.DeveloperConfiguration{
								FeatureGates: []string{
									VolumeMigrationConfig,
									VolumesUpdateStrategy,
								},
							},
						},
					},
					Status: virtv1.KubeVirtStatus{
						OperatorVersion: "1.4.3",
					},
				},
			),
			plan:    plan.DeepCopy(),
			wantErr: false,
			wantConditions: []migapi.Condition{
				{
					Type:     KubeVirtStorageLiveMigrationNotEnabled,
					Status:   True,
					Reason:   NotSupported,
					Category: Warn,
					Message:  KubeVirtStorageLiveMigrationNotEnabledMessage,
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
			if err := r.validateKubeVirtInstalled(context.TODO(), tt.client, tt.plan); (err != nil) != tt.wantErr {
				t.Errorf("ReconcileMigPlan.validateKubeVirtInstalled() error = %v, wantErr %v", err, tt.wantErr)
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

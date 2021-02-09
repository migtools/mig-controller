/*
Copyright 2019 Red Hat Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package miganalytic

import (
	"context"
	"reflect"
	"strconv"
	"testing"
	"time"

	migapi "github.com/konveyor/mig-controller/pkg/apis/migration/v1alpha1"
	"github.com/konveyor/mig-controller/pkg/compat"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/record"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

var c client.Client

var expectedRequest = reconcile.Request{NamespacedName: types.NamespacedName{Name: "foo", Namespace: "default"}}
var depKey = types.NamespacedName{Name: "foo-deployment", Namespace: "default"}

const timeout = time.Second * 5

func TestReconcile(t *testing.T) {
	// g := gomega.NewGomegaWithT(t)
	// instance := &migrationv1alpha1.MigAnalytic{ObjectMeta: metav1.ObjectMeta{Name: "foo", Namespace: "default"}}

	// // Setup the Manager and Controller.  Wrap the Controller Reconcile function so it writes each request to a
	// // channel when it is finished.
	// mgr, err := manager.New(cfg, manager.Options{})
	// g.Expect(err).NotTo(gomega.HaveOccurred())
	// c = mgr.GetClient()

	// recFn, requests := SetupTestReconcile(newReconciler(mgr))
	// g.Expect(add(mgr, recFn)).NotTo(gomega.HaveOccurred())

	// stopMgr, mgrStopped := StartTestManager(mgr, g)

	// defer func() {
	// 	close(stopMgr)
	// 	mgrStopped.Wait()
	// }()

	// // Create the MigAnalytic object and expect the Reconcile and Deployment to be created
	// err = c.Create(context.TODO(), instance)
	// // The instance object may not be a valid object because it might be missing some required fields.
	// // Please modify the instance object by adding required fields and then remove the following if statement.
	// if apierrors.IsInvalid(err) {
	// 	t.Logf("failed to create object, got an invalid object error: %v", err)
	// 	return
	// }
	// g.Expect(err).NotTo(gomega.HaveOccurred())
	// defer c.Delete(context.TODO(), instance)
	// g.Eventually(requests, timeout).Should(gomega.Receive(gomega.Equal(expectedRequest)))

	// deploy := &appsv1.Deployment{}
	// g.Eventually(func() error { return c.Get(context.TODO(), depKey, deploy) }, timeout).
	// 	Should(gomega.Succeed())

	// // Delete the Deployment and expect Reconcile to be called for Deployment deletion
	// g.Expect(c.Delete(context.TODO(), deploy)).NotTo(gomega.HaveOccurred())
	// g.Eventually(requests, timeout).Should(gomega.Receive(gomega.Equal(expectedRequest)))
	// g.Eventually(func() error { return c.Get(context.TODO(), depKey, deploy) }, timeout).
	// 	Should(gomega.Succeed())

	// // Manually delete Deployment since GC isn't enabled in the test control plane
	// g.Expect(c.Delete(context.TODO(), deploy)).To(gomega.Succeed())

}

func TestReconcileMigAnalytic_analyzeExtendedPVCapacity(t *testing.T) {
	type fields struct {
		Client client.Client
	}
	type args struct {
		client compat.Client
		ns     *migapi.MigAnalyticNamespace
	}
	tests := []struct {
		name    string
		fields  fields
		args    args
		wantErr bool
	}{}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := &ReconcileMigAnalytic{
				Client: tt.fields.Client,
			}
			if err := r.analyzeExtendedPVCapacity(tt.args.client, tt.args.ns); (err != nil) != tt.wantErr {
				t.Errorf("ReconcileMigAnalytic.analyzeExtendedPVCapacity() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

var testSourceNamespace = corev1.Namespace{
	ObjectMeta: metav1.ObjectMeta{
		Name: "test-ns",
	},
}

var testTargetMigCluster = migapi.MigCluster{
	ObjectMeta: metav1.ObjectMeta{
		Name:      "migcluster-host",
		Namespace: "openshift-migration",
	},
	Spec: migapi.MigClusterSpec{
		IsHostCluster: true,
		Insecure:      true,
	},
	Status: migapi.MigClusterStatus{
		Conditions: migapi.Conditions{
			List: []migapi.Condition{
				{
					Type:     "Ready",
					Status:   "True",
					Category: "Required",
				},
			},
		},
	},
}

var testSourceMigCluster = migapi.MigCluster{
	ObjectMeta: metav1.ObjectMeta{
		Name:      "migcluster-source",
		Namespace: "openshift-migration",
	},
	Spec: migapi.MigClusterSpec{
		IsHostCluster: false,
		Insecure:      true,
	},
	Status: migapi.MigClusterStatus{
		Conditions: migapi.Conditions{
			List: []migapi.Condition{
				{
					Type:     "Ready",
					Status:   "True",
					Category: "Required",
				},
			},
		},
	},
}

var testMigStorage = migapi.MigStorage{
	ObjectMeta: metav1.ObjectMeta{
		Name:      "migstorage",
		Namespace: "openshift-migration",
	},
	Spec: migapi.MigStorageSpec{
		BackupStorageProvider: "aws",
	},
	Status: migapi.MigStorageStatus{
		Conditions: migapi.Conditions{
			List: []migapi.Condition{
				{
					Type:     "Ready",
					Status:   "True",
					Category: "Required",
				},
			},
		},
	},
}

var testMigPlan = migapi.MigPlan{
	ObjectMeta: metav1.ObjectMeta{
		Name:      "migplan-00",
		Namespace: "openshift-migration",
	},
	Spec: migapi.MigPlanSpec{
		DestMigClusterRef: &corev1.ObjectReference{
			Name:      "migcluster-host",
			Namespace: "openshift-migration",
		},
		SrcMigClusterRef: &corev1.ObjectReference{
			Name:      "migcluster-source",
			Namespace: "openshift-migration",
		},
		Namespaces: []string{"test-ns"},
		MigStorageRef: &corev1.ObjectReference{
			Name:      "migstorage",
			Namespace: "openshift-migration",
		},
	},
	Status: migapi.MigPlanStatus{
		Conditions: migapi.Conditions{
			List: []migapi.Condition{
				{
					Category: "Required",
					Status:   "True",
					Type:     "Ready",
				},
			},
		},
	},
}

func TestReconcileMigAnalytic_Reconcile(t *testing.T) {
	type fields struct {
		Client        client.Client
		EventRecorder record.EventRecorder
		scheme        *runtime.Scheme
	}
	type args struct {
		request reconcile.Request
	}
	tests := []struct {
		name        string
		fields      fields
		args        args
		want        reconcile.Result
		wantErr     bool
		wantReady   bool
		migAnalytic migapi.MigAnalytic
	}{
		{
			name: "do nothing when Refresh is not set and analytic is Ready",
			fields: fields{
				Client: fake.NewFakeClient(&migapi.MigAnalytic{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "miganalytic-00",
						Namespace: "openshift-migration",
					},
					Spec: migapi.MigAnalyticSpec{
						MigPlanRef: &corev1.ObjectReference{
							Namespace:  "openshift-migration",
							Name:       "migplan-00",
							Kind:       "MigPlan",
							APIVersion: "migration.openshift.io/v1alpha1",
						},
						Refresh: false,
					},
					Status: migapi.MigAnalyticStatus{
						Conditions: migapi.Conditions{
							List: []migapi.Condition{
								{
									Type:     "Ready",
									Status:   "True",
									Category: "Required",
									Message:  "The MigAnalytic is Ready",
								},
							},
						},
					},
				}),
			},
			want:      reconcile.Result{},
			wantErr:   false,
			wantReady: true,
		},
		{
			name: "trigger reconcile when Refresh is set",
			fields: fields{
				EventRecorder: record.NewFakeRecorder(20000),
				Client: fake.NewFakeClient(&migapi.MigAnalytic{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "miganalytic-00",
						Namespace: "openshift-migration",
					},
					Spec: migapi.MigAnalyticSpec{
						MigPlanRef: &corev1.ObjectReference{
							Namespace:  "openshift-migration",
							Name:       "migplan-00",
							Kind:       "MigPlan",
							APIVersion: "migration.openshift.io/v1alpha1",
						},
						Refresh: true,
					},
					Status: migapi.MigAnalyticStatus{
						Conditions: migapi.Conditions{
							List: []migapi.Condition{
								{
									Type:     "Ready",
									Status:   "False",
									Category: "Required",
									Message:  "The MigAnalytic is Ready",
								},
							},
						},
					},
				}, &testMigPlan, &testMigStorage, &testSourceMigCluster, &testTargetMigCluster, &testSourceNamespace),
			},
			want:      reconcile.Result{},
			wantErr:   false,
			wantReady: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := &ReconcileMigAnalytic{
				Client:        tt.fields.Client,
				EventRecorder: tt.fields.EventRecorder,
			}
			got, err := r.Reconcile(tt.args.request)
			if (err != nil) != tt.wantErr {
				t.Errorf("ReconcileMigAnalytic.Reconcile() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("ReconcileMigAnalytic.Reconcile() = %v, want %v", got, tt.want)
			}
			if err := r.Client.Get(context.TODO(), types.NamespacedName{
				Namespace: "openshift-migration",
				Name:      "miganalytic-00",
			}, &tt.migAnalytic); err == nil {
				for _, cond := range tt.migAnalytic.Status.Conditions.List {
					if cond.Type == "Ready" {
						isReady, _ := strconv.ParseBool(cond.Status)
						if tt.wantReady != isReady {
							t.Errorf("ReconcileMigAnalytic.Reconcile() = %v, want %v", isReady, tt.wantReady)
						}
					}
				}
			}
		})
	}
}

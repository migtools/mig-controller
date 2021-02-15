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

package migplan

import (
	"context"
	migapi "github.com/konveyor/mig-controller/pkg/apis/migration/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/tools/record"
	"reflect"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"testing"
	"time"

	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

var c client.Client

var expectedRequest = reconcile.Request{NamespacedName: types.NamespacedName{Name: "foo", Namespace: "default"}}
var depKey = types.NamespacedName{Name: "foo-deployment", Namespace: "default"}

const timeout = time.Second * 5

func TestReconcile(t *testing.T) {
	// g := gomega.NewGomegaWithT(t)
	// instance := &migrationv1alpha1.MigPlan{ObjectMeta: metav1.ObjectMeta{Name: "foo", Namespace: "default"}}

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

	// // Create the MigPlan object and expect the Reconcile and Deployment to be created
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

var migPlan = &migapi.MigPlan{
	ObjectMeta: metav1.ObjectMeta{
		Name:      "test-plan",
		Namespace: "test-ns",
	},
}
var migAnalytic = &migapi.MigAnalytic{
	ObjectMeta: metav1.ObjectMeta{
		Name:      "test-analtics",
		Namespace: "test-ns",
		Labels:    map[string]string{MigPlan: "test-plan"},
	},
	Spec: migapi.MigAnalyticSpec{
		MigPlanRef: &corev1.ObjectReference{
			Namespace: "test-plan",
			Name:      "test-ns",
		},
	},
}
var migPlan3 = &migapi.MigPlan{
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

var migAnalytic3 = &migapi.MigAnalytic{
	ObjectMeta: metav1.ObjectMeta{
		Name:      "miganalytic-00",
		Namespace: "openshift-migration",
		Labels:    map[string]string{MigPlan: "migplan-00"},
	},
	Spec: migapi.MigAnalyticSpec{
		MigPlanRef: &corev1.ObjectReference{
			Namespace:  "openshift-migration",
			Name:       "migplan-00",
			Kind:       "MigPlan",
			APIVersion: "migration.openshift.io/v1alpha1",
		},
		AnalyzeExntendedPVCapacity: true,
		Refresh:                    false,
	},
	Status: migapi.MigAnalyticStatus{
		Analytics: migapi.MigAnalyticPlan{
			PVCapacity:     resource.MustParse("1Gi"),
			ImageSizeTotal: resource.MustParse("1Gi"),
		},
	},
}
var migPlan4 = &migapi.MigPlan{
	ObjectMeta: metav1.ObjectMeta{
		Name:      "migplan-01",
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
		Refresh: true,
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

var migAnalytic4 = &migapi.MigAnalytic{
	ObjectMeta: metav1.ObjectMeta{
		Name:      "miganalytic-01",
		Namespace: "openshift-migration",
		Labels:    map[string]string{MigPlan: "migplan-01"},
	},
	Spec: migapi.MigAnalyticSpec{
		MigPlanRef: &corev1.ObjectReference{
			Namespace:  "openshift-migration",
			Name:       "migplan-01",
			Kind:       "MigPlan",
			APIVersion: "migration.openshift.io/v1alpha1",
		},
		AnalyzeExntendedPVCapacity: true,
		Refresh:                    false,
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
		Analytics: migapi.MigAnalyticPlan{
			PVCapacity:     resource.MustParse("1Gi"),
			ImageSizeTotal: resource.MustParse("1Gi"),
		},
	},
}

var expected1 = migapi.MigAnalytic{
	ObjectMeta: metav1.ObjectMeta{
		GenerateName: "test-plan-",
		Namespace:    "test-ns",
		Labels:       map[string]string{MigPlan: "test-plan", CreatedBy: "test-plan"},
		Annotations:  map[string]string{MigPlan: "test-plan", CreatedBy: "test-plan"},
		OwnerReferences: []metav1.OwnerReference{
			{
				APIVersion: "",
				Kind:       "",
				Name:       "test-plan",
				UID:        "",
			},
		},
	},
	Spec: migapi.MigAnalyticSpec{
		MigPlanRef: &corev1.ObjectReference{
			Namespace: "test-ns",
			Name:      "test-plan",
		},
		AnalyzeExntendedPVCapacity: true,
	},
	Status: migapi.MigAnalyticStatus{
		Analytics: migapi.MigAnalyticPlan{
			PVCapacity:     resource.MustParse("0"),
			ImageSizeTotal: resource.MustParse("0"),
		},
	},
}

var expected2 = migapi.MigAnalytic{
	ObjectMeta: metav1.ObjectMeta{
		Name:      "miganalytic-01",
		Namespace: "openshift-migration",
		Labels:    map[string]string{MigPlan: "migplan-01"},
	},
	Spec: migapi.MigAnalyticSpec{
		MigPlanRef: &corev1.ObjectReference{
			Namespace:  "openshift-migration",
			Name:       "migplan-01",
			Kind:       "MigPlan",
			APIVersion: "migration.openshift.io/v1alpha1",
		},
		AnalyzeExntendedPVCapacity: true,
		Refresh:                    true,
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
		Analytics: migapi.MigAnalyticPlan{
			PVCapacity:     resource.MustParse("1Gi"),
			ImageSizeTotal: resource.MustParse("1Gi"),
		},
	},
}

func TestReconcileMigPlan_ensureMigAnalytics(t *testing.T) {
	type fields struct {
		Client        client.Client
		EventRecorder record.EventRecorder
		scheme        *runtime.Scheme
	}
	type args struct {
		plan *migapi.MigPlan
	}
	tests := []struct {
		name    string
		fields  fields
		args    args
		wantErr bool
		want    migapi.MigAnalytic
	}{
		// TODO: Add test cases.
		{
			name: "If not migAnalytics exists, make sure the controller creates one",
			fields: fields{
				Client: fake.NewFakeClient(migPlan),
			},
			args: args{
				plan: migPlan,
			},
			wantErr: false,
			want:    expected1,
		},
		{
			name: "If migAnalytics exists without AnalyzeExntendedPVCapacity field, create a new migAnalytics",
			fields: fields{
				Client: fake.NewFakeClient(migPlan, migAnalytic),
			},
			args: args{
				plan: migPlan,
			},
			wantErr: false,
			want:    expected1,
		},
		{
			name: "If migAnalytics exists with AnalyzeExntendedPVCapacity field, and migplan.refresh is not set or set to false, do nothing",
			fields: fields{
				Client: fake.NewFakeClient(migPlan3, migAnalytic3),
			},
			args: args{
				plan: migPlan3,
			},
			wantErr: false,
			want:    *migAnalytic3,
		},
		{
			name: "If migAnalytics exists with AnalyzeExntendedPVCapacity field, and migplan.refresh is set to true, refresh migAnalytics",
			fields: fields{
				Client: fake.NewFakeClient(migPlan4, migAnalytic4),
			},
			args: args{
				plan: migPlan4,
			},
			wantErr: false,
			want:    expected2,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := ReconcileMigPlan{
				Client:        tt.fields.Client,
				EventRecorder: tt.fields.EventRecorder,
				scheme:        tt.fields.scheme,
			}
			if err := r.ensureMigAnalytics(tt.args.plan); (err != nil) != tt.wantErr {
				t.Errorf("ensureMigAnalytics() error = %v, wantErr %v", err, tt.wantErr)
			}
			gotList := &migapi.MigAnalyticList{}
			err := r.List(context.TODO(), &client.ListOptions{}, gotList)
			if err != nil {
				t.Errorf("ensureMigAnalytics() error = %v, wantErr %v", err, tt.wantErr)
			}
			for _, got := range gotList.Items {
				if got.Name == "miganalytic-00" || got.Name == "miganalytic-01" || got.GenerateName == "test-plan-" {
					if !reflect.DeepEqual(got, tt.want) {
						t.Errorf("waitForMigAnalyticsReady() got = %v, want %v", got, tt.want)
					}
				}
			}
		})
	}
}

func TestReconcileMigPlan_waitForMigAnalyticsReady(t *testing.T) {
	type fields struct {
		Client        client.Client
		EventRecorder record.EventRecorder
		scheme        *runtime.Scheme
	}
	type args struct {
		plan *migapi.MigPlan
	}
	tests := []struct {
		name    string
		fields  fields
		args    args
		want    *migapi.MigAnalytic
		wantErr bool
	}{
		// TODO: Add test cases.
		{
			name: "No migAnalytics with AnalyzeExntendedPVCapacity field true exists for a migPlan",
			fields: fields{
				Client: fake.NewFakeClient(migPlan, migAnalytic),
			},
			args: args{
				plan: migPlan,
			},
			wantErr: true,
			want:    nil,
		},
		{
			name: "migAnalytics with AnalyzeExntendedPVCapacity field true exists, but is not in ready state",
			fields: fields{
				Client: fake.NewFakeClient(migPlan3, migAnalytic3),
			},
			args: args{
				plan: migPlan3,
			},
			wantErr: true,
			want:    nil,
		},
		{
			name: "migAnalytics with AnalyzeExntendedPVCapacity field true exists and is in ready state",
			fields: fields{
				Client: fake.NewFakeClient(migPlan4, migAnalytic4),
			},
			args: args{
				plan: migPlan4,
			},
			wantErr: false,
			want:    migAnalytic4,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := ReconcileMigPlan{
				Client:        tt.fields.Client,
				EventRecorder: tt.fields.EventRecorder,
				scheme:        tt.fields.scheme,
			}
			got, err := r.waitForMigAnalyticsReady(tt.args.plan)
			if (err != nil) != tt.wantErr {
				t.Errorf("waitForMigAnalyticsReady() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("waitForMigAnalyticsReady() got = %v, want %v", got, tt.want)
			}
		})
	}
}

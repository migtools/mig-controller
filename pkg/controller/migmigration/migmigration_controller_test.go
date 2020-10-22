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

package migmigration

import (
	"reflect"
	"testing"
	"time"

	"github.com/onsi/gomega"
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
	// instance := &migrationv1alpha1.MigMigration{ObjectMeta: metav1.ObjectMeta{Name: "foo", Namespace: "default"}}

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

	// // Create the MigMigration object and expect the Reconcile and Deployment to be created
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

// Ensure that the stage itinerary is contained within the
// final itinerary.  At some point the itineraries may diverge
// but for not they are expected to be the identical.
func Test_Itineraries(t *testing.T) {
	g := gomega.NewGomegaWithT(t)
	common := []Phase{}

	stagePhases := []Phase{}
	finalPhases := []Phase{}
	for _, step := range StageItinerary.Steps {
		for _, phase := range step.Phases {
			stagePhases = append(stagePhases, phase)
		}
	}
	for _, step := range FinalItinerary.Steps {
		for _, phase := range step.Phases {
			finalPhases = append(finalPhases, phase)
		}
	}
	for i, stagePhase := range stagePhases {
		found := false
		for _, finalPhase := range finalPhases[i:] {
			if stagePhase.Name == finalPhase.Name {
				common = append(common, finalPhase)
				found = true
				break
			}
		}
		if !found {
			t.Errorf("%s is not contained in Final itinerary", stagePhase.Name)
		}
	}
	g.Expect(reflect.DeepEqual(stagePhases, common)).To(gomega.BeTrue())
}

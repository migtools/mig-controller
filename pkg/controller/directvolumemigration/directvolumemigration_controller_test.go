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

package directvolumemigration

import (
	"testing"
	"time"

	migapi "github.com/konveyor/mig-controller/pkg/apis/migration/v1alpha1"
	migrationv1alpha1 "github.com/konveyor/mig-controller/pkg/apis/migration/v1alpha1"
	"github.com/onsi/gomega"
	"golang.org/x/net/context"
	kapi "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/utils/ptr"
	virtv1 "kubevirt.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/metrics/server"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

var c client.Client

var expectedRequest = reconcile.Request{NamespacedName: types.NamespacedName{Name: "foo", Namespace: migrationv1alpha1.OpenshiftMigrationNamespace}}
var depKey = types.NamespacedName{Name: "foo-deployment", Namespace: "default"}

const timeout = time.Second * 5

func TestReconcile(t *testing.T) {
	g := gomega.NewGomegaWithT(t)
	instance := &migrationv1alpha1.DirectVolumeMigration{ObjectMeta: metav1.ObjectMeta{Name: "foo", Namespace: migrationv1alpha1.OpenshiftMigrationNamespace}}

	// Setup the Manager and Controller.  Wrap the Controller Reconcile function so it writes each request to a
	// channel when it is finished.
	mgr, err := manager.New(cfg, manager.Options{
		Metrics: server.Options{
			BindAddress: "0",
		},
	})
	g.Expect(err).NotTo(gomega.HaveOccurred())
	c = mgr.GetClient()

	recFn, requests := SetupTestReconcile(newReconciler(mgr))
	g.Expect(add(mgr, recFn)).NotTo(gomega.HaveOccurred())

	stopFunc := StartTestManager(mgr, g)

	defer stopFunc()

	err = c.Create(context.TODO(), &kapi.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: migrationv1alpha1.OpenshiftMigrationNamespace,
		},
	})
	if apierrors.IsInvalid(err) {
		t.Logf("failed to create object, got an invalid object error: %v", err)
		return
	}
	// Create the directvolumemigration object and expect the Reconcile and Deployment to be created
	err = c.Create(context.TODO(), instance)
	// The instance object may not be a valid object because it might be missing some required fields.
	// Please modify the instance object by adding required fields and then remove the following if statement.
	if apierrors.IsInvalid(err) {
		t.Logf("failed to create object, got an invalid object error: %v", err)
		return
	}
	g.Expect(err).NotTo(gomega.HaveOccurred())
	defer c.Delete(context.TODO(), instance)
	g.Eventually(requests, timeout).Should(gomega.Receive(gomega.Equal(expectedRequest)))
}

func TestCleanupTargetResourcesInNamespaces(t *testing.T) {
	g := gomega.NewGomegaWithT(t)
	reconciler := &ReconcileDirectVolumeMigration{}
	instance := &migrationv1alpha1.DirectVolumeMigration{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "foo",
			Namespace: migrationv1alpha1.OpenshiftMigrationNamespace,
			UID:       "1234",
		},
	}
	client := getFakeClientWithObjs(
		createDVMPod("namespace1", string(instance.GetUID())),
		createDVMSecret("namespace1", string(instance.GetUID())),
		createDVMSecret("namespace2", string(instance.GetUID())),
	)

	_, err := reconciler.cleanupResourcesInNamespaces(client, instance.GetUID(), []string{"namespace1", "namespace2"})
	g.Expect(err).NotTo(gomega.HaveOccurred())

	// ensure the pod is gone
	pod := &kapi.Pod{}
	err = client.Get(context.TODO(), types.NamespacedName{Name: "dvm-pod", Namespace: "namespace1"}, pod)
	g.Expect(apierrors.IsNotFound(err)).To(gomega.BeTrue())

	// ensure the secrets are gone
	secret := &kapi.Secret{}
	err = client.Get(context.TODO(), types.NamespacedName{Name: "dvm-secret", Namespace: "namespace1"}, secret)
	g.Expect(apierrors.IsNotFound(err)).To(gomega.BeTrue())
	secret = &kapi.Secret{}
	err = client.Get(context.TODO(), types.NamespacedName{Name: "dvm-secret", Namespace: "namespace2"}, secret)
	g.Expect(apierrors.IsNotFound(err)).To(gomega.BeTrue())
}

func TestCancelLiveMigrationsStage(t *testing.T) {
	g := gomega.NewGomegaWithT(t)
	reconciler := &ReconcileDirectVolumeMigration{}
	instance := &migrationv1alpha1.DirectVolumeMigration{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "foo",
			Namespace: migrationv1alpha1.OpenshiftMigrationNamespace,
			UID:       "1234",
		},
		Spec: migrationv1alpha1.DirectVolumeMigrationSpec{
			MigrationType: ptr.To[migapi.DirectVolumeMigrationType](migapi.MigrationTypeStage),
		},
	}
	client := getFakeClientWithObjs()
	allCompleted, err := reconciler.cancelLiveMigrations(client, instance)
	g.Expect(err).NotTo(gomega.HaveOccurred())
	g.Expect(allCompleted).To(gomega.BeTrue())
}

func TestCancelLiveMigrationsCutoverAllCompleted(t *testing.T) {
	g := gomega.NewGomegaWithT(t)
	reconciler := &ReconcileDirectVolumeMigration{}
	instance := &migrationv1alpha1.DirectVolumeMigration{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "foo",
			Namespace: migrationv1alpha1.OpenshiftMigrationNamespace,
			UID:       "1234",
		},
		Spec: migrationv1alpha1.DirectVolumeMigrationSpec{
			MigrationType: ptr.To[migapi.DirectVolumeMigrationType](migapi.MigrationTypeFinal),
			LiveMigrate:   ptr.To[bool](true),
			PersistentVolumeClaims: []migapi.PVCToMigrate{
				{
					ObjectReference: &kapi.ObjectReference{
						Namespace: testNamespace,
						Name:      "pvc1",
					},
					TargetNamespace: testNamespace,
					TargetName:      "target-pvc1",
				},
				{
					ObjectReference: &kapi.ObjectReference{
						Namespace: testNamespace,
						Name:      "pvc2",
					},
					TargetNamespace: testNamespace,
					TargetName:      "target-pvc2",
				},
				{
					ObjectReference: &kapi.ObjectReference{
						Namespace: testNamespace,
						Name:      "pvc3",
					},
					TargetNamespace: testNamespace,
					TargetName:      "target-pvc3",
				},
				{
					ObjectReference: &kapi.ObjectReference{
						Namespace: testNamespace,
						Name:      "pvc4",
					},
					TargetNamespace: testNamespace,
					TargetName:      "target-pvc4",
				},
			},
		},
	}
	client := getFakeClientWithObjs(
		createVirtualMachineWithVolumes("vm1", testNamespace, []virtv1.Volume{
			{
				Name: "pvc1",
				VolumeSource: virtv1.VolumeSource{
					DataVolume: &virtv1.DataVolumeSource{
						Name: "pvc1",
					},
				},
			},
		}),
		createVirtlauncherPod("vm1", testNamespace, []string{"pvc1"}),
		createVirtualMachineWithVolumes("vm2", testNamespace, []virtv1.Volume{
			{
				Name: "pvc2",
				VolumeSource: virtv1.VolumeSource{
					DataVolume: &virtv1.DataVolumeSource{
						Name: "pvc2",
					},
				},
			},
			{
				Name: "pvc3",
				VolumeSource: virtv1.VolumeSource{
					DataVolume: &virtv1.DataVolumeSource{
						Name: "pvc3",
					},
				},
			},
		}),
		createVirtlauncherPod("vm2", testNamespace, []string{"pvc2", "pvc3"}),
		createVirtualMachine("vm3", testNamespace),
	)
	allCompleted, err := reconciler.cancelLiveMigrations(client, instance)
	g.Expect(err).NotTo(gomega.HaveOccurred())
	g.Expect(allCompleted).To(gomega.BeTrue())
}

func TestCancelLiveMigrationsCutoverNotCompleted(t *testing.T) {
	g := gomega.NewGomegaWithT(t)
	reconciler := &ReconcileDirectVolumeMigration{}
	instance := &migrationv1alpha1.DirectVolumeMigration{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "foo",
			Namespace: migrationv1alpha1.OpenshiftMigrationNamespace,
			UID:       "1234",
		},
		Spec: migrationv1alpha1.DirectVolumeMigrationSpec{
			MigrationType: ptr.To[migapi.DirectVolumeMigrationType](migapi.MigrationTypeFinal),
			LiveMigrate:   ptr.To[bool](true),
			PersistentVolumeClaims: []migapi.PVCToMigrate{
				{
					ObjectReference: &kapi.ObjectReference{
						Namespace: testNamespace,
						Name:      "pvc1",
					},
					TargetNamespace: testNamespace,
					TargetName:      "target-pvc1",
				},
			},
		},
	}
	client := getFakeClientWithObjs(
		createVirtualMachineWithVolumes("vm1", testNamespace, []virtv1.Volume{
			{
				Name: "pvc1",
				VolumeSource: virtv1.VolumeSource{
					DataVolume: &virtv1.DataVolumeSource{
						Name: "pvc1",
					},
				},
			},
		}),
		createVirtlauncherPod("vm1", testNamespace, []string{"pvc1"}),
		createInProgressVirtualMachineMigration("vmim", testNamespace, "vm1"),
	)
	allCompleted, err := reconciler.cancelLiveMigrations(client, instance)
	g.Expect(err).NotTo(gomega.HaveOccurred())
	g.Expect(allCompleted).To(gomega.BeFalse())
}

func createDVMPod(namespace, uid string) *kapi.Pod {
	return &kapi.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "dvm-pod",
			Namespace: namespace,
			Labels: map[string]string{
				"directvolumemigration": uid,
			},
		},
	}
}

func createDVMSecret(namespace, uid string) *kapi.Secret {
	return &kapi.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "dvm-secret",
			Namespace: namespace,
			Labels: map[string]string{
				"directvolumemigration": uid,
			},
		},
	}
}

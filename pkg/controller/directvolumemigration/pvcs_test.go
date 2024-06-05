package directvolumemigration

import (
	"context"
	"github.com/go-logr/logr/testr"
	"testing"

	. "github.com/onsi/gomega"

	migapi "github.com/konveyor/mig-controller/pkg/apis/migration/v1alpha1"
	"github.com/konveyor/mig-controller/pkg/compat"
	"github.com/konveyor/mig-controller/pkg/settings"
	kapi "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
)

const (
	migNamespace   = "openshift-migration"
	testNamespace  = "test-namespace"
	testNamespace2 = "test-namespace2"
)

func Test_CreateDestinationPVCsNewPVCMigrationOwner(t *testing.T) {
	RegisterTestingT(t)
	migPlan := createMigPlan()
	sourcePVC := createSourcePvc("pvc1", testNamespace)
	client := getFakeCompatClient(&migapi.MigCluster{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "cluster",
			Namespace: migNamespace,
		},
		Spec: migapi.MigClusterSpec{
			IsHostCluster: true,
			Insecure:      true,
		},
	}, migPlan, sourcePVC, &migapi.MigMigration{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test",
			Namespace: migNamespace,
			UID:       types.UID("1111-2222-3333-4444"),
		},
	})
	task := createPlanWithPvcs(migPlan, sourcePVC, client, &migapi.PVCToMigrate{
		ObjectReference: &kapi.ObjectReference{
			Kind:       "PersistentVolumeClaim",
			Name:       "pvc1",
			Namespace:  testNamespace,
			APIVersion: "",
		},
		TargetName: "target-pvc1",
	})
	task.Log = testr.New(t)
	err := task.createDestinationPVCs()
	Expect(err).NotTo(HaveOccurred())
	resPvc := &kapi.PersistentVolumeClaim{}
	err = client.Get(context.TODO(), types.NamespacedName{Name: "target-pvc1", Namespace: testNamespace}, resPvc)
	Expect(err).NotTo(HaveOccurred())
	Expect(resPvc.Name).To(Equal("target-pvc1"))
	Expect(resPvc.Spec.DataSource).To(BeNil())
	Expect(resPvc.Spec.DataSourceRef).To(BeNil())
	pvcLabels := resPvc.GetLabels()
	Expect(pvcLabels).To(HaveKeyWithValue(migapi.MigMigrationLabel, "1111-2222-3333-4444"))
	Expect(pvcLabels).To(HaveKeyWithValue(migapi.MigPlanLabel, "4444-3333-2222-1111"))
}

func Test_CreateDestinationPVCsNewPVCDVMOwnerOtherNS(t *testing.T) {
	RegisterTestingT(t)
	migPlan := createMigPlan()
	sourcePVC := createSourcePvc("pvc1", testNamespace)
	client := getFakeCompatClient(&migapi.MigCluster{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "cluster",
			Namespace: migNamespace,
		},
		Spec: migapi.MigClusterSpec{
			IsHostCluster: true,
			Insecure:      true,
		},
	}, migPlan, sourcePVC)
	task := createPlanWithPvcs(migPlan, sourcePVC, client, &migapi.PVCToMigrate{
		ObjectReference: &kapi.ObjectReference{
			Kind:       "PersistentVolumeClaim",
			Name:       "pvc1",
			Namespace:  testNamespace,
			APIVersion: "",
		},
		TargetName:      "target-pvc1",
		TargetNamespace: testNamespace2,
	})
	task.Log = testr.New(t)
	task.Owner.SetUID(types.UID("5555-6666-7777-8888"))
	err := task.createDestinationPVCs()
	Expect(err).NotTo(HaveOccurred())
	resPvc := &kapi.PersistentVolumeClaim{}
	err = client.Get(context.TODO(), types.NamespacedName{Name: "target-pvc1", Namespace: testNamespace}, resPvc)
	Expect(err).To(HaveOccurred())
	err = client.Get(context.TODO(), types.NamespacedName{Name: "target-pvc1", Namespace: testNamespace2}, resPvc)
	Expect(err).NotTo(HaveOccurred())
	Expect(resPvc.Name).To(Equal("target-pvc1"))
	Expect(resPvc.Spec.DataSource).To(BeNil())
	Expect(resPvc.Spec.DataSourceRef).To(BeNil())
	Expect(resPvc.GetLabels()).To(HaveKeyWithValue(MigratedByDirectVolumeMigration, "5555-6666-7777-8888"))
	Expect(resPvc.Spec.Resources.Requests[kapi.ResourceStorage]).To(Equal(resource.MustParse("1Gi")))
}

func Test_CreateDestinationPVCsExpandPVSize(t *testing.T) {
	RegisterTestingT(t)
	settings.Settings.DvmOpts.EnablePVResizing = true
	defer func() { settings.Settings.DvmOpts.EnablePVResizing = false }()
	migPlan := createMigPlan()
	sourcePVC := createSourcePvc("pvc1", testNamespace)
	migPlan.Spec.PersistentVolumes.List = append(migPlan.Spec.PersistentVolumes.List, migapi.PV{
		PVC: migapi.PVC{
			Namespace: testNamespace,
			Name:      "pvc1",
		},
		Capacity: resource.MustParse("2Gi"),
	})
	client := getFakeCompatClient(&migapi.MigCluster{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "cluster",
			Namespace: migNamespace,
		},
		Spec: migapi.MigClusterSpec{
			IsHostCluster: true,
			Insecure:      true,
		},
	}, migPlan, sourcePVC)
	task := createPlanWithPvcs(migPlan, sourcePVC, client, &migapi.PVCToMigrate{
		ObjectReference: &kapi.ObjectReference{
			Kind:       "PersistentVolumeClaim",
			Name:       "pvc1",
			Namespace:  testNamespace,
			APIVersion: "",
		},
		TargetName: "target-pvc1",
	})
	task.Log = testr.New(t)
	err := task.createDestinationPVCs()
	Expect(err).NotTo(HaveOccurred())
	resPvc := &kapi.PersistentVolumeClaim{}
	err = client.Get(context.TODO(), types.NamespacedName{Name: "target-pvc1", Namespace: testNamespace}, resPvc)
	Expect(err).NotTo(HaveOccurred())
	Expect(resPvc.Name).To(Equal("target-pvc1"))
	Expect(resPvc.Spec.DataSource).To(BeNil())
	Expect(resPvc.Spec.DataSourceRef).To(BeNil())
	Expect(resPvc.Spec.Resources.Requests[kapi.ResourceStorage]).To(Equal(resource.MustParse("2Gi")))
}

func Test_CreateDestinationPVCsExpandProposedPVSize(t *testing.T) {
	RegisterTestingT(t)
	settings.Settings.DvmOpts.EnablePVResizing = true
	defer func() { settings.Settings.DvmOpts.EnablePVResizing = false }()
	migPlan := createMigPlan()
	sourcePVC := createSourcePvc("pvc1", testNamespace)
	migPlan.Spec.PersistentVolumes.List = append(migPlan.Spec.PersistentVolumes.List, migapi.PV{
		PVC: migapi.PVC{
			Namespace: testNamespace,
			Name:      "pvc1",
		},
		ProposedCapacity: resource.MustParse("4Gi"),
		Capacity:         resource.MustParse("2Gi"),
	})
	client := getFakeCompatClient(&migapi.MigCluster{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "cluster",
			Namespace: migNamespace,
		},
		Spec: migapi.MigClusterSpec{
			IsHostCluster: true,
			Insecure:      true,
		},
	}, migPlan, sourcePVC)
	task := createPlanWithPvcs(migPlan, sourcePVC, client, &migapi.PVCToMigrate{
		ObjectReference: &kapi.ObjectReference{
			Kind:       "PersistentVolumeClaim",
			Name:       "pvc1",
			Namespace:  testNamespace,
			APIVersion: "",
		},
		TargetName: "target-pvc1",
	})
	task.Log = testr.New(t)
	err := task.createDestinationPVCs()
	Expect(err).NotTo(HaveOccurred())
	resPvc := &kapi.PersistentVolumeClaim{}
	err = client.Get(context.TODO(), types.NamespacedName{Name: "target-pvc1", Namespace: testNamespace}, resPvc)
	Expect(err).NotTo(HaveOccurred())
	Expect(resPvc.Name).To(Equal("target-pvc1"))
	Expect(resPvc.Spec.DataSource).To(BeNil())
	Expect(resPvc.Spec.DataSourceRef).To(BeNil())
	Expect(resPvc.Spec.Resources.Requests[kapi.ResourceStorage]).To(Equal(resource.MustParse("4Gi")))
}

func Test_CreateDestinationPVCsExistingTargetPVC(t *testing.T) {
	RegisterTestingT(t)
	migPlan := createMigPlan()
	sourcePVC := createSourcePvc("pvc1", testNamespace)
	client := getFakeCompatClient(&migapi.MigCluster{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "cluster",
			Namespace: migNamespace,
		},
		Spec: migapi.MigClusterSpec{
			IsHostCluster: true,
			Insecure:      true,
		},
	}, migPlan, sourcePVC, &kapi.PersistentVolumeClaim{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "target-pvc1",
			Namespace: testNamespace,
		},
	})
	task := createPlanWithPvcs(migPlan, sourcePVC, client, &migapi.PVCToMigrate{
		ObjectReference: &kapi.ObjectReference{
			Kind:       "PersistentVolumeClaim",
			Name:       "pvc1",
			Namespace:  testNamespace,
			APIVersion: "",
		},
		TargetName: "target-pvc1",
	})
	task.Log = testr.New(t)
	err := task.createDestinationPVCs()
	Expect(err).NotTo(HaveOccurred())
	resPvc := &kapi.PersistentVolumeClaim{}
	err = client.Get(context.TODO(), types.NamespacedName{Name: "target-pvc1", Namespace: testNamespace}, resPvc)
	Expect(err).NotTo(HaveOccurred())
	Expect(resPvc.Name).To(Equal("target-pvc1"))
	Expect(resPvc.Spec.DataSource).To(BeNil())
	Expect(resPvc.Spec.DataSourceRef).To(BeNil())
}

func createMigPlan() *migapi.MigPlan {
	return &migapi.MigPlan{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "plan",
			Namespace: migNamespace,
			UID:       types.UID("4444-3333-2222-1111"),
		},
		Spec: migapi.MigPlanSpec{
			PersistentVolumes: migapi.PersistentVolumes{},
		},
	}
}

func createSourcePvc(name, namespace string) *kapi.PersistentVolumeClaim {
	return &kapi.PersistentVolumeClaim{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
		Spec: kapi.PersistentVolumeClaimSpec{
			AccessModes: []kapi.PersistentVolumeAccessMode{kapi.ReadWriteOnce},
			Resources: kapi.VolumeResourceRequirements{
				Requests: kapi.ResourceList{
					kapi.ResourceStorage: resource.MustParse("1Gi"),
				},
			},
			DataSource:    &kapi.TypedLocalObjectReference{},
			DataSourceRef: &kapi.TypedObjectReference{},
		},
	}
}

func createPlanWithPvcs(migPlan *migapi.MigPlan, sourcePVC *kapi.PersistentVolumeClaim, client compat.Client, migratePvcs ...*migapi.PVCToMigrate) *Task {
	task := &Task{
		Client:            client,
		sourceClient:      client,
		destinationClient: client,
		PlanResources: &migapi.PlanResources{
			MigPlan: migPlan,
		},
		Owner: &migapi.DirectVolumeMigration{
			ObjectMeta: metav1.ObjectMeta{
				Name: "test",
				OwnerReferences: []metav1.OwnerReference{
					{
						Name: "test",
					},
				},
			},
			Spec: migapi.DirectVolumeMigrationSpec{
				SrcMigClusterRef: &kapi.ObjectReference{
					Name:      "cluster",
					Namespace: migNamespace,
				},
				DestMigClusterRef: &kapi.ObjectReference{
					Name:      "cluster",
					Namespace: migNamespace,
				},
				PersistentVolumeClaims: []migapi.PVCToMigrate{},
			},
		},
	}
	for _, pvc := range migratePvcs {
		task.Owner.Spec.PersistentVolumeClaims = append(task.Owner.Spec.PersistentVolumeClaims, *pvc)
	}
	return task
}

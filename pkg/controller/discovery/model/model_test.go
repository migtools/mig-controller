package model

import (
	"github.com/fusor/mig-controller/pkg/logging"
	"github.com/google/uuid"
	"github.com/onsi/gomega"
	rbac "k8s.io/api/rbac/v1beta1"
	"os"
	pathlib "path"
	"testing"
)

func init() {
	Settings.Load()
	log := logging.WithName("Test")
	Log = &log
}

func uid() string {
	uid, _ := uuid.NewUUID()
	return uid.String()
}

func TestModels(t *testing.T) {
	g := gomega.NewGomegaWithT(t)
	path := pathlib.Join(Settings.WorkingDir, "discovery.db")
	os.Remove(path)
	db, err := Create()
	g.Expect(db != nil).To(gomega.BeTrue())
	g.Expect(err).To(gomega.BeNil())

	// Cluster
	cluster := Cluster{
		Base: Base{
			UID:       uid(),
			Version:   uid(),
			Namespace: "ns1",
			Name:      "cl",
		},
	}
	err = cluster.Insert(db)
	g.Expect(err).To(gomega.BeNil())
	err = cluster.Select(db)
	g.Expect(err).To(gomega.BeNil())
	err = cluster.Update(db)
	g.Expect(err).To(gomega.BeNil())

	// Plan
	plan := Plan{
		Base: Base{
			UID:       uid(),
			Version:   uid(),
			Namespace: "ns1",
			Name:      "cl",
		},
		Source:      "{}",
		Destination: "{}",
	}

	err = plan.Insert(db)
	g.Expect(err).To(gomega.BeNil())
	err = plan.Select(db)
	g.Expect(err).To(gomega.BeNil())
	err = plan.Update(db)
	g.Expect(err).To(gomega.BeNil())

	// Namespaces
	ns := Namespace{
		Base: Base{
			UID:     uid(),
			Version: uid(),
			Cluster: cluster.PK,
			Name:    "ns1",
		},
	}
	err = ns.Insert(db)
	g.Expect(err).To(gomega.BeNil())

	// Pod
	pod := Pod{
		Base: Base{
			Cluster:   cluster.PK,
			UID:       uid(),
			Version:   uid(),
			Namespace: ns.Name,
			Name:      "pod1",
		},
		Definition: "{}",
		labels:     Labels{"app": "cam"},
	}
	err = pod.Insert(db)
	g.Expect(err).To(gomega.BeNil())
	err = pod.addLabels(db)
	g.Expect(err).To(gomega.BeNil())
	err = pod.Select(db)
	g.Expect(err).To(gomega.BeNil())
	err = pod.Update(db)
	g.Expect(err).To(gomega.BeNil())

	// PV
	pv := PV{
		Base: Base{
			Cluster:   cluster.PK,
			UID:       uid(),
			Version:   uid(),
			Namespace: ns.Name,
			Name:      "pv1",
		},
		Definition: "{}",
	}
	err = pv.Insert(db)
	g.Expect(err).To(gomega.BeNil())
	err = pv.Select(db)
	g.Expect(err).To(gomega.BeNil())
	err = pv.Update(db)
	g.Expect(err).To(gomega.BeNil())

	// RoleBinding
	rb := RoleBinding{
		Base: Base{
			Cluster:   cluster.PK,
			UID:       uid(),
			Version:   uid(),
			Namespace: ns.Name,
			Name:      "rb",
		},
		Role: "admin",
		subjects: []rbac.Subject{
			{
				Kind:      "User",
				Namespace: ns.Name,
				Name:      "user1",
			},
			{
				Kind:      "ServiceAccount",
				Namespace: ns.Name,
				Name:      "sa1",
			},
		},
	}
	err = rb.Insert(db)
	g.Expect(err).To(gomega.BeNil())
	err = rb.Select(db)
	g.Expect(err).To(gomega.BeNil())
	err = rb.Update(db)
	g.Expect(err).To(gomega.BeNil())

	// RoleBinding
	crb := RoleBinding{
		Base: Base{
			Cluster: cluster.PK,
			UID:     uid(),
			Version: uid(),
			Name:    "rb",
		},
		Role: "admin",
		subjects: []rbac.Subject{
			{
				Kind: "User",
				Name: "user1",
			},
			{
				Kind:      "ServiceAccount",
				Namespace: ns.Name,
				Name:      "sa1",
			},
		},
	}
	err = crb.Insert(db)
	g.Expect(err).To(gomega.BeNil())
	err = crb.Select(db)
	g.Expect(err).To(gomega.BeNil())
	err = crb.Update(db)
	g.Expect(err).To(gomega.BeNil())

	// role
	role := Role{
		Base: Base{
			Cluster:   cluster.PK,
			UID:       uid(),
			Version:   uid(),
			Namespace: ns.Name,
			Name:      "role1",
		},
		Rules: "[]",
	}
	err = role.Insert(db)
	g.Expect(err).To(gomega.BeNil())
	err = role.Select(db)
	g.Expect(err).To(gomega.BeNil())
	err = role.Update(db)
	g.Expect(err).To(gomega.BeNil())

	// Listing
	labelFilter := LabelFilter{Label{Name: "app", Value: "cam"}}

	cList, err := ClusterList(db, &Page{Limit: 1})
	g.Expect(err).To(gomega.BeNil())
	g.Expect(len(cList)).To(gomega.Equal(1))

	nsList, err := cluster.NsList(db, &Page{Limit: 1})
	g.Expect(err).To(gomega.BeNil())
	g.Expect(len(nsList)).To(gomega.Equal(1))

	podList, err := cluster.PodList(db, &Page{Limit: 1})
	g.Expect(err).To(gomega.BeNil())
	g.Expect(len(podList)).To(gomega.Equal(1))

	podList, err = ns.PodList(db, &Page{Limit: 1})
	g.Expect(err).To(gomega.BeNil())
	g.Expect(len(podList)).To(gomega.Equal(1))

	podList, err = cluster.PodListByLabel(db, labelFilter, &Page{Limit: 1})
	g.Expect(err).To(gomega.BeNil())
	g.Expect(len(podList) == 1).To(gomega.BeTrue())

	podList, err = ns.PodListByLabel(db, labelFilter, &Page{Limit: 1})
	g.Expect(err).To(gomega.BeNil())
	g.Expect(len(podList)).To(gomega.Equal(1))

	podList, err = PodListByLabel(db, labelFilter, &Page{Limit: 1})
	g.Expect(err).To(gomega.BeNil())
	g.Expect(len(podList)).To(gomega.Equal(1))

	pvList, err := cluster.PvList(db, &Page{Limit: 1})
	g.Expect(err).To(gomega.BeNil())
	g.Expect(len(pvList) == 1).To(gomega.BeTrue())

	rbList, err := cluster.RoleBindingList(db)
	g.Expect(err).To(gomega.BeNil())
	g.Expect(len(rbList) == 2).To(gomega.BeTrue())

	rbList, err = cluster.RoleBindingListBySubject(db, Subject{Kind: SubjectUser, Name: "user1"})
	g.Expect(err).To(gomega.BeNil())
	g.Expect(len(rbList) == 1).To(gomega.BeTrue())

	roleList, err := cluster.RoleList(db)
	g.Expect(err).To(gomega.BeNil())
	g.Expect(len(roleList) == 1).To(gomega.BeTrue())

	// Delete all.
	err = role.Delete(db)
	g.Expect(err).To(gomega.BeNil())
	err = rb.Delete(db)
	g.Expect(err).To(gomega.BeNil())
	err = ns.Delete(db)
	g.Expect(err).To(gomega.BeNil())
	err = pv.Delete(db)
	g.Expect(err).To(gomega.BeNil())
	g.Expect(err).To(gomega.BeNil())
	err = pod.Delete(db)
	g.Expect(err).To(gomega.BeNil())
	err = plan.Delete(db)
	g.Expect(err).To(gomega.BeNil())
	err = cluster.Delete(db)
	g.Expect(err).To(gomega.BeNil())
}

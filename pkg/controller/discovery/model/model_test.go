package model

import (
	"github.com/fusor/mig-controller/pkg/logging"
	"github.com/onsi/gomega"
	"os"
	pathlib "path"
	"testing"
)

func init() {
	Settings.Load()
	log := logging.WithName("Test")
	Log = &log
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
			Name:    "ns1",
			Cluster: cluster.PK,
		},
	}
	err = ns.Insert(db)
	g.Expect(err).To(gomega.BeNil())

	// Pod
	pod := Pod{
		Base: Base{
			Namespace: ns.Name,
			Name:      "pod1",
			Cluster:   cluster.PK,
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
			Namespace: ns.Name,
			Name:      "pv1",
			Cluster:   cluster.PK,
		},
		Definition: "{}",
	}
	err = pv.Insert(db)
	g.Expect(err).To(gomega.BeNil())
	err = pv.Select(db)
	g.Expect(err).To(gomega.BeNil())
	err = pv.Update(db)
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

	// Delete all.
	err = ns.Delete(db)
	g.Expect(err).To(gomega.BeNil())
	err = pod.Delete(db)
	g.Expect(err).To(gomega.BeNil())
	err = plan.Delete(db)
	g.Expect(err).To(gomega.BeNil())
	err = cluster.Delete(db)
	g.Expect(err).To(gomega.BeNil())
}

package model

import (
	"fmt"
	"github.com/konveyor/controller/pkg/logging"
	"github.com/onsi/gomega"
	"os"
	pathlib "path"
	"testing"
)

var uid = 0

func init() {
	Settings.Load()
	log := logging.WithName("Test")
	Log = log
}

func UID() string {
	uid++
	return fmt.Sprint(uid)
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
		CR: CR{
			UID:       UID(),
			Namespace: "ns1",
			Name:      "c1",
		},
	}
	err = cluster.Insert(db)
	g.Expect(err).To(gomega.BeNil())
	err = cluster.Get(db)
	g.Expect(err).To(gomega.BeNil())
	err = cluster.Update(db)
	g.Expect(err).To(gomega.BeNil())

	// Plan
	plan := Plan{
		CR: CR{
			UID:       UID(),
			Namespace: "ns1",
			Name:      "p1",
			Object:    "{}",
		},
	}

	err = plan.Insert(db)
	g.Expect(err).To(gomega.BeNil())
	err = plan.Get(db)
	g.Expect(err).To(gomega.BeNil())
	err = plan.Update(db)
	g.Expect(err).To(gomega.BeNil())

	// Namespaces
	ns := Namespace{
		Base: Base{
			UID:     UID(),
			Name:    "ns1",
			Cluster: cluster.PK,
		},
	}
	err = ns.Insert(db)
	g.Expect(err).To(gomega.BeNil())
	ns2 := Namespace{
		Base: Base{
			UID:     UID(),
			Name:    "ns2",
			Cluster: cluster.PK,
		},
	}
	err = ns2.Insert(db)
	g.Expect(err).To(gomega.BeNil())

	// Pod
	labels := Labels{
		"app":     "cam",
		"owner":   "redhat",
		"purpose": "testing",
	}

	pod := &Pod{
		Base: Base{
			UID:       UID(),
			Namespace: ns.Name,
			Name:      "pod1",
			Cluster:   cluster.PK,
			Object:    "{}",
			labels:    labels,
		},
	}
	err = pod.Insert(db)
	g.Expect(err).To(gomega.BeNil())
	g.Expect(err).To(gomega.BeNil())
	err = pod.Get(db)
	g.Expect(err).To(gomega.BeNil())
	err = pod.Update(db)
	g.Expect(err).To(gomega.BeNil())
	pod2 := Pod{
		Base: Base{
			UID:       UID(),
			Namespace: "ns2",
			Name:      "pod2",
			Cluster:   cluster.PK,
			Object:    "{}",
		},
	}
	err = pod2.Insert(db)
	g.Expect(err).To(gomega.BeNil())

	// PV
	pv := PV{
		Base: Base{
			UID:       UID(),
			Namespace: ns.Name,
			Name:      "pv1",
			Cluster:   cluster.PK,
			Object:    "{}",
		},
	}
	err = pv.Insert(db)
	g.Expect(err).To(gomega.BeNil())
	err = pv.Get(db)
	g.Expect(err).To(gomega.BeNil())
	err = pv.Update(db)
	g.Expect(err).To(gomega.BeNil())

	// Listing

	cList, err := Cluster{}.List(db, ListOptions{})
	g.Expect(err).To(gomega.BeNil())
	g.Expect(len(cList)).To(gomega.Equal(1))

	nsList, err := Namespace{
		Base: Base{
			Cluster: cluster.PK,
		},
	}.List(db, ListOptions{Sort: []int{4, 5}})
	g.Expect(err).To(gomega.BeNil())
	g.Expect(len(nsList)).To(gomega.Equal(2))

	podList, err := Pod{
		Base: Base{
			Cluster:   cluster.PK,
			Namespace: ns.Name,
		},
	}.List(db, ListOptions{})
	g.Expect(err).To(gomega.BeNil())
	g.Expect(len(podList)).To(gomega.Equal(1))

	podList, err = Pod{
		Base: Base{
			Cluster: cluster.PK,
		},
	}.List(db,
		ListOptions{
			Page: &Page{Limit: 10},
			Labels: Labels{
				"app": "cam",
			},
		})
	g.Expect(err).To(gomega.BeNil())
	g.Expect(len(podList)).To(gomega.Equal(1))
	g.Expect(podList[0].Name).To(gomega.Equal(pod.Name))

	// count
	count, err := Table{db}.Count(&Pod{}, ListOptions{})
	g.Expect(err).To(gomega.BeNil())
	g.Expect(count).To(gomega.Equal(int64(2)))

	count, err = Table{db}.Count(
		&Pod{}, ListOptions{
			Labels: Labels{
				"app": "cam",
			},
		})
	g.Expect(err).To(gomega.BeNil())
	g.Expect(count).To(gomega.Equal(int64(1)))

	// Delete all.
	err = pv.Delete(db)
	g.Expect(err).To(gomega.BeNil())
	err = ns.Delete(db)
	g.Expect(err).To(gomega.BeNil())
	err = pod.Delete(db)
	g.Expect(err).To(gomega.BeNil())
	err = plan.Delete(db)
	g.Expect(err).To(gomega.BeNil())
	err = cluster.Delete(db)
	g.Expect(err).To(gomega.BeNil())

}

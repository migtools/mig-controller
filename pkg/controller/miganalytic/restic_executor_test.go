/*
Copyright 2021 Red Hat Inc.

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
	"math/rand"

	migapi "github.com/konveyor/mig-controller/pkg/apis/migration/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func getResticPod(ns string, name string, nodeName string) corev1.Pod {
	return corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: ns,
		},
		Spec: corev1.PodSpec{
			NodeName: nodeName,
		},
	}
}

var resticPod1 = getResticPod("openshift-migration", "restic-00", "node1.migration.internal")
var resticPod2 = getResticPod("openshift-migration", "restic-01", "node2.migration.internal")

var testAnalytic = migapi.MigAnalytic{
	ObjectMeta: metav1.ObjectMeta{
		Namespace: "openshift-migration",
	},
}

// FakeDFExecutor mocks df executor
type FakeDFExecutor struct{}

func withChance(prob float64) bool {
	return rand.Int63n(100) > int64(prob*100)
}

// Execute mocks execution of df command
func (f *FakeDFExecutor) Execute(pvNodeMap map[string][]MigAnalyticPersistentVolumeDetails) ([]DFOutput, error) {
	gatheredData := []DFOutput{}
	for node, pvcs := range pvNodeMap {
		for _, pvc := range pvcs {
			pvUsageData := DFOutput{}
			pvUsageData.Node = node
			pvUsageData.Name = pvc.Name
			pvUsageData.Namespace = pvc.Namespace
			if withChance(0.7) {
				pvUsageData.IsError = true
			} else {
				pvUsageData.UsagePercentage = rand.Int63n(99)
				if withChance(0.3) {
					pvUsageData.TotalSize = pvc.RequestedCapacity
				} else if withChance(0.5) {
					pvUsageData.TotalSize = pvc.ProvisionedCapacity
				} else {
					adder := resource.Quantity{
						Format: pvc.ProvisionedCapacity.Format,
					}
					adder.Set(int64(float64(pvUsageData.TotalSize.Value()) * rand.Float64()))
					pvUsageData.TotalSize = pvc.ProvisionedCapacity
					pvUsageData.TotalSize.Add(adder)
				}
			}
			gatheredData = append(gatheredData, pvUsageData)
		}
	}
	return gatheredData, nil
}
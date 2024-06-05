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
	"context"
	"path"
	"sync"

	liberr "github.com/konveyor/controller/pkg/error"
	"github.com/konveyor/mig-controller/pkg/compat"
	"github.com/konveyor/mig-controller/pkg/pods"
	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	// ResticPodLabelKey is the key of the label used to discover Restic pod
	ResticPodLabelKey = "name"
	// ResticPodLabelValue is the value of the label used to discover Restic pod
	ResticPodLabelValue = "node-agent"
)

// ResticDFCommandExecutor uses Restic pods to run DF command
type ResticDFCommandExecutor struct {
	// Namespace is the ns in which Restic pods are present
	Namespace string
	// Client to interact with Restic pods
	Client compat.Client
	// ResticPodReferences is a local cache of known Restic pods
	ResticPodReferences map[string]*corev1.Pod
}

// ExecuteStorageCommands given a podRef and a list of volumes, runs df command, returns with structured command context
// any errors running the df command are suppressed here. DFCommand.stdErr field should be used to determine failure
func (r *ResticDFCommandExecutor) ExecuteStorageCommands(podRef *corev1.Pod, persistentVolumes []MigAnalyticPersistentVolumeDetails) (DF, DU) {
	// TODO: use the appropriate block size based on PVCs
	storageCommand := StorageCommand{
		BaseLocation: "/host_pods",
		BlockSize:    DecimalSIMega,
		StdOut:       "",
		StdErr:       "",
	}
	dfCmd := DF{StorageCommand: storageCommand}
	duCmd := DU{StorageCommand: storageCommand}
	dfCmdString := dfCmd.PrepareCommand(persistentVolumes)
	duCmdString := duCmd.PrepareCommand(persistentVolumes)
	restCfg := r.Client.RestConfig()
	podDfCommand := pods.PodCommand{
		Pod:     podRef,
		RestCfg: restCfg,
		Args:    dfCmdString,
	}
	podDuCommand := pods.PodCommand{
		Pod:     podRef,
		RestCfg: restCfg,
		Args:    duCmdString,
	}
	log.Info(0, "Executing df command inside source cluster Restic Pod to measure actual usage for extended PV analysis",
		"pod", path.Join(podRef.Namespace, podRef.Name),
		"command", dfCmdString)
	err := podDfCommand.Run()
	if err != nil {
		log.Error(err, "Failed running df command inside Restic Pod",
			"pod", path.Join(podRef.Namespace, podRef.Name),
			"command", dfCmdString)
	}
	dfCmd.StdErr = podDfCommand.Err.String()
	dfCmd.StdOut = podDfCommand.Out.String()
	log.Info(0, "Executing du command inside source cluster Restic Pod to measure actual usage for extended PV analysis",
		"pod", path.Join(podRef.Namespace, podRef.Name),
		"command", duCmdString)
	err = podDuCommand.Run()
	if err != nil {
		log.Error(err, "Failed running du command inside Restic Pod",
			"pod", path.Join(podRef.Namespace, podRef.Name),
			"command", duCmdString)
	}
	duCmd.StdErr = podDuCommand.Err.String()
	duCmd.StdOut = podDuCommand.Out.String()
	return dfCmd, duCmd
}

// getResticPodForNode lookup Restic Pod ref in local cache
func (r *ResticDFCommandExecutor) getResticPodForNode(nodeName string) *corev1.Pod {
	if podRef, exists := r.ResticPodReferences[nodeName]; exists {
		return podRef
	}
	return nil
}

// loadResticPodReferences load Restic Pod refs in-memory
func (r *ResticDFCommandExecutor) loadResticPodReferences() error {
	if r.ResticPodReferences == nil {
		r.ResticPodReferences = make(map[string]*corev1.Pod)
	}
	resticPodList := corev1.PodList{}

	err := r.Client.List(context.TODO(),
		&resticPodList,
		client.InNamespace(r.Namespace),
		client.MatchingLabels(map[string]string{ResticPodLabelKey: ResticPodLabelValue}),
	)
	if err != nil {
		return liberr.Wrap(err)
	}
	for i := range resticPodList.Items {
		if resticPodList.Items[i].Spec.NodeName != "" {
			r.ResticPodReferences[resticPodList.Items[i].Spec.NodeName] = &resticPodList.Items[i]
		}
	}
	return nil
}

type DfDu struct {
	DF
	DU
}

// Execute given a map node->[]pvc, runs Df command for each, returns list of structured df output per pvc
func (r *ResticDFCommandExecutor) Execute(pvcNodeMap map[string][]MigAnalyticPersistentVolumeDetails) ([]DFOutput, []DUOutput, error) {
	gatheredDfData := []DFOutput{}
	gatheredDuData := []DUOutput{}
	err := r.loadResticPodReferences()
	if err != nil {
		return gatheredDfData, gatheredDuData, liberr.Wrap(err)
	}
	// dfOutputs for n nodes
	dfOutputs := make(map[string]DfDu, len(pvcNodeMap))
	waitGroup := sync.WaitGroup{}
	mutex := sync.Mutex{}
	// allows setting a limit on number of concurrent df threads running
	bufferedExecutionChannel := make(chan struct{}, 10)
	// run df concurrently for 'n' nodes
	for node := range pvcNodeMap {
		resticPodRef := r.getResticPodForNode(node)
		// if no Restic pod is found for this node, all PVCs on this node are skipped
		if resticPodRef == nil {
			for _, pvc := range pvcNodeMap[node] {
				StorageCommandOutput := StorageCommandOutput{
					IsError: true,
				}
				dfOutput := DFOutput{
					StorageCommandOutput: StorageCommandOutput,
					Node:                 node,
					Name:                 pvc.Name,
					Namespace:            pvc.Namespace}
				duOutput := DUOutput{
					StorageCommandOutput: StorageCommandOutput,
					Name:                 pvc.Name,
					Namespace:            pvc.Namespace}
				gatheredDfData = append(gatheredDfData, dfOutput)
				gatheredDuData = append(gatheredDuData, duOutput)
			}
			continue
		}
		waitGroup.Add(1)
		go func(n string, podRef *corev1.Pod) {
			// block until channel empty
			bufferedExecutionChannel <- struct{}{}
			defer waitGroup.Done()
			dfOutput, duOutput := r.ExecuteStorageCommands(podRef, pvcNodeMap[n])
			mutex.Lock()
			defer mutex.Unlock()
			dfOutputs[n] = DfDu{
				DF: dfOutput,
				DU: duOutput,
			}
			// free up channel indicating execution finished
			<-bufferedExecutionChannel
		}(node, resticPodRef)
	}
	// wait for all command instances to return
	waitGroup.Wait()
	for node, cmdOutput := range dfOutputs {
		for _, pvc := range pvcNodeMap[node] {
			pvcDFInfo := cmdOutput.DF.GetOutputForPV(pvc.VolumeName, pvc.PodUID)
			pvcDFInfo.Node = node
			pvcDFInfo.Name = pvc.Name
			pvcDFInfo.Namespace = pvc.Namespace
			pvcDUInfo := cmdOutput.DU.GetOutputForPV(pvc.VolumeName, pvc.PodUID)
			pvcDUInfo.Name = pvc.Name
			pvcDUInfo.Namespace = pvc.Namespace
			gatheredDfData = append(gatheredDfData, pvcDFInfo)
			gatheredDuData = append(gatheredDuData, pvcDUInfo)
			log.Info(0, "Got `df` command output from Restic Pod",
				"persistentVolumeClaim", path.Join(pvc.Namespace, pvc.Name),
				"resticPodUID", pvc.PodUID,
				"pvcStorageClass", pvc.StorageClass,
				"pvcRequestedCapacity", pvc.RequestedCapacity,
				"pvcProvisionedCapacity", pvc.ProvisionedCapacity,
				"usagePercentage", pvcDFInfo.UsagePercentage,
				"totalSize", pvcDFInfo.TotalSize)
			log.Info(0, "Got `du` command output from Restic Pod",
				"persistentVolumeClaim", path.Join(pvc.Namespace, pvc.Name),
				"usage", pvcDUInfo.Usage)
		}
	}
	return gatheredDfData, gatheredDuData, nil
}

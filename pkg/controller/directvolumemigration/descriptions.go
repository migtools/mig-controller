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

package directvolumemigration

var phaseDescriptions = map[string]string{
	Created:                              "DVM CR has been created",
	Started:                              "DVM Controller is configuring DVM CR",
	Prepare:                              "DVM Controller is preparing the environment for volume migration.",
	CleanStaleRsyncResources:             "Cleaning up stale resources from previous migrations",
	WaitForStaleRsyncResourcesTerminated: "Waiting for stale resources to terminate",
	CreateDestinationNamespaces:          "Creating target namespaces",
	DestinationNamespacesCreated:         "Checking if the target namespaces have been created.",
	CreateDestinationPVCs:                "Creating PVCs in the target namespaces",
	DestinationPVCsCreated:               "Checking whether the created PVCs are bound",
	CreateRsyncRoute:                     "Creating one route for each namespace for Rsync on the target cluster",
	CreateRsyncConfig:                    "Creating a config map and secrets on both the source and target clusters for Rsync configuration",
	CreateStunnelConfig:                  "Creating a config map and secrets for Stunnel to connect to Rsync on the source and target clusters",
	CreatePVProgressCRs:                  "Creating a Direct Volume Migration Progress CR to get progress percentage and transfer rate",
	CreateRsyncTransferPods:              "Creating Rsync daemon pods on the target cluster",
	WaitForRsyncTransferPodsRunning:      "Waiting for the Rsync daemon pod to run",
	EnsureRsyncRouteAdmitted:             "Waiting for Rsync route to be admitted.",
	CreateStunnelClientPods:              "Creating Stunnel client pods on the source cluster",
	WaitForStunnelClientPodsRunning:      "Waiting for the Stunnel client pods to run",
	CreateRsyncClientPods:                "Creating Rsync client pods",
	WaitForRsyncClientPodsCompleted:      "Waiting for the Rsync client pods to be completed",
	DeleteRsyncResources:                 "Deleting Rsync resources created by this migration",
	WaitForRsyncResourcesTerminated:      "Waiting for Rsync resources to terminate",
	RunRsyncOperations:                   "Attempting to transfer Persistent Volume data using Rsync",
	Verification:                         "Verifying migration was successful",
	MigrationFailed:                      "The migration attempt failed, please see errors for more details",
	Completed:                            "Complete",
}

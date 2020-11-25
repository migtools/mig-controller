package directvolumemigration

var phaseDescriptions = map[string]string{
	Created:                         "The DVM controller just saw DVM CR being created",
	Started:                         "The DVM controller is starting to work on the DVM CR",
	Prepare:                         "The DVM controller prepares the environment for volume migration, currently it is a no-op",
	CreateDestinationNamespaces:     "Creating destination namespaces, no-op if they exist",
	DestinationNamespacesCreated:    "Checking if the destination namespace is created, currently it is a no-op",
	CreateDestinationPVCs:           "Creating PVCs to be migrated on the destination namespace, no-op if they exist",
	DestinationPVCsCreated:          "Check if all the PVCs that were created are Bound, currently a no-op",
	CreateRsyncRoute:                "Creating one route per namespace for rsync on the destination cluster",
	CreateRsyncConfig:               "Creating configmap and secrets on both source and destination clusters for rsync configuration",
	CreateStunnelConfig:             "Creating configmap and secrets for stunnel certs used to connect to rsync on both source and destionation cluster",
	CreatePVProgressCRs:             "Creating direct volume migration progress CR to get progress percent and transfer rate",
	CreateRsyncTransferPods:         "Creating rsync daemon pods on the destination cluster",
	WaitForRsyncTransferPodsRunning: "Wait for the rsync daemon pod to be running",
	CreateStunnelClientPods:         "Create stunnel client pods on the source cluster",
	WaitForStunnelClientPodsRunning: "Waiting for the stunnel client pods to be running",
	CreateRsyncClientPods:           "Create Rsync client pods",
	WaitForRsyncClientPodsCompleted: "Wait for the rsync client pods to be completed",
	DeleteRsyncResources:            "Delete all the resources created while running this migration",
	MigrationFailed:                 "The migration attempt failed, please see errors for more details",
	Completed:                       "Complete",
}

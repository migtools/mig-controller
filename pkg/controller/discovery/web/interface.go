package web

// Migration application CR.
type MigResource interface {
	// Get a map containing the correlation label.
	// Correlation labels are used to track any resource
	// created by the controller.  The includes the application
	// (global) label and the resource label.
	GetCorrelationLabels() map[string]string
	// Get the resource correlation label.
	// The label is used to track resources created by the
	// controller that is related to this resource.
	GetCorrelationLabel() (string, string)
	// Get the resource namespace.
	GetNamespace() string
	// Get the resource name.
	GetName() string
	// Mark the resource as having been reconciled.
	// Updates the ObservedDigest.
	// Update the touch annotation. This ensures that the resource
	// is changed on every reconcile. This needed to support
	// remote watch event propagation on OCP4.
	MarkReconciled()
	// Get whether the resource has been reconciled.
	HasReconciled() bool
}

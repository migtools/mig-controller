package migmigration

func (t *Task) EnsureAuthorized() ([]string, error) {
	notAuthorized := make([]string, 0)

	sourceAuth, err := t.sourceNamespacesAuthorized()
	if err != nil {
		return nil, err
	}
	for ns, auth := range sourceAuth {
		if !auth {
			notAuthorized = append(notAuthorized, "(source) "+ns)
		}
	}

	destAuth, err := t.destinationNamespacesAuthorized()
	if err != nil {
		return nil, err
	}
	for ns, auth := range destAuth {
		if !auth {
			notAuthorized = append(notAuthorized, "(destination) "+ns)
		}
	}

	return notAuthorized, nil
}

func (t *Task) sourceNamespacesAuthorized() (map[string]bool, error) {
	var authorized map[string]bool
	sourceNamespaces := t.PlanResources.MigPlan.GetSourceNamespaces()
	identity, err := t.PlanResources.MigPlan.GetSourceIdentity(t.Client)
	if err != nil {
		return authorized, err
	}
	authorized, err = identity.HasRead(sourceNamespaces)
	if err != nil {
		return authorized, err
	}
	migrateAuthorized, err := identity.HasMigrate(sourceNamespaces)
	if err != nil {
		return authorized, err
	}
	for ns, auth := range migrateAuthorized {
		authorized[ns] = authorized[ns] && auth
	}

	return authorized, nil
}

func (t *Task) destinationNamespacesAuthorized() (map[string]bool, error) {
	var authorized map[string]bool
	destinationNamespaces := t.PlanResources.MigPlan.GetDestinationNamespaces()
	identity, err := t.PlanResources.MigPlan.GetDestinationIdentity(t.Client)
	if err != nil {
		return authorized, err
	}
	authorized, err = identity.HasMigrate(destinationNamespaces)
	if err != nil {
		return authorized, err
	}

	return authorized, nil
}

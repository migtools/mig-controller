package migtoken

import (
	migapi "github.com/konveyor/mig-controller/pkg/apis/migration/v1alpha1"
)

// Categories
const (
	Critical = migapi.Critical
)

// Reasons
const (
	NotAuthorized    = "NotAuthorized"
	NotAuthenticated = "NotAuthenticated"
	UseNotGranted    = "UseNotGranted"
)

// Statuses
const (
	True = migapi.True
)

// Messages
const (
	ReadyMessage            = "The token is ready."
	NotAuthenticatedMessage = "The token is not authenticated"
	UseNotGrantedMessage    = "The token is not granted use on the associated MigCluster. Please contact your administrator."
)

func (r ReconcileMigToken) validate(token *migapi.MigToken) error {
	authenticated, err := r.validateAuthentication(token)
	if err != nil {
		return err
	}
	if !authenticated {
		token.Status.SetCondition(migapi.Condition{
			Type:     NotAuthenticated,
			Status:   True,
			Reason:   NotAuthenticated,
			Category: Critical,
			Message:  NotAuthenticatedMessage,
		})
	}
	allowed, err := r.validateMigClusterRef(token)
	if err != nil {
		return err
	}
	if !allowed {
		token.Status.SetCondition(migapi.Condition{
			Type:     NotAuthorized,
			Status:   True,
			Reason:   UseNotGranted,
			Category: Critical,
			Message:  UseNotGrantedMessage,
		})
	}
	return nil
}

func (r ReconcileMigToken) validateAuthentication(token *migapi.MigToken) (bool, error) {
	auth, err := token.Authenticate(r.Client)
	if err != nil {
		return false, err
	}
	return auth, nil
}

func (r ReconcileMigToken) validateMigClusterRef(token *migapi.MigToken) (bool, error) {
	auth, err := token.HasUsePermission(r.Client)
	return auth, err
}

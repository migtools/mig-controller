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
	NotAuthenticated = "NotAuthenticated"
)

// Statuses
const (
	True = migapi.True
)

// Nessages
const (
	ReadyMessage            = "The token is ready."
	NotAuthenticatedMessage = "The token is not authenticated"
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
	return nil
}

func (r ReconcileMigToken) validateAuthentication(token *migapi.MigToken) (bool, error) {
	auth, err := token.Authenticate(r.Client)
	if err != nil {
		return false, err
	}
	return auth, nil
}

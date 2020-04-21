package auth

import (
	"context"

	"k8s.io/api/authentication/v1beta1"
	authapi "k8s.io/api/authorization/v1"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	k8sclient "sigs.k8s.io/controller-runtime/pkg/client"
)

type Authorized map[string]bool

type Identity struct {
	Token   string
	RestCfg rest.Config
	Client  k8sclient.Client
}

func (r *Identity) HasRead(namespaces []string) (Authorized, error) {
	authorized := Authorized{}
	err := r.BuildClient()
	if err != nil {
		return authorized, err
	}
	for _, namespace := range namespaces {
		sar := authapi.SelfSubjectAccessReview{
			Spec: authapi.SelfSubjectAccessReviewSpec{
				ResourceAttributes: &authapi.ResourceAttributes{
					Resource:  "namespaces",
					Namespace: namespace,
					Verb:      "get",
				},
			},
		}
		err := r.Client.Create(context.TODO(), &sar)
		if err != nil {
			return authorized, err
		}
		authorized[namespace] = sar.Status.Allowed
	}

	return authorized, nil
}

func (r *Identity) HasMigrate(namespaces []string) (Authorized, error) {
	authorized := Authorized{}
	err := r.BuildClient()
	if err != nil {
		return authorized, err
	}

	return authorized, nil
}

func (r *Identity) Authenticates(client k8sclient.Client) (bool, error) {
	tokenReview := v1beta1.TokenReview{
		Spec: v1beta1.TokenReviewSpec{
			Token: r.Token,
		},
	}
	err := client.Create(context.TODO(), &tokenReview)
	if err != nil {
		return false, err
	}
	return tokenReview.Status.Authenticated, nil
}

func (r *Identity) BuildClient() error {
	if r.Client != nil {
		return nil
	}
	// build client using r.RestCfg and replacing the token with r.Token.
	restCfg := r.RestCfg
	restCfg.BearerToken = r.Token
	client, err := k8sclient.New(&restCfg, k8sclient.Options{Scheme: scheme.Scheme})
	if err != nil {
		return err
	}
	r.Client = client
	return nil
}

// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package certmanagement

import (
	"context"
	"fmt"

	certv1alpha1 "github.com/gardener/cert-management/pkg/apis/cert/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	operatorv1alpha1 "github.com/gardener/gardener/pkg/apis/operator/v1alpha1"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	"github.com/gardener/gardener/pkg/component"
	"github.com/gardener/gardener/pkg/utils/managedresources"
)

const (
	// componentName is the name of the cert-management component.
	componentName = "cert-management"
	// issuersManagedResourceName is the name of the issuers ManagedResource.
	issuersManagedResourceName = "cert-management-issuers"
	// DefaultIssuerName is the name of the default issuer.
	DefaultIssuerName = "default-issuer"

	appName     = "app.kubernetes.io/name"
	appInstance = "app.kubernetes.io/instance"
)

type certManagement struct {
	values Values
	client client.Client
}

// Values is a set of configuration values for the cert-management component.
type Values struct {
	Image         string
	Namespace     string
	DeployConfig  *operatorv1alpha1.CertManagementConfig
	DefaultIssuer operatorv1alpha1.DefaultIssuer
}

var listOpts = []client.ListOption{
	client.InNamespace(v1beta1constants.GardenNamespace),
	client.MatchingLabels{"app.kubernetes.io/name": componentName},
}

// NewDefaultIssuer creates a new Deployer for the cert-management component.
func NewDefaultIssuer(
	cl client.Client,
	values Values,
) component.DeployWaiter {
	return &certManagement{
		values: values,
		client: cl,
	}
}

var _ component.DeployWaiter = &certManagement{}

func (c *certManagement) Destroy(ctx context.Context) error {
	return managedresources.DeleteForSeed(ctx, c.client, v1beta1constants.GardenNamespace, issuersManagedResourceName)
}

func (c *certManagement) Wait(ctx context.Context) error {
	return managedresources.WaitUntilHealthy(ctx, c.client, v1beta1constants.GardenNamespace, issuersManagedResourceName)
}

func (c *certManagement) WaitCleanup(ctx context.Context) error {
	return managedresources.WaitUntilDeleted(ctx, c.client, v1beta1constants.GardenNamespace, issuersManagedResourceName)
}

func (c *certManagement) Deploy(ctx context.Context) error {
	registry := managedresources.NewRegistry(kubernetes.SeedScheme, kubernetes.SeedCodec, kubernetes.SeedSerializer)

	issuerObj := &certv1alpha1.Issuer{
		ObjectMeta: metav1.ObjectMeta{
			Name:      DefaultIssuerName,
			Namespace: c.values.Namespace,
		},
		Spec: certv1alpha1.IssuerSpec{
			ACME: &certv1alpha1.ACMESpec{
				AutoRegistration: c.values.DefaultIssuer.SecretRef == nil,
				Email:            c.values.DefaultIssuer.Email,
				Server:           c.values.DefaultIssuer.Server,
			},
		},
	}
	objects := []client.Object{issuerObj}
	if c.values.DefaultIssuer.SecretRef != nil {
		issuerSecret := &corev1.Secret{}
		if err := c.client.Get(ctx, getObjectKeyLocalObjectRef(*c.values.DefaultIssuer.SecretRef), issuerSecret); err != nil {
			return fmt.Errorf("cannot read secret for issuer %s: %w", DefaultIssuerName, err)
		}
		issuerObj.Spec.ACME.PrivateKeySecretRef = &corev1.SecretReference{
			Name:      issuerSecret.Name,
			Namespace: issuerSecret.Namespace,
		}
	}
	if len(c.values.DefaultIssuer.PrecheckNameservers) > 0 {
		issuerObj.Spec.ACME.PrecheckNameservers = c.values.DefaultIssuer.PrecheckNameservers
	}

	resources, err := registry.AddAllAndSerialize(objects...)
	if err != nil {
		return err
	}

	if err := managedresources.CreateForSeed(ctx, c.client, v1beta1constants.GardenNamespace, issuersManagedResourceName, false, resources); err != nil {
		return fmt.Errorf("creating issuers failed: %w", err)
	}

	return nil
}

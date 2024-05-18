// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package discoveryserver

import (
	"context"
	"fmt"
	"time"

	"github.com/Masterminds/semver/v3"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"

	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	operatorv1alpha1 "github.com/gardener/gardener/pkg/apis/operator/v1alpha1"
	"github.com/gardener/gardener/pkg/component"
	operatorclient "github.com/gardener/gardener/pkg/operator/client"
	"github.com/gardener/gardener/pkg/utils/flow"
	kubernetesutils "github.com/gardener/gardener/pkg/utils/kubernetes"
	"github.com/gardener/gardener/pkg/utils/managedresources"
	secretsutils "github.com/gardener/gardener/pkg/utils/secrets"
	secretsmanager "github.com/gardener/gardener/pkg/utils/secrets/manager"
)

const (
	// ManagedResourceNameRuntime is the name of the ManagedResource for the runtime resources.
	ManagedResourceNameRuntime = "gardener-discovery-server-runtime"
	// ManagedResourceNameVirtual is the name of the ManagedResource for the virtual resources.
	ManagedResourceNameVirtual = "gardener-discovery-server-virtual"

	// DeploymentName is the name of the Gardener Discovery Server deployment.
	DeploymentName = "gardener-discovery-server"
	role           = "discovery-server"

	// TimeoutWaitForManagedResource is the timeout used while waiting for the ManagedResources to become healthy or
	// deleted.
	TimeoutWaitForManagedResource = 5 * time.Minute
)

// Values contains configuration values for the gardener-discovery-server resources.
type Values struct {
	// Image defines the container image of gardener-discovery-server.
	Image string
	// RuntimeVersion is the Kubernetes version of the runtime cluster.
	RuntimeVersion *semver.Version
	// Hostname is the hostname that will be used by the discovery server to serve metadata on.
	Hostname string
	// TLSSecretName is the name of the secret that will be used by the discovery server to handle TLS.
	// If not provided then self-signed certificate will be generated.
	TLSSecretName *string
}

// New creates a new [GardenerDiscoveryServer] capable of deploying gardener-discovery-server.
func New(client client.Client, namespace string, secretsManager secretsmanager.Interface, values Values) *GardenerDiscoveryServer {
	return &GardenerDiscoveryServer{
		client:         client,
		namespace:      namespace,
		secretsManager: secretsManager,
		values:         values,
	}
}

var _ component.DeployWaiter = (*GardenerDiscoveryServer)(nil)

// GardenerDiscoveryServer is capable of deploying the Gardener Discovery Server.
type GardenerDiscoveryServer struct {
	client         client.Client
	namespace      string
	secretsManager secretsmanager.Interface
	values         Values
}

// Deploy deploys the Gardener Discovery Server.
func (g *GardenerDiscoveryServer) Deploy(ctx context.Context) error {
	var (
		runtimeRegistry           = managedresources.NewRegistry(operatorclient.RuntimeScheme, operatorclient.RuntimeCodec, operatorclient.RuntimeSerializer)
		managedResourceLabels     = map[string]string{v1beta1constants.LabelCareConditionType: string(operatorv1alpha1.VirtualComponentsHealthy)}
		virtualGardenAccessSecret = g.newVirtualGardenAccessSecret()
	)

	if err := virtualGardenAccessSecret.Reconcile(ctx, g.client); err != nil {
		return err
	}

	secretGenericTokenKubeconfig, found := g.secretsManager.Get(v1beta1constants.SecretNameGenericTokenKubeconfig)
	if !found {
		return fmt.Errorf("secret %q not found", v1beta1constants.SecretNameGenericTokenKubeconfig)
	}

	tlsSecretName := ptr.Deref(g.values.TLSSecretName, "")
	if tlsSecretName == "" {
		ingressTLSSecret, err := g.secretsManager.Generate(ctx, &secretsutils.CertificateSecretConfig{
			Name:                        DeploymentName + "-tls",
			CommonName:                  DeploymentName,
			DNSNames:                    []string{g.values.Hostname},
			CertType:                    secretsutils.ServerCert,
			Validity:                    ptr.To(v1beta1constants.IngressTLSCertificateValidity),
			SkipPublishingCACertificate: true,
		}, secretsmanager.SignedByCA(operatorv1alpha1.SecretNameCAGardener))
		if err != nil {
			return err
		}
		tlsSecretName = ingressTLSSecret.Name
	}

	runtimeResources, err := runtimeRegistry.AddAllAndSerialize(
		g.deployment(secretGenericTokenKubeconfig.Name, virtualGardenAccessSecret.Secret.Name, tlsSecretName),
		g.service(),
		g.podDisruptionBudget(),
		g.verticalPodAutoscaler(),
		g.ingress(),
		g.serviceMonitor(),
	)
	if err != nil {
		return err
	}

	if err := managedresources.CreateForSeedWithLabels(ctx, g.client, g.namespace, ManagedResourceNameRuntime, false, managedResourceLabels, runtimeResources); err != nil {
		return err
	}

	var (
		virtualRegistry = managedresources.NewRegistry(operatorclient.VirtualScheme, operatorclient.VirtualCodec, operatorclient.VirtualSerializer)
	)

	virtualResources, err := virtualRegistry.AddAllAndSerialize(
		g.clusterRole(),
		g.clusterRoleBinding(virtualGardenAccessSecret.ServiceAccountName),
		g.role(),
		g.roleBinding(virtualGardenAccessSecret.ServiceAccountName),
		g.newServiceAccountIssuerConfigSecret(),
	)
	if err != nil {
		return err
	}

	return managedresources.CreateForShootWithLabels(ctx, g.client, g.namespace, ManagedResourceNameVirtual, managedresources.LabelValueGardener, false, managedResourceLabels, virtualResources)
}

// Wait waits for the Gardener Discovery Server to become healthy.
func (g *GardenerDiscoveryServer) Wait(ctx context.Context) error {
	timeoutCtx, cancel := context.WithTimeout(ctx, TimeoutWaitForManagedResource)
	defer cancel()

	return flow.Parallel(
		func(ctx context.Context) error {
			return managedresources.WaitUntilHealthy(ctx, g.client, g.namespace, ManagedResourceNameRuntime)
		},
		func(ctx context.Context) error {
			return managedresources.WaitUntilHealthy(ctx, g.client, g.namespace, ManagedResourceNameVirtual)
		},
	)(timeoutCtx)
}

// Destroy uninstalls the Gardener Discovery Server.
func (g *GardenerDiscoveryServer) Destroy(ctx context.Context) error {
	if err := managedresources.DeleteForShoot(ctx, g.client, g.namespace, ManagedResourceNameVirtual); err != nil {
		return err
	}

	if err := managedresources.DeleteForSeed(ctx, g.client, g.namespace, ManagedResourceNameRuntime); err != nil {
		return err
	}

	return kubernetesutils.DeleteObjects(ctx, g.client, g.newVirtualGardenAccessSecret().Secret)
}

// WaitCleanup waits for the Gardener Discovery Server resources to be removed.
func (g *GardenerDiscoveryServer) WaitCleanup(ctx context.Context) error {
	timeoutCtx, cancel := context.WithTimeout(ctx, TimeoutWaitForManagedResource)
	defer cancel()

	return flow.Parallel(
		func(ctx context.Context) error {
			return managedresources.WaitUntilDeleted(ctx, g.client, g.namespace, ManagedResourceNameRuntime)
		},
		func(ctx context.Context) error {
			return managedresources.WaitUntilDeleted(ctx, g.client, g.namespace, ManagedResourceNameVirtual)
		},
	)(timeoutCtx)
}

func labels() map[string]string {
	return map[string]string{
		v1beta1constants.LabelApp:  v1beta1constants.LabelGardener,
		v1beta1constants.LabelRole: role,
	}
}

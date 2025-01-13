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
	// managedResourceNameRuntime is the name of the ManagedResource for the runtime resources.
	managedResourceNameRuntime = "gardener-discovery-server-runtime"
	// managedResourceNameVirtual is the name of the ManagedResource for the virtual resources.
	managedResourceNameVirtual = "gardener-discovery-server-virtual"

	// deploymentName is the name of the Gardener Discovery Server deployment.
	deploymentName = "gardener-discovery-server"
	// ServiceName is the name of the service used to expose the Gardener Discovery Server.
	ServiceName = deploymentName

	role = "discovery-server"

	// timeoutWaitForManagedResource is the timeout used while waiting for the ManagedResources
	// to become healthy or deleted.
	timeoutWaitForManagedResource = 5 * time.Minute
)

// Values contains configuration values for the gardener-discovery-server resources.
type Values struct {
	// Image defines the container image of gardener-discovery-server.
	Image string
	// RuntimeVersion is the Kubernetes version of the runtime cluster.
	RuntimeVersion *semver.Version
	// Domain will be used by the discovery server to serve metadata on.
	Domain string
	// TLSSecretName is the name of the secret that will be used by the discovery server to handle TLS.
	// If not provided then self-signed certificate will be generated.
	TLSSecretName *string
	// WorkloadIdentityTokenIssuer is the issuer URL of the workload identity token issuer.
	WorkloadIdentityTokenIssuer string
}

// New creates a new [component.DeployWaiter] capable of deploying gardener-discovery-server.
func New(client client.Client, namespace string, secretsManager secretsmanager.Interface, values Values) component.DeployWaiter {
	return &gardenerDiscoveryServer{
		client:         client,
		namespace:      namespace,
		secretsManager: secretsManager,
		values:         values,
	}
}

var _ component.DeployWaiter = (*gardenerDiscoveryServer)(nil)

// gardenerDiscoveryServer is capable of deploying the Gardener Discovery Server.
type gardenerDiscoveryServer struct {
	client         client.Client
	namespace      string
	secretsManager secretsmanager.Interface
	values         Values
}

func (g *gardenerDiscoveryServer) Deploy(ctx context.Context) error {
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

	secretWorkloadIdentityDiscoveryDocuments, err := g.workloadIdentitySecret()
	if err != nil {
		return fmt.Errorf("failed to get the secret with the workload identity discovery documents: %w", err)
	}

	tlsSecretName := ptr.Deref(g.values.TLSSecretName, "")
	if tlsSecretName == "" {
		ingressTLSSecret, err := g.secretsManager.Generate(ctx, &secretsutils.CertificateSecretConfig{
			Name:                        deploymentName + "-tls",
			CommonName:                  deploymentName,
			DNSNames:                    []string{g.values.Domain},
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
		g.deployment(secretGenericTokenKubeconfig.Name, virtualGardenAccessSecret.Secret.Name, tlsSecretName, secretWorkloadIdentityDiscoveryDocuments.GetName()),
		g.service(),
		g.podDisruptionBudget(),
		g.verticalPodAutoscaler(),
		g.ingress(),
		g.serviceMonitor(),
		secretWorkloadIdentityDiscoveryDocuments,
	)
	if err != nil {
		return err
	}

	if err := managedresources.CreateForSeedWithLabels(ctx, g.client, g.namespace, managedResourceNameRuntime, false, managedResourceLabels, runtimeResources); err != nil {
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

	return managedresources.CreateForShootWithLabels(ctx, g.client, g.namespace, managedResourceNameVirtual, managedresources.LabelValueGardener, false, managedResourceLabels, virtualResources)
}

func (g *gardenerDiscoveryServer) Wait(ctx context.Context) error {
	timeoutCtx, cancel := context.WithTimeout(ctx, timeoutWaitForManagedResource)
	defer cancel()

	return flow.Parallel(
		func(ctx context.Context) error {
			return managedresources.WaitUntilHealthy(ctx, g.client, g.namespace, managedResourceNameRuntime)
		},
		func(ctx context.Context) error {
			return managedresources.WaitUntilHealthy(ctx, g.client, g.namespace, managedResourceNameVirtual)
		},
	)(timeoutCtx)
}

func (g *gardenerDiscoveryServer) Destroy(ctx context.Context) error {
	if err := managedresources.DeleteForShoot(ctx, g.client, g.namespace, managedResourceNameVirtual); err != nil {
		return err
	}

	if err := managedresources.DeleteForSeed(ctx, g.client, g.namespace, managedResourceNameRuntime); err != nil {
		return err
	}

	return kubernetesutils.DeleteObjects(ctx, g.client, g.newVirtualGardenAccessSecret().Secret)
}

func (g *gardenerDiscoveryServer) WaitCleanup(ctx context.Context) error {
	timeoutCtx, cancel := context.WithTimeout(ctx, timeoutWaitForManagedResource)
	defer cancel()

	return flow.Parallel(
		func(ctx context.Context) error {
			return managedresources.WaitUntilDeleted(ctx, g.client, g.namespace, managedResourceNameRuntime)
		},
		func(ctx context.Context) error {
			return managedresources.WaitUntilDeleted(ctx, g.client, g.namespace, managedResourceNameVirtual)
		},
	)(timeoutCtx)
}

func labels() map[string]string {
	return map[string]string{
		v1beta1constants.LabelApp:  v1beta1constants.LabelGardener,
		v1beta1constants.LabelRole: role,
	}
}

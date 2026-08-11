// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package perses

import (
	"context"
	"time"

	persesv1alpha2 "github.com/perses/perses-operator/api/v1alpha2"
	"sigs.k8s.io/controller-runtime/pkg/client"

	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	"github.com/gardener/gardener/pkg/component"
	"github.com/gardener/gardener/pkg/utils/managedresources"
	secretsmanager "github.com/gardener/gardener/pkg/utils/secrets/manager"
)

const (
	managedResourceNamePrefix = "perses"
)

// TimeoutWaitForManagedResource is the timeout used while waiting for the ManagedResources to become healthy
// or deleted.
var TimeoutWaitForManagedResource = 2 * time.Minute

// Interface contains functions for a Perses deployer.
type Interface interface {
	component.DeployWaiter
}

// Values contains configuration values for the Perses resources.
type Values struct {
	// Image is the container image used for Perses.
	Image string
	// ClusterType is the type of the cluster.
	ClusterType component.ClusterType
	// Replicas is the number of replicas.
	Replicas int32
	// VPAEnabled states whether VerticalPodAutoscaler is enabled.
	VPAEnabled bool
	// IsGardenCluster specifies whether the cluster is a garden cluster.
	IsGardenCluster bool
	// ExternalExposure contains configuration for exposing this Perses instance via a VirtualService resource.
	ExternalExposure *ExposureValues
	// VictoriaLogsEnabled indicates whether VictoriaLogs is enabled as the logging backend.
	VictoriaLogsEnabled bool
	// OnlyDeployDatasourcesAndDashboards only leads to deployment of the PersesDatasource and PersesDashboard CRs.
	// This is relevant when the Perses instance is already deployed by another component (e.g., gardener-operator),
	// and the gardenlet wants to contribute seed-specific configuration.
	OnlyDeployDatasourcesAndDashboards bool
	// Dashboards is a map of PersesDashboard CRs to deploy, keyed by name.
	Dashboards map[string]persesv1alpha2.Dashboard
}

// ExposureValues contains configuration for exposing this Perses instance via a VirtualService resource.
type ExposureValues struct {
	// AuthSecretName is the name of the auth secret.
	AuthSecretName string
	// Host is the hostname under which the Perses instance should be exposed.
	Host string
	// IsGardenCluster specifies whether the cluster is the garden cluster.
	IsGardenCluster bool
	// IstioIngressGatewayLabels are the labels identifying the corresponding istio ingress gateway.
	IstioIngressGatewayLabels map[string]string
	// IstioIngressGatewayNamespace is the namespace of the corresponding istio ingress gateway. The Gateway,
	// VirtualService and DestinationRule are exported to this namespace only, and the TLS certificate is copied there.
	IstioIngressGatewayNamespace string
	// SecretsManager is the secrets manager used for generating the TLS certificate if no wildcard certificate is
	// provided.
	SecretsManager secretsmanager.Interface
	// SigningCA is the name of the CA that should be used to sign a self-signed server certificate. Only needed when
	// no wildcard certificate secret is provided.
	SigningCA string
	// WildcardCertSecretName is name of a secret containing the wildcard TLS certificate which is issued for the
	// ingress domain. If not provided, a self-signed server certificate will be created.
	WildcardCertSecretName *string
}

// New creates a new instance of Interface for Perses.
func New(
	client client.Client,
	namespace string,
	values Values,
) Interface {
	return &perses{
		client:    client,
		namespace: namespace,
		values:    values,
	}
}

type perses struct {
	client    client.Client
	namespace string
	values    Values
}

func (p *perses) Deploy(ctx context.Context) error {
	registry := managedresources.NewRegistry(kubernetes.SeedScheme, kubernetes.SeedCodec, kubernetes.SeedSerializer)

	objs := []client.Object{
		p.perses(),
		p.serviceMonitor(),
	}

	resources, err := registry.AddAllAndSerialize(objs...)
	if err != nil {
		return err
	}

	return managedresources.CreateForSeedWithLabels(ctx, p.client, p.namespace, p.managedResourceName(), false, map[string]string{v1beta1constants.LabelCareConditionType: v1beta1constants.ObservabilityComponentsHealthy}, resources)
}

func (p *perses) Destroy(ctx context.Context) error {
	return managedresources.DeleteForSeed(ctx, p.client, p.namespace, p.managedResourceName())
}

func (p *perses) Wait(ctx context.Context) error {
	timeoutCtx, cancel := context.WithTimeout(ctx, TimeoutWaitForManagedResource)
	defer cancel()

	return managedresources.WaitUntilHealthy(timeoutCtx, p.client, p.namespace, p.managedResourceName())
}

func (p *perses) WaitCleanup(ctx context.Context) error {
	timeoutCtx, cancel := context.WithTimeout(ctx, TimeoutWaitForManagedResource)
	defer cancel()

	return managedresources.WaitUntilDeleted(timeoutCtx, p.client, p.namespace, p.managedResourceName())
}

func (p *perses) managedResourceName() string {
	if p.values.OnlyDeployDatasourcesAndDashboards {
		return managedResourceNamePrefix + "-seed-config-only"
	}
	return managedResourceNamePrefix
}

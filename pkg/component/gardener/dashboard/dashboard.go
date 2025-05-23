// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package dashboard

import (
	"context"
	"fmt"
	"time"

	"sigs.k8s.io/controller-runtime/pkg/client"

	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	operatorv1alpha1 "github.com/gardener/gardener/pkg/apis/operator/v1alpha1"
	"github.com/gardener/gardener/pkg/component"
	operatorclient "github.com/gardener/gardener/pkg/operator/client"
	"github.com/gardener/gardener/pkg/utils/flow"
	kubernetesutils "github.com/gardener/gardener/pkg/utils/kubernetes"
	"github.com/gardener/gardener/pkg/utils/managedresources"
	secretsmanager "github.com/gardener/gardener/pkg/utils/secrets/manager"
)

const (
	// ManagedResourceNameRuntime is the name of the ManagedResource for the runtime resources.
	ManagedResourceNameRuntime = "gardener-dashboard-runtime"
	// ManagedResourceNameVirtual is the name of the ManagedResource for the virtual resources.
	ManagedResourceNameVirtual = "gardener-dashboard-virtual"

	deploymentName = "gardener-dashboard"
	roleName       = "dashboard"
)

// TimeoutWaitForManagedResource is the timeout used while waiting for the ManagedResources to become healthy or
// deleted.
var TimeoutWaitForManagedResource = 5 * time.Minute

// Values contains configuration values for the gardener-dashboard resources.
type Values struct {
	// Image defines the container image of gardener-dashboard.
	Image string
	// LogLevel is the level/severity for the logs.
	LogLevel string
	// APIServerURL is the URL of the API server of the virtual garden cluster.
	APIServerURL string
	// APIServerCABundle is the CA bundle of the API server of the virtual garden cluster.
	APIServerCABundle *string
	// EnableTokenLogin specifies whether token-based login is enabled.
	EnableTokenLogin bool
	// Ingress contains the ingress configuration.
	Ingress IngressValues
	// Terminal contains the terminal configuration.
	Terminal *TerminalValues
	// OIDC is the configuration for the OIDC settings.
	OIDC *OIDCValues
	// GitHub is the configuration for the GitHub settings.
	GitHub *operatorv1alpha1.DashboardGitHub
	// FrontendConfigMapName is the name of the ConfigMap containing the frontend configuration.
	FrontendConfigMapName *string
	// AssetsConfigMapName is the name of the ConfigMap containing the assets.
	AssetsConfigMapName *string
}

// TerminalValues contains the terminal configuration.
type TerminalValues struct {
	operatorv1alpha1.DashboardTerminal
	// GardenTerminalSeedHost is the name of a seed hosting the garden terminals.
	GardenTerminalSeedHost string
}

// OIDCValues contains the OIDC configuration.
type OIDCValues struct {
	operatorv1alpha1.DashboardOIDC
	// IssuerURL is the issuer URL.
	IssuerURL string
	// ClientIDPublic is the public client ID.
	ClientIDPublic string
}

// IngressValues contains the Ingress configuration.
type IngressValues struct {
	// Enabled specifies if the ingress resource should be deployed.
	Enabled bool
	// Domains is the list of ingress domains.
	Domains []string
	// WildcardCertSecretName is name of a secret containing the wildcard TLS certificate which is issued for the
	// ingress domains. If not provided, a self-signed server certificate will be created.
	WildcardCertSecretName *string
}

// Interface contains function for deploying the gardener-dashboard.
type Interface interface {
	component.DeployWaiter
	// SetGardenTerminalSeedHost sets the terminal seed host field.
	SetGardenTerminalSeedHost(string)
	// SetAPIServerCABundle sets the API server CA bundle field.
	SetAPIServerCABundle(*string)
}

// New creates a new instance of DeployWaiter for the gardener-dashboard.
func New(client client.Client, namespace string, secretsManager secretsmanager.Interface, values Values) Interface {
	return &gardenerDashboard{
		client:         client,
		namespace:      namespace,
		secretsManager: secretsManager,
		values:         values,
	}
}

type gardenerDashboard struct {
	client         client.Client
	namespace      string
	secretsManager secretsmanager.Interface
	values         Values
}

func (g *gardenerDashboard) Deploy(ctx context.Context) error {
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

	sessionSecret, err := g.reconcileSessionSecret(ctx)
	if err != nil {
		return err
	}

	sessionSecretPrevious, _ := g.secretsManager.Get("gardener-dashboard-session-secret", secretsmanager.Old)

	configMap, err := g.configMap(ctx)
	if err != nil {
		return err
	}

	deployment, err := g.deployment(ctx, secretGenericTokenKubeconfig.Name, virtualGardenAccessSecret.Secret.Name, sessionSecret.Name, sessionSecretPrevious, configMap.Name)
	if err != nil {
		return err
	}

	if g.values.Ingress.Enabled {
		ingress, err := g.ingress(ctx)
		if err != nil {
			return err
		}

		if err := runtimeRegistry.Add(ingress); err != nil {
			return err
		}
	}

	runtimeResources, err := runtimeRegistry.AddAllAndSerialize(
		configMap,
		deployment,
		g.service(),
		g.podDisruptionBudget(),
		g.verticalPodAutoscaler(),
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

	if g.values.GitHub != nil {
		if err := virtualRegistry.Add(
			g.role(),
			g.roleBinding(virtualGardenAccessSecret.ServiceAccountName),
		); err != nil {
			return err
		}
	}

	virtualResources, err := virtualRegistry.AddAllAndSerialize(
		g.clusterRole(),
		g.clusterRoleBinding(virtualGardenAccessSecret.ServiceAccountName),
		g.serviceAccountTerminal(),
		g.clusterRoleBindingTerminal(),
		g.clusterRoleTerminalProjectMember(),
	)
	if err != nil {
		return err
	}

	return managedresources.CreateForShootWithLabels(ctx, g.client, g.namespace, ManagedResourceNameVirtual, managedresources.LabelValueGardener, false, managedResourceLabels, virtualResources)
}

func (g *gardenerDashboard) Wait(ctx context.Context) error {
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

func (g *gardenerDashboard) Destroy(ctx context.Context) error {
	if err := managedresources.DeleteForShoot(ctx, g.client, g.namespace, ManagedResourceNameVirtual); err != nil {
		return err
	}

	if err := managedresources.DeleteForSeed(ctx, g.client, g.namespace, ManagedResourceNameRuntime); err != nil {
		return err
	}

	return kubernetesutils.DeleteObjects(ctx, g.client, g.newVirtualGardenAccessSecret().Secret)
}

func (g *gardenerDashboard) WaitCleanup(ctx context.Context) error {
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

func (g *gardenerDashboard) SetGardenTerminalSeedHost(host string) {
	if g.values.Terminal != nil {
		g.values.Terminal.GardenTerminalSeedHost = host
	}
}

func (g *gardenerDashboard) SetAPIServerCABundle(bundle *string) {
	g.values.APIServerCABundle = bundle
}

// GetLabels returns the labels for the gardener-dashboard.
func GetLabels() map[string]string {
	return map[string]string{
		v1beta1constants.LabelApp:  v1beta1constants.LabelGardener,
		v1beta1constants.LabelRole: roleName,
	}
}

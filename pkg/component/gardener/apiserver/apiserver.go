// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package apiserver

import (
	"context"
	"fmt"
	"time"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	gardencorev1 "github.com/gardener/gardener/pkg/apis/core/v1"
	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	operationsv1alpha1 "github.com/gardener/gardener/pkg/apis/operations/v1alpha1"
	operatorv1alpha1 "github.com/gardener/gardener/pkg/apis/operator/v1alpha1"
	securityv1alpha1 "github.com/gardener/gardener/pkg/apis/security/v1alpha1"
	seedmanagementv1alpha1 "github.com/gardener/gardener/pkg/apis/seedmanagement/v1alpha1"
	settingsv1alpha1 "github.com/gardener/gardener/pkg/apis/settings/v1alpha1"
	"github.com/gardener/gardener/pkg/component/apiserver"
	"github.com/gardener/gardener/pkg/component/etcd/etcd"
	operatorclient "github.com/gardener/gardener/pkg/operator/client"
	kubernetesutils "github.com/gardener/gardener/pkg/utils/kubernetes"
	"github.com/gardener/gardener/pkg/utils/managedresources"
	"github.com/gardener/gardener/pkg/utils/retry"
	secretsmanager "github.com/gardener/gardener/pkg/utils/secrets/manager"
)

const (
	// DeploymentName is the name of the deployment.
	DeploymentName = "gardener-apiserver"

	// ManagedResourceNameRuntime is the name of the ManagedResource for the runtime resources.
	ManagedResourceNameRuntime = "gardener-apiserver-runtime"
	// ManagedResourceNameVirtual is the name of the ManagedResource for the virtual resources.
	ManagedResourceNameVirtual = "gardener-apiserver-virtual"
)

// TimeoutWaitForManagedResource is the timeout used while waiting for the ManagedResources to become healthy or
// deleted.
var TimeoutWaitForManagedResource = 5 * time.Minute

const (
	// intervalWaitServiceRuntime is the interval when waiting for
	// gardener-apiserver service to get created by gardener-resource-manager
	intervalWaitServiceRuntime = 5 * time.Second
	// timeoutWaitServiceRuntime is the timeout when waiting for
	// gardener-apiserver service to get created by gardener-resource-manager
	timeoutWaitServiceRuntime = 30 * time.Second
)

// Interface contains functions for a gardener-apiserver deployer.
type Interface interface {
	apiserver.Interface
	// GetValues returns the current configuration values of the deployer.
	GetValues() Values
	// SetWorkloadIdentityKeyRotationPhase sets the current workload identity key rotation phase.
	SetWorkloadIdentityKeyRotationPhase(gardencorev1beta1.CredentialsRotationPhase)
}

// Values contains configuration values for the gardener-apiserver resources.
type Values struct {
	apiserver.Values
	// Autoscaling contains information for configuring autoscaling settings for the API server.
	Autoscaling AutoscalingConfig
	// ClusterIdentity is the identity of the garden cluster.
	ClusterIdentity string
	// Image is the container image used for the gardener-apiserver pods.
	Image string
	// LogLevel is the level/severity for the logs. Must be one of [info,debug,error].
	LogLevel string
	// LogFormat is the output format for the logs. Must be one of [text,json].
	LogFormat string
	// GoAwayChance can be used to prevent HTTP/2 clients from getting stuck on a single apiserver, randomly close a
	// connection (GOAWAY).
	GoAwayChance *float64
	// TopologyAwareRoutingEnabled specifies where the topology-aware feature is enabled.
	TopologyAwareRoutingEnabled bool
	// WorkloadIdentityTokenIssuer is the issuer identifier of the workload identity tokens set in the 'iss' claim.
	WorkloadIdentityTokenIssuer string
	// WorkloadIdentityKeyRotationPhase is the rotation phase of workload identity key.
	WorkloadIdentityKeyRotationPhase gardencorev1beta1.CredentialsRotationPhase
	// ShootAdminKubeconfigMaxExpiration is the maximum expiration time of the admin kubeconfig.
	ShootAdminKubeconfigMaxExpiration *metav1.Duration
}

// AutoscalingConfig contains information for configuring autoscaling settings for the API server.
type AutoscalingConfig struct {
	// APIServerResources are the resource requirements for the API server container.
	APIServerResources corev1.ResourceRequirements
	// Replicas is the number of pod replicas for the API server.
	Replicas *int32
}

// New creates a new instance of DeployWaiter for the gardener-apiserver.
func New(client client.Client, namespace string, secretsManager secretsmanager.Interface, values Values) Interface {
	return &gardenerAPIServer{
		client:         client,
		namespace:      namespace,
		secretsManager: secretsManager,
		values:         values,
	}
}

type gardenerAPIServer struct {
	client         client.Client
	namespace      string
	secretsManager secretsmanager.Interface
	values         Values
}

func (g *gardenerAPIServer) Deploy(ctx context.Context) error {
	var (
		runtimeRegistry = managedresources.NewRegistry(operatorclient.RuntimeScheme, operatorclient.RuntimeCodec, operatorclient.RuntimeSerializer)
		virtualRegistry = managedresources.NewRegistry(operatorclient.VirtualScheme, operatorclient.VirtualCodec, operatorclient.VirtualSerializer)

		managedResourceLabels = map[string]string{v1beta1constants.LabelCareConditionType: string(operatorv1alpha1.VirtualComponentsHealthy)}

		configMapAuditPolicy              = g.emptyConfigMap(configMapAuditPolicyNamePrefix)
		configMapAdmissionConfigs         = g.emptyConfigMap(configMapAdmissionNamePrefix)
		secretAdmissionKubeconfigs        = g.emptySecret(secretAdmissionKubeconfigsNamePrefix)
		secretETCDEncryptionConfiguration = g.emptySecret(v1beta1constants.SecretNamePrefixGardenerETCDEncryptionConfiguration)
		secretAuditWebhookKubeconfig      = g.emptySecret(secretAuditWebhookKubeconfigNamePrefix)
		secretVirtualGardenAccess         = g.newVirtualGardenAccessSecret()
	)

	secretServer, err := g.reconcileSecretServer(ctx)
	if err != nil {
		return err
	}

	secretWorkloadIdentityKey, err := g.reconcileWorkloadIdentityKey(ctx)
	if err != nil {
		return err
	}

	if err := secretVirtualGardenAccess.Reconcile(ctx, g.client); err != nil {
		return err
	}

	if err := g.reconcileSecretETCDEncryptionConfiguration(ctx, secretETCDEncryptionConfiguration); err != nil {
		return err
	}

	if err := apiserver.ReconcileConfigMapAdmission(ctx, g.client, configMapAdmissionConfigs, g.values.Values); err != nil {
		return err
	}
	if err := apiserver.ReconcileSecretAdmissionKubeconfigs(ctx, g.client, secretAdmissionKubeconfigs, g.values.Values); err != nil {
		return err
	}

	if err := apiserver.ReconcileConfigMapAuditPolicy(ctx, g.client, configMapAuditPolicy, g.values.Audit); err != nil {
		return err
	}
	if err := apiserver.ReconcileSecretAuditWebhookKubeconfig(ctx, g.client, secretAuditWebhookKubeconfig, g.values.Audit); err != nil {
		return err
	}

	secretCAGardener, found := g.secretsManager.Get(operatorv1alpha1.SecretNameCAGardener)
	if !found {
		return fmt.Errorf("secret %q not found", operatorv1alpha1.SecretNameCAGardener)
	}

	secretCAETCD, found := g.secretsManager.Get(v1beta1constants.SecretNameCAETCD)
	if !found {
		return fmt.Errorf("secret %q not found", v1beta1constants.SecretNameCAETCD)
	}

	secretETCDClient, found := g.secretsManager.Get(etcd.SecretNameClient)
	if !found {
		return fmt.Errorf("secret %q not found", etcd.SecretNameClient)
	}

	secretGenericTokenKubeconfig, found := g.secretsManager.Get(v1beta1constants.SecretNameGenericTokenKubeconfig)
	if !found {
		return fmt.Errorf("secret %q not found", v1beta1constants.SecretNameGenericTokenKubeconfig)
	}

	runtimeResources, err := runtimeRegistry.AddAllAndSerialize(
		g.podDisruptionBudget(),
		g.serviceRuntime(),
		g.verticalPodAutoscaler(),
		g.deployment(secretCAETCD, secretETCDClient, secretGenericTokenKubeconfig, secretServer, secretAdmissionKubeconfigs, secretETCDEncryptionConfiguration, secretAuditWebhookKubeconfig, secretWorkloadIdentityKey, secretVirtualGardenAccess, configMapAuditPolicy, configMapAdmissionConfigs),
		g.serviceMonitor(),
	)
	if err != nil {
		return err
	}

	if err := managedresources.CreateForSeedWithLabels(ctx, g.client, g.namespace, ManagedResourceNameRuntime, false, managedResourceLabels, runtimeResources); err != nil {
		return err
	}

	serviceRuntime := &corev1.Service{ObjectMeta: metav1.ObjectMeta{Name: serviceName, Namespace: g.namespace}}
	if err := retry.UntilTimeout(ctx, intervalWaitServiceRuntime, timeoutWaitServiceRuntime, func(ctx context.Context) (bool, error) {
		if err := g.client.Get(ctx, client.ObjectKeyFromObject(serviceRuntime), serviceRuntime); err != nil {
			if apierrors.IsNotFound(err) {
				return retry.MinorError(fmt.Errorf("service %s was not yet created", client.ObjectKeyFromObject(serviceRuntime)))
			}
			return retry.SevereError(fmt.Errorf("unexpected error while retrieving service %s: %w", client.ObjectKeyFromObject(serviceRuntime), err))
		}
		return retry.Ok()
	}); err != nil {
		return fmt.Errorf("failed waiting for service %s to get created by gardener-resource-manager: %w", client.ObjectKeyFromObject(serviceRuntime), err)
	}

	virtualResources, err := virtualRegistry.AddAllAndSerialize(
		g.apiService(secretCAGardener, gardencorev1.SchemeGroupVersion.Group, gardencorev1.SchemeGroupVersion.Version),
		g.apiService(secretCAGardener, gardencorev1beta1.SchemeGroupVersion.Group, gardencorev1beta1.SchemeGroupVersion.Version),
		g.apiService(secretCAGardener, seedmanagementv1alpha1.SchemeGroupVersion.Group, seedmanagementv1alpha1.SchemeGroupVersion.Version),
		g.apiService(secretCAGardener, operationsv1alpha1.SchemeGroupVersion.Group, operationsv1alpha1.SchemeGroupVersion.Version),
		g.apiService(secretCAGardener, settingsv1alpha1.SchemeGroupVersion.Group, settingsv1alpha1.SchemeGroupVersion.Version),
		g.apiService(secretCAGardener, securityv1alpha1.SchemeGroupVersion.Group, securityv1alpha1.SchemeGroupVersion.Version),
		g.service(),
		g.endpoints(serviceRuntime.Spec.ClusterIP),
		g.clusterRole(),
		g.clusterRoleBinding(secretVirtualGardenAccess.ServiceAccountName),
		g.clusterRoleBindingAuthDelegation(secretVirtualGardenAccess.ServiceAccountName),
		g.roleBindingAuthReader(secretVirtualGardenAccess.ServiceAccountName),
	)
	if err != nil {
		return err
	}

	return managedresources.CreateForShootWithLabels(ctx, g.client, g.namespace, ManagedResourceNameVirtual, managedresources.LabelValueGardener, false, managedResourceLabels, virtualResources)
}

func (g *gardenerAPIServer) Destroy(ctx context.Context) error {
	if err := managedresources.DeleteForShoot(ctx, g.client, g.namespace, ManagedResourceNameVirtual); err != nil {
		return err
	}
	if err := managedresources.DeleteForSeed(ctx, g.client, g.namespace, ManagedResourceNameRuntime); err != nil {
		return err
	}
	return kubernetesutils.DeleteObjects(ctx, g.client, g.newVirtualGardenAccessSecret().Secret)
}

func (g *gardenerAPIServer) Wait(ctx context.Context) error {
	timeoutCtx, cancel := context.WithTimeout(ctx, TimeoutWaitForManagedResource)
	defer cancel()

	// Typically, we use managedresources.WaitUntilHealthy by default everywhere/in all components.
	// However, here we have to wait for the runtime resources to no longer be progressing before we can update the
	// virtual resources. This is important for credentials rotation since we want all GAPI pods to run with the new
	// server certificate before we drop the old CA from the bundle in the APIServices (which get deployed via the
	// virtual resources).
	if err := managedresources.WaitUntilHealthyAndNotProgressing(timeoutCtx, g.client, g.namespace, ManagedResourceNameRuntime); err != nil {
		return err
	}

	timeoutCtx, cancel = context.WithTimeout(ctx, TimeoutWaitForManagedResource)
	defer cancel()

	return managedresources.WaitUntilHealthy(timeoutCtx, g.client, g.namespace, ManagedResourceNameVirtual)
}

func (g *gardenerAPIServer) WaitCleanup(ctx context.Context) error {
	timeoutCtx, cancel := context.WithTimeout(ctx, TimeoutWaitForManagedResource)
	defer cancel()

	if err := managedresources.WaitUntilDeleted(timeoutCtx, g.client, g.namespace, ManagedResourceNameVirtual); err != nil {
		return err
	}

	timeoutCtx, cancel = context.WithTimeout(ctx, TimeoutWaitForManagedResource)
	defer cancel()

	return managedresources.WaitUntilDeleted(timeoutCtx, g.client, g.namespace, ManagedResourceNameRuntime)
}

func (g *gardenerAPIServer) GetValues() Values {
	return g.values
}

func (g *gardenerAPIServer) GetAutoscalingReplicas() *int32 {
	return g.values.Autoscaling.Replicas
}

func (g *gardenerAPIServer) SetAutoscalingAPIServerResources(resources corev1.ResourceRequirements) {
	g.values.Autoscaling.APIServerResources = resources
}

func (g *gardenerAPIServer) SetAutoscalingReplicas(replicas *int32) {
	g.values.Autoscaling.Replicas = replicas
}

func (g *gardenerAPIServer) SetETCDEncryptionConfig(config apiserver.ETCDEncryptionConfig) {
	g.values.ETCDEncryption = config
}

func (g *gardenerAPIServer) SetWorkloadIdentityKeyRotationPhase(phase gardencorev1beta1.CredentialsRotationPhase) {
	g.values.WorkloadIdentityKeyRotationPhase = phase
}

// GetLabels returns the labels for the gardener-apiserver.
func GetLabels() map[string]string {
	return map[string]string{
		v1beta1constants.LabelApp:  v1beta1constants.LabelGardener,
		v1beta1constants.LabelRole: v1beta1constants.LabelAPIServer,
	}
}

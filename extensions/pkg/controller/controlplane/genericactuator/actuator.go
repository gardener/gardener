// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package genericactuator

import (
	"context"
	"fmt"
	"sync/atomic"
	"time"

	"github.com/go-logr/logr"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/clock"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/manager"

	extensionscontroller "github.com/gardener/gardener/extensions/pkg/controller"
	"github.com/gardener/gardener/extensions/pkg/controller/controlplane"
	extensionssecretsmanager "github.com/gardener/gardener/extensions/pkg/util/secret/manager"
	"github.com/gardener/gardener/extensions/pkg/webhook"
	extensionsshootwebhook "github.com/gardener/gardener/extensions/pkg/webhook/shoot"
	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	v1beta1helper "github.com/gardener/gardener/pkg/apis/core/v1beta1/helper"
	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
	kubernetesclient "github.com/gardener/gardener/pkg/client/kubernetes"
	"github.com/gardener/gardener/pkg/utils/chart"
	gardenerutils "github.com/gardener/gardener/pkg/utils/gardener"
	"github.com/gardener/gardener/pkg/utils/imagevector"
	kubernetesutils "github.com/gardener/gardener/pkg/utils/kubernetes"
	"github.com/gardener/gardener/pkg/utils/managedresources"
	secretsmanager "github.com/gardener/gardener/pkg/utils/secrets/manager"
)

// ValuesProvider provides values for the 2 charts applied by this actuator.
type ValuesProvider interface {
	// GetConfigChartValues returns the values for the config chart applied by this actuator.
	GetConfigChartValues(ctx context.Context, cp *extensionsv1alpha1.ControlPlane, cluster *extensionscontroller.Cluster) (map[string]any, error)
	// GetControlPlaneChartValues returns the values for the control plane chart applied by this actuator.
	GetControlPlaneChartValues(ctx context.Context, cp *extensionsv1alpha1.ControlPlane, cluster *extensionscontroller.Cluster, secretsReader secretsmanager.Reader, checksums map[string]string, scaledDown bool) (map[string]any, error)
	// GetControlPlaneShootChartValues returns the values for the control plane shoot chart applied by this actuator.
	GetControlPlaneShootChartValues(ctx context.Context, cp *extensionsv1alpha1.ControlPlane, cluster *extensionscontroller.Cluster, secretsReader secretsmanager.Reader, checksums map[string]string) (map[string]any, error)
	// GetControlPlaneShootCRDsChartValues returns the values for the control plane shoot CRDs chart applied by this actuator.
	GetControlPlaneShootCRDsChartValues(ctx context.Context, cp *extensionsv1alpha1.ControlPlane, cluster *extensionscontroller.Cluster) (map[string]any, error)
	// GetStorageClassesChartValues returns the values for the storage classes chart applied by this actuator.
	GetStorageClassesChartValues(ctx context.Context, cp *extensionsv1alpha1.ControlPlane, cluster *extensionscontroller.Cluster) (map[string]any, error)
	// GetControlPlaneExposureChartValues returns the values for the control plane exposure chart applied by this actuator.
	//
	// Deprecated: Control plane with purpose `exposure` is being deprecated and will be removed in gardener v1.123.0.
	// TODO(theoddora): Remove this function in v1.123.0 when the Purpose field is removed.
	GetControlPlaneExposureChartValues(ctx context.Context, cp *extensionsv1alpha1.ControlPlane, cluster *extensionscontroller.Cluster, secretsReader secretsmanager.Reader, checksums map[string]string) (map[string]any, error)
}

// NewActuator creates a new Actuator that acts upon and updates the status of ControlPlane resources.
// It creates / deletes the given secrets and applies / deletes the given charts, using the given image vector and
// the values provided by the given values provider.
func NewActuator(
	mgr manager.Manager,
	providerName string,
	secretConfigs func(namespace string) []extensionssecretsmanager.SecretConfigWithOptions, shootAccessSecrets func(namespace string) []*gardenerutils.AccessSecret,
	exposureSecretConfigs func(namespace string) []extensionssecretsmanager.SecretConfigWithOptions, exposureShootAccessSecrets func(namespace string) []*gardenerutils.AccessSecret,
	configChart, controlPlaneChart, controlPlaneShootChart, controlPlaneShootCRDsChart, storageClassesChart, controlPlaneExposureChart chart.Interface,
	vp ValuesProvider,
	chartRendererFactory extensionscontroller.ChartRendererFactory,
	imageVector imagevector.ImageVector,
	configName string,
	atomicShootWebhookConfig *atomic.Value,
	webhookServerNamespace string,
) (
	controlplane.Actuator,
	error,
) {
	gardenerClientset, err := kubernetesclient.NewWithConfig(kubernetesclient.WithRESTConfig(mgr.GetConfig()))
	if err != nil {
		return nil, err
	}

	return &actuator{
		providerName: providerName,

		secretConfigsFunc:      secretConfigs,
		shootAccessSecretsFunc: shootAccessSecrets,

		exposureSecretConfigsFunc:      exposureSecretConfigs,
		exposureShootAccessSecretsFunc: exposureShootAccessSecrets,

		configChart:                configChart,
		controlPlaneChart:          controlPlaneChart,
		controlPlaneShootChart:     controlPlaneShootChart,
		controlPlaneShootCRDsChart: controlPlaneShootCRDsChart,
		storageClassesChart:        storageClassesChart,
		controlPlaneExposureChart:  controlPlaneExposureChart,
		vp:                         vp,
		chartRendererFactory:       chartRendererFactory,
		imageVector:                imageVector,
		configName:                 configName,
		atomicShootWebhookConfig:   atomicShootWebhookConfig,
		webhookServerNamespace:     webhookServerNamespace,

		gardenerClientset: gardenerClientset,
		client:            mgr.GetClient(),

		newSecretsManager: extensionssecretsmanager.SecretsManagerForCluster,
	}, nil
}

type newSecretsManagerFunc func(context.Context, logr.Logger, clock.Clock, client.Client, *extensionscontroller.Cluster, string, []extensionssecretsmanager.SecretConfigWithOptions) (secretsmanager.Interface, error)

// actuator is an Actuator that acts upon and updates the status of ControlPlane resources.
type actuator struct {
	providerName string

	secretConfigsFunc      func(namespace string) []extensionssecretsmanager.SecretConfigWithOptions
	shootAccessSecretsFunc func(namespace string) []*gardenerutils.AccessSecret

	exposureSecretConfigsFunc      func(namespace string) []extensionssecretsmanager.SecretConfigWithOptions
	exposureShootAccessSecretsFunc func(namespace string) []*gardenerutils.AccessSecret

	configChart                chart.Interface
	controlPlaneChart          chart.Interface
	controlPlaneShootChart     chart.Interface
	controlPlaneShootCRDsChart chart.Interface
	storageClassesChart        chart.Interface
	controlPlaneExposureChart  chart.Interface
	vp                         ValuesProvider
	chartRendererFactory       extensionscontroller.ChartRendererFactory
	imageVector                imagevector.ImageVector
	configName                 string
	atomicShootWebhookConfig   *atomic.Value
	webhookServerNamespace     string

	gardenerClientset kubernetesclient.Interface
	client            client.Client

	newSecretsManager newSecretsManagerFunc
}

const (
	// ControlPlaneShootChartResourceName is the name of the managed resource for the control plane
	ControlPlaneShootChartResourceName = "extension-controlplane-shoot"
	// ControlPlaneShootCRDsChartResourceName is the name of the managed resource for the extension control plane shoot CRDs
	ControlPlaneShootCRDsChartResourceName = "extension-controlplane-shoot-crds"
	// StorageClassesChartResourceName is the name of the managed resource for the extension control plane storageclasses
	StorageClassesChartResourceName = "extension-controlplane-storageclasses"
	// ShootWebhooksResourceName is the name of the managed resource for the extension control plane webhooks
	ShootWebhooksResourceName = "extension-controlplane-shoot-webhooks"
)

// ShootWebhookNamespaceSelector returns a namespace selector for shoot webhooks relevant to provider extensions.
func ShootWebhookNamespaceSelector(providerType string) map[string]string {
	return map[string]string{v1beta1constants.LabelShootProvider: providerType}
}

// Reconcile reconciles the given controlplane and cluster, creating or updating the additional Shoot
// control plane components as needed.
func (a *actuator) Reconcile(
	ctx context.Context,
	log logr.Logger,
	cp *extensionsv1alpha1.ControlPlane,
	cluster *extensionscontroller.Cluster,
) (bool, error) {
	if cp.Spec.Purpose != nil && *cp.Spec.Purpose == extensionsv1alpha1.Exposure {
		return a.reconcileControlPlaneExposure(ctx, log, cp, cluster)
	}
	return a.reconcileControlPlane(ctx, log, cp, cluster)
}

func (a *actuator) reconcileControlPlaneExposure(
	ctx context.Context,
	log logr.Logger,
	cp *extensionsv1alpha1.ControlPlane,
	cluster *extensionscontroller.Cluster,
) (bool, error) {
	if a.controlPlaneExposureChart == nil {
		return false, nil
	}

	var secretConfigs []extensionssecretsmanager.SecretConfigWithOptions
	if a.exposureSecretConfigsFunc != nil {
		secretConfigs = a.exposureSecretConfigsFunc(cp.Namespace)
	}

	sm, err := a.newSecretsManagerForControlPlane(ctx, log, cp, cluster, secretConfigs)
	if err != nil {
		return false, fmt.Errorf("failed to create secrets manager for ControlPlane: %w", err)
	}

	// Deploy secrets managed by secretsmanager
	log.Info("Deploying control plane exposure secrets")
	deployedSecrets, err := extensionssecretsmanager.GenerateAllSecrets(ctx, sm, secretConfigs)
	if err != nil {
		return false, fmt.Errorf("could not deploy control plane exposure secrets for controlplane '%s': %w", client.ObjectKeyFromObject(cp), err)
	}

	// Deploy shoot access secrets
	if a.exposureShootAccessSecretsFunc != nil {
		for _, shootAccessSecret := range a.exposureShootAccessSecretsFunc(cp.Namespace) {
			if err := shootAccessSecret.Reconcile(ctx, a.client); err != nil {
				return false, fmt.Errorf("could not reconcile control plane exposure shoot access secret '%s' for controlplane '%s': %w", shootAccessSecret.Secret.Name, client.ObjectKeyFromObject(cp), err)
			}
		}
	}

	// Compute all needed checksums
	checksums := controlplane.ComputeChecksums(deployedSecrets, nil)

	// Get control plane exposure chart values
	values, err := a.vp.GetControlPlaneExposureChartValues(ctx, cp, cluster, sm, checksums)
	if err != nil {
		return false, err
	}

	// Apply control plane exposure chart
	log.Info("Applying control plane exposure chart", "values", values)
	version := cluster.Shoot.Spec.Kubernetes.Version
	if err := a.controlPlaneExposureChart.Apply(ctx, a.gardenerClientset.ChartApplier(), cp.Namespace, a.imageVector, a.gardenerClientset.Version(), version, values); err != nil {
		return false, fmt.Errorf("could not apply control plane exposure chart for controlplane '%s': %w", client.ObjectKeyFromObject(cp), err)
	}

	return false, sm.Cleanup(ctx)
}

// reconcileControlPlane reconciles the given controlplane and cluster, creating or updating the additional Shoot
// control plane components as needed.
func (a *actuator) reconcileControlPlane(
	ctx context.Context,
	log logr.Logger,
	cp *extensionsv1alpha1.ControlPlane,
	cluster *extensionscontroller.Cluster,
) (
	bool,
	error,
) {
	if a.hasShootWebhooks(cluster.Shoot) {
		value := a.atomicShootWebhookConfig.Load()
		webhookConfig, ok := value.(*webhook.Configs)
		if !ok {
			return false, fmt.Errorf("expected *webhook.Configs, got %T", value)
		}

		if err := extensionsshootwebhook.ReconcileWebhookConfig(ctx, a.client, cp.Namespace, ShootWebhooksResourceName, *webhookConfig, cluster, true); err != nil {
			return false, fmt.Errorf("could not reconcile shoot webhooks: %w", err)
		}
	}

	var secretConfigs []extensionssecretsmanager.SecretConfigWithOptions
	if a.secretConfigsFunc != nil {
		secretConfigs = a.secretConfigsFunc(cp.Namespace)
	}

	sm, err := a.newSecretsManagerForControlPlane(ctx, log, cp, cluster, secretConfigs)
	if err != nil {
		return false, fmt.Errorf("failed to create secrets manager for ControlPlane: %w", err)
	}

	// Deploy secrets managed by secretsmanager
	log.Info("Deploying secrets")
	deployedSecrets, err := extensionssecretsmanager.GenerateAllSecrets(ctx, sm, secretConfigs)
	if err != nil {
		return false, fmt.Errorf("could not deploy secrets for controlplane '%s': %w", client.ObjectKeyFromObject(cp), err)
	}

	// Deploy shoot access secrets
	if a.shootAccessSecretsFunc != nil {
		for _, shootAccessSecret := range a.shootAccessSecretsFunc(cp.Namespace) {
			if err := shootAccessSecret.Reconcile(ctx, a.client); err != nil {
				return false, fmt.Errorf("could not reconcile shoot access secret '%s' for controlplane '%s': %w", shootAccessSecret.Secret.Name, client.ObjectKeyFromObject(cp), err)
			}
		}
	}

	// Get config chart values
	if a.configChart != nil {
		values, err := a.vp.GetConfigChartValues(ctx, cp, cluster)
		if err != nil {
			return false, err
		}

		// Apply config chart
		log.Info("Applying configuration chart")
		if err := a.configChart.Apply(ctx, a.gardenerClientset.ChartApplier(), cp.Namespace, nil, "", "", values); err != nil {
			return false, fmt.Errorf("could not apply configuration chart for controlplane '%s': %w", client.ObjectKeyFromObject(cp), err)
		}
	}

	// Compute all needed checksums
	checksums, err := a.computeChecksums(ctx, deployedSecrets, cp.Namespace)
	if err != nil {
		return false, err
	}

	var (
		requeue    = false
		scaledDown = false
	)

	if extensionscontroller.IsHibernationEnabled(cluster) {
		dep := &appsv1.Deployment{}
		if err := a.client.Get(ctx, client.ObjectKey{Namespace: cp.Namespace, Name: v1beta1constants.DeploymentNameKubeAPIServer}, dep); client.IgnoreNotFound(err) != nil {
			return false, fmt.Errorf("could not get deployment '%s/%s': %w", cp.Namespace, v1beta1constants.DeploymentNameKubeAPIServer, err)
		}

		// If the cluster is hibernated, check if kube-apiserver has been already scaled down. If it is not yet scaled down
		// then we requeue the `ControlPlane` CRD in order to give the provider-specific control plane components time to
		// properly prepare the cluster for hibernation (whatever needs to be done). If the kube-apiserver is already scaled down
		// then we allow continuing the reconciliation.
		if cluster.Shoot.DeletionTimestamp == nil && (cluster.Shoot.Status.LastOperation == nil || cluster.Shoot.Status.LastOperation.Type != gardencorev1beta1.LastOperationTypeMigrate) {
			if dep.Spec.Replicas != nil && *dep.Spec.Replicas > 0 {
				requeue = true
			} else {
				scaledDown = true
			}
		}
	}

	// Apply control plane chart
	version := cluster.Shoot.Spec.Kubernetes.Version

	if a.controlPlaneChart != nil {
		// Get control plane chart values
		values, err := a.vp.GetControlPlaneChartValues(ctx, cp, cluster, sm, checksums, scaledDown)
		if err != nil {
			return false, err
		}

		log.Info("Applying control plane chart")
		if err := a.controlPlaneChart.Apply(ctx, a.gardenerClientset.ChartApplier(), cp.Namespace, a.imageVector, a.gardenerClientset.Version(), version, values); err != nil {
			return false, fmt.Errorf("could not apply control plane chart for controlplane '%s': %w", client.ObjectKeyFromObject(cp), err)
		}
	}

	// Create shoot chart renderer
	chartRenderer, err := a.chartRendererFactory.NewChartRendererForShoot(version)
	if err != nil {
		return false, fmt.Errorf("could not create chart renderer for shoot '%s': %w", cp.Namespace, err)
	}

	if a.controlPlaneShootChart != nil {
		// Get control plane shoot chart values
		values, err := a.vp.GetControlPlaneShootChartValues(ctx, cp, cluster, sm, checksums)
		if err != nil {
			return false, err
		}

		if err := managedresources.RenderChartAndCreate(ctx, cp.Namespace, ControlPlaneShootChartResourceName, false, a.client, chartRenderer, a.controlPlaneShootChart, values, a.imageVector, metav1.NamespaceSystem, version, true, false); err != nil {
			return false, fmt.Errorf("could not apply control plane shoot chart for controlplane '%s': %w", client.ObjectKeyFromObject(cp), err)
		}
	}

	if a.controlPlaneShootCRDsChart != nil {
		// Get control plane shoot CRDs chart values
		values, err := a.vp.GetControlPlaneShootCRDsChartValues(ctx, cp, cluster)
		if err != nil {
			return false, err
		}

		if err := managedresources.RenderChartAndCreate(ctx, cp.Namespace, ControlPlaneShootCRDsChartResourceName, false, a.client, chartRenderer, a.controlPlaneShootCRDsChart, values, a.imageVector, metav1.NamespaceSystem, version, true, false); err != nil {
			return false, fmt.Errorf("could not apply control plane shoot CRDs chart for controlplane '%s': %w", client.ObjectKeyFromObject(cp), err)
		}
	}

	if a.storageClassesChart != nil {
		// Get storage class chart values
		values, err := a.vp.GetStorageClassesChartValues(ctx, cp, cluster)
		if err != nil {
			return false, err
		}

		if err := managedresources.RenderChartAndCreate(ctx, cp.Namespace, StorageClassesChartResourceName, false, a.client, chartRenderer, a.storageClassesChart, values, a.imageVector, metav1.NamespaceSystem, version, true, true); err != nil {
			return false, fmt.Errorf("could not apply storage classes chart for controlplane '%s': %w", client.ObjectKeyFromObject(cp), err)
		}
	}

	return requeue, sm.Cleanup(ctx)
}

// Delete reconciles the given controlplane and cluster, deleting the additional
// control plane components as needed.
func (a *actuator) Delete(
	ctx context.Context,
	log logr.Logger,
	cp *extensionsv1alpha1.ControlPlane,
	cluster *extensionscontroller.Cluster,
) error {
	sm, err := a.newSecretsManagerForControlPlane(ctx, log, cp, cluster, nil)
	if err != nil {
		return fmt.Errorf("failed to create secrets manager for ControlPlane: %w", err)
	}

	if err := a.delete(ctx, log, cp, cluster); err != nil {
		return err
	}

	return sm.Cleanup(ctx)
}

// ForceDelete forcefully deletes the controlplane.
func (a *actuator) ForceDelete(
	ctx context.Context,
	log logr.Logger,
	cp *extensionsv1alpha1.ControlPlane,
	cluster *extensionscontroller.Cluster,
) error {
	return a.Delete(ctx, log, cp, cluster)
}

func (a *actuator) delete(ctx context.Context, log logr.Logger, cp *extensionsv1alpha1.ControlPlane, cluster *extensionscontroller.Cluster) error {
	if cp.Spec.Purpose != nil && *cp.Spec.Purpose == extensionsv1alpha1.Exposure {
		return a.deleteControlPlaneExposure(ctx, log, cp)
	}

	return a.deleteControlPlane(ctx, log, cp, cluster)
}

// deleteControlPlaneExposure reconciles the given controlplane and cluster, deleting the additional Seed
// control plane components as needed.
func (a *actuator) deleteControlPlaneExposure(
	ctx context.Context,
	log logr.Logger,
	cp *extensionsv1alpha1.ControlPlane,
) error {
	// Delete control plane objects
	if a.controlPlaneExposureChart != nil {
		log.Info("Deleting control plane exposure with objects")
		if err := a.controlPlaneExposureChart.Delete(ctx, a.client, cp.Namespace); client.IgnoreNotFound(err) != nil {
			return fmt.Errorf("could not delete control plane exposure objects for controlplane '%s': %w", client.ObjectKeyFromObject(cp), err)
		}
	}

	if a.exposureShootAccessSecretsFunc != nil {
		for _, shootAccessSecret := range a.exposureShootAccessSecretsFunc(cp.Namespace) {
			if err := kubernetesutils.DeleteObject(ctx, a.client, shootAccessSecret.Secret); err != nil {
				return fmt.Errorf("could not delete control plane exposure shoot access secret '%s' for controlplane '%s': %w", shootAccessSecret.Secret.Name, client.ObjectKeyFromObject(cp), err)
			}
		}
	}

	return nil
}

// deleteControlPlane reconciles the given controlplane and cluster, deleting the additional Shoot
// control plane components as needed.
func (a *actuator) deleteControlPlane(
	ctx context.Context,
	log logr.Logger,
	cp *extensionsv1alpha1.ControlPlane,
	cluster *extensionscontroller.Cluster,
) error {
	forceDelete := cluster != nil && v1beta1helper.ShootNeedsForceDeletion(cluster.Shoot)

	// Get config chart values
	if a.configChart != nil {
		values, err := a.vp.GetConfigChartValues(ctx, cp, cluster)
		if err != nil {
			return fmt.Errorf("failed to get configuration chart values before deletion of controlplane %s: %w", client.ObjectKeyFromObject(cp), err)
		}

		// Apply config chart
		log.Info("Applying configuration chart before deletion")
		if err := a.configChart.Apply(ctx, a.gardenerClientset.ChartApplier(), cp.Namespace, nil, "", "", values); err != nil {
			return fmt.Errorf("could not apply configuration chart before deletion of controlplane '%s': %w", client.ObjectKeyFromObject(cp), err)
		}
	}

	// Delete the managed resources
	if err := managedresources.Delete(ctx, a.client, cp.Namespace, StorageClassesChartResourceName, false); err != nil {
		return fmt.Errorf("could not delete managed resource containing storage classes chart for controlplane '%s': %w", client.ObjectKeyFromObject(cp), err)
	}
	if a.controlPlaneShootCRDsChart != nil {
		if err := managedresources.Delete(ctx, a.client, cp.Namespace, ControlPlaneShootCRDsChartResourceName, false); err != nil {
			return fmt.Errorf("could not delete managed resource containing shoot CRDs chart for controlplane '%s': %w", client.ObjectKeyFromObject(cp), err)
		}

		if !forceDelete {
			// Wait for shoot CRDs chart ManagedResource deletion before deleting the shoot chart ManagedResource
			timeoutCtx1, cancel := context.WithTimeout(ctx, 2*time.Minute)
			defer cancel()
			if err := managedresources.WaitUntilDeleted(timeoutCtx1, a.client, cp.Namespace, ControlPlaneShootCRDsChartResourceName); err != nil {
				return fmt.Errorf("error while waiting for managed resource containing shoot CRDs chart for controlplane '%s' to be deleted: %w", client.ObjectKeyFromObject(cp), err)
			}
		}
	}
	if err := managedresources.Delete(ctx, a.client, cp.Namespace, ControlPlaneShootChartResourceName, false); err != nil {
		return fmt.Errorf("could not delete managed resource containing shoot chart for controlplane '%s': %w", client.ObjectKeyFromObject(cp), err)
	}

	if !forceDelete {
		timeoutCtx2, cancel := context.WithTimeout(ctx, 2*time.Minute)
		defer cancel()
		if err := managedresources.WaitUntilDeleted(timeoutCtx2, a.client, cp.Namespace, StorageClassesChartResourceName); err != nil {
			return fmt.Errorf("error while waiting for managed resource containing storage classes chart for controlplane '%s' to be deleted: %w", client.ObjectKeyFromObject(cp), err)
		}

		timeoutCtx3, cancel := context.WithTimeout(ctx, 2*time.Minute)
		defer cancel()
		if err := managedresources.WaitUntilDeleted(timeoutCtx3, a.client, cp.Namespace, ControlPlaneShootChartResourceName); err != nil {
			return fmt.Errorf("error while waiting for managed resource containing shoot chart for controlplane '%s' to be deleted: %w", client.ObjectKeyFromObject(cp), err)
		}
	}

	// Delete control plane objects
	if a.controlPlaneChart != nil {
		log.Info("Deleting control plane objects")
		if err := a.controlPlaneChart.Delete(ctx, a.client, cp.Namespace); client.IgnoreNotFound(err) != nil {
			return fmt.Errorf("could not delete control plane objects for controlplane '%s': %w", client.ObjectKeyFromObject(cp), err)
		}
	}

	if a.configChart != nil {
		// Delete config objects
		log.Info("Deleting configuration objects")
		if err := a.configChart.Delete(ctx, a.client, cp.Namespace); client.IgnoreNotFound(err) != nil {
			return fmt.Errorf("could not delete configuration objects for controlplane '%s': %w", client.ObjectKeyFromObject(cp), err)
		}
	}

	if a.shootAccessSecretsFunc != nil {
		for _, shootAccessSecret := range a.shootAccessSecretsFunc(cp.Namespace) {
			if err := kubernetesutils.DeleteObject(ctx, a.client, shootAccessSecret.Secret); err != nil {
				return fmt.Errorf("could not delete shoot access secret '%s' for controlplane '%s': %w", shootAccessSecret.Secret.Name, client.ObjectKeyFromObject(cp), err)
			}
		}
	}

	if a.hasShootWebhooks(cluster.Shoot) {
		if err := managedresources.Delete(ctx, a.client, cp.Namespace, ShootWebhooksResourceName, false); err != nil {
			return fmt.Errorf("could not delete managed resource containing shoot webhooks for controlplane '%s': %w", client.ObjectKeyFromObject(cp), err)
		}

		if !forceDelete {
			timeoutCtx4, cancel := context.WithTimeout(ctx, 2*time.Minute)
			defer cancel()
			if err := managedresources.WaitUntilDeleted(timeoutCtx4, a.client, cp.Namespace, ShootWebhooksResourceName); err != nil {
				return fmt.Errorf("error while waiting for managed resource containing shoot webhooks for controlplane '%s' to be deleted: %w", client.ObjectKeyFromObject(cp), err)
			}
		}
	}

	return nil
}

func (a *actuator) hasShootWebhooks(shoot *gardencorev1beta1.Shoot) bool {
	return a.atomicShootWebhookConfig != nil && !v1beta1helper.IsShootAutonomous(shoot)
}

// computeChecksums computes and returns all needed checksums. This includes the checksums for the given deployed secrets,
// as well as the cloud provider secret and configmap that are fetched from the cluster.
func (a *actuator) computeChecksums(
	ctx context.Context,
	deployedSecrets map[string]*corev1.Secret,
	namespace string,
) (map[string]string, error) {
	// Get cloud provider secret and config from cluster
	cpSecret := &corev1.Secret{}
	if err := a.client.Get(ctx, client.ObjectKey{Namespace: namespace, Name: v1beta1constants.SecretNameCloudProvider}, cpSecret); client.IgnoreNotFound(err) != nil {
		return nil, fmt.Errorf("could not get secret '%s/%s': %w", namespace, v1beta1constants.SecretNameCloudProvider, err)
	}

	csSecrets := controlplane.MergeSecretMaps(deployedSecrets, map[string]*corev1.Secret{
		v1beta1constants.SecretNameCloudProvider: cpSecret,
	})

	var csConfigMaps map[string]*corev1.ConfigMap
	if len(a.configName) != 0 {
		cpConfigMap := &corev1.ConfigMap{}
		if err := a.client.Get(ctx, client.ObjectKey{Namespace: namespace, Name: a.configName}, cpConfigMap); err != nil {
			return nil, fmt.Errorf("could not get configmap '%s/%s': %w", namespace, a.configName, err)
		}

		csConfigMaps = map[string]*corev1.ConfigMap{
			a.configName: cpConfigMap,
		}
	}

	return controlplane.ComputeChecksums(csSecrets, csConfigMaps), nil
}

// Restore reconciles the given controlplane and cluster, restoring the additional Shoot
// control plane components as needed.
func (a *actuator) Restore(
	ctx context.Context,
	log logr.Logger,
	cp *extensionsv1alpha1.ControlPlane,
	cluster *extensionscontroller.Cluster,
) (
	bool,
	error,
) {
	return a.Reconcile(ctx, log, cp, cluster)
}

// Migrate reconciles the given controlplane and cluster, deleting the additional
// control plane components as needed.
func (a *actuator) Migrate(
	ctx context.Context,
	log logr.Logger,
	cp *extensionsv1alpha1.ControlPlane,
	cluster *extensionscontroller.Cluster,
) error {
	// Keep objects for shoot managed resources so that they are not deleted from the shoot during the migration
	if err := managedresources.SetKeepObjects(ctx, a.client, cp.Namespace, ControlPlaneShootChartResourceName, true); err != nil {
		return fmt.Errorf("could not keep objects of managed resource containing shoot chart for controlplane '%s': %w", client.ObjectKeyFromObject(cp), err)
	}
	if a.controlPlaneShootCRDsChart != nil {
		if err := managedresources.SetKeepObjects(ctx, a.client, cp.Namespace, ControlPlaneShootCRDsChartResourceName, true); err != nil {
			return fmt.Errorf("could not keep objects of managed resource containing shoot CRDs chart for controlplane '%s': %w", client.ObjectKeyFromObject(cp), err)
		}
	}
	if err := managedresources.SetKeepObjects(ctx, a.client, cp.Namespace, StorageClassesChartResourceName, true); err != nil {
		return fmt.Errorf("could not keep objects of managed resource containing storage classes chart for controlplane '%s': %w", client.ObjectKeyFromObject(cp), err)
	}

	return a.delete(ctx, log, cp, cluster)
}

func (a *actuator) newSecretsManagerForControlPlane(ctx context.Context, log logr.Logger, cp *extensionsv1alpha1.ControlPlane, cluster *extensionscontroller.Cluster, secretConfigs []extensionssecretsmanager.SecretConfigWithOptions) (secretsmanager.Interface, error) {
	identity := a.providerName + "-controlplane"
	if purpose := cp.Spec.Purpose; purpose != nil && *purpose != extensionsv1alpha1.Normal {
		identity += "-" + string(*purpose)
	}

	return a.newSecretsManager(ctx, log.WithName("secretsmanager"), clock.RealClock{}, a.client, cluster, identity, secretConfigs)
}

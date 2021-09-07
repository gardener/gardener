// Copyright (c) 2019 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package genericactuator

import (
	"bytes"
	"context"
	"fmt"
	"time"

	extensionscontroller "github.com/gardener/gardener/extensions/pkg/controller"
	"github.com/gardener/gardener/extensions/pkg/controller/controlplane"
	"github.com/gardener/gardener/extensions/pkg/webhook"
	extensionswebhookshoot "github.com/gardener/gardener/extensions/pkg/webhook/shoot"
	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
	clientkubernetes "github.com/gardener/gardener/pkg/client/kubernetes"
	gardenerkubernetes "github.com/gardener/gardener/pkg/client/kubernetes"
	"github.com/gardener/gardener/pkg/extensions"
	"github.com/gardener/gardener/pkg/utils/chart"
	"github.com/gardener/gardener/pkg/utils/flow"
	"github.com/gardener/gardener/pkg/utils/imagevector"
	kutil "github.com/gardener/gardener/pkg/utils/kubernetes"
	"github.com/gardener/gardener/pkg/utils/managedresources"
	secretutil "github.com/gardener/gardener/pkg/utils/secrets"
	"github.com/gardener/gardener/pkg/utils/version"

	"github.com/Masterminds/semver"
	"github.com/go-logr/logr"
	admissionregistrationv1 "k8s.io/api/admissionregistration/v1"
	admissionregistrationv1beta1 "k8s.io/api/admissionregistration/v1beta1"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/serializer/json"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/runtime/inject"
)

// ValuesProvider provides values for the 2 charts applied by this actuator.
type ValuesProvider interface {
	// GetConfigChartValues returns the values for the config chart applied by this actuator.
	GetConfigChartValues(ctx context.Context, cp *extensionsv1alpha1.ControlPlane, cluster *extensionscontroller.Cluster) (map[string]interface{}, error)
	// GetControlPlaneChartValues returns the values for the control plane chart applied by this actuator.
	GetControlPlaneChartValues(ctx context.Context, cp *extensionsv1alpha1.ControlPlane, cluster *extensionscontroller.Cluster, checksums map[string]string, scaledDown bool) (map[string]interface{}, error)
	// GetControlPlaneShootChartValues returns the values for the control plane shoot chart applied by this actuator.
	GetControlPlaneShootChartValues(ctx context.Context, cp *extensionsv1alpha1.ControlPlane, cluster *extensionscontroller.Cluster, checksums map[string]string) (map[string]interface{}, error)
	// GetControlPlaneShootCRDsChartValues returns the values for the control plane shoot CRDs chart applied by this actuator.
	GetControlPlaneShootCRDsChartValues(ctx context.Context, cp *extensionsv1alpha1.ControlPlane, cluster *extensionscontroller.Cluster) (map[string]interface{}, error)
	// GetStorageClassesChartValues returns the values for the storage classes chart applied by this actuator.
	GetStorageClassesChartValues(ctx context.Context, cp *extensionsv1alpha1.ControlPlane, cluster *extensionscontroller.Cluster) (map[string]interface{}, error)
	// GetControlPlaneExposureChartValues returns the values for the control plane exposure chart applied by this actuator.
	GetControlPlaneExposureChartValues(ctx context.Context, cp *extensionsv1alpha1.ControlPlane, cluster *extensionscontroller.Cluster, checksums map[string]string) (map[string]interface{}, error)
}

// NewActuator creates a new Actuator that acts upon and updates the status of ControlPlane resources.
// It creates / deletes the given secrets and applies / deletes the given charts, using the given image vector and
// the values provided by the given values provider.
func NewActuator(
	providerName string,
	secrets, exposureSecrets secretutil.Interface,
	configChart, controlPlaneChart, controlPlaneShootChart, controlPlaneShootCRDsChart, storageClassesChart, controlPlaneExposureChart chart.Interface,
	vp ValuesProvider,
	chartRendererFactory extensionscontroller.ChartRendererFactory,
	imageVector imagevector.ImageVector,
	configName string,
	shootWebhooks []admissionregistrationv1.MutatingWebhook,
	webhookServerPort int,
	logger logr.Logger,
) controlplane.Actuator {
	return &actuator{
		providerName:               providerName,
		secrets:                    secrets,
		exposureSecrets:            exposureSecrets,
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
		shootWebhooks:              shootWebhooks,
		webhookServerPort:          webhookServerPort,
		logger:                     logger.WithName("controlplane-actuator"),
	}
}

// actuator is an Actuator that acts upon and updates the status of ControlPlane resources.
type actuator struct {
	providerName               string
	secrets                    secretutil.Interface
	exposureSecrets            secretutil.Interface
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
	shootWebhooks              []admissionregistrationv1.MutatingWebhook
	webhookServerPort          int

	clientset         kubernetes.Interface
	gardenerClientset gardenerkubernetes.Interface
	chartApplier      gardenerkubernetes.ChartApplier
	client            client.Client
	logger            logr.Logger
}

// InjectFunc enables injecting Kubernetes dependencies into actuator's dependencies.
func (a *actuator) InjectFunc(f inject.Func) error {
	return f(a.vp)
}

// InjectConfig injects the given config into the actuator.
func (a *actuator) InjectConfig(config *rest.Config) error {
	// Create clientset
	var err error
	a.clientset, err = kubernetes.NewForConfig(config)
	if err != nil {
		return fmt.Errorf("could not create Kubernetes client: %w", err)
	}

	// Create Gardener clientset
	a.gardenerClientset, err = gardenerkubernetes.NewWithConfig(gardenerkubernetes.WithRESTConfig(config))
	if err != nil {
		return fmt.Errorf("could not create Gardener client: %w", err)
	}

	a.chartApplier = a.gardenerClientset.ChartApplier()

	return nil
}

// InjectClient injects the given client into the valuesProvider.
func (a *actuator) InjectClient(client client.Client) error {
	a.client = client
	return nil
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

// Reconcile reconciles the given controlplane and cluster, creating or updating the additional Shoot
// control plane components as needed.
func (a *actuator) Reconcile(
	ctx context.Context,
	cp *extensionsv1alpha1.ControlPlane,
	cluster *extensionscontroller.Cluster,
) (bool, error) {
	if cp.Spec.Purpose != nil && *cp.Spec.Purpose == extensionsv1alpha1.Exposure {
		return a.reconcileControlPlaneExposure(ctx, cp, cluster)
	}
	return a.reconcileControlPlane(ctx, cp, cluster)
}

func (a *actuator) reconcileControlPlaneExposure(
	ctx context.Context,
	cp *extensionsv1alpha1.ControlPlane,
	cluster *extensionscontroller.Cluster,
) (bool, error) {
	if a.controlPlaneExposureChart == nil {
		return false, nil
	}

	// Deploy secrets
	checksums := make(map[string]string)
	if a.exposureSecrets != nil {
		a.logger.Info("Deploying control plane exposure secrets", "controlplane", kutil.ObjectName(cp))
		deployedSecrets, err := a.exposureSecrets.Deploy(ctx, a.clientset, a.gardenerClientset, cp.Namespace)
		if err != nil {
			return false, fmt.Errorf("could not deploy control plane exposure secrets for controlplane '%s': %w", kutil.ObjectName(cp), err)
		}
		// Compute needed checksums
		checksums = controlplane.ComputeChecksums(deployedSecrets, nil)
	}

	// Get control plane exposure chart values
	values, err := a.vp.GetControlPlaneExposureChartValues(ctx, cp, cluster, checksums)
	if err != nil {
		return false, err
	}

	// Apply control plane exposure chart
	a.logger.Info("Applying control plane exposure chart", "controlplaneexposure", kutil.ObjectName(cp), "values", values)
	version := cluster.Shoot.Spec.Kubernetes.Version
	if err := a.controlPlaneExposureChart.Apply(ctx, a.chartApplier, cp.Namespace, a.imageVector, a.gardenerClientset.Version(), version, values); err != nil {
		return false, fmt.Errorf("could not apply control plane exposure chart for controlplane '%s': %w", kutil.ObjectName(cp), err)
	}

	return false, nil
}

// reconcileControlPlane reconciles the given controlplane and cluster, creating or updating the additional Shoot
// control plane components as needed.
func (a *actuator) reconcileControlPlane(
	ctx context.Context,
	cp *extensionsv1alpha1.ControlPlane,
	cluster *extensionscontroller.Cluster,
) (bool, error) {

	if len(a.shootWebhooks) > 0 {
		if err := ReconcileShootWebhooks(ctx, a.client, cp.Namespace, a.providerName, a.webhookServerPort, a.shootWebhooks, cluster); err != nil {
			return false, fmt.Errorf("could not reconcile shoot webhooks: %w", err)
		}
	}

	// Deploy secrets
	a.logger.Info("Deploying secrets", "controlplane", kutil.ObjectName(cp))
	deployedSecrets, err := a.secrets.Deploy(ctx, a.clientset, a.gardenerClientset, cp.Namespace)
	if err != nil {
		return false, fmt.Errorf("could not deploy secrets for controlplane '%s': %w", kutil.ObjectName(cp), err)
	}

	// Get config chart values
	if a.configChart != nil {
		values, err := a.vp.GetConfigChartValues(ctx, cp, cluster)
		if err != nil {
			return false, err
		}

		// Apply config chart
		a.logger.Info("Applying configuration chart", "controlplane", kutil.ObjectName(cp))
		if err := a.configChart.Apply(ctx, a.chartApplier, cp.Namespace, nil, "", "", values); err != nil {
			return false, fmt.Errorf("could not apply configuration chart for controlplane '%s': %w", kutil.ObjectName(cp), err)
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

	if extensionscontroller.IsHibernated(cluster) {
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

	// Get control plane chart values
	values, err := a.vp.GetControlPlaneChartValues(ctx, cp, cluster, checksums, scaledDown)
	if err != nil {
		return false, err
	}

	// Apply control plane chart
	version := cluster.Shoot.Spec.Kubernetes.Version
	a.logger.Info("Applying control plane chart", "controlplane", kutil.ObjectName(cp))
	if err := a.controlPlaneChart.Apply(ctx, a.chartApplier, cp.Namespace, a.imageVector, a.gardenerClientset.Version(), version, values); err != nil {
		return false, fmt.Errorf("could not apply control plane chart for controlplane '%s': %w", kutil.ObjectName(cp), err)
	}

	// Create shoot chart renderer
	chartRenderer, err := a.chartRendererFactory.NewChartRendererForShoot(version)
	if err != nil {
		return false, fmt.Errorf("could not create chart renderer for shoot '%s': %w", cp.Namespace, err)
	}

	// Get control plane shoot chart values
	values, err = a.vp.GetControlPlaneShootChartValues(ctx, cp, cluster, checksums)
	if err != nil {
		return false, err
	}

	if err := managedresources.RenderChartAndCreate(ctx, cp.Namespace, ControlPlaneShootChartResourceName, false, a.client, chartRenderer, a.controlPlaneShootChart, values, a.imageVector, metav1.NamespaceSystem, version, true, false); err != nil {
		return false, fmt.Errorf("could not apply control plane shoot chart for controlplane '%s': %w", kutil.ObjectName(cp), err)
	}

	if a.controlPlaneShootCRDsChart != nil {
		// Get control plane shoot CRDs chart values
		values, err = a.vp.GetControlPlaneShootCRDsChartValues(ctx, cp, cluster)
		if err != nil {
			return false, err
		}

		if err := managedresources.RenderChartAndCreate(ctx, cp.Namespace, ControlPlaneShootCRDsChartResourceName, false, a.client, chartRenderer, a.controlPlaneShootCRDsChart, values, a.imageVector, metav1.NamespaceSystem, version, true, false); err != nil {
			return false, fmt.Errorf("could not apply control plane shoot CRDs chart for controlplane '%s': %w", kutil.ObjectName(cp), err)
		}
	}

	// Get storage classes
	values, err = a.vp.GetStorageClassesChartValues(ctx, cp, cluster)
	if err != nil {
		return false, err
	}

	if err := managedresources.RenderChartAndCreate(ctx, cp.Namespace, StorageClassesChartResourceName, false, a.client, chartRenderer, a.storageClassesChart, values, a.imageVector, metav1.NamespaceSystem, version, true, true); err != nil {
		return false, fmt.Errorf("could not apply storage classes chart for controlplane '%s': %w", kutil.ObjectName(cp), err)
	}

	return requeue, nil
}

// Delete reconciles the given controlplane and cluster, deleting the additional
// control plane components as needed.
func (a *actuator) Delete(
	ctx context.Context,
	cp *extensionsv1alpha1.ControlPlane,
	cluster *extensionscontroller.Cluster,
) error {
	if cp.Spec.Purpose != nil && *cp.Spec.Purpose == extensionsv1alpha1.Exposure {
		return a.deleteControlPlaneExposure(ctx, cp, cluster)
	}
	return a.deleteControlPlane(ctx, cp, cluster)
}

// deleteControlPlaneExposure reconciles the given controlplane and cluster, deleting the additional Seed
// control plane components as needed.
func (a *actuator) deleteControlPlaneExposure(
	ctx context.Context,
	cp *extensionsv1alpha1.ControlPlane,
	cluster *extensionscontroller.Cluster,
) error {
	// Delete control plane objects
	if a.controlPlaneExposureChart != nil {
		a.logger.Info("Deleting control plane exposure with objects", "controlplane", kutil.ObjectName(cp))
		if err := a.controlPlaneExposureChart.Delete(ctx, a.client, cp.Namespace); client.IgnoreNotFound(err) != nil {
			return fmt.Errorf("could not delete control plane exposure objects for controlplane '%s': %w", kutil.ObjectName(cp), err)
		}
	}

	// Delete secrets
	if a.exposureSecrets != nil {
		a.logger.Info("Deleting secrets for control plane with purpose exposure", "controlplane", kutil.ObjectName(cp))
		if err := a.exposureSecrets.Delete(ctx, a.clientset, cp.Namespace); client.IgnoreNotFound(err) != nil {
			return fmt.Errorf("could not delete secrets for controlplane exposure '%s': %w", kutil.ObjectName(cp), err)
		}
	}

	return nil
}

// deleteControlPlane reconciles the given controlplane and cluster, deleting the additional Shoot
// control plane components as needed.
func (a *actuator) deleteControlPlane(
	ctx context.Context,
	cp *extensionsv1alpha1.ControlPlane,
	cluster *extensionscontroller.Cluster,
) error {
	// Delete the managed resources
	if err := managedresources.Delete(ctx, a.client, cp.Namespace, StorageClassesChartResourceName, false); err != nil {
		return fmt.Errorf("could not delete managed resource containing storage classes chart for controlplane '%s': %w", kutil.ObjectName(cp), err)
	}
	if a.controlPlaneShootCRDsChart != nil {
		if err := managedresources.Delete(ctx, a.client, cp.Namespace, ControlPlaneShootCRDsChartResourceName, false); err != nil {
			return fmt.Errorf("could not delete managed resource containing shoot CRDs chart for controlplane '%s': %w", kutil.ObjectName(cp), err)
		}

		// Wait for shoot CRDs chart ManagedResource deletion before deleting the shoot chart ManagedResource
		timeoutCtx1, cancel := context.WithTimeout(ctx, 2*time.Minute)
		defer cancel()
		if err := managedresources.WaitUntilDeleted(timeoutCtx1, a.client, cp.Namespace, ControlPlaneShootCRDsChartResourceName); err != nil {
			return fmt.Errorf("error while waiting for managed resource containing shoot CRDs chart for controlplane '%s' to be deleted: %w", kutil.ObjectName(cp), err)
		}
	}
	if err := managedresources.Delete(ctx, a.client, cp.Namespace, ControlPlaneShootChartResourceName, false); err != nil {
		return fmt.Errorf("could not delete managed resource containing shoot chart for controlplane '%s': %w", kutil.ObjectName(cp), err)
	}

	timeoutCtx2, cancel := context.WithTimeout(ctx, 2*time.Minute)
	defer cancel()
	if err := managedresources.WaitUntilDeleted(timeoutCtx2, a.client, cp.Namespace, StorageClassesChartResourceName); err != nil {
		return fmt.Errorf("error while waiting for managed resource containing storage classes chart for controlplane '%s' to be deleted: %w", kutil.ObjectName(cp), err)
	}

	timeoutCtx3, cancel := context.WithTimeout(ctx, 2*time.Minute)
	defer cancel()
	if err := managedresources.WaitUntilDeleted(timeoutCtx3, a.client, cp.Namespace, ControlPlaneShootChartResourceName); err != nil {
		return fmt.Errorf("error while waiting for managed resource containing shoot chart for controlplane '%s' to be deleted: %w", kutil.ObjectName(cp), err)
	}

	// Delete control plane objects
	a.logger.Info("Deleting control plane objects", "controlplane", kutil.ObjectName(cp))
	if err := a.controlPlaneChart.Delete(ctx, a.client, cp.Namespace); client.IgnoreNotFound(err) != nil {
		return fmt.Errorf("could not delete control plane objects for controlplane '%s': %w", kutil.ObjectName(cp), err)
	}

	if a.configChart != nil {
		// Delete config objects
		a.logger.Info("Deleting configuration objects", "controlplane", kutil.ObjectName(cp))
		if err := a.configChart.Delete(ctx, a.client, cp.Namespace); client.IgnoreNotFound(err) != nil {
			return fmt.Errorf("could not delete configuration objects for controlplane '%s': %w", kutil.ObjectName(cp), err)
		}
	}

	// Delete secrets
	a.logger.Info("Deleting secrets", "controlplane", kutil.ObjectName(cp))
	if err := a.secrets.Delete(ctx, a.clientset, cp.Namespace); client.IgnoreNotFound(err) != nil {
		return fmt.Errorf("could not delete secrets for controlplane '%s': %w", kutil.ObjectName(cp), err)
	}

	if len(a.shootWebhooks) > 0 {
		networkPolicy := extensionswebhookshoot.GetNetworkPolicyMeta(cp.Namespace, a.providerName)
		if err := a.client.Delete(ctx, networkPolicy); client.IgnoreNotFound(err) != nil {
			return fmt.Errorf("could not delete network policy for shoot webhooks in namespace '%s': %w", cp.Namespace, err)
		}

		if err := managedresources.Delete(ctx, a.client, cp.Namespace, ShootWebhooksResourceName, false); err != nil {
			return fmt.Errorf("could not delete managed resource containing shoot webhooks for controlplane '%s': %w", kutil.ObjectName(cp), err)
		}

		timeoutCtx4, cancel := context.WithTimeout(ctx, 2*time.Minute)
		defer cancel()
		if err := managedresources.WaitUntilDeleted(timeoutCtx4, a.client, cp.Namespace, ShootWebhooksResourceName); err != nil {
			return fmt.Errorf("error while waiting for managed resource containing shoot webhooks for controlplane '%s' to be deleted: %w", kutil.ObjectName(cp), err)
		}
	}

	return nil
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
	if err := a.client.Get(ctx, client.ObjectKey{Namespace: namespace, Name: v1beta1constants.SecretNameCloudProvider}, cpSecret); err != nil {
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

func marshalWebhooks(webhooks []admissionregistrationv1.MutatingWebhook, name string, k8sVersion *semver.Version) ([]byte, error) {
	var (
		buf     = new(bytes.Buffer)
		encoder = json.NewYAMLSerializer(json.DefaultMetaFactory, nil, nil)

		apiVersion, kind                            = admissionregistrationv1.SchemeGroupVersion.WithKind("MutatingWebhookConfiguration").ToAPIVersionAndKind()
		mutatingWebhookConfiguration runtime.Object = &admissionregistrationv1.MutatingWebhookConfiguration{
			TypeMeta: metav1.TypeMeta{
				APIVersion: apiVersion,
				Kind:       kind,
			},
			ObjectMeta: metav1.ObjectMeta{
				Name: webhook.NamePrefix + name + webhook.NameSuffixShoot,
			},
			Webhooks: webhooks,
		}
	)

	if version.ConstraintK8sLessEqual115.Check(k8sVersion) {
		u := &unstructured.Unstructured{}
		if err := clientkubernetes.ShootScheme.Convert(mutatingWebhookConfiguration, u, nil); err != nil {
			return nil, err
		}
		// Set APIVersion to v1beta1. We can transform v1 directly to v1beta1 because both APIs are identical.
		u.SetAPIVersion(admissionregistrationv1beta1.SchemeGroupVersion.String())
		mutatingWebhookConfiguration = u
	}

	if err := encoder.Encode(mutatingWebhookConfiguration, buf); err != nil {
		return nil, err
	}

	return buf.Bytes(), nil
}

// Restore reconciles the given controlplane and cluster, restoring the additional Shoot
// control plane components as needed.
func (a *actuator) Restore(
	ctx context.Context,
	cp *extensionsv1alpha1.ControlPlane,
	cluster *extensionscontroller.Cluster,
) (bool, error) {
	return a.Reconcile(ctx, cp, cluster)
}

// Migrate reconciles the given controlplane and cluster, deleting the additional
// control plane components as needed.
func (a *actuator) Migrate(
	ctx context.Context,
	cp *extensionsv1alpha1.ControlPlane,
	cluster *extensionscontroller.Cluster,
) error {
	// Keep objects for shoot managed resources so that they are not deleted from the shoot during the migration
	if err := managedresources.SetKeepObjects(ctx, a.client, cp.Namespace, ControlPlaneShootChartResourceName, true); err != nil {
		return fmt.Errorf("could not keep objects of managed resource containing shoot chart for controlplane '%s': %w", kutil.ObjectName(cp), err)
	}
	if a.controlPlaneShootCRDsChart != nil {
		if err := managedresources.SetKeepObjects(ctx, a.client, cp.Namespace, ControlPlaneShootCRDsChartResourceName, true); err != nil {
			return fmt.Errorf("could not keep objects of managed resource containing shoot CRDs chart for controlplane '%s': %w", kutil.ObjectName(cp), err)
		}
	}
	if err := managedresources.SetKeepObjects(ctx, a.client, cp.Namespace, StorageClassesChartResourceName, true); err != nil {
		return fmt.Errorf("could not keep objects of managed resource containing storage classes chart for controlplane '%s': %w", kutil.ObjectName(cp), err)
	}

	return a.Delete(ctx, cp, cluster)
}

// ReconcileShootWebhooks deploys the shoot webhook configuration, i.e., a network policy to allow the kube-apiserver to
// talk to the provider extension, and a managed resource that contains the MutatingWebhookConfiguration.
func ReconcileShootWebhooks(ctx context.Context, c client.Client, namespace, providerName string, serverPort int, shootWebhooks []admissionregistrationv1.MutatingWebhook, cluster *extensionscontroller.Cluster) error {
	if err := extensionswebhookshoot.EnsureNetworkPolicy(ctx, c, namespace, providerName, serverPort); err != nil {
		return fmt.Errorf("could not create or update network policy for shoot webhooks in namespace '%s': %w", namespace, err)
	}

	if cluster.Shoot == nil {
		return fmt.Errorf("no shoot found in cluster resource")
	}

	shootK8sVersion, err := semver.NewVersion(cluster.Shoot.Spec.Kubernetes.Version)
	if err != nil {
		return err
	}

	webhookConfiguration, err := marshalWebhooks(shootWebhooks, providerName, shootK8sVersion)
	if err != nil {
		return err
	}
	data := map[string][]byte{"mutatingwebhookconfiguration.yaml": webhookConfiguration}

	if err := managedresources.Create(ctx, c, namespace, ShootWebhooksResourceName, false, "", data, nil, nil, nil); err != nil {
		return fmt.Errorf("could not create or update managed resource '%s/%s' containing shoot webhooks: %w", namespace, ShootWebhooksResourceName, err)
	}

	return nil
}

// ReconcileShootWebhooksForAllNamespaces reconciles the shoot webhooks in all shoot namespaces of the given
// provider type. This is necessary in case the webhook port is changed (otherwise, the network policy would only be
// updated again as part of the ControlPlane reconciliation which might only happen in the next 24h).
func ReconcileShootWebhooksForAllNamespaces(ctx context.Context, c client.Client, providerName, providerType string, port int, shootWebhooks []admissionregistrationv1.MutatingWebhook) error {
	namespaceList := &corev1.NamespaceList{}
	if err := c.List(ctx, namespaceList, client.MatchingLabels{
		v1beta1constants.GardenRole:         v1beta1constants.GardenRoleShoot,
		v1beta1constants.LabelShootProvider: providerType,
	}); err != nil {
		return err
	}

	fns := make([]flow.TaskFn, 0, len(namespaceList.Items))

	for _, namespace := range namespaceList.Items {
		var (
			networkPolicy     = extensionswebhookshoot.GetNetworkPolicyMeta(namespace.Name, providerName)
			namespaceName     = namespace.Name
			networkPolicyName = networkPolicy.Name
		)

		fns = append(fns, func(ctx context.Context) error {
			if err := c.Get(ctx, kutil.Key(namespaceName, networkPolicyName), &networkingv1.NetworkPolicy{}); err != nil {
				if !apierrors.IsNotFound(err) {
					return err
				}
				return nil
			}

			cluster, err := extensions.GetCluster(ctx, c, namespaceName)
			if err != nil {
				return err
			}

			return ReconcileShootWebhooks(ctx, c, namespaceName, providerName, port, shootWebhooks, cluster)
		})
	}

	return flow.Parallel(fns...)(ctx)
}

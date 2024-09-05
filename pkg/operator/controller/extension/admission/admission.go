// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package admission

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/go-logr/logr"
	appsv1 "k8s.io/api/apps/v1"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/tools/record"
	"sigs.k8s.io/controller-runtime/pkg/client"

	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	operatorv1alpha1 "github.com/gardener/gardener/pkg/apis/operator/v1alpha1"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	"github.com/gardener/gardener/pkg/utils"
	gardenerutils "github.com/gardener/gardener/pkg/utils/gardener"
	kubernetesutils "github.com/gardener/gardener/pkg/utils/kubernetes"
	"github.com/gardener/gardener/pkg/utils/managedresources"
	"github.com/gardener/gardener/pkg/utils/oci"
)

// Interface contains functions for the admission deployment.
type Interface interface {
	// Reconcile creates or updates admission resources.
	Reconcile(context.Context, logr.Logger, kubernetes.Interface, string, *operatorv1alpha1.Extension) error
	// Delete deletes all admission resources.
	Delete(context.Context, logr.Logger, *operatorv1alpha1.Extension) error
}

type deployment struct {
	runtimeClientSet kubernetes.Interface
	recorder         record.EventRecorder

	gardenNamespace string
	helmRegistry    oci.Interface
}

// Reconcile creates or updates admission resources.
// If the extension doesn't define an admission, the deployment is deleted.
func (d *deployment) Reconcile(ctx context.Context, log logr.Logger, virtualClusterClientSet kubernetes.Interface, genericTokenKubeconfigSecretName string, extension *operatorv1alpha1.Extension) error {
	if virtualDeploymentSpecified(extension) {
		log.Info("Deploying admission virtual garden resources")
		if err := d.createOrUpdateAdmissionVirtualClusterResources(ctx, virtualClusterClientSet, extension); err != nil {
			return err
		}
		d.recorder.Event(extension, corev1.EventTypeNormal, "Reconciliation", "Admission deployment applied successfully in virtual cluster")
	} else {
		if err := d.deleteAdmissionVirtualClusterResources(ctx, log, extension); err != nil {
			return err
		}
		d.recorder.Event(extension, corev1.EventTypeNormal, "Deletion", "Admission deployment deleted successfully in virtual cluster")
	}

	if runtimeDeploymentSpecified(extension) {
		log.Info("Deploying admission runtime resources")
		if err := d.createOrUpdateAdmissionRuntimeClusterResources(ctx, genericTokenKubeconfigSecretName, extension); err != nil {
			return err
		}
		d.recorder.Event(extension, corev1.EventTypeNormal, "Reconciliation", "Admission deployment applied successfully in runtime cluster")
	} else {
		if err := d.deleteAdmissionRuntimeClusterResources(ctx, log, extension); err != nil {
			return err
		}
		d.recorder.Event(extension, corev1.EventTypeNormal, "Deletion", "Admission deployment deleted successfully in runtime cluster")
	}

	return nil
}

// Delete deletes all admission resources.
func (d *deployment) Delete(ctx context.Context, log logr.Logger, extension *operatorv1alpha1.Extension) error {
	log.Info("Deleting admission deployment")
	if err := d.deleteAdmissionRuntimeClusterResources(ctx, log, extension); err != nil {
		return err
	}
	return d.deleteAdmissionVirtualClusterResources(ctx, log, extension)
}

func resourceName(extension *operatorv1alpha1.Extension) string {
	return fmt.Sprintf("extension-admission-%s", extension.Name)
}

func runtimeManagedResourceName(extension *operatorv1alpha1.Extension) string {
	return fmt.Sprintf("extension-admission-runtime-%s", extension.Name)
}

func (d *deployment) createOrUpdateAdmissionRuntimeClusterResources(ctx context.Context, genericTokenKubeconfigSecretName string, extension *operatorv1alpha1.Extension) error {
	archive, err := d.helmRegistry.Pull(ctx, extension.Spec.Deployment.AdmissionDeployment.RuntimeCluster.Helm.OCIRepository)
	if err != nil {
		return fmt.Errorf("failed pulling Helm chart from OCI repository: %w", err)
	}

	accessSecret := d.getVirtualClusterAccessSecret(resourceName(extension))
	if err := accessSecret.Reconcile(ctx, d.runtimeClientSet.Client()); err != nil {
		return fmt.Errorf("failed reconciling access secret: %w", err)
	}

	gardenerValues := map[string]any{
		"gardener": map[string]any{
			"runtimeCluster": map[string]any{
				"priorityClassName": v1beta1constants.PriorityClassNameGardenSystem400,
			},
		},
	}

	var helmValues map[string]any
	if extension.Spec.Deployment.AdmissionDeployment.Values != nil {
		if err := json.Unmarshal(extension.Spec.Deployment.AdmissionDeployment.Values.Raw, &helmValues); err != nil {
			return err
		}
	}

	renderedChart, err := d.runtimeClientSet.ChartRenderer().RenderArchive(archive, extension.Name, v1beta1constants.GardenNamespace, utils.MergeMaps(helmValues, gardenerValues))
	if err != nil {
		return fmt.Errorf("failed rendering Helm chart: %w", err)
	}

	secretData := renderedChart.AsSecretData()

	// Inject Kubeconfig for Garden cluster access.
	if err := gardenerutils.MutateObjectsInSecretData(
		secretData,
		d.gardenNamespace,
		[]string{appsv1.GroupName, batchv1.GroupName},
		func(obj runtime.Object) error {
			return gardenerutils.InjectGenericGardenKubeconfig(
				obj,
				genericTokenKubeconfigSecretName,
				accessSecret.Secret.Name,
				gardenerutils.VolumeMountPathGenericKubeconfig,
			)
		}); err != nil {
		return fmt.Errorf("failed to inject garden access secrets: %w", err)
	}

	if err := managedresources.CreateForSeedWithLabels(
		ctx,
		d.runtimeClientSet.Client(),
		d.gardenNamespace,
		runtimeManagedResourceName(extension),
		false,
		map[string]string{managedresources.LabelKeyOrigin: managedresources.LabelValueOperator},
		secretData,
	); err != nil {
		return fmt.Errorf("failed creating ManagedResource: %w", err)
	}

	if err := managedresources.WaitUntilHealthyAndNotProgressing(ctx, d.runtimeClientSet.Client(), d.gardenNamespace, runtimeManagedResourceName(extension)); err != nil {
		return fmt.Errorf("failed waiting for ManagedResource to be healthy: %w", err)
	}
	return nil
}

func (d *deployment) deleteAdmissionRuntimeClusterResources(ctx context.Context, log logr.Logger, extension *operatorv1alpha1.Extension) error {
	managedResourceName := runtimeManagedResourceName(extension)

	log.Info("Deleting admission ManagedResource for runtime cluster if present", "managedResource", client.ObjectKey{Name: managedResourceName, Namespace: d.gardenNamespace})
	if err := managedresources.DeleteForSeed(ctx, d.runtimeClientSet.Client(), d.gardenNamespace, managedResourceName); err != nil {
		return fmt.Errorf("failed deleting ManagedResource: %w", err)
	}

	if err := managedresources.WaitUntilDeleted(ctx, d.runtimeClientSet.Client(), d.gardenNamespace, runtimeManagedResourceName(extension)); err != nil {
		return fmt.Errorf("failed waiting for ManagedResource to be deleted: %w", err)
	}

	accessSecret := d.getVirtualClusterAccessSecret(resourceName(extension)).Secret

	log.Info("Deleting admission access secret for virtual cluster", "secret", client.ObjectKeyFromObject(accessSecret))
	return kubernetesutils.DeleteObjects(ctx, d.runtimeClientSet.Client(), accessSecret)
}

func admissionVirtualManagedResourceName(extension *operatorv1alpha1.Extension) string {
	return fmt.Sprintf("extension-admission-virtual-%s", extension.Name)
}

func (d *deployment) createOrUpdateAdmissionVirtualClusterResources(ctx context.Context, virtualClusterClientSet kubernetes.Interface, extension *operatorv1alpha1.Extension) error {
	archive, err := d.helmRegistry.Pull(ctx, extension.Spec.Deployment.AdmissionDeployment.VirtualCluster.Helm.OCIRepository)
	if err != nil {
		return fmt.Errorf("failed pulling Helm chart from OCI repository: %w", err)
	}

	accessSecret := d.getVirtualClusterAccessSecret(resourceName(extension))

	gardenerValues := map[string]any{
		"gardener": map[string]any{
			"virtualCluster": map[string]any{
				"serviceAccount": map[string]any{
					"name":      accessSecret.ServiceAccountName,
					"namespace": metav1.NamespaceSystem,
				},
			},
		},
	}

	var helmValues map[string]any
	if extension.Spec.Deployment.AdmissionDeployment.Values != nil {
		if err := json.Unmarshal(extension.Spec.Deployment.AdmissionDeployment.Values.Raw, &helmValues); err != nil {
			return err
		}
	}

	renderedChart, err := virtualClusterClientSet.ChartRenderer().RenderArchive(archive, extension.Name, v1beta1constants.GardenNamespace, utils.MergeMaps(helmValues, gardenerValues))
	if err != nil {
		return fmt.Errorf("failed rendering Helm chart: %w", err)
	}

	if err := managedresources.CreateForShoot(ctx, d.runtimeClientSet.Client(), d.gardenNamespace, admissionVirtualManagedResourceName(extension), managedresources.LabelValueOperator, false, renderedChart.AsSecretData()); err != nil {
		return fmt.Errorf("failed creating ManagedResource: %w", err)
	}

	if err := managedresources.WaitUntilHealthyAndNotProgressing(ctx, d.runtimeClientSet.Client(), d.gardenNamespace, admissionVirtualManagedResourceName(extension)); err != nil {
		return fmt.Errorf("failed waiting for ManagedResource to be healthy: %w", err)
	}
	return nil
}

func (d *deployment) deleteAdmissionVirtualClusterResources(ctx context.Context, log logr.Logger, extension *operatorv1alpha1.Extension) error {
	managedResourceName := admissionVirtualManagedResourceName(extension)

	log.Info("Deleting admission ManagedResource for virtual cluster", "managedResource", client.ObjectKey{Name: managedResourceName, Namespace: d.gardenNamespace})
	if err := managedresources.DeleteForShoot(ctx, d.runtimeClientSet.Client(), d.gardenNamespace, managedResourceName); err != nil {
		return fmt.Errorf("failed deleting ManagedResource: %w", err)
	}

	return managedresources.WaitUntilDeleted(ctx, d.runtimeClientSet.Client(), d.gardenNamespace, admissionVirtualManagedResourceName(extension))
}

func (d *deployment) getVirtualClusterAccessSecret(name string) *gardenerutils.AccessSecret {
	return gardenerutils.NewShootAccessSecret(name, d.gardenNamespace)
}

func runtimeDeploymentSpecified(extension *operatorv1alpha1.Extension) bool {
	return extension.Spec.Deployment != nil &&
		extension.Spec.Deployment.AdmissionDeployment != nil &&
		extension.Spec.Deployment.AdmissionDeployment.RuntimeCluster != nil &&
		extension.Spec.Deployment.AdmissionDeployment.RuntimeCluster.Helm != nil
}

func virtualDeploymentSpecified(extension *operatorv1alpha1.Extension) bool {
	return extension.Spec.Deployment != nil &&
		extension.Spec.Deployment.AdmissionDeployment != nil &&
		extension.Spec.Deployment.AdmissionDeployment.VirtualCluster != nil &&
		extension.Spec.Deployment.AdmissionDeployment.VirtualCluster.Helm != nil
}

// New creates a new admission deployer.
func New(runtimeClientSet kubernetes.Interface, recorder record.EventRecorder, gardenNamespace string, registry oci.Interface) Interface {
	return &deployment{
		runtimeClientSet: runtimeClientSet,
		recorder:         recorder,
		gardenNamespace:  gardenNamespace,
		helmRegistry:     registry,
	}
}

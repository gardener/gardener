// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package extension

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
	"sigs.k8s.io/controller-runtime/pkg/client"

	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	operatorv1alpha1 "github.com/gardener/gardener/pkg/apis/operator/v1alpha1"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	"github.com/gardener/gardener/pkg/utils"
	gardenerutils "github.com/gardener/gardener/pkg/utils/gardener"
	kubernetesutils "github.com/gardener/gardener/pkg/utils/kubernetes"
	"github.com/gardener/gardener/pkg/utils/managedresources"
)

func (r *Reconciler) reconcileAdmissionDeployment(ctx context.Context, log logr.Logger, virtualClusterClientSet kubernetes.Interface, genericTokenKubeconfigSecretName string, extension *operatorv1alpha1.Extension) error {
	if extension.Spec.Deployment == nil ||
		extension.Spec.Deployment.AdmissionDeployment == nil ||
		extension.Spec.Deployment.AdmissionDeployment.RuntimeCluster == nil ||
		extension.Spec.Deployment.AdmissionDeployment.RuntimeCluster.Helm == nil {
		if err := r.deleteAdmissionDeployment(ctx, log, extension); err != nil {
			return err
		}
		r.Recorder.Event(extension, corev1.EventTypeNormal, "Deletion", "Admission deployment deleted successfully")

		return nil
	}

	log.Info("Reconciling admission virtual garden resources")
	if err := r.createOrUpdateAdmissionVirtualClusterResources(ctx, log, virtualClusterClientSet, extension); err != nil {
		return err
	}

	log.Info("Reconciling admission runtime resources")
	if err := r.createOrUpdateAdmissionRuntimeClusterResources(ctx, log, genericTokenKubeconfigSecretName, extension); err != nil {
		return err
	}
	r.Recorder.Event(extension, corev1.EventTypeNormal, "Reconciliation", "Admission deployment applied successfully")

	return nil
}

func (r *Reconciler) deleteAdmissionDeployment(ctx context.Context, log logr.Logger, extension *operatorv1alpha1.Extension) error {
	log.Info("Deleting admission deployment")
	if err := r.deleteAdmissionRuntimeClusterResources(ctx, log, extension); err != nil {
		return err
	}
	return r.deleteAdmissionVirtualClusterResources(ctx, log, extension)
}

func admissionResourceName(extension *operatorv1alpha1.Extension) string {
	return fmt.Sprintf("extension-admission-%s", extension.Name)
}

func admissionRuntimeManagedResourceName(extension *operatorv1alpha1.Extension) string {
	return fmt.Sprintf("extension-admission-runtime-%s", extension.Name)
}

func (r *Reconciler) createOrUpdateAdmissionRuntimeClusterResources(ctx context.Context, log logr.Logger, genericTokenKubeconfigSecretName string, extension *operatorv1alpha1.Extension) error {
	archive, err := r.HelmRegistry.Pull(ctx, extension.Spec.Deployment.AdmissionDeployment.RuntimeCluster.Helm.OCIRepository)
	if err != nil {
		return fmt.Errorf("failed pulling Helm chart from OCI repository: %w", err)
	}

	accessSecret := r.getVirtualClusterAccessSecret(admissionResourceName(extension))
	if err := accessSecret.Reconcile(ctx, r.RuntimeClientSet.Client()); err != nil {
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

	renderedChart, err := r.RuntimeClientSet.ChartRenderer().RenderArchive(archive, extension.Name, v1beta1constants.GardenNamespace, utils.MergeMaps(helmValues, gardenerValues))
	if err != nil {
		return fmt.Errorf("failed rendering Helm chart: %w", err)
	}

	secretData := renderedChart.AsSecretData()

	// Inject Kubeconfig for Garden cluster access.
	if err := gardenerutils.MutateObjectsInSecretData(
		secretData,
		r.GardenNamespace,
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
		r.RuntimeClientSet.Client(),
		r.GardenNamespace,
		admissionRuntimeManagedResourceName(extension),
		false,
		map[string]string{managedresources.LabelKeyOrigin: managedresources.LabelValueOperator},
		secretData,
	); err != nil {
		return fmt.Errorf("failed creating ManagedResource: %w", err)
	}

	if err := managedresources.WaitUntilHealthyAndNotProgressing(ctx, r.RuntimeClientSet.Client(), r.GardenNamespace, admissionRuntimeManagedResourceName(extension)); err != nil {
		return fmt.Errorf("failed waiting for ManagedResource to be healthy: %w", err)
	}

	r.Recorder.Event(extension, corev1.EventTypeNormal, "Reconciliation", "Admission Helm chart applied successfully to runtime cluster")

	return nil
}

func (r *Reconciler) deleteAdmissionRuntimeClusterResources(ctx context.Context, log logr.Logger, extension *operatorv1alpha1.Extension) error {
	managedResourceName := admissionRuntimeManagedResourceName(extension)

	log.Info("Deleting admission ManagedResource for runtime cluster if present", "managedResource", client.ObjectKey{Name: managedResourceName, Namespace: r.GardenNamespace})
	if err := managedresources.DeleteForSeed(ctx, r.RuntimeClientSet.Client(), r.GardenNamespace, managedResourceName); err != nil {
		return fmt.Errorf("failed deleting ManagedResource: %w", err)
	}

	if err := managedresources.WaitUntilDeleted(ctx, r.RuntimeClientSet.Client(), r.GardenNamespace, admissionRuntimeManagedResourceName(extension)); err != nil {
		return fmt.Errorf("failed waiting for ManagedResource to be deleted: %w", err)
	}

	accessSecret := r.getVirtualClusterAccessSecret(admissionResourceName(extension)).Secret

	log.Info("Deleting admission access secret for virtual cluster", "secret", client.ObjectKeyFromObject(accessSecret))
	return kubernetesutils.DeleteObjects(ctx, r.RuntimeClientSet.Client(), accessSecret)
}

func admissionVirtualManagedResourceName(extension *operatorv1alpha1.Extension) string {
	return fmt.Sprintf("extension-admission-virtual-%s", extension.Name)
}

func (r *Reconciler) createOrUpdateAdmissionVirtualClusterResources(ctx context.Context, log logr.Logger, virtualClusterClientSet kubernetes.Interface, extension *operatorv1alpha1.Extension) error {
	archive, err := r.HelmRegistry.Pull(ctx, extension.Spec.Deployment.AdmissionDeployment.VirtualCluster.Helm.OCIRepository)
	if err != nil {
		return fmt.Errorf("failed pulling Helm chart from OCI repository: %w", err)
	}

	accessSecret := r.getVirtualClusterAccessSecret(admissionResourceName(extension))

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

	if err := managedresources.CreateForShoot(ctx, r.RuntimeClientSet.Client(), r.GardenNamespace, admissionVirtualManagedResourceName(extension), managedresources.LabelValueOperator, false, renderedChart.AsSecretData()); err != nil {
		return fmt.Errorf("failed creating ManagedResource: %w", err)
	}

	if err := managedresources.WaitUntilHealthyAndNotProgressing(ctx, r.RuntimeClientSet.Client(), r.GardenNamespace, admissionVirtualManagedResourceName(extension)); err != nil {
		return fmt.Errorf("failed waiting for ManagedResource to be healthy: %w", err)
	}

	r.Recorder.Event(extension, corev1.EventTypeNormal, "Reconciliation", "Admission Helm chart applied successfully to virtual cluster")

	return nil
}

func (r *Reconciler) deleteAdmissionVirtualClusterResources(ctx context.Context, log logr.Logger, extension *operatorv1alpha1.Extension) error {
	managedResourceName := admissionVirtualManagedResourceName(extension)

	log.Info("Deleting admission ManagedResource for virtual cluster", "managedResource", client.ObjectKey{Name: managedResourceName, Namespace: r.GardenNamespace})
	if err := managedresources.DeleteForShoot(ctx, r.RuntimeClientSet.Client(), r.GardenNamespace, managedResourceName); err != nil {
		return fmt.Errorf("failed deleting ManagedResource: %w", err)
	}

	return managedresources.WaitUntilDeleted(ctx, r.RuntimeClientSet.Client(), r.GardenNamespace, admissionVirtualManagedResourceName(extension))
}

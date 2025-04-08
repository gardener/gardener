// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package runtime

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/tools/record"
	podsecurityadmissionapi "k8s.io/pod-security-admission/api"
	"sigs.k8s.io/controller-runtime/pkg/client"

	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	operatorv1alpha1 "github.com/gardener/gardener/pkg/apis/operator/v1alpha1"
	resourcesv1alpha1 "github.com/gardener/gardener/pkg/apis/resources/v1alpha1"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	"github.com/gardener/gardener/pkg/controllerutils"
	"github.com/gardener/gardener/pkg/utils"
	"github.com/gardener/gardener/pkg/utils/gardener"
	kubernetesutils "github.com/gardener/gardener/pkg/utils/kubernetes"
	"github.com/gardener/gardener/pkg/utils/managedresources"
	"github.com/gardener/gardener/pkg/utils/oci"
)

// Interface contains functions for the extension deployer in the garden runtime cluster.
type Interface interface {
	// Reconcile creates or updates the extension deployment in the garden runtime cluster.
	Reconcile(context.Context, logr.Logger, *operatorv1alpha1.Extension) error
	// Delete deletes the extension deployment in the garden runtime cluster.
	Delete(context.Context, logr.Logger, *operatorv1alpha1.Extension) error
}

type deployer struct {
	runtimeClientSet kubernetes.Interface
	recorder         record.EventRecorder

	gardenNamespace string
	helmRegistry    oci.Interface
}

// Reconcile creates or updates the extension deployment in the garden runtime cluster.
func (d *deployer) Reconcile(ctx context.Context, log logr.Logger, extension *operatorv1alpha1.Extension) error {
	if !extensionDeploymentSpecified(extension) {
		return d.Delete(ctx, log, extension)
	}

	if err := d.createOrUpdateResources(ctx, extension); err != nil {
		return err
	}
	d.recorder.Event(extension, corev1.EventTypeNormal, "Reconciliation", "Extension applied successfully in runtime cluster")
	return nil
}

// Delete deletes the extension deployment in the garden runtime cluster.
func (d *deployer) Delete(ctx context.Context, log logr.Logger, extension *operatorv1alpha1.Extension) error {
	log.Info("Deleting extension resources in garden runtime cluster")
	if err := d.deleteResources(ctx, log, extension); err != nil {
		return err
	}

	d.recorder.Event(extension, corev1.EventTypeNormal, "Deletion", "Extension deployment deleted successfully in runtime cluster")
	return nil
}

func (d *deployer) createOrUpdateResources(ctx context.Context, extension *operatorv1alpha1.Extension) error {
	archive, err := d.helmRegistry.Pull(ctx, extension.Spec.Deployment.ExtensionDeployment.Helm.OCIRepository)
	if err != nil {
		return fmt.Errorf("failed pulling Helm chart from OCI repository %q: %w", extension.Spec.Deployment.ExtensionDeployment.Helm.OCIRepository.GetURL(), err)
	}

	gardenerValues := map[string]any{
		"gardener": map[string]any{
			"runtimeCluster": map[string]any{
				"enabled":           "true",
				"priorityClassName": v1beta1constants.PriorityClassNameGardenSystem200,
			},
		},
	}

	var helmValues map[string]any
	if extension.Spec.Deployment.ExtensionDeployment.RuntimeClusterValues != nil {
		if err := json.Unmarshal(extension.Spec.Deployment.ExtensionDeployment.RuntimeClusterValues.Raw, &helmValues); err != nil {
			return err
		}
	}

	namespace := gardener.ExtensionRuntimeNamespaceName(extension.Name)
	if err := d.ensureNamespace(ctx, namespace, extension); err != nil {
		return fmt.Errorf("failed ensuring namespace %q: %w", namespace, err)
	}

	renderedChart, err := d.runtimeClientSet.ChartRenderer().RenderArchive(archive, extension.Name, namespace, utils.MergeMaps(helmValues, gardenerValues))
	if err != nil {
		return fmt.Errorf("failed rendering Helm chart %q: %w", extension.Spec.Deployment.ExtensionDeployment.Helm.OCIRepository.GetURL(), err)
	}

	mrName := gardener.ExtensionRuntimeManagedResourceName(extension.Name)
	if err := managedresources.CreateForSeed(ctx, d.runtimeClientSet.Client(), d.gardenNamespace, mrName, false, renderedChart.AsSecretData()); err != nil {
		return fmt.Errorf("failed creating ManagedResource: %w", err)
	}

	if err := managedresources.WaitUntilHealthyAndNotProgressing(ctx, d.runtimeClientSet.Client(), d.gardenNamespace, mrName); err != nil {
		return fmt.Errorf("failed waiting for ManagedResource to be healthy: %w", err)
	}
	return nil
}

func (d *deployer) deleteResources(ctx context.Context, log logr.Logger, extension *operatorv1alpha1.Extension) error {
	mrName := gardener.ExtensionRuntimeManagedResourceName(extension.Name)
	namespace := gardener.ExtensionRuntimeNamespaceName(extension.Name)

	log.Info("Deleting extension ManagedResource for runtime cluster if present", "managedResource", client.ObjectKey{Name: mrName, Namespace: d.gardenNamespace})
	if err := client.IgnoreNotFound(managedresources.DeleteForSeed(ctx, d.runtimeClientSet.Client(), d.gardenNamespace, mrName)); err != nil {
		return fmt.Errorf("failed deleting ManagedResource: %w", err)
	}

	if err := managedresources.WaitUntilDeleted(ctx, d.runtimeClientSet.Client(), d.gardenNamespace, mrName); err != nil {
		return fmt.Errorf("failed waiting for ManagedResource to be deleted: %w", err)
	}

	if err := d.deleteNamespace(ctx, namespace); err != nil {
		return fmt.Errorf("failed deleting namespace %q: %w", namespace, err)
	}

	return nil
}

func (d *deployer) ensureNamespace(ctx context.Context, name string, extension *operatorv1alpha1.Extension) error {
	gardenNamespace := &corev1.Namespace{}
	if err := d.runtimeClientSet.Client().Get(ctx, client.ObjectKey{Name: d.gardenNamespace}, gardenNamespace); err != nil {
		return err
	}
	namespace := &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
		},
	}
	if _, err := controllerutils.GetAndCreateOrMergePatch(ctx, d.runtimeClientSet.Client(), namespace, func() error {
		metav1.SetMetaDataLabel(&namespace.ObjectMeta, v1beta1constants.GardenRole, v1beta1constants.GardenRoleExtension)
		metav1.SetMetaDataLabel(&namespace.ObjectMeta, resourcesv1alpha1.HighAvailabilityConfigConsider, "true")
		metav1.SetMetaDataLabel(&namespace.ObjectMeta, v1beta1constants.LabelNetworkPolicyAccessTargetAPIServer, "allowed")

		if zones := gardenNamespace.Annotations[resourcesv1alpha1.HighAvailabilityConfigZones]; zones != "" {
			metav1.SetMetaDataAnnotation(&namespace.ObjectMeta, resourcesv1alpha1.HighAvailabilityConfigZones, zones)
		} else {
			delete(namespace.Annotations, resourcesv1alpha1.HighAvailabilityConfigZones)
		}

		if podSecurityEnforce, ok := extension.Annotations[v1beta1constants.AnnotationPodSecurityEnforce]; ok {
			metav1.SetMetaDataLabel(&namespace.ObjectMeta, podsecurityadmissionapi.EnforceLevelLabel, podSecurityEnforce)
		} else {
			delete(namespace.Labels, podsecurityadmissionapi.EnforceLevelLabel)
		}

		return nil
	}); err != nil {
		return err
	}
	return nil
}

func (d *deployer) deleteNamespace(ctx context.Context, name string) error {
	namespace := &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
		},
	}
	if err := client.IgnoreNotFound(d.runtimeClientSet.Client().Delete(ctx, namespace)); err != nil {
		return err
	}
	return kubernetesutils.WaitUntilResourceDeleted(ctx, d.runtimeClientSet.Client(), namespace, time.Second)
}

func extensionDeploymentSpecified(extension *operatorv1alpha1.Extension) bool {
	return extension.Spec.Deployment != nil &&
		extension.Spec.Deployment.ExtensionDeployment != nil &&
		extension.Spec.Deployment.ExtensionDeployment.Helm != nil
}

// New creates a new runtime deployer.
func New(runtimeClientSet kubernetes.Interface, recorder record.EventRecorder, gardenNamespace string, registry oci.Interface) Interface {
	return &deployer{
		runtimeClientSet: runtimeClientSet,
		recorder:         recorder,
		gardenNamespace:  gardenNamespace,
		helmRegistry:     registry,
	}
}

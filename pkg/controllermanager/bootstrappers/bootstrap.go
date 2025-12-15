// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package bootstrappers

import (
	"context"
	"fmt"
	"strconv"
	"time"

	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/discovery"
	kubernetesclientset "k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/utils/clock"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	gardenerutils "github.com/gardener/gardener/pkg/utils/gardener"
	secretsutils "github.com/gardener/gardener/pkg/utils/secrets"
	secretsmanager "github.com/gardener/gardener/pkg/utils/secrets/manager"
	"github.com/gardener/gardener/pkg/utils/version"
)

const (
	// ConfigMapsPerShootAnnotation is the annotation key for the amount of ConfigMaps created by Gardener in the project namespace.
	ConfigMapsPerShootAnnotation = "gardener.cloud/configmaps-per-shoot"
	// SecretsPerShootAnnotation is the annotation key for the amount of Secrets created by Gardener in the project namespace.
	SecretsPerShootAnnotation = "gardener.cloud/secrets-per-shoot"
	shootCountResource        = "count/shoots.core.gardener.cloud"
)

// ResourceQuotaUsages describes the resource quota usages per shoot cluster in project namespaces.
type ResourceQuotaUsages struct {
	Annotation       string
	QuotaKey         corev1.ResourceName
	ExpectedPerShoot int
}

var (
	// PerShootQuotaDescriptors describes resources that Gardener creates per Shoot Cluster in the project namespace.
	// Exposed for testing.
	PerShootQuotaDescriptors = []ResourceQuotaUsages{
		{
			ConfigMapsPerShootAnnotation,
			"count/configmaps",
			len(gardenerutils.GetShootProjectConfigMapSuffixes()),
		},
		{
			SecretsPerShootAnnotation,
			"count/secrets",
			len(gardenerutils.GetShootProjectSecretSuffixes()),
		},
	}
)

// Bootstrapper is a runnable for bootstrapping the garden cluster.
type Bootstrapper struct {
	Log        logr.Logger
	Client     client.Client
	RESTConfig *rest.Config
}

// Start runs as soon as the manager got leader.
func (b *Bootstrapper) Start(parentCtx context.Context) error {
	// Other controllers depend on garden cluster bootstrapping.
	// Hence, if we can't bootstrap the garden cluster in a short timeout, terminate and try again after restart.
	ctx, cancel := context.WithTimeout(parentCtx, time.Minute)
	defer cancel()

	kubernetesClient, err := kubernetesclientset.NewForConfig(b.RESTConfig)
	if err != nil {
		return fmt.Errorf("failed creating kubernetes client: %w", err)
	}

	secretsManager, err := secretsmanager.New(ctx, b.Log.WithName("secretsmanager"), clock.RealClock{}, b.Client, v1beta1constants.SecretManagerIdentityControllerManager, secretsmanager.Config{}, v1beta1constants.GardenNamespace)
	if err != nil {
		return fmt.Errorf("failed creating new secrets manager: %w", err)
	}

	if err := bootstrapCluster(ctx, b.Client, kubernetesClient.Discovery(), secretsManager); err != nil {
		return fmt.Errorf("failed bootstrapping garden cluster: %w", err)
	}

	if err := secretsManager.Cleanup(ctx); err != nil {
		return fmt.Errorf("failed cleaning up no longer required secrets: %w", err)
	}

	if err := b.bumpProjectResourceQuotas(ctx); err != nil {
		return fmt.Errorf("failed bumping project resource quotas: %w", err)
	}

	b.Log.Info("Successfully bootstrapped Garden cluster")
	return nil
}

func (b *Bootstrapper) getProjectNamespaces(ctx context.Context) (*metav1.PartialObjectMetadataList, error) {
	projectNamespaces := &metav1.PartialObjectMetadataList{}
	projectNamespaces.SetGroupVersionKind(corev1.SchemeGroupVersion.WithKind("Namespace"))
	if err := b.Client.List(ctx, projectNamespaces, client.MatchingLabels{v1beta1constants.GardenRole: v1beta1constants.GardenRoleProject}); err != nil {
		return nil, fmt.Errorf("failed listing project namespaces: %w", err)
	}
	return projectNamespaces, nil
}

func (b *Bootstrapper) getResourceQuotas(ctx context.Context, namespace string) (*corev1.ResourceQuotaList, error) {
	resourceQuotas := &corev1.ResourceQuotaList{}
	if err := b.Client.List(ctx, resourceQuotas, client.InNamespace(namespace)); err != nil {
		if !errors.IsNotFound(err) {
			return nil, fmt.Errorf("failed listing project resource quotas: %w", err)
		}
	}
	return resourceQuotas, nil
}

func (b *Bootstrapper) getMaximumShootsInProject(ctx context.Context, resourceQuota corev1.ResourceQuota, projectNamespace string) (int64, error) {
	var maximum int64
	if limit, hasQuota := resourceQuota.Spec.Hard[corev1.ResourceName(shootCountResource)]; hasQuota {
		maximum = limit.Value()
	} else {
		shootList := &metav1.PartialObjectMetadataList{}
		shootList.SetGroupVersionKind(gardencorev1beta1.SchemeGroupVersion.WithKind("ShootList"))
		if err := b.Client.List(ctx, shootList, client.InNamespace(projectNamespace)); err != nil {
			return 0, fmt.Errorf("could not list shoots in project namespace %q: %w", projectNamespace, err)
		}
		maximum = int64(len(shootList.Items))
	}
	return maximum, nil
}

func (b *Bootstrapper) alignResourceQuota(ctx context.Context, log logr.Logger, resourceQuota corev1.ResourceQuota, projectNamespace string) error {
	if resourceQuota.DeletionTimestamp != nil {
		return nil
	}

	for _, resourceQuotaUsage := range PerShootQuotaDescriptors {
		usageAnnotation := resourceQuotaUsage.Annotation
		usageSpecKey := resourceQuotaUsage.QuotaKey
		expectedCount := resourceQuotaUsage.ExpectedPerShoot

		annotationCountString, ok := resourceQuota.Annotations[usageAnnotation]
		if !ok {
			if err := b.handleMissingAnnotation(ctx, log, &resourceQuota, usageAnnotation, usageSpecKey, expectedCount, projectNamespace); err != nil {
				return err
			}
			continue
		}

		annotationCount, err := strconv.Atoi(annotationCountString)
		if err != nil {
			return fmt.Errorf("failed converting resource quota annotation %q to int: %w", usageAnnotation, err)
		}

		if annotationCount != expectedCount {
			if err := b.handleAnnotationMismatch(ctx, log, &resourceQuota, usageAnnotation, usageSpecKey, annotationCount, expectedCount, projectNamespace); err != nil {
				return err
			}
		}
	}
	return nil
}

func (b *Bootstrapper) handleMissingAnnotation(ctx context.Context, log logr.Logger, resourceQuota *corev1.ResourceQuota, usageAnnotation string, usageSpecKey corev1.ResourceName, expectedCount int, projectNamespace string) error {
	maximumShootsInProject, err := b.getMaximumShootsInProject(ctx, *resourceQuota, projectNamespace)
	if err != nil {
		return err
	}

	currentQuotaResource, ok := resourceQuota.Spec.Hard[usageSpecKey]
	if !ok {
		return b.setResourceQuotaAnnotation(ctx, resourceQuota, usageAnnotation, strconv.Itoa(expectedCount))
	}

	currentQuota := ptr.To(currentQuotaResource).Value()
	requiredQuota := int64(expectedCount) * maximumShootsInProject

	if currentQuota < requiredQuota {
		log.Info("Current quota is less than required quota, bumping up", "currentQuota", currentQuota, "requiredQuota", requiredQuota, "quotaType", usageSpecKey.String())
		newVal := strconv.Itoa(int(requiredQuota))
		if err := b.updateResourceQuotaHard(ctx, resourceQuota, usageSpecKey, newVal); err != nil {
			return err
		}
	} else {
		log.Info("Current quota is sufficient for required quota, not changing quota", "currentQuota", currentQuota, "requiredQuota", requiredQuota, "quotaType", usageSpecKey.String())
	}

	return b.setResourceQuotaAnnotation(ctx, resourceQuota, usageAnnotation, strconv.Itoa(expectedCount))
}

func (b *Bootstrapper) handleAnnotationMismatch(ctx context.Context, log logr.Logger, resourceQuota *corev1.ResourceQuota, usageAnnotation string, usageSpecKey corev1.ResourceName, annotationCount, expectedCount int, projectNamespace string) error {
	log.Info("Bumping resource quota per shoot", "quotaType", usageSpecKey.String(), "from", annotationCount, "to", expectedCount)
	countDiff := int64(max(expectedCount-annotationCount, 0))
	if countDiff == 0 {
		return nil
	}

	maximum, err := b.getMaximumShootsInProject(ctx, *resourceQuota, projectNamespace)
	if err != nil {
		return err
	}

	newQuota := ptr.To(resourceQuota.Spec.Hard[usageSpecKey]).Value() + maximum*countDiff
	newVal := strconv.Itoa(int(newQuota))
	log.Info("Updating resource quota with value", "quotaType", usageSpecKey.String(), "value", newVal)
	if err := b.updateResourceQuotaHard(ctx, resourceQuota, usageSpecKey, newVal); err != nil {
		return err
	}

	return b.setResourceQuotaAnnotation(ctx, resourceQuota, usageAnnotation, strconv.Itoa(expectedCount))
}

func (b *Bootstrapper) updateResourceQuotaHard(ctx context.Context, resourceQuota *corev1.ResourceQuota, specKey corev1.ResourceName, newVal string) error {
	patch := client.MergeFrom(resourceQuota.DeepCopy())
	resourceQuota.Spec.Hard[specKey] = resource.MustParse(newVal)
	if err := b.Client.Patch(ctx, resourceQuota, patch); err != nil {
		return fmt.Errorf("failed updating resource quota %v: %w", resourceQuota, err)
	}
	return nil
}

func (b *Bootstrapper) setResourceQuotaAnnotation(ctx context.Context, resourceQuota *corev1.ResourceQuota, annotation string, value string) error {
	patch := client.MergeFrom(resourceQuota.DeepCopy())
	metav1.SetMetaDataAnnotation(&resourceQuota.ObjectMeta, annotation, value)
	if err := b.Client.Patch(ctx, resourceQuota, patch); err != nil {
		return fmt.Errorf("failed updating resource quota %v: %w", resourceQuota, err)
	}
	return nil
}

// bumpProjectResourceQuotas checks if the amount of resources per shoot clusters has increased and adapts ResourceQuotas in project namespaces
func (b *Bootstrapper) bumpProjectResourceQuotas(ctx context.Context) error {
	log := b.Log.WithName("bumpQuota")

	projectNamespaces, err := b.getProjectNamespaces(ctx)
	if err != nil {
		return err
	}
	log.Info("Found project namespaces", "count", len(projectNamespaces.Items))

	for _, projectNamespace := range projectNamespaces.Items {
		resourceQuotas, err := b.getResourceQuotas(ctx, projectNamespace.Name)
		if err != nil {
			return err
		}
		log.Info("Found project resource quotas in namespace", "count", len(resourceQuotas.Items), "namespace", projectNamespace.Name)

		for _, resourceQuota := range resourceQuotas.Items {
			err := b.alignResourceQuota(ctx, log, resourceQuota, projectNamespace.Name)
			if err != nil {
				return err
			}
		}
	}
	return nil
}

func bootstrapCluster(ctx context.Context, gardenClient client.Client, discoveryClient discovery.DiscoveryInterface, secretsManager secretsmanager.Interface) error {
	const minKubernetesVersion = "1.30"

	serverVersion, err := discoveryClient.ServerVersion()
	if err != nil {
		return fmt.Errorf("failed discovering garden cluster kubernetes version: %w", err)
	}

	gardenVersionOK, err := version.CompareVersions(serverVersion.GitVersion, ">=", minKubernetesVersion)
	if err != nil {
		return err
	}
	if !gardenVersionOK {
		return fmt.Errorf("the Kubernetes version of the Garden cluster must be at least %s", minKubernetesVersion)
	}

	secretList := &corev1.SecretList{}
	if err := gardenClient.List(
		ctx,
		secretList,
		client.InNamespace(v1beta1constants.GardenNamespace),
		client.MatchingLabels{v1beta1constants.GardenRole: v1beta1constants.GardenRoleGlobalMonitoring},
	); err != nil {
		return err
	}

	mustGenerateMonitoringSecret := true
	for _, s := range secretList.Items {
		managedBySecretsManager := s.Labels[secretsmanager.LabelKeyManagedBy] == secretsmanager.LabelValueSecretsManager &&
			s.Labels[secretsmanager.LabelKeyManagerIdentity] == v1beta1constants.SecretManagerIdentityControllerManager

		if !managedBySecretsManager {
			// found a custom monitoring secret managed by a human operator
			// keep it and don't take over responsibility for the monitoring secret
			mustGenerateMonitoringSecret = false
			break
		}
	}

	// we don't want to override custom monitoring secret managed by a human operator
	// only take over responsibility over monitoring secret if we find the legacy secret created by GCM or a new one managed by SecretsManager
	if mustGenerateMonitoringSecret {
		if _, err = generateGlobalMonitoringSecret(ctx, gardenClient, secretsManager); err != nil {
			return err
		}
	}

	return nil
}

func generateGlobalMonitoringSecret(ctx context.Context, k8sGardenClient client.Client, secretsManager secretsmanager.Interface) (*corev1.Secret, error) {
	credentialsSecret, err := secretsManager.Generate(ctx, &secretsutils.BasicAuthSecretConfig{
		Name:           v1beta1constants.SecretNameObservabilityIngress,
		Format:         secretsutils.BasicAuthFormatNormal,
		Username:       "admin",
		PasswordLength: 32,
	})
	if err != nil {
		return nil, err
	}

	patch := client.MergeFrom(credentialsSecret.DeepCopy())
	metav1.SetMetaDataLabel(&credentialsSecret.ObjectMeta, v1beta1constants.GardenRole, v1beta1constants.GardenRoleGlobalMonitoring)
	return credentialsSecret, k8sGardenClient.Patch(ctx, credentialsSecret, patch)
}

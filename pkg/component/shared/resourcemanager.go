// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package shared

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"strings"
	"time"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apiserver/pkg/authentication/user"
	"k8s.io/component-base/version"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/gardener/gardener/imagevector"
	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	v1beta1helper "github.com/gardener/gardener/pkg/apis/core/v1beta1/helper"
	resourcesv1alpha1 "github.com/gardener/gardener/pkg/apis/resources/v1alpha1"
	"github.com/gardener/gardener/pkg/component"
	"github.com/gardener/gardener/pkg/component/gardener/resourcemanager"
	"github.com/gardener/gardener/pkg/component/networking/nginxingress"
	resourcemanagerconfigv1alpha1 "github.com/gardener/gardener/pkg/resourcemanager/apis/config/v1alpha1"
	gardenerutils "github.com/gardener/gardener/pkg/utils/gardener"
	"github.com/gardener/gardener/pkg/utils/managedresources"
	retryutils "github.com/gardener/gardener/pkg/utils/retry"
	secretsutils "github.com/gardener/gardener/pkg/utils/secrets"
	secretsmanager "github.com/gardener/gardener/pkg/utils/secrets/manager"
)

// NewRuntimeGardenerResourceManager instantiates a new `gardener-resource-manager` component
// configured to reconcile objects in the runtime (seed) cluster.
func NewRuntimeGardenerResourceManager(
	c client.Client,
	gardenNamespaceName string,
	secretsManager secretsmanager.Interface,
	values resourcemanager.Values,
) (
	resourcemanager.Interface,
	error,
) {
	image, err := imagevector.Containers().FindImage(imagevector.ContainerImageNameGardenerResourceManager)
	if err != nil {
		return nil, err
	}
	image.WithOptionalTag(version.Get().GitVersion)

	defaultValues := resourcemanager.Values{
		ConcurrentSyncs:                      ptr.To(20),
		HealthSyncPeriod:                     &metav1.Duration{Duration: time.Minute},
		Image:                                image.String(),
		MaxConcurrentNetworkPolicyWorkers:    ptr.To(20),
		MaxConcurrentTokenInvalidatorWorkers: ptr.To(5),
		NetworkPolicyControllerIngressControllerSelector: &resourcemanagerconfigv1alpha1.IngressControllerSelector{
			Namespace: v1beta1constants.GardenNamespace,
			PodSelector: metav1.LabelSelector{MatchLabels: map[string]string{
				v1beta1constants.LabelApp:      nginxingress.LabelAppValue,
				nginxingress.LabelKeyComponent: nginxingress.LabelValueController,
			}},
		},
		// TODO(timuthy): Remove PodTopologySpreadConstraints webhook once for all seeds the
		//  MatchLabelKeysInPodTopologySpread feature gate is beta and enabled by default (probably 1.26+).
		PodTopologySpreadConstraintsEnabled: true,
		Replicas:                            ptr.To[int32](2),
		ResourceClass:                       ptr.To(v1beta1constants.SeedResourceManagerClass),
		ResponsibilityMode:                  resourcemanager.ForSource,
	}

	applyDefaults(&values, defaultValues)
	return resourcemanager.New(c, gardenNamespaceName, secretsManager, values), nil
}

// NewTargetGardenerResourceManager instantiates a new `gardener-resource-manager` component
// configured to reconcile object in a target (shoot) cluster.
func NewTargetGardenerResourceManager(
	c client.Client,
	namespaceName string,
	secretsManager secretsmanager.Interface,
	values resourcemanager.Values,
) (
	resourcemanager.Interface,
	error,
) {
	image, err := imagevector.Containers().FindImage(imagevector.ContainerImageNameGardenerResourceManager)
	if err != nil {
		return nil, err
	}
	image.WithOptionalTag(version.Get().GitVersion)

	defaultValues := resourcemanager.Values{
		AlwaysUpdate:                         ptr.To(true),
		ConcurrentSyncs:                      ptr.To(20),
		HealthSyncPeriod:                     &metav1.Duration{Duration: time.Minute},
		Image:                                image.String(),
		MaxConcurrentCSRApproverWorkers:      ptr.To(5),
		MaxConcurrentHealthWorkers:           ptr.To(10),
		MaxConcurrentTokenInvalidatorWorkers: ptr.To(5),
		MaxConcurrentTokenRequestorWorkers:   ptr.To(5),
		ResponsibilityMode:                   resourcemanager.ForTarget,
		WatchedNamespace:                     &namespaceName,
	}

	applyDefaults(&values, defaultValues)
	return resourcemanager.New(c, namespaceName, secretsManager, values), nil
}

var (
	// TimeoutWaitForGardenerResourceManagerBootstrapping is the maximum time the bootstrap process for the
	// gardener-resource-manager may take.
	// Exposed for testing.
	TimeoutWaitForGardenerResourceManagerBootstrapping = 2 * time.Minute
	// IntervalWaitForGardenerResourceManagerBootstrapping is the interval how often it's checked whether the bootstrap
	// process for the gardener-resource-manager has completed.
	// Exposed for testing.
	IntervalWaitForGardenerResourceManagerBootstrapping = 5 * time.Second
)

// DeployGardenerResourceManager deploys the gardener-resource-manager
func DeployGardenerResourceManager(
	ctx context.Context,
	c client.Client,
	secretsManager secretsmanager.Interface,
	gardenerResourceManager resourcemanager.Interface,
	namespace string,
	determineReplicas func(ctx context.Context) (int32, error),
	getAPIServerAddress func() string,
) error {
	var secrets resourcemanager.Secrets

	if gardenerResourceManager.GetReplicas() == nil {
		replicaCount, err := determineReplicas(ctx)
		if err != nil {
			return err
		}
		gardenerResourceManager.SetReplicas(&replicaCount)
	}

	mustBootstrap, err := mustBootstrapGardenerResourceManager(ctx, c, gardenerResourceManager, namespace)
	if err != nil {
		return err
	}

	if mustBootstrap {
		bootstrapKubeconfigSecret, err := reconcileGardenerResourceManagerBootstrapKubeconfigSecret(
			ctx,
			secretsManager,
			namespace,
			getAPIServerAddress,
		)
		if err != nil {
			return err
		}

		secrets.BootstrapKubeconfig = &component.Secret{Name: bootstrapKubeconfigSecret.Name}
		gardenerResourceManager.SetSecrets(secrets)

		if err := gardenerResourceManager.Deploy(ctx); err != nil {
			return err
		}

		timeoutCtx, cancel := context.WithTimeout(ctx, TimeoutWaitForGardenerResourceManagerBootstrapping)
		defer cancel()

		if err := waitUntilGardenerResourceManagerBootstrapped(timeoutCtx, c, namespace); err != nil {
			return err
		}

		if err := c.Delete(ctx, bootstrapKubeconfigSecret); client.IgnoreNotFound(err) != nil {
			return err
		}
	}

	secrets.BootstrapKubeconfig = nil
	gardenerResourceManager.SetSecrets(secrets)

	return gardenerResourceManager.Deploy(ctx)
}

func mustBootstrapGardenerResourceManager(ctx context.Context, c client.Client, gardenerResourceManager resourcemanager.Interface, namespace string) (bool, error) {
	if ptr.Deref(gardenerResourceManager.GetReplicas(), 0) == 0 {
		return false, nil // GRM should not be scaled up, hence no need to bootstrap.
	}

	shootAccessSecret := gardenerutils.NewShootAccessSecret(resourcemanager.SecretNameShootAccess, namespace)
	if err := c.Get(ctx, client.ObjectKeyFromObject(shootAccessSecret.Secret), shootAccessSecret.Secret); err != nil {
		if !apierrors.IsNotFound(err) {
			return false, err
		}
		return true, nil // Shoot access secret does not yet exist.
	}

	renewTimestamp, ok := shootAccessSecret.Secret.Annotations[resourcesv1alpha1.ServiceAccountTokenRenewTimestamp]
	if !ok {
		return true, nil // Shoot access secret was never reconciled yet
	}

	renewTime, err2 := time.Parse(time.RFC3339, renewTimestamp)
	if err2 != nil {
		return false, fmt.Errorf("could not parse renew timestamp: %w", err2)
	}
	if time.Now().UTC().After(renewTime.UTC()) {
		return true, nil // Shoot token was not renewed.
	}

	managedResource := &resourcesv1alpha1.ManagedResource{
		ObjectMeta: metav1.ObjectMeta{
			Name:      resourcemanager.ManagedResourceName,
			Namespace: namespace,
		},
	}

	if err := c.Get(ctx, client.ObjectKeyFromObject(managedResource), managedResource); err != nil {
		if !apierrors.IsNotFound(err) {
			return false, err
		}
		return true, nil // ManagedResource (containing the RBAC resources) does not yet exist.
	}

	if conditionApplied := v1beta1helper.GetCondition(managedResource.Status.Conditions, resourcesv1alpha1.ResourcesApplied); conditionApplied != nil &&
		conditionApplied.Status == gardencorev1beta1.ConditionFalse &&
		(strings.Contains(conditionApplied.Message, `forbidden: User "system:serviceaccount:kube-system:gardener-resource-manager" cannot`) ||
			strings.Contains(conditionApplied.Message, ": Unauthorized")) {
		return true, nil // ServiceAccount lost access.
	}

	return false, nil
}

func reconcileGardenerResourceManagerBootstrapKubeconfigSecret(ctx context.Context, secretsManager secretsmanager.Interface, namespace string, getAPIServerAddress func() string) (*corev1.Secret, error) {
	caBundleSecret, found := secretsManager.Get(v1beta1constants.SecretNameCACluster)
	if !found {
		return nil, fmt.Errorf("secret %q not found", v1beta1constants.SecretNameCACluster)
	}

	return secretsManager.Generate(ctx, &secretsutils.ControlPlaneSecretConfig{
		Name: resourcemanager.SecretNameShootAccess + "-bootstrap",
		CertificateSecretConfig: &secretsutils.CertificateSecretConfig{
			CommonName:                  "gardener.cloud:system:gardener-resource-manager",
			Organization:                []string{user.SystemPrivilegedGroup},
			CertType:                    secretsutils.ClientCert,
			Validity:                    ptr.To(10 * time.Minute),
			SkipPublishingCACertificate: true,
		},
		KubeConfigRequests: []secretsutils.KubeConfigRequest{{
			ClusterName:   namespace,
			APIServerHost: getAPIServerAddress(),
			CAData:        caBundleSecret.Data[secretsutils.DataKeyCertificateBundle],
		}},
	}, secretsmanager.SignedByCA(v1beta1constants.SecretNameCAClient))
}

func waitUntilGardenerResourceManagerBootstrapped(ctx context.Context, c client.Client, namespace string) error {
	shootAccessSecret := gardenerutils.NewShootAccessSecret(resourcemanager.SecretNameShootAccess, namespace)

	if err := retryutils.Until(ctx, IntervalWaitForGardenerResourceManagerBootstrapping, func(ctx context.Context) (bool, error) {
		if err2 := c.Get(ctx, client.ObjectKeyFromObject(shootAccessSecret.Secret), shootAccessSecret.Secret); err2 != nil {
			if apierrors.IsNotFound(err2) {
				return retryutils.MinorError(err2)
			}
			return retryutils.SevereError(err2)
		}

		renewTimestamp, ok := shootAccessSecret.Secret.Annotations[resourcesv1alpha1.ServiceAccountTokenRenewTimestamp]
		if !ok {
			return retryutils.MinorError(errors.New("token not yet generated"))
		}

		renewTime, err2 := time.Parse(time.RFC3339, renewTimestamp)
		if err2 != nil {
			return retryutils.SevereError(fmt.Errorf("could not parse renew timestamp: %w", err2))
		}

		if time.Now().UTC().After(renewTime.UTC()) {
			return retryutils.MinorError(errors.New("token not yet renewed"))
		}

		return retryutils.Ok()
	}); err != nil {
		return err
	}

	return managedresources.WaitUntilHealthy(ctx, c, namespace, resourcemanager.ManagedResourceName)
}

func applyDefaults(userValues *resourcemanager.Values, defaultValues resourcemanager.Values) {
	var (
		vUser    = reflect.ValueOf(userValues).Elem()
		vDefault = reflect.ValueOf(defaultValues)
	)

	for i := 0; i < vUser.NumField(); i++ {
		userField := vUser.Field(i)
		defaultField := vDefault.Field(i)

		if userField.IsZero() {
			userField.Set(defaultField)
		}
	}
}

// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package gardener

import (
	"context"
	"fmt"
	"strings"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/sets"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/gardener/gardener/pkg/api/core/v1beta1/helper"
	"github.com/gardener/gardener/pkg/apis/core"
	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
	seedmanagementv1alpha1 "github.com/gardener/gardener/pkg/apis/seedmanagement/v1alpha1"
)

const (
	// SeedNamespaceNamePrefix is the prefix used for seed namespaces.
	SeedNamespaceNamePrefix = "seed-"
)

// ComputeGardenNamespace returns the name of the namespace belonging to the given seed in the Garden cluster.
func ComputeGardenNamespace(seedName string) string {
	return SeedNamespaceNamePrefix + seedName
}

// ComputeSeedName computes the name of the seed out of the seed namespace in the Garden cluster.
func ComputeSeedName(seedNamespaceName string) string {
	seedName := strings.TrimPrefix(seedNamespaceName, SeedNamespaceNamePrefix)
	if seedName == seedNamespaceName {
		return ""
	}
	return seedName
}

// GetWildcardCertificate gets the wildcard TLS certificate for the seed ingress domain.
// Nil is returned if no wildcard certificate is configured.
func GetWildcardCertificate(ctx context.Context, c client.Client) (*corev1.Secret, error) {
	return getWildcardCertificate(ctx, c, v1beta1constants.GardenNamespace, v1beta1constants.GardenRoleControlPlaneWildcardCert)
}

// getWildcardCertificate gets the wildcard TLS certificate for the ingress domain for the given role.
// Nil is returned if no wildcard certificate is configured.
func getWildcardCertificate(ctx context.Context, c client.Client, namespace, role string) (*corev1.Secret, error) {
	wildcardCerts := &corev1.SecretList{}
	if err := c.List(
		ctx,
		wildcardCerts,
		client.InNamespace(namespace),
		client.MatchingLabels{v1beta1constants.GardenRole: role},
	); err != nil {
		return nil, err
	}

	if len(wildcardCerts.Items) > 1 {
		return nil, fmt.Errorf("misconfigured cluster: not possible to provide more than one secret with label %s=%s", v1beta1constants.GardenRole, role)
	}

	if len(wildcardCerts.Items) == 1 {
		return &wildcardCerts.Items[0], nil
	}
	return nil, nil
}

// ComputeRequiredExtensionsForSeed computes the extension kind/type combinations that are required for the
// seed reconciliation flow.
func ComputeRequiredExtensionsForSeed(seed *gardencorev1beta1.Seed, controllerRegistrationList *gardencorev1beta1.ControllerRegistrationList) sets.Set[string] {
	wantedKindTypeCombinations := sets.New[string]()

	if seed.Spec.DNS.Provider != nil {
		wantedKindTypeCombinations.Insert(ExtensionsID(extensionsv1alpha1.DNSRecordResource, seed.Spec.DNS.Provider.Type))
	}

	for enabledExtensionType := range ComputeEnabledTypesForKindExtensionSeed(seed, controllerRegistrationList) {
		wantedKindTypeCombinations.Insert(ExtensionsID(extensionsv1alpha1.ExtensionResource, enabledExtensionType))
	}

	// add extension combinations for seed provider type
	wantedKindTypeCombinations.Insert(ExtensionsID(extensionsv1alpha1.ControlPlaneResource, seed.Spec.Provider.Type))
	wantedKindTypeCombinations.Insert(ExtensionsID(extensionsv1alpha1.InfrastructureResource, seed.Spec.Provider.Type))
	wantedKindTypeCombinations.Insert(ExtensionsID(extensionsv1alpha1.WorkerResource, seed.Spec.Provider.Type))

	return wantedKindTypeCombinations
}

// ComputeEnabledTypesForKindExtensionSeed computes the enabled extension types for a given Seed and ControllerRegistrationList.
// It considers extensions explicitly enabled or disabled in the Seed specification and those automatically enabled
// based on the ControllerRegistration resources.
func ComputeEnabledTypesForKindExtensionSeed(seed *gardencorev1beta1.Seed, controllerRegistrationList *gardencorev1beta1.ControllerRegistrationList) sets.Set[string] {
	return computeEnabledTypesForKindExtension(
		gardencorev1beta1.ClusterTypeSeed,
		seed.Spec.Extensions,
		controllerRegistrationList,
		nil,
	)
}

// ExtensionKindAndTypeForID returns the extension's type and kind based on the given ID.
func ExtensionKindAndTypeForID(extensionID string) (extensionKind string, extensionType string, err error) {
	split := strings.Split(extensionID, "/")
	if len(split) != 2 {
		return "", "", fmt.Errorf("unexpected required extension: %q", extensionID)
	}
	extensionKind, extensionType = split[0], split[1]
	return
}

// RequiredExtensionsReady checks if all required extensions for a seed exist and are ready.
func RequiredExtensionsReady(ctx context.Context, gardenClient client.Client, seedName string, requiredExtensions sets.Set[string]) error {
	controllerInstallationList := &gardencorev1beta1.ControllerInstallationList{}
	if err := gardenClient.List(ctx, controllerInstallationList, client.MatchingFields{
		core.SeedRefName: seedName,
	}); err != nil {
		return err
	}

	for _, controllerInstallation := range controllerInstallationList.Items {
		controllerRegistration := &gardencorev1beta1.ControllerRegistration{}
		if err := gardenClient.Get(ctx, client.ObjectKey{Name: controllerInstallation.Spec.RegistrationRef.Name}, controllerRegistration); err != nil {
			return err
		}

		for _, kindType := range requiredExtensions.UnsortedList() {
			extensionKind, extensionType, err := ExtensionKindAndTypeForID(kindType)
			if err != nil {
				return err
			}

			if helper.IsResourceSupported(controllerRegistration.Spec.Resources, extensionKind, extensionType) && helper.IsControllerInstallationSuccessful(controllerInstallation) {
				requiredExtensions.Delete(kindType)
			}
		}
	}

	if len(requiredExtensions) > 0 {
		return fmt.Errorf("extension controllers missing or unready: %+v", requiredExtensions)
	}

	return nil
}

// GetIPStackForSeed returns the value for the AnnotationKeyIPStack annotation based on the given seed.
// It falls back to IPv4 if no IP families are available.
func GetIPStackForSeed(seed *gardencorev1beta1.Seed) string {
	return getIPStackForFamilies(seed.Spec.Networks.IPFamilies)
}

// ClusterIsManagedByManagedSeed checks whether a ManagedSeed object exists for the given seed name. It treats Forbidden
// errors as NotFound because the SeedAuthorizer only grants access to ManagedSeeds related to this seed via the
// resource graph — for unmanaged seeds, no graph edge is present and the authorizer returns Forbidden.
func ClusterIsManagedByManagedSeed(ctx context.Context, gardenReader client.Reader, seedName string) (bool, error) {
	managedSeed := &seedmanagementv1alpha1.ManagedSeed{ObjectMeta: metav1.ObjectMeta{Name: seedName, Namespace: v1beta1constants.GardenNamespace}}
	if err := gardenReader.Get(ctx, client.ObjectKeyFromObject(managedSeed), managedSeed); err != nil {
		if apierrors.IsNotFound(err) || apierrors.IsForbidden(err) {
			return false, nil
		}
		return false, fmt.Errorf("failed checking whether seed %q is managed by a ManagedSeed: %w", seedName, err)
	}
	return true, nil
}

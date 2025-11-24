// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package gardener

import (
	"context"
	"fmt"
	"strings"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/gardener/gardener/pkg/apis/core"
	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	"github.com/gardener/gardener/pkg/apis/core/v1beta1/helper"
	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
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

	disabledExtensionTypes := sets.New[string]()
	for _, extension := range seed.Spec.Extensions {
		if ptr.Deref(extension.Disabled, false) {
			disabledExtensionTypes.Insert(extension.Type)
		} else {
			wantedKindTypeCombinations.Insert(ExtensionsID(extensionsv1alpha1.ExtensionResource, extension.Type))
		}
	}

	for _, controllerRegistration := range controllerRegistrationList.Items {
		for _, resource := range controllerRegistration.Spec.Resources {
			if extensionEnabledForCluster(gardencorev1beta1.ClusterTypeSeed, resource, disabledExtensionTypes) {
				wantedKindTypeCombinations.Insert(ExtensionsID(extensionsv1alpha1.ExtensionResource, resource.Type))
			}
		}
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

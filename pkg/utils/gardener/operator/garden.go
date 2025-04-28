// SPDX-FileCopyrightText: 2025 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package operator

import (
	"context"
	"fmt"
	"slices"
	"strings"

	"github.com/go-logr/logr"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/sets"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/gardener/gardener/pkg/apis/authentication"
	gardencore "github.com/gardener/gardener/pkg/apis/core"
	v1beta1helper "github.com/gardener/gardener/pkg/apis/core/v1beta1/helper"
	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
	"github.com/gardener/gardener/pkg/apis/operations"
	operatorv1alpha1 "github.com/gardener/gardener/pkg/apis/operator/v1alpha1"
	operatorv1alpha1helper "github.com/gardener/gardener/pkg/apis/operator/v1alpha1/helper"
	resourcesv1alpha1 "github.com/gardener/gardener/pkg/apis/resources/v1alpha1"
	"github.com/gardener/gardener/pkg/apis/security"
	"github.com/gardener/gardener/pkg/apis/seedmanagement"
	"github.com/gardener/gardener/pkg/apis/settings"
	"github.com/gardener/gardener/pkg/utils/gardener"
	"github.com/gardener/gardener/pkg/utils/kubernetes/health"
)

// IsServedByGardenerAPIServer returns true if the passed resources is served by the Gardener API Server.
func IsServedByGardenerAPIServer(resource string) bool {
	for _, groupName := range []string{
		authentication.GroupName,
		gardencore.GroupName,
		operations.GroupName,
		security.GroupName,
		settings.GroupName,
		seedmanagement.GroupName,
	} {
		if strings.HasSuffix(resource, groupName) {
			return true
		}
	}

	return false
}

// IsServedByKubeAPIServer returns true if the passed resources is served by the Kube API Server.
func IsServedByKubeAPIServer(resource string) bool {
	return !IsServedByGardenerAPIServer(resource)
}

// ComputeRequiredExtensionsForGarden computes the extension kind/type combinations that are required for the
// garden reconciliation flow.
func ComputeRequiredExtensionsForGarden(garden *operatorv1alpha1.Garden, extensionList *operatorv1alpha1.ExtensionList) sets.Set[string] {
	requiredExtensions := sets.New[string]()

	if operatorv1alpha1helper.GetETCDMainBackup(garden) != nil {
		requiredExtensions.Insert(gardener.ExtensionsID(extensionsv1alpha1.BackupBucketResource, garden.Spec.VirtualCluster.ETCD.Main.Backup.Provider))
	}

	for _, provider := range operatorv1alpha1helper.GetDNSProviders(garden) {
		requiredExtensions.Insert(gardener.ExtensionsID(extensionsv1alpha1.DNSRecordResource, provider.Type))
	}

	for _, extension := range garden.Spec.Extensions {
		requiredExtensions.Insert(gardener.ExtensionsID(extensionsv1alpha1.ExtensionResource, extension.Type))
	}

	for _, extension := range extensionList.Items {
		for _, resource := range extension.Spec.Resources {
			if resource.Kind == extensionsv1alpha1.ExtensionResource && slices.Contains(resource.AutoEnable, operatorv1alpha1.ClusterTypeGarden) {
				requiredExtensions.Insert(gardener.ExtensionsID(extensionsv1alpha1.ExtensionResource, resource.Type))
			}
		}
	}

	return requiredExtensions
}

// IsRuntimeExtensionInstallationSuccessful returns an error if an Extension is not marked as "successfully" in the Garden runtime cluster.
func IsRuntimeExtensionInstallationSuccessful(ctx context.Context, c client.Client, gardenNamespace, extensionName string) error {
	managedResource := &resourcesv1alpha1.ManagedResource{
		ObjectMeta: metav1.ObjectMeta{
			Name:      ExtensionRuntimeManagedResourceName(extensionName),
			Namespace: gardenNamespace,
		},
	}

	if err := c.Get(ctx, client.ObjectKeyFromObject(managedResource), managedResource); err != nil {
		return err
	}

	if err := health.CheckManagedResource(managedResource); err != nil {
		return err
	}

	return health.CheckManagedResourceProgressing(managedResource)
}

// RequiredGardenExtensionsReady checks if all required extensions for a garden exist and are ready.
func RequiredGardenExtensionsReady(ctx context.Context, log logr.Logger, c client.Client, gardenNamespace string, requiredExtensions sets.Set[string]) error {
	extensionList := &operatorv1alpha1.ExtensionList{}
	if err := c.List(ctx, extensionList); err != nil {
		return fmt.Errorf("failed to check if required extensions are ready: %w", err)
	}

	for _, extension := range extensionList.Items {
		var (
			extensionChecked  bool
			extensionCheckErr error
		)

		for _, kindType := range requiredExtensions.UnsortedList() {
			extensionKind, extensionType, err := gardener.ExtensionKindAndTypeForID(kindType)
			if err != nil {
				return fmt.Errorf("failed to check if required extensions are ready: %w", err)
			}

			if !v1beta1helper.IsResourceSupported(extension.Spec.Resources, extensionKind, extensionType) {
				continue
			}

			if !extensionChecked {
				extensionCheckErr = IsRuntimeExtensionInstallationSuccessful(ctx, c, gardenNamespace, extension.Name)
				extensionChecked = true
			}

			if extensionCheckErr != nil {
				log.Error(err, "Extension installation not successful", "kind", kindType)
				continue
			}

			requiredExtensions.Delete(kindType)
		}
	}

	if len(requiredExtensions) > 0 {
		return fmt.Errorf("extension controllers missing or unready: %+v", requiredExtensions)
	}

	return nil
}

// SPDX-FileCopyrightText: 2020 SAP SE or an SAP affiliate company and Gardener contributors
// SPDX-License-Identifier: Apache-2.0

package botanist

import (
	"context"
	"fmt"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/pointer"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
	"github.com/gardener/gardener/pkg/component/extensions/extension"
)

// DefaultExtension creates the default deployer for the Extension custom resources.
func (b *Botanist) DefaultExtension(ctx context.Context) (extension.Interface, error) {
	controllerRegistrations := &gardencorev1beta1.ControllerRegistrationList{}
	if err := b.GardenClient.List(ctx, controllerRegistrations); err != nil {
		return nil, err
	}

	extensions, err := mergeExtensions(controllerRegistrations.Items, b.Shoot.GetInfo().Spec.Extensions, b.Shoot.SeedNamespace, b.Shoot.IsWorkerless)
	if err != nil {
		return nil, fmt.Errorf("cannot calculate required extensions for shoot %s: %w", b.Shoot.GetInfo().Name, err)
	}

	return extension.New(
		b.Logger,
		b.SeedClientSet.Client(),
		&extension.Values{
			Namespace:  b.Shoot.SeedNamespace,
			Extensions: extensions,
		},
		extension.DefaultInterval,
		extension.DefaultSevereThreshold,
		extension.DefaultTimeout,
	), nil
}

// DeployExtensionsAfterKubeAPIServer deploys the Extension custom resources and triggers the restore operation in case
// the Shoot is in the restore phase of the control plane migration.
func (b *Botanist) DeployExtensionsAfterKubeAPIServer(ctx context.Context) error {
	if b.IsRestorePhase() {
		return b.Shoot.Components.Extensions.Extension.RestoreAfterKubeAPIServer(ctx, b.Shoot.GetShootState())
	}
	return b.Shoot.Components.Extensions.Extension.DeployAfterKubeAPIServer(ctx)
}

// DeployExtensionsBeforeKubeAPIServer deploys the Extension custom resources and triggers the restore operation in case
// the Shoot is in the restore phase of the control plane migration.
func (b *Botanist) DeployExtensionsBeforeKubeAPIServer(ctx context.Context) error {
	if b.IsRestorePhase() {
		return b.Shoot.Components.Extensions.Extension.RestoreBeforeKubeAPIServer(ctx, b.Shoot.GetShootState())
	}
	return b.Shoot.Components.Extensions.Extension.DeployBeforeKubeAPIServer(ctx)
}

func mergeExtensions(registrations []gardencorev1beta1.ControllerRegistration, extensions []gardencorev1beta1.Extension, namespace string, workerlessShoot bool) (map[string]extension.Extension, error) {
	var (
		typeToExtension    = make(map[string]extension.Extension)
		requiredExtensions = make(map[string]extension.Extension)
	)

	// Extensions enabled by default for all Shoot clusters.
	for _, reg := range registrations {
		for _, res := range reg.Spec.Resources {
			if res.Kind != extensionsv1alpha1.ExtensionResource {
				continue
			}

			timeout := extension.DefaultTimeout
			if res.ReconcileTimeout != nil {
				timeout = res.ReconcileTimeout.Duration
			}

			typeToExtension[res.Type] = extension.Extension{
				Extension: extensionsv1alpha1.Extension{
					ObjectMeta: metav1.ObjectMeta{
						Name:      res.Type,
						Namespace: namespace,
					},
					Spec: extensionsv1alpha1.ExtensionSpec{
						DefaultSpec: extensionsv1alpha1.DefaultSpec{
							Type: res.Type,
						},
					},
				},
				Timeout:   timeout,
				Lifecycle: res.Lifecycle,
			}

			if res.GloballyEnabled != nil && *res.GloballyEnabled {
				if workerlessShoot && !pointer.BoolDeref(res.WorkerlessSupported, false) {
					continue
				}
				requiredExtensions[res.Type] = typeToExtension[res.Type]
			}
		}
	}

	// Extensions defined in Shoot resource.
	for _, extension := range extensions {
		if obj, ok := typeToExtension[extension.Type]; ok {
			if pointer.BoolDeref(extension.Disabled, false) {
				delete(requiredExtensions, extension.Type)
				continue
			}

			obj.Spec.ProviderConfig = extension.ProviderConfig
			requiredExtensions[extension.Type] = obj
			continue
		}
	}

	return requiredExtensions, nil
}

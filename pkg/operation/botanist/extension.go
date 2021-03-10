// Copyright (c) 2020 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

package botanist

import (
	"context"
	"fmt"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
	"github.com/gardener/gardener/pkg/operation/botanist/component/extensions/extension"
	"github.com/gardener/gardener/pkg/utils"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// DefaultExtension creates the default deployer for the Extension custom resources.
func (b *Botanist) DefaultExtension(ctx context.Context, seedClient client.Client) (extension.Interface, error) {
	controllerRegistrations := &gardencorev1beta1.ControllerRegistrationList{}
	if err := b.K8sGardenClient.Client().List(ctx, controllerRegistrations); err != nil {
		return nil, err
	}

	extensions, err := mergeExtensions(controllerRegistrations.Items, b.Shoot.Info.Spec.Extensions, b.Shoot.SeedNamespace)
	if err != nil {
		return nil, fmt.Errorf("cannot calculate required extensions for shoot %s: %v", b.Shoot.Info.Name, err)
	}

	return extension.New(
		b.Logger,
		seedClient,
		&extension.Values{
			Namespace:  b.Shoot.SeedNamespace,
			Extensions: extensions,
		},
		extension.DefaultInterval,
		extension.DefaultSevereThreshold,
		extension.DefaultTimeout,
	), nil
}

// DeployExtensions deploys the Extension custom resources and triggers the restore operation in case
// the Shoot is in the restore phase of the control plane migration.
func (b *Botanist) DeployExtensions(ctx context.Context) error {
	if b.isRestorePhase() {
		return b.Shoot.Components.Extensions.Extension.Restore(ctx, b.ShootState)
	}
	return b.Shoot.Components.Extensions.Extension.Deploy(ctx)
}

func mergeExtensions(registrations []gardencorev1beta1.ControllerRegistration, extensions []gardencorev1beta1.Extension, namespace string) (map[string]extension.Extension, error) {
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
				Timeout: timeout,
			}

			if res.GloballyEnabled != nil && *res.GloballyEnabled {
				requiredExtensions[res.Type] = typeToExtension[res.Type]
			}
		}
	}

	// Extensions defined in Shoot resource.
	for _, extension := range extensions {
		if obj, ok := typeToExtension[extension.Type]; ok {
			if utils.IsTrue(extension.Disabled) {
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

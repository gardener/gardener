// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package shared

import (
	"context"

	"github.com/go-logr/logr"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
	"github.com/gardener/gardener/pkg/component/extensions/extension"
)

// NewExtension creates the default deployer for the Extension custom resources.
func NewExtension(
	ctx context.Context,
	log logr.Logger,
	gardenClient client.Client,
	seedClient client.Client,
	namespace string,
	extensions []gardencorev1beta1.Extension,
	workerlessSupported bool,
) (extension.Interface, error) {
	controllerRegistrations := &gardencorev1beta1.ControllerRegistrationList{}
	if err := gardenClient.List(ctx, controllerRegistrations); err != nil {
		return nil, err
	}

	return extension.New(
		log,
		seedClient,
		&extension.Values{
			Namespace:  namespace,
			Extensions: mergeExtensions(controllerRegistrations.Items, extensions, namespace, workerlessSupported),
		},
		extension.DefaultInterval,
		extension.DefaultSevereThreshold,
		extension.DefaultTimeout,
	), nil
}

func mergeExtensions(registrations []gardencorev1beta1.ControllerRegistration, extensions []gardencorev1beta1.Extension, namespace string, workerlessShoot bool) map[string]extension.Extension {
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
				if workerlessShoot && !ptr.Deref(res.WorkerlessSupported, false) {
					continue
				}
				requiredExtensions[res.Type] = typeToExtension[res.Type]
			}
		}
	}

	for _, extension := range extensions {
		if obj, ok := typeToExtension[extension.Type]; ok {
			if ptr.Deref(extension.Disabled, false) {
				delete(requiredExtensions, extension.Type)
				continue
			}

			obj.Spec.ProviderConfig = extension.ProviderConfig
			requiredExtensions[extension.Type] = obj
			continue
		}
	}

	return requiredExtensions
}

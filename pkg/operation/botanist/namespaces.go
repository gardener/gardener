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
	"time"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	"github.com/gardener/gardener/pkg/controllerutils"
	"github.com/gardener/gardener/pkg/operation/botanist/component"
	"github.com/gardener/gardener/pkg/operation/botanist/component/namespaces"
	"github.com/gardener/gardener/pkg/utils"
	"github.com/gardener/gardener/pkg/utils/retry"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/sets"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// DeploySeedNamespace creates a namespace in the Seed cluster which is used to deploy all the control plane
// components for the Shoot cluster. Moreover, the cloud provider configuration and all the secrets will be
// stored as ConfigMaps/Secrets.
func (b *Botanist) DeploySeedNamespace(ctx context.Context) error {
	namespace := &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: b.Shoot.SeedNamespace,
		},
	}

	if _, err := controllerutils.GetAndCreateOrMergePatch(ctx, b.K8sSeedClient.Client(), namespace, func() error {
		namespace.Annotations = map[string]string{
			v1beta1constants.ShootUID: string(b.Shoot.GetInfo().Status.UID),
		}
		namespace.Labels = map[string]string{
			v1beta1constants.GardenRole:              v1beta1constants.GardenRoleShoot,
			v1beta1constants.LabelSeedProvider:       b.Seed.GetInfo().Spec.Provider.Type,
			v1beta1constants.LabelShootProvider:      b.Shoot.GetInfo().Spec.Provider.Type,
			v1beta1constants.LabelNetworkingProvider: b.Shoot.GetInfo().Spec.Networking.Type,
			v1beta1constants.LabelBackupProvider:     b.Seed.GetInfo().Spec.Provider.Type,
		}

		requiredExtensions, err := getShootRequiredExtensionTypes(ctx, b)
		if err != nil {
			return err
		}

		for extensionType := range requiredExtensions {
			namespace.Labels[v1beta1constants.LabelExtensionPrefix+extensionType] = "true"
		}

		if b.Seed.GetInfo().Spec.Backup != nil {
			namespace.Labels[v1beta1constants.LabelBackupProvider] = b.Seed.GetInfo().Spec.Backup.Provider
		}

		return nil
	}); err != nil {
		return err
	}

	b.SeedNamespaceObject = namespace
	return nil
}

// DeleteSeedNamespace deletes the namespace in the Seed cluster which holds the control plane components. The built-in
// garbage collection in Kubernetes will automatically delete all resources which belong to this namespace. This
// comprises volumes and load balancers as well.
func (b *Botanist) DeleteSeedNamespace(ctx context.Context) error {
	namespace := &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: b.Shoot.SeedNamespace,
		},
	}

	err := b.K8sSeedClient.Client().Delete(ctx, namespace, kubernetes.DefaultDeleteOptions...)
	if apierrors.IsNotFound(err) || apierrors.IsConflict(err) {
		return nil
	}

	return err
}

// WaitUntilSeedNamespaceDeleted waits until the namespace of the Shoot cluster within the Seed cluster is deleted.
func (b *Botanist) WaitUntilSeedNamespaceDeleted(ctx context.Context) error {
	return retry.UntilTimeout(ctx, 5*time.Second, 900*time.Second, func(ctx context.Context) (done bool, err error) {
		if err := b.K8sSeedClient.Client().Get(ctx, client.ObjectKey{Name: b.Shoot.SeedNamespace}, &corev1.Namespace{}); err != nil {
			if apierrors.IsNotFound(err) {
				return retry.Ok()
			}
			return retry.SevereError(err)
		}
		b.Logger.Infof("Waiting until the namespace '%s' has been cleaned up and deleted in the Seed cluster...", b.Shoot.SeedNamespace)
		return retry.MinorError(fmt.Errorf("namespace %q is not yet cleaned up", b.Shoot.SeedNamespace))
	})
}

// DefaultShootNamespaces returns a deployer for the shoot namespaces.
func (b *Botanist) DefaultShootNamespaces() component.DeployWaiter {
	return namespaces.New(b.K8sSeedClient.Client(), b.Shoot.SeedNamespace)
}

// getShootRequiredExtensionTypes returns all extension types that are enabled or explicitly disabled for the shoot.
// The function considers only extensions of kind `Extension`.
func getShootRequiredExtensionTypes(ctx context.Context, b *Botanist) (sets.String, error) {
	controllerRegistrationList := &gardencorev1beta1.ControllerRegistrationList{}
	if err := b.K8sGardenClient.Client().List(ctx, controllerRegistrationList); err != nil {
		return nil, err
	}

	types := sets.String{}
	for _, reg := range controllerRegistrationList.Items {
		for _, res := range reg.Spec.Resources {
			if res.Kind == extensionsv1alpha1.ExtensionResource && utils.IsTrue(res.GloballyEnabled) {
				types.Insert(res.Type)
			}
		}
	}

	for _, extension := range b.Shoot.GetInfo().Spec.Extensions {
		if utils.IsTrue(extension.Disabled) {
			types.Delete(extension.Type)
		} else {
			types.Insert(extension.Type)
		}
	}

	return types, nil
}

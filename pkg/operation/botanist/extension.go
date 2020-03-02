// Copyright (c) 2019 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	gardencorev1beta1helper "github.com/gardener/gardener/pkg/apis/core/v1beta1/helper"
	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	"github.com/gardener/gardener/pkg/operation/shoot"
	"github.com/gardener/gardener/pkg/utils/flow"
	kutil "github.com/gardener/gardener/pkg/utils/kubernetes"
	"github.com/gardener/gardener/pkg/utils/kubernetes/health"
	"github.com/gardener/gardener/pkg/utils/retry"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/sets"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

// DeployExtensionResources creates the `Extension` extension resource in the shoot namespace in the seed
// cluster. Gardener waits until an external controller did reconcile the cluster successfully.
func (b *Botanist) DeployExtensionResources(ctx context.Context) error {
	fns := make([]flow.TaskFn, 0, len(b.Shoot.Extensions))
	for _, extension := range b.Shoot.Extensions {
		var (
			extensionType  = extension.Spec.Type
			providerConfig = extension.Spec.ProviderConfig
			toApply        = extensionsv1alpha1.Extension{
				ObjectMeta: metav1.ObjectMeta{
					Name:      extension.Name,
					Namespace: extension.Namespace,
				},
			}
		)

		fns = append(fns, func(ctx context.Context) error {
			_, err := controllerutil.CreateOrUpdate(ctx, b.K8sSeedClient.Client(), &toApply, func() error {
				metav1.SetMetaDataAnnotation(&toApply.ObjectMeta, v1beta1constants.GardenerOperation, v1beta1constants.GardenerOperationReconcile)

				toApply.Spec.Type = extensionType
				toApply.Spec.ProviderConfig = providerConfig
				return nil
			})
			return err
		})
	}

	return flow.Parallel(fns...)(ctx)
}

// DeleteStaleExtensionResources deletes unused extensions from the shoot namespace in the seed.
func (b *Botanist) DeleteStaleExtensionResources(ctx context.Context) error {
	wantedExtensions := sets.NewString()
	for _, extension := range b.Shoot.Extensions {
		wantedExtensions.Insert(extension.Spec.Type)
	}

	deployedExtensions := &extensionsv1alpha1.ExtensionList{}
	if err := b.K8sSeedClient.Client().List(ctx, deployedExtensions, client.InNamespace(b.Shoot.SeedNamespace)); err != nil {
		return err
	}

	fns := make([]flow.TaskFn, 0, meta.LenList(deployedExtensions))
	for _, deployedExtension := range deployedExtensions.Items {
		if !wantedExtensions.Has(deployedExtension.Spec.Type) {
			toDelete := &extensionsv1alpha1.Extension{
				ObjectMeta: metav1.ObjectMeta{
					Name:      deployedExtension.Name,
					Namespace: deployedExtension.Namespace,
				},
			}
			fns = append(fns, func(ctx context.Context) error {
				return client.IgnoreNotFound(b.K8sSeedClient.Client().Delete(ctx, toDelete, kubernetes.DefaultDeleteOptions...))
			})
		}
	}

	return flow.Parallel(fns...)(ctx)
}

// WaitUntilExtensionResourcesReady waits until all extension resources report `Succeeded` in their last operation state.
// The state must be reported before the passed context is cancelled or an extension's timeout has been reached.
// As soon as one timeout has been overstepped the function returns an error, further waits on extensions will be aborted.
func (b *Botanist) WaitUntilExtensionResourcesReady(ctx context.Context) error {
	fns := make([]flow.TaskFn, 0, len(b.Shoot.Extensions))
	for _, extension := range b.Shoot.Extensions {
		var (
			name      = extension.Name
			namespace = extension.Namespace
		)
		fns = append(fns, func(ctx context.Context) error {
			if err := retry.UntilTimeout(ctx, DefaultInterval, shoot.ExtensionDefaultTimeout, func(ctx context.Context) (bool, error) {
				req := &extensionsv1alpha1.Extension{}
				if err := b.K8sSeedClient.Client().Get(ctx, kutil.Key(namespace, name), req); err != nil {
					return retry.SevereError(err)
				}

				if err := health.CheckExtensionObject(req); err != nil {
					b.Logger.WithError(err).Errorf("Extension %s/%s did not get ready yet", namespace, name)
					return retry.MinorError(err)
				}

				return retry.Ok()
			}); err != nil {
				return gardencorev1beta1helper.DetermineError(fmt.Sprintf("failed waiting for extension %s to be ready: %v", name, err))
			}
			return nil
		})
	}

	return flow.ParallelExitOnError(fns...)(ctx)
}

// DeleteExtensionResources deletes all extension resources from the Shoot namespace in the Seed.
func (b *Botanist) DeleteExtensionResources(ctx context.Context) error {
	return b.K8sSeedClient.Client().DeleteAllOf(ctx, &extensionsv1alpha1.Extension{}, client.InNamespace(b.Shoot.SeedNamespace))
}

// WaitUntilExtensionResourcesDeleted waits until all extension resources are gone or the context is cancelled.
func (b *Botanist) WaitUntilExtensionResourcesDeleted(ctx context.Context) error {
	var (
		lastError  *gardencorev1beta1.LastError
		extensions = &extensionsv1alpha1.ExtensionList{}
	)

	if err := b.K8sSeedClient.Client().List(ctx, extensions, client.InNamespace(b.Shoot.SeedNamespace)); err != nil {
		return err
	}

	fns := make([]flow.TaskFn, 0, len(extensions.Items))
	for _, extension := range extensions.Items {
		if extension.GetDeletionTimestamp() == nil {
			continue
		}

		var (
			name      = extension.Name
			namespace = extension.Namespace
			status    = extension.Status
		)

		fns = append(fns, func(ctx context.Context) error {
			if err := retry.UntilTimeout(ctx, DefaultInterval, shoot.ExtensionDefaultTimeout, func(ctx context.Context) (bool, error) {
				if err := b.K8sSeedClient.Client().Get(ctx, kutil.Key(namespace, name), &extensionsv1alpha1.Extension{}); err != nil {
					if apierrors.IsNotFound(err) {
						return retry.Ok()
					}
					return retry.SevereError(err)
				}

				if lastErr := status.LastError; lastErr != nil {
					b.Logger.Errorf("Extension %s did not get deleted yet, lastError is: %s", name, lastErr.Description)
					lastError = lastErr
				}

				return retry.MinorError(gardencorev1beta1helper.WrapWithLastError(fmt.Errorf("extension %s is still present", name), lastError))
			}); err != nil {
				message := fmt.Sprintf("Failed waiting for extension delete")
				if lastError != nil {
					return gardencorev1beta1helper.DetermineError(fmt.Sprintf("%s: %s", message, lastError.Description))
				}
				return gardencorev1beta1helper.DetermineError(fmt.Sprintf("%s: %s", message, err.Error()))
			}
			return nil
		})
	}

	return flow.Parallel(fns...)(ctx)
}

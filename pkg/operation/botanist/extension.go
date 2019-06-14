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
	"time"

	corev1alpha1 "github.com/gardener/gardener/pkg/apis/core/v1alpha1"
	gardencorev1alpha1 "github.com/gardener/gardener/pkg/apis/core/v1alpha1"
	gardencorev1alpha1helper "github.com/gardener/gardener/pkg/apis/core/v1alpha1/helper"
	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
	"github.com/gardener/gardener/pkg/operation/shoot"
	"github.com/gardener/gardener/pkg/utils/flow"
	kutil "github.com/gardener/gardener/pkg/utils/kubernetes"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	"sigs.k8s.io/controller-runtime/pkg/client"
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
			return kutil.CreateOrUpdate(ctx, b.K8sSeedClient.Client(), &toApply, func() error {
				toApply.Spec.Type = extensionType
				toApply.Spec.ProviderConfig = providerConfig
				return nil
			})
		})
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
			name                 = extension.Name
			namespace            = extension.Namespace
			timedContext, cancel = context.WithTimeout(ctx, extension.Timeout)
		)
		fns = append(fns, func(ctx context.Context) error {
			defer cancel()

			var lastError *gardencorev1alpha1.LastError

			if err := wait.PollUntil(5*time.Second, func() (bool, error) {
				req := &extensionsv1alpha1.Extension{}
				if err := b.K8sSeedClient.Client().Get(ctx, kutil.Key(namespace, name), req); err != nil {
					return false, err
				}

				if lastErr := req.Status.LastError; lastErr != nil {
					b.Logger.Errorf("Extension %s/%s did not get ready yet, lastError is: %s", namespace, name, lastErr.Description)
					lastError = lastErr
				}

				if req.Status.LastOperation != nil &&
					req.Status.LastOperation.State == corev1alpha1.LastOperationStateSucceeded &&
					req.Status.ObservedGeneration == req.Generation {
					return true, nil
				}

				b.Logger.Infof("Waiting for extension %s/%s to be ready...", namespace, name)
				return false, nil
			}, timedContext.Done()); err != nil {
				message := fmt.Sprintf("Failed waiting for extension %s is ready", name)
				if lastError != nil {
					return gardencorev1alpha1helper.DetermineError(fmt.Sprintf("%s: %s", message, lastError.Description))
				}
				return gardencorev1alpha1helper.DetermineError(fmt.Sprintf("%s: %s", message, err.Error()))
			}
			return nil
		})
	}

	return flow.ParallelExitOnError(fns...)(ctx)
}

// DeleteExtensionResources deletes all extension resources from the Shoot namespace in the Seed.
func (b *Botanist) DeleteExtensionResources(ctx context.Context) error {
	extensions := &extensionsv1alpha1.ExtensionList{}
	if err := b.K8sSeedClient.Client().List(ctx, extensions, client.InNamespace(b.Shoot.SeedNamespace)); err != nil {
		return err
	}

	fns := make([]flow.TaskFn, 0, len(extensions.Items))
	for _, extension := range extensions.Items {
		fns = append(fns, func(ctx context.Context) error {
			toDelete := extensionsv1alpha1.Extension{
				ObjectMeta: metav1.ObjectMeta{
					Name:      extension.Name,
					Namespace: extension.Namespace,
				},
			}

			if err := b.K8sSeedClient.Client().Delete(ctx, &toDelete); err != nil && !apierrors.IsNotFound(err) {
				return err
			}
			return nil
		})
	}

	return flow.Parallel(fns...)(ctx)
}

// WaitUntilExtensionResourcesDeleted waits until all extension resources are gone or the context is cancelled.
func (b *Botanist) WaitUntilExtensionResourcesDeleted(ctx context.Context) error {
	var (
		lastError *gardencorev1alpha1.LastError

		extensions           = &extensionsv1alpha1.ExtensionList{}
		timedContext, cancel = context.WithTimeout(ctx, shoot.ExtensionDefaultTimeout)
	)
	defer cancel()

	if err := b.K8sSeedClient.Client().List(ctx, extensions, client.InNamespace(b.Shoot.SeedNamespace)); err != nil {
		return err
	}

	fns := make([]flow.TaskFn, 0, len(extensions.Items))
	for _, extension := range extensions.Items {
		var (
			name      = extension.Name
			namespace = extension.Namespace
			status    = extension.Status
		)

		fns = append(fns, func(ctx context.Context) error {
			if err := wait.PollUntil(5*time.Second, func() (bool, error) {
				if err := b.K8sSeedClient.Client().Get(ctx, kutil.Key(namespace, name), &extensionsv1alpha1.Extension{}); err != nil {
					if apierrors.IsNotFound(err) {
						return true, nil
					}
					return false, err
				}

				if lastErr := status.LastError; lastErr != nil {
					b.Logger.Errorf("Extension %s did not get deleted yet, lastError is: %s", name, lastErr.Description)
					lastError = lastErr
				}

				return false, nil
			}, ctx.Done()); err != nil {
				message := fmt.Sprintf("Failed waiting for extension delete")
				if lastError != nil {
					return gardencorev1alpha1helper.DetermineError(fmt.Sprintf("%s: %s", message, lastError.Description))
				}
				return gardencorev1alpha1helper.DetermineError(fmt.Sprintf("%s: %s", message, err.Error()))
			}
			return nil
		})
	}

	return flow.Parallel(fns...)(timedContext)
}

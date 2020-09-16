// SPDX-FileCopyrightText: 2019 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package botanist

import (
	"context"
	"time"

	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
	"github.com/gardener/gardener/pkg/operation/common"
	"github.com/gardener/gardener/pkg/operation/shoot"
	"github.com/gardener/gardener/pkg/utils/flow"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/sets"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

// DeployExtensionResources creates the `Extension` extension resource in the shoot namespace in the seed
// cluster. Gardener waits until an external controller did reconcile the cluster successfully.
func (b *Botanist) DeployExtensionResources(ctx context.Context) error {
	var (
		restorePhase      = b.isRestorePhase()
		gardenerOperation = v1beta1constants.GardenerOperationReconcile
	)

	if restorePhase {
		gardenerOperation = v1beta1constants.GardenerOperationWaitForState
	}

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
				metav1.SetMetaDataAnnotation(&toApply.ObjectMeta, v1beta1constants.GardenerOperation, gardenerOperation)
				metav1.SetMetaDataAnnotation(&toApply.ObjectMeta, v1beta1constants.GardenerTimestamp, time.Now().UTC().String())
				toApply.Spec.Type = extensionType
				toApply.Spec.ProviderConfig = providerConfig
				return nil
			})

			if restorePhase {
				return b.restoreExtensionObject(ctx, &toApply, extensionsv1alpha1.ExtensionResource)
			}

			return err
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
		fns = append(fns, func(ctx context.Context) error {
			return common.WaitUntilExtensionCRReady(
				ctx,
				b.K8sSeedClient.DirectClient(),
				b.Logger,
				func() runtime.Object { return &extensionsv1alpha1.Extension{} },
				"Extension",
				extension.Namespace,
				extension.Name,
				DefaultInterval,
				DefaultSevereThreshold,
				extension.Timeout,
				nil,
			)
		})
	}

	return flow.ParallelExitOnError(fns...)(ctx)
}

// DeleteStaleExtensionResources deletes unused extensions from the shoot namespace in the seed.
func (b *Botanist) DeleteStaleExtensionResources(ctx context.Context) error {
	wantedExtensionTypes := sets.NewString()
	for _, extension := range b.Shoot.Extensions {
		wantedExtensionTypes.Insert(extension.Spec.Type)
	}
	return b.deleteExtensionResources(ctx, wantedExtensionTypes)
}

// DeleteAllExtensionResources deletes all extension resources from the Shoot namespace in the Seed.
func (b *Botanist) DeleteAllExtensionResources(ctx context.Context) error {
	return b.deleteExtensionResources(ctx, sets.NewString())
}

func (b *Botanist) deleteExtensionResources(ctx context.Context, wantedExtensionTypes sets.String) error {
	return common.DeleteExtensionCRs(
		ctx,
		b.K8sSeedClient.Client(),
		&extensionsv1alpha1.ExtensionList{},
		func() extensionsv1alpha1.Object { return &extensionsv1alpha1.Extension{} },
		b.Shoot.SeedNamespace,
		func(obj extensionsv1alpha1.Object) bool {
			return !wantedExtensionTypes.Has(obj.GetExtensionSpec().GetExtensionType())
		},
	)
}

// WaitUntilExtensionResourcesDeleted waits until all extension resources are gone or the context is cancelled.
func (b *Botanist) WaitUntilExtensionResourcesDeleted(ctx context.Context) error {
	return common.WaitUntilExtensionCRsDeleted(
		ctx,
		b.K8sSeedClient.DirectClient(),
		b.Logger,
		&extensionsv1alpha1.ExtensionList{},
		func() extensionsv1alpha1.Object { return &extensionsv1alpha1.Extension{} },
		"Extension",
		b.Shoot.SeedNamespace,
		DefaultInterval,
		shoot.ExtensionDefaultTimeout,
		nil,
	)
}

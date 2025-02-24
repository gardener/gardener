// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package kubernetes

import (
	"context"
	"time"

	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/gardener/gardener/pkg/utils/flow"
	"github.com/gardener/gardener/pkg/utils/kubernetes/health"
	"github.com/gardener/gardener/pkg/utils/retry"
)

var (
	// WaitTimeout specifies the total time to wait for CRDs to become ready or to be deleted. Exposed for testing.
	WaitTimeout = 2 * time.Minute
)

// WaitUntilCRDManifestsReady takes names of CRDs and waits for them to get ready with a timeout of 15 seconds.
func WaitUntilCRDManifestsReady(ctx context.Context, c client.Client, crdNames ...string) error {
	var fns []flow.TaskFn
	for _, crdName := range crdNames {
		fns = append(fns, func(ctx context.Context) error {
			timeoutCtx, cancel := context.WithTimeout(ctx, WaitTimeout)
			defer cancel()
			return retry.Until(timeoutCtx, 1*time.Second, func(ctx context.Context) (done bool, err error) {
				crd := &apiextensionsv1.CustomResourceDefinition{}

				if err := c.Get(ctx, client.ObjectKey{Name: crdName}, crd); err != nil {
					if client.IgnoreNotFound(err) == nil {
						return retry.MinorError(err)
					}
					return retry.SevereError(err)
				}

				if err := health.CheckCustomResourceDefinition(crd); err != nil {
					return retry.MinorError(err)
				}
				return retry.Ok()
			})
		})
	}
	return flow.Parallel(fns...)(ctx)
}

// WaitUntilCRDManifestsDestroyed takes CRD names and waits for them to be gone with a timeout of 15 seconds.
func WaitUntilCRDManifestsDestroyed(ctx context.Context, c client.Client, crdNames ...string) error {
	var fns []flow.TaskFn

	for _, resourceName := range crdNames {
		crd := &apiextensionsv1.CustomResourceDefinition{
			ObjectMeta: metav1.ObjectMeta{
				Name: resourceName,
			},
		}

		fns = append(fns, func(ctx context.Context) error {
			timeoutCtx, cancel := context.WithTimeout(ctx, WaitTimeout)
			defer cancel()
			return WaitUntilResourceDeleted(timeoutCtx, c, crd, 1*time.Second)
		})
	}
	return flow.Parallel(fns...)(ctx)
}

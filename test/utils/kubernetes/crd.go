// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package kubernetes

import (
	"context"
	"fmt"
	"time"

	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/gardener/gardener/pkg/utils/flow"
	"github.com/gardener/gardener/pkg/utils/retry"
)

// VerifyCRDDeletionByDeletionTimestamp verifies that the given CRDs are deleted by checking the deletion timestamp is present.
func VerifyCRDDeletionByDeletionTimestamp(ctx context.Context, c client.Client, crdNames ...string) error {
	var taskFns []flow.TaskFn
	for _, crdName := range crdNames {
		taskFns = append(taskFns, func(ctx context.Context) error {
			return retry.Until(ctx, 200*time.Millisecond, func(ctx context.Context) (done bool, err error) {
				crd := &metav1.PartialObjectMetadata{}
				crd.SetGroupVersionKind(apiextensionsv1.SchemeGroupVersion.WithKind("CustomResourceDefinition"))
				if err := c.Get(ctx, client.ObjectKey{Name: crdName}, crd); err != nil {
					if apierrors.IsNotFound(err) {
						return retry.Ok()
					}
					return retry.MinorError(err)
				}
				if crd.GetDeletionTimestamp().IsZero() {
					return retry.MinorError(fmt.Errorf("CRD %s is not deleted yet", crdName))
				}
				return retry.Ok()
			})
		})
	}
	return flow.Parallel(taskFns...)(ctx)
}

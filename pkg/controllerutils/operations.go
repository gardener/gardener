// SPDX-FileCopyrightText: 2018 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package controllerutils

import (
	"context"

	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	kutil "github.com/gardener/gardener/pkg/utils/kubernetes"

	"k8s.io/apimachinery/pkg/util/wait"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// RemoveGardenerOperationAnnotation removes a  gardener operation annotation and retries the operation with the given <backoff>.
func RemoveGardenerOperationAnnotation(ctx context.Context, backoff wait.Backoff, cli client.Client, obj kutil.Object) error {
	return kutil.TryUpdate(ctx, backoff, cli, obj, func() error {
		delete(obj.GetAnnotations(), v1beta1constants.GardenerOperation)
		return nil
	})
}

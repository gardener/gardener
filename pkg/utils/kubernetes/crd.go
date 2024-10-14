// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package kubernetes

import (
	"context"
	"time"

	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/gardener/gardener/pkg/client/kubernetes"
	"github.com/gardener/gardener/pkg/utils/kubernetes/health"
	"github.com/gardener/gardener/pkg/utils/retry"
)

// WaitUntilCRDManifestsReady takes manifests as strings and waits for them to get ready with a timeout of 15 seconds
func WaitUntilCRDManifestsReady(ctx context.Context, c client.Client, manifests []string) error {
	timeoutCtx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()

	return retry.Until(timeoutCtx, 1*time.Second, func(ctx context.Context) (done bool, err error) {
		for _, resource := range manifests {
			crd := &apiextensionsv1.CustomResourceDefinition{}

			obj, err := kubernetes.NewManifestReader([]byte(resource)).Read()
			if err != nil {
				return retry.SevereError(err)
			}

			if err := c.Get(ctx, client.ObjectKeyFromObject(obj), crd); client.IgnoreNotFound(err) != nil {
				return retry.SevereError(err)
			}

			if err := health.CheckCustomResourceDefinition(crd); err != nil {
				return retry.MinorError(err)
			}
		}
		return retry.Ok()
	})
}

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

// WaitUntilCRDManifestsReady takes CRD ObjectKeys and waits for them to get ready with a timeout of 15 seconds
func WaitUntilCRDManifestsReady(ctx context.Context, c client.Client, crdObjectKeys []client.ObjectKey) error {
	timeoutCtx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()

	return retry.Until(timeoutCtx, 1*time.Second, func(ctx context.Context) (done bool, err error) {
		for _, crdObjectKey := range crdObjectKeys {
			crd := &apiextensionsv1.CustomResourceDefinition{}

			if err := c.Get(ctx, crdObjectKey, crd); client.IgnoreNotFound(err) != nil {
				return retry.SevereError(err)
			}

			if err := health.CheckCustomResourceDefinition(crd); err != nil {
				return retry.MinorError(err)
			}
		}
		return retry.Ok()
	})
}

// GetObjectKeyFromManifest takes a manifest and returns its corresponding ObjectKey
func GetObjectKeyFromManifest(manifest string) (client.ObjectKey, error) {
	object, err := kubernetes.NewManifestReader([]byte(manifest)).Read()
	if err != nil {
		return client.ObjectKey{}, err
	}

	return client.ObjectKeyFromObject(object), nil
}

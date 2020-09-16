// SPDX-FileCopyrightText: 2020 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package istio

import (
	"context"
	"path/filepath"

	"github.com/gardener/gardener/pkg/client/kubernetes"
	"github.com/gardener/gardener/pkg/operation/botanist/component"

	apiextensionsv1beta1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1beta1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	crclient "sigs.k8s.io/controller-runtime/pkg/client"
)

type crds struct {
	kubernetes.ChartApplier
	chartPath      string
	client         crclient.Client
	deprecatedCRDs []apiextensionsv1beta1.CustomResourceDefinition
}

// NewIstioCRD can be used to deploy istio CRDs.
// Destroy does nothing.
func NewIstioCRD(
	applier kubernetes.ChartApplier,
	chartsRootPath string,
	client crclient.Client,
) component.DeployWaiter {
	return &crds{
		ChartApplier: applier,
		chartPath:    filepath.Join(chartsRootPath, "istio", "istio-crds"),
		client:       client,
		deprecatedCRDs: []apiextensionsv1beta1.CustomResourceDefinition{
			// TODO: remove this after several gardener releases
			{ObjectMeta: metav1.ObjectMeta{Name: "meshpolicies.authentication.istio.io"}},
			// TODO: remove this after several gardener releases
			{ObjectMeta: metav1.ObjectMeta{Name: "policies.authentication.istio.io"}},
		},
	}
}

func (c *crds) Deploy(ctx context.Context) error {
	for _, deprecatedCRD := range c.deprecatedCRDs {
		if err := crclient.IgnoreNotFound(c.client.Delete(ctx, &deprecatedCRD)); err != nil {
			return err
		}
	}

	return c.Apply(ctx, c.chartPath, "", "istio")
}

func (c *crds) Destroy(ctx context.Context) error {
	// istio cannot be safely removed
	return nil
}

func (c *crds) Wait(ctx context.Context) error {
	return nil
}

func (c *crds) WaitCleanup(ctx context.Context) error {
	return nil
}

// SPDX-FileCopyrightText: 2020 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package istio

import (
	"context"
	"path/filepath"

	"github.com/gardener/gardener/pkg/client/kubernetes"
	"github.com/gardener/gardener/pkg/operation/botanist/component"
	"k8s.io/apimachinery/pkg/api/meta"
)

type proxyProtocol struct {
	namespace    string
	chartApplier kubernetes.ChartApplier
	chartPath    string
}

// NewProxyProtocolGateway creates a new DeployWaiter for istio which
// adds a PROXY Protocol listener to the istio-ingressgateway.
func NewProxyProtocolGateway(
	namespace string,
	chartApplier kubernetes.ChartApplier,
	chartsRootPath string,
) component.DeployWaiter {
	return &proxyProtocol{
		namespace:    namespace,
		chartApplier: chartApplier,
		chartPath:    filepath.Join(chartsRootPath, istioReleaseName, "istio-proxy-protocol"),
	}
}

func (i *proxyProtocol) Deploy(ctx context.Context) error {
	return i.chartApplier.Apply(ctx, i.chartPath, i.namespace, istioReleaseName)
}

func (i *proxyProtocol) Destroy(ctx context.Context) error {
	return i.chartApplier.Delete(
		ctx,
		i.chartPath,
		i.namespace,
		istioReleaseName,
		kubernetes.TolerateErrorFunc(meta.IsNoMatchError),
	)
}

func (i *proxyProtocol) Wait(ctx context.Context) error {
	return nil
}

func (i *proxyProtocol) WaitCleanup(ctx context.Context) error {
	return nil
}

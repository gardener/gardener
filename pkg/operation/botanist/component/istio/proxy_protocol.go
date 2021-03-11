// Copyright (c) 2020 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

package istio

import (
	"context"
	"path/filepath"

	"k8s.io/apimachinery/pkg/api/meta"

	"github.com/gardener/gardener/pkg/client/kubernetes"
	"github.com/gardener/gardener/pkg/operation/botanist/component"
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

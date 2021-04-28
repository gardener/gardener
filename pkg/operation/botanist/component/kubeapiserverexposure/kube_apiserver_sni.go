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

package kubeapiserverexposure

import (
	"context"
	"path/filepath"

	"github.com/gardener/gardener/pkg/client/kubernetes"
	"github.com/gardener/gardener/pkg/operation/botanist/component"

	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// SNIValues configure the kube-apiserver service SNI.
type SNIValues struct {
	Hosts                    []string            `json:"hosts,omitempty"`
	Name                     string              `json:"name,omitempty"`
	NamespaceUID             types.UID           `json:"namespaceUID,omitempty"`
	ApiserverClusterIP       string              `json:"apiserverClusterIP,omitempty"`
	IstioIngressGateway      IstioIngressGateway `json:"istioIngressGateway,omitempty"`
	InternalDNSNameApiserver string              `json:"internalDNSNameApiserver,omitempty"`
	ReversedVPN              ReversedVPN         `json:"reversedVPN,omitempty"`
}

// IstioIngressGateway contains the values for istio ingress gateway configuration.
type IstioIngressGateway struct {
	Namespace string            `json:"namespace,omitempty"`
	Labels    map[string]string `json:"labels,omitempty"`
}

// ReversedVPN contains whether the reversed vpn is enabled or not.
type ReversedVPN struct {
	Enabled bool `json:"enabled,omitempty"`
}

// NewSNI creates a new instance of DeployWaiter which deploys Istio resources for
// kube-apiserver SNI access.
func NewSNI(
	values *SNIValues,
	namespace string,
	applier kubernetes.ChartApplier,
	chartsRootPath string,
) component.DeployWaiter {
	if values == nil {
		values = &SNIValues{}
	}

	return &sni{
		ChartApplier: applier,
		chartPath:    filepath.Join(chartsRootPath, "seed-controlplane", "charts", "kube-apiserver-sni"),
		values:       values,
		namespace:    namespace,
	}
}

type sni struct {
	values    *SNIValues
	namespace string
	kubernetes.ChartApplier
	chartPath string
}

func (s *sni) Deploy(ctx context.Context) error {
	return s.Apply(
		ctx,
		s.chartPath,
		s.namespace,
		s.values.Name,
		kubernetes.Values(s.values),
	)
}

func (s *sni) Destroy(ctx context.Context) error {
	return s.Delete(
		ctx,
		s.chartPath,
		s.namespace,
		s.values.Name,
		kubernetes.Values(s.values),
		kubernetes.TolerateErrorFunc(meta.IsNoMatchError),
	)
}

func (s *sni) Wait(ctx context.Context) error        { return nil }
func (s *sni) WaitCleanup(ctx context.Context) error { return nil }

// AnyDeployedSNI returns true if any SNI is deployed in the cluster.
func AnyDeployedSNI(ctx context.Context, c client.Client) (bool, error) {
	l := &unstructured.UnstructuredList{
		Object: map[string]interface{}{
			"apiVersion": "networking.istio.io/v1beta1",
			"kind":       "VirtualServiceList",
		},
	}

	if err := c.List(ctx, l, client.MatchingFields{"metadata.name": "kube-apiserver"}, client.Limit(1)); err != nil && !meta.IsNoMatchError(err) {
		return false, err
	}

	return len(l.Items) > 0, nil
}

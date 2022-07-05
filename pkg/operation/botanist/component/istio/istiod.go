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
	"embed"
	"path/filepath"
	"time"

	"github.com/gardener/gardener/pkg/chartrenderer"
	"github.com/gardener/gardener/pkg/features"
	gardenletfeatures "github.com/gardener/gardener/pkg/gardenlet/features"
	"github.com/gardener/gardener/pkg/operation/botanist/component"
	kutil "github.com/gardener/gardener/pkg/utils/kubernetes"
	"github.com/gardener/gardener/pkg/utils/managedresources"

	networkingv1alpha3 "istio.io/client-go/pkg/apis/networking/v1alpha3"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	crclient "sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	// ManagedResourceControlName is the name of the ManagedResource containing the resource specifications.
	ManagedResourceControlName = "istio"

	istiodServiceName            = "istiod"
	istiodServicePortNameMetrics = "metrics"
)

var (
	//go:embed charts/istio/istio-istiod
	chartIstiod     embed.FS
	chartPathIstiod = filepath.Join("charts", "istio", "istio-istiod")
)

type istiod struct {
	client                    crclient.Client
	chartRenderer             chartrenderer.Interface
	namespace                 string
	values                    IstiodValues
	istioIngressGatewayValues []IngressGateway
	istioProxyProtocolValues  []ProxyProtocol
}

// IstiodValues holds values for the istio-istiod chart.
type IstiodValues struct {
	TrustDomain string `json:"trustDomain,omitempty"`
	Image       string `json:"image,omitempty"`
}

// NewIstio can be used to deploy istio's istiod in a namespace.
// Destroy does nothing.
func NewIstio(
	client crclient.Client,
	chartRenderer chartrenderer.Interface,
	values IstiodValues,
	namespace string,
	istioIngressGatewayValues []IngressGateway,
	istioProxyProtocolValues []ProxyProtocol,
) component.DeployWaiter {
	return &istiod{
		client:                    client,
		chartRenderer:             chartRenderer,
		values:                    values,
		namespace:                 namespace,
		istioIngressGatewayValues: istioIngressGatewayValues,
		istioProxyProtocolValues:  istioProxyProtocolValues,
	}
}

func (i *istiod) Deploy(ctx context.Context) error {
	if err := i.client.Create(
		ctx,
		&corev1.Namespace{
			ObjectMeta: metav1.ObjectMeta{
				Name: i.namespace,
				Labels: map[string]string{
					"istio-operator-managed": "Reconcile",
					"istio-injection":        "disabled",
				},
			},
		},
	); kutil.IgnoreAlreadyExists(err) != nil {
		return err
	}

	// TODO(mvladev): Rotate this on every istio version upgrade.
	for _, filterName := range []string{"tcp-stats-filter-1.10", "stats-filter-1.10"} {
		if err := crclient.IgnoreNotFound(i.client.Delete(ctx, &networkingv1alpha3.EnvoyFilter{
			ObjectMeta: metav1.ObjectMeta{Name: filterName, Namespace: i.namespace},
		})); err != nil {
			return err
		}
	}

	renderedIstiodChart, err := i.generateIstiodChart()
	if err != nil {
		return err
	}

	renderedIstioIngressGatewayChart, err := i.generateIstioIngressGatewayChart()
	if err != nil {
		return err
	}

	renderedIstioProxyProtocolChart, err := i.generateIstioProxyProtocolChart()
	if err != nil {
		return err
	}

	for _, istioIngressGateway := range i.istioIngressGatewayValues {
		if err := i.client.Create(
			ctx,
			&corev1.Namespace{
				ObjectMeta: metav1.ObjectMeta{
					Name:   istioIngressGateway.Namespace,
					Labels: getIngressGatewayNamespaceLabels(istioIngressGateway.Values.Labels),
				},
			},
		); kutil.IgnoreAlreadyExists(err) != nil {
			return err
		}
	}

	renderedChart := renderedIstiodChart
	renderedChart.Manifests = append(renderedChart.Manifests, renderedIstioIngressGatewayChart.Manifests...)
	if gardenletfeatures.FeatureGate.Enabled(features.APIServerSNI) {
		renderedChart.Manifests = append(renderedChart.Manifests, renderedIstioProxyProtocolChart.Manifests...)
	}

	return managedresources.CreateForSeed(ctx, i.client, i.namespace, ManagedResourceControlName, false, renderedChart.AsSecretData())
}

func (i *istiod) Destroy(ctx context.Context) error {
	if err := managedresources.DeleteForSeed(ctx, i.client, i.namespace, ManagedResourceControlName); err != nil {
		return err
	}

	if err := i.client.Delete(ctx, &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: i.namespace,
		},
	}); crclient.IgnoreNotFound(err) != nil {
		return err
	}

	for _, istioIngressGateway := range i.istioIngressGatewayValues {
		if err := i.client.Delete(ctx, &corev1.Namespace{
			ObjectMeta: metav1.ObjectMeta{
				Name: istioIngressGateway.Namespace,
			},
		}); crclient.IgnoreNotFound(err) != nil {
			return err
		}
	}

	return nil
}

// TimeoutWaitForManagedResource is the timeout used while waiting for the ManagedResources to become healthy
// or deleted.
var TimeoutWaitForManagedResource = 2 * time.Minute

func (i *istiod) Wait(ctx context.Context) error {
	timeoutCtx, cancel := context.WithTimeout(ctx, TimeoutWaitForManagedResource)
	defer cancel()

	return managedresources.WaitUntilHealthy(timeoutCtx, i.client, i.namespace, ManagedResourceControlName)
}

func (i *istiod) WaitCleanup(ctx context.Context) error {
	timeoutCtx, cancel := context.WithTimeout(ctx, TimeoutWaitForManagedResource)
	defer cancel()

	return managedresources.WaitUntilDeleted(timeoutCtx, i.client, i.namespace, ManagedResourceControlName)
}

func (i *istiod) generateIstiodChart() (*chartrenderer.RenderedChart, error) {
	return i.chartRenderer.RenderEmbeddedFS(chartIstiod, chartPathIstiod, ManagedResourceControlName, i.namespace, map[string]interface{}{
		"serviceName": istiodServiceName,
		"trustDomain": i.values.TrustDomain,
		"labels": map[string]interface{}{
			"app":   "istiod",
			"istio": "pilot",
		},
		"deployNamespace":   false,
		"priorityClassName": "istiod",
		"ports": map[string]interface{}{
			"https": 10250,
		},
		"portsNames": map[string]interface{}{
			"metrics": istiodServicePortNameMetrics,
		},
		"image": i.values.Image,
	})
}

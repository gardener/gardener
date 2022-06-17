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
	"time"

	"github.com/gardener/gardener/pkg/chartrenderer"
	"github.com/gardener/gardener/pkg/features"
	gardenletfeatures "github.com/gardener/gardener/pkg/gardenlet/features"
	"github.com/gardener/gardener/pkg/operation/botanist/component"
	kutil "github.com/gardener/gardener/pkg/utils/kubernetes"
	"github.com/gardener/gardener/pkg/utils/managedresources"

	networkingv1alpha3 "istio.io/client-go/pkg/apis/networking/v1alpha3"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	crclient "sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	// ManagedResourceIstio is the name of the ManagedResource containing the resource specifications.
	ManagedResourceIstio = "istio"
)

type istiod struct {
	client                    crclient.Client
	chartRenderer             chartrenderer.Interface
	namespace                 string
	values                    *IstiodValues
	chartPath                 string
	istioIngressGatewayValues []*IngressGateway
	istioProxyProtocolValues  []*IstioProxyProtocol
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
	values *IstiodValues,
	namespace string,
	chartsRootPath string,
	istioIngressGatewayValues []*IngressGateway,
	istioProxyProtocolValues []*IstioProxyProtocol,
) component.DeployWaiter {
	return &istiod{
		client:                    client,
		chartRenderer:             chartRenderer,
		values:                    values,
		namespace:                 namespace,
		chartPath:                 filepath.Join(chartsRootPath, istioReleaseName, "istio-istiod"),
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

	// TODO(mvladev): Rotate this on on every istio version upgrade.
	for _, filterName := range []string{"tcp-metadata-exchange-1.9", "tcp-metadata-exchange-1.10", "metadata-exchange-1.9", "metadata-exchange-1.10", "tcp-stats-filter-1.9", "stats-filter-1.9"} {
		if err := crclient.IgnoreNotFound(i.client.Delete(ctx, &networkingv1alpha3.EnvoyFilter{
			ObjectMeta: metav1.ObjectMeta{Name: filterName, Namespace: i.namespace},
		})); err != nil {
			return err
		}
	}

	renderedIstiodChart, err := i.generateIstioIstiodChart(ctx)
	if err != nil {
		return err
	}

	renderedIstioIngressGatewayChart, err := i.generateIstioIngressGatewayChart(ctx)
	if err != nil {
		return err
	}

	renderedIstioProxyProtocolChart, err := i.generateIstioProxyProtocolChart(ctx)
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

	return managedresources.CreateForSeed(ctx, i.client, i.namespace, ManagedResourceIstio, false, renderedChart.AsSecretData())
}

func (i *istiod) Destroy(ctx context.Context) error {

	if err := managedresources.DeleteForSeed(ctx, i.client, i.namespace, ManagedResourceIstio); err != nil {
		return err
	}

	//delete the namespaces
	if err := i.client.Delete(ctx, &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: i.namespace,
		},
	}); err != nil {
		if !apierrors.IsNotFound(err) {
			return err
		}
	}

	for _, istioIngressGateway := range i.istioIngressGatewayValues {
		if err := i.client.Delete(ctx, &corev1.Namespace{
			ObjectMeta: metav1.ObjectMeta{
				Name: istioIngressGateway.Namespace,
			},
		}); err != nil {
			if !apierrors.IsNotFound(err) {
				return err
			}
		}
	}

	return nil
}

// TimeoutWaitForManagedResource is the timeout used while waiting for the ManagedResources to become healthy
// or deleted.
var TimeoutWaitForManagedResource = 2 * time.Minute

func (i *istiod) Wait(ctx context.Context) error {
	return nil
}

func (i *istiod) WaitCleanup(ctx context.Context) error {
	timeoutCtx, cancel := context.WithTimeout(ctx, TimeoutWaitForManagedResource)
	defer cancel()

	return managedresources.WaitUntilDeleted(timeoutCtx, i.client, i.namespace, ManagedResourceIstio)
}

func (i *istiod) generateIstioIstiodChart(ctx context.Context) (*chartrenderer.RenderedChart, error) {

	values := map[string]interface{}{
		"trustDomain":       i.values.TrustDomain,
		"labels":            map[string]interface{}{"app": "istiod", "istio": "pilot"},
		"deployNamespace":   false,
		"priorityClassName": "istio",
		"ports":             map[string]interface{}{"https": 10250},
		"image":             i.values.Image,
	}

	return i.chartRenderer.Render(i.chartPath, ManagedResourceIstio, i.namespace, values)
}

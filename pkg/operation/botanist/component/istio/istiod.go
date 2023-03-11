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
	"fmt"
	"path/filepath"
	"strings"
	"time"

	networkingv1alpha3 "istio.io/client-go/pkg/apis/networking/v1alpha3"
	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	resourcesv1alpha1 "github.com/gardener/gardener/pkg/apis/resources/v1alpha1"
	"github.com/gardener/gardener/pkg/chartrenderer"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	"github.com/gardener/gardener/pkg/controllerutils"
	"github.com/gardener/gardener/pkg/operation/botanist/component"
	"github.com/gardener/gardener/pkg/utils/managedresources"
)

const (
	// ManagedResourceControlName is the name of the ManagedResource containing the resource specifications.
	ManagedResourceControlName = "istio"
	// ManagedResourceIstioSystemName is the name of the ManagedResource containing Istio-System resource specifications.
	ManagedResourceIstioSystemName = "istio-system"
	// DefaultZoneKey is the label key for the istio default ingress gateway.
	DefaultZoneKey = "istio"
	// IstiodServiceName is the name of the istiod service.
	IstiodServiceName            = "istiod"
	istiodServicePortNameMetrics = "metrics"
	// PortWebhookServer is the port of the validating webhook server.
	PortWebhookServer = 10250

	releaseName = "istio"
)

var (
	//go:embed charts/istio/istio-istiod
	chartIstiod     embed.FS
	chartPathIstiod = filepath.Join("charts", "istio", "istio-istiod")
)

type istiod struct {
	client        client.Client
	chartRenderer chartrenderer.Interface
	values        Values
}

// IstiodValues contains configuration values for the Istiod component.
type IstiodValues struct {
	// Enabled controls if `istiod` is deployed.
	Enabled bool
	// Namespace (a.k.a. Istio-System namespace) is the namespace `istiod` is deployed to.
	Namespace string
	// Image is the image used for the `istiod` deployment.
	Image string
	// TrustDomain is the domain used for service discovery, e.g. `cluster.local`.
	TrustDomain string
	// Zones are the availability zones used for this `istiod` deployment.
	Zones []string
}

// Values contains configuration values for the Istio component.
type Values struct {
	// Istiod are configuration values for the istiod chart.
	Istiod IstiodValues
	// IngressGateway are configuration values for ingress gateway deployments and objects.
	IngressGateway []IngressGatewayValues
	// Suffix can be used to append arbitrary identifiers to resources which are deployed to common namespaces.
	Suffix string
}

// NewIstio can be used to deploy istio's istiod in a namespace.
// Destroy does nothing.
func NewIstio(
	client client.Client,
	chartRenderer chartrenderer.Interface,
	values Values,
) component.DeployWaiter {
	return &istiod{
		client:        client,
		chartRenderer: chartRenderer,
		values:        values,
	}
}

func (i *istiod) deployIstiod(ctx context.Context) error {
	if !i.values.Istiod.Enabled {
		return nil
	}

	istiodNamespace := &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: i.values.Istiod.Namespace}}
	if _, err := controllerutils.CreateOrGetAndMergePatch(ctx, i.client, istiodNamespace, func() error {
		metav1.SetMetaDataLabel(&istiodNamespace.ObjectMeta, "istio-operator-managed", "Reconcile")
		metav1.SetMetaDataLabel(&istiodNamespace.ObjectMeta, "istio-injection", "disabled")
		metav1.SetMetaDataLabel(&istiodNamespace.ObjectMeta, resourcesv1alpha1.HighAvailabilityConfigConsider, "true")
		metav1.SetMetaDataLabel(&istiodNamespace.ObjectMeta, v1beta1constants.GardenRole, v1beta1constants.GardenRoleIstioSystem)
		metav1.SetMetaDataAnnotation(&istiodNamespace.ObjectMeta, resourcesv1alpha1.HighAvailabilityConfigZones, strings.Join(i.values.Istiod.Zones, ","))
		return nil
	}); err != nil {
		return err
	}

	renderedIstiodChart, err := i.generateIstiodChart(false)
	if err != nil {
		return err
	}

	return managedresources.CreateForSeed(ctx, i.client, i.values.Istiod.Namespace, resourceName(ManagedResourceIstioSystemName, i.values.Suffix), false, renderedIstiodChart.AsSecretData())
}

func (i *istiod) Deploy(ctx context.Context) error {
	if err := i.deployIstiod(ctx); err != nil {
		return err
	}

	var renderedChart = &chartrenderer.RenderedChart{}

	// TODO(timuthy): Block to be removed in a future version. Only required to move istiod assets to separate ManagedResource.
	if i.values.Istiod.Enabled {
		renderedIstiodChartIgnore, err := i.generateIstiodChart(true)
		if err != nil {
			return err
		}

		renderedChart.Manifests = append(renderedChart.Manifests, renderedIstiodChartIgnore.Manifests...)
	}

	// TODO(mvladev): Rotate this on every istio version upgrade.
	for _, ingressGateway := range i.values.IngressGateway {
		for _, filterName := range []string{"tcp-stats-filter-1.11", "stats-filter-1.11", "tcp-stats-filter-1.12", "stats-filter-1.12"} {
			if err := client.IgnoreNotFound(i.client.Delete(ctx, &networkingv1alpha3.EnvoyFilter{
				ObjectMeta: metav1.ObjectMeta{Name: filterName, Namespace: ingressGateway.Namespace},
			})); err != nil {
				return err
			}
		}
	}

	renderedIstioIngressGatewayChart, err := i.generateIstioIngressGatewayChart()
	if err != nil {
		return err
	}

	for _, istioIngressGateway := range i.values.IngressGateway {
		gatewayNamespace := &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: istioIngressGateway.Namespace}}
		if _, err := controllerutils.CreateOrGetAndMergePatch(ctx, i.client, gatewayNamespace, func() error {
			metav1.SetMetaDataLabel(&gatewayNamespace.ObjectMeta, "istio-operator-managed", "Reconcile")
			metav1.SetMetaDataLabel(&gatewayNamespace.ObjectMeta, "istio-injection", "disabled")
			metav1.SetMetaDataLabel(&gatewayNamespace.ObjectMeta, v1beta1constants.GardenRole, v1beta1constants.GardenRoleIstioIngress)

			if value, ok := istioIngressGateway.Labels[v1beta1constants.GardenRole]; ok && strings.HasPrefix(value, v1beta1constants.GardenRoleExposureClassHandler) {
				metav1.SetMetaDataLabel(&gatewayNamespace.ObjectMeta, v1beta1constants.GardenRole, value)
			}
			if value, ok := istioIngressGateway.Labels[v1beta1constants.LabelExposureClassHandlerName]; ok {
				metav1.SetMetaDataLabel(&gatewayNamespace.ObjectMeta, v1beta1constants.LabelExposureClassHandlerName, value)
			}

			if value, ok := istioIngressGateway.Labels[DefaultZoneKey]; ok {
				metav1.SetMetaDataLabel(&gatewayNamespace.ObjectMeta, DefaultZoneKey, value)
			}

			metav1.SetMetaDataLabel(&gatewayNamespace.ObjectMeta, resourcesv1alpha1.HighAvailabilityConfigConsider, "true")
			zones := i.values.Istiod.Zones
			if len(istioIngressGateway.Zones) > 0 {
				zones = istioIngressGateway.Zones
			}
			metav1.SetMetaDataAnnotation(&gatewayNamespace.ObjectMeta, resourcesv1alpha1.HighAvailabilityConfigZones, strings.Join(zones, ","))
			if len(zones) == 1 {
				metav1.SetMetaDataAnnotation(&gatewayNamespace.ObjectMeta, resourcesv1alpha1.HighAvailabilityConfigZonePinning, "true")
			}
			return nil
		}); err != nil {
			return err
		}
	}

	renderedChart.Manifests = append(renderedChart.Manifests, renderedIstioIngressGatewayChart.Manifests...)

	registry := managedresources.NewRegistry(kubernetes.SeedScheme, kubernetes.SeedCodec, kubernetes.SeedSerializer)

	for _, istioIngressGateway := range i.values.IngressGateway {
		for _, transformer := range getIstioIngressNetworkPolicyTransformers() {
			obj := &networkingv1.NetworkPolicy{
				ObjectMeta: metav1.ObjectMeta{
					Name:      transformer.name,
					Namespace: istioIngressGateway.Namespace,
				},
			}

			if err := transformer.transform(obj)(); err != nil {
				return err
			}

			if err := registry.Add(obj); err != nil {
				return err
			}
		}
	}

	chartsMap := renderedChart.AsSecretData()
	objMap := registry.SerializedObjects()

	for key := range objMap {
		chartsMap[key] = objMap[key]
	}

	return managedresources.CreateForSeed(ctx, i.client, i.values.Istiod.Namespace, resourceName(ManagedResourceControlName, i.values.Suffix), false, chartsMap)
}

func (i *istiod) Destroy(ctx context.Context) error {
	if i.values.Istiod.Enabled {
		if err := managedresources.DeleteForSeed(ctx, i.client, i.values.Istiod.Namespace, ManagedResourceIstioSystemName); err != nil {
			return err
		}
		if err := i.client.Delete(ctx, &corev1.Namespace{
			ObjectMeta: metav1.ObjectMeta{
				Name: i.values.Istiod.Namespace,
			},
		}); client.IgnoreNotFound(err) != nil {
			return err
		}
	}

	if err := managedresources.DeleteForSeed(ctx, i.client, i.values.Istiod.Namespace, resourceName(ManagedResourceControlName, i.values.Suffix)); err != nil {
		return err
	}

	for _, istioIngressGateway := range i.values.IngressGateway {
		if err := i.client.Delete(ctx, &corev1.Namespace{
			ObjectMeta: metav1.ObjectMeta{
				Name: istioIngressGateway.Namespace,
			},
		}); client.IgnoreNotFound(err) != nil {
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

	return managedresources.WaitUntilHealthy(timeoutCtx, i.client, i.values.Istiod.Namespace, ManagedResourceControlName)
}

func (i *istiod) WaitCleanup(ctx context.Context) error {
	timeoutCtx, cancel := context.WithTimeout(ctx, TimeoutWaitForManagedResource)
	defer cancel()

	return managedresources.WaitUntilDeleted(timeoutCtx, i.client, i.values.Istiod.Namespace, ManagedResourceControlName)
}

func (i *istiod) generateIstiodChart(ignoreMode bool) (*chartrenderer.RenderedChart, error) {
	return i.chartRenderer.RenderEmbeddedFS(chartIstiod, chartPathIstiod, releaseName, i.values.Istiod.Namespace, map[string]interface{}{
		"serviceName": IstiodServiceName,
		"trustDomain": i.values.Istiod.TrustDomain,
		"labels": map[string]interface{}{
			"app":   "istiod",
			"istio": "pilot",
		},
		"deployNamespace":   false,
		"priorityClassName": "istiod",
		"ports": map[string]interface{}{
			"https": PortWebhookServer,
		},
		"portsNames": map[string]interface{}{
			"metrics": istiodServicePortNameMetrics,
		},
		"image":      i.values.Istiod.Image,
		"ignoreMode": ignoreMode,
	})
}

func resourceName(name, suffix string) string {
	if len(suffix) == 0 {
		return name
	}
	return fmt.Sprintf("%s-%s", name, suffix)
}

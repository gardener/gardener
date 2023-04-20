// Copyright 2020 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	resourcesv1alpha1 "github.com/gardener/gardener/pkg/apis/resources/v1alpha1"
	"github.com/gardener/gardener/pkg/chartrenderer"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	"github.com/gardener/gardener/pkg/controllerutils"
	"github.com/gardener/gardener/pkg/operation/botanist/component"
	"github.com/gardener/gardener/pkg/utils/flow"
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
	IstiodServiceName = "istiod"
	// PortWebhookServer is the port of the validating webhook server.
	PortWebhookServer = 10250

	istiodServicePortNameMetrics = "metrics"
	releaseName                  = "istio"
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

	managedResourceIstioIngressName string
	managedResourceIstioSystemName  string
}

// IstiodValues contains configuration values for the Istiod component.
type IstiodValues struct {
	// Enabled controls if `istiod` is deployed.
	Enabled bool
	// Namespace (a.k.a. Istio-System namespace) is the namespace `istiod` is deployed to.
	Namespace string
	// Image is the image used for the `istiod` deployment.
	Image string
	// PriorityClassName is the name of the priority class used for the Istiod deployment.
	PriorityClassName string
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
	// NamePrefix can be used to prepend arbitrary identifiers to resources which are deployed to common namespaces.
	NamePrefix string
}

// Interface contains functions for an Istio deployer.
type Interface interface {
	component.DeployWaiter
	// AddIngressGateway adds another ingress gateway to the existing Istio deployer.
	AddIngressGateway(values IngressGatewayValues)
	// GetValues returns the configured values of the Istio deployer.
	GetValues() Values
}

var _ Interface = (*istiod)(nil)

// NewIstio can be used to deploy istio's istiod in a namespace.
// Destroy does nothing.
func NewIstio(
	client client.Client,
	chartRenderer chartrenderer.Interface,
	values Values,
) Interface {
	return &istiod{
		client:        client,
		chartRenderer: chartRenderer,
		values:        values,

		managedResourceIstioIngressName: resourceName(values.NamePrefix, ManagedResourceControlName),
		managedResourceIstioSystemName:  resourceName(values.NamePrefix, ManagedResourceIstioSystemName),
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

	return managedresources.CreateForSeed(ctx, i.client, i.values.Istiod.Namespace, i.managedResourceIstioSystemName, false, renderedIstiodChart.AsSecretData())
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

	var (
		registry  = managedresources.NewRegistry(kubernetes.SeedScheme, kubernetes.SeedCodec, kubernetes.SeedSerializer)
		chartsMap = renderedChart.AsSecretData()
		objMap    = registry.SerializedObjects()
	)

	for key := range objMap {
		chartsMap[key] = objMap[key]
	}

	return managedresources.CreateForSeed(ctx, i.client, i.values.Istiod.Namespace, i.managedResourceIstioIngressName, false, chartsMap)
}

func (i *istiod) Destroy(ctx context.Context) error {
	for _, mr := range ManagedResourceNames(i.values.Istiod.Enabled, i.values.NamePrefix) {
		if err := managedresources.DeleteForSeed(ctx, i.client, i.values.Istiod.Namespace, mr); err != nil {
			return err
		}
	}

	if i.values.Istiod.Enabled {
		if err := i.client.Delete(ctx, &corev1.Namespace{
			ObjectMeta: metav1.ObjectMeta{
				Name: i.values.Istiod.Namespace,
			},
		}); client.IgnoreNotFound(err) != nil {
			return err
		}
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

	managedResources := ManagedResourceNames(i.values.Istiod.Enabled, i.values.NamePrefix)
	taskFns := make([]flow.TaskFn, 0, len(managedResources))
	for _, mr := range managedResources {
		name := mr
		taskFns = append(taskFns, func(ctx context.Context) error {
			return managedresources.WaitUntilHealthy(ctx, i.client, i.values.Istiod.Namespace, name)
		})
	}

	return flow.Parallel(taskFns...)(timeoutCtx)
}

func (i *istiod) WaitCleanup(ctx context.Context) error {
	timeoutCtx, cancel := context.WithTimeout(ctx, TimeoutWaitForManagedResource)
	defer cancel()

	managedResources := ManagedResourceNames(i.values.Istiod.Enabled, i.values.NamePrefix)
	taskFns := make([]flow.TaskFn, 0, len(managedResources))
	for _, mr := range managedResources {
		name := mr
		taskFns = append(taskFns, func(ctx context.Context) error {
			return managedresources.WaitUntilDeleted(timeoutCtx, i.client, i.values.Istiod.Namespace, name)
		})
	}

	return flow.Parallel(taskFns...)(timeoutCtx)
}

func (i *istiod) AddIngressGateway(values IngressGatewayValues) {
	i.values.IngressGateway = append(i.values.IngressGateway, values)
}

func (i *istiod) GetValues() Values {
	return i.values
}

func (i *istiod) generateIstiodChart(ignoreMode bool) (*chartrenderer.RenderedChart, error) {
	istiodValues := i.values.Istiod

	return i.chartRenderer.RenderEmbeddedFS(chartIstiod, chartPathIstiod, releaseName, i.values.Istiod.Namespace, map[string]interface{}{
		"serviceName": IstiodServiceName,
		"trustDomain": istiodValues.TrustDomain,
		"labels": map[string]interface{}{
			"app":   "istiod",
			"istio": "pilot",
		},
		"deployNamespace":   false,
		"priorityClassName": istiodValues.PriorityClassName,
		"ports": map[string]interface{}{
			"https": PortWebhookServer,
		},
		"portsNames": map[string]interface{}{
			"metrics": istiodServicePortNameMetrics,
		},
		"image":      istiodValues.Image,
		"ignoreMode": ignoreMode,
	})
}

// ManagedResourceNames returns the names of the `ManagedResource`s being used by Istio.
func ManagedResourceNames(istiodEnabled bool, namePrefix string) []string {
	names := []string{resourceName(namePrefix, ManagedResourceControlName)}
	if istiodEnabled {
		names = append(names, resourceName(namePrefix, ManagedResourceIstioSystemName))
	}
	return names
}

func resourceName(prefix, name string) string {
	return fmt.Sprintf("%s%s", prefix, name)
}

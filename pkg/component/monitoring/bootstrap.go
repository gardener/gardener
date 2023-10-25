// Copyright 2023 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

package monitoring

import (
	"context"
	"embed"
	"fmt"
	"path/filepath"
	"strings"

	istionetworkingv1beta1 "istio.io/client-go/pkg/apis/networking/v1beta1"
	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	vpaautoscalingv1 "k8s.io/autoscaler/vertical-pod-autoscaler/pkg/apis/autoscaling.k8s.io/v1"
	"k8s.io/utils/pointer"
	"sigs.k8s.io/controller-runtime/pkg/client"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	"github.com/gardener/gardener/pkg/component"
	"github.com/gardener/gardener/pkg/component/etcd"
	"github.com/gardener/gardener/pkg/component/hvpa"
	"github.com/gardener/gardener/pkg/component/istio"
	"github.com/gardener/gardener/pkg/component/kubestatemetrics"
	"github.com/gardener/gardener/pkg/utils"
	istioutils "github.com/gardener/gardener/pkg/utils/istio"
	kubernetesutils "github.com/gardener/gardener/pkg/utils/kubernetes"
	"github.com/gardener/gardener/pkg/utils/managedresources"
	secretsutils "github.com/gardener/gardener/pkg/utils/secrets"
	secretsmanager "github.com/gardener/gardener/pkg/utils/secrets/manager"
)

const (
	aggregatePrometheusName                = "aggregate-" + prometheusName
	managedResourceNameAggregatePrometheus = aggregatePrometheusName
)

var (
	//go:embed charts/bootstrap
	chartBootstrap     embed.FS
	chartPathBootstrap = filepath.Join("charts", "bootstrap")
)

// ValuesBootstrap is a set of configuration values for the monitoring components.
type ValuesBootstrap struct {
	// AlertingSMTPSecret is the alerting SMTP secret..
	AlertingSMTPSecret *corev1.Secret
	// GlobalMonitoringSecret is the global monitoring secret for the garden cluster.
	GlobalMonitoringSecret *corev1.Secret
	// HVPAEnabled states whether HVPA is enabled or not.
	HVPAEnabled bool
	// ImageAlertmanager is the image of Alertmanager.
	ImageAlertmanager string
	// ImageAlpine is the image of Alpine.
	ImageAlpine string
	// ImageConfigmapReloader is the image of ConfigmapReloader.
	ImageConfigmapReloader string
	// ImagePrometheus is the image of Prometheus.
	ImagePrometheus string
	// IngressHost is the host name of Prometheus.
	IngressHost string
	// SeedName is the name of the seed.
	SeedName string
	// StorageCapacityAlertmanager is the storage capacity of Alertmanager.
	StorageCapacityAlertmanager string
	// StorageCapacityPrometheus is the storage capacity of Prometheus.
	StorageCapacityPrometheus string
	// StorageCapacityAggregatePrometheus is the storage capacity of AggregatePrometheus.
	StorageCapacityAggregatePrometheus string
	// WildcardCert is the wildcard tls certificate which is issued for the seed's ingress domain.
	WildcardCert *corev1.Secret
	// IstioIngressGatewayLabels are the labels for identifying the used istio ingress gateway.
	IstioIngressGatewayLabels map[string]string
	// IstioIngressGatewayNamespace is the namespace of the used istio ingress gateway.
	IstioIngressGatewayNamespace string
}

// NewBootstrap creates a new instance of Deployer for the monitoring components.
func NewBootstrap(
	client client.Client,
	chartApplier kubernetes.ChartApplier,
	secretsManager secretsmanager.Interface,
	namespace string,
	values ValuesBootstrap,
) component.Deployer {
	return &bootstrapper{
		client:         client,
		chartApplier:   chartApplier,
		namespace:      namespace,
		secretsManager: secretsManager,
		values:         values,
	}
}

type bootstrapper struct {
	client         client.Client
	chartApplier   kubernetes.ChartApplier
	namespace      string
	secretsManager secretsmanager.Interface
	values         ValuesBootstrap
}

func (b *bootstrapper) Deploy(ctx context.Context) error {
	if b.values.HVPAEnabled {
		if err := kubernetesutils.DeleteObjects(ctx, b.client,
			&vpaautoscalingv1.VerticalPodAutoscaler{ObjectMeta: metav1.ObjectMeta{Name: "prometheus-vpa", Namespace: b.namespace}},
			&vpaautoscalingv1.VerticalPodAutoscaler{ObjectMeta: metav1.ObjectMeta{Name: "aggregate-prometheus-vpa", Namespace: b.namespace}},
		); err != nil {
			return err
		}
	}

	// Fetch component-specific aggregate and central monitoring configuration
	var (
		aggregateScrapeConfigs                = strings.Builder{}
		aggregateMonitoringComponentFunctions = []component.AggregateMonitoringConfiguration{
			istio.AggregateMonitoringConfiguration,
		}

		centralScrapeConfigs                            = strings.Builder{}
		centralCAdvisorScrapeConfigMetricRelabelConfigs = strings.Builder{}
		centralMonitoringComponentFunctions             = []component.CentralMonitoringConfiguration{
			etcd.CentralMonitoringConfiguration,
			hvpa.CentralMonitoringConfiguration,
			kubestatemetrics.CentralMonitoringConfiguration,
		}
	)

	for _, componentFn := range aggregateMonitoringComponentFunctions {
		aggregateMonitoringConfig, err := componentFn()
		if err != nil {
			return err
		}

		for _, config := range aggregateMonitoringConfig.ScrapeConfigs {
			aggregateScrapeConfigs.WriteString(fmt.Sprintf("- %s\n", utils.Indent(config, 2)))
		}
	}

	for _, componentFn := range centralMonitoringComponentFunctions {
		centralMonitoringConfig, err := componentFn()
		if err != nil {
			return err
		}

		for _, config := range centralMonitoringConfig.ScrapeConfigs {
			centralScrapeConfigs.WriteString(fmt.Sprintf("- %s\n", utils.Indent(config, 2)))
		}

		for _, config := range centralMonitoringConfig.CAdvisorScrapeConfigMetricRelabelConfigs {
			centralCAdvisorScrapeConfigMetricRelabelConfigs.WriteString(fmt.Sprintf("- %s\n", utils.Indent(config, 2)))
		}
	}

	// Monitoring resource values
	monitoringResources := map[string]interface{}{
		"prometheus":           map[string]interface{}{},
		"aggregate-prometheus": map[string]interface{}{},
	}

	if b.values.HVPAEnabled {
		for resource := range monitoringResources {
			currentResources, err := kubernetesutils.GetContainerResourcesInStatefulSet(ctx, b.client, kubernetesutils.Key(b.namespace, resource))
			if err != nil {
				return err
			}
			if len(currentResources) != 0 && currentResources["prometheus"] != nil {
				monitoringResources[resource] = map[string]interface{}{
					"prometheus": currentResources["prometheus"],
				}
			}
		}
	}

	// AlertManager configuration
	alertManagerConfig := map[string]interface{}{
		"storage": b.values.StorageCapacityAlertmanager,
	}

	if b.values.AlertingSMTPSecret != nil {
		emailConfig := map[string]interface{}{
			"to":            string(b.values.AlertingSMTPSecret.Data["to"]),
			"from":          string(b.values.AlertingSMTPSecret.Data["from"]),
			"smarthost":     string(b.values.AlertingSMTPSecret.Data["smarthost"]),
			"auth_username": string(b.values.AlertingSMTPSecret.Data["auth_username"]),
			"auth_identity": string(b.values.AlertingSMTPSecret.Data["auth_identity"]),
			"auth_password": string(b.values.AlertingSMTPSecret.Data["auth_password"]),
		}
		alertManagerConfig["enabled"] = true
		alertManagerConfig["emailConfigs"] = []map[string]interface{}{emailConfig}
	} else {
		alertManagerConfig["enabled"] = false
		if err := deleteAlertmanager(ctx, b.client, b.namespace); err != nil {
			return err
		}
	}

	var (
		vpaGK    = schema.GroupKind{Group: "autoscaling.k8s.io", Kind: "VerticalPodAutoscaler"}
		hvpaGK   = schema.GroupKind{Group: "autoscaling.k8s.io", Kind: "Hvpa"}
		issuerGK = schema.GroupKind{Group: "certmanager.k8s.io", Kind: "ClusterIssuer"}

		applierOptions          = kubernetes.CopyApplierOptions(kubernetes.DefaultMergeFuncs)
		retainStatusInformation = func(new, old *unstructured.Unstructured) {
			// Apply status from old Object to retain status information
			new.Object["status"] = old.Object["status"]
		}
	)

	applierOptions[vpaGK] = retainStatusInformation
	applierOptions[hvpaGK] = retainStatusInformation
	applierOptions[issuerGK] = retainStatusInformation

	var aggregatePrometheusIngressTLSSecret *corev1.Secret
	if b.values.WildcardCert != nil {
		aggregatePrometheusIngressTLSSecret = b.values.WildcardCert
	} else {
		ingressTLSSecret, err := b.secretsManager.Generate(ctx, &secretsutils.CertificateSecretConfig{
			Name:                        "aggregate-prometheus-tls",
			CommonName:                  "prometheus",
			Organization:                []string{"gardener.cloud:monitoring:ingress"},
			DNSNames:                    []string{b.values.IngressHost},
			CertType:                    secretsutils.ServerCert,
			Validity:                    pointer.Duration(v1beta1constants.IngressTLSCertificateValidity),
			SkipPublishingCACertificate: true,
		}, secretsmanager.SignedByCA(v1beta1constants.SecretNameCASeed))
		if err != nil {
			return err
		}
		aggregatePrometheusIngressTLSSecret = ingressTLSSecret
	}

	values := kubernetes.Values(map[string]interface{}{
		"global": map[string]interface{}{
			"images": map[string]string{
				"alertmanager":       b.values.ImageAlertmanager,
				"alpine":             b.values.ImageAlpine,
				"configmap-reloader": b.values.ImageConfigmapReloader,
				"prometheus":         b.values.ImagePrometheus,
			},
		},
		"prometheus": map[string]interface{}{
			"resources":               monitoringResources["prometheus"],
			"storage":                 b.values.StorageCapacityPrometheus,
			"additionalScrapeConfigs": centralScrapeConfigs.String(),
			"additionalCAdvisorScrapeConfigMetricRelabelConfigs": centralCAdvisorScrapeConfigMetricRelabelConfigs.String(),
		},
		"aggregatePrometheus": map[string]interface{}{
			"resources":               monitoringResources["aggregate-prometheus"],
			"storage":                 b.values.StorageCapacityAggregatePrometheus,
			"seed":                    b.values.SeedName,
			"additionalScrapeConfigs": aggregateScrapeConfigs.String(),
		},
		"alertmanager": alertManagerConfig,
		"hvpa": map[string]interface{}{
			"enabled": b.values.HVPAEnabled,
		},
		"istio": map[string]interface{}{
			"enabled": true,
		},
	})

	istioTLSSecret := aggregatePrometheusIngressTLSSecret.DeepCopy()
	istioTLSSecret.Type = aggregatePrometheusIngressTLSSecret.Type
	istioTLSSecret.ObjectMeta = metav1.ObjectMeta{
		Name:      fmt.Sprintf("seed-%s", aggregatePrometheusIngressTLSSecret.Name),
		Namespace: b.values.IstioIngressGatewayNamespace,
		Labels:    b.getIstioTLSSecretLabels(),
	}
	if err := b.ensureIstioTLSSecret(ctx, istioTLSSecret); err != nil {
		return err
	}

	gateway := &istionetworkingv1beta1.Gateway{
		ObjectMeta: metav1.ObjectMeta{
			Name:      aggregatePrometheusName,
			Namespace: b.namespace,
		},
	}
	if err := istioutils.GatewayWithTLSTermination(gateway, getAggregatePrometheusLabels(), b.values.IstioIngressGatewayLabels, []string{b.values.IngressHost}, externalPort, istioTLSSecret.Name)(); err != nil {
		return err
	}

	virtualService := &istionetworkingv1beta1.VirtualService{
		ObjectMeta: metav1.ObjectMeta{
			Name:      aggregatePrometheusName,
			Namespace: b.namespace,
		},
	}
	destinationHost := fmt.Sprintf("%s-web.%s.svc.%s", aggregatePrometheusName, b.namespace, gardencorev1beta1.DefaultDomain)
	if err := istioutils.VirtualServiceWithSNIMatchAndBasicAuth(virtualService, getAggregatePrometheusLabels(), []string{b.values.IngressHost}, aggregatePrometheusName, externalPort, destinationHost, prometheusServicePort, string(b.values.GlobalMonitoringSecret.Data[corev1.BasicAuthUsernameKey]), string(b.values.GlobalMonitoringSecret.Data[corev1.BasicAuthPasswordKey]))(); err != nil {
		return err
	}

	destinationRule := &istionetworkingv1beta1.DestinationRule{
		ObjectMeta: metav1.ObjectMeta{
			Name:      aggregatePrometheusName,
			Namespace: b.namespace,
		},
	}
	if err := istioutils.DestinationRuleWithLocalityPreference(destinationRule, getAggregatePrometheusLabels(), destinationHost)(); err != nil {
		return err
	}

	registry := managedresources.NewRegistry(kubernetes.SeedScheme, kubernetes.SeedCodec, kubernetes.SeedSerializer)
	data, err := registry.AddAllAndSerialize(
		gateway,
		virtualService,
		destinationRule,
	)
	if err != nil {
		return err
	}
	if err := managedresources.CreateForSeed(ctx, b.client, b.namespace, managedResourceNameAggregatePrometheus, false, data); err != nil {
		return err
	}

	// TODO(scheererj): Remove with next release after all ingress objects have been deleted.
	if err := kubernetesutils.DeleteObjects(ctx, b.client, &networkingv1.Ingress{
		ObjectMeta: metav1.ObjectMeta{
			Name:      aggregatePrometheusName,
			Namespace: b.namespace,
		},
	}); err != nil {
		return err
	}

	if err := b.chartApplier.ApplyFromEmbeddedFS(ctx, chartBootstrap, chartPathBootstrap, b.namespace, "monitoring", values, applierOptions); err != nil {
		return err
	}

	return b.cleanupOldIstioTLSSecrets(ctx, istioTLSSecret)
}

func (b *bootstrapper) Destroy(ctx context.Context) error {
	if err := managedresources.DeleteForSeed(ctx, b.client, b.namespace, managedResourceNameAggregatePrometheus); err != nil {
		return err
	}

	return b.cleanupOldIstioTLSSecrets(ctx, nil)
}

func getAggregatePrometheusLabels() map[string]string {
	return map[string]string{
		v1beta1constants.LabelApp:  aggregatePrometheusName,
		v1beta1constants.LabelRole: v1beta1constants.GardenRoleMonitoring,
	}
}

func (b *bootstrapper) getIstioTLSSecretLabels() map[string]string {
	return utils.MergeStringMaps(getAggregatePrometheusLabels(), map[string]string{labelTLSSecretOwner: "seed"})
}

func (b *bootstrapper) ensureIstioTLSSecret(ctx context.Context, tlsSecret *corev1.Secret) error {
	return ensureIstioTLSSecret(ctx, b.client, tlsSecret)
}

func (b *bootstrapper) cleanupOldIstioTLSSecrets(ctx context.Context, tlsSecret *corev1.Secret) error {
	return cleanupOldIstioTLSSecrets(ctx, b.client, tlsSecret, b.values.IstioIngressGatewayNamespace, b.getIstioTLSSecretLabels)
}

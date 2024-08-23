// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package shoot

import (
	_ "embed"

	monitoringv1 "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1"
	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"

	monitoringutils "github.com/gardener/gardener/pkg/component/observability/monitoring/utils"
)

var (
	// general rules
	//go:embed assets/prometheusrules/prometheus.yaml
	prometheusYAML []byte
	prometheus     *monitoringv1.PrometheusRule
	//go:embed assets/prometheusrules/verticalpodautoscaler.yaml
	vpaYAML []byte
	vpa     *monitoringv1.PrometheusRule

	// optional rules
	//go:embed assets/prometheusrules/optional/alertmanager.yaml
	alertManagerYAML []byte
	alertManager     *monitoringv1.PrometheusRule

	// worker rules
	//go:embed assets/prometheusrules/worker/kube-kubelet.yaml
	workerKubeKubeletYAML []byte
	workerKubeKubelet     *monitoringv1.PrometheusRule
	//go:embed assets/prometheusrules/worker/kube-pods.yaml
	workerKubePodsYAML []byte
	workerKubePods     *monitoringv1.PrometheusRule
	//go:embed assets/prometheusrules/worker/networking.yaml
	workerNetworkingYAML []byte
	workerNetworking     *monitoringv1.PrometheusRule

	// workerless rules
	//go:embed assets/prometheusrules/workerless/kube-pods.yaml
	workerlessKubePodsYAML []byte
	workerlessKubePods     *monitoringv1.PrometheusRule
	//go:embed assets/prometheusrules/workerless/networking.yaml
	workerlessNetworkingYAML []byte
	workerlessNetworking     *monitoringv1.PrometheusRule
)

func init() {
	// general rules
	prometheus = &monitoringv1.PrometheusRule{}
	utilruntime.Must(runtime.DecodeInto(monitoringutils.Decoder, prometheusYAML, prometheus))
	vpa = &monitoringv1.PrometheusRule{}
	utilruntime.Must(runtime.DecodeInto(monitoringutils.Decoder, vpaYAML, vpa))

	// optional rules
	alertManager = &monitoringv1.PrometheusRule{}
	utilruntime.Must(runtime.DecodeInto(monitoringutils.Decoder, alertManagerYAML, alertManager))

	// worker rules
	workerKubeKubelet = &monitoringv1.PrometheusRule{}
	utilruntime.Must(runtime.DecodeInto(monitoringutils.Decoder, workerKubeKubeletYAML, workerKubeKubelet))
	workerKubePods = &monitoringv1.PrometheusRule{}
	utilruntime.Must(runtime.DecodeInto(monitoringutils.Decoder, workerKubePodsYAML, workerKubePods))
	workerNetworking = &monitoringv1.PrometheusRule{}
	utilruntime.Must(runtime.DecodeInto(monitoringutils.Decoder, workerNetworkingYAML, workerNetworking))

	// workerless rules
	workerlessKubePods = &monitoringv1.PrometheusRule{}
	utilruntime.Must(runtime.DecodeInto(monitoringutils.Decoder, workerlessKubePodsYAML, workerlessKubePods))
	workerlessNetworking = &monitoringv1.PrometheusRule{}
	utilruntime.Must(runtime.DecodeInto(monitoringutils.Decoder, workerlessNetworkingYAML, workerlessNetworking))
}

// CentralPrometheusRules returns the central PrometheusRule resources for the shoot prometheus.
func CentralPrometheusRules(isWorkerless, wantsAlertmanager bool) []*monitoringv1.PrometheusRule {
	out := []*monitoringv1.PrometheusRule{
		prometheus.DeepCopy(),
		vpa.DeepCopy(),
	}

	if isWorkerless {
		out = append(out, workerlessKubePods.DeepCopy(), workerlessNetworking.DeepCopy())
	} else {
		out = append(out, workerKubeKubelet.DeepCopy(), workerKubePods.DeepCopy(), workerNetworking.DeepCopy())
	}

	if wantsAlertmanager {
		out = append(out, alertManager.DeepCopy())
	}

	return out
}

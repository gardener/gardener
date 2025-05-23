// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package garden

import (
	_ "embed"

	monitoringv1 "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/intstr"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/utils/ptr"

	monitoringutils "github.com/gardener/gardener/pkg/component/observability/monitoring/utils"
	"github.com/gardener/gardener/pkg/utils"
)

var (
	//go:embed assets/prometheusrules/auditlog.yaml
	auditLogYAML []byte
	auditLog     *monitoringv1.PrometheusRule

	//go:embed assets/prometheusrules/etcd.yaml
	etcdYAML []byte
	etcd     *monitoringv1.PrometheusRule

	//go:embed assets/prometheusrules/metering-meta.yaml
	meteringYAML []byte
	metering     *monitoringv1.PrometheusRule

	//go:embed assets/prometheusrules/recording.yaml
	recordingYAML []byte
	recording     *monitoringv1.PrometheusRule

	//go:embed assets/prometheusrules/seed.yaml
	seedYAML []byte
	seed     *monitoringv1.PrometheusRule

	//go:embed assets/prometheusrules/shoot.yaml
	shootYAML []byte
	shoot     *monitoringv1.PrometheusRule
)

func init() {
	auditLog = &monitoringv1.PrometheusRule{}
	utilruntime.Must(runtime.DecodeInto(monitoringutils.Decoder, auditLogYAML, auditLog))

	etcd = &monitoringv1.PrometheusRule{}
	utilruntime.Must(runtime.DecodeInto(monitoringutils.Decoder, etcdYAML, etcd))

	metering = &monitoringv1.PrometheusRule{}
	utilruntime.Must(runtime.DecodeInto(monitoringutils.Decoder, meteringYAML, metering))

	recording = &monitoringv1.PrometheusRule{}
	utilruntime.Must(runtime.DecodeInto(monitoringutils.Decoder, recordingYAML, recording))

	seed = &monitoringv1.PrometheusRule{}
	utilruntime.Must(runtime.DecodeInto(monitoringutils.Decoder, seedYAML, seed))

	shoot = &monitoringv1.PrometheusRule{}
	utilruntime.Must(runtime.DecodeInto(monitoringutils.Decoder, shootYAML, shoot))
}

// CentralPrometheusRules returns the central PrometheusRule resources for the garden prometheus.
func CentralPrometheusRules(isGardenerDiscoveryServerEnabled bool) []*monitoringv1.PrometheusRule {
	return []*monitoringv1.PrometheusRule{
		auditLog.DeepCopy(),
		etcd.DeepCopy(),
		gardenPrometheusRule(isGardenerDiscoveryServerEnabled).DeepCopy(),
		metering.DeepCopy(),
		recording.DeepCopy(),
		seed.DeepCopy(),
		shoot.DeepCopy(),
	}
}

func gardenPrometheusRule(isGardenerDiscoveryServerEnabled bool) *monitoringv1.PrometheusRule {
	getLabels := func(severity string) map[string]string {
		return map[string]string{
			"severity": severity,
			"topology": "garden",
		}
	}

	rules := []monitoringv1.Rule{
		{
			Alert:  "DashboardDown",
			Expr:   intstr.FromString("probe_success{job = \"blackbox-dashboard\",purpose = \"availability\"} == 0"),
			For:    ptr.To(monitoringv1.Duration("5m")),
			Labels: utils.MergeStringMaps(getLabels("critical"), map[string]string{"service": "dashboard"}),
			Annotations: map[string]string{
				"summary": "Dashboard is Down in landscape {{$externalLabels.landscape}}",
				"description": "The http health probe to the Gardener Dashboard failed for at least 5 " +
					"minutes (instance is {{$labels.instance}}). " +
					"(Status code is " +
					"{{printf `probe_http_status_code{job      = \"blackbox-dashboard\" " +
					",                                purpose  = \"availability\" " +
					",                                instance = \"%s\"}` " +
					"$labels.instance " +
					"| query | first | value}}).",
			},
		},
		{
			Alert:  "ApiServerDown",
			Expr:   intstr.FromString("probe_success{job = \"blackbox-apiserver\", purpose = \"availability\"} == 0"),
			For:    ptr.To(monitoringv1.Duration("2m")),
			Labels: utils.MergeStringMaps(getLabels("critical"), map[string]string{"service": "apiserver"}),
			Annotations: map[string]string{
				"summary": "ApiServer is Down in landscape {{$externalLabels.landscape}}",
				"description": "The http health probe to the Api Server failed for at least 2 minutes " +
					"(instance is {{$labels.instance}}). " +
					" " +
					"(Status code is " +
					"{{printf `probe_http_status_code{job      = \"blackbox-apiserver\" " +
					",                                purpose  = \"availability\" " +
					",                                instance = \"%s\"}` " +
					"$labels.instance | query | first | value}}).",
			},
		},
		{
			Alert:  "GardenerApiServerDown",
			Expr:   intstr.FromString("probe_success{job = \"blackbox-gardener-apiserver\",purpose = \"availability\"} == 0"),
			For:    ptr.To(monitoringv1.Duration("2m")),
			Labels: utils.MergeStringMaps(getLabels("critical"), map[string]string{"service": "gardener-apiserver"}),
			Annotations: map[string]string{
				"summary": "Gardener ApiServer is Down in landscape {{$externalLabels.landscape}}",
				"description": "The http health probe to the Gardener Api Server failed for at least 2 " +
					"minutes (instance is {{$labels.instance}}). " +
					" " +
					"(Status code is " +
					"{{printf `probe_http_status_code{job      = \"blackbox-gardener-apiserver\" " +
					",                                purpose  = \"availability\" " +
					",                                instance = \"%s\"}` " +
					"$labels.instance " +
					"| query | first | value}}).",
			},
		},
		{
			Alert: "ProjectStuckInDeletion",
			Expr: intstr.FromString("avg(garden_projects_status{phase=\"Terminating\"}) without (instance) " +
				"and " +
				"avg(garden_projects_status{phase=\"Terminating\"} offset 1w) without (instance)"),
			For:    ptr.To(monitoringv1.Duration("5m")),
			Labels: getLabels("warning"),
			Annotations: map[string]string{
				"summary":     "Project {{$labels.name}} stuck in {{$labels.phase}} phase in landscape {{$externalLabels.landscape}}",
				"description": "Project {{$labels.name}} has been stuck in {{$labels.phase}} phase for 1 week. Please investigate.",
			},
		},
		{
			Alert:  "GardenerControllerManagerDown",
			Expr:   intstr.FromString("absent( up{job = \"gardener-controller-manager\"} == 1 )"),
			For:    ptr.To(monitoringv1.Duration("10m")),
			Labels: utils.MergeStringMaps(getLabels("critical"), map[string]string{"service": "gardener-controller-manager"}),
			Annotations: map[string]string{
				"summary":     "Gardener Controller Manager is Down in landscape {{$externalLabels.landscape}}",
				"description": "Scraping the /metrics endpoint of the Gardener Controller Manager failed for at least 10 minutes.",
			},
		},
		{
			Alert:  "GardenerMetricsExporterDown",
			Expr:   intstr.FromString("absent( up{job=\"gardener-metrics-exporter\"} == 1 )"),
			For:    ptr.To(monitoringv1.Duration("15m")),
			Labels: getLabels("critical"),
			Annotations: map[string]string{
				"summary": "The gardener-metrics-exporter is down in landscape: {{$externalLabels.landscape}}.",
				"description": "The gardener-metrics-exporter is down. Alert conditions for the " +
					"gardenlets, shoots, and seeds cannot be detected. Metering will also not " +
					"work because there are no metrics.",
			},
		},
		{
			Alert:  "MissingGardenMetrics",
			Expr:   intstr.FromString("scrape_samples_scraped{job=\"gardener-metrics-exporter\"} == 0"),
			For:    ptr.To(monitoringv1.Duration("15m")),
			Labels: getLabels("critical"),
			Annotations: map[string]string{
				"summary": "The gardener-metrics-exporter is not exposing metrics in landscape: {{$externalLabels.landscape}}",
				"description": "The gardener-metrics-exporter is not exposing any metrics. Alert " +
					"conditions for the gardenlets, shoots, and seeds cannot be detected. " +
					"Metering will also not work because there are no metrics.",
			},
		},
		{
			Alert:  "PodFrequentlyRestarting",
			Expr:   intstr.FromString("increase(kube_pod_container_status_restarts_total[1h]) > 5"),
			For:    ptr.To(monitoringv1.Duration("10m")),
			Labels: getLabels("warning"),
			Annotations: map[string]string{
				"summary": "Pod is restarting frequently",
				"description": "Pod {{$labels.namespace}}/{{$labels.pod}} in landscape " +
					"{{$externalLabels.landscape}} was restarted more than 5 times within the " +
					"last hour. The pod is running on the garden cluster.",
			},
		},
		{
			Alert:  "GardenConditionStatusNotTrue",
			Expr:   intstr.FromString(`garden_garden_condition{status!="True"} == 1`),
			For:    ptr.To(monitoringv1.Duration("10m")),
			Labels: getLabels("critical"),
			Annotations: map[string]string{
				"summary": "Garden runtime condition {{$labels.condition}} is in state {{$labels.status}}",
				"description": "Garden {{$labels.name}} in landscape " +
					"{{$externalLabels.landscape}} has condition {{$labels.condition}} unequal to True" +
					" for 10 minutes.",
			},
		},
		{
			Alert:  "GardenKubeStateMetricsDown",
			Expr:   intstr.FromString(`absent(up{job="kube-state-metrics"} == 1)`),
			For:    ptr.To(monitoringv1.Duration("10m")),
			Labels: getLabels("critical"),
			Annotations: map[string]string{
				"summary": "Garden runtime kube-state-metrics is down",
				"description": "Garden runtime kube-state-metrics in landscape " +
					"{{$externalLabels.landscape}} has not been scraped for 10 minutes.",
			},
		},
		{
			Alert: "VerticalPodAutoscalerCappedRecommendation",
			Expr: intstr.FromString(`
  count_over_time(
    (
        {__name__=~"kube_customresource_verticalpodautoscaler_status_recommendation_containerrecommendations_uncappedtarget_.+"}
      >
        {__name__=~"kube_customresource_verticalpodautoscaler_status_recommendation_containerrecommendations_target_.+"}
    )[5m:]
  )
==
  5`),
			Labels: getLabels("warning"),
			Annotations: map[string]string{
				"summary": "A VPA recommendation in the garden cluster is capped.",
				"description": "The following VPA in the garden cluster shows a " +
					"{{ if eq .Labels.unit \"core\" -}} CPU {{- else if eq .Labels.unit \"byte\" -}} memory {{- end }} " +
					"uncapped target recommendation larger than the regular target recommendation:\n" +
					"- landscape = {{ $externalLabels.landscape }}\n" +
					"- namespace = {{ $labels.namespace }}\n" +
					"- vpa = {{ $labels.verticalpodautoscaler }}\n" +
					"- container = {{ $labels.container }}",
			},
		},
	}

	if isGardenerDiscoveryServerEnabled {
		rules = append(rules,
			monitoringv1.Rule{
				Alert:  "DiscoveryServerDown",
				Expr:   intstr.FromString("probe_success{job = \"blackbox-discovery-server\", purpose = \"availability\"} == 0"),
				For:    ptr.To(monitoringv1.Duration("5m")),
				Labels: utils.MergeStringMaps(getLabels("critical"), map[string]string{"service": "discovery-service"}),
				Annotations: map[string]string{
					"summary": "Discovery Server is Down in landscape {{$externalLabels.landscape}}",
					"description": "The http health probe to the Gardener Discovery Server failed for at least 5 " +
						"minutes (instance is {{$labels.instance}}). " +
						"(Status code is " +
						"{{printf `probe_http_status_code{job      = \"blackbox-discovery-server\" " +
						",                                purpose  = \"availability\" " +
						",                                instance = \"%s\"}` " +
						"$labels.instance " +
						"| query | first | value}}).",
				},
			},
		)
	}

	return &monitoringv1.PrometheusRule{
		TypeMeta: metav1.TypeMeta{
			APIVersion: monitoringv1.SchemeGroupVersion.String(),
			Kind:       "PrometheusRule",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name: "garden",
		},
		Spec: monitoringv1.PrometheusRuleSpec{
			Groups: []monitoringv1.RuleGroup{
				{
					Name:  "garden",
					Rules: rules,
				},
			},
		},
	}
}

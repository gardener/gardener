// Copyright 2022 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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
	monitoringv1 "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	"github.com/gardener/gardener/pkg/component/observability/monitoring/prometheus/aggregate"
	monitoringutils "github.com/gardener/gardener/pkg/component/observability/monitoring/utils"
)

const (
	monitoringPrometheusJobNameIstiod              = "istiod"
	monitoringPrometheusJobNameIstioIngressGateway = "istio-ingressgateway"

	// istiod metrics
	monitoringMetricGalleyValidationFailed                          = "galley_validation_failed"
	monitoringMetricGalleyValidationPassed                          = "galley_validation_passed"
	monitoringMetricPilotConflictInboundListener                    = "pilot_conflict_inbound_listener"
	monitoringMetricPilotConflictOutboundListenerHttpOverCurrentTcp = "pilot_conflict_outbound_listener_http_over_current_tcp"
	monitoringMetricPilotConflictOutboundListenerTcpOverCurrentHttp = "pilot_conflict_outbound_listener_tcp_over_current_http"
	monitoringMetricPilotConflictOutboundListenerTcpOverCurrentTcp  = "pilot_conflict_outbound_listener_tcp_over_current_tcp"
	monitoringMetricPilotK8sCfgEvents                               = "pilot_k8s_cfg_events"
	monitoringMetricPilotProxy_convergenceTimeBucket                = "pilot_proxy_convergence_time_bucket"
	monitoringMetricPilotServices                                   = "pilot_services"
	monitoringMetricPilotTotalXdsInternalErrors                     = "pilot_total_xds_internal_errors"
	monitoringMetricPilotTotalXdsRejects                            = "pilot_total_xds_rejects"
	monitoringMetricPilotVirtServices                               = "pilot_virt_services"
	monitoringMetricPilotXds                                        = "pilot_xds"
	monitoringMetricPilotXdsCdsReject                               = "pilot_xds_cds_reject"
	monitoringMetricPilotXdsEdsReject                               = "pilot_xds_eds_reject"
	monitoringMetricPilotXdsLdsReject                               = "pilot_xds_lds_reject"
	monitoringMetricPilotXdsPushContextErrors                       = "pilot_xds_push_context_errors"
	monitoringMetricPilotXdsPushes                                  = "pilot_xds_pushes"
	monitoringMetricPilotXdsRdsReject                               = "pilot_xds_rds_reject"
	monitoringMetricPilotXdsWriteTimeout                            = "pilot_xds_write_timeout"

	// istio-ingressgateway metrics
	monitoringMetricEnvoyClusterUpstreamCxActive           = "envoy_cluster_upstream_cx_active"
	monitoringMetricEnvoyClusterUpstreamCxConnectFail      = "envoy_cluster_upstream_cx_connect_fail"
	monitoringMetricEnvoyClusterUpstreamCxTotal            = "envoy_cluster_upstream_cx_total"
	monitoringMetricEnvoyClusterUpstreamCxTxBytesTotal     = "envoy_cluster_upstream_cx_tx_bytes_total"
	monitoringMetricEnvoyServerHotRestartEpoch             = "envoy_server_hot_restart_epoch"
	monitoringMetricIstioRequestBytesBucket                = "istio_request_bytes_bucket"
	monitoringMetricIstioRequestBytesSum                   = "istio_request_bytes_sum"
	monitoringMetricIstioRequestDurationMillisecondsBucket = "istio_request_duration_milliseconds_bucket"
	monitoringMetricIstioRequestDurationSecondsBucket      = "istio_request_duration_seconds_bucket"
	monitoringMetricIstioRequestsTotal                     = "istio_requests_total"
	monitoringMetricIstioResponseBytesBucket               = "istio_response_bytes_bucket"
	monitoringMetricIstioResponseBytesSum                  = "istio_response_bytes_sum"
	monitoringMetricIstioTcpConnectionsClosedTotal         = "istio_tcp_connections_closed_total"
	monitoringMetricIstioTcpConnectionsOpenedTotal         = "istio_tcp_connections_opened_total"
	monitoringMetricIstioTcpReceivedBytesTotal             = "istio_tcp_received_bytes_total"
	monitoringMetricIstioTcpSentBytesTotal                 = "istio_tcp_sent_bytes_total"

	// shared metrics
	monitoringMetricGoGoroutines               = "go_goroutines"
	monitoringMetricGoMemstatsAllocBytes       = "go_memstats_alloc_bytes"
	monitoringMetricGoMemstatsHeapAllocBytes   = "go_memstats_heap_alloc_bytes"
	monitoringMetricGoMemstatsHeapInuseBytes   = "go_memstats_heap_inuse_bytes"
	monitoringMetricGoMemstatsHeapSysBytes     = "go_memstats_heap_sys_bytes"
	monitoringMetricGoMemstatsStackInuseBytes  = "go_memstats_stack_inuse_bytes"
	monitoringMetricIstioBuild                 = "istio_build"
	monitoringMetricProcessCpuSecondsTotal     = "process_cpu_seconds_total"
	monitoringMetricProcessOpenFds             = "process_open_fds"
	monitoringMetricProcessResidentMemoryBytes = "process_resident_memory_bytes"
	monitoringMetricProcessVirtualMemoryBytes  = "process_virtual_memory_bytes"
)

var (
	serviceMonitorIstiod = &monitoringv1.ServiceMonitor{
		ObjectMeta: monitoringutils.ConfigObjectMeta(monitoringPrometheusJobNameIstiod, v1beta1constants.IstioSystemNamespace, aggregate.Label),
		Spec: monitoringv1.ServiceMonitorSpec{
			Selector: metav1.LabelSelector{MatchLabels: getIstiodLabels()},
			Endpoints: []monitoringv1.Endpoint{{
				Port: istiodServicePortNameMetrics,
				MetricRelabelConfigs: monitoringutils.StandardMetricRelabelConfig(
					monitoringMetricGalleyValidationFailed,
					monitoringMetricGalleyValidationPassed,
					monitoringMetricGoGoroutines,
					monitoringMetricGoMemstatsAllocBytes,
					monitoringMetricGoMemstatsHeapAllocBytes,
					monitoringMetricGoMemstatsHeapInuseBytes,
					monitoringMetricGoMemstatsHeapSysBytes,
					monitoringMetricGoMemstatsStackInuseBytes,
					monitoringMetricIstioBuild,
					monitoringMetricPilotConflictInboundListener,
					monitoringMetricPilotConflictOutboundListenerHttpOverCurrentTcp,
					monitoringMetricPilotConflictOutboundListenerTcpOverCurrentHttp,
					monitoringMetricPilotConflictOutboundListenerTcpOverCurrentTcp,
					monitoringMetricPilotK8sCfgEvents,
					monitoringMetricPilotProxy_convergenceTimeBucket,
					monitoringMetricPilotServices,
					monitoringMetricPilotTotalXdsInternalErrors,
					monitoringMetricPilotTotalXdsRejects,
					monitoringMetricPilotVirtServices,
					monitoringMetricPilotXds,
					monitoringMetricPilotXdsCdsReject,
					monitoringMetricPilotXdsEdsReject,
					monitoringMetricPilotXdsLdsReject,
					monitoringMetricPilotXdsPushContextErrors,
					monitoringMetricPilotXdsPushes,
					monitoringMetricPilotXdsRdsReject,
					monitoringMetricPilotXdsWriteTimeout,
					monitoringMetricProcessCpuSecondsTotal,
					monitoringMetricProcessOpenFds,
					monitoringMetricProcessResidentMemoryBytes,
					monitoringMetricProcessVirtualMemoryBytes,
				),
			}},
		},
	}

	serviceMonitorIstioIngress = &monitoringv1.ServiceMonitor{
		ObjectMeta: monitoringutils.ConfigObjectMeta(monitoringPrometheusJobNameIstioIngressGateway, v1beta1constants.IstioSystemNamespace, aggregate.Label),
		Spec: monitoringv1.ServiceMonitorSpec{
			Selector:          metav1.LabelSelector{MatchLabels: map[string]string{v1beta1constants.LabelApp: "istio-ingressgateway"}},
			NamespaceSelector: monitoringv1.NamespaceSelector{Any: true},
			Endpoints: []monitoringv1.Endpoint{{
				Path: "/stats/prometheus",
				Port: "tls-tunnel",
				RelabelConfigs: []*monitoringv1.RelabelConfig{{
					SourceLabels: []monitoringv1.LabelName{"__meta_kubernetes_pod_ip"},
					Action:       "replace",
					TargetLabel:  "__address__",
					Regex:        `(.+)`,
					Replacement:  "${1}:15020",
				}},
				MetricRelabelConfigs: monitoringutils.StandardMetricRelabelConfig(
					monitoringMetricEnvoyClusterUpstreamCxActive,
					monitoringMetricEnvoyClusterUpstreamCxConnectFail,
					monitoringMetricEnvoyClusterUpstreamCxTotal,
					monitoringMetricEnvoyClusterUpstreamCxTxBytesTotal,
					monitoringMetricEnvoyServerHotRestartEpoch,
					monitoringMetricGoGoroutines,
					monitoringMetricGoMemstatsAllocBytes,
					monitoringMetricGoMemstatsHeapAllocBytes,
					monitoringMetricGoMemstatsHeapInuseBytes,
					monitoringMetricGoMemstatsHeapSysBytes,
					monitoringMetricGoMemstatsStackInuseBytes,
					monitoringMetricIstioBuild,
					monitoringMetricIstioRequestBytesBucket,
					monitoringMetricIstioRequestBytesSum,
					monitoringMetricIstioRequestDurationMillisecondsBucket,
					monitoringMetricIstioRequestDurationSecondsBucket,
					monitoringMetricIstioRequestsTotal,
					monitoringMetricIstioResponseBytesBucket,
					monitoringMetricIstioResponseBytesSum,
					monitoringMetricIstioTcpConnectionsClosedTotal,
					monitoringMetricIstioTcpConnectionsOpenedTotal,
					monitoringMetricIstioTcpReceivedBytesTotal,
					monitoringMetricIstioTcpSentBytesTotal,
					monitoringMetricProcessCpuSecondsTotal,
					monitoringMetricProcessOpenFds,
					monitoringMetricProcessResidentMemoryBytes,
					monitoringMetricProcessVirtualMemoryBytes,
				),
			}},
		},
	}
)

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

package etcd

import (
	"bytes"
	"fmt"
	"strings"
	"text/template"

	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"

	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
)

const (
	monitoringPrometheusJobEtcdNamePrefix          = "kube-etcd3"
	monitoringPrometheusJobBackupRestoreNamePrefix = "kube-etcd3-backup-restore"

	monitoringMetricEtcdDiskBackendCommitDurationSecondsBucket = "etcd_disk_backend_commit_duration_seconds_bucket"
	monitoringMetricEtcdDiskWalFsyncDurationSecondsBucket      = "etcd_disk_wal_fsync_duration_seconds_bucket"
	monitoringMetricEtcdMvccDBTotalSizeInBytes                 = "etcd_mvcc_db_total_size_in_bytes"
	monitoringMetricEtcdNetworkClientGrpcReceivedBytesTotal    = "etcd_network_client_grpc_received_bytes_total"
	monitoringMetricEtcdNetworkClientGrpcSentBytesTotal        = "etcd_network_client_grpc_sent_bytes_total"
	monitoringMetricEtcdNetworkPeerReceivedBytesTotal          = "etcd_network_peer_received_bytes_total"
	monitoringMetricEtcdNetworkPeerSentBytesTotal              = "etcd_network_peer_sent_bytes_total"
	monitoringMetricEtcdServerHasLeader                        = "etcd_server_has_leader"
	monitoringMetricEtcdServerLeaderChangesSeenTotal           = "etcd_server_leader_changes_seen_total"
	monitoringMetricEtcdServerProposalsAppliedTotal            = "etcd_server_proposals_applied_total"
	monitoringMetricEtcdServerProposalsCommittedTotal          = "etcd_server_proposals_committed_total"
	monitoringMetricEtcdServerProposalsFailedTotal             = "etcd_server_proposals_failed_total"
	monitoringMetricEtcdServerProposalsPending                 = "etcd_server_proposals_pending"
	monitoringMetricGrpcServerHandledTotal                     = "grpc_server_handled_total"
	monitoringMetricGrpcServerStartedTotal                     = "grpc_server_started_total"

	monitoringMetricBackupRestoreDefragmentationDurationSecondsBucket = "etcdbr_defragmentation_duration_seconds_bucket"
	monitoringMetricBackupRestoreDefragmentationDurationSecondsCount  = "etcdbr_defragmentation_duration_seconds_count"
	monitoringMetricBackupRestoreDefragmentationDurationSecondsSum    = "etcdbr_defragmentation_duration_seconds_sum"
	monitoringMetricBackupRestoreNetworkReceivedBytes                 = "etcdbr_network_received_bytes"
	monitoringMetricBackupRestoreNetworkTransmittedBytes              = "etcdbr_network_transmitted_bytes"
	monitoringMetricBackupRestoreRestorationDurationSecondsBucket     = "etcdbr_restoration_duration_seconds_bucket"
	monitoringMetricBackupRestoreRestorationDurationSecondsCount      = "etcdbr_restoration_duration_seconds_count"
	monitoringMetricBackupRestoreRestorationDurationSecondsSum        = "etcdbr_restoration_duration_seconds_sum"
	monitoringMetricBackupRestoreSnapshotDurationSecondsBucket        = "etcdbr_snapshot_duration_seconds_bucket"
	monitoringMetricBackupRestoreSnapshotDurationSecondsCount         = "etcdbr_snapshot_duration_seconds_count"
	monitoringMetricBackupRestoreSnapshotDurationSecondsSum           = "etcdbr_snapshot_duration_seconds_sum"
	monitoringMetricBackupRestoreSnapshotGCTotal                      = "etcdbr_snapshot_gc_total"
	monitoringMetricBackupRestoreSnapshotLatestRevision               = "etcdbr_snapshot_latest_revision"
	monitoringMetricBackupRestoreSnapshotLatestTimestamp              = "etcdbr_snapshot_latest_timestamp"
	monitoringMetricBackupRestoreSnapshotRequired                     = "etcdbr_snapshot_required"
	monitoringMetricBackupRestoreValidationDurationSecondsBucket      = "etcdbr_validation_duration_seconds_bucket"
	monitoringMetricBackupRestoreValidationDurationSecondsCount       = "etcdbr_validation_duration_seconds_count"
	monitoringMetricBackupRestoreValidationDurationSecondsSum         = "etcdbr_validation_duration_seconds_sum"
	monitoringMetricBackupRestoreSnapshotterFailure                   = "etcdbr_snapshotter_failure"

	monitoringMetricProcessMaxFds              = "process_max_fds"
	monitoringMetricProcessOpenFds             = "process_open_fds"
	monitoringMetricProcessResidentMemoryBytes = "process_resident_memory_bytes"
	monitoringMetricProcessCPUSecondsTotal     = "process_cpu_seconds_total"

	monitoringAlertingRules = `groups:
- name: ` + monitoringPrometheusJobEtcdNamePrefix + `-{{ .role }}.rules
  rules:
  # alert if etcd is down
  - alert: KubeEtcd{{ .Role }}Down
    expr: sum(up{job="` + monitoringPrometheusJobEtcdNamePrefix + `-{{ .role }}"}) < 1
    for: {{ if eq .class .classImportant }}5m{{ else }}15m{{ end }}
    labels:
      service: etcd
      severity: {{ if eq .class .classImportant }}blocker{{ else }}critical{{ end }}
      type: seed
      visibility: operator
    annotations:
      description: Etcd3 cluster {{ .role }} is unavailable or cannot be scraped. As long as etcd3 {{ .role }} is down the cluster is unreachable.
      summary: Etcd3 {{ .role }} cluster down.
  # etcd leader alerts
  - alert: KubeEtcd3{{ .Role }}NoLeader
    expr: sum(` + monitoringMetricEtcdServerHasLeader + `{job="` + monitoringPrometheusJobEtcdNamePrefix + `-{{ .role }}"}) < count(` + monitoringMetricEtcdServerHasLeader + `{job="` + monitoringPrometheusJobEtcdNamePrefix + `-{{ .role }}"})
    for: {{ if eq .class .classImportant }}10m{{ else }}15m{{ end }}
    labels:
      service: etcd
      severity: critical
      type: seed
      visibility: operator
    annotations:
      description: Etcd3 {{ .role }} has no leader. No communication with etcd {{ .role }} possible. Apiserver is read only.
      summary: Etcd3 {{ .role }} has no leader.

  ### etcd proposal alerts ###
  # alert if there are several failed proposals within an hour
  # Note: Increasing the failedProposals count to 80, known issue in etcd, fix in progress
  # https://github.com/kubernetes/kubernetes/pull/64539 - fix in Kubernetes to be released with v1.15
  # https://github.com/etcd-io/etcd/issues/9360 - ongoing discussion in etcd
  # TODO (shreyas-s-rao): change value from 120 to 5 after upgrading to etcd 3.4
  - alert: KubeEtcd3HighNumberOfFailedProposals
    expr: increase(` + monitoringMetricEtcdServerProposalsFailedTotal + `{job="` + monitoringPrometheusJobEtcdNamePrefix + `-{{ .role }}"}[1h]) > 120
    labels:
      service: etcd
      severity: warning
      type: seed
      visibility: operator
    annotations:
      description: Etcd3 {{ .role }} pod {{"{{ $labels.pod }}"}} has seen {{"{{ $value }}"}} proposal failures
        within the last hour.
      summary: High number of failed etcd proposals

  - alert: KubeEtcd3HighMemoryConsumption
    expr: sum(container_memory_working_set_bytes{pod="etcd-main-0",container="etcd"}) / sum(vpa_spec_container_resource_policy_allowed{allowed="max",container="etcd", targetName="etcd-main", resource="memory"}) > .4
    for: 15m
    labels:
      service: etcd
      severity: warning
      type: seed
      visibility: operator
    annotations:
      description: Etcd is consuming over 50% of the max allowed value specified by VPA.
      summary: Etcd is consuming too much memory

  # etcd DB size alerts
  - alert: KubeEtcd3DbSizeLimitApproaching
    expr: (` + monitoringMetricEtcdMvccDBTotalSizeInBytes + `{job="` + monitoringPrometheusJobEtcdNamePrefix + `-{{ .role }}"} > bool 7516193000) + (` + monitoringMetricEtcdMvccDBTotalSizeInBytes + `{job="` + monitoringPrometheusJobEtcdNamePrefix + `-{{ .role }}"} <= bool 8589935000) == 2 # between 7GB and 8GB
    labels:
      service: etcd
      severity: warning
      type: seed
      visibility: all
    annotations:
      description: Etcd3 {{ .role }} DB size is approaching its current practical limit of 8GB. Etcd quota might need to be increased.
      summary: Etcd3 {{ .role }} DB size is approaching its current practical limit.

  - alert: KubeEtcd3DbSizeLimitCrossed
    expr: ` + monitoringMetricEtcdMvccDBTotalSizeInBytes + `{job="` + monitoringPrometheusJobEtcdNamePrefix + `-{{ .role }}"} > 8589935000 # above 8GB
    labels:
      service: etcd
      severity: critical
      type: seed
      visibility: all
    annotations:
      description: Etcd3 {{ .role }} DB size has crossed its current practical limit of 8GB. Etcd quota must be increased to allow updates.
      summary: Etcd3 {{ .role }} DB size has crossed its current practical limit.

  - record: shoot:etcd_object_counts:sum_by_resource
    expr: max(etcd_object_counts) by (resource)

  {{- if .backupEnabled }}
  # etcd backup failure alerts
  - alert: KubeEtcdDeltaBackupFailed
    expr: (time() - ` + monitoringMetricBackupRestoreSnapshotLatestTimestamp + `{job="` + monitoringPrometheusJobBackupRestoreNamePrefix + `-{{ .role }}",kind="Incr"} > bool 900) + (etcdbr_snapshot_required{job="` + monitoringPrometheusJobBackupRestoreNamePrefix + `-{{ .role }}", kind="Incr"} >= bool 1) == 2
    for: 15m
    labels:
      service: etcd
      severity: critical
      type: seed
      visibility: operator
    annotations:
      description: No delta snapshot for the past at least 30 minutes.
      summary: Etcd delta snapshot failure.
  - alert: KubeEtcdFullBackupFailed
    expr: (time() - ` + monitoringMetricBackupRestoreSnapshotLatestTimestamp + `{job="` + monitoringPrometheusJobBackupRestoreNamePrefix + `-{{ .role }}",kind="Full"} > bool 86400) + (etcdbr_snapshot_required{job="` + monitoringPrometheusJobBackupRestoreNamePrefix + `-{{ .role }}", kind="Full"} >= bool 1) == 2
    for: 15m
    labels:
      service: etcd
      severity: critical
      type: seed
      visibility: operator
    annotations:
      description: No full snapshot taken in the past day.
      summary: Etcd full snapshot failure.

  # etcd data restoration failure alert
  - alert: KubeEtcdRestorationFailed
    expr: rate(` + monitoringMetricBackupRestoreRestorationDurationSecondsCount + `{job="` + monitoringPrometheusJobBackupRestoreNamePrefix + `-{{ .role }}",succeeded="false"}[2m]) > 0
    labels:
      service: etcd
      severity: critical
      type: seed
      visibility: operator
    annotations:
      description: Etcd data restoration was triggered, but has failed.
      summary: Etcd data restoration failure.

  # etcd backup failure alert
  - alert: KubeEtcdBackupRestore{{ .Role }}Down
    expr: (sum(up{job="` + monitoringPrometheusJobEtcdNamePrefix + `-{{ .role }}"}) - sum(up{job="` + monitoringPrometheusJobBackupRestoreNamePrefix + `-{{ .role }}"}) > 0) or (rate(` + monitoringMetricBackupRestoreSnapshotterFailure + `{job="` + monitoringPrometheusJobBackupRestoreNamePrefix + `-{{ .role }}"}[5m]) > 0)
    for: 10m
    labels:
      service: etcd
      severity: critical
      type: seed
      visibility: operator
    annotations:
      description: Etcd backup restore {{ .role }} process down or snapshotter failed with error. Backups will not be triggered unless backup restore is brought back up. This is unsafe behaviour and may cause data loss.
      summary: Etcd backup restore {{ .role }} process down or snapshotter failed with error
  {{- end }}
`
)

var (
	monitoringAllowedMetricsEtcd = []string{
		monitoringMetricEtcdDiskBackendCommitDurationSecondsBucket,
		monitoringMetricEtcdDiskWalFsyncDurationSecondsBucket,
		monitoringMetricEtcdMvccDBTotalSizeInBytes,
		monitoringMetricEtcdNetworkClientGrpcReceivedBytesTotal,
		monitoringMetricEtcdNetworkClientGrpcSentBytesTotal,
		monitoringMetricEtcdNetworkPeerReceivedBytesTotal,
		monitoringMetricEtcdNetworkPeerSentBytesTotal,
		monitoringMetricEtcdServerHasLeader,
		monitoringMetricEtcdServerLeaderChangesSeenTotal,
		monitoringMetricEtcdServerProposalsAppliedTotal,
		monitoringMetricEtcdServerProposalsCommittedTotal,
		monitoringMetricEtcdServerProposalsFailedTotal,
		monitoringMetricEtcdServerProposalsPending,
		monitoringMetricGrpcServerHandledTotal,
		monitoringMetricGrpcServerStartedTotal,
		monitoringMetricProcessMaxFds,
		monitoringMetricProcessOpenFds,
		monitoringMetricProcessResidentMemoryBytes,
	}

	monitoringAllowedMetricsBackupRestore = []string{
		monitoringMetricBackupRestoreDefragmentationDurationSecondsBucket,
		monitoringMetricBackupRestoreDefragmentationDurationSecondsCount,
		monitoringMetricBackupRestoreDefragmentationDurationSecondsSum,
		monitoringMetricBackupRestoreNetworkReceivedBytes,
		monitoringMetricBackupRestoreNetworkTransmittedBytes,
		monitoringMetricBackupRestoreRestorationDurationSecondsBucket,
		monitoringMetricBackupRestoreRestorationDurationSecondsCount,
		monitoringMetricBackupRestoreRestorationDurationSecondsSum,
		monitoringMetricBackupRestoreSnapshotDurationSecondsBucket,
		monitoringMetricBackupRestoreSnapshotDurationSecondsCount,
		monitoringMetricBackupRestoreSnapshotDurationSecondsSum,
		monitoringMetricBackupRestoreSnapshotGCTotal,
		monitoringMetricBackupRestoreSnapshotLatestRevision,
		monitoringMetricBackupRestoreSnapshotLatestTimestamp,
		monitoringMetricBackupRestoreSnapshotRequired,
		monitoringMetricBackupRestoreValidationDurationSecondsBucket,
		monitoringMetricBackupRestoreValidationDurationSecondsCount,
		monitoringMetricBackupRestoreValidationDurationSecondsSum,
		monitoringMetricBackupRestoreSnapshotterFailure,
		monitoringMetricProcessResidentMemoryBytes,
		monitoringMetricProcessCPUSecondsTotal,
	}

	// TODO: Replace below hard-coded paths to Prometheus certificates once its deployment has been refactored.
	monitoringScrapeConfigEtcdTmpl = `job_name: ` + monitoringPrometheusJobEtcdNamePrefix + `-{{ .role }}
scheme: https
tls_config:
  # This is needed because the etcd's certificates are not are generated
  # for a specific pod IP
  insecure_skip_verify: true
  cert_file: /srv/kubernetes/etcd/client/tls.crt
  key_file: /srv/kubernetes/etcd/client/tls.key
kubernetes_sd_configs:
- role: endpoints
  namespaces:
    names: [{{ .namespace }}]
relabel_configs:
- source_labels:
  - __meta_kubernetes_service_label_` + v1beta1constants.LabelApp + `
  - __meta_kubernetes_service_label_` + v1beta1constants.LabelRole + `
  - __meta_kubernetes_endpoint_port_name
  action: keep
  regex: ` + LabelAppValue + `;{{ .role }};` + portNameClient + `
- source_labels: [ __meta_kubernetes_service_label_` + v1beta1constants.LabelRole + ` ]
  target_label: role
- source_labels: [ __meta_kubernetes_pod_name ]
  target_label: pod
metric_relabel_configs:
- regex: ^instance$
  action: labeldrop
- source_labels: [ __name__ ]
  action: keep
  regex: ^(` + strings.Join(monitoringAllowedMetricsEtcd, "|") + `)$`

	// TODO: Replace below hard-coded paths to Prometheus certificates once its deployment has been refactored.
	monitoringScrapeConfigBackupRestoreTmpl = `job_name: ` + monitoringPrometheusJobBackupRestoreNamePrefix + `-{{ .role }}
scheme: https
tls_config:
  # Etcd backup sidecar TLS reuses etcd's TLS cert bundle
  insecure_skip_verify: true
  cert_file: /srv/kubernetes/etcd/client/tls.crt
  key_file: /srv/kubernetes/etcd/client/tls.key
kubernetes_sd_configs:
- role: endpoints
  namespaces:
    names: [{{ .namespace }}]
relabel_configs:
- source_labels:
  - __meta_kubernetes_service_label_` + v1beta1constants.LabelApp + `
  - __meta_kubernetes_service_label_` + v1beta1constants.LabelRole + `
  - __meta_kubernetes_endpoint_port_name
  action: keep
  regex: ` + LabelAppValue + `;{{ .role }};` + portNameBackupRestore + `
- source_labels: [ __meta_kubernetes_service_label_role ]
  target_label: role
- source_labels: [ __meta_kubernetes_pod_name ]
  target_label: pod
metric_relabel_configs:
- regex: ^instance$
  action: labeldrop
- source_labels: [ __name__ ]
  action: keep
  regex: ^(` + strings.Join(monitoringAllowedMetricsBackupRestore, "|") + `)$`
)

var (
	monitoringAlertingRulesTemplate             *template.Template
	monitoringScrapeConfigEtcdTemplate          *template.Template
	monitoringScrapeConfigBackupRestoreTemplate *template.Template
)

func init() {
	var err error

	monitoringAlertingRulesTemplate, err = template.New("monitoring-alerting-rules").Parse(monitoringAlertingRules)
	utilruntime.Must(err)
	monitoringScrapeConfigEtcdTemplate, err = template.New("monitoring-scrape-config-etcd").Parse(monitoringScrapeConfigEtcdTmpl)
	utilruntime.Must(err)
	monitoringScrapeConfigBackupRestoreTemplate, err = template.New("monitoring-scrape-config-backup-restore").Parse(monitoringScrapeConfigBackupRestoreTmpl)
	utilruntime.Must(err)
}

// ScrapeConfigs returns the scrape configurations for Prometheus.
func (e *etcd) ScrapeConfigs() ([]string, error) {
	values := map[string]interface{}{
		"namespace": e.namespace,
		"role":      e.role,
	}

	var scrapeConfigEtcd bytes.Buffer
	if err := monitoringScrapeConfigEtcdTemplate.Execute(&scrapeConfigEtcd, values); err != nil {
		return nil, err
	}

	var scrapeConfigBackupRestore bytes.Buffer
	if err := monitoringScrapeConfigBackupRestoreTemplate.Execute(&scrapeConfigBackupRestore, values); err != nil {
		return nil, err
	}

	return []string{
		scrapeConfigEtcd.String(),
		scrapeConfigBackupRestore.String(),
	}, nil
}

// AlertingRules returns the alerting rules for AlertManager.
func (e *etcd) AlertingRules() (map[string]string, error) {
	var alertingRules bytes.Buffer
	if err := monitoringAlertingRulesTemplate.Execute(&alertingRules, map[string]interface{}{
		"role":           e.role,
		"Role":           strings.Title(e.role),
		"class":          e.class,
		"classImportant": ClassImportant,
		"backupEnabled":  e.backupConfig != nil,
	}); err != nil {
		return nil, err
	}

	return map[string]string{fmt.Sprintf("kube-etcd3-%s.rules.yaml", e.role): alertingRules.String()}, nil
}

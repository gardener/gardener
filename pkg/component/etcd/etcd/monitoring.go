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

package etcd

import (
	"bytes"
	"fmt"
	"strings"
	"text/template"

	"golang.org/x/text/cases"
	"golang.org/x/text/language"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"

	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
)

const (
	monitoringPrometheusJobEtcdNamePrefix          = "kube-etcd3"
	monitoringPrometheusJobBackupRestoreNamePrefix = "kube-etcd3-backup-restore"
	monitoringPrometheusJobDruidName               = "etcd-druid"

	monitoringMetricEtcdDiskBackendCommitDurationSecondsBucket = "etcd_disk_backend_commit_duration_seconds_bucket"
	monitoringMetricEtcdDiskWalFsyncDurationSecondsBucket      = "etcd_disk_wal_fsync_duration_seconds_bucket"
	monitoringMetricEtcdMvccDBTotalSizeInBytes                 = "etcd_mvcc_db_total_size_in_bytes"
	monitoringMetricEtcdMvccDBTotalSizeInUseInBytes            = "etcd_mvcc_db_total_size_in_use_in_bytes"
	monitoringMetricEtcdNetworkClientGrpcReceivedBytesTotal    = "etcd_network_client_grpc_received_bytes_total"
	monitoringMetricEtcdNetworkClientGrpcSentBytesTotal        = "etcd_network_client_grpc_sent_bytes_total"
	monitoringMetricEtcdNetworkPeerReceivedBytesTotal          = "etcd_network_peer_received_bytes_total"
	monitoringMetricEtcdNetworkPeerSentBytesTotal              = "etcd_network_peer_sent_bytes_total"
	monitoringMetricEtcdNetworkActivePeers                     = "etcd_network_active_peers"
	monitoringMetricEtcdNetworkPeerRoundTripTimeSecondsBucket  = "etcd_network_peer_round_trip_time_seconds_bucket"
	monitoringMetricEtcdServerHasLeader                        = "etcd_server_has_leader"
	monitoringMetricEtcdServerIsLeader                         = "etcd_server_is_leader"
	monitoringMetricEtcdServerLeaderChangesSeenTotal           = "etcd_server_leader_changes_seen_total"
	monitoringMetricEtcdServerIsLearner                        = "etcd_server_is_learner"
	monitoringMetricEtcdServerLearnerPromoteSuccesses          = "etcd_server_learner_promote_successes"
	monitoringMetricEtcdServerProposalsAppliedTotal            = "etcd_server_proposals_applied_total"
	monitoringMetricEtcdServerProposalsCommittedTotal          = "etcd_server_proposals_committed_total"
	monitoringMetricEtcdServerProposalsFailedTotal             = "etcd_server_proposals_failed_total"
	monitoringMetricEtcdServerProposalsPending                 = "etcd_server_proposals_pending"
	monitoringMetricEtcdServerHeartbeatSendFailuresTotal       = "etcd_server_heartbeat_send_failures_total"
	monitoringMetricEtcdServerSlowReadIndexesTotal             = "etcd_server_slow_read_indexes_total"
	monitoringMetricEtcdServerSlowApplyTotal                   = "etcd_server_slow_apply_total"
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
	monitoringMetricBackupRestoreClusterSize                          = "etcdbr_cluster_size"
	monitoringMetricBackupRestoreIsLearner                            = "etcdbr_is_learner"
	monitoringMetricBackupRestoreIsLearnerCountTotal                  = "etcdbr_is_learner_count_total"
	monitoringMetricBackupRestoreAddLearnerDurationSecondsBucket      = "etcdbr_add_learner_duration_seconds_bucket"
	monitoringMetricBackupRestoreAddLearnerDurationSecondsSum         = "etcdbr_add_learner_duration_seconds_sum"
	monitoringMetricBackupRestoreMemberRemoveDurationSecondsBucket    = "etcdbr_member_remove_duration_seconds_bucket"
	monitoringMetricBackupRestoreMemberRemoveDurationSecondsSum       = "etcdbr_member_remove_duration_seconds_sum"
	monitoringMetricBackupRestoreMemberPromoteDurationSecondsBucket   = "etcdbr_member_promote_duration_seconds_bucket"
	monitoringMetricBackupRestoreMemberPromoteDurationSecondsSum      = "etcdbr_member_promote_duration_seconds_sum"

	monitoringMetricProcessMaxFds              = "process_max_fds"
	monitoringMetricProcessOpenFds             = "process_open_fds"
	monitoringMetricProcessResidentMemoryBytes = "process_resident_memory_bytes"
	monitoringMetricProcessCPUSecondsTotal     = "process_cpu_seconds_total"

	monitoringAlertingRules = `groups:
- name: ` + monitoringPrometheusJobEtcdNamePrefix + `-{{ .role }}.rules
  rules:
  # alert if etcd is down
  - alert: KubeEtcd{{ .Role }}Down
    expr: sum(up{job="` + monitoringPrometheusJobEtcdNamePrefix + `-{{ .role }}"}) < {{ .etcdQuorumReplicas }}
    for: {{ if eq .class .classImportant }}5m{{ else }}15m{{ end }}
    labels:
      service: etcd
      severity: {{ if eq .class .classImportant }}blocker{{ else }}critical{{ end }}
      type: seed
      visibility: operator
    annotations:
      description: Etcd3 cluster {{ .role }} is unavailable{{ if .isHA }} (due to possible quorum loss){{ end }} or cannot be scraped. As long as etcd3 {{ .role }} is down, the cluster is unreachable.
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
      description: Etcd3 {{ .role }} has no leader.{{ if .isHA }} Possible network partition in the etcd cluster.{{ end }}
      summary: Etcd3 {{ .role }} has no leader.

  ### etcd proposal alerts ###
  # alert if there are several failed proposals within an hour
  - alert: KubeEtcd3HighNumberOfFailedProposals
    expr: increase(` + monitoringMetricEtcdServerProposalsFailedTotal + `{job="` + monitoringPrometheusJobEtcdNamePrefix + `-{{ .role }}"}[1h]) > 5
    labels:
      service: etcd
      severity: warning
      type: seed
      visibility: operator
    annotations:
      description: Etcd3 {{ .role }} pod {{"{{ $labels.pod }}"}} has seen {{"{{ $value }}"}} proposal failures within the last hour.
      summary: High number of failed etcd proposals

  - alert: KubeEtcd3HighMemoryConsumption
    expr: sum(container_memory_working_set_bytes{pod="etcd-{{ .role }}-0",container="etcd"}) / sum(kube_verticalpodautoscaler_spec_resourcepolicy_container_policies_maxallowed{container="etcd", targetName="etcd-{{ .role }}", resource="memory"}) > .5
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

  - record: shoot:apiserver_storage_objects:sum_by_resource
    expr: max(apiserver_storage_objects) by (resource)

  {{- if .backupEnabled }}
  # etcd backup failure alerts
  - alert: KubeEtcdDeltaBackupFailed
    expr:
            (
                (
                    time() - ` + monitoringMetricBackupRestoreSnapshotLatestTimestamp + `{job="` + monitoringPrometheusJobBackupRestoreNamePrefix + `-{{ .role }}",kind="Incr"}
                  > bool
                    900
                )
              *
                etcdbr_snapshot_required{job="` + monitoringPrometheusJobBackupRestoreNamePrefix + `-{{ .role }}",kind="Incr"}
            )
          * on (pod, role)
            ` + monitoringMetricEtcdServerIsLeader + `{job="` + monitoringPrometheusJobEtcdNamePrefix + `-{{ .role }}"}
        >
          0
    for: 15m
    labels:
      service: etcd
      severity: critical
      type: seed
      visibility: operator
    annotations:
      description: No delta snapshot for the past 30 minutes have been taken by backup-restore leader.
      summary: Etcd delta snapshot failure.
  - alert: KubeEtcdFullBackupFailed
    expr:
            (
                (
                    time() - ` + monitoringMetricBackupRestoreSnapshotLatestTimestamp + `{job="` + monitoringPrometheusJobBackupRestoreNamePrefix + `-{{ .role }}",kind="Full"}
                  > bool
                    86400
                )
              *
                etcdbr_snapshot_required{job="` + monitoringPrometheusJobBackupRestoreNamePrefix + `-{{ .role }}",kind="Full"}
            )
          * on (pod, role)
            ` + monitoringMetricEtcdServerIsLeader + `{job="` + monitoringPrometheusJobEtcdNamePrefix + `-{{ .role }}"}
        >
          0
    for: 15m
    labels:
      service: etcd
      severity: critical
      type: seed
      visibility: operator
    annotations:
      description: No full snapshot for at least last 24 hours have been taken by backup-restore leader.
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
		monitoringMetricEtcdMvccDBTotalSizeInUseInBytes,
		monitoringMetricEtcdNetworkClientGrpcReceivedBytesTotal,
		monitoringMetricEtcdNetworkClientGrpcSentBytesTotal,
		monitoringMetricEtcdNetworkPeerReceivedBytesTotal,
		monitoringMetricEtcdNetworkPeerSentBytesTotal,
		monitoringMetricEtcdNetworkActivePeers,
		monitoringMetricEtcdNetworkPeerRoundTripTimeSecondsBucket,
		monitoringMetricEtcdServerHasLeader,
		monitoringMetricEtcdServerIsLeader,
		monitoringMetricEtcdServerLeaderChangesSeenTotal,
		monitoringMetricEtcdServerIsLearner,
		monitoringMetricEtcdServerLearnerPromoteSuccesses,
		monitoringMetricEtcdServerProposalsAppliedTotal,
		monitoringMetricEtcdServerProposalsCommittedTotal,
		monitoringMetricEtcdServerProposalsFailedTotal,
		monitoringMetricEtcdServerProposalsPending,
		monitoringMetricEtcdServerHeartbeatSendFailuresTotal,
		monitoringMetricEtcdServerSlowReadIndexesTotal,
		monitoringMetricEtcdServerSlowApplyTotal,
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
		monitoringMetricBackupRestoreClusterSize,
		monitoringMetricBackupRestoreIsLearner,
		monitoringMetricBackupRestoreIsLearnerCountTotal,
		monitoringMetricBackupRestoreAddLearnerDurationSecondsBucket,
		monitoringMetricBackupRestoreAddLearnerDurationSecondsSum,
		monitoringMetricBackupRestoreMemberRemoveDurationSecondsBucket,
		monitoringMetricBackupRestoreMemberRemoveDurationSecondsSum,
		monitoringMetricBackupRestoreMemberPromoteDurationSecondsBucket,
		monitoringMetricBackupRestoreMemberPromoteDurationSecondsSum,
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
  - __meta_kubernetes_pod_label_` + v1beta1constants.LabelApp + `
  - __meta_kubernetes_pod_label_` + v1beta1constants.LabelRole + `
  - __meta_kubernetes_endpoint_port_name
  action: keep
  regex: ` + LabelAppValue + `;{{ .role }};` + portNameClient + `
- source_labels: [ __meta_kubernetes_pod_label_` + v1beta1constants.LabelRole + ` ]
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
  - __meta_kubernetes_pod_label_` + v1beta1constants.LabelApp + `
  - __meta_kubernetes_pod_label_` + v1beta1constants.LabelRole + `
  - __meta_kubernetes_endpoint_port_name
  action: keep
  regex: ` + LabelAppValue + `;{{ .role }};` + portNameBackupRestore + `
- source_labels: [ __meta_kubernetes_pod_label_role ]
  target_label: role
- source_labels: [ __meta_kubernetes_pod_name ]
  target_label: pod
metric_relabel_configs:
- regex: ^instance$
  action: labeldrop
- source_labels: [ __name__ ]
  action: keep
  regex: ^(` + strings.Join(monitoringAllowedMetricsBackupRestore, "|") + `)$`

	monitoringScrapeConfigDruidTmpl = `job_name: ` + monitoringPrometheusJobDruidName + `
honor_timestamps: false
metrics_path: /federate
params:
  'match[]':
  - '{job="` + monitoringPrometheusJobDruidName + `",etcd_namespace="{{ .namespace }}"}'
static_configs:
- targets:
  - prometheus-cache.garden.svc`
)

var (
	monitoringAlertingRulesTemplate             *template.Template
	monitoringScrapeConfigEtcdTemplate          *template.Template
	monitoringScrapeConfigBackupRestoreTemplate *template.Template
	monitoringScrapeConfigDruidTemplate         *template.Template
)

func init() {
	var err error

	monitoringAlertingRulesTemplate, err = template.New("monitoring-alerting-rules").Parse(monitoringAlertingRules)
	utilruntime.Must(err)
	monitoringScrapeConfigEtcdTemplate, err = template.New("monitoring-scrape-config-etcd").Parse(monitoringScrapeConfigEtcdTmpl)
	utilruntime.Must(err)
	monitoringScrapeConfigBackupRestoreTemplate, err = template.New("monitoring-scrape-config-backup-restore").Parse(monitoringScrapeConfigBackupRestoreTmpl)
	utilruntime.Must(err)
	monitoringScrapeConfigDruidTemplate, err = template.New("monitoring-scrape-config-druid").Parse(monitoringScrapeConfigDruidTmpl)
	utilruntime.Must(err)
}

// ScrapeConfigs returns the scrape configurations for Prometheus.
func (e *etcd) ScrapeConfigs() ([]string, error) {
	values := map[string]interface{}{
		"namespace": e.namespace,
		"role":      e.values.Role,
	}

	var scrapeConfigEtcd bytes.Buffer
	if err := monitoringScrapeConfigEtcdTemplate.Execute(&scrapeConfigEtcd, values); err != nil {
		return nil, err
	}

	var scrapeConfigBackupRestore bytes.Buffer
	if err := monitoringScrapeConfigBackupRestoreTemplate.Execute(&scrapeConfigBackupRestore, values); err != nil {
		return nil, err
	}

	cfgs := []string{
		scrapeConfigEtcd.String(),
		scrapeConfigBackupRestore.String(),
	}

	// Add scrape config for druid metrics only if the role 'main' exist
	if e.values.Role == v1beta1constants.ETCDRoleMain {
		var scrapeConfigDruid bytes.Buffer
		if err := monitoringScrapeConfigDruidTemplate.Execute(&scrapeConfigDruid, values); err != nil {
			return nil, err
		}

		cfgs = append(cfgs, scrapeConfigDruid.String())
	}

	return cfgs, nil
}

// AlertingRules returns the alerting rules for AlertManager.
func (e *etcd) AlertingRules() (map[string]string, error) {
	var alertingRules bytes.Buffer

	etcdReplicas := int32(1)
	if e.values.Replicas != nil {
		etcdReplicas = *e.values.Replicas
	}

	if err := monitoringAlertingRulesTemplate.Execute(&alertingRules, map[string]interface{}{
		"role":               e.values.Role,
		"Role":               cases.Title(language.English).String(e.values.Role),
		"class":              e.values.Class,
		"classImportant":     ClassImportant,
		"backupEnabled":      e.values.BackupConfig != nil,
		"etcdQuorumReplicas": int(etcdReplicas/2) + 1,
		"isHA":               etcdReplicas > 1,
	}); err != nil {
		return nil, err
	}

	return map[string]string{fmt.Sprintf("kube-etcd3-%s.rules.yaml", e.values.Role): alertingRules.String()}, nil
}

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

package etcd_test

import (
	"fmt"
	"path/filepath"

	"github.com/go-logr/logr"
	. "github.com/onsi/ginkgo/v2"
	"k8s.io/utils/ptr"

	. "github.com/gardener/gardener/pkg/component/etcd"
	"github.com/gardener/gardener/pkg/component/test"
)

var _ = Describe("Monitoring", func() {
	Describe("#ScrapeConfig", func() {
		It("should successfully test the scrape configuration", func() {
			etcd := New(logr.Discard(), nil, testNamespace, nil, Values{
				Role:     testRole,
				Class:    ClassNormal,
				Replicas: ptr.To(int32(1)),
			})
			test.ScrapeConfigs(etcd, expectedScrapeConfigEtcd, expectedScrapeConfigBackupRestore, expectedScrapeConfigDruid)
		})
	})

	Describe("#AlertingRules", func() {
		Context("for single-node etcd", func() {
			Context("w/o backup", func() {
				It("should successfully test the alerting rules (normal) for single-node etcd", func() {
					etcd := New(logr.Discard(), nil, testNamespace, nil, Values{
						Role:     testRole,
						Class:    ClassNormal,
						Replicas: ptr.To(int32(1)),
					})
					test.AlertingRulesWithPromtool(
						etcd,
						map[string]string{fmt.Sprintf("kube-etcd3-%s.rules.yaml", testRole): expectedAlertingRulesNormalSingleNodeWithoutBackup},
						filepath.Join("testdata", "monitoring_alertingrules_normal_singlenode_without_backup.yaml"),
					)
				})

				It("should successfully test the alerting rules (important) for single-node etcd", func() {
					etcd := New(logr.Discard(), nil, testNamespace, nil, Values{
						Role:     testRole,
						Class:    ClassImportant,
						Replicas: ptr.To(int32(1)),
					})
					test.AlertingRulesWithPromtool(
						etcd,
						map[string]string{fmt.Sprintf("kube-etcd3-%s.rules.yaml", testRole): expectedAlertingRulesImportantSingleNodeWithoutBackup},
						filepath.Join("testdata", "monitoring_alertingrules_important_singlenode_without_backup.yaml"),
					)
				})
			})

			Context("w/ backup", func() {
				It("should successfully test the alerting rules (normal) for single-node etcd", func() {
					etcd := New(logr.Discard(), nil, testNamespace, nil, Values{
						Role:     testRole,
						Class:    ClassNormal,
						Replicas: ptr.To(int32(1)),
					})
					etcd.SetBackupConfig(&BackupConfig{})
					test.AlertingRulesWithPromtool(
						etcd,
						map[string]string{fmt.Sprintf("kube-etcd3-%s.rules.yaml", testRole): expectedAlertingRulesNormalSingleNodeWithBackup},
						filepath.Join("testdata", "monitoring_alertingrules_normal_singlenode_with_backup.yaml"),
					)
				})

				It("should successfully test the alerting rules (important) for single-node etcd", func() {
					etcd := New(logr.Discard(), nil, testNamespace, nil, Values{
						Role:     testRole,
						Class:    ClassImportant,
						Replicas: ptr.To(int32(1)),
					})
					etcd.SetBackupConfig(&BackupConfig{})
					test.AlertingRulesWithPromtool(
						etcd,
						map[string]string{fmt.Sprintf("kube-etcd3-%s.rules.yaml", testRole): expectedAlertingRulesImportantSingleNodeWithBackup},
						filepath.Join("testdata", "monitoring_alertingrules_important_singlenode_with_backup.yaml"),
					)
				})
			})
		})
		Context("for multinode etcd", func() {
			Context("w/o backup", func() {
				It("should successfully test the alerting rules (normal) for multinode etcd", func() {
					etcd := New(logr.Discard(), nil, testNamespace, nil, Values{
						Role:     testRole,
						Class:    ClassNormal,
						Replicas: ptr.To(int32(3)),
					})
					test.AlertingRulesWithPromtool(
						etcd,
						map[string]string{fmt.Sprintf("kube-etcd3-%s.rules.yaml", testRole): expectedAlertingRulesNormalMultiNodeWithoutBackup},
						filepath.Join("testdata", "monitoring_alertingrules_normal_multinode_without_backup.yaml"),
					)
				})

				It("should successfully test the alerting rules (important) for multinode etcd", func() {
					etcd := New(logr.Discard(), nil, testNamespace, nil, Values{
						Role:     testRole,
						Class:    ClassImportant,
						Replicas: ptr.To(int32(3)),
					})
					test.AlertingRulesWithPromtool(
						etcd,
						map[string]string{fmt.Sprintf("kube-etcd3-%s.rules.yaml", testRole): expectedAlertingRulesImportantMultiNodeWithoutBackup},
						filepath.Join("testdata", "monitoring_alertingrules_important_multinode_without_backup.yaml"),
					)
				})
			})

			Context("w/ backup", func() {
				It("should successfully test the alerting rules (normal) for multinode etcd", func() {
					etcd := New(logr.Discard(), nil, testNamespace, nil, Values{
						Role:     testRole,
						Class:    ClassNormal,
						Replicas: ptr.To(int32(3)),
					})
					etcd.SetBackupConfig(&BackupConfig{})
					test.AlertingRulesWithPromtool(
						etcd,
						map[string]string{fmt.Sprintf("kube-etcd3-%s.rules.yaml", testRole): expectedAlertingRulesNormalMultiNodeWithBackup},
						filepath.Join("testdata", "monitoring_alertingrules_normal_multinode_with_backup.yaml"),
					)
				})

				It("should successfully test the alerting rules (important) for multinode etcd", func() {
					etcd := New(logr.Discard(), nil, testNamespace, nil, Values{
						Role:     testRole,
						Class:    ClassImportant,
						Replicas: ptr.To(int32(3)),
					})
					etcd.SetBackupConfig(&BackupConfig{})
					test.AlertingRulesWithPromtool(
						etcd,
						map[string]string{fmt.Sprintf("kube-etcd3-%s.rules.yaml", testRole): expectedAlertingRulesImportantMultiNodeWithBackup},
						filepath.Join("testdata", "monitoring_alertingrules_important_multinode_with_backup.yaml"),
					)
				})
			})
		})
	})
})

const (
	expectedScrapeConfigEtcd = `job_name: kube-etcd3-` + testRole + `
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
    names: [` + testNamespace + `]
relabel_configs:
- source_labels:
  - __meta_kubernetes_pod_label_app
  - __meta_kubernetes_pod_label_role
  - __meta_kubernetes_endpoint_port_name
  action: keep
  regex: etcd-statefulset;` + testRole + `;client
- source_labels: [ __meta_kubernetes_pod_label_role ]
  target_label: role
- source_labels: [ __meta_kubernetes_pod_name ]
  target_label: pod
metric_relabel_configs:
- regex: ^instance$
  action: labeldrop
- source_labels: [ __name__ ]
  action: keep
  regex: ^(etcd_disk_backend_commit_duration_seconds_bucket|etcd_disk_wal_fsync_duration_seconds_bucket|etcd_mvcc_db_total_size_in_bytes|etcd_mvcc_db_total_size_in_use_in_bytes|etcd_network_client_grpc_received_bytes_total|etcd_network_client_grpc_sent_bytes_total|etcd_network_peer_received_bytes_total|etcd_network_peer_sent_bytes_total|etcd_network_active_peers|etcd_network_peer_round_trip_time_seconds_bucket|etcd_server_has_leader|etcd_server_is_leader|etcd_server_leader_changes_seen_total|etcd_server_is_learner|etcd_server_learner_promote_successes|etcd_server_proposals_applied_total|etcd_server_proposals_committed_total|etcd_server_proposals_failed_total|etcd_server_proposals_pending|etcd_server_heartbeat_send_failures_total|etcd_server_slow_read_indexes_total|etcd_server_slow_apply_total|grpc_server_handled_total|grpc_server_started_total|process_max_fds|process_open_fds|process_resident_memory_bytes)$`

	expectedScrapeConfigBackupRestore = `job_name: kube-etcd3-backup-restore-` + testRole + `
scheme: https
tls_config:
  # Etcd backup sidecar TLS reuses etcd's TLS cert bundle
  insecure_skip_verify: true
  cert_file: /srv/kubernetes/etcd/client/tls.crt
  key_file: /srv/kubernetes/etcd/client/tls.key
kubernetes_sd_configs:
- role: endpoints
  namespaces:
    names: [` + testNamespace + `]
relabel_configs:
- source_labels:
  - __meta_kubernetes_pod_label_app
  - __meta_kubernetes_pod_label_role
  - __meta_kubernetes_endpoint_port_name
  action: keep
  regex: etcd-statefulset;` + testRole + `;backuprestore
- source_labels: [ __meta_kubernetes_pod_label_role ]
  target_label: role
- source_labels: [ __meta_kubernetes_pod_name ]
  target_label: pod
metric_relabel_configs:
- regex: ^instance$
  action: labeldrop
- source_labels: [ __name__ ]
  action: keep
  regex: ^(etcdbr_defragmentation_duration_seconds_bucket|etcdbr_defragmentation_duration_seconds_count|etcdbr_defragmentation_duration_seconds_sum|etcdbr_network_received_bytes|etcdbr_network_transmitted_bytes|etcdbr_restoration_duration_seconds_bucket|etcdbr_restoration_duration_seconds_count|etcdbr_restoration_duration_seconds_sum|etcdbr_snapshot_duration_seconds_bucket|etcdbr_snapshot_duration_seconds_count|etcdbr_snapshot_duration_seconds_sum|etcdbr_snapshot_gc_total|etcdbr_snapshot_latest_revision|etcdbr_snapshot_latest_timestamp|etcdbr_snapshot_required|etcdbr_validation_duration_seconds_bucket|etcdbr_validation_duration_seconds_count|etcdbr_validation_duration_seconds_sum|etcdbr_snapshotter_failure|etcdbr_cluster_size|etcdbr_is_learner|etcdbr_is_learner_count_total|etcdbr_add_learner_duration_seconds_bucket|etcdbr_add_learner_duration_seconds_sum|etcdbr_member_remove_duration_seconds_bucket|etcdbr_member_remove_duration_seconds_sum|etcdbr_member_promote_duration_seconds_bucket|etcdbr_member_promote_duration_seconds_sum|process_resident_memory_bytes|process_cpu_seconds_total)$`

	expectedScrapeConfigDruid = `job_name: etcd-druid` + `
honor_timestamps: false
metrics_path: /federate
params:
  'match[]':
  - '{job="etcd-druid",etcd_namespace="` + testNamespace + `"}'
static_configs:
- targets:
  - prometheus-cache.garden.svc`

	alertingRulesNormalSingleNode = `groups:
- name: kube-etcd3-` + testRole + `.rules
  rules:
  # alert if etcd is down
  - alert: KubeEtcd` + testROLE + `Down
    expr: sum(up{job="kube-etcd3-` + testRole + `"}) < 1
    for: 15m
    labels:
      service: etcd
      severity: critical
      type: seed
      visibility: operator
    annotations:
      description: Etcd3 cluster ` + testRole + ` is unavailable or cannot be scraped. As long as etcd3 ` + testRole + ` is down, the cluster is unreachable.
      summary: Etcd3 ` + testRole + ` cluster down.
  # etcd leader alerts
  - alert: KubeEtcd3` + testROLE + `NoLeader
    expr: sum(etcd_server_has_leader{job="kube-etcd3-` + testRole + `"}) < count(etcd_server_has_leader{job="kube-etcd3-` + testRole + `"})
    for: 15m
    labels:
      service: etcd
      severity: critical
      type: seed
      visibility: operator
    annotations:
      description: Etcd3 ` + testRole + ` has no leader.
      summary: Etcd3 ` + testRole + ` has no leader.

`

	alertingRulesNormalMultiNode = `groups:
- name: kube-etcd3-` + testRole + `.rules
  rules:
  # alert if etcd is down
  - alert: KubeEtcd` + testROLE + `Down
    expr: sum(up{job="kube-etcd3-` + testRole + `"}) < 2
    for: 15m
    labels:
      service: etcd
      severity: critical
      type: seed
      visibility: operator
    annotations:
      description: Etcd3 cluster ` + testRole + ` is unavailable (due to possible quorum loss) or cannot be scraped. As long as etcd3 ` + testRole + ` is down, the cluster is unreachable.
      summary: Etcd3 ` + testRole + ` cluster down.
  # etcd leader alerts
  - alert: KubeEtcd3` + testROLE + `NoLeader
    expr: sum(etcd_server_has_leader{job="kube-etcd3-` + testRole + `"}) < count(etcd_server_has_leader{job="kube-etcd3-` + testRole + `"})
    for: 15m
    labels:
      service: etcd
      severity: critical
      type: seed
      visibility: operator
    annotations:
      description: Etcd3 ` + testRole + ` has no leader. Possible network partition in the etcd cluster.
      summary: Etcd3 ` + testRole + ` has no leader.

`

	alertingRulesImportantSingleNode = `groups:
- name: kube-etcd3-` + testRole + `.rules
  rules:
  # alert if etcd is down
  - alert: KubeEtcd` + testROLE + `Down
    expr: sum(up{job="kube-etcd3-` + testRole + `"}) < 1
    for: 5m
    labels:
      service: etcd
      severity: blocker
      type: seed
      visibility: operator
    annotations:
      description: Etcd3 cluster ` + testRole + ` is unavailable or cannot be scraped. As long as etcd3 ` + testRole + ` is down, the cluster is unreachable.
      summary: Etcd3 ` + testRole + ` cluster down.
  # etcd leader alerts
  - alert: KubeEtcd3` + testROLE + `NoLeader
    expr: sum(etcd_server_has_leader{job="kube-etcd3-` + testRole + `"}) < count(etcd_server_has_leader{job="kube-etcd3-` + testRole + `"})
    for: 10m
    labels:
      service: etcd
      severity: critical
      type: seed
      visibility: operator
    annotations:
      description: Etcd3 ` + testRole + ` has no leader.
      summary: Etcd3 ` + testRole + ` has no leader.

`

	alertingRulesImportantMultiNode = `groups:
- name: kube-etcd3-` + testRole + `.rules
  rules:
  # alert if etcd is down
  - alert: KubeEtcd` + testROLE + `Down
    expr: sum(up{job="kube-etcd3-` + testRole + `"}) < 2
    for: 5m
    labels:
      service: etcd
      severity: blocker
      type: seed
      visibility: operator
    annotations:
      description: Etcd3 cluster ` + testRole + ` is unavailable (due to possible quorum loss) or cannot be scraped. As long as etcd3 ` + testRole + ` is down, the cluster is unreachable.
      summary: Etcd3 ` + testRole + ` cluster down.
  # etcd leader alerts
  - alert: KubeEtcd3` + testROLE + `NoLeader
    expr: sum(etcd_server_has_leader{job="kube-etcd3-` + testRole + `"}) < count(etcd_server_has_leader{job="kube-etcd3-` + testRole + `"})
    for: 10m
    labels:
      service: etcd
      severity: critical
      type: seed
      visibility: operator
    annotations:
      description: Etcd3 ` + testRole + ` has no leader. Possible network partition in the etcd cluster.
      summary: Etcd3 ` + testRole + ` has no leader.

`

	alertingRulesDefault = `  ### etcd proposal alerts ###
  # alert if there are several failed proposals within an hour
  - alert: KubeEtcd3HighNumberOfFailedProposals
    expr: increase(etcd_server_proposals_failed_total{job="kube-etcd3-` + testRole + `"}[1h]) > 5
    labels:
      service: etcd
      severity: warning
      type: seed
      visibility: operator
    annotations:
      description: Etcd3 ` + testRole + ` pod {{ $labels.pod }} has seen {{ $value }} proposal failures within the last hour.
      summary: High number of failed etcd proposals

  - alert: KubeEtcd3HighMemoryConsumption
    expr: sum(container_memory_working_set_bytes{pod="etcd-` + testRole + `-0",container="etcd"}) / sum(kube_verticalpodautoscaler_spec_resourcepolicy_container_policies_maxallowed{container="etcd", targetName="etcd-` + testRole + `", resource="memory"}) > .5
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
    expr: (etcd_mvcc_db_total_size_in_bytes{job="kube-etcd3-` + testRole + `"} > bool 7516193000) + (etcd_mvcc_db_total_size_in_bytes{job="kube-etcd3-` + testRole + `"} <= bool 8589935000) == 2 # between 7GB and 8GB
    labels:
      service: etcd
      severity: warning
      type: seed
      visibility: all
    annotations:
      description: Etcd3 ` + testRole + ` DB size is approaching its current practical limit of 8GB. Etcd quota might need to be increased.
      summary: Etcd3 ` + testRole + ` DB size is approaching its current practical limit.

  - alert: KubeEtcd3DbSizeLimitCrossed
    expr: etcd_mvcc_db_total_size_in_bytes{job="kube-etcd3-` + testRole + `"} > 8589935000 # above 8GB
    labels:
      service: etcd
      severity: critical
      type: seed
      visibility: all
    annotations:
      description: Etcd3 ` + testRole + ` DB size has crossed its current practical limit of 8GB. Etcd quota must be increased to allow updates.
      summary: Etcd3 ` + testRole + ` DB size has crossed its current practical limit.

  - record: shoot:apiserver_storage_objects:sum_by_resource
    expr: max(apiserver_storage_objects) by (resource)
`

	alertingRulesBackup = `  # etcd backup failure alerts
  - alert: KubeEtcdDeltaBackupFailed
    expr:
            (
                (
                    time() - etcdbr_snapshot_latest_timestamp{job="kube-etcd3-backup-restore-` + testRole + `",kind="Incr"}
                  > bool
                    900
                )
              *
                etcdbr_snapshot_required{job="kube-etcd3-backup-restore-` + testRole + `",kind="Incr"}
            )
          * on (pod, role)
            etcd_server_is_leader{job="kube-etcd3-` + testRole + `"}
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
                    time() - etcdbr_snapshot_latest_timestamp{job="kube-etcd3-backup-restore-` + testRole + `",kind="Full"}
                  > bool
                    86400
                )
              *
                etcdbr_snapshot_required{job="kube-etcd3-backup-restore-` + testRole + `",kind="Full"}
            )
          * on (pod, role)
            etcd_server_is_leader{job="kube-etcd3-` + testRole + `"}
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
    expr: rate(etcdbr_restoration_duration_seconds_count{job="kube-etcd3-backup-restore-` + testRole + `",succeeded="false"}[2m]) > 0
    labels:
      service: etcd
      severity: critical
      type: seed
      visibility: operator
    annotations:
      description: Etcd data restoration was triggered, but has failed.
      summary: Etcd data restoration failure.

  # etcd backup failure alert
  - alert: KubeEtcdBackupRestore` + testROLE + `Down
    expr: (sum(up{job="kube-etcd3-` + testRole + `"}) - sum(up{job="kube-etcd3-backup-restore-` + testRole + `"}) > 0) or (rate(etcdbr_snapshotter_failure{job="kube-etcd3-backup-restore-` + testRole + `"}[5m]) > 0)
    for: 10m
    labels:
      service: etcd
      severity: critical
      type: seed
      visibility: operator
    annotations:
      description: Etcd backup restore ` + testRole + ` process down or snapshotter failed with error. Backups will not be triggered unless backup restore is brought back up. This is unsafe behaviour and may cause data loss.
      summary: Etcd backup restore ` + testRole + ` process down or snapshotter failed with error
`

	expectedAlertingRulesNormalSingleNodeWithoutBackup    = alertingRulesNormalSingleNode + alertingRulesDefault
	expectedAlertingRulesImportantSingleNodeWithoutBackup = alertingRulesImportantSingleNode + alertingRulesDefault
	expectedAlertingRulesNormalMultiNodeWithoutBackup     = alertingRulesNormalMultiNode + alertingRulesDefault
	expectedAlertingRulesImportantMultiNodeWithoutBackup  = alertingRulesImportantMultiNode + alertingRulesDefault
	expectedAlertingRulesNormalSingleNodeWithBackup       = alertingRulesNormalSingleNode + alertingRulesDefault + alertingRulesBackup
	expectedAlertingRulesImportantSingleNodeWithBackup    = alertingRulesImportantSingleNode + alertingRulesDefault + alertingRulesBackup
	expectedAlertingRulesNormalMultiNodeWithBackup        = alertingRulesNormalMultiNode + alertingRulesDefault + alertingRulesBackup
	expectedAlertingRulesImportantMultiNodeWithBackup     = alertingRulesImportantMultiNode + alertingRulesDefault + alertingRulesBackup
)

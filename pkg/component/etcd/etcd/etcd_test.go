// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package etcd_test

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/Masterminds/semver/v3"
	druidv1alpha1 "github.com/gardener/etcd-druid/api/core/v1alpha1"
	"github.com/go-logr/logr"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	monitoringv1 "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1"
	monitoringv1alpha1 "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1alpha1"
	"go.uber.org/mock/gomock"
	appsv1 "k8s.io/api/apps/v1"
	autoscalingv1 "k8s.io/api/autoscaling/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/intstr"
	vpaautoscalingv1 "k8s.io/autoscaler/vertical-pod-autoscaler/pkg/apis/autoscaling.k8s.io/v1"
	kubernetesscheme "k8s.io/client-go/kubernetes/scheme"
	testclock "k8s.io/utils/clock/testing"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"
	fakeclient "sigs.k8s.io/controller-runtime/pkg/client/fake"
	logf "sigs.k8s.io/controller-runtime/pkg/log"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	. "github.com/gardener/gardener/pkg/component/etcd/etcd"
	"github.com/gardener/gardener/pkg/component/etcd/etcd/constants"
	componenttest "github.com/gardener/gardener/pkg/component/test"
	"github.com/gardener/gardener/pkg/utils"
	"github.com/gardener/gardener/pkg/utils/gardener"
	secretsutils "github.com/gardener/gardener/pkg/utils/secrets"
	secretsmanager "github.com/gardener/gardener/pkg/utils/secrets/manager"
	fakesecretsmanager "github.com/gardener/gardener/pkg/utils/secrets/manager/fake"
	"github.com/gardener/gardener/pkg/utils/test"
	. "github.com/gardener/gardener/pkg/utils/test/matchers"
	versionutils "github.com/gardener/gardener/pkg/utils/version"
	"github.com/gardener/gardener/third_party/mock/client-go/rest"
	mockclient "github.com/gardener/gardener/third_party/mock/controller-runtime/client"
)

var _ = Describe("Etcd", func() {
	Describe("#ServiceName", func() {
		It("should return the expected service name", func() {
			Expect(constants.ServiceName(testRole)).To(Equal("etcd-" + testRole + "-client"))
		})
	})

	var (
		ctrl       *gomock.Controller
		c          *mockclient.MockClient
		fakeClient client.Client
		sm         secretsmanager.Interface
		etcd       Interface
		log        logr.Logger

		ctx                     = context.Background()
		fakeErr                 = errors.New("fake err")
		class                   = ClassNormal
		replicas                = ptr.To[int32](1)
		storageCapacity         = "12Gi"
		storageCapacityQuantity = resource.MustParse(storageCapacity)
		storageClassName        = "my-storage-class"
		defragmentationSchedule = "abcd"
		priorityClassName       = "some-priority-class"

		secretNameCA         = "ca-etcd"
		secretNamePeerCA     = "ca-etcd-peer"
		secretNameServer     = "etcd-server-" + testRole
		secretNameServerPeer = "etcd-peer-server-" + testRole
		secretNameClient     = "etcd-client"

		maintenanceTimeWindow = gardencorev1beta1.MaintenanceTimeWindow{
			Begin: "1234",
			End:   "5678",
		}
		highAvailabilityEnabled bool
		caRotationPhase         gardencorev1beta1.CredentialsRotationPhase
		autoscalingConfig       AutoscalingConfig
		backupConfig            *BackupConfig
		now                     = time.Time{}
		quota                   = resource.MustParse("8Gi")
		garbageCollectionPolicy = druidv1alpha1.GarbageCollectionPolicy(druidv1alpha1.GarbageCollectionPolicyExponential)
		garbageCollectionPeriod = metav1.Duration{Duration: 12 * time.Hour}
		compressionPolicy       = druidv1alpha1.GzipCompression
		compressionSpec         = druidv1alpha1.CompressionSpec{
			Enabled: ptr.To(true),
			Policy:  &compressionPolicy,
		}
		backupLeaderElectionEtcdConnectionTimeout = &metav1.Duration{Duration: 10 * time.Second}
		backupLeaderElectionReelectionPeriod      = &metav1.Duration{Duration: 11 * time.Second}

		vpaUpdateMode       = vpaautoscalingv1.UpdateModeAuto
		containerPolicyOff  = vpaautoscalingv1.ContainerScalingModeOff
		containerPolicyAuto = vpaautoscalingv1.ContainerScalingModeAuto
		controlledValues    = vpaautoscalingv1.ContainerControlledValuesRequestsOnly
		metricsBasic        = druidv1alpha1.Basic
		metricsExtensive    = druidv1alpha1.Extensive

		etcdName = "etcd-" + testRole
		vpaName  = etcdName

		etcdObjFor = func(
			class Class,
			replicas int32,
			backupConfig *BackupConfig,
			existingDefragmentationSchedule,
			existingBackupSchedule string,
			existingResourcesContainerEtcd *corev1.ResourceRequirements,
			existingResourcesContainerBackupRestore *corev1.ResourceRequirements,
			caSecretName string,
			clientSecretName string,
			serverSecretName string,
			peerCASecretName *string,
			peerServerSecretName *string,
			topologyAwareRoutingEnabled bool,
			runtimeKubernetesVersion *semver.Version,
		) *druidv1alpha1.Etcd {
			defragSchedule := defragmentationSchedule
			if existingDefragmentationSchedule != "" {
				defragSchedule = existingDefragmentationSchedule
			}

			resourcesContainerEtcd := &corev1.ResourceRequirements{
				Requests: corev1.ResourceList{
					corev1.ResourceCPU:    resource.MustParse("300m"),
					corev1.ResourceMemory: resource.MustParse("1G"),
				},
			}
			if existingResourcesContainerEtcd != nil {
				resourcesContainerEtcd = existingResourcesContainerEtcd
			}

			resourcesContainerBackupRestore := &corev1.ResourceRequirements{
				Requests: corev1.ResourceList{
					corev1.ResourceCPU:    resource.MustParse("10m"),
					corev1.ResourceMemory: resource.MustParse("40Mi"),
				},
			}
			if existingResourcesContainerBackupRestore != nil {
				resourcesContainerBackupRestore = existingResourcesContainerBackupRestore
			}

			resourcesContainerCompactionJob := &corev1.ResourceRequirements{
				Requests: corev1.ResourceList{
					corev1.ResourceCPU:    resource.MustParse("600m"),
					corev1.ResourceMemory: resource.MustParse("3Gi"),
				},
			}

			clientService := &corev1.Service{
				ObjectMeta: metav1.ObjectMeta{
					Annotations: map[string]string{
						"networking.resources.gardener.cloud/from-all-scrape-targets-allowed-ports": `[{"protocol":"TCP","port":2379},{"protocol":"TCP","port":8080}]`,
						"networking.resources.gardener.cloud/namespace-selectors":                   `[{"matchLabels":{"kubernetes.io/metadata.name":"garden"}}]`,
						"networking.resources.gardener.cloud/pod-label-selector-namespace-alias":    "all-shoots",
					},
				},
			}
			if topologyAwareRoutingEnabled {
				if versionutils.ConstraintK8sGreaterEqual132.Check(runtimeKubernetesVersion) {
					clientService.Spec.TrafficDistribution = ptr.To(corev1.ServiceTrafficDistributionPreferClose)
				} else if versionutils.ConstraintK8sEqual131.Check(runtimeKubernetesVersion) {
					clientService.Spec.TrafficDistribution = ptr.To(corev1.ServiceTrafficDistributionPreferClose)
					metav1.SetMetaDataLabel(&clientService.ObjectMeta, "endpoint-slice-hints.resources.gardener.cloud/consider", "true")
				} else {
					metav1.SetMetaDataAnnotation(&clientService.ObjectMeta, "service.kubernetes.io/topology-mode", "auto")
					metav1.SetMetaDataLabel(&clientService.ObjectMeta, "endpoint-slice-hints.resources.gardener.cloud/consider", "true")
				}
			}

			obj := &druidv1alpha1.Etcd{
				ObjectMeta: metav1.ObjectMeta{
					Name:      etcdName,
					Namespace: testNamespace,
					Annotations: map[string]string{
						"gardener.cloud/operation": "reconcile",
						"gardener.cloud/timestamp": now.Format(time.RFC3339Nano),
					},
					Labels: map[string]string{
						"gardener.cloud/role": "controlplane",
						"role":                testRole,
					},
				},
				Spec: druidv1alpha1.EtcdSpec{
					Replicas:          replicas,
					PriorityClassName: &priorityClassName,
					Labels: map[string]string{
						"gardener.cloud/role":              "controlplane",
						"role":                             testRole,
						"app":                              "etcd-statefulset",
						"networking.gardener.cloud/to-dns": "allowed",
						"networking.gardener.cloud/to-public-networks":   "allowed",
						"networking.gardener.cloud/to-private-networks":  "allowed",
						"networking.gardener.cloud/to-runtime-apiserver": "allowed",
					},
					Selector: &metav1.LabelSelector{
						MatchLabels: map[string]string{
							"gardener.cloud/role": "controlplane",
							"role":                testRole,
							"app":                 "etcd-statefulset",
						},
					},
					Etcd: druidv1alpha1.EtcdConfig{
						Resources: resourcesContainerEtcd,
						ClientUrlTLS: &druidv1alpha1.TLSConfig{
							TLSCASecretRef: druidv1alpha1.SecretReference{
								SecretReference: corev1.SecretReference{
									Name:      caSecretName,
									Namespace: testNamespace,
								},
								DataKey: ptr.To("bundle.crt"),
							},
							ServerTLSSecretRef: corev1.SecretReference{
								Name:      serverSecretName,
								Namespace: testNamespace,
							},
							ClientTLSSecretRef: corev1.SecretReference{
								Name:      clientSecretName,
								Namespace: testNamespace,
							},
						},
						ServerPort:              ptr.To[int32](2380),
						ClientPort:              ptr.To[int32](2379),
						Metrics:                 &metricsBasic,
						DefragmentationSchedule: &defragSchedule,
						Quota:                   &quota,
						ClientService: &druidv1alpha1.ClientService{
							Annotations:         clientService.Annotations,
							Labels:              clientService.Labels,
							TrafficDistribution: clientService.Spec.TrafficDistribution,
						},
					},
					Backup: druidv1alpha1.BackupSpec{
						TLS: &druidv1alpha1.TLSConfig{
							TLSCASecretRef: druidv1alpha1.SecretReference{
								SecretReference: corev1.SecretReference{
									Name:      caSecretName,
									Namespace: testNamespace,
								},
								DataKey: ptr.To("bundle.crt"),
							},
							ServerTLSSecretRef: corev1.SecretReference{
								Name:      serverSecretName,
								Namespace: testNamespace,
							},
							ClientTLSSecretRef: corev1.SecretReference{
								Name:      clientSecretName,
								Namespace: testNamespace,
							},
						},
						Port:                    ptr.To[int32](8080),
						Resources:               resourcesContainerBackupRestore,
						CompactionResources:     resourcesContainerCompactionJob,
						GarbageCollectionPolicy: &garbageCollectionPolicy,
						GarbageCollectionPeriod: &garbageCollectionPeriod,
						SnapshotCompression:     &compressionSpec,
					},
					StorageCapacity:     &storageCapacityQuantity,
					StorageClass:        &storageClassName,
					VolumeClaimTemplate: ptr.To(etcdName),
				},
			}

			if class == ClassImportant {
				if replicas == 1 {
					obj.Spec.Annotations = map[string]string{"cluster-autoscaler.kubernetes.io/safe-to-evict": "false"}
				}
				obj.Spec.Backup.Resources = &corev1.ResourceRequirements{
					Requests: corev1.ResourceList{
						corev1.ResourceCPU:    resource.MustParse("20m"),
						corev1.ResourceMemory: resource.MustParse("80Mi"),
					},
				}
				obj.Spec.Etcd.Metrics = &metricsExtensive
				obj.Spec.VolumeClaimTemplate = ptr.To(testRole + "-etcd")
			} else if class == ClassNormal {
				metav1.SetMetaDataAnnotation(&obj.ObjectMeta, "resources.druid.gardener.cloud/allow-unhealthy-pod-eviction", "")
			}

			if replicas == 3 {
				obj.Spec.Labels = utils.MergeStringMaps(obj.Spec.Labels, map[string]string{
					"networking.resources.gardener.cloud/to-etcd-" + testRole + "-client-tcp-2379": "allowed",
					"networking.resources.gardener.cloud/to-etcd-" + testRole + "-client-tcp-2380": "allowed",
					"networking.resources.gardener.cloud/to-etcd-" + testRole + "-client-tcp-8080": "allowed",
				})
				obj.Spec.Etcd.PeerUrlTLS = &druidv1alpha1.TLSConfig{
					ServerTLSSecretRef: corev1.SecretReference{
						Name:      secretNameServerPeer,
						Namespace: testNamespace,
					},
					TLSCASecretRef: druidv1alpha1.SecretReference{
						SecretReference: corev1.SecretReference{
							Name:      secretNamePeerCA,
							Namespace: testNamespace,
						},
						DataKey: ptr.To(secretsutils.DataKeyCertificateBundle),
					},
				}
			}

			if ptr.Deref(peerServerSecretName, "") != "" {
				obj.Spec.Etcd.PeerUrlTLS.ServerTLSSecretRef = corev1.SecretReference{
					Name:      *peerServerSecretName,
					Namespace: testNamespace,
				}
			}

			if ptr.Deref(peerCASecretName, "") != "" {
				obj.Spec.Etcd.PeerUrlTLS.TLSCASecretRef = druidv1alpha1.SecretReference{
					SecretReference: corev1.SecretReference{
						Name:      *peerCASecretName,
						Namespace: testNamespace,
					},
					DataKey: ptr.To(secretsutils.DataKeyCertificateBundle),
				}
			}

			if backupConfig != nil {
				fullSnapshotSchedule := backupConfig.FullSnapshotSchedule
				if existingBackupSchedule != "" {
					fullSnapshotSchedule = existingBackupSchedule
				}

				provider := druidv1alpha1.StorageProvider(backupConfig.Provider)
				deltaSnapshotPeriod := metav1.Duration{Duration: 5 * time.Minute}
				deltaSnapshotMemoryLimit := resource.MustParse("100Mi")

				obj.Spec.Backup.Store = &druidv1alpha1.StoreSpec{
					SecretRef: &corev1.SecretReference{Name: backupConfig.SecretRefName},
					Container: &backupConfig.Container,
					Provider:  &provider,
					Prefix:    backupConfig.Prefix + "/etcd-" + testRole,
				}
				obj.Spec.Backup.FullSnapshotSchedule = &fullSnapshotSchedule
				obj.Spec.Backup.DeltaSnapshotPeriod = &deltaSnapshotPeriod
				obj.Spec.Backup.DeltaSnapshotRetentionPeriod = &metav1.Duration{Duration: 15 * 24 * time.Hour}
				obj.Spec.Backup.DeltaSnapshotMemoryLimit = &deltaSnapshotMemoryLimit

				if backupConfig.LeaderElection != nil {
					obj.Spec.Backup.LeaderElection = &druidv1alpha1.LeaderElectionSpec{
						EtcdConnectionTimeout: backupLeaderElectionEtcdConnectionTimeout,
						ReelectionPeriod:      backupLeaderElectionReelectionPeriod,
					}
				}
			}

			return obj
		}

		expectedVPAFor = func(class Class, evictionRequirement string, minAllowed corev1.ResourceList) *vpaautoscalingv1.VerticalPodAutoscaler {
			minAllowedConfig := minAllowed
			if minAllowedConfig == nil {
				minAllowedConfig = corev1.ResourceList{corev1.ResourceMemory: resource.MustParse("60M")}
			}

			vpa := &vpaautoscalingv1.VerticalPodAutoscaler{
				ObjectMeta: metav1.ObjectMeta{
					Name:      vpaName,
					Namespace: testNamespace,
					Labels:    map[string]string{v1beta1constants.LabelRole: "etcd-vpa-main"},
				},
				Spec: vpaautoscalingv1.VerticalPodAutoscalerSpec{
					TargetRef: &autoscalingv1.CrossVersionObjectReference{
						Name: etcdName, Kind: "StatefulSet",
						APIVersion: appsv1.SchemeGroupVersion.String(),
					},
					UpdatePolicy: &vpaautoscalingv1.PodUpdatePolicy{
						UpdateMode: &vpaUpdateMode,
					},
					ResourcePolicy: &vpaautoscalingv1.PodResourcePolicy{
						ContainerPolicies: []vpaautoscalingv1.ContainerResourcePolicy{
							{
								ContainerName:    "etcd",
								MinAllowed:       minAllowedConfig,
								ControlledValues: &controlledValues,
								Mode:             &containerPolicyAuto,
							},
							{
								ContainerName:    "backup-restore",
								Mode:             &containerPolicyOff,
								ControlledValues: &controlledValues,
							},
						},
					},
				},
			}

			switch evictionRequirement {
			case v1beta1constants.EvictionRequirementInMaintenanceWindowOnly:
				metav1.SetMetaDataLabel(&vpa.ObjectMeta, v1beta1constants.LabelVPAEvictionRequirementsController, v1beta1constants.EvictionRequirementManagedByController)
				metav1.SetMetaDataAnnotation(&vpa.ObjectMeta, v1beta1constants.AnnotationVPAEvictionRequirementDownscaleRestriction, v1beta1constants.EvictionRequirementInMaintenanceWindowOnly)
				metav1.SetMetaDataAnnotation(&vpa.ObjectMeta, v1beta1constants.AnnotationShootMaintenanceWindow, maintenanceTimeWindow.Begin+","+maintenanceTimeWindow.End)
			case v1beta1constants.EvictionRequirementNever:
				metav1.SetMetaDataLabel(&vpa.ObjectMeta, v1beta1constants.LabelVPAEvictionRequirementsController, v1beta1constants.EvictionRequirementManagedByController)
				metav1.SetMetaDataAnnotation(&vpa.ObjectMeta, v1beta1constants.AnnotationVPAEvictionRequirementDownscaleRestriction, v1beta1constants.EvictionRequirementNever)
			}

			if class == ClassImportant {
				vpa.Spec.ResourcePolicy.ContainerPolicies[0].MinAllowed = corev1.ResourceList{
					corev1.ResourceMemory: resource.MustParse("300M"),
				}
			}

			return vpa
		}

		serviceMonitorJobNames = func(prometheusName string) (string, string) {
			if prometheusName == "garden" {
				return "virtual-garden-etcd", "virtual-garden-etcd-backup"
			}
			return "kube-etcd3-" + testRole, "kube-etcd3-backup-restore-" + testRole
		}

		serviceMonitor = func(prometheusName, clientSecretName string) *monitoringv1.ServiceMonitor {
			jobNameEtcd, jobNameBackupRestore := serviceMonitorJobNames(prometheusName)

			return &monitoringv1.ServiceMonitor{
				ObjectMeta: metav1.ObjectMeta{
					Name:      prometheusName + "-" + etcdName,
					Namespace: testNamespace,
					Labels:    map[string]string{"prometheus": prometheusName},
				},
				Spec: monitoringv1.ServiceMonitorSpec{
					Selector: metav1.LabelSelector{MatchLabels: map[string]string{
						druidv1alpha1.LabelAppNameKey: fmt.Sprintf("%s-client", etcdName),
						druidv1alpha1.LabelPartOfKey:  etcdName,
					}},
					Endpoints: []monitoringv1.Endpoint{
						{
							Port:   "client",
							Scheme: "https",
							TLSConfig: &monitoringv1.TLSConfig{SafeTLSConfig: monitoringv1.SafeTLSConfig{
								InsecureSkipVerify: ptr.To(true),
								Cert: monitoringv1.SecretOrConfigMap{Secret: &corev1.SecretKeySelector{
									LocalObjectReference: corev1.LocalObjectReference{Name: clientSecretName},
									Key:                  secretsutils.DataKeyCertificate,
								}},
								KeySecret: &corev1.SecretKeySelector{
									LocalObjectReference: corev1.LocalObjectReference{Name: clientSecretName},
									Key:                  secretsutils.DataKeyPrivateKey,
								},
							}},
							RelabelConfigs: []monitoringv1.RelabelConfig{
								{
									SourceLabels: []monitoringv1.LabelName{"__meta_kubernetes_service_label_app_kubernetes_io_part_of"},
									TargetLabel:  "role",
								},
								{
									Action:      "replace",
									Replacement: ptr.To(jobNameEtcd),
									TargetLabel: "job",
								},
							},
							MetricRelabelConfigs: []monitoringv1.RelabelConfig{{
								Action: "labeldrop",
								Regex:  `^instance$`,
							}},
						},
						{
							Port:   "backuprestore",
							Scheme: "https",
							TLSConfig: &monitoringv1.TLSConfig{SafeTLSConfig: monitoringv1.SafeTLSConfig{
								InsecureSkipVerify: ptr.To(true),
								Cert: monitoringv1.SecretOrConfigMap{Secret: &corev1.SecretKeySelector{
									LocalObjectReference: corev1.LocalObjectReference{Name: clientSecretName},
									Key:                  secretsutils.DataKeyCertificate,
								}},
								KeySecret: &corev1.SecretKeySelector{
									LocalObjectReference: corev1.LocalObjectReference{Name: clientSecretName},
									Key:                  secretsutils.DataKeyPrivateKey,
								},
							}},
							RelabelConfigs: []monitoringv1.RelabelConfig{
								{
									SourceLabels: []monitoringv1.LabelName{"__meta_kubernetes_service_label_app_kubernetes_io_part_of"},
									TargetLabel:  "role",
								},

								{
									Action:      "replace",
									Replacement: ptr.To(jobNameBackupRestore),
									TargetLabel: "job",
								},
							},
							MetricRelabelConfigs: []monitoringv1.RelabelConfig{
								{
									Action: "labeldrop",
									Regex:  `^instance$`,
								},
								{
									SourceLabels: []monitoringv1.LabelName{"__name__"},
									Action:       "keep",
									Regex:        `^(etcdbr_defragmentation_duration_seconds_bucket|etcdbr_defragmentation_duration_seconds_count|etcdbr_defragmentation_duration_seconds_sum|etcdbr_network_received_bytes|etcdbr_network_transmitted_bytes|etcdbr_restoration_duration_seconds_bucket|etcdbr_restoration_duration_seconds_count|etcdbr_restoration_duration_seconds_sum|etcdbr_snapshot_duration_seconds_bucket|etcdbr_snapshot_duration_seconds_count|etcdbr_snapshot_duration_seconds_sum|etcdbr_snapshot_gc_total|etcdbr_snapshot_latest_revision|etcdbr_snapshot_latest_timestamp|etcdbr_snapshot_required|etcdbr_validation_duration_seconds_bucket|etcdbr_validation_duration_seconds_count|etcdbr_validation_duration_seconds_sum|etcdbr_snapshotter_failure|etcdbr_cluster_size|etcdbr_is_learner|etcdbr_is_learner_count_total|etcdbr_add_learner_duration_seconds_bucket|etcdbr_add_learner_duration_seconds_sum|etcdbr_member_remove_duration_seconds_bucket|etcdbr_member_remove_duration_seconds_sum|etcdbr_member_promote_duration_seconds_bucket|etcdbr_member_promote_duration_seconds_sum|process_resident_memory_bytes|process_cpu_seconds_total)$`,
								},
							},
						},
					},
				},
			}
		}
		prometheusRule = func(prometheusName string, class Class, replicas int32, backupEnabled bool) *monitoringv1.PrometheusRule {
			jobNameEtcd, jobNameBackupRestore := serviceMonitorJobNames(prometheusName)

			alertFor1, severity1, alertFor2 := "15m", "critical", "15m"
			if class == ClassImportant {
				alertFor1, severity1, alertFor2 = "5m", "blocker", "10m"
			}

			obj := &monitoringv1.PrometheusRule{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "shoot-" + etcdName,
					Namespace: testNamespace,
					Labels:    map[string]string{"prometheus": "shoot"},
				},
				Spec: monitoringv1.PrometheusRuleSpec{
					Groups: []monitoringv1.RuleGroup{{
						Name: jobNameEtcd + ".rules",
						Rules: []monitoringv1.Rule{
							{
								Alert: "KubeEtcd" + testROLE + "Down",
								Expr:  intstr.FromString(`sum(up{job="` + jobNameEtcd + `"}) < ` + strconv.Itoa(int(replicas/2)+1)),
								For:   ptr.To(monitoringv1.Duration(alertFor1)),
								Labels: map[string]string{
									"service":    "etcd",
									"severity":   severity1,
									"type":       "seed",
									"visibility": "operator",
								},
								Annotations: map[string]string{
									"summary":     "Etcd3 " + testRole + " cluster down.",
									"description": "Etcd3 cluster " + testRole + " is unavailable (due to possible quorum loss) or cannot be scraped. As long as etcd3 " + testRole + " is down, the cluster is unreachable.",
								},
							},
							// etcd leader alerts
							{
								Alert: "KubeEtcd3" + testROLE + "NoLeader",
								Expr:  intstr.FromString(`sum(etcd_server_has_leader{job="` + jobNameEtcd + `"}) < count(etcd_server_has_leader{job="` + jobNameEtcd + `"})`),
								For:   ptr.To(monitoringv1.Duration(alertFor2)),
								Labels: map[string]string{
									"service":    "etcd",
									"severity":   "critical",
									"type":       "seed",
									"visibility": "operator",
								},
								Annotations: map[string]string{
									"summary":     "Etcd3 " + testRole + " has no leader.",
									"description": "Etcd3 cluster " + testRole + " has no leader. Possible network partition in the etcd cluster.",
								},
							},
							{
								Alert: "KubeEtcd3" + testROLE + "HighMemoryConsumption",
								Expr:  intstr.FromString(`sum(container_memory_working_set_bytes{pod="etcd-` + testRole + `-0",container="etcd"}) / sum(kube_verticalpodautoscaler_spec_resourcepolicy_container_policies_maxallowed{container="etcd", targetName="etcd-` + testRole + `", resource="memory"}) > .5`),
								For:   ptr.To(monitoringv1.Duration("15m")),
								Labels: map[string]string{
									"service":    "etcd",
									"severity":   "warning",
									"type":       "seed",
									"visibility": "operator",
								},
								Annotations: map[string]string{
									"summary":     "Etcd3 " + testRole + " is consuming too much memory",
									"description": "Etcd3 " + testRole + " is consuming over 50% of the max allowed value specified by VPA.",
								},
							},
							{
								Alert: "KubeEtcd3" + testROLE + "DbSizeLimitApproaching",
								Expr:  intstr.FromString(`(etcd_mvcc_db_total_size_in_bytes{job="` + jobNameEtcd + `"} > bool 7516193000) + (etcd_mvcc_db_total_size_in_bytes{job="` + jobNameEtcd + `"} <= bool 8589935000) == 2`),
								Labels: map[string]string{
									"service":    "etcd",
									"severity":   "warning",
									"type":       "seed",
									"visibility": "all",
								},
								Annotations: map[string]string{
									"summary":     "Etcd3 " + testRole + " DB size is approaching its current practical limit.",
									"description": "Etcd3 " + testRole + " DB size is approaching its current practical limit of 8GB. Etcd quota might need to be increased.",
								},
							},
							{
								Alert: "KubeEtcd3" + testROLE + "DbSizeLimitCrossed",
								Expr:  intstr.FromString(`etcd_mvcc_db_total_size_in_bytes{job="` + jobNameEtcd + `"} > 8589935000`),
								Labels: map[string]string{
									"service":    "etcd",
									"severity":   "critical",
									"type":       "seed",
									"visibility": "all",
								},
								Annotations: map[string]string{
									"summary":     "Etcd3 " + testRole + " DB size has crossed its current practical limit.",
									"description": "Etcd3 " + testRole + " DB size has crossed its current practical limit of 8GB. Etcd quota must be increased to allow updates.",
								},
							},
							{
								Record: "shoot:apiserver_storage_objects:sum_by_resource",
								Expr:   intstr.FromString(`max(apiserver_storage_objects) by (resource)`),
							},
						},
					}},
				},
			}

			if backupEnabled {
				obj.Spec.Groups[0].Rules = append(obj.Spec.Groups[0].Rules,
					monitoringv1.Rule{
						Alert: "KubeEtcd" + testROLE + "DeltaBackupFailed",
						Expr:  intstr.FromString(`((time() - etcdbr_snapshot_latest_timestamp{job="` + jobNameBackupRestore + `",kind="Incr"} > bool 900) * etcdbr_snapshot_required{job="` + jobNameBackupRestore + `",kind="Incr"}) * on (pod, role) etcd_server_is_leader{job="` + jobNameEtcd + `"} > 0`),
						For:   ptr.To(monitoringv1.Duration("15m")),
						Labels: map[string]string{
							"service":    "etcd",
							"severity":   "critical",
							"type":       "seed",
							"visibility": "operator",
						},
						Annotations: map[string]string{
							"summary":     "Etcd delta snapshot failure.",
							"description": "No delta snapshot for the past 30 minutes have been taken by backup-restore leader.",
						},
					},
					monitoringv1.Rule{
						Alert: "KubeEtcd" + testROLE + "FullBackupFailed",
						Expr:  intstr.FromString(`((time() - etcdbr_snapshot_latest_timestamp{job="` + jobNameBackupRestore + `",kind="Full"} > bool 86400) * etcdbr_snapshot_required{job="` + jobNameBackupRestore + `",kind="Full"}) * on (pod, role) etcd_server_is_leader{job="` + jobNameEtcd + `"} > 0`),
						For:   ptr.To(monitoringv1.Duration("15m")),
						Labels: map[string]string{
							"service":    "etcd",
							"severity":   "critical",
							"type":       "seed",
							"visibility": "operator",
						},
						Annotations: map[string]string{
							"summary":     "Etcd full snapshot failure.",
							"description": "No full snapshot for at least last 24 hours have been taken by backup-restore leader.",
						},
					},
					monitoringv1.Rule{
						Alert: "KubeEtcd" + testROLE + "RestorationFailed",
						Expr:  intstr.FromString(`rate(etcdbr_restoration_duration_seconds_count{job="` + jobNameBackupRestore + `",succeeded="false"}[2m]) > 0`),
						Labels: map[string]string{
							"service":    "etcd",
							"severity":   "critical",
							"type":       "seed",
							"visibility": "operator",
						},
						Annotations: map[string]string{
							"summary":     "Etcd data restoration failure.",
							"description": "Etcd data restoration was triggered, but has failed.",
						},
					},
					// etcd backup failure alert
					monitoringv1.Rule{
						Alert: "KubeEtcd" + testROLE + "BackupRestoreDown",
						Expr:  intstr.FromString(`(sum(up{job="` + jobNameEtcd + `"}) - sum(up{job="` + jobNameBackupRestore + `"}) > 0) or (rate(etcdbr_snapshotter_failure{job="` + jobNameBackupRestore + `"}[5m]) > 0)`),
						For:   ptr.To(monitoringv1.Duration("10m")),
						Labels: map[string]string{
							"service":    "etcd",
							"severity":   "critical",
							"type":       "seed",
							"visibility": "operator",
						},
						Annotations: map[string]string{
							"summary":     "Etcd backup restore " + testRole + " process down or snapshotter failed with error",
							"description": "Etcd backup restore " + testRole + " process down or snapshotter failed with error. Backups will not be triggered unless backup restore is brought back up. This is unsafe behaviour and may cause data loss.",
						},
					},
				)
			}

			return obj
		}
	)

	BeforeEach(func() {
		caRotationPhase = ""
		fakeClient = fakeclient.NewClientBuilder().WithScheme(kubernetesscheme.Scheme).Build()
		sm = fakesecretsmanager.New(fakeClient, testNamespace)
		autoscalingConfig = AutoscalingConfig{}
		backupConfig = nil
		replicas = ptr.To[int32](1)
		highAvailabilityEnabled = false
	})

	JustBeforeEach(func() {
		ctrl = gomock.NewController(GinkgoT())
		c = mockclient.NewMockClient(ctrl)
		log = logr.Discard()

		By("Create secrets managed outside of this package for whose secretsmanager.Get() will be called")
		Expect(fakeClient.Create(ctx, &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: "ca-etcd", Namespace: testNamespace}})).To(Succeed())
		etcd = New(log, c, testNamespace, sm, Values{
			Role:                    testRole,
			Class:                   class,
			Replicas:                replicas,
			Autoscaling:             autoscalingConfig,
			StorageCapacity:         storageCapacity,
			StorageClassName:        &storageClassName,
			DefragmentationSchedule: &defragmentationSchedule,
			CARotationPhase:         caRotationPhase,
			PriorityClassName:       priorityClassName,
			MaintenanceTimeWindow:   maintenanceTimeWindow,
			HighAvailabilityEnabled: highAvailabilityEnabled,
			BackupConfig:            backupConfig,
		})
	})

	AfterEach(func() {
		ctrl.Finish()
	})

	Describe("#Deploy", func() {
		It("should fail because the etcd object retrieval fails", func() {
			c.EXPECT().Get(ctx, client.ObjectKey{Namespace: testNamespace, Name: etcdName}, gomock.AssignableToTypeOf(&druidv1alpha1.Etcd{})).Return(fakeErr)

			Expect(etcd.Deploy(ctx)).To(MatchError(fakeErr))
		})

		It("should fail because the etcd cannot be created", func() {
			gomock.InOrder(
				c.EXPECT().Get(ctx, client.ObjectKey{Namespace: testNamespace, Name: etcdName}, gomock.AssignableToTypeOf(&druidv1alpha1.Etcd{})).Return(apierrors.NewNotFound(schema.GroupResource{}, "")),
				c.EXPECT().Get(ctx, client.ObjectKey{Namespace: testNamespace, Name: etcdName}, gomock.AssignableToTypeOf(&druidv1alpha1.Etcd{})),
				c.EXPECT().Patch(ctx, gomock.AssignableToTypeOf(&druidv1alpha1.Etcd{}), gomock.Any()).Return(fakeErr),
			)

			Expect(etcd.Deploy(ctx)).To(MatchError(fakeErr))
		})

		It("should successfully deploy (normal etcd)", func() {
			oldTimeNow := TimeNow
			defer func() { TimeNow = oldTimeNow }()
			TimeNow = func() time.Time { return now }

			gomock.InOrder(
				c.EXPECT().Get(ctx, client.ObjectKey{Namespace: testNamespace, Name: etcdName}, gomock.AssignableToTypeOf(&druidv1alpha1.Etcd{})).Return(apierrors.NewNotFound(schema.GroupResource{}, "")),
				c.EXPECT().Get(ctx, client.ObjectKey{Namespace: testNamespace, Name: etcdName}, gomock.AssignableToTypeOf(&druidv1alpha1.Etcd{})),
				c.EXPECT().Patch(ctx, gomock.AssignableToTypeOf(&druidv1alpha1.Etcd{}), gomock.Any()).Do(func(_ context.Context, obj client.Object, _ client.Patch, _ ...client.PatchOption) {
					Expect(obj).To(DeepEqual(etcdObjFor(
						class,
						1,
						nil,
						"",
						"",
						nil,
						nil,
						secretNameCA,
						secretNameClient,
						secretNameServer,
						nil,
						nil,
						false,
						nil)))
				}),
				c.EXPECT().Get(ctx, client.ObjectKey{Namespace: testNamespace, Name: vpaName}, gomock.AssignableToTypeOf(&vpaautoscalingv1.VerticalPodAutoscaler{})).Return(apierrors.NewNotFound(schema.GroupResource{}, "")),
				c.EXPECT().Create(ctx, gomock.AssignableToTypeOf(&vpaautoscalingv1.VerticalPodAutoscaler{}), gomock.Any()).Do(func(_ context.Context, obj client.Object, _ ...client.CreateOption) {
					Expect(obj).To(DeepEqual(expectedVPAFor(class, "", nil)))
				}),
				c.EXPECT().Get(ctx, client.ObjectKey{Namespace: testNamespace, Name: "shoot-etcd-" + testRole}, gomock.AssignableToTypeOf(&monitoringv1.ServiceMonitor{})),
				c.EXPECT().Patch(ctx, gomock.AssignableToTypeOf(&monitoringv1.ServiceMonitor{}), gomock.Any()).Do(func(_ context.Context, obj client.Object, _ client.Patch, _ ...client.PatchOption) {
					Expect(obj).To(DeepEqual(serviceMonitor("shoot", "etcd-client")))
				}),
				c.EXPECT().Get(ctx, client.ObjectKey{Namespace: testNamespace, Name: "shoot-etcd-" + testRole}, gomock.AssignableToTypeOf(&monitoringv1.PrometheusRule{})),
				c.EXPECT().Patch(ctx, gomock.AssignableToTypeOf(&monitoringv1.PrometheusRule{}), gomock.Any()).Do(func(_ context.Context, obj client.Object, _ client.Patch, _ ...client.PatchOption) {
					Expect(obj).To(DeepEqual(prometheusRule("shoot", class, 1, false)))
				}),
			)

			Expect(etcd.Deploy(ctx)).To(Succeed())
		})

		It("should not panic during deploy when etcd resource exists, but its status is not yet populated", func() {
			oldTimeNow := TimeNow
			defer func() { TimeNow = oldTimeNow }()
			TimeNow = func() time.Time { return now }

			var (
				existingReplicas int32 = 245
			)

			etcd = New(log, c, testNamespace, sm, Values{
				Role:                    testRole,
				Class:                   class,
				Replicas:                nil,
				StorageCapacity:         storageCapacity,
				StorageClassName:        &storageClassName,
				DefragmentationSchedule: &defragmentationSchedule,
				CARotationPhase:         "",
				PriorityClassName:       priorityClassName,
				MaintenanceTimeWindow:   maintenanceTimeWindow,
			})

			gomock.InOrder(
				c.EXPECT().Get(ctx, client.ObjectKey{Namespace: testNamespace, Name: etcdName}, gomock.AssignableToTypeOf(&druidv1alpha1.Etcd{})).Return(apierrors.NewNotFound(schema.GroupResource{}, "")).DoAndReturn(func(_ context.Context, _ client.ObjectKey, obj client.Object, _ ...client.GetOption) error {
					(&druidv1alpha1.Etcd{
						ObjectMeta: metav1.ObjectMeta{
							Name:      etcdName,
							Namespace: testNamespace,
						},
						Spec: druidv1alpha1.EtcdSpec{
							Replicas: existingReplicas,
						},
					}).DeepCopyInto(obj.(*druidv1alpha1.Etcd))
					return nil
				}),
				c.EXPECT().Get(ctx, client.ObjectKey{Namespace: testNamespace, Name: etcdName}, gomock.AssignableToTypeOf(&druidv1alpha1.Etcd{})),
				c.EXPECT().Patch(ctx, gomock.AssignableToTypeOf(&druidv1alpha1.Etcd{}), gomock.Any()).Do(func(_ context.Context, obj *druidv1alpha1.Etcd, _ client.Patch, _ ...client.PatchOption) {
					// ignore status when comparing
					obj.Status = druidv1alpha1.EtcdStatus{}

					Expect(obj).To(DeepEqual(etcdObjFor(
						class,
						existingReplicas,
						nil,
						"",
						"",
						nil,
						nil,
						secretNameCA,
						secretNameClient,
						secretNameServer,
						nil,
						nil,
						false,
						nil)))
				}),
				c.EXPECT().Get(ctx, client.ObjectKey{Namespace: testNamespace, Name: vpaName}, gomock.AssignableToTypeOf(&vpaautoscalingv1.VerticalPodAutoscaler{})).Return(apierrors.NewNotFound(schema.GroupResource{}, "")),
				c.EXPECT().Create(ctx, gomock.AssignableToTypeOf(&vpaautoscalingv1.VerticalPodAutoscaler{}), gomock.Any()).Do(func(_ context.Context, obj client.Object, _ ...client.CreateOption) {
					Expect(obj).To(DeepEqual(expectedVPAFor(class, "", nil)))
				}),
				c.EXPECT().Get(ctx, client.ObjectKey{Namespace: testNamespace, Name: "shoot-etcd-" + testRole}, gomock.AssignableToTypeOf(&monitoringv1.ServiceMonitor{})),
				c.EXPECT().Patch(ctx, gomock.AssignableToTypeOf(&monitoringv1.ServiceMonitor{}), gomock.Any()).Do(func(_ context.Context, obj client.Object, _ client.Patch, _ ...client.PatchOption) {
					Expect(obj).To(DeepEqual(serviceMonitor("shoot", "etcd-client")))
				}),
				c.EXPECT().Get(ctx, client.ObjectKey{Namespace: testNamespace, Name: "shoot-etcd-" + testRole}, gomock.AssignableToTypeOf(&monitoringv1.PrometheusRule{})),
				c.EXPECT().Patch(ctx, gomock.AssignableToTypeOf(&monitoringv1.PrometheusRule{}), gomock.Any()).Do(func(_ context.Context, obj client.Object, _ client.Patch, _ ...client.PatchOption) {
					Expect(obj).To(DeepEqual(prometheusRule("shoot", class, existingReplicas, false)))
				}),
			)

			Expect(etcd.Deploy(ctx)).To(Succeed())
		})

		It("should successfully deploy (normal etcd) and retain replicas (etcd found)", func() {
			oldTimeNow := TimeNow
			defer func() { TimeNow = oldTimeNow }()
			TimeNow = func() time.Time { return now }

			var (
				existingReplicas int32 = 245
			)

			etcd = New(log, c, testNamespace, sm, Values{
				Role:                    testRole,
				Class:                   class,
				Replicas:                nil,
				StorageCapacity:         storageCapacity,
				StorageClassName:        &storageClassName,
				DefragmentationSchedule: &defragmentationSchedule,
				CARotationPhase:         "",
				MaintenanceTimeWindow:   maintenanceTimeWindow,
				PriorityClassName:       priorityClassName,
			})

			gomock.InOrder(
				c.EXPECT().Get(ctx, client.ObjectKey{Namespace: testNamespace, Name: etcdName}, gomock.AssignableToTypeOf(&druidv1alpha1.Etcd{})).Return(apierrors.NewNotFound(schema.GroupResource{}, "")).DoAndReturn(func(_ context.Context, _ client.ObjectKey, obj client.Object, _ ...client.GetOption) error {
					(&druidv1alpha1.Etcd{
						ObjectMeta: metav1.ObjectMeta{
							Name:      etcdName,
							Namespace: testNamespace,
						},
						Spec: druidv1alpha1.EtcdSpec{
							Replicas: existingReplicas,
						},
						Status: druidv1alpha1.EtcdStatus{
							Etcd: &druidv1alpha1.CrossVersionObjectReference{
								Name: etcdName,
							},
						},
					}).DeepCopyInto(obj.(*druidv1alpha1.Etcd))
					return nil
				}),
				c.EXPECT().Get(ctx, client.ObjectKey{Namespace: testNamespace, Name: etcdName}, gomock.AssignableToTypeOf(&druidv1alpha1.Etcd{})),
				c.EXPECT().Patch(ctx, gomock.AssignableToTypeOf(&druidv1alpha1.Etcd{}), gomock.Any()).Do(func(_ context.Context, obj *druidv1alpha1.Etcd, _ client.Patch, _ ...client.PatchOption) {
					// ignore status when comparing
					obj.Status = druidv1alpha1.EtcdStatus{}

					Expect(obj).To(DeepEqual(etcdObjFor(
						class,
						existingReplicas,
						nil,
						"",
						"",
						nil,
						nil,
						secretNameCA,
						secretNameClient,
						secretNameServer,
						nil,
						nil,
						false,
						nil)))
				}),
				c.EXPECT().Get(ctx, client.ObjectKey{Namespace: testNamespace, Name: vpaName}, gomock.AssignableToTypeOf(&vpaautoscalingv1.VerticalPodAutoscaler{})).Return(apierrors.NewNotFound(schema.GroupResource{}, "")),
				c.EXPECT().Create(ctx, gomock.AssignableToTypeOf(&vpaautoscalingv1.VerticalPodAutoscaler{}), gomock.Any()).Do(func(_ context.Context, obj client.Object, _ ...client.CreateOption) {
					Expect(obj).To(DeepEqual(expectedVPAFor(class, "", nil)))
				}),
				c.EXPECT().Get(ctx, client.ObjectKey{Namespace: testNamespace, Name: "shoot-etcd-" + testRole}, gomock.AssignableToTypeOf(&monitoringv1.ServiceMonitor{})),
				c.EXPECT().Patch(ctx, gomock.AssignableToTypeOf(&monitoringv1.ServiceMonitor{}), gomock.Any()).Do(func(_ context.Context, obj client.Object, _ client.Patch, _ ...client.PatchOption) {
					Expect(obj).To(DeepEqual(serviceMonitor("shoot", "etcd-client")))
				}),
				c.EXPECT().Get(ctx, client.ObjectKey{Namespace: testNamespace, Name: "shoot-etcd-" + testRole}, gomock.AssignableToTypeOf(&monitoringv1.PrometheusRule{})),
				c.EXPECT().Patch(ctx, gomock.AssignableToTypeOf(&monitoringv1.PrometheusRule{}), gomock.Any()).Do(func(_ context.Context, obj client.Object, _ client.Patch, _ ...client.PatchOption) {
					Expect(obj).To(DeepEqual(prometheusRule("shoot", class, existingReplicas, false)))
				}),
			)

			Expect(etcd.Deploy(ctx)).To(Succeed())
		})

		It("should successfully deploy (normal etcd) and retain annotations (etcd found)", func() {
			oldTimeNow := TimeNow
			defer func() { TimeNow = oldTimeNow }()
			TimeNow = func() time.Time { return now }

			gomock.InOrder(
				c.EXPECT().Get(ctx, client.ObjectKey{Namespace: testNamespace, Name: etcdName}, gomock.AssignableToTypeOf(&druidv1alpha1.Etcd{})).Return(apierrors.NewNotFound(schema.GroupResource{}, "")).DoAndReturn(func(_ context.Context, _ client.ObjectKey, obj client.Object, _ ...client.GetOption) error {
					(&druidv1alpha1.Etcd{
						ObjectMeta: metav1.ObjectMeta{
							Annotations: map[string]string{
								"foo": "bar",
							},
							Name:      etcdName,
							Namespace: testNamespace,
						},
						Status: druidv1alpha1.EtcdStatus{
							Etcd: &druidv1alpha1.CrossVersionObjectReference{
								Name: etcdName,
							},
						},
					}).DeepCopyInto(obj.(*druidv1alpha1.Etcd))
					return nil
				}),
				c.EXPECT().Get(ctx, client.ObjectKey{Namespace: testNamespace, Name: etcdName}, gomock.AssignableToTypeOf(&druidv1alpha1.Etcd{})),
				c.EXPECT().Patch(ctx, gomock.AssignableToTypeOf(&druidv1alpha1.Etcd{}), gomock.Any()).Do(func(_ context.Context, obj *druidv1alpha1.Etcd, _ client.Patch, _ ...client.PatchOption) {
					// ignore status when comparing
					obj.Status = druidv1alpha1.EtcdStatus{}

					expectedObj := etcdObjFor(
						class,
						1,
						nil,
						"",
						"",
						nil,
						nil,
						secretNameCA,
						secretNameClient,
						secretNameServer,
						nil,
						nil,
						false,
						nil)
					expectedObj.Annotations = utils.MergeStringMaps(expectedObj.Annotations, map[string]string{
						"foo": "bar",
					})

					Expect(obj).To(DeepEqual(expectedObj))
				}),
				c.EXPECT().Get(ctx, client.ObjectKey{Namespace: testNamespace, Name: vpaName}, gomock.AssignableToTypeOf(&vpaautoscalingv1.VerticalPodAutoscaler{})).Return(apierrors.NewNotFound(schema.GroupResource{}, "")),
				c.EXPECT().Create(ctx, gomock.AssignableToTypeOf(&vpaautoscalingv1.VerticalPodAutoscaler{}), gomock.Any()).Do(func(_ context.Context, obj client.Object, _ ...client.CreateOption) {
					Expect(obj).To(DeepEqual(expectedVPAFor(class, "", nil)))
				}),
				c.EXPECT().Get(ctx, client.ObjectKey{Namespace: testNamespace, Name: "shoot-etcd-" + testRole}, gomock.AssignableToTypeOf(&monitoringv1.ServiceMonitor{})),
				c.EXPECT().Patch(ctx, gomock.AssignableToTypeOf(&monitoringv1.ServiceMonitor{}), gomock.Any()).Do(func(_ context.Context, obj client.Object, _ client.Patch, _ ...client.PatchOption) {
					Expect(obj).To(DeepEqual(serviceMonitor("shoot", "etcd-client")))
				}),
				c.EXPECT().Get(ctx, client.ObjectKey{Namespace: testNamespace, Name: "shoot-etcd-" + testRole}, gomock.AssignableToTypeOf(&monitoringv1.PrometheusRule{})),
				c.EXPECT().Patch(ctx, gomock.AssignableToTypeOf(&monitoringv1.PrometheusRule{}), gomock.Any()).Do(func(_ context.Context, obj client.Object, _ client.Patch, _ ...client.PatchOption) {
					Expect(obj).To(DeepEqual(prometheusRule("shoot", class, 1, false)))
				}),
			)

			Expect(etcd.Deploy(ctx)).To(Succeed())
		})

		It("should successfully deploy (normal etcd) and keep the existing defragmentation schedule", func() {
			oldTimeNow := TimeNow
			defer func() { TimeNow = oldTimeNow }()
			TimeNow = func() time.Time { return now }

			existingDefragmentationSchedule := "foobardefragexisting"

			gomock.InOrder(
				c.EXPECT().Get(ctx, client.ObjectKey{Namespace: testNamespace, Name: etcdName}, gomock.AssignableToTypeOf(&druidv1alpha1.Etcd{})).DoAndReturn(func(_ context.Context, _ client.ObjectKey, obj client.Object, _ ...client.GetOption) error {
					(&druidv1alpha1.Etcd{
						ObjectMeta: metav1.ObjectMeta{
							Name:      etcdName,
							Namespace: testNamespace,
						},
						Spec: druidv1alpha1.EtcdSpec{
							Etcd: druidv1alpha1.EtcdConfig{
								DefragmentationSchedule: &existingDefragmentationSchedule,
							},
						},
						Status: druidv1alpha1.EtcdStatus{
							Etcd: &druidv1alpha1.CrossVersionObjectReference{
								Name: etcdName,
							},
						},
					}).DeepCopyInto(obj.(*druidv1alpha1.Etcd))
					return nil
				}),
				c.EXPECT().Get(ctx, client.ObjectKey{Namespace: testNamespace, Name: etcdName}, gomock.AssignableToTypeOf(&druidv1alpha1.Etcd{})),
				c.EXPECT().Patch(ctx, gomock.AssignableToTypeOf(&druidv1alpha1.Etcd{}), gomock.Any()).Do(func(_ context.Context, obj *druidv1alpha1.Etcd, _ client.Patch, _ ...client.PatchOption) {
					// ignore status when comparing
					obj.Status = druidv1alpha1.EtcdStatus{}

					Expect(obj).To(DeepEqual(etcdObjFor(
						class,
						1,
						nil,
						existingDefragmentationSchedule,
						"",
						nil,
						nil,
						secretNameCA,
						secretNameClient,
						secretNameServer,
						nil,
						nil,
						false,
						nil)))
				}),
				c.EXPECT().Get(ctx, client.ObjectKey{Namespace: testNamespace, Name: vpaName}, gomock.AssignableToTypeOf(&vpaautoscalingv1.VerticalPodAutoscaler{})).Return(apierrors.NewNotFound(schema.GroupResource{}, "")),
				c.EXPECT().Create(ctx, gomock.AssignableToTypeOf(&vpaautoscalingv1.VerticalPodAutoscaler{}), gomock.Any()).Do(func(_ context.Context, obj client.Object, _ ...client.CreateOption) {
					Expect(obj).To(DeepEqual(expectedVPAFor(class, "", nil)))
				}),
				c.EXPECT().Get(ctx, client.ObjectKey{Namespace: testNamespace, Name: "shoot-etcd-" + testRole}, gomock.AssignableToTypeOf(&monitoringv1.ServiceMonitor{})),
				c.EXPECT().Patch(ctx, gomock.AssignableToTypeOf(&monitoringv1.ServiceMonitor{}), gomock.Any()).Do(func(_ context.Context, obj client.Object, _ client.Patch, _ ...client.PatchOption) {
					Expect(obj).To(DeepEqual(serviceMonitor("shoot", "etcd-client")))
				}),
				c.EXPECT().Get(ctx, client.ObjectKey{Namespace: testNamespace, Name: "shoot-etcd-" + testRole}, gomock.AssignableToTypeOf(&monitoringv1.PrometheusRule{})),
				c.EXPECT().Patch(ctx, gomock.AssignableToTypeOf(&monitoringv1.PrometheusRule{}), gomock.Any()).Do(func(_ context.Context, obj client.Object, _ client.Patch, _ ...client.PatchOption) {
					Expect(obj).To(DeepEqual(prometheusRule("shoot", class, 1, false)))
				}),
			)

			Expect(etcd.Deploy(ctx)).To(Succeed())
		})

		It("should successfully deploy (normal etcd) and not keep the existing resource request settings", func() {
			oldTimeNow := TimeNow
			defer func() { TimeNow = oldTimeNow }()
			TimeNow = func() time.Time { return now }

			gomock.InOrder(
				c.EXPECT().Get(ctx, client.ObjectKey{Namespace: testNamespace, Name: etcdName}, gomock.AssignableToTypeOf(&druidv1alpha1.Etcd{})).Return(apierrors.NewNotFound(schema.GroupResource{}, "")),
				c.EXPECT().Get(ctx, client.ObjectKey{Namespace: testNamespace, Name: etcdName}, gomock.AssignableToTypeOf(&druidv1alpha1.Etcd{})),
				c.EXPECT().Patch(ctx, gomock.AssignableToTypeOf(&druidv1alpha1.Etcd{}), gomock.Any()).Do(func(_ context.Context, obj client.Object, _ client.Patch, _ ...client.PatchOption) {
					Expect(obj).To(DeepEqual(etcdObjFor(
						class,
						1,
						nil,
						"",
						"",
						nil,
						nil,
						secretNameCA,
						secretNameClient,
						secretNameServer,
						nil,
						nil,
						false,
						nil)))
				}),
				c.EXPECT().Get(ctx, client.ObjectKey{Namespace: testNamespace, Name: vpaName}, gomock.AssignableToTypeOf(&vpaautoscalingv1.VerticalPodAutoscaler{})).Return(apierrors.NewNotFound(schema.GroupResource{}, "")),
				c.EXPECT().Create(ctx, gomock.AssignableToTypeOf(&vpaautoscalingv1.VerticalPodAutoscaler{}), gomock.Any()).Do(func(_ context.Context, obj client.Object, _ ...client.CreateOption) {
					Expect(obj).To(DeepEqual(expectedVPAFor(class, "", nil)))
				}),
				c.EXPECT().Get(ctx, client.ObjectKey{Namespace: testNamespace, Name: "shoot-etcd-" + testRole}, gomock.AssignableToTypeOf(&monitoringv1.ServiceMonitor{})),
				c.EXPECT().Patch(ctx, gomock.AssignableToTypeOf(&monitoringv1.ServiceMonitor{}), gomock.Any()).Do(func(_ context.Context, obj client.Object, _ client.Patch, _ ...client.PatchOption) {
					Expect(obj).To(DeepEqual(serviceMonitor("shoot", "etcd-client")))
				}),
				c.EXPECT().Get(ctx, client.ObjectKey{Namespace: testNamespace, Name: "shoot-etcd-" + testRole}, gomock.AssignableToTypeOf(&monitoringv1.PrometheusRule{})),
				c.EXPECT().Patch(ctx, gomock.AssignableToTypeOf(&monitoringv1.PrometheusRule{}), gomock.Any()).Do(func(_ context.Context, obj client.Object, _ client.Patch, _ ...client.PatchOption) {
					Expect(obj).To(DeepEqual(prometheusRule("shoot", class, 1, false)))
				}),
			)

			Expect(etcd.Deploy(ctx)).To(Succeed())
		})

		for _, shootPurpose := range []gardencorev1beta1.ShootPurpose{gardencorev1beta1.ShootPurposeEvaluation, gardencorev1beta1.ShootPurposeProduction} {
			var purpose = shootPurpose
			It(fmt.Sprintf("should successfully deploy (important etcd): purpose = %q", purpose), func() {
				oldTimeNow := TimeNow
				defer func() { TimeNow = oldTimeNow }()
				TimeNow = func() time.Time { return now }

				class := ClassImportant

				evictionRequirement := v1beta1constants.EvictionRequirementInMaintenanceWindowOnly

				if purpose == gardencorev1beta1.ShootPurposeProduction {
					evictionRequirement = v1beta1constants.EvictionRequirementNever
				}

				etcd = New(log, c, testNamespace, sm, Values{
					Role:                    testRole,
					Class:                   class,
					Replicas:                replicas,
					StorageCapacity:         storageCapacity,
					StorageClassName:        &storageClassName,
					DefragmentationSchedule: &defragmentationSchedule,
					CARotationPhase:         "",
					MaintenanceTimeWindow:   maintenanceTimeWindow,
					PriorityClassName:       priorityClassName,
					EvictionRequirement:     ptr.To(evictionRequirement),
				})

				gomock.InOrder(
					c.EXPECT().Get(ctx, client.ObjectKey{Namespace: testNamespace, Name: etcdName}, gomock.AssignableToTypeOf(&druidv1alpha1.Etcd{})).Return(apierrors.NewNotFound(schema.GroupResource{}, "")),
					c.EXPECT().Get(ctx, client.ObjectKey{Namespace: testNamespace, Name: etcdName}, gomock.AssignableToTypeOf(&druidv1alpha1.Etcd{})),
					c.EXPECT().Patch(ctx, gomock.AssignableToTypeOf(&druidv1alpha1.Etcd{}), gomock.Any()).Do(func(_ context.Context, obj client.Object, _ client.Patch, _ ...client.PatchOption) {
						Expect(obj).To(DeepEqual(etcdObjFor(
							class,
							1,
							nil,
							"",
							"",
							nil,
							nil,
							secretNameCA,
							secretNameClient,
							secretNameServer,
							nil,
							nil,
							false,
							nil)))
					}),
					c.EXPECT().Get(ctx, client.ObjectKey{Namespace: testNamespace, Name: vpaName}, gomock.AssignableToTypeOf(&vpaautoscalingv1.VerticalPodAutoscaler{})).Return(apierrors.NewNotFound(schema.GroupResource{}, "")),
					c.EXPECT().Create(ctx, gomock.AssignableToTypeOf(&vpaautoscalingv1.VerticalPodAutoscaler{}), gomock.Any()).Do(func(_ context.Context, obj client.Object, _ ...client.CreateOption) {
						Expect(obj).To(DeepEqual(expectedVPAFor(class, evictionRequirement, nil)))
					}),
					c.EXPECT().Get(ctx, client.ObjectKey{Namespace: testNamespace, Name: "shoot-etcd-" + testRole}, gomock.AssignableToTypeOf(&monitoringv1.ServiceMonitor{})),
					c.EXPECT().Patch(ctx, gomock.AssignableToTypeOf(&monitoringv1.ServiceMonitor{}), gomock.Any()).Do(func(_ context.Context, obj client.Object, _ client.Patch, _ ...client.PatchOption) {
						Expect(obj).To(DeepEqual(serviceMonitor("shoot", "etcd-client")))
					}),
					c.EXPECT().Get(ctx, client.ObjectKey{Namespace: testNamespace, Name: "shoot-etcd-" + testRole}, gomock.AssignableToTypeOf(&monitoringv1.PrometheusRule{})),
					c.EXPECT().Patch(ctx, gomock.AssignableToTypeOf(&monitoringv1.PrometheusRule{}), gomock.Any()).Do(func(_ context.Context, obj client.Object, _ client.Patch, _ ...client.PatchOption) {
						Expect(obj).To(DeepEqual(prometheusRule("shoot", class, 1, false)))
					}),
				)

				Expect(etcd.Deploy(ctx)).To(Succeed())
			})
		}

		When("backup is configured", func() {
			var backupConfig = &BackupConfig{
				Provider:                     "prov",
				SecretRefName:                "secret-key",
				Prefix:                       "prefix",
				Container:                    "bucket",
				FullSnapshotSchedule:         "1234",
				DeltaSnapshotRetentionPeriod: &metav1.Duration{Duration: 15 * 24 * time.Hour},
			}

			JustBeforeEach(func() {
				etcd.SetBackupConfig(backupConfig)
			})

			It("should successfully deploy (with backup)", func() {
				oldTimeNow := TimeNow
				defer func() { TimeNow = oldTimeNow }()
				TimeNow = func() time.Time { return now }

				gomock.InOrder(
					c.EXPECT().Get(ctx, client.ObjectKey{Namespace: testNamespace, Name: etcdName}, gomock.AssignableToTypeOf(&druidv1alpha1.Etcd{})).Return(apierrors.NewNotFound(schema.GroupResource{}, "")),
					c.EXPECT().Get(ctx, client.ObjectKey{Namespace: testNamespace, Name: etcdName}, gomock.AssignableToTypeOf(&druidv1alpha1.Etcd{})),
					c.EXPECT().Patch(ctx, gomock.AssignableToTypeOf(&druidv1alpha1.Etcd{}), gomock.Any()).Do(func(_ context.Context, obj client.Object, _ client.Patch, _ ...client.PatchOption) {
						Expect(obj).To(DeepEqual(etcdObjFor(
							class,
							1,
							backupConfig,
							"",
							"",
							nil,
							nil,
							secretNameCA,
							secretNameClient,
							secretNameServer,
							nil,
							nil,
							false,
							nil)))
					}),
					c.EXPECT().Get(ctx, client.ObjectKey{Namespace: testNamespace, Name: vpaName}, gomock.AssignableToTypeOf(&vpaautoscalingv1.VerticalPodAutoscaler{})).Return(apierrors.NewNotFound(schema.GroupResource{}, "")),
					c.EXPECT().Create(ctx, gomock.AssignableToTypeOf(&vpaautoscalingv1.VerticalPodAutoscaler{}), gomock.Any()).Do(func(_ context.Context, obj client.Object, _ ...client.CreateOption) {
						Expect(obj).To(DeepEqual(expectedVPAFor(class, "", nil)))
					}),
					c.EXPECT().Get(ctx, client.ObjectKey{Namespace: testNamespace, Name: "shoot-etcd-" + testRole}, gomock.AssignableToTypeOf(&monitoringv1.ServiceMonitor{})),
					c.EXPECT().Patch(ctx, gomock.AssignableToTypeOf(&monitoringv1.ServiceMonitor{}), gomock.Any()).Do(func(_ context.Context, obj client.Object, _ client.Patch, _ ...client.PatchOption) {
						Expect(obj).To(DeepEqual(serviceMonitor("shoot", "etcd-client")))
					}),
					c.EXPECT().Get(ctx, client.ObjectKey{Namespace: testNamespace, Name: "shoot-etcd-" + testRole}, gomock.AssignableToTypeOf(&monitoringv1.PrometheusRule{})),
					c.EXPECT().Patch(ctx, gomock.AssignableToTypeOf(&monitoringv1.PrometheusRule{}), gomock.Any()).Do(func(_ context.Context, obj client.Object, _ client.Patch, _ ...client.PatchOption) {
						Expect(obj).To(DeepEqual(prometheusRule("shoot", class, *replicas, true)))
					}),
				)

				Expect(etcd.Deploy(ctx)).To(Succeed())
			})

			It("should successfully deploy (with backup) and keep the existing backup schedule", func() {
				oldTimeNow := TimeNow
				defer func() { TimeNow = oldTimeNow }()
				TimeNow = func() time.Time { return now }

				existingBackupSchedule := "foobarbackupexisting"

				gomock.InOrder(
					c.EXPECT().Get(ctx, client.ObjectKey{Namespace: testNamespace, Name: etcdName}, gomock.AssignableToTypeOf(&druidv1alpha1.Etcd{})).DoAndReturn(func(_ context.Context, _ client.ObjectKey, obj client.Object, _ ...client.GetOption) error {
						(&druidv1alpha1.Etcd{
							ObjectMeta: metav1.ObjectMeta{
								Name:      etcdName,
								Namespace: testNamespace,
							},
							Spec: druidv1alpha1.EtcdSpec{
								Backup: druidv1alpha1.BackupSpec{
									FullSnapshotSchedule: &existingBackupSchedule,
								},
							},
							Status: druidv1alpha1.EtcdStatus{
								Etcd: &druidv1alpha1.CrossVersionObjectReference{
									Name: "",
								},
							},
						}).DeepCopyInto(obj.(*druidv1alpha1.Etcd))
						return nil
					}),
					c.EXPECT().Get(ctx, client.ObjectKey{Namespace: testNamespace, Name: etcdName}, gomock.AssignableToTypeOf(&druidv1alpha1.Etcd{})),
					c.EXPECT().Patch(ctx, gomock.AssignableToTypeOf(&druidv1alpha1.Etcd{}), gomock.Any()).Do(func(_ context.Context, obj client.Object, _ client.Patch, _ ...client.PatchOption) {
						expobj := etcdObjFor(
							class,
							1,
							backupConfig,
							"",
							existingBackupSchedule,
							nil,
							nil,
							secretNameCA,
							secretNameClient,
							secretNameServer,
							nil,
							nil,
							false,
							nil)
						expobj.Status.Etcd = &druidv1alpha1.CrossVersionObjectReference{}

						Expect(obj).To(DeepEqual(expobj))
					}),
					c.EXPECT().Get(ctx, client.ObjectKey{Namespace: testNamespace, Name: vpaName}, gomock.AssignableToTypeOf(&vpaautoscalingv1.VerticalPodAutoscaler{})).Return(apierrors.NewNotFound(schema.GroupResource{}, "")),
					c.EXPECT().Create(ctx, gomock.AssignableToTypeOf(&vpaautoscalingv1.VerticalPodAutoscaler{}), gomock.Any()).Do(func(_ context.Context, obj client.Object, _ ...client.CreateOption) {
						Expect(obj).To(DeepEqual(expectedVPAFor(class, "", nil)))
					}),
					c.EXPECT().Get(ctx, client.ObjectKey{Namespace: testNamespace, Name: "shoot-etcd-" + testRole}, gomock.AssignableToTypeOf(&monitoringv1.ServiceMonitor{})),
					c.EXPECT().Patch(ctx, gomock.AssignableToTypeOf(&monitoringv1.ServiceMonitor{}), gomock.Any()).Do(func(_ context.Context, obj client.Object, _ client.Patch, _ ...client.PatchOption) {
						Expect(obj).To(DeepEqual(serviceMonitor("shoot", "etcd-client")))
					}),
					c.EXPECT().Get(ctx, client.ObjectKey{Namespace: testNamespace, Name: "shoot-etcd-" + testRole}, gomock.AssignableToTypeOf(&monitoringv1.PrometheusRule{})),
					c.EXPECT().Patch(ctx, gomock.AssignableToTypeOf(&monitoringv1.PrometheusRule{}), gomock.Any()).Do(func(_ context.Context, obj client.Object, _ client.Patch, _ ...client.PatchOption) {
						Expect(obj).To(DeepEqual(prometheusRule("shoot", class, *replicas, true)))
					}),
				)

				Expect(etcd.Deploy(ctx)).To(Succeed())
			})
		})

		When("HA setup is configured", func() {
			BeforeEach(func() {
				highAvailabilityEnabled = true
				replicas = ptr.To[int32](3)
				Expect(fakeClient.Create(ctx, &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: "ca-etcd-peer", Namespace: testNamespace}})).To(Succeed())
			})

			createExpectations := func(caSecretName, clientSecretName, serverSecretName, peerCASecretName, peerServerSecretName string) {
				gomock.InOrder(
					c.EXPECT().Get(ctx, client.ObjectKey{Namespace: testNamespace, Name: etcdName}, gomock.AssignableToTypeOf(&druidv1alpha1.Etcd{})).Return(apierrors.NewNotFound(schema.GroupResource{}, "")),
					c.EXPECT().Get(ctx, client.ObjectKey{Namespace: testNamespace, Name: etcdName}, gomock.AssignableToTypeOf(&druidv1alpha1.Etcd{})).DoAndReturn(
						func(_ context.Context, _ client.ObjectKey, etcd *druidv1alpha1.Etcd, _ ...client.GetOption) error {
							if peerServerSecretName != "" {
								etcd.Spec.Etcd.PeerUrlTLS = &druidv1alpha1.TLSConfig{
									ServerTLSSecretRef: corev1.SecretReference{
										Name:      peerServerSecretName,
										Namespace: testNamespace,
									},
								}
							}
							return nil
						}),
					c.EXPECT().Patch(ctx, gomock.AssignableToTypeOf(&druidv1alpha1.Etcd{}), gomock.Any()).Do(func(_ context.Context, obj client.Object, _ client.Patch, _ ...client.PatchOption) {
						Expect(obj).To(DeepEqual(etcdObjFor(
							class,
							3,
							nil,
							"",
							"",
							nil,
							nil,
							caSecretName,
							clientSecretName,
							serverSecretName,
							&peerCASecretName,
							&peerServerSecretName,
							false,
							nil,
						)))
					}),
					c.EXPECT().Get(ctx, client.ObjectKey{Namespace: testNamespace, Name: vpaName}, gomock.AssignableToTypeOf(&vpaautoscalingv1.VerticalPodAutoscaler{})).Return(apierrors.NewNotFound(schema.GroupResource{}, "")),
					c.EXPECT().Create(ctx, gomock.AssignableToTypeOf(&vpaautoscalingv1.VerticalPodAutoscaler{}), gomock.Any()).Do(func(_ context.Context, obj client.Object, _ ...client.CreateOption) {
						Expect(obj).To(DeepEqual(expectedVPAFor(class, "", nil)))
					}),
					c.EXPECT().Get(ctx, client.ObjectKey{Namespace: testNamespace, Name: "shoot-etcd-" + testRole}, gomock.AssignableToTypeOf(&monitoringv1.ServiceMonitor{})),
					c.EXPECT().Patch(ctx, gomock.AssignableToTypeOf(&monitoringv1.ServiceMonitor{}), gomock.Any()).Do(func(_ context.Context, obj client.Object, _ client.Patch, _ ...client.PatchOption) {
						Expect(obj).To(DeepEqual(serviceMonitor("shoot", clientSecretName)))
					}),
					c.EXPECT().Get(ctx, client.ObjectKey{Namespace: testNamespace, Name: "shoot-etcd-" + testRole}, gomock.AssignableToTypeOf(&monitoringv1.PrometheusRule{})),
					c.EXPECT().Patch(ctx, gomock.AssignableToTypeOf(&monitoringv1.PrometheusRule{}), gomock.Any()).Do(func(_ context.Context, obj client.Object, _ client.Patch, _ ...client.PatchOption) {
						Expect(obj).To(DeepEqual(prometheusRule("shoot", class, *replicas, false)))
					}),
				)
			}

			Context("when CA rotation phase is in `Preparing` state", func() {
				var (
					clientCASecret *corev1.Secret
					peerCASecret   *corev1.Secret
				)

				BeforeEach(func() {
					caRotationPhase = gardencorev1beta1.RotationPreparing

					secretNamesToTimes := map[string]time.Time{}

					// A "real" SecretsManager is needed here because in further tests we want to differentiate
					// between what was issued by the old and new CAs.
					var err error
					sm, err = secretsmanager.New(
						ctx,
						logr.New(logf.NullLogSink{}),
						testclock.NewFakeClock(time.Now()),
						fakeClient,
						testNamespace,
						"",
						secretsmanager.Config{
							SecretNamesToTimes: secretNamesToTimes,
						})
					Expect(err).ToNot(HaveOccurred())

					// Create new etcd CA
					_, err = sm.Generate(ctx,
						&secretsutils.CertificateSecretConfig{Name: v1beta1constants.SecretNameCAETCD, CommonName: "etcd", CertType: secretsutils.CACert})
					Expect(err).ToNot(HaveOccurred())

					// Create new peer CA
					_, err = sm.Generate(ctx,
						&secretsutils.CertificateSecretConfig{Name: v1beta1constants.SecretNameCAETCDPeer, CommonName: "etcd-peer", CertType: secretsutils.CACert})
					Expect(err).ToNot(HaveOccurred())

					// Set time to trigger CA rotation
					secretNamesToTimes[v1beta1constants.SecretNameCAETCDPeer] = now

					// Rotate CA
					_, err = sm.Generate(ctx,
						&secretsutils.CertificateSecretConfig{Name: v1beta1constants.SecretNameCAETCDPeer, CommonName: "etcd-peer", CertType: secretsutils.CACert},
						secretsmanager.Rotate(secretsmanager.KeepOld))
					Expect(err).ToNot(HaveOccurred())

					var ok bool
					clientCASecret, ok = sm.Get(v1beta1constants.SecretNameCAETCD)
					Expect(ok).To(BeTrue())

					peerCASecret, ok = sm.Get(v1beta1constants.SecretNameCAETCDPeer)
					Expect(ok).To(BeTrue())

					DeferCleanup(func() {
						caRotationPhase = ""
					})
				})

				It("should successfully deploy", func() {
					oldTimeNow := TimeNow
					defer func() { TimeNow = oldTimeNow }()
					TimeNow = func() time.Time { return now }

					peerServerSecret, err := sm.Generate(ctx, &secretsutils.CertificateSecretConfig{
						Name:       "etcd-peer-server-" + testRole,
						CommonName: "etcd-server",
						DNSNames: []string{
							"etcd-" + testRole + "-peer",
							"etcd-" + testRole + "-peer.shoot--test--test",
							"etcd-" + testRole + "-peer.shoot--test--test.svc",
							"etcd-" + testRole + "-peer.shoot--test--test.svc.cluster.local",
							"*.etcd-" + testRole + "-peer",
							"*.etcd-" + testRole + "-peer.shoot--test--test",
							"*.etcd-" + testRole + "-peer.shoot--test--test.svc",
							"*.etcd-" + testRole + "-peer.shoot--test--test.svc.cluster.local",
						},
						CertType:                    secretsutils.ServerClientCert,
						SkipPublishingCACertificate: true,
					}, secretsmanager.SignedByCA(v1beta1constants.SecretNameCAETCDPeer, secretsmanager.UseCurrentCA), secretsmanager.Rotate(secretsmanager.InPlace))
					Expect(err).ToNot(HaveOccurred())

					clientSecret, err := sm.Generate(ctx, &secretsutils.CertificateSecretConfig{
						Name:                        SecretNameClient,
						CommonName:                  "etcd-client",
						CertType:                    secretsutils.ClientCert,
						SkipPublishingCACertificate: true,
					}, secretsmanager.SignedByCA(v1beta1constants.SecretNameCAETCD), secretsmanager.Rotate(secretsmanager.InPlace))
					Expect(err).ToNot(HaveOccurred())

					serverSecret, err := sm.Generate(ctx, &secretsutils.CertificateSecretConfig{
						Name:       "etcd-server-" + testRole,
						CommonName: "etcd-server",
						DNSNames: []string{
							"etcd-" + testRole + "-local",
							"etcd-" + testRole + "-client",
							"etcd-" + testRole + "-client.shoot--test--test",
							"etcd-" + testRole + "-client.shoot--test--test.svc",
							"etcd-" + testRole + "-client.shoot--test--test.svc.cluster.local",
							"*.etcd-" + testRole + "-peer",
							"*.etcd-" + testRole + "-peer.shoot--test--test",
							"*.etcd-" + testRole + "-peer.shoot--test--test.svc",
							"*.etcd-" + testRole + "-peer.shoot--test--test.svc.cluster.local",
						},
						CertType:                    secretsutils.ServerClientCert,
						SkipPublishingCACertificate: true,
					}, secretsmanager.SignedByCA(v1beta1constants.SecretNameCAETCD), secretsmanager.Rotate(secretsmanager.InPlace))
					Expect(err).ToNot(HaveOccurred())

					createExpectations(clientCASecret.Name, clientSecret.Name, serverSecret.Name, peerCASecret.Name, peerServerSecret.Name)

					Expect(etcd.Deploy(ctx)).To(Succeed())
				})
			})
		})

		When("etcd cluster is hibernated", func() {
			BeforeEach(func() {
				secretNamesToTimes := map[string]time.Time{}
				replicas = ptr.To[int32](0)
				caRotationPhase = gardencorev1beta1.RotationCompleted

				var err error
				sm, err = secretsmanager.New(
					ctx,
					logr.New(logf.NullLogSink{}),
					testclock.NewFakeClock(time.Now()),
					fakeClient,
					testNamespace,
					"",
					secretsmanager.Config{
						SecretNamesToTimes: secretNamesToTimes,
					})
				Expect(err).ToNot(HaveOccurred())

				// Create new etcd CA
				_, err = sm.Generate(ctx, &secretsutils.CertificateSecretConfig{Name: v1beta1constants.SecretNameCAETCD, CommonName: "etcd", CertType: secretsutils.CACert})
				Expect(err).ToNot(HaveOccurred())

				// Create new peer CA
				_, err = sm.Generate(ctx, &secretsutils.CertificateSecretConfig{Name: v1beta1constants.SecretNameCAETCDPeer, CommonName: "etcd-peer", CertType: secretsutils.CACert})
				Expect(err).ToNot(HaveOccurred())
			})

			Context("when peer url secrets are present in etcd CR", func() {
				BeforeEach(func() {
					highAvailabilityEnabled = true
				})

				It("should not remove peer URL secrets", func() {
					var clientSecretName string

					gomock.InOrder(
						c.EXPECT().Get(ctx, client.ObjectKey{Namespace: testNamespace, Name: etcdName}, gomock.AssignableToTypeOf(&druidv1alpha1.Etcd{})).DoAndReturn(func(_ context.Context, _ client.ObjectKey, obj client.Object, _ ...client.GetOption) error {
							(&druidv1alpha1.Etcd{
								ObjectMeta: metav1.ObjectMeta{
									Name:      etcdName,
									Namespace: testNamespace,
								},
								Spec: druidv1alpha1.EtcdSpec{
									Replicas: 3,
									Etcd: druidv1alpha1.EtcdConfig{
										PeerUrlTLS: &druidv1alpha1.TLSConfig{
											ServerTLSSecretRef: corev1.SecretReference{
												Name:      "peerServerSecretName",
												Namespace: testNamespace,
											},
										},
									},
								},
							}).DeepCopyInto(obj.(*druidv1alpha1.Etcd))
							return nil
						}),
						c.EXPECT().Get(ctx, client.ObjectKey{Namespace: testNamespace, Name: etcdName}, gomock.AssignableToTypeOf(&druidv1alpha1.Etcd{})).DoAndReturn(
							func(_ context.Context, _ client.ObjectKey, etcd *druidv1alpha1.Etcd, _ ...client.GetOption) error {
								etcd.Spec.Etcd.PeerUrlTLS = &druidv1alpha1.TLSConfig{
									ServerTLSSecretRef: corev1.SecretReference{
										Name:      "peerServerSecretName",
										Namespace: testNamespace,
									},
								}
								return nil
							}),
						c.EXPECT().Patch(ctx, gomock.AssignableToTypeOf(&druidv1alpha1.Etcd{}), gomock.Any()).Do(func(_ context.Context, obj client.Object, _ client.Patch, _ ...client.PatchOption) {
							Expect(obj.(*druidv1alpha1.Etcd).Spec.Replicas).To(Equal(int32(0)))
							Expect(obj.(*druidv1alpha1.Etcd).Spec.Etcd.PeerUrlTLS).NotTo(BeNil())
							clientSecretName = obj.(*druidv1alpha1.Etcd).Spec.Etcd.ClientUrlTLS.ClientTLSSecretRef.Name
						}),
						c.EXPECT().Get(ctx, client.ObjectKey{Namespace: testNamespace, Name: vpaName}, gomock.AssignableToTypeOf(&vpaautoscalingv1.VerticalPodAutoscaler{})).Return(apierrors.NewNotFound(schema.GroupResource{}, "")),
						c.EXPECT().Create(ctx, gomock.AssignableToTypeOf(&vpaautoscalingv1.VerticalPodAutoscaler{}), gomock.Any()).Do(func(_ context.Context, obj client.Object, _ ...client.CreateOption) {
							Expect(obj).To(DeepEqual(expectedVPAFor(class, "", nil)))
						}),
						c.EXPECT().Get(ctx, client.ObjectKey{Namespace: testNamespace, Name: "shoot-etcd-" + testRole}, gomock.AssignableToTypeOf(&monitoringv1.ServiceMonitor{})),
						c.EXPECT().Patch(ctx, gomock.AssignableToTypeOf(&monitoringv1.ServiceMonitor{}), gomock.Any()).Do(func(_ context.Context, obj client.Object, _ client.Patch, _ ...client.PatchOption) {
							Expect(obj).To(DeepEqual(serviceMonitor("shoot", clientSecretName)))
						}),
						c.EXPECT().Get(ctx, client.ObjectKey{Namespace: testNamespace, Name: "shoot-etcd-" + testRole}, gomock.AssignableToTypeOf(&monitoringv1.PrometheusRule{})),
						c.EXPECT().Patch(ctx, gomock.AssignableToTypeOf(&monitoringv1.PrometheusRule{}), gomock.Any()).Do(func(_ context.Context, obj client.Object, _ client.Patch, _ ...client.PatchOption) {
							Expect(obj).To(DeepEqual(prometheusRule("shoot", class, 1, false)))
						}),
					)

					Expect(etcd.Deploy(ctx)).To(Succeed())
				})
			})

			Context("when peer url secrets are not present in etcd CR", func() {
				BeforeEach(func() {
					highAvailabilityEnabled = true
				})

				It("should add peer url secrets", func() {
					var clientSecretName string

					gomock.InOrder(
						c.EXPECT().Get(ctx, client.ObjectKey{Namespace: testNamespace, Name: etcdName}, gomock.AssignableToTypeOf(&druidv1alpha1.Etcd{})).DoAndReturn(func(_ context.Context, _ client.ObjectKey, obj client.Object, _ ...client.GetOption) error {
							(&druidv1alpha1.Etcd{
								ObjectMeta: metav1.ObjectMeta{
									Name:      etcdName,
									Namespace: testNamespace,
								},
								Spec: druidv1alpha1.EtcdSpec{
									Replicas: 3,
									Etcd: druidv1alpha1.EtcdConfig{
										PeerUrlTLS: nil,
									},
								},
							}).DeepCopyInto(obj.(*druidv1alpha1.Etcd))
							return nil
						}),
						c.EXPECT().Get(ctx, client.ObjectKey{Namespace: testNamespace, Name: etcdName}, gomock.AssignableToTypeOf(&druidv1alpha1.Etcd{})).DoAndReturn(
							func(_ context.Context, _ client.ObjectKey, _ *druidv1alpha1.Etcd, _ ...client.GetOption) error {
								return nil
							}),
						c.EXPECT().Patch(ctx, gomock.AssignableToTypeOf(&druidv1alpha1.Etcd{}), gomock.Any()).Do(func(_ context.Context, obj client.Object, _ client.Patch, _ ...client.PatchOption) {
							Expect(obj.(*druidv1alpha1.Etcd).Spec.Replicas).To(Equal(int32(0)))
							Expect(obj.(*druidv1alpha1.Etcd).Spec.Etcd.PeerUrlTLS).NotTo(BeNil())
							clientSecretName = obj.(*druidv1alpha1.Etcd).Spec.Etcd.ClientUrlTLS.ClientTLSSecretRef.Name
						}),
						c.EXPECT().Get(ctx, client.ObjectKey{Namespace: testNamespace, Name: vpaName}, gomock.AssignableToTypeOf(&vpaautoscalingv1.VerticalPodAutoscaler{})).Return(apierrors.NewNotFound(schema.GroupResource{}, "")),
						c.EXPECT().Create(ctx, gomock.AssignableToTypeOf(&vpaautoscalingv1.VerticalPodAutoscaler{}), gomock.Any()).Do(func(_ context.Context, obj client.Object, _ ...client.CreateOption) {
							Expect(obj).To(DeepEqual(expectedVPAFor(class, "", nil)))
						}),
						c.EXPECT().Get(ctx, client.ObjectKey{Namespace: testNamespace, Name: "shoot-etcd-" + testRole}, gomock.AssignableToTypeOf(&monitoringv1.ServiceMonitor{})),
						c.EXPECT().Patch(ctx, gomock.AssignableToTypeOf(&monitoringv1.ServiceMonitor{}), gomock.Any()).Do(func(_ context.Context, obj client.Object, _ client.Patch, _ ...client.PatchOption) {
							Expect(obj).To(DeepEqual(serviceMonitor("shoot", clientSecretName)))
						}),
						c.EXPECT().Get(ctx, client.ObjectKey{Namespace: testNamespace, Name: "shoot-etcd-" + testRole}, gomock.AssignableToTypeOf(&monitoringv1.PrometheusRule{})),
						c.EXPECT().Patch(ctx, gomock.AssignableToTypeOf(&monitoringv1.PrometheusRule{}), gomock.Any()).Do(func(_ context.Context, obj client.Object, _ client.Patch, _ ...client.PatchOption) {
							Expect(obj).To(DeepEqual(prometheusRule("shoot", class, 1, false)))
						}),
					)

					Expect(etcd.Deploy(ctx)).To(Succeed())
				})
			})
		})

		When("TopologyAwareRoutingEnabled=true", func() {
			DescribeTable("should successfully deploy with expected etcd client service annotation, label and spec field",
				func(runtimeKubernetesVersion *semver.Version) {
					oldTimeNow := TimeNow
					defer func() { TimeNow = oldTimeNow }()
					TimeNow = func() time.Time { return now }

					class := ClassImportant

					replicas = ptr.To[int32](1)

					etcd = New(log, c, testNamespace, sm, Values{
						Role:                        testRole,
						Class:                       class,
						Replicas:                    replicas,
						StorageCapacity:             storageCapacity,
						StorageClassName:            &storageClassName,
						DefragmentationSchedule:     &defragmentationSchedule,
						CARotationPhase:             "",
						RuntimeKubernetesVersion:    runtimeKubernetesVersion,
						PriorityClassName:           priorityClassName,
						MaintenanceTimeWindow:       maintenanceTimeWindow,
						TopologyAwareRoutingEnabled: true,
					})

					gomock.InOrder(
						c.EXPECT().Get(ctx, client.ObjectKey{Namespace: testNamespace, Name: etcdName}, gomock.AssignableToTypeOf(&druidv1alpha1.Etcd{})).Return(apierrors.NewNotFound(schema.GroupResource{}, "")),
						c.EXPECT().Get(ctx, client.ObjectKey{Namespace: testNamespace, Name: etcdName}, gomock.AssignableToTypeOf(&druidv1alpha1.Etcd{})),
						c.EXPECT().Patch(ctx, gomock.AssignableToTypeOf(&druidv1alpha1.Etcd{}), gomock.Any()).Do(func(_ context.Context, obj client.Object, _ client.Patch, _ ...client.PatchOption) {
							Expect(obj).To(DeepEqual(etcdObjFor(
								class,
								1,
								nil,
								"",
								"",
								nil,
								nil,
								secretNameCA,
								secretNameClient,
								secretNameServer,
								nil,
								nil,
								true,
								runtimeKubernetesVersion)))
						}),
						c.EXPECT().Get(ctx, client.ObjectKey{Namespace: testNamespace, Name: vpaName}, gomock.AssignableToTypeOf(&vpaautoscalingv1.VerticalPodAutoscaler{})).Return(apierrors.NewNotFound(schema.GroupResource{}, "")),
						c.EXPECT().Create(ctx, gomock.AssignableToTypeOf(&vpaautoscalingv1.VerticalPodAutoscaler{}), gomock.Any()).Do(func(_ context.Context, obj client.Object, _ ...client.CreateOption) {
							Expect(obj).To(DeepEqual(expectedVPAFor(class, "", nil)))
						}),
						c.EXPECT().Get(ctx, client.ObjectKey{Namespace: testNamespace, Name: "shoot-etcd-" + testRole}, gomock.AssignableToTypeOf(&monitoringv1.ServiceMonitor{})),
						c.EXPECT().Patch(ctx, gomock.AssignableToTypeOf(&monitoringv1.ServiceMonitor{}), gomock.Any()).Do(func(_ context.Context, obj client.Object, _ client.Patch, _ ...client.PatchOption) {
							Expect(obj).To(DeepEqual(serviceMonitor("shoot", "etcd-client")))
						}),
						c.EXPECT().Get(ctx, client.ObjectKey{Namespace: testNamespace, Name: "shoot-etcd-" + testRole}, gomock.AssignableToTypeOf(&monitoringv1.PrometheusRule{})),
						c.EXPECT().Patch(ctx, gomock.AssignableToTypeOf(&monitoringv1.PrometheusRule{}), gomock.Any()).Do(func(_ context.Context, obj client.Object, _ client.Patch, _ ...client.PatchOption) {
							Expect(obj).To(DeepEqual(prometheusRule("shoot", class, 1, false)))
						}),
					)

					Expect(etcd.Deploy(ctx)).To(Succeed())
				},

				Entry("when runtime Kubernetes version is >= 1.32", semver.MustParse("1.32.1")),
				Entry("when runtime Kubernetes version is 1.31", semver.MustParse("1.31.2")),
				Entry("when runtime Kubernetes version is < 1.31", semver.MustParse("1.30.3")),
			)
		})

		When("name prefix is set", func() {
			It("should successfully deploy the service monitor", func() {
				oldTimeNow := TimeNow
				defer func() { TimeNow = oldTimeNow }()
				TimeNow = func() time.Time { return now }

				class := ClassImportant

				replicas = ptr.To[int32](1)

				etcd = New(log, c, testNamespace, sm, Values{
					Role:                    testRole,
					Class:                   class,
					Replicas:                replicas,
					StorageCapacity:         storageCapacity,
					StorageClassName:        &storageClassName,
					DefragmentationSchedule: &defragmentationSchedule,
					CARotationPhase:         "",
					PriorityClassName:       priorityClassName,
					NamePrefix:              "virtual-garden-",
				})

				DeferCleanup(test.WithVar(&etcdName, "virtual-garden-"+etcdName))
				DeferCleanup(test.WithVar(&vpaName, "virtual-garden-"+vpaName))

				etcdObj := etcdObjFor(
					class,
					1,
					nil,
					"",
					"",
					nil,
					nil,
					secretNameCA,
					secretNameClient,
					secretNameServer,
					nil,
					nil,
					false,
					nil,
				)
				etcdObj.Name = etcdName
				etcdObj.Spec.VolumeClaimTemplate = ptr.To(testRole + "-virtual-garden-etcd")
				etcdObj.Spec.Etcd.ClientService.Annotations["networking.resources.gardener.cloud/from-all-garden-scrape-targets-allowed-ports"] = `[{"protocol":"TCP","port":2379},{"protocol":"TCP","port":8080}]`
				delete(etcdObj.Spec.Etcd.ClientService.Annotations, "networking.resources.gardener.cloud/from-all-scrape-targets-allowed-ports")
				delete(etcdObj.Spec.Etcd.ClientService.Annotations, "networking.resources.gardener.cloud/namespace-selectors")
				delete(etcdObj.Spec.Etcd.ClientService.Annotations, "networking.resources.gardener.cloud/pod-label-selector-namespace-alias")

				gomock.InOrder(
					c.EXPECT().Get(ctx, client.ObjectKey{Namespace: testNamespace, Name: etcdName}, gomock.AssignableToTypeOf(&druidv1alpha1.Etcd{})).Return(apierrors.NewNotFound(schema.GroupResource{}, "")),
					c.EXPECT().Get(ctx, client.ObjectKey{Namespace: testNamespace, Name: etcdName}, gomock.AssignableToTypeOf(&druidv1alpha1.Etcd{})),
					c.EXPECT().Patch(ctx, gomock.AssignableToTypeOf(&druidv1alpha1.Etcd{}), gomock.Any()).Do(func(_ context.Context, obj client.Object, _ client.Patch, _ ...client.PatchOption) {
						Expect(obj).To(DeepEqual(etcdObj))
					}),
					c.EXPECT().Get(ctx, client.ObjectKey{Namespace: testNamespace, Name: vpaName}, gomock.AssignableToTypeOf(&vpaautoscalingv1.VerticalPodAutoscaler{})).Return(apierrors.NewNotFound(schema.GroupResource{}, "")),
					c.EXPECT().Create(ctx, gomock.AssignableToTypeOf(&vpaautoscalingv1.VerticalPodAutoscaler{}), gomock.Any()).Do(func(_ context.Context, obj client.Object, _ ...client.CreateOption) {
						Expect(obj).To(DeepEqual(expectedVPAFor(class, "", nil)))
					}),
					c.EXPECT().Get(ctx, client.ObjectKey{Namespace: testNamespace, Name: "garden-virtual-garden-etcd-" + testRole}, gomock.AssignableToTypeOf(&monitoringv1.ServiceMonitor{})),
					c.EXPECT().Patch(ctx, gomock.AssignableToTypeOf(&monitoringv1.ServiceMonitor{}), gomock.Any()).Do(func(_ context.Context, obj client.Object, _ client.Patch, _ ...client.PatchOption) {
						Expect(obj).To(DeepEqual(serviceMonitor("garden", "etcd-client")))
					}),
				)

				Expect(etcd.Deploy(ctx)).To(Succeed())
			})
		})

		When("minAllowed is configured", func() {
			var minAllowed corev1.ResourceList

			BeforeEach(func() {
				minAllowed = corev1.ResourceList{
					corev1.ResourceCPU:    resource.MustParse("250m"),
					corev1.ResourceMemory: resource.MustParse("1.5Gi"),
				}

				autoscalingConfig.MinAllowed = minAllowed
			})

			It("should successfully deploy the VPA resource", func() {
				oldTimeNow := TimeNow
				defer func() { TimeNow = oldTimeNow }()
				TimeNow = func() time.Time { return now }

				gomock.InOrder(
					c.EXPECT().Get(ctx, client.ObjectKey{Namespace: testNamespace, Name: etcdName}, gomock.AssignableToTypeOf(&druidv1alpha1.Etcd{})).Return(apierrors.NewNotFound(schema.GroupResource{}, "")),
					c.EXPECT().Get(ctx, client.ObjectKey{Namespace: testNamespace, Name: etcdName}, gomock.AssignableToTypeOf(&druidv1alpha1.Etcd{})),
					c.EXPECT().Patch(ctx, gomock.AssignableToTypeOf(&druidv1alpha1.Etcd{}), gomock.Any()).Do(func(_ context.Context, obj client.Object, _ client.Patch, _ ...client.PatchOption) {
						Expect(obj).To(DeepEqual(etcdObjFor(
							class,
							1,
							nil,
							"",
							"",
							&corev1.ResourceRequirements{
								Requests: corev1.ResourceList{
									corev1.ResourceCPU:    resource.MustParse("300m"),
									corev1.ResourceMemory: resource.MustParse("1.5Gi"),
								},
							},
							nil,
							secretNameCA,
							secretNameClient,
							secretNameServer,
							nil,
							nil,
							false,
							nil)))
					}),
					c.EXPECT().Get(ctx, client.ObjectKey{Namespace: testNamespace, Name: vpaName}, gomock.AssignableToTypeOf(&vpaautoscalingv1.VerticalPodAutoscaler{})).Return(apierrors.NewNotFound(schema.GroupResource{}, "")),
					c.EXPECT().Create(ctx, gomock.AssignableToTypeOf(&vpaautoscalingv1.VerticalPodAutoscaler{}), gomock.Any()).Do(func(_ context.Context, obj client.Object, _ ...client.CreateOption) {
						Expect(obj).To(DeepEqual(expectedVPAFor(class, "", minAllowed)))
					}),
					c.EXPECT().Get(ctx, client.ObjectKey{Namespace: testNamespace, Name: "shoot-etcd-" + testRole}, gomock.AssignableToTypeOf(&monitoringv1.ServiceMonitor{})),
					c.EXPECT().Patch(ctx, gomock.AssignableToTypeOf(&monitoringv1.ServiceMonitor{}), gomock.Any()).Do(func(_ context.Context, obj client.Object, _ client.Patch, _ ...client.PatchOption) {
						Expect(obj).To(DeepEqual(serviceMonitor("shoot", "etcd-client")))
					}),
					c.EXPECT().Get(ctx, client.ObjectKey{Namespace: testNamespace, Name: "shoot-etcd-" + testRole}, gomock.AssignableToTypeOf(&monitoringv1.PrometheusRule{})),
					c.EXPECT().Patch(ctx, gomock.AssignableToTypeOf(&monitoringv1.PrometheusRule{}), gomock.Any()).Do(func(_ context.Context, obj client.Object, _ client.Patch, _ ...client.PatchOption) {
						Expect(obj).To(DeepEqual(prometheusRule("shoot", class, 1, false)))
					}),
				)

				Expect(etcd.Deploy(ctx)).To(Succeed())
			})
		})

		Describe("Prometheus rules", func() {
			It("should successfully run the rule tests", func() {
				componenttest.PrometheusRule(prometheusRule("shoot", ClassNormal, 1, false), "testdata/shoot-etcd-normal-singlenode-without-backup.prometheusrule.test.yaml")
				componenttest.PrometheusRule(prometheusRule("shoot", ClassNormal, 1, true), "testdata/shoot-etcd-normal-singlenode-with-backup.prometheusrule.test.yaml")
				componenttest.PrometheusRule(prometheusRule("shoot", ClassNormal, 3, false), "testdata/shoot-etcd-normal-multinode-without-backup.prometheusrule.test.yaml")
				componenttest.PrometheusRule(prometheusRule("shoot", ClassNormal, 3, true), "testdata/shoot-etcd-normal-multinode-with-backup.prometheusrule.test.yaml")

				componenttest.PrometheusRule(prometheusRule("shoot", ClassImportant, 1, false), "testdata/shoot-etcd-important-singlenode-without-backup.prometheusrule.test.yaml")
				componenttest.PrometheusRule(prometheusRule("shoot", ClassImportant, 1, true), "testdata/shoot-etcd-important-singlenode-with-backup.prometheusrule.test.yaml")
				componenttest.PrometheusRule(prometheusRule("shoot", ClassImportant, 3, false), "testdata/shoot-etcd-important-multinode-without-backup.prometheusrule.test.yaml")
				componenttest.PrometheusRule(prometheusRule("shoot", ClassImportant, 3, true), "testdata/shoot-etcd-important-multinode-with-backup.prometheusrule.test.yaml")
			})
		})
	})

	Describe("#Destroy", func() {
		var (
			etcdRes *druidv1alpha1.Etcd
			nowFunc func() time.Time
		)

		JustBeforeEach(func() {
			etcd = New(log, c, testNamespace, sm, Values{
				Role:                    testRole,
				Class:                   class,
				Replicas:                ptr.To[int32](1),
				StorageCapacity:         storageCapacity,
				StorageClassName:        &storageClassName,
				DefragmentationSchedule: &defragmentationSchedule,
				CARotationPhase:         "",
				PriorityClassName:       priorityClassName,
			})
		})

		BeforeEach(func() {
			nowFunc = func() time.Time {
				return time.Date(1, 1, 1, 1, 1, 1, 1, time.UTC)
			}
			etcdRes = &druidv1alpha1.Etcd{ObjectMeta: metav1.ObjectMeta{
				Name:      "etcd-" + testRole,
				Namespace: testNamespace,
				Annotations: map[string]string{
					"confirmation.gardener.cloud/deletion": "true",
					"gardener.cloud/timestamp":             nowFunc().Format(time.RFC3339Nano),
				},
			}}
		})

		It("should properly delete all expected objects", func() {
			defer test.WithVar(&gardener.TimeNow, nowFunc)()

			gomock.InOrder(
				c.EXPECT().Patch(ctx, etcdRes, gomock.Any()),
				c.EXPECT().Delete(ctx, &vpaautoscalingv1.VerticalPodAutoscaler{ObjectMeta: metav1.ObjectMeta{Name: "etcd-" + testRole, Namespace: testNamespace}}),
				c.EXPECT().Delete(ctx, &monitoringv1.ServiceMonitor{ObjectMeta: metav1.ObjectMeta{Name: "shoot-etcd-" + testRole, Namespace: testNamespace, Labels: map[string]string{"prometheus": "shoot"}}}),
				c.EXPECT().Delete(ctx, &monitoringv1alpha1.ScrapeConfig{ObjectMeta: metav1.ObjectMeta{Name: "shoot-etcd-druid", Namespace: testNamespace, Labels: map[string]string{"prometheus": "shoot"}}}),
				c.EXPECT().Delete(ctx, &monitoringv1.PrometheusRule{ObjectMeta: metav1.ObjectMeta{Name: "shoot-etcd-" + testRole, Namespace: testNamespace, Labels: map[string]string{"prometheus": "shoot"}}}),
				c.EXPECT().Delete(ctx, etcdRes),
			)
			Expect(etcd.Destroy(ctx)).To(Succeed())
		})

		It("should fail when VPA deletion fails", func() {
			defer test.WithVar(&gardener.TimeNow, nowFunc)()

			gomock.InOrder(
				c.EXPECT().Patch(ctx, etcdRes, gomock.Any()),
				c.EXPECT().Delete(ctx, &vpaautoscalingv1.VerticalPodAutoscaler{ObjectMeta: metav1.ObjectMeta{Name: "etcd-" + testRole, Namespace: testNamespace}}).Return(fakeErr),
			)

			Expect(etcd.Destroy(ctx)).To(MatchError(fakeErr))
		})

		It("should fail when the service monitor deletion fails", func() {
			defer test.WithVar(&gardener.TimeNow, nowFunc)()

			gomock.InOrder(
				c.EXPECT().Patch(ctx, etcdRes, gomock.Any()),
				c.EXPECT().Delete(ctx, &vpaautoscalingv1.VerticalPodAutoscaler{ObjectMeta: metav1.ObjectMeta{Name: "etcd-" + testRole, Namespace: testNamespace}}),
				c.EXPECT().Delete(ctx, &monitoringv1.ServiceMonitor{ObjectMeta: metav1.ObjectMeta{Name: "shoot-etcd-" + testRole, Namespace: testNamespace, Labels: map[string]string{"prometheus": "shoot"}}}).Return(fakeErr),
			)

			Expect(etcd.Destroy(ctx)).To(MatchError(fakeErr))
		})

		It("should fail when the scrape config deletion fails", func() {
			defer test.WithVar(&gardener.TimeNow, nowFunc)()

			gomock.InOrder(
				c.EXPECT().Patch(ctx, etcdRes, gomock.Any()),
				c.EXPECT().Delete(ctx, &vpaautoscalingv1.VerticalPodAutoscaler{ObjectMeta: metav1.ObjectMeta{Name: "etcd-" + testRole, Namespace: testNamespace}}),
				c.EXPECT().Delete(ctx, &monitoringv1.ServiceMonitor{ObjectMeta: metav1.ObjectMeta{Name: "shoot-etcd-" + testRole, Namespace: testNamespace, Labels: map[string]string{"prometheus": "shoot"}}}),
				c.EXPECT().Delete(ctx, &monitoringv1alpha1.ScrapeConfig{ObjectMeta: metav1.ObjectMeta{Name: "shoot-etcd-druid", Namespace: testNamespace, Labels: map[string]string{"prometheus": "shoot"}}}).Return(fakeErr),
			)

			Expect(etcd.Destroy(ctx)).To(MatchError(fakeErr))
		})

		It("should fail when the prometheus rule deletion fails", func() {
			defer test.WithVar(&gardener.TimeNow, nowFunc)()

			gomock.InOrder(
				c.EXPECT().Patch(ctx, etcdRes, gomock.Any()),
				c.EXPECT().Delete(ctx, &vpaautoscalingv1.VerticalPodAutoscaler{ObjectMeta: metav1.ObjectMeta{Name: "etcd-" + testRole, Namespace: testNamespace}}),
				c.EXPECT().Delete(ctx, &monitoringv1.ServiceMonitor{ObjectMeta: metav1.ObjectMeta{Name: "shoot-etcd-" + testRole, Namespace: testNamespace, Labels: map[string]string{"prometheus": "shoot"}}}),
				c.EXPECT().Delete(ctx, &monitoringv1alpha1.ScrapeConfig{ObjectMeta: metav1.ObjectMeta{Name: "shoot-etcd-druid", Namespace: testNamespace, Labels: map[string]string{"prometheus": "shoot"}}}),
				c.EXPECT().Delete(ctx, &monitoringv1.PrometheusRule{ObjectMeta: metav1.ObjectMeta{Name: "shoot-etcd-" + testRole, Namespace: testNamespace, Labels: map[string]string{"prometheus": "shoot"}}}).Return(fakeErr),
			)

			Expect(etcd.Destroy(ctx)).To(MatchError(fakeErr))
		})

		It("should fail when the etcd deletion fails", func() {
			defer test.WithVar(&gardener.TimeNow, nowFunc)()

			gomock.InOrder(
				c.EXPECT().Patch(ctx, etcdRes, gomock.Any()),
				c.EXPECT().Delete(ctx, &vpaautoscalingv1.VerticalPodAutoscaler{ObjectMeta: metav1.ObjectMeta{Name: "etcd-" + testRole, Namespace: testNamespace}}),
				c.EXPECT().Delete(ctx, &monitoringv1.ServiceMonitor{ObjectMeta: metav1.ObjectMeta{Name: "shoot-etcd-" + testRole, Namespace: testNamespace, Labels: map[string]string{"prometheus": "shoot"}}}),
				c.EXPECT().Delete(ctx, &monitoringv1alpha1.ScrapeConfig{ObjectMeta: metav1.ObjectMeta{Name: "shoot-etcd-druid", Namespace: testNamespace, Labels: map[string]string{"prometheus": "shoot"}}}),
				c.EXPECT().Delete(ctx, &monitoringv1.PrometheusRule{ObjectMeta: metav1.ObjectMeta{Name: "shoot-etcd-" + testRole, Namespace: testNamespace, Labels: map[string]string{"prometheus": "shoot"}}}),
				c.EXPECT().Delete(ctx, etcdRes).Return(fakeErr),
			)

			Expect(etcd.Destroy(ctx)).To(MatchError(fakeErr))
		})
	})

	Describe("#Snapshot", func() {
		It("should return an error when the backup config is nil", func() {
			Expect(etcd.Snapshot(ctx, nil)).To(MatchError(ContainSubstring("no backup is configured")))
		})

		Context("w/ backup configuration", func() {
			var mockHttpClient *rest.MockHTTPClient

			BeforeEach(func() {
				mockHttpClient = rest.NewMockHTTPClient(ctrl)
				backupConfig = &BackupConfig{}
			})

			It("should successfully execute the full snapshot command", func() {
				url := fmt.Sprintf("https://etcd-%s-client.%s:8080/snapshot/full?final=true", testRole, testNamespace)
				request, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
				Expect(err).ToNot(HaveOccurred())

				mockHttpClient.EXPECT().Do(request)

				Expect(etcd.Snapshot(ctx, mockHttpClient)).To(Succeed())
			})

			It("should return an error when the execution command fails", func() {
				url := fmt.Sprintf("https://etcd-%s-client.%s:8080/snapshot/full?final=true", testRole, testNamespace)
				request, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
				Expect(err).ToNot(HaveOccurred())

				mockHttpClient.EXPECT().Do(request).Return(nil, fakeErr)

				Expect(etcd.Snapshot(ctx, mockHttpClient)).To(MatchError(fakeErr))
			})
		})
	})

	Describe("#Scale", func() {
		var etcdObj *druidv1alpha1.Etcd

		BeforeEach(func() {
			etcdObj = &druidv1alpha1.Etcd{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "etcd-" + testRole,
					Namespace: testNamespace,
				},
			}
		})

		It("should scale ETCD from 0 to 1", func() {
			etcdObj.Spec.Replicas = 0

			nowFunc := func() time.Time {
				return now
			}
			defer test.WithVar(&TimeNow, nowFunc)()

			c.EXPECT().Get(ctx, client.ObjectKeyFromObject(etcdObj), gomock.AssignableToTypeOf(&druidv1alpha1.Etcd{})).DoAndReturn(
				func(_ context.Context, _ client.ObjectKey, etcd *druidv1alpha1.Etcd, _ ...client.GetOption) error {
					*etcd = *etcdObj
					return nil
				},
			)

			c.EXPECT().Patch(ctx, gomock.AssignableToTypeOf(&druidv1alpha1.Etcd{}), gomock.Any()).DoAndReturn(
				func(_ context.Context, etcd *druidv1alpha1.Etcd, patch client.Patch, _ ...client.PatchOption) error {
					data, err := patch.Data(etcd)
					Expect(err).ToNot(HaveOccurred())
					Expect(string(data)).To(Equal(fmt.Sprintf(`{"metadata":{"annotations":{"gardener.cloud/operation":"reconcile","gardener.cloud/timestamp":"%s"}},"spec":{"replicas":1}}`, now.Format(time.RFC3339Nano))))
					return nil
				})

			Expect(etcd.Scale(ctx, 1)).To(Succeed())
		})

		It("should set operation annotation when replica count is unchanged", func() {
			etcdObj.Spec.Replicas = 1

			nowFunc := func() time.Time {
				return now
			}
			defer test.WithVar(&TimeNow, nowFunc)()

			c.EXPECT().Get(ctx, client.ObjectKeyFromObject(etcdObj), gomock.AssignableToTypeOf(&druidv1alpha1.Etcd{})).DoAndReturn(
				func(_ context.Context, _ client.ObjectKey, etcd *druidv1alpha1.Etcd, _ ...client.GetOption) error {
					*etcd = *etcdObj
					return nil
				},
			)

			c.EXPECT().Patch(ctx, gomock.AssignableToTypeOf(&druidv1alpha1.Etcd{}), gomock.Any()).DoAndReturn(
				func(_ context.Context, etcd *druidv1alpha1.Etcd, patch client.Patch, _ ...client.PatchOption) error {
					data, err := patch.Data(etcd)
					Expect(err).ToNot(HaveOccurred())
					Expect(string(data)).To(Equal(fmt.Sprintf(`{"metadata":{"annotations":{"gardener.cloud/operation":"reconcile","gardener.cloud/timestamp":"%s"}}}`, now.Format(time.RFC3339Nano))))
					return nil
				})

			Expect(etcd.Scale(ctx, 1)).To(Succeed())
		})

		It("should fail if GardenerTimestamp is unexpected", func() {
			nowFunc := func() time.Time {
				return now
			}
			defer test.WithVar(&TimeNow, nowFunc)()

			gomock.InOrder(
				c.EXPECT().Get(ctx, client.ObjectKeyFromObject(etcdObj), gomock.AssignableToTypeOf(&druidv1alpha1.Etcd{})).DoAndReturn(
					func(_ context.Context, _ client.ObjectKey, etcd *druidv1alpha1.Etcd, _ ...client.GetOption) error {
						*etcd = *etcdObj
						return nil
					},
				),
				c.EXPECT().Patch(ctx, gomock.AssignableToTypeOf(&druidv1alpha1.Etcd{}), gomock.Any()),
				c.EXPECT().Get(ctx, client.ObjectKeyFromObject(etcdObj), gomock.AssignableToTypeOf(&druidv1alpha1.Etcd{})).DoAndReturn(
					func(_ context.Context, _ client.ObjectKey, etcd *druidv1alpha1.Etcd, _ ...client.GetOption) error {
						etcdObj.Annotations = map[string]string{
							v1beta1constants.GardenerTimestamp: "foo",
						}
						*etcd = *etcdObj
						return nil
					},
				),
			)

			Expect(etcd.Scale(ctx, 1)).To(Succeed())
			Expect(etcd.Scale(ctx, 1)).Should(MatchError(`object's "gardener.cloud/timestamp" annotation is not "0001-01-01T00:00:00Z" but "foo"`))
		})

		It("should fail because operation annotation is set", func() {
			c.EXPECT().Get(ctx, client.ObjectKeyFromObject(etcdObj), gomock.AssignableToTypeOf(&druidv1alpha1.Etcd{})).DoAndReturn(
				func(_ context.Context, _ client.ObjectKey, etcd *druidv1alpha1.Etcd, _ ...client.GetOption) error {
					etcdObj.Annotations = map[string]string{
						v1beta1constants.GardenerOperation: v1beta1constants.GardenerOperationReconcile,
					}
					*etcd = *etcdObj
					return nil
				},
			)

			Expect(etcd.Scale(ctx, 1)).Should(MatchError(`etcd object still has operation annotation set`))
		})
	})

	Describe("#RolloutPeerCA", func() {
		var highAvailability bool

		JustBeforeEach(func() {
			etcd = New(log, c, testNamespace, sm, Values{
				Role:                    testRole,
				Class:                   class,
				Replicas:                replicas,
				StorageCapacity:         storageCapacity,
				StorageClassName:        &storageClassName,
				DefragmentationSchedule: &defragmentationSchedule,
				CARotationPhase:         "",
				PriorityClassName:       priorityClassName,
				HighAvailabilityEnabled: highAvailability,
			})
		})

		Context("when HA control-plane is not requested", func() {
			BeforeEach(func() {
				replicas = ptr.To[int32](1)
			})

			It("should do nothing and succeed without expectations", func() {
				Expect(etcd.RolloutPeerCA(ctx)).To(Succeed())
			})
		})

		Context("when HA control-plane is requested", func() {
			BeforeEach(func() {
				highAvailability = true
			})

			createEtcdObj := func(caName string) *druidv1alpha1.Etcd {
				return &druidv1alpha1.Etcd{
					ObjectMeta: metav1.ObjectMeta{
						Name:       etcdName,
						Namespace:  testNamespace,
						Generation: 1,
					},
					Spec: druidv1alpha1.EtcdSpec{
						Etcd: druidv1alpha1.EtcdConfig{
							PeerUrlTLS: &druidv1alpha1.TLSConfig{
								TLSCASecretRef: druidv1alpha1.SecretReference{
									SecretReference: corev1.SecretReference{
										Name:      caName,
										Namespace: testNamespace,
									},
									DataKey: ptr.To(secretsutils.DataKeyCertificateBundle),
								},
							},
						},
					},
					Status: druidv1alpha1.EtcdStatus{
						ObservedGeneration: ptr.To[int64](1),
						Ready:              ptr.To(true),
						Conditions: []druidv1alpha1.Condition{
							{
								Type:   druidv1alpha1.ConditionTypeAllMembersUpdated,
								Status: druidv1alpha1.ConditionTrue,
							},
						},
					},
				}
			}

			BeforeEach(func() {
				replicas = ptr.To[int32](3)
				DeferCleanup(test.WithVar(&TimeNow, func() time.Time { return now }))
			})

			It("should patch the etcd resource with the new peer CA secret name", func() {
				Expect(fakeClient.Create(ctx, &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: "ca-etcd-peer", Namespace: testNamespace}})).To(Succeed())

				c.EXPECT().Get(ctx, client.ObjectKey{Namespace: testNamespace, Name: etcdName}, gomock.AssignableToTypeOf(&druidv1alpha1.Etcd{})).Return(apierrors.NewNotFound(schema.GroupResource{}, "")).DoAndReturn(func(_ context.Context, _ client.ObjectKey, obj client.Object, _ ...client.GetOption) error {
					createEtcdObj("old-ca").DeepCopyInto(obj.(*druidv1alpha1.Etcd))
					return nil
				})

				c.EXPECT().Patch(ctx, gomock.AssignableToTypeOf(&druidv1alpha1.Etcd{}), gomock.Any()).DoAndReturn(
					func(_ context.Context, obj *druidv1alpha1.Etcd, patch client.Patch, _ ...client.PatchOption) error {
						data, err := patch.Data(obj)
						Expect(err).ToNot(HaveOccurred())
						Expect(data).To(MatchJSON("{\"metadata\":{\"annotations\":{\"gardener.cloud/operation\":\"reconcile\",\"gardener.cloud/timestamp\":\"0001-01-01T00:00:00Z\"}},\"spec\":{\"etcd\":{\"peerUrlTls\":{\"tlsCASecretRef\":{\"name\":\"ca-etcd-peer\"}}}}}"))
						return nil
					})

				c.EXPECT().Get(gomock.Any(), client.ObjectKey{Namespace: testNamespace, Name: etcdName}, gomock.AssignableToTypeOf(&druidv1alpha1.Etcd{})).Return(apierrors.NewNotFound(schema.GroupResource{}, "")).DoAndReturn(func(_ context.Context, _ client.ObjectKey, obj client.Object, _ ...client.GetOption) error {
					createEtcdObj("old-ca").DeepCopyInto(obj.(*druidv1alpha1.Etcd))
					obj.(*druidv1alpha1.Etcd).ObjectMeta.Annotations = map[string]string{"gardener.cloud/timestamp": "0001-01-01T00:00:00Z"}
					return nil
				}).AnyTimes()

				Expect(etcd.RolloutPeerCA(ctx)).To(Succeed())
			})

			It("should only patch reconcile annotation data because the expected CA ref is already configured", func() {
				peerCAName := "ca-etcd-peer"

				Expect(fakeClient.Create(ctx, &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: peerCAName, Namespace: testNamespace}})).To(Succeed())

				c.EXPECT().Get(ctx, client.ObjectKey{Namespace: testNamespace, Name: etcdName}, gomock.AssignableToTypeOf(&druidv1alpha1.Etcd{})).Return(apierrors.NewNotFound(schema.GroupResource{}, "")).DoAndReturn(func(_ context.Context, _ client.ObjectKey, obj client.Object, _ ...client.GetOption) error {
					createEtcdObj(peerCAName).DeepCopyInto(obj.(*druidv1alpha1.Etcd))
					return nil
				})

				c.EXPECT().Patch(ctx, gomock.AssignableToTypeOf(&druidv1alpha1.Etcd{}), gomock.Any()).DoAndReturn(
					func(_ context.Context, obj *druidv1alpha1.Etcd, patch client.Patch, _ ...client.PatchOption) error {
						data, err := patch.Data(obj)
						Expect(err).ToNot(HaveOccurred())
						Expect(data).To(MatchJSON("{\"metadata\":{\"annotations\":{\"gardener.cloud/operation\":\"reconcile\",\"gardener.cloud/timestamp\":\"0001-01-01T00:00:00Z\"}}}"))
						return nil
					})

				c.EXPECT().Get(gomock.Any(), client.ObjectKey{Namespace: testNamespace, Name: etcdName}, gomock.AssignableToTypeOf(&druidv1alpha1.Etcd{})).Return(apierrors.NewNotFound(schema.GroupResource{}, "")).DoAndReturn(func(_ context.Context, _ client.ObjectKey, obj client.Object, _ ...client.GetOption) error {
					createEtcdObj(peerCAName).DeepCopyInto(obj.(*druidv1alpha1.Etcd))
					obj.(*druidv1alpha1.Etcd).ObjectMeta.Annotations = map[string]string{"gardener.cloud/timestamp": "0001-01-01T00:00:00Z"}
					return nil
				}).AnyTimes()

				Expect(etcd.RolloutPeerCA(ctx)).To(Succeed())
			})

			It("should fail because CA cannot be found", func() {
				Expect(etcd.RolloutPeerCA(ctx)).To(MatchError("secret \"ca-etcd-peer\" not found"))
			})
		})
	})
})

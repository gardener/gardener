// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package etcd_test

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"strconv"
	"time"

	"github.com/Masterminds/semver/v3"
	druidcorev1alpha1 "github.com/gardener/etcd-druid/api/core/v1alpha1"
	"github.com/go-logr/logr"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	monitoringv1 "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1"
	monitoringv1alpha1 "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1alpha1"
	"go.uber.org/mock/gomock"
	autoscalingv1 "k8s.io/api/autoscaling/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/intstr"
	vpaautoscalingv1 "k8s.io/autoscaler/vertical-pod-autoscaler/pkg/apis/autoscaling.k8s.io/v1"
	kubernetesscheme "k8s.io/client-go/kubernetes/scheme"
	testclock "k8s.io/utils/clock/testing"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"
	fakeclient "sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/client/interceptor"
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
)

var _ = Describe("Etcd", func() {
	Describe("#ServiceName", func() {
		It("should return the expected service name", func() {
			Expect(constants.ServiceName(testRole)).To(Equal("etcd-" + testRole + "-client"))
		})
	})

	var (
		ctrl *gomock.Controller
		c    client.Client
		sm   secretsmanager.Interface
		etcd Interface
		log  logr.Logger

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
		secretNameServer     string
		secretNameServerPeer string
		secretNameClient     = "etcd-client"

		maintenanceTimeWindow = gardencorev1beta1.MaintenanceTimeWindow{
			Begin: "1234",
			End:   "5678",
		}
		highAvailabilityEnabled bool
		staticPodConfig         *StaticPodConfig
		role                    string
		caRotationPhase         gardencorev1beta1.CredentialsRotationPhase
		autoscalingConfig       AutoscalingConfig
		backupConfig            *BackupConfig
		now                     = time.Time{}
		quota                   = resource.MustParse("8Gi")
		garbageCollectionPolicy = druidcorev1alpha1.GarbageCollectionPolicy(druidcorev1alpha1.GarbageCollectionPolicyExponential)
		garbageCollectionPeriod = metav1.Duration{Duration: 12 * time.Hour}
		compressionPolicy       = druidcorev1alpha1.GzipCompression
		compressionSpec         = druidcorev1alpha1.CompressionSpec{
			Enabled: ptr.To(true),
			Policy:  &compressionPolicy,
		}
		snapshotCompactionSpec = druidcorev1alpha1.SnapshotCompactionSpec{
			Resources: &corev1.ResourceRequirements{
				Requests: corev1.ResourceList{
					corev1.ResourceCPU:    resource.MustParse("600m"),
					corev1.ResourceMemory: resource.MustParse("3Gi"),
				},
			},
		}
		backupLeaderElectionEtcdConnectionTimeout = &metav1.Duration{Duration: 10 * time.Second}
		backupLeaderElectionReelectionPeriod      = &metav1.Duration{Duration: 11 * time.Second}

		metricsBasic     = druidcorev1alpha1.Basic
		metricsExtensive = druidcorev1alpha1.Extensive

		etcdName string
		vpaName  string

		etcdObjFor = func(
			class Class,
			role string,
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
			staticPodConfig *StaticPodConfig,
		) *druidcorev1alpha1.Etcd {
			defragSchedule := defragmentationSchedule
			if existingDefragmentationSchedule != "" {
				defragSchedule = existingDefragmentationSchedule
			}

			resourcesContainerEtcd := &corev1.ResourceRequirements{
				Requests: corev1.ResourceList{
					corev1.ResourceCPU:    resource.MustParse("30m"),
					corev1.ResourceMemory: resource.MustParse("150M"),
				},
			}

			if class == ClassImportant {
				resourcesContainerEtcd = &corev1.ResourceRequirements{
					Requests: corev1.ResourceList{
						corev1.ResourceCPU:    resource.MustParse("150m"),
						corev1.ResourceMemory: resource.MustParse("500M"),
					},
				}
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
				if versionutils.ConstraintK8sGreaterEqual134.Check(runtimeKubernetesVersion) {
					clientService.Spec.TrafficDistribution = ptr.To(corev1.ServiceTrafficDistributionPreferSameZone)
				} else {
					// For Kubernetes < 1.34, use PreferClose
					clientService.Spec.TrafficDistribution = ptr.To(corev1.ServiceTrafficDistributionPreferClose)
				}
			}

			obj := &druidcorev1alpha1.Etcd{
				ObjectMeta: metav1.ObjectMeta{
					Name:      etcdName,
					Namespace: testNamespace,
					Annotations: map[string]string{
						"gardener.cloud/operation": "reconcile",
						"gardener.cloud/timestamp": now.Format(time.RFC3339Nano),
					},
					Labels: map[string]string{
						"gardener.cloud/role": "controlplane",
						"role":                role,
					},
				},
				Spec: druidcorev1alpha1.EtcdSpec{
					Replicas:          replicas,
					PriorityClassName: &priorityClassName,
					Labels: map[string]string{
						"gardener.cloud/role":              "controlplane",
						"role":                             role,
						"app":                              "etcd-statefulset",
						"networking.gardener.cloud/to-dns": "allowed",
						"networking.gardener.cloud/to-public-networks":   "allowed",
						"networking.gardener.cloud/to-private-networks":  "allowed",
						"networking.gardener.cloud/to-runtime-apiserver": "allowed",
					},
					Selector: &metav1.LabelSelector{
						MatchLabels: map[string]string{
							"gardener.cloud/role": "controlplane",
							"role":                role,
							"app":                 "etcd-statefulset",
						},
					},
					Etcd: druidcorev1alpha1.EtcdConfig{
						Resources: resourcesContainerEtcd,
						ClientUrlTLS: &druidcorev1alpha1.TLSConfig{
							TLSCASecretRef: druidcorev1alpha1.SecretReference{
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
						WrapperPort:             ptr.To[int32](9095),
						Metrics:                 &metricsBasic,
						DefragmentationSchedule: &defragSchedule,
						Quota:                   &quota,
						ClientService: &druidcorev1alpha1.ClientService{
							Annotations:         clientService.Annotations,
							Labels:              clientService.Labels,
							TrafficDistribution: clientService.Spec.TrafficDistribution,
						},
					},
					Backup: druidcorev1alpha1.BackupSpec{
						TLS: &druidcorev1alpha1.TLSConfig{
							TLSCASecretRef: druidcorev1alpha1.SecretReference{
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
						SnapshotCompaction:      &snapshotCompactionSpec,
						GarbageCollectionPolicy: &garbageCollectionPolicy,
						GarbageCollectionPeriod: &garbageCollectionPeriod,
						SnapshotCompression:     &compressionSpec,
					},
					StorageCapacity:     &storageCapacityQuantity,
					StorageClass:        &storageClassName,
					VolumeClaimTemplate: ptr.To(etcdName),
				},
			}

			switch class {
			case ClassImportant:
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
				obj.Spec.VolumeClaimTemplate = ptr.To(role + "-etcd")
			case ClassNormal:
				metav1.SetMetaDataAnnotation(&obj.ObjectMeta, "resources.druid.gardener.cloud/allow-unhealthy-pod-eviction", "")
			}

			if replicas == 3 {
				obj.Spec.Labels = utils.MergeStringMaps(obj.Spec.Labels, map[string]string{
					"networking.resources.gardener.cloud/to-etcd-" + role + "-client-tcp-2379": "allowed",
					"networking.resources.gardener.cloud/to-etcd-" + role + "-client-tcp-2380": "allowed",
					"networking.resources.gardener.cloud/to-etcd-" + role + "-client-tcp-8080": "allowed",
				})
				obj.Spec.Etcd.PeerUrlTLS = &druidcorev1alpha1.TLSConfig{
					ServerTLSSecretRef: corev1.SecretReference{
						Name:      secretNameServerPeer,
						Namespace: testNamespace,
					},
					TLSCASecretRef: druidcorev1alpha1.SecretReference{
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
				obj.Spec.Etcd.PeerUrlTLS.TLSCASecretRef = druidcorev1alpha1.SecretReference{
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

				provider := druidcorev1alpha1.StorageProvider(backupConfig.Provider)
				deltaSnapshotPeriod := metav1.Duration{Duration: 5 * time.Minute}
				deltaSnapshotMemoryLimit := resource.MustParse("100Mi")

				obj.Spec.Backup.Store = &druidcorev1alpha1.StoreSpec{
					SecretRef: &corev1.SecretReference{Name: backupConfig.SecretRefName},
					Container: &backupConfig.Container,
					Provider:  &provider,
					Prefix:    backupConfig.Prefix + "/etcd-" + role,
				}
				obj.Spec.Backup.FullSnapshotSchedule = &fullSnapshotSchedule
				obj.Spec.Backup.DeltaSnapshotPeriod = &deltaSnapshotPeriod
				obj.Spec.Backup.DeltaSnapshotRetentionPeriod = &metav1.Duration{Duration: 15 * 24 * time.Hour}
				obj.Spec.Backup.DeltaSnapshotMemoryLimit = &deltaSnapshotMemoryLimit

				if backupConfig.LeaderElection != nil {
					obj.Spec.Backup.LeaderElection = &druidcorev1alpha1.LeaderElectionSpec{
						EtcdConnectionTimeout: backupLeaderElectionEtcdConnectionTimeout,
						ReelectionPeriod:      backupLeaderElectionReelectionPeriod,
					}
				}
			}

			if staticPodConfig != nil {
				obj.Annotations["druid.gardener.cloud/disable-etcd-runtime-component-creation"] = ""
				obj.Spec.RunAsRoot = ptr.To(true)

				if role == "events" {
					obj.Spec.Backup.Port = ptr.To[int32](8081)
					obj.Spec.Etcd.ClientPort = ptr.To[int32](2382)
					obj.Spec.Etcd.ServerPort = ptr.To[int32](2383)
					obj.Spec.Etcd.WrapperPort = ptr.To[int32](9096)
				}
			}

			return obj
		}

		expectedVPAFor = func(class Class, role string, evictionRequirement string, minAllowed corev1.ResourceList) *vpaautoscalingv1.VerticalPodAutoscaler {
			minAllowedConfig := minAllowed
			if minAllowedConfig == nil {
				minAllowedConfig = corev1.ResourceList{
					corev1.ResourceMemory: resource.MustParse("60M"),
				}
			}

			vpa := &vpaautoscalingv1.VerticalPodAutoscaler{
				ObjectMeta: metav1.ObjectMeta{
					Name:      vpaName,
					Namespace: testNamespace,
					Labels:    map[string]string{v1beta1constants.LabelRole: "etcd-vpa-" + role},
				},
				Spec: vpaautoscalingv1.VerticalPodAutoscalerSpec{
					TargetRef: &autoscalingv1.CrossVersionObjectReference{
						APIVersion: druidcorev1alpha1.SchemeGroupVersion.String(),
						Kind:       "Etcd",
						Name:       etcdName,
					},
					UpdatePolicy: &vpaautoscalingv1.PodUpdatePolicy{
						UpdateMode: ptr.To(vpaautoscalingv1.UpdateModeRecreate),
					},
					ResourcePolicy: &vpaautoscalingv1.PodResourcePolicy{
						ContainerPolicies: []vpaautoscalingv1.ContainerResourcePolicy{
							{
								ContainerName:    "etcd",
								MinAllowed:       minAllowedConfig,
								ControlledValues: ptr.To(vpaautoscalingv1.ContainerControlledValuesRequestsOnly),
								Mode:             ptr.To(vpaautoscalingv1.ContainerScalingModeAuto),
							},
							{
								ContainerName: "*",
								Mode:          ptr.To(vpaautoscalingv1.ContainerScalingModeOff),
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
			return "kube-etcd3-" + role, "kube-etcd3-backup-restore-" + role
		}

		serviceMonitorAlertName = func(role string) string {
			alertRole := "Main"
			if role == "events" {
				alertRole = "Events"
			}

			return alertRole
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
						druidcorev1alpha1.LabelAppNameKey: fmt.Sprintf("%s-client", etcdName),
						druidcorev1alpha1.LabelPartOfKey:  etcdName,
					}},
					Endpoints: []monitoringv1.Endpoint{
						{
							Port:   "client",
							Scheme: ptr.To(monitoringv1.SchemeHTTPS),
							HTTPConfigWithProxyAndTLSFiles: monitoringv1.HTTPConfigWithProxyAndTLSFiles{
								HTTPConfigWithTLSFiles: monitoringv1.HTTPConfigWithTLSFiles{
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
								},
							},
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
							Scheme: ptr.To(monitoringv1.SchemeHTTPS),
							HTTPConfigWithProxyAndTLSFiles: monitoringv1.HTTPConfigWithProxyAndTLSFiles{
								HTTPConfigWithTLSFiles: monitoringv1.HTTPConfigWithTLSFiles{
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
								},
							},
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
								Alert: "KubeEtcd" + serviceMonitorAlertName(role) + "Down",
								Expr:  intstr.FromString(`sum(up{job="` + jobNameEtcd + `"}) < ` + strconv.Itoa(int(replicas/2)+1)),
								For:   ptr.To(monitoringv1.Duration(alertFor1)),
								Labels: map[string]string{
									"service":    "etcd",
									"severity":   severity1,
									"type":       "seed",
									"visibility": "operator",
								},
								Annotations: map[string]string{
									"summary":     "Etcd3 " + role + " cluster down.",
									"description": "Etcd3 cluster " + role + " is unavailable (due to possible quorum loss) or cannot be scraped. As long as etcd3 " + role + " is down, the cluster is unreachable.",
								},
							},
							// etcd leader alerts
							{
								Alert: "KubeEtcd3" + serviceMonitorAlertName(role) + "NoLeader",
								Expr:  intstr.FromString(`sum(etcd_server_has_leader{job="` + jobNameEtcd + `"}) < count(etcd_server_has_leader{job="` + jobNameEtcd + `"})`),
								For:   ptr.To(monitoringv1.Duration(alertFor2)),
								Labels: map[string]string{
									"service":    "etcd",
									"severity":   "critical",
									"type":       "seed",
									"visibility": "operator",
								},
								Annotations: map[string]string{
									"summary":     "Etcd3 " + role + " has no leader.",
									"description": "Etcd3 cluster " + role + " has no leader. Possible network partition in the etcd cluster.",
								},
							},
							{
								Alert: "KubeEtcd3" + serviceMonitorAlertName(role) + "HighMemoryConsumption",
								Expr:  intstr.FromString(`sum(container_memory_working_set_bytes{pod="etcd-` + role + `-0",container="etcd"}) / sum(kube_verticalpodautoscaler_spec_resourcepolicy_container_policies_maxallowed{container="etcd", targetName="etcd-` + role + `", resource="memory"}) > .5`),
								For:   ptr.To(monitoringv1.Duration("15m")),
								Labels: map[string]string{
									"service":    "etcd",
									"severity":   "warning",
									"type":       "seed",
									"visibility": "operator",
								},
								Annotations: map[string]string{
									"summary":     "Etcd3 " + role + " is consuming too much memory",
									"description": "Etcd3 " + role + " is consuming over 50% of the max allowed value specified by VPA.",
								},
							},
							{
								Alert: "KubeEtcd3" + serviceMonitorAlertName(role) + "DbSizeLimitApproaching",
								Expr:  intstr.FromString(`(etcd_mvcc_db_total_size_in_bytes{job="` + jobNameEtcd + `"} > bool 7516193000) + (etcd_mvcc_db_total_size_in_bytes{job="` + jobNameEtcd + `"} <= bool 8589935000) == 2`),
								Labels: map[string]string{
									"service":    "etcd",
									"severity":   "warning",
									"type":       "seed",
									"visibility": "all",
								},
								Annotations: map[string]string{
									"summary":     "Etcd3 " + role + " DB size is approaching its current practical limit.",
									"description": "Etcd3 " + role + " DB size is approaching its current practical limit of 8GB. Etcd quota might need to be increased.",
								},
							},
							{
								Alert: "KubeEtcd3" + serviceMonitorAlertName(role) + "DbSizeLimitCrossed",
								Expr:  intstr.FromString(`etcd_mvcc_db_total_size_in_bytes{job="` + jobNameEtcd + `"} > 8589935000`),
								Labels: map[string]string{
									"service":    "etcd",
									"severity":   "critical",
									"type":       "seed",
									"visibility": "all",
								},
								Annotations: map[string]string{
									"summary":     "Etcd3 " + role + " DB size has crossed its current practical limit.",
									"description": "Etcd3 " + role + " DB size has crossed its current practical limit of 8GB. Etcd quota must be increased to allow updates.",
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
		s := runtime.NewScheme()
		Expect(kubernetesscheme.AddToScheme(s)).To(Succeed())
		Expect(druidcorev1alpha1.AddToScheme(s)).To(Succeed())
		Expect(vpaautoscalingv1.AddToScheme(s)).To(Succeed())
		Expect(monitoringv1alpha1.AddToScheme(s)).To(Succeed())
		Expect(monitoringv1.AddToScheme(s)).To(Succeed())
		c = fakeclient.NewClientBuilder().WithScheme(s).Build()
		sm = fakesecretsmanager.New(c, testNamespace)
		autoscalingConfig = AutoscalingConfig{}
		backupConfig = nil
		replicas = ptr.To[int32](1)
		highAvailabilityEnabled = false
		staticPodConfig = nil
		role = testRole

		DeferCleanup(test.WithVar(&TimeNow, func() time.Time { return now }))
	})

	JustBeforeEach(func() {
		ctrl = gomock.NewController(GinkgoT())
		log = logr.Discard()

		secretNameServer = "etcd-server-" + role
		secretNameServerPeer = "etcd-peer-server-" + role
		etcdName = "etcd-" + role
		vpaName = etcdName

		By("Create secrets managed outside of this package for whose secretsmanager.Get() will be called")
		Expect(c.Create(ctx, &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: "ca-etcd", Namespace: testNamespace}})).To(Succeed())
		etcd = New(log, c, testNamespace, sm, Values{
			Role:                    role,
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
			StaticPodConfig:         staticPodConfig,
		})
	})

	AfterEach(func() {
		ctrl.Finish()
	})

	Describe("#Deploy", func() {
		It("should fail because the etcd object retrieval fails", func() {
			c = fakeclient.NewClientBuilder().WithScheme(c.Scheme()).WithInterceptorFuncs(interceptor.Funcs{
				Get: func(ctx context.Context, client client.WithWatch, key client.ObjectKey, obj client.Object, opts ...client.GetOption) error {
					if _, ok := obj.(*druidcorev1alpha1.Etcd); ok {
						return fakeErr
					}
					return client.Get(ctx, key, obj, opts...)
				},
			}).Build()
			sm = fakesecretsmanager.New(c, testNamespace)
			Expect(c.Create(ctx, &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: "ca-etcd", Namespace: testNamespace}})).To(Succeed())
			etcd = New(log, c, testNamespace, sm, Values{
				Role: role, Class: class, Replicas: replicas, Autoscaling: autoscalingConfig,
				StorageCapacity: storageCapacity, StorageClassName: &storageClassName,
				DefragmentationSchedule: &defragmentationSchedule, CARotationPhase: caRotationPhase,
				PriorityClassName: priorityClassName, MaintenanceTimeWindow: maintenanceTimeWindow,
				HighAvailabilityEnabled: highAvailabilityEnabled, BackupConfig: backupConfig, StaticPodConfig: staticPodConfig,
			})
			Expect(etcd.Deploy(ctx)).To(MatchError(fakeErr))
		})

		It("should fail because the etcd cannot be created", func() {
			c = fakeclient.NewClientBuilder().WithScheme(c.Scheme()).WithInterceptorFuncs(interceptor.Funcs{
				Create: func(ctx context.Context, client client.WithWatch, obj client.Object, opts ...client.CreateOption) error {
					if _, ok := obj.(*druidcorev1alpha1.Etcd); ok {
						return fakeErr
					}
					return client.Create(ctx, obj, opts...)
				},
			}).Build()
			sm = fakesecretsmanager.New(c, testNamespace)
			Expect(c.Create(ctx, &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: "ca-etcd", Namespace: testNamespace}})).To(Succeed())
			etcd = New(log, c, testNamespace, sm, Values{
				Role: role, Class: class, Replicas: replicas, Autoscaling: autoscalingConfig,
				StorageCapacity: storageCapacity, StorageClassName: &storageClassName,
				DefragmentationSchedule: &defragmentationSchedule, CARotationPhase: caRotationPhase,
				PriorityClassName: priorityClassName, MaintenanceTimeWindow: maintenanceTimeWindow,
				HighAvailabilityEnabled: highAvailabilityEnabled, BackupConfig: backupConfig, StaticPodConfig: staticPodConfig,
			})
			Expect(etcd.Deploy(ctx)).To(MatchError(fakeErr))
		})

		It("should successfully deploy (normal etcd)", func() {
			Expect(etcd.Deploy(ctx)).To(Succeed())

			etcd := &druidcorev1alpha1.Etcd{}
			Expect(c.Get(ctx, client.ObjectKey{Namespace: testNamespace, Name: etcdName}, etcd)).To(Succeed())
			etcd.ResourceVersion = ""
			Expect(etcd).To(DeepEqual(etcdObjFor(class, role, 1, nil, "", "", nil, nil, secretNameCA, secretNameClient, secretNameServer, nil, nil, false, nil, staticPodConfig)))

			vpa := &vpaautoscalingv1.VerticalPodAutoscaler{}
			Expect(c.Get(ctx, client.ObjectKey{Namespace: testNamespace, Name: vpaName}, vpa)).To(Succeed())
			vpa.ResourceVersion = ""
			Expect(vpa).To(DeepEqual(expectedVPAFor(class, role, "", nil)))

			sm := &monitoringv1.ServiceMonitor{}
			Expect(c.Get(ctx, client.ObjectKey{Namespace: testNamespace, Name: "shoot-etcd-" + testRole}, sm)).To(Succeed())
			sm.ResourceVersion = ""
			Expect(sm).To(DeepEqual(serviceMonitor("shoot", "etcd-client")))

			pr := &monitoringv1.PrometheusRule{}
			Expect(c.Get(ctx, client.ObjectKey{Namespace: testNamespace, Name: "shoot-etcd-" + testRole}, pr)).To(Succeed())
			pr.ResourceVersion = ""
			Expect(pr).To(DeepEqual(prometheusRule("shoot", class, 1, false)))
		})

		It("should not panic during deploy when etcd resource exists, but its status is not yet populated", func() {
			var existingReplicas int32 = 245

			Expect(c.Create(ctx, &druidcorev1alpha1.Etcd{
				ObjectMeta: metav1.ObjectMeta{Name: etcdName, Namespace: testNamespace},
				Spec:       druidcorev1alpha1.EtcdSpec{Replicas: existingReplicas},
			})).To(Succeed())

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

			Expect(etcd.Deploy(ctx)).To(Succeed())

			etcd := &druidcorev1alpha1.Etcd{}
			Expect(c.Get(ctx, client.ObjectKey{Namespace: testNamespace, Name: etcdName}, etcd)).To(Succeed())
			etcd.ResourceVersion = ""
			Expect(etcd).To(DeepEqual(etcdObjFor(class, role, existingReplicas, nil, "", "", nil, nil, secretNameCA, secretNameClient, secretNameServer, nil, nil, false, nil, staticPodConfig)))

			vpa := &vpaautoscalingv1.VerticalPodAutoscaler{}
			Expect(c.Get(ctx, client.ObjectKey{Namespace: testNamespace, Name: vpaName}, vpa)).To(Succeed())
			vpa.ResourceVersion = ""
			Expect(vpa).To(DeepEqual(expectedVPAFor(class, role, "", nil)))

			sm := &monitoringv1.ServiceMonitor{}
			Expect(c.Get(ctx, client.ObjectKey{Namespace: testNamespace, Name: "shoot-etcd-" + testRole}, sm)).To(Succeed())
			sm.ResourceVersion = ""
			Expect(sm).To(DeepEqual(serviceMonitor("shoot", "etcd-client")))

			pr := &monitoringv1.PrometheusRule{}
			Expect(c.Get(ctx, client.ObjectKey{Namespace: testNamespace, Name: "shoot-etcd-" + testRole}, pr)).To(Succeed())
			pr.ResourceVersion = ""
			Expect(pr).To(DeepEqual(prometheusRule("shoot", class, existingReplicas, false)))
		})

		It("should successfully deploy (normal etcd) and retain replicas (etcd found)", func() {
			var existingReplicas int32 = 245

			Expect(c.Create(ctx, &druidcorev1alpha1.Etcd{
				ObjectMeta: metav1.ObjectMeta{Name: etcdName, Namespace: testNamespace},
				Spec:       druidcorev1alpha1.EtcdSpec{Replicas: existingReplicas},
				Status: druidcorev1alpha1.EtcdStatus{
					Etcd: &druidcorev1alpha1.CrossVersionObjectReference{Name: etcdName},
				},
			})).To(Succeed())

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

			Expect(etcd.Deploy(ctx)).To(Succeed())

			etcd := &druidcorev1alpha1.Etcd{}
			Expect(c.Get(ctx, client.ObjectKey{Namespace: testNamespace, Name: etcdName}, etcd)).To(Succeed())
			etcd.ResourceVersion = ""

			expectedEtcd := etcdObjFor(class, role, existingReplicas, nil, "", "", nil, nil, secretNameCA, secretNameClient, secretNameServer, nil, nil, false, nil, staticPodConfig)
			expectedEtcd.Status = druidcorev1alpha1.EtcdStatus{
				Etcd: &druidcorev1alpha1.CrossVersionObjectReference{Name: etcdName},
			}
			Expect(etcd).To(Equal(expectedEtcd))
		})

		It("should successfully deploy (normal etcd) and retain annotations (etcd found)", func() {
			Expect(c.Create(ctx, &druidcorev1alpha1.Etcd{
				ObjectMeta: metav1.ObjectMeta{
					Name:        etcdName,
					Namespace:   testNamespace,
					Annotations: map[string]string{"foo": "bar"},
				},
				Status: druidcorev1alpha1.EtcdStatus{
					Etcd: &druidcorev1alpha1.CrossVersionObjectReference{Name: etcdName},
				},
			})).To(Succeed())

			Expect(etcd.Deploy(ctx)).To(Succeed())

			etcd := &druidcorev1alpha1.Etcd{}
			Expect(c.Get(ctx, client.ObjectKey{Namespace: testNamespace, Name: etcdName}, etcd)).To(Succeed())
			Expect(etcd.Annotations).To(HaveKeyWithValue("foo", "bar"))
		})

		It("should successfully deploy (normal etcd) and keep the existing defragmentation schedule", func() {
			existingDefragmentationSchedule := "foobardefragexisting"

			Expect(c.Create(ctx, &druidcorev1alpha1.Etcd{
				ObjectMeta: metav1.ObjectMeta{Name: etcdName, Namespace: testNamespace},
				Spec: druidcorev1alpha1.EtcdSpec{
					Etcd: druidcorev1alpha1.EtcdConfig{
						DefragmentationSchedule: &existingDefragmentationSchedule,
					},
				},
				Status: druidcorev1alpha1.EtcdStatus{
					Etcd: &druidcorev1alpha1.CrossVersionObjectReference{Name: etcdName},
				},
			})).To(Succeed())

			Expect(etcd.Deploy(ctx)).To(Succeed())

			etcd := &druidcorev1alpha1.Etcd{}
			Expect(c.Get(ctx, client.ObjectKey{Namespace: testNamespace, Name: etcdName}, etcd)).To(Succeed())
			Expect(etcd.Spec.Etcd.DefragmentationSchedule).To(HaveValue(Equal(existingDefragmentationSchedule)))
		})

		It("should successfully deploy (normal etcd) and not keep the existing resource request settings", func() {
			existingResourceRequests := &corev1.ResourceRequirements{
				Requests: corev1.ResourceList{
					corev1.ResourceMemory: resource.MustParse("123Mi"),
				},
			}

			Expect(c.Create(ctx, &druidcorev1alpha1.Etcd{
				ObjectMeta: metav1.ObjectMeta{Name: etcdName, Namespace: testNamespace},
				Spec: druidcorev1alpha1.EtcdSpec{
					Etcd: druidcorev1alpha1.EtcdConfig{
						Resources: existingResourceRequests,
					},
				},
				Status: druidcorev1alpha1.EtcdStatus{
					Etcd: &druidcorev1alpha1.CrossVersionObjectReference{Name: etcdName},
				},
			})).To(Succeed())

			Expect(etcd.Deploy(ctx)).To(Succeed())

			etcd := &druidcorev1alpha1.Etcd{}
			Expect(c.Get(ctx, client.ObjectKey{Namespace: testNamespace, Name: etcdName}, etcd)).To(Succeed())
			Expect(etcd.Spec.Etcd.Resources).NotTo(Equal(existingResourceRequests))
		})

		for _, shootPurpose := range []gardencorev1beta1.ShootPurpose{gardencorev1beta1.ShootPurposeEvaluation, gardencorev1beta1.ShootPurposeProduction} {
			var purpose = shootPurpose
			It(fmt.Sprintf("should successfully deploy (important etcd): purpose = %q", purpose), func() {

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

				Expect(etcd.Deploy(ctx)).To(Succeed())

				etcd := &druidcorev1alpha1.Etcd{}
				Expect(c.Get(ctx, client.ObjectKey{Namespace: testNamespace, Name: etcdName}, etcd)).To(Succeed())
				etcd.ResourceVersion = ""
				Expect(etcd).To(DeepEqual(etcdObjFor(class, role, 1, nil, "", "", nil, nil, secretNameCA, secretNameClient, secretNameServer, nil, nil, false, nil, staticPodConfig)))

				vpa := &vpaautoscalingv1.VerticalPodAutoscaler{}
				Expect(c.Get(ctx, client.ObjectKey{Namespace: testNamespace, Name: vpaName}, vpa)).To(Succeed())
				vpa.ResourceVersion = ""
				Expect(vpa).To(DeepEqual(expectedVPAFor(class, role, evictionRequirement, nil)))

				sm := &monitoringv1.ServiceMonitor{}
				Expect(c.Get(ctx, client.ObjectKey{Namespace: testNamespace, Name: "shoot-etcd-" + testRole}, sm)).To(Succeed())
				sm.ResourceVersion = ""
				Expect(sm).To(DeepEqual(serviceMonitor("shoot", "etcd-client")))

				pr := &monitoringv1.PrometheusRule{}
				Expect(c.Get(ctx, client.ObjectKey{Namespace: testNamespace, Name: "shoot-etcd-" + testRole}, pr)).To(Succeed())
				pr.ResourceVersion = ""
				Expect(pr).To(DeepEqual(prometheusRule("shoot", class, 1, false)))
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
				Expect(etcd.Deploy(ctx)).To(Succeed())

				etcd := &druidcorev1alpha1.Etcd{}
				Expect(c.Get(ctx, client.ObjectKey{Namespace: testNamespace, Name: etcdName}, etcd)).To(Succeed())
				etcd.ResourceVersion = ""
				Expect(etcd).To(Equal(etcdObjFor(class, role, 1, backupConfig, "", "", nil, nil, secretNameCA, secretNameClient, secretNameServer, nil, nil, false, nil, staticPodConfig)))

				vpa := &vpaautoscalingv1.VerticalPodAutoscaler{}
				Expect(c.Get(ctx, client.ObjectKey{Namespace: testNamespace, Name: vpaName}, vpa)).To(Succeed())
				vpa.ResourceVersion = ""
				Expect(vpa).To(DeepEqual(expectedVPAFor(class, role, "", nil)))

				sm := &monitoringv1.ServiceMonitor{}
				Expect(c.Get(ctx, client.ObjectKey{Namespace: testNamespace, Name: "shoot-etcd-" + testRole}, sm)).To(Succeed())
				sm.ResourceVersion = ""
				Expect(sm).To(DeepEqual(serviceMonitor("shoot", "etcd-client")))

				pr := &monitoringv1.PrometheusRule{}
				Expect(c.Get(ctx, client.ObjectKey{Namespace: testNamespace, Name: "shoot-etcd-" + testRole}, pr)).To(Succeed())
				pr.ResourceVersion = ""
				Expect(pr).To(DeepEqual(prometheusRule("shoot", class, *replicas, true)))
			})

			It("should successfully deploy (with backup) and keep the existing backup schedule", func() {
				existingBackupSchedule := "foobarbackupexisting"

				Expect(c.Create(ctx, &druidcorev1alpha1.Etcd{
					ObjectMeta: metav1.ObjectMeta{Name: etcdName, Namespace: testNamespace},
					Spec: druidcorev1alpha1.EtcdSpec{
						Backup: druidcorev1alpha1.BackupSpec{FullSnapshotSchedule: &existingBackupSchedule},
					},
					Status: druidcorev1alpha1.EtcdStatus{Etcd: &druidcorev1alpha1.CrossVersionObjectReference{Name: ""}},
				})).To(Succeed())

				Expect(etcd.Deploy(ctx)).To(Succeed())

				reconciledEtcd := &druidcorev1alpha1.Etcd{}
				Expect(c.Get(ctx, client.ObjectKey{Namespace: testNamespace, Name: etcdName}, reconciledEtcd)).To(Succeed())
				Expect(reconciledEtcd.Spec.Backup.FullSnapshotSchedule).To(HaveValue(Equal(existingBackupSchedule)))
			})
		})

		When("etcd should run as static pod", func() {
			BeforeEach(func() {
				staticPodConfig = &StaticPodConfig{}
			})

			Describe("main etcd", func() {
				It("should successfully deploy the etcd", func() {
					Expect(etcd.Deploy(ctx)).To(Succeed())

					etcd := &druidcorev1alpha1.Etcd{}
					Expect(c.Get(ctx, client.ObjectKey{Namespace: testNamespace, Name: etcdName}, etcd)).To(Succeed())
					etcd.ResourceVersion = ""
					Expect(etcd).To(DeepEqual(etcdObjFor(class, role, 1, backupConfig, "", "", nil, nil, secretNameCA, secretNameClient, secretNameServer, nil, nil, false, nil, staticPodConfig)))

					vpa := &vpaautoscalingv1.VerticalPodAutoscaler{}
					Expect(c.Get(ctx, client.ObjectKey{Namespace: testNamespace, Name: vpaName}, vpa)).To(Succeed())
					vpa.ResourceVersion = ""
					Expect(vpa).To(DeepEqual(expectedVPAFor(class, role, "", nil)))

					sm := &monitoringv1.ServiceMonitor{}
					Expect(c.Get(ctx, client.ObjectKey{Namespace: testNamespace, Name: "shoot-etcd-" + testRole}, sm)).To(Succeed())
					sm.ResourceVersion = ""
					Expect(sm).To(DeepEqual(serviceMonitor("shoot", "etcd-client")))

					pr := &monitoringv1.PrometheusRule{}
					Expect(c.Get(ctx, client.ObjectKey{Namespace: testNamespace, Name: "shoot-etcd-" + testRole}, pr)).To(Succeed())
					pr.ResourceVersion = ""
					Expect(pr).To(DeepEqual(prometheusRule("shoot", class, *replicas, false)))
				})
			})

			Describe("events etcd", func() {
				BeforeEach(func() {
					class = ClassNormal
					role = "events"

					DeferCleanup(test.WithVars(&vpaName, "etcd-"+role))
				})

				It("should successfully deploy the etcd", func() {
					Expect(etcd.Deploy(ctx)).To(Succeed())

					etcd := &druidcorev1alpha1.Etcd{}
					Expect(c.Get(ctx, client.ObjectKey{Namespace: testNamespace, Name: etcdName}, etcd)).To(Succeed())
					etcd.ResourceVersion = ""
					Expect(etcd).To(DeepEqual(etcdObjFor(class, role, 1, backupConfig, "", "", nil, nil, secretNameCA, secretNameClient, secretNameServer, nil, nil, false, nil, staticPodConfig)))

					vpa := &vpaautoscalingv1.VerticalPodAutoscaler{}
					Expect(c.Get(ctx, client.ObjectKey{Namespace: testNamespace, Name: vpaName}, vpa)).To(Succeed())
					vpa.ResourceVersion = ""
					Expect(vpa).To(DeepEqual(expectedVPAFor(class, role, "", nil)))

					sm := &monitoringv1.ServiceMonitor{}
					Expect(c.Get(ctx, client.ObjectKey{Namespace: testNamespace, Name: "shoot-etcd-" + role}, sm)).To(Succeed())
					sm.ResourceVersion = ""
					Expect(sm).To(DeepEqual(serviceMonitor("shoot", "etcd-client")))

					pr := &monitoringv1.PrometheusRule{}
					Expect(c.Get(ctx, client.ObjectKey{Namespace: testNamespace, Name: "shoot-etcd-" + role}, pr)).To(Succeed())
					pr.ResourceVersion = ""
					Expect(pr).To(DeepEqual(prometheusRule("shoot", class, *replicas, false)))
				})
			})
		})

		When("HA setup is configured", func() {
			var (
				clientCASecret *corev1.Secret
				peerCASecret   *corev1.Secret
			)

			BeforeEach(func() {
				highAvailabilityEnabled = true
				replicas = ptr.To[int32](3)
				Expect(c.Create(ctx, &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: "ca-etcd-peer", Namespace: testNamespace}})).To(Succeed())
			})

			createExpectations := func(caSecretName, clientSecretName, serverSecretName, peerCASecretName, peerServerSecretName string) {
				GinkgoHelper()

				etcd := &druidcorev1alpha1.Etcd{}
				Expect(c.Get(ctx, client.ObjectKey{Namespace: testNamespace, Name: etcdName}, etcd)).To(Succeed())
				etcd.ResourceVersion = ""
				Expect(etcd).To(DeepEqual(etcdObjFor(class, role, 3, nil, "", "", nil, nil, caSecretName, clientSecretName, serverSecretName, &peerCASecretName, &peerServerSecretName, false, nil, staticPodConfig)))

				vpa := &vpaautoscalingv1.VerticalPodAutoscaler{}
				Expect(c.Get(ctx, client.ObjectKey{Namespace: testNamespace, Name: vpaName}, vpa)).To(Succeed())
				vpa.ResourceVersion = ""
				Expect(vpa).To(DeepEqual(expectedVPAFor(class, role, "", nil)))

				sm := &monitoringv1.ServiceMonitor{}
				Expect(c.Get(ctx, client.ObjectKey{Namespace: testNamespace, Name: "shoot-etcd-" + testRole}, sm)).To(Succeed())
				sm.ResourceVersion = ""
				Expect(sm).To(DeepEqual(serviceMonitor("shoot", clientSecretName)))

				pr := &monitoringv1.PrometheusRule{}
				Expect(c.Get(ctx, client.ObjectKey{Namespace: testNamespace, Name: "shoot-etcd-" + testRole}, pr)).To(Succeed())
				pr.ResourceVersion = ""
				Expect(pr).To(DeepEqual(prometheusRule("shoot", class, *replicas, false)))
			}

			Context("when CA rotation phase is in `Preparing` state", func() {
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
						c,
						"",
						secretsmanager.WithSecretNamesToTimes(secretNamesToTimes),
						secretsmanager.WithNamespaces(testNamespace),
					)
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
						IPAddresses:                 []net.IP{net.ParseIP("127.0.0.1"), net.ParseIP("::1")},
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
						IPAddresses:                 []net.IP{net.ParseIP("127.0.0.1"), net.ParseIP("::1")},
						CertType:                    secretsutils.ServerClientCert,
						SkipPublishingCACertificate: true,
					}, secretsmanager.SignedByCA(v1beta1constants.SecretNameCAETCD), secretsmanager.Rotate(secretsmanager.InPlace))
					Expect(err).ToNot(HaveOccurred())

					Expect(etcd.Deploy(ctx)).To(Succeed())

					createExpectations(clientCASecret.Name, clientSecret.Name, serverSecret.Name, peerCASecret.Name, peerServerSecret.Name)
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
					c,
					"",
					secretsmanager.WithSecretNamesToTimes(secretNamesToTimes),
					secretsmanager.WithNamespaces(testNamespace),
				)
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

					Expect(c.Create(ctx, &druidcorev1alpha1.Etcd{
						ObjectMeta: metav1.ObjectMeta{Name: etcdName, Namespace: testNamespace},
						Spec: druidcorev1alpha1.EtcdSpec{
							Replicas: 3,
							Etcd: druidcorev1alpha1.EtcdConfig{
								PeerUrlTLS: &druidcorev1alpha1.TLSConfig{
									ServerTLSSecretRef: corev1.SecretReference{Name: "peerServerSecretName", Namespace: testNamespace},
								},
							},
						},
					})).To(Succeed())

					Expect(etcd.Deploy(ctx)).To(Succeed())

					reconciledEtcd := &druidcorev1alpha1.Etcd{}
					Expect(c.Get(ctx, client.ObjectKey{Namespace: testNamespace, Name: etcdName}, reconciledEtcd)).To(Succeed())
					Expect(reconciledEtcd.Spec.Etcd.PeerUrlTLS).NotTo(BeNil())
					Expect(reconciledEtcd.Spec.Etcd.PeerUrlTLS.ServerTLSSecretRef.Name).NotTo(BeEmpty())
					Expect(reconciledEtcd.Spec.Replicas).To(Equal(int32(0)))

					clientSecretName = reconciledEtcd.Spec.Etcd.ClientUrlTLS.ClientTLSSecretRef.Name
					vpa := &vpaautoscalingv1.VerticalPodAutoscaler{}
					Expect(c.Get(ctx, client.ObjectKey{Namespace: testNamespace, Name: vpaName}, vpa)).To(Succeed())
					vpa.ResourceVersion = ""
					Expect(vpa).To(DeepEqual(expectedVPAFor(class, role, "", nil)))

					sm := &monitoringv1.ServiceMonitor{}
					Expect(c.Get(ctx, client.ObjectKey{Namespace: testNamespace, Name: "shoot-etcd-" + testRole}, sm)).To(Succeed())
					sm.ResourceVersion = ""
					Expect(sm).To(DeepEqual(serviceMonitor("shoot", clientSecretName)))

					pr := &monitoringv1.PrometheusRule{}
					Expect(c.Get(ctx, client.ObjectKey{Namespace: testNamespace, Name: "shoot-etcd-" + testRole}, pr)).To(Succeed())
					pr.ResourceVersion = ""
					Expect(pr).To(DeepEqual(prometheusRule("shoot", class, *replicas, false)))
				})
			})

			Context("when peer url secrets are not present in etcd CR", func() {
				BeforeEach(func() {
					highAvailabilityEnabled = true
				})

				It("should add peer url secrets", func() {
					Expect(c.Create(ctx, &druidcorev1alpha1.Etcd{
						ObjectMeta: metav1.ObjectMeta{Name: etcdName, Namespace: testNamespace},
						Spec: druidcorev1alpha1.EtcdSpec{
							Replicas: 3,
							Etcd:     druidcorev1alpha1.EtcdConfig{PeerUrlTLS: nil},
						},
					})).To(Succeed())

					Expect(etcd.Deploy(ctx)).To(Succeed())

					reconciledEtcd := &druidcorev1alpha1.Etcd{}
					Expect(c.Get(ctx, client.ObjectKey{Namespace: testNamespace, Name: etcdName}, reconciledEtcd)).To(Succeed())
					Expect(reconciledEtcd.Spec.Etcd.PeerUrlTLS).NotTo(BeNil())
					Expect(reconciledEtcd.Spec.Replicas).To(Equal(int32(0)))
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

					Expect(etcd.Deploy(ctx)).To(Succeed())

					etcd := &druidcorev1alpha1.Etcd{}
					Expect(c.Get(ctx, client.ObjectKey{Namespace: testNamespace, Name: etcdName}, etcd)).To(Succeed())
					etcd.ResourceVersion = ""
					Expect(etcd).To(DeepEqual(etcdObjFor(class, role, 1, nil, "", "", nil, nil, secretNameCA, secretNameClient, secretNameServer, nil, nil, true, runtimeKubernetesVersion, staticPodConfig)))
				},

				Entry("when runtime Kubernetes version is >= 1.34", semver.MustParse("1.34.0")),
				Entry("when runtime Kubernetes version is >= 1.32, < 1.34", semver.MustParse("1.32.1")),
				Entry("when runtime Kubernetes version is 1.31", semver.MustParse("1.31.2")),
			)
		})

		When("name prefix is set", func() {
			It("should successfully deploy the service monitor", func() {
				var (
					class    = ClassImportant
					replicas = ptr.To[int32](1)
				)

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

				Expect(etcd.Deploy(ctx)).To(Succeed())

				expected := etcdObjFor(class, role, 1, nil, "", "", nil, nil, secretNameCA, secretNameClient, secretNameServer, nil, nil, false, nil, staticPodConfig)
				expected.Name = etcdName
				expected.Spec.VolumeClaimTemplate = ptr.To(testRole + "-virtual-garden-etcd")
				delete(expected.Spec.Etcd.ClientService.Annotations, "networking.resources.gardener.cloud/from-all-scrape-targets-allowed-ports")
				expected.Spec.Etcd.ClientService.Annotations["networking.resources.gardener.cloud/from-all-garden-scrape-targets-allowed-ports"] = `[{"protocol":"TCP","port":2379},{"protocol":"TCP","port":8080}]`
				delete(expected.Spec.Etcd.ClientService.Annotations, "networking.resources.gardener.cloud/namespace-selectors")
				delete(expected.Spec.Etcd.ClientService.Annotations, "networking.resources.gardener.cloud/pod-label-selector-namespace-alias")

				etcd := &druidcorev1alpha1.Etcd{}
				Expect(c.Get(ctx, client.ObjectKey{Namespace: testNamespace, Name: etcdName}, etcd)).To(Succeed())
				etcd.ResourceVersion = ""
				Expect(etcd).To(DeepEqual(expected))

				vpa := &vpaautoscalingv1.VerticalPodAutoscaler{}
				Expect(c.Get(ctx, client.ObjectKey{Namespace: testNamespace, Name: vpaName}, vpa)).To(Succeed())
				vpa.ResourceVersion = ""
				Expect(vpa).To(DeepEqual(expectedVPAFor(class, role, "", nil)))

				sm := &monitoringv1.ServiceMonitor{}
				Expect(c.Get(ctx, client.ObjectKey{Namespace: testNamespace, Name: "garden-virtual-garden-etcd-" + testRole}, sm)).To(Succeed())
				sm.ResourceVersion = ""
				Expect(sm).To(DeepEqual(serviceMonitor("garden", "etcd-client")))
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
				Expect(etcd.Deploy(ctx)).To(Succeed())

				vpa := &vpaautoscalingv1.VerticalPodAutoscaler{}
				Expect(c.Get(ctx, client.ObjectKey{Namespace: testNamespace, Name: vpaName}, vpa)).To(Succeed())
				vpa.ResourceVersion = ""
				Expect(vpa).To(DeepEqual(expectedVPAFor(class, role, "", minAllowed)))
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
		var nowFunc func() time.Time

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
		})

		It("should properly delete all expected objects", func() {
			defer test.WithVar(&gardener.TimeNow, nowFunc)()

			etcdObj := &druidcorev1alpha1.Etcd{ObjectMeta: metav1.ObjectMeta{Name: "etcd-" + testRole, Namespace: testNamespace}}
			vpaObj := &vpaautoscalingv1.VerticalPodAutoscaler{ObjectMeta: metav1.ObjectMeta{Name: "etcd-" + testRole, Namespace: testNamespace}}
			smObj := &monitoringv1.ServiceMonitor{ObjectMeta: metav1.ObjectMeta{Name: "shoot-etcd-" + testRole, Namespace: testNamespace}}
			scObj := &monitoringv1alpha1.ScrapeConfig{ObjectMeta: metav1.ObjectMeta{Name: "shoot-etcd-druid", Namespace: testNamespace}}
			prObj := &monitoringv1.PrometheusRule{ObjectMeta: metav1.ObjectMeta{Name: "shoot-etcd-" + testRole, Namespace: testNamespace}}
			Expect(c.Create(ctx, etcdObj)).To(Succeed())
			Expect(c.Create(ctx, vpaObj)).To(Succeed())
			Expect(c.Create(ctx, smObj)).To(Succeed())
			Expect(c.Create(ctx, scObj)).To(Succeed())
			Expect(c.Create(ctx, prObj)).To(Succeed())

			Expect(etcd.Destroy(ctx)).To(Succeed())

			Expect(c.Get(ctx, client.ObjectKeyFromObject(etcdObj), &druidcorev1alpha1.Etcd{})).To(BeNotFoundError())
			Expect(c.Get(ctx, client.ObjectKeyFromObject(vpaObj), &vpaautoscalingv1.VerticalPodAutoscaler{})).To(BeNotFoundError())
			Expect(c.Get(ctx, client.ObjectKeyFromObject(smObj), &monitoringv1.ServiceMonitor{})).To(BeNotFoundError())
			Expect(c.Get(ctx, client.ObjectKeyFromObject(scObj), &monitoringv1alpha1.ScrapeConfig{})).To(BeNotFoundError())
			Expect(c.Get(ctx, client.ObjectKeyFromObject(prObj), &monitoringv1.PrometheusRule{})).To(BeNotFoundError())
		})

		It("should fail if a resource deletion fails", func() {
			defer test.WithVar(&gardener.TimeNow, nowFunc)()

			c = fakeclient.NewClientBuilder().WithScheme(c.Scheme()).WithInterceptorFuncs(interceptor.Funcs{
				Delete: func(ctx context.Context, cl client.WithWatch, obj client.Object, opts ...client.DeleteOption) error {
					if _, ok := obj.(*druidcorev1alpha1.Etcd); ok {
						return fakeErr
					}
					return cl.Delete(ctx, obj, opts...)
				},
			}).Build()
			Expect(c.Create(ctx, &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: "ca-etcd", Namespace: testNamespace}})).To(Succeed())
			etcd = New(log, c, testNamespace, sm, Values{
				Role: testRole, Class: class, Replicas: ptr.To[int32](1),
				StorageCapacity: storageCapacity, StorageClassName: &storageClassName,
				DefragmentationSchedule: &defragmentationSchedule, PriorityClassName: priorityClassName,
			})
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
		var (
			etcdObj *druidcorev1alpha1.Etcd
			etcd    Interface
		)

		BeforeEach(func() {
			etcdObj = &druidcorev1alpha1.Etcd{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "etcd-" + testRole,
					Namespace: testNamespace,
				},
			}

			etcd = New(log, c, testNamespace, sm, Values{
				Role: role, Class: class, Replicas: replicas, PriorityClassName: priorityClassName,
				StorageCapacity: storageCapacity, StorageClassName: &storageClassName,
				DefragmentationSchedule: &defragmentationSchedule, MaintenanceTimeWindow: maintenanceTimeWindow,
			})

		})

		It("should scale ETCD from 0 to 1", func() {
			etcdObj.Spec.Replicas = 0

			Expect(c.Create(ctx, etcdObj)).To(Succeed())

			Expect(etcd.Scale(ctx, 1)).To(Succeed())

			etcd := &druidcorev1alpha1.Etcd{}
			Expect(c.Get(ctx, client.ObjectKeyFromObject(etcdObj), etcd)).To(Succeed())
			Expect(etcd.Spec.Replicas).To(Equal(int32(1)))
		})

		It("should set operation annotation when replica count is unchanged", func() {
			etcdObj.Spec.Replicas = 1
			Expect(c.Create(ctx, etcdObj)).To(Succeed())

			Expect(etcd.Scale(ctx, 1)).To(Succeed())

			etcd := &druidcorev1alpha1.Etcd{}
			Expect(c.Get(ctx, client.ObjectKeyFromObject(etcdObj), etcd)).To(Succeed())
			Expect(etcd.Annotations).To(HaveKeyWithValue(v1beta1constants.GardenerOperation, v1beta1constants.GardenerOperationReconcile))
		})

		It("should fail if GardenerTimestamp is unexpected", func() {
			Expect(c.Create(ctx, etcdObj)).To(Succeed())
			Expect(etcd.Scale(ctx, 1)).To(Succeed())

			Expect(c.Get(ctx, client.ObjectKeyFromObject(etcdObj), etcdObj)).To(Succeed())
			metav1.SetMetaDataAnnotation(&etcdObj.ObjectMeta, v1beta1constants.GardenerTimestamp, "foo")
			Expect(c.Update(ctx, etcdObj)).To(Succeed())

			Expect(etcd.Scale(ctx, 1)).Should(MatchError(`object's "gardener.cloud/timestamp" annotation is not "0001-01-01T00:00:00Z" but "foo"`))
		})

		It("should fail because operation annotation is set", func() {
			etcdObj.Annotations = map[string]string{
				v1beta1constants.GardenerOperation: v1beta1constants.GardenerOperationReconcile,
			}
			Expect(c.Create(ctx, etcdObj)).To(Succeed())

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

			createEtcdObj := func(caName string) *druidcorev1alpha1.Etcd {
				return &druidcorev1alpha1.Etcd{
					ObjectMeta: metav1.ObjectMeta{
						Name:       etcdName,
						Namespace:  testNamespace,
						Generation: 1,
					},
					Spec: druidcorev1alpha1.EtcdSpec{
						Etcd: druidcorev1alpha1.EtcdConfig{
							PeerUrlTLS: &druidcorev1alpha1.TLSConfig{
								TLSCASecretRef: druidcorev1alpha1.SecretReference{
									SecretReference: corev1.SecretReference{
										Name:      caName,
										Namespace: testNamespace,
									},
									DataKey: ptr.To(secretsutils.DataKeyCertificateBundle),
								},
							},
						},
					},
					Status: druidcorev1alpha1.EtcdStatus{
						ObservedGeneration: ptr.To[int64](1),
						Ready:              ptr.To(true),
						Conditions: []druidcorev1alpha1.Condition{
							{
								Type:   druidcorev1alpha1.ConditionTypeAllMembersUpdated,
								Status: druidcorev1alpha1.ConditionTrue,
							},
						},
					},
				}
			}

			BeforeEach(func() {
				replicas = ptr.To[int32](3)
			})

			It("should patch the etcd resource with the new peer CA secret name", func() {
				Expect(c.Create(ctx, &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: "ca-etcd-peer", Namespace: testNamespace}})).To(Succeed())

				existingEtcd := createEtcdObj("old-ca")
				Expect(c.Create(ctx, existingEtcd)).To(Succeed())

				var patchData []byte
				callCount := 0
				c = fakeclient.NewClientBuilder().WithScheme(c.Scheme()).WithObjects(existingEtcd).WithStatusSubresource(&druidcorev1alpha1.Etcd{}).WithInterceptorFuncs(interceptor.Funcs{
					Patch: func(ctx context.Context, cl client.WithWatch, obj client.Object, patch client.Patch, opts ...client.PatchOption) error {
						if e, ok := obj.(*druidcorev1alpha1.Etcd); ok {
							var err error
							patchData, err = patch.Data(e)
							Expect(err).ToNot(HaveOccurred())
						}
						return cl.Patch(ctx, obj, patch, opts...)
					},
					Get: func(ctx context.Context, cl client.WithWatch, key client.ObjectKey, obj client.Object, opts ...client.GetOption) error {
						if err := cl.Get(ctx, key, obj, opts...); err != nil {
							return err
						}
						// After patch, simulate druid picking up operation annotation
						if e, ok := obj.(*druidcorev1alpha1.Etcd); ok {
							callCount++
							if callCount > 1 {
								delete(e.Annotations, v1beta1constants.GardenerOperation)
							}
						}
						return nil
					},
				}).Build()
				Expect(c.Create(ctx, &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: "ca-etcd", Namespace: testNamespace}})).To(Succeed())

				etcd = New(log, c, testNamespace, sm, Values{
					Role: role, Class: class, Replicas: replicas, PriorityClassName: priorityClassName,
					StorageCapacity: storageCapacity, StorageClassName: &storageClassName,
					DefragmentationSchedule: &defragmentationSchedule, HighAvailabilityEnabled: highAvailability,
				})

				Expect(etcd.RolloutPeerCA(ctx)).To(Succeed())

				Expect(patchData).To(MatchJSON("{\"metadata\":{\"annotations\":{\"gardener.cloud/operation\":\"reconcile\",\"gardener.cloud/timestamp\":\"0001-01-01T00:00:00Z\"}},\"spec\":{\"etcd\":{\"peerUrlTls\":{\"tlsCASecretRef\":{\"name\":\"ca-etcd-peer\"}}}}}"))
			})

			It("should only patch reconcile annotation data because the expected CA ref is already configured", func() {
				peerCAName := "ca-etcd-peer"

				Expect(c.Create(ctx, &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: peerCAName, Namespace: testNamespace}})).To(Succeed())

				existingEtcd := createEtcdObj(peerCAName)
				Expect(c.Create(ctx, existingEtcd)).To(Succeed())

				var patchData []byte
				callCount := 0
				c = fakeclient.NewClientBuilder().WithScheme(c.Scheme()).WithObjects(existingEtcd).WithStatusSubresource(&druidcorev1alpha1.Etcd{}).WithInterceptorFuncs(interceptor.Funcs{
					Patch: func(ctx context.Context, cl client.WithWatch, obj client.Object, patch client.Patch, opts ...client.PatchOption) error {
						if e, ok := obj.(*druidcorev1alpha1.Etcd); ok {
							var err error
							patchData, err = patch.Data(e)
							Expect(err).ToNot(HaveOccurred())
						}
						return cl.Patch(ctx, obj, patch, opts...)
					},
					Get: func(ctx context.Context, cl client.WithWatch, key client.ObjectKey, obj client.Object, opts ...client.GetOption) error {
						if err := cl.Get(ctx, key, obj, opts...); err != nil {
							return err
						}
						if e, ok := obj.(*druidcorev1alpha1.Etcd); ok {
							callCount++
							if callCount > 1 {
								delete(e.Annotations, v1beta1constants.GardenerOperation)
							}
						}
						return nil
					},
				}).Build()
				Expect(c.Create(ctx, &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: "ca-etcd", Namespace: testNamespace}})).To(Succeed())
				Expect(c.Create(ctx, &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: peerCAName, Namespace: testNamespace}})).To(Succeed())

				etcd = New(log, c, testNamespace, sm, Values{
					Role: role, Class: class, Replicas: replicas, PriorityClassName: priorityClassName,
					StorageCapacity: storageCapacity, StorageClassName: &storageClassName,
					DefragmentationSchedule: &defragmentationSchedule, HighAvailabilityEnabled: highAvailability,
				})

				Expect(etcd.RolloutPeerCA(ctx)).To(Succeed())

				Expect(patchData).To(MatchJSON("{\"metadata\":{\"annotations\":{\"gardener.cloud/operation\":\"reconcile\",\"gardener.cloud/timestamp\":\"0001-01-01T00:00:00Z\"}}}"))
			})

			It("should fail because CA cannot be found", func() {
				Expect(etcd.RolloutPeerCA(ctx)).To(MatchError("secret \"ca-etcd-peer\" not found"))
			})
		})
	})

	Describe("#Name", func() {
		It("should return the expected name", func() {
			Expect(Name("foo")).To(Equal("etcd-foo"))
		})
	})
})

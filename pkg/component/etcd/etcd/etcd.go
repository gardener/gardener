// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package etcd

import (
	"context"
	"fmt"
	"maps"
	"net"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/Masterminds/semver/v3"
	druidcorev1alpha1 "github.com/gardener/etcd-druid/api/core/v1alpha1"
	"github.com/go-logr/logr"
	monitoringv1 "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1"
	monitoringv1alpha1 "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1alpha1"
	"golang.org/x/text/cases"
	"golang.org/x/text/language"
	appsv1 "k8s.io/api/apps/v1"
	autoscalingv1 "k8s.io/api/autoscaling/v1"
	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	vpaautoscalingv1 "k8s.io/autoscaler/vertical-pod-autoscaler/pkg/apis/autoscaling.k8s.io/v1"
	"k8s.io/client-go/rest"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	resourcesv1alpha1 "github.com/gardener/gardener/pkg/apis/resources/v1alpha1"
	"github.com/gardener/gardener/pkg/component"
	etcdconstants "github.com/gardener/gardener/pkg/component/etcd/etcd/constants"
	"github.com/gardener/gardener/pkg/component/observability/monitoring/prometheus/garden"
	"github.com/gardener/gardener/pkg/component/observability/monitoring/prometheus/shoot"
	monitoringutils "github.com/gardener/gardener/pkg/component/observability/monitoring/utils"
	"github.com/gardener/gardener/pkg/controllerutils"
	gardenletconfigv1alpha1 "github.com/gardener/gardener/pkg/gardenlet/apis/config/v1alpha1"
	"github.com/gardener/gardener/pkg/utils"
	gardenerutils "github.com/gardener/gardener/pkg/utils/gardener"
	kubernetesutils "github.com/gardener/gardener/pkg/utils/kubernetes"
	"github.com/gardener/gardener/pkg/utils/kubernetes/health"
	secretsutils "github.com/gardener/gardener/pkg/utils/secrets"
	secretsmanager "github.com/gardener/gardener/pkg/utils/secrets/manager"
)

// Class is a string type alias for etcd classes.
type Class string

const (
	// ClassNormal is a constant for a normal etcd (without extensive metrics or higher resource settings, etc.)
	ClassNormal Class = "normal"
	// ClassImportant is a constant for an important etcd (with extensive metrics or higher resource settings, etc.).
	// Such etcds are also unsafe to evict (from the PoV of the cluster-autoscaler when trying to scale down).
	ClassImportant Class = "important"

	// SecretNameClient is the name of the secret containing the client certificate and key for the etcd.
	SecretNameClient       = "etcd-client"
	secretNamePrefixServer = "etcd-server-" // #nosec G101 -- No credential.

	// secretNamePrefixPeerServer is the prefix for the secret containing the server certificate and key for the etcd peer network.
	secretNamePrefixPeerServer = "etcd-peer-server-" // #nosec G101 -- No credential.

	// LabelAppValue is the value of a label whose key is 'app'.
	LabelAppValue = "etcd-statefulset"

	portNameClient        = "client"
	portNameBackupRestore = "backuprestore"

	statefulSetNamePrefix      = "etcd"
	containerNameEtcd          = "etcd"
	containerNameBackupRestore = "backup-restore"
)

var (
	// TimeNow is a function returning the current time exposed for testing.
	TimeNow = time.Now
)

// Interface contains functions for a etcd deployer.
type Interface interface {
	component.DeployWaiter
	// Snapshot triggers the backup-restore sidecar to perform a full snapshot in case backup configuration is provided.
	Snapshot(context.Context, rest.HTTPClient) error
	// SetBackupConfig sets the backup configuration.
	SetBackupConfig(config *BackupConfig)
	// Get retrieves the Etcd resource
	Get(context.Context) (*druidcorev1alpha1.Etcd, error)
	// Scale scales the etcd resource to the given replica count.
	Scale(context.Context, int32) error
	// RolloutPeerCA gets the peer CA and patches the
	// related `etcd` resource to use this new CA for peer communication.
	RolloutPeerCA(context.Context) error
	// GetValues returns the current configuration values of the deployer.
	GetValues() Values
	// GetReplicas gets the Replicas field in the Values.
	GetReplicas() *int32
	// SetReplicas sets the Replicas field in the Values.
	SetReplicas(*int32)
}

// New creates a new instance of DeployWaiter for the Etcd.
func New(
	log logr.Logger,
	c client.Client,
	namespace string,
	secretsManager secretsmanager.Interface,
	values Values,
) Interface {
	name := values.NamePrefix + "etcd-" + values.Role
	log = log.WithValues("etcd", client.ObjectKey{Namespace: namespace, Name: name})

	return &etcd{
		client:         c,
		log:            log,
		namespace:      namespace,
		secretsManager: secretsManager,
		values:         values,
		etcd: &druidcorev1alpha1.Etcd{
			ObjectMeta: metav1.ObjectMeta{
				Name:      name,
				Namespace: namespace,
			},
		},
	}
}

type etcd struct {
	client         client.Client
	log            logr.Logger
	namespace      string
	secretsManager secretsmanager.Interface
	values         Values
	etcd           *druidcorev1alpha1.Etcd
}

// Values are the configuration values for the ETCD.
type Values struct {
	NamePrefix                  string
	Role                        string
	Class                       Class
	Replicas                    *int32
	StorageCapacity             string
	StorageClassName            *string
	DefragmentationSchedule     *string
	CARotationPhase             gardencorev1beta1.CredentialsRotationPhase
	Autoscaling                 AutoscalingConfig
	RuntimeKubernetesVersion    *semver.Version
	BackupConfig                *BackupConfig
	MaintenanceTimeWindow       gardencorev1beta1.MaintenanceTimeWindow
	EvictionRequirement         *string
	PriorityClassName           string
	HighAvailabilityEnabled     bool
	TopologyAwareRoutingEnabled bool
}

// BackupConfig contains information for configuring the backup-restore sidecar so that it takes regularly backups of
// the etcd's data directory.
type BackupConfig struct {
	// Provider is the name of the infrastructure provider for the blob storage bucket.
	Provider string
	// Container is the name of the blob storage bucket.
	Container string
	// SecretRefName is the name of a Secret object containing the credentials of the selected infrastructure provider.
	SecretRefName string
	// Prefix is a prefix that shall be used for the filename of the backups of this etcd.
	Prefix string
	// FullSnapshotSchedule is a cron schedule that declares how frequent full snapshots shall be taken.
	FullSnapshotSchedule string
	// LeaderElection contains configuration for the leader election for the etcd backup-restore sidecar.
	LeaderElection *gardenletconfigv1alpha1.ETCDBackupLeaderElection
	// DeltaSnapshotRetentionPeriod defines the duration for which delta snapshots will be retained, excluding the latest snapshot set.
	DeltaSnapshotRetentionPeriod *metav1.Duration
}

// AutoscalingConfig contains information for configuring autoscaling settings for etcd.
type AutoscalingConfig struct {
	// MinAllowed are the minimum allowed resources for vertical autoscaling.
	MinAllowed corev1.ResourceList
}

func (e *etcd) Deploy(ctx context.Context) error {
	var existingEtcd *druidcorev1alpha1.Etcd

	if err := e.client.Get(ctx, client.ObjectKeyFromObject(e.etcd), e.etcd); err != nil {
		if !apierrors.IsNotFound(err) {
			return err
		}
	} else {
		existingEtcd = e.etcd.DeepCopy()
	}

	var (
		vpa            = e.emptyVerticalPodAutoscaler()
		serviceMonitor = e.emptyServiceMonitor()

		replicas = e.computeReplicas(existingEtcd)

		garbageCollectionPolicy = druidcorev1alpha1.GarbageCollectionPolicy(druidcorev1alpha1.GarbageCollectionPolicyExponential)
		garbageCollectionPeriod = metav1.Duration{Duration: 12 * time.Hour}
		compressionPolicy       = druidcorev1alpha1.GzipCompression
		compressionSpec         = druidcorev1alpha1.CompressionSpec{
			Enabled: ptr.To(true),
			Policy:  &compressionPolicy,
		}

		annotations         map[string]string
		metrics             = druidcorev1alpha1.Basic
		volumeClaimTemplate = e.etcd.Name

		minAllowed             = e.computeMinAllowedForETCDContainer()
		resourcesEtcd          = e.computeETCDContainerResources(minAllowed)
		resourcesBackupRestore = e.computeBackupRestoreContainerResources()
		resourcesCompactionJob = e.computeCompactionJobContainerResources()
	)

	if e.values.Class == ClassImportant {
		if !e.values.HighAvailabilityEnabled {
			annotations = map[string]string{"cluster-autoscaler.kubernetes.io/safe-to-evict": "false"}
		}
		metrics = druidcorev1alpha1.Extensive
		volumeClaimTemplate = e.values.Role + "-" + strings.TrimSuffix(e.etcd.Name, "-"+e.values.Role)
	}

	etcdCASecret, serverSecret, clientSecret, err := GenerateClientServerCertificates(
		ctx,
		e.secretsManager,
		e.values.Role,
		e.clientServiceDNSNames(),
		nil,
	)
	if err != nil {
		return err
	}

	// add peer certs if shoot has HA control plane
	var (
		etcdPeerCASecretName string
		peerServerSecretName string
	)

	if etcdPeerCASecretName, peerServerSecretName, err = e.handlePeerCertificates(ctx); err != nil {
		return err
	}

	clientService := &corev1.Service{}
	gardenerutils.ReconcileTopologyAwareRoutingSettings(clientService, e.values.TopologyAwareRoutingEnabled, e.values.RuntimeKubernetesVersion)

	ports := []networkingv1.NetworkPolicyPort{
		{Port: ptr.To(intstr.FromInt32(etcdconstants.PortEtcdClient)), Protocol: ptr.To(corev1.ProtocolTCP)},
		{Port: ptr.To(intstr.FromInt32(etcdconstants.PortBackupRestore)), Protocol: ptr.To(corev1.ProtocolTCP)},
	}
	if e.values.NamePrefix != "" {
		// etcd deployed for garden cluster
		utilruntime.Must(gardenerutils.InjectNetworkPolicyAnnotationsForGardenScrapeTargets(clientService, ports...))
	} else {
		// etcd deployed for shoot cluster
		utilruntime.Must(gardenerutils.InjectNetworkPolicyAnnotationsForScrapeTargets(clientService, ports...))
		utilruntime.Must(gardenerutils.InjectNetworkPolicyNamespaceSelectors(clientService, metav1.LabelSelector{MatchLabels: map[string]string{corev1.LabelMetadataName: v1beta1constants.GardenNamespace}}))
		metav1.SetMetaDataAnnotation(&clientService.ObjectMeta, resourcesv1alpha1.NetworkingPodLabelSelectorNamespaceAlias, v1beta1constants.LabelNetworkPolicyShootNamespaceAlias)
	}

	if _, err := controllerutils.GetAndCreateOrMergePatch(ctx, e.client, e.etcd, func() error {
		metav1.SetMetaDataAnnotation(&e.etcd.ObjectMeta, v1beta1constants.GardenerOperation, v1beta1constants.GardenerOperationReconcile)
		metav1.SetMetaDataAnnotation(&e.etcd.ObjectMeta, v1beta1constants.GardenerTimestamp, TimeNow().UTC().Format(time.RFC3339Nano))

		e.etcd.Labels = map[string]string{
			v1beta1constants.LabelRole:  e.values.Role,
			v1beta1constants.GardenRole: v1beta1constants.GardenRoleControlPlane,
		}
		if e.values.Class == ClassNormal {
			metav1.SetMetaDataAnnotation(&e.etcd.ObjectMeta, "resources.druid.gardener.cloud/allow-unhealthy-pod-eviction", "")
		}
		e.etcd.Spec.Replicas = replicas
		e.etcd.Spec.PriorityClassName = &e.values.PriorityClassName
		e.etcd.Spec.Annotations = annotations
		e.etcd.Spec.Labels = utils.MergeStringMaps(e.getRoleLabels(), map[string]string{
			v1beta1constants.LabelApp:                             LabelAppValue,
			v1beta1constants.LabelNetworkPolicyToDNS:              v1beta1constants.LabelNetworkPolicyAllowed,
			v1beta1constants.LabelNetworkPolicyToPublicNetworks:   v1beta1constants.LabelNetworkPolicyAllowed,
			v1beta1constants.LabelNetworkPolicyToPrivateNetworks:  v1beta1constants.LabelNetworkPolicyAllowed,
			v1beta1constants.LabelNetworkPolicyToRuntimeAPIServer: v1beta1constants.LabelNetworkPolicyAllowed,
		})

		if e.values.HighAvailabilityEnabled {
			// Allow etcd p2p communication
			e.etcd.Spec.Labels = utils.MergeStringMaps(e.etcd.Spec.Labels, map[string]string{
				gardenerutils.NetworkPolicyLabel(e.values.NamePrefix+etcdconstants.ServiceName(e.values.Role), etcdconstants.PortEtcdClient):    v1beta1constants.LabelNetworkPolicyAllowed,
				gardenerutils.NetworkPolicyLabel(e.values.NamePrefix+etcdconstants.ServiceName(e.values.Role), etcdconstants.PortBackupRestore): v1beta1constants.LabelNetworkPolicyAllowed,
				gardenerutils.NetworkPolicyLabel(e.values.NamePrefix+etcdconstants.ServiceName(e.values.Role), etcdconstants.PortEtcdPeer):      v1beta1constants.LabelNetworkPolicyAllowed,
			})
		}

		e.etcd.Spec.Selector = &metav1.LabelSelector{
			MatchLabels: utils.MergeStringMaps(e.getRoleLabels(), map[string]string{
				v1beta1constants.LabelApp: LabelAppValue,
			}),
		}
		e.etcd.Spec.Etcd = druidcorev1alpha1.EtcdConfig{
			Resources: resourcesEtcd,
			ClientUrlTLS: &druidcorev1alpha1.TLSConfig{
				TLSCASecretRef: druidcorev1alpha1.SecretReference{
					SecretReference: corev1.SecretReference{
						Name:      etcdCASecret.Name,
						Namespace: etcdCASecret.Namespace,
					},
					DataKey: ptr.To(secretsutils.DataKeyCertificateBundle),
				},
				ServerTLSSecretRef: corev1.SecretReference{
					Name:      serverSecret.Name,
					Namespace: serverSecret.Namespace,
				},
				ClientTLSSecretRef: corev1.SecretReference{
					Name:      clientSecret.Name,
					Namespace: clientSecret.Namespace,
				},
			},
			ServerPort:              ptr.To(etcdconstants.PortEtcdPeer),
			ClientPort:              ptr.To(etcdconstants.PortEtcdClient),
			Metrics:                 &metrics,
			DefragmentationSchedule: e.computeDefragmentationSchedule(existingEtcd),
			Quota:                   ptr.To(resource.MustParse("8Gi")),
			ClientService: &druidcorev1alpha1.ClientService{
				Annotations:         clientService.Annotations,
				Labels:              clientService.Labels,
				TrafficDistribution: clientService.Spec.TrafficDistribution,
			},
		}

		// TODO(timuthy): Once https://github.com/gardener/etcd-backup-restore/issues/538 is resolved we can enable PeerUrlTLS for all remaining clusters as well.
		if e.values.HighAvailabilityEnabled {
			e.etcd.Spec.Etcd.PeerUrlTLS = &druidcorev1alpha1.TLSConfig{
				TLSCASecretRef: druidcorev1alpha1.SecretReference{
					SecretReference: corev1.SecretReference{
						Name:      etcdPeerCASecretName,
						Namespace: e.namespace,
					},
					DataKey: ptr.To(secretsutils.DataKeyCertificateBundle),
				},
				ServerTLSSecretRef: corev1.SecretReference{
					Name:      peerServerSecretName,
					Namespace: e.namespace,
				},
			}
		}

		e.etcd.Spec.Backup = druidcorev1alpha1.BackupSpec{
			TLS: &druidcorev1alpha1.TLSConfig{
				TLSCASecretRef: druidcorev1alpha1.SecretReference{
					SecretReference: corev1.SecretReference{
						Name:      etcdCASecret.Name,
						Namespace: etcdCASecret.Namespace,
					},
					DataKey: ptr.To(secretsutils.DataKeyCertificateBundle),
				},
				ServerTLSSecretRef: corev1.SecretReference{
					Name:      serverSecret.Name,
					Namespace: serverSecret.Namespace,
				},
				ClientTLSSecretRef: corev1.SecretReference{
					Name:      clientSecret.Name,
					Namespace: clientSecret.Namespace,
				},
			},
			Port:                    ptr.To(etcdconstants.PortBackupRestore),
			Resources:               resourcesBackupRestore,
			CompactionResources:     resourcesCompactionJob,
			GarbageCollectionPolicy: &garbageCollectionPolicy,
			GarbageCollectionPeriod: &garbageCollectionPeriod,
			SnapshotCompression:     &compressionSpec,
		}

		if e.values.BackupConfig != nil {
			var (
				provider            = druidcorev1alpha1.StorageProvider(e.values.BackupConfig.Provider)
				deltaSnapshotPeriod = metav1.Duration{Duration: 5 * time.Minute}
			)

			e.etcd.Spec.Backup.Store = &druidcorev1alpha1.StoreSpec{
				SecretRef: &corev1.SecretReference{Name: e.values.BackupConfig.SecretRefName},
				Container: &e.values.BackupConfig.Container,
				Provider:  &provider,
				Prefix:    fmt.Sprintf("%s/etcd-%s", e.values.BackupConfig.Prefix, e.values.Role),
			}
			e.etcd.Spec.Backup.FullSnapshotSchedule = e.computeFullSnapshotSchedule(existingEtcd)
			e.etcd.Spec.Backup.DeltaSnapshotPeriod = &deltaSnapshotPeriod
			e.etcd.Spec.Backup.DeltaSnapshotMemoryLimit = ptr.To(resource.MustParse("100Mi"))
			e.etcd.Spec.Backup.DeltaSnapshotRetentionPeriod = e.values.BackupConfig.DeltaSnapshotRetentionPeriod

			if e.values.BackupConfig.LeaderElection != nil {
				e.etcd.Spec.Backup.LeaderElection = &druidcorev1alpha1.LeaderElectionSpec{
					EtcdConnectionTimeout: e.values.BackupConfig.LeaderElection.EtcdConnectionTimeout,
					ReelectionPeriod:      e.values.BackupConfig.LeaderElection.ReelectionPeriod,
				}
			}
		}

		if existingEtcd == nil || existingEtcd.Spec.StorageCapacity == nil {
			e.etcd.Spec.StorageCapacity = ptr.To(resource.MustParse(e.values.StorageCapacity))
		}
		e.etcd.Spec.StorageClass = e.values.StorageClassName
		e.etcd.Spec.VolumeClaimTemplate = &volumeClaimTemplate
		return nil
	}); err != nil {
		return err
	}

	if err := e.reconcileVerticalPodAutoscaler(ctx, vpa, minAllowed); err != nil {
		return err
	}

	// etcd deployed for shoot cluster
	serviceMonitorJobNameEtcd, serviceMonitorJobNameBackupRestore := "kube-etcd3-"+e.values.Role, "kube-etcd3-backup-restore-"+e.values.Role
	if e.values.NamePrefix != "" {
		// etcd deployed for garden cluster
		serviceMonitorJobNameEtcd, serviceMonitorJobNameBackupRestore = e.values.NamePrefix+"etcd", e.values.NamePrefix+"etcd-backup"
	}

	if _, err := controllerutils.GetAndCreateOrMergePatch(ctx, e.client, serviceMonitor, func() error {
		serviceMonitor.Labels = monitoringutils.Labels(e.prometheusLabel())
		serviceMonitor.Spec = monitoringv1.ServiceMonitorSpec{
			Selector: metav1.LabelSelector{MatchLabels: map[string]string{
				druidcorev1alpha1.LabelAppNameKey: fmt.Sprintf("%s-client", e.etcd.Name),
				druidcorev1alpha1.LabelPartOfKey:  e.etcd.Name,
			}},
			Endpoints: []monitoringv1.Endpoint{
				{
					Port:   portNameClient,
					Scheme: "https",
					TLSConfig: &monitoringv1.TLSConfig{SafeTLSConfig: monitoringv1.SafeTLSConfig{
						// This is needed because the etcd's certificates are not are generated for a specific pod IP.
						InsecureSkipVerify: ptr.To(true),
						Cert: monitoringv1.SecretOrConfigMap{Secret: &corev1.SecretKeySelector{
							LocalObjectReference: corev1.LocalObjectReference{Name: clientSecret.Name},
							Key:                  secretsutils.DataKeyCertificate,
						}},
						KeySecret: &corev1.SecretKeySelector{
							LocalObjectReference: corev1.LocalObjectReference{Name: clientSecret.Name},
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
							Replacement: ptr.To(serviceMonitorJobNameEtcd),
							TargetLabel: "job",
						},
					},
					MetricRelabelConfigs: []monitoringv1.RelabelConfig{{
						Action: "labeldrop",
						Regex:  `^instance$`,
					}},
				},
				{
					Port:   portNameBackupRestore,
					Scheme: "https",
					TLSConfig: &monitoringv1.TLSConfig{SafeTLSConfig: monitoringv1.SafeTLSConfig{
						// This is needed because the etcd's certificates are not are generated for a specific pod IP.
						InsecureSkipVerify: ptr.To(true),
						Cert: monitoringv1.SecretOrConfigMap{Secret: &corev1.SecretKeySelector{
							LocalObjectReference: corev1.LocalObjectReference{Name: clientSecret.Name},
							Key:                  secretsutils.DataKeyCertificate,
						}},
						KeySecret: &corev1.SecretKeySelector{
							LocalObjectReference: corev1.LocalObjectReference{Name: clientSecret.Name},
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
							Replacement: ptr.To(serviceMonitorJobNameBackupRestore),
							TargetLabel: "job",
						},
					},
					MetricRelabelConfigs: append([]monitoringv1.RelabelConfig{{
						Action: "labeldrop",
						Regex:  `^instance$`,
					}}, monitoringutils.StandardMetricRelabelConfig(
						"etcdbr_defragmentation_duration_seconds_bucket",
						"etcdbr_defragmentation_duration_seconds_count",
						"etcdbr_defragmentation_duration_seconds_sum",
						"etcdbr_network_received_bytes",
						"etcdbr_network_transmitted_bytes",
						"etcdbr_restoration_duration_seconds_bucket",
						"etcdbr_restoration_duration_seconds_count",
						"etcdbr_restoration_duration_seconds_sum",
						"etcdbr_snapshot_duration_seconds_bucket",
						"etcdbr_snapshot_duration_seconds_count",
						"etcdbr_snapshot_duration_seconds_sum",
						"etcdbr_snapshot_gc_total",
						"etcdbr_snapshot_latest_revision",
						"etcdbr_snapshot_latest_timestamp",
						"etcdbr_snapshot_required",
						"etcdbr_validation_duration_seconds_bucket",
						"etcdbr_validation_duration_seconds_count",
						"etcdbr_validation_duration_seconds_sum",
						"etcdbr_snapshotter_failure",
						"etcdbr_cluster_size",
						"etcdbr_is_learner",
						"etcdbr_is_learner_count_total",
						"etcdbr_add_learner_duration_seconds_bucket",
						"etcdbr_add_learner_duration_seconds_sum",
						"etcdbr_member_remove_duration_seconds_bucket",
						"etcdbr_member_remove_duration_seconds_sum",
						"etcdbr_member_promote_duration_seconds_bucket",
						"etcdbr_member_promote_duration_seconds_sum",
						"process_resident_memory_bytes",
						"process_cpu_seconds_total",
					)...),
				},
			},
		}
		return nil
	}); err != nil {
		return err
	}

	// etcd deployed for shoot cluster
	if e.values.NamePrefix == "" {
		// TODO: The PrometheusRules for the garden cluster case are maintained in a separate file located here:
		//  pkg/component/observability/monitoring/prometheus/garden/assets/prometheusrules/etcd.yaml
		//  These rules highly overlap with those for the shoots maintained here. They should be merged in the future.
		var (
			role     = cases.Title(language.English).String(e.values.Role)
			alertFor = func(classImportantDuration, classNormalDuration monitoringv1.Duration) *monitoringv1.Duration {
				if e.values.Class == ClassImportant {
					return &classImportantDuration
				}
				return &classNormalDuration
			}
			severityLabel = func(classImportantValue, classNormalValue string) string {
				if e.values.Class == ClassImportant {
					return classImportantValue
				}
				return classNormalValue
			}
		)

		prometheusRule := e.emptyPrometheusRule()
		if _, err := controllerutils.GetAndCreateOrMergePatch(ctx, e.client, prometheusRule, func() error {
			prometheusRule.Labels = monitoringutils.Labels(e.prometheusLabel())
			prometheusRule.Spec = monitoringv1.PrometheusRuleSpec{
				Groups: []monitoringv1.RuleGroup{{
					Name: serviceMonitorJobNameEtcd + ".rules",
					Rules: []monitoringv1.Rule{
						// alert if etcd is down
						{
							Alert: "KubeEtcd" + role + "Down",
							Expr:  intstr.FromString(`sum(up{job="` + serviceMonitorJobNameEtcd + `"}) < ` + strconv.Itoa(int(replicas/2)+1)),
							For:   alertFor("5m", "15m"),
							Labels: map[string]string{
								"service":    "etcd",
								"severity":   severityLabel("blocker", "critical"),
								"type":       "seed",
								"visibility": "operator",
							},
							Annotations: map[string]string{
								"summary":     "Etcd3 " + e.values.Role + " cluster down.",
								"description": "Etcd3 cluster " + e.values.Role + " is unavailable (due to possible quorum loss) or cannot be scraped. As long as etcd3 " + e.values.Role + " is down, the cluster is unreachable.",
							},
						},
						// etcd leader alerts
						{
							Alert: "KubeEtcd3" + role + "NoLeader",
							Expr:  intstr.FromString(`sum(etcd_server_has_leader{job="` + serviceMonitorJobNameEtcd + `"}) < count(etcd_server_has_leader{job="` + serviceMonitorJobNameEtcd + `"})`),
							For:   alertFor("10m", "15m"),
							Labels: map[string]string{
								"service":    "etcd",
								"severity":   "critical",
								"type":       "seed",
								"visibility": "operator",
							},
							Annotations: map[string]string{
								"summary":     "Etcd3 " + e.values.Role + " has no leader.",
								"description": "Etcd3 cluster " + e.values.Role + " has no leader. Possible network partition in the etcd cluster.",
							},
						},
						{
							Alert: "KubeEtcd3" + role + "HighMemoryConsumption",
							Expr:  intstr.FromString(`sum(container_memory_working_set_bytes{pod="etcd-` + e.values.Role + `-0",container="` + containerNameEtcd + `"}) / sum(kube_verticalpodautoscaler_spec_resourcepolicy_container_policies_maxallowed{container="` + containerNameEtcd + `", targetName="etcd-` + e.values.Role + `", resource="memory"}) > .5`),
							For:   ptr.To(monitoringv1.Duration("15m")),
							Labels: map[string]string{
								"service":    "etcd",
								"severity":   "warning",
								"type":       "seed",
								"visibility": "operator",
							},
							Annotations: map[string]string{
								"summary":     "Etcd3 " + e.values.Role + " is consuming too much memory",
								"description": "Etcd3 " + e.values.Role + " is consuming over 50% of the max allowed value specified by VPA.",
							},
						},
						// etcd DB size alerts
						{
							Alert: "KubeEtcd3" + role + "DbSizeLimitApproaching",
							Expr:  intstr.FromString(`(etcd_mvcc_db_total_size_in_bytes{job="` + serviceMonitorJobNameEtcd + `"} > bool 7516193000) + (etcd_mvcc_db_total_size_in_bytes{job="` + serviceMonitorJobNameEtcd + `"} <= bool 8589935000) == 2`), // between 7GB and 8GB
							Labels: map[string]string{
								"service":    "etcd",
								"severity":   "warning",
								"type":       "seed",
								"visibility": "all",
							},
							Annotations: map[string]string{
								"summary":     "Etcd3 " + e.values.Role + " DB size is approaching its current practical limit.",
								"description": "Etcd3 " + e.values.Role + " DB size is approaching its current practical limit of 8GB. Etcd quota might need to be increased.",
							},
						},
						{
							Alert: "KubeEtcd3" + role + "DbSizeLimitCrossed",
							Expr:  intstr.FromString(`etcd_mvcc_db_total_size_in_bytes{job="` + serviceMonitorJobNameEtcd + `"} > 8589935000`), // above 8GB
							Labels: map[string]string{
								"service":    "etcd",
								"severity":   "critical",
								"type":       "seed",
								"visibility": "all",
							},
							Annotations: map[string]string{
								"summary":     "Etcd3 " + e.values.Role + " DB size has crossed its current practical limit.",
								"description": "Etcd3 " + e.values.Role + " DB size has crossed its current practical limit of 8GB. Etcd quota must be increased to allow updates.",
							},
						},
						{
							Record: "shoot:apiserver_storage_objects:sum_by_resource",
							Expr:   intstr.FromString(`max(apiserver_storage_objects) by (resource)`),
						},
					},
				}},
			}

			if e.values.BackupConfig != nil {
				prometheusRule.Spec.Groups[0].Rules = append(prometheusRule.Spec.Groups[0].Rules,
					// etcd backup failure alerts
					monitoringv1.Rule{
						Alert: "KubeEtcd" + role + "DeltaBackupFailed",
						Expr:  intstr.FromString(`((time() - etcdbr_snapshot_latest_timestamp{job="` + serviceMonitorJobNameBackupRestore + `",kind="Incr"} > bool 900) * etcdbr_snapshot_required{job="` + serviceMonitorJobNameBackupRestore + `",kind="Incr"}) * on (pod, role) etcd_server_is_leader{job="` + serviceMonitorJobNameEtcd + `"} > 0`),
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
						Alert: "KubeEtcd" + role + "FullBackupFailed",
						Expr:  intstr.FromString(`((time() - etcdbr_snapshot_latest_timestamp{job="` + serviceMonitorJobNameBackupRestore + `",kind="Full"} > bool 86400) * etcdbr_snapshot_required{job="` + serviceMonitorJobNameBackupRestore + `",kind="Full"}) * on (pod, role) etcd_server_is_leader{job="` + serviceMonitorJobNameEtcd + `"} > 0`),
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
					// etcd data restoration failure alert
					monitoringv1.Rule{
						Alert: "KubeEtcd" + role + "RestorationFailed",
						Expr:  intstr.FromString(`rate(etcdbr_restoration_duration_seconds_count{job="` + serviceMonitorJobNameBackupRestore + `",succeeded="false"}[2m]) > 0`),
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
						Alert: "KubeEtcd" + role + "BackupRestoreDown",
						Expr:  intstr.FromString(`(sum(up{job="` + serviceMonitorJobNameEtcd + `"}) - sum(up{job="` + serviceMonitorJobNameBackupRestore + `"}) > 0) or (rate(etcdbr_snapshotter_failure{job="` + serviceMonitorJobNameBackupRestore + `"}[5m]) > 0)`),
						For:   ptr.To(monitoringv1.Duration("10m")),
						Labels: map[string]string{
							"service":    "etcd",
							"severity":   "critical",
							"type":       "seed",
							"visibility": "operator",
						},
						Annotations: map[string]string{
							"summary":     "Etcd backup restore " + e.values.Role + " process down or snapshotter failed with error",
							"description": "Etcd backup restore " + e.values.Role + " process down or snapshotter failed with error. Backups will not be triggered unless backup restore is brought back up. This is unsafe behaviour and may cause data loss.",
						},
					},
				)
			}

			return nil
		}); err != nil {
			return err
		}
	}

	return nil
}

func (e *etcd) Destroy(ctx context.Context) error {
	if err := gardenerutils.ConfirmDeletion(ctx, e.client, e.etcd); client.IgnoreNotFound(err) != nil {
		return err
	}

	return kubernetesutils.DeleteObjects(ctx, e.client,
		e.emptyVerticalPodAutoscaler(),
		e.emptyServiceMonitor(),
		e.emptyScrapeConfig(),
		e.emptyPrometheusRule(),
		e.etcd,
	)
}

func (e *etcd) getRoleLabels() map[string]string {
	return utils.MergeStringMaps(map[string]string{
		v1beta1constants.GardenRole: v1beta1constants.GardenRoleControlPlane,
		v1beta1constants.LabelRole:  e.values.Role,
	})
}

func (e *etcd) prometheusLabel() string {
	if e.values.NamePrefix != "" {
		return garden.Label
	}
	return shoot.Label
}

func (e *etcd) emptyServiceMonitor() *monitoringv1.ServiceMonitor {
	return &monitoringv1.ServiceMonitor{ObjectMeta: monitoringutils.ConfigObjectMeta(e.etcd.Name, e.namespace, e.prometheusLabel())}
}

func (e *etcd) emptyPrometheusRule() *monitoringv1.PrometheusRule {
	return &monitoringv1.PrometheusRule{ObjectMeta: monitoringutils.ConfigObjectMeta(e.etcd.Name, e.namespace, e.prometheusLabel())}
}

func (e *etcd) emptyScrapeConfig() *monitoringv1alpha1.ScrapeConfig {
	return &monitoringv1alpha1.ScrapeConfig{ObjectMeta: monitoringutils.ConfigObjectMeta(Druid, e.namespace, e.prometheusLabel())}
}

func (e *etcd) emptyVerticalPodAutoscaler() *vpaautoscalingv1.VerticalPodAutoscaler {
	return &vpaautoscalingv1.VerticalPodAutoscaler{ObjectMeta: metav1.ObjectMeta{Name: e.etcd.Name, Namespace: e.namespace}}
}

func (e *etcd) reconcileVerticalPodAutoscaler(ctx context.Context, vpa *vpaautoscalingv1.VerticalPodAutoscaler, minAllowedETCD corev1.ResourceList) error {
	vpaUpdateMode := vpaautoscalingv1.UpdateModeAuto
	containerPolicyOff := vpaautoscalingv1.ContainerScalingModeOff
	containerPolicyAuto := vpaautoscalingv1.ContainerScalingModeAuto
	controlledValues := vpaautoscalingv1.ContainerControlledValuesRequestsOnly

	_, err := controllerutils.GetAndCreateOrMergePatch(ctx, e.client, vpa, func() error {
		var evictionRequirement *string

		metav1.SetMetaDataLabel(&vpa.ObjectMeta, v1beta1constants.LabelRole, "etcd-vpa-"+e.values.Role)
		evictionRequirement = e.values.EvictionRequirement
		if ptr.Deref(evictionRequirement, "") == v1beta1constants.EvictionRequirementNever {
			metav1.SetMetaDataLabel(&vpa.ObjectMeta, v1beta1constants.LabelVPAEvictionRequirementsController, v1beta1constants.EvictionRequirementManagedByController)
			metav1.SetMetaDataAnnotation(&vpa.ObjectMeta, v1beta1constants.AnnotationVPAEvictionRequirementDownscaleRestriction, v1beta1constants.EvictionRequirementNever)
		} else if ptr.Deref(evictionRequirement, "") == v1beta1constants.EvictionRequirementInMaintenanceWindowOnly {
			metav1.SetMetaDataLabel(&vpa.ObjectMeta, v1beta1constants.LabelVPAEvictionRequirementsController, v1beta1constants.EvictionRequirementManagedByController)
			metav1.SetMetaDataAnnotation(&vpa.ObjectMeta, v1beta1constants.AnnotationVPAEvictionRequirementDownscaleRestriction, v1beta1constants.EvictionRequirementInMaintenanceWindowOnly)
			metav1.SetMetaDataAnnotation(&vpa.ObjectMeta, v1beta1constants.AnnotationShootMaintenanceWindow, e.values.MaintenanceTimeWindow.Begin+","+e.values.MaintenanceTimeWindow.End)
		} else {
			delete(vpa.GetLabels(), v1beta1constants.LabelVPAEvictionRequirementsController)
			delete(vpa.GetAnnotations(), v1beta1constants.AnnotationVPAEvictionRequirementDownscaleRestriction)
			delete(vpa.GetLabels(), v1beta1constants.AnnotationShootMaintenanceWindow)
		}

		vpa.Spec = vpaautoscalingv1.VerticalPodAutoscalerSpec{
			TargetRef: &autoscalingv1.CrossVersionObjectReference{
				APIVersion: appsv1.SchemeGroupVersion.String(),
				Kind:       "StatefulSet",
				Name:       e.etcd.Name,
			},
			UpdatePolicy: &vpaautoscalingv1.PodUpdatePolicy{
				UpdateMode: &vpaUpdateMode,
			},
			ResourcePolicy: &vpaautoscalingv1.PodResourcePolicy{
				ContainerPolicies: []vpaautoscalingv1.ContainerResourcePolicy{
					{
						ContainerName:    containerNameEtcd,
						MinAllowed:       minAllowedETCD,
						ControlledValues: &controlledValues,
						Mode:             &containerPolicyAuto,
					},
					{
						ContainerName:    containerNameBackupRestore,
						Mode:             &containerPolicyOff,
						ControlledValues: &controlledValues,
					},
				},
			},
		}

		return nil
	})

	return err
}

func (e *etcd) Snapshot(ctx context.Context, httpClient rest.HTTPClient) error {
	if e.values.BackupConfig == nil {
		return fmt.Errorf("no backup is configured for this etcd, cannot make a snapshot")
	}

	url := fmt.Sprintf("https://%s%s.%s:%d/snapshot/full?final=true", e.values.NamePrefix, etcdconstants.ServiceName(e.values.Role), e.namespace, etcdconstants.PortBackupRestore)

	request, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return err
	}

	resp, err := httpClient.Do(request)
	if err == nil && resp != nil && resp.StatusCode != http.StatusOK {
		return fmt.Errorf("error occurred while initiating ETCD snapshot: %s", resp.Status)
	}

	return err
}

func (e *etcd) clientServiceDNSNames() []string {
	var domainNames []string
	domainNames = append(domainNames, fmt.Sprintf("%s-local", e.etcd.Name))
	domainNames = append(domainNames, kubernetesutils.DNSNamesForService(fmt.Sprintf("%s-client", e.etcd.Name), e.namespace)...)

	// The peer service needs to be considered here since the etcd-backup-restore side-car
	// connects to member pods via pod domain names (e.g. for defragmentation).
	// See https://github.com/gardener/etcd-backup-restore/issues/494
	domainNames = append(domainNames, kubernetesutils.DNSNamesForService(fmt.Sprintf("*.%s-peer", e.etcd.Name), e.namespace)...)

	return domainNames
}

func (e *etcd) peerServiceDNSNames() []string {
	return append(
		kubernetesutils.DNSNamesForService(fmt.Sprintf("%s-peer", e.etcd.Name), e.namespace),
		kubernetesutils.DNSNamesForService(fmt.Sprintf("*.%s-peer", e.etcd.Name), e.namespace)...,
	)
}

// Get retrieves the Etcd resource
func (e *etcd) Get(ctx context.Context) (*druidcorev1alpha1.Etcd, error) {
	if err := e.client.Get(ctx, client.ObjectKeyFromObject(e.etcd), e.etcd); err != nil {
		return nil, err
	}
	return e.etcd, nil
}

func (e *etcd) SetBackupConfig(backupConfig *BackupConfig) { e.values.BackupConfig = backupConfig }

func (e *etcd) Scale(ctx context.Context, replicas int32) error {
	etcdObj := &druidcorev1alpha1.Etcd{}
	if err := e.client.Get(ctx, client.ObjectKeyFromObject(e.etcd), etcdObj); err != nil {
		return err
	}

	if expectedTimestamp, ok := e.etcd.Annotations[v1beta1constants.GardenerTimestamp]; ok {
		if err := health.ObjectHasAnnotationWithValue(v1beta1constants.GardenerTimestamp, expectedTimestamp)(etcdObj); err != nil {
			return err
		}
	}

	if _, ok := etcdObj.Annotations[v1beta1constants.GardenerOperation]; ok {
		return fmt.Errorf("etcd object still has operation annotation set")
	}

	patch := client.MergeFrom(etcdObj.DeepCopy())
	if e.etcd.Annotations == nil {
		etcdObj.SetAnnotations(make(map[string]string))
	}

	etcdObj.Annotations[v1beta1constants.GardenerOperation] = v1beta1constants.GardenerOperationReconcile
	etcdObj.Annotations[v1beta1constants.GardenerTimestamp] = TimeNow().UTC().Format(time.RFC3339Nano)
	etcdObj.Spec.Replicas = replicas

	e.etcd = etcdObj

	return e.client.Patch(ctx, etcdObj, patch)
}

func (e *etcd) RolloutPeerCA(ctx context.Context) error {
	if !e.values.HighAvailabilityEnabled {
		return nil
	}

	etcdPeerCASecret, found := e.secretsManager.Get(v1beta1constants.SecretNameCAETCDPeer)
	if !found {
		return fmt.Errorf("secret %q not found", v1beta1constants.SecretNameCAETCDPeer)
	}

	if _, err := controllerutils.GetAndCreateOrMergePatch(ctx, e.client, e.etcd, func() error {
		e.etcd.Annotations = map[string]string{
			v1beta1constants.GardenerOperation: v1beta1constants.GardenerOperationReconcile,
			v1beta1constants.GardenerTimestamp: TimeNow().UTC().Format(time.RFC3339Nano),
		}

		var dataKey *string
		if e.etcd.Spec.Etcd.PeerUrlTLS != nil {
			dataKey = e.etcd.Spec.Etcd.PeerUrlTLS.TLSCASecretRef.DataKey
		}

		if e.etcd.Spec.Etcd.PeerUrlTLS == nil {
			e.etcd.Spec.Etcd.PeerUrlTLS = &druidcorev1alpha1.TLSConfig{}
		}

		e.etcd.Spec.Etcd.PeerUrlTLS.TLSCASecretRef = druidcorev1alpha1.SecretReference{
			SecretReference: corev1.SecretReference{
				Name:      etcdPeerCASecret.Name,
				Namespace: e.etcd.Namespace,
			},
			DataKey: dataKey,
		}
		return nil
	}); err != nil {
		return err
	}

	return e.Wait(ctx)
}

func (e *etcd) GetValues() Values { return e.values }

func (e *etcd) GetReplicas() *int32 { return e.values.Replicas }

func (e *etcd) SetReplicas(replicas *int32) { e.values.Replicas = replicas }

func (e *etcd) computeMinAllowedForETCDContainer() corev1.ResourceList {
	minAllowed := corev1.ResourceList{
		corev1.ResourceMemory: resource.MustParse("60M"),
	}

	if e.values.Class == ClassImportant {
		minAllowed = corev1.ResourceList{
			corev1.ResourceMemory: resource.MustParse("300M"),
		}
	}

	maps.Insert(minAllowed, maps.All(e.values.Autoscaling.MinAllowed))
	return minAllowed
}

func (e *etcd) computeETCDContainerResources(minAllowedETCD corev1.ResourceList) *corev1.ResourceRequirements {
	resourcesETCD := corev1.ResourceList{
		corev1.ResourceCPU:    resource.MustParse("300m"),
		corev1.ResourceMemory: resource.MustParse("1G"),
	}

	if len(minAllowedETCD) > 0 {
		return &corev1.ResourceRequirements{Requests: minAllowedETCD}
	}

	return &corev1.ResourceRequirements{Requests: resourcesETCD}
}

func (e *etcd) computeBackupRestoreContainerResources() *corev1.ResourceRequirements {
	resourcesBackupRestore := corev1.ResourceList{
		corev1.ResourceCPU:    resource.MustParse("10m"),
		corev1.ResourceMemory: resource.MustParse("40Mi"),
	}

	if e.values.Class == ClassImportant {
		resourcesBackupRestore = corev1.ResourceList{
			corev1.ResourceCPU:    resource.MustParse("20m"),
			corev1.ResourceMemory: resource.MustParse("80Mi"),
		}
	}

	return &corev1.ResourceRequirements{Requests: resourcesBackupRestore}
}

func (e *etcd) computeCompactionJobContainerResources() *corev1.ResourceRequirements {
	return &corev1.ResourceRequirements{
		Requests: corev1.ResourceList{
			corev1.ResourceCPU:    resource.MustParse("600m"),
			corev1.ResourceMemory: resource.MustParse("3Gi"),
		},
	}
}

func (e *etcd) computeReplicas(existingEtcd *druidcorev1alpha1.Etcd) int32 {
	if e.values.Replicas != nil {
		return *e.values.Replicas
	}

	if existingEtcd != nil {
		return existingEtcd.Spec.Replicas
	}
	return 0
}

func (e *etcd) computeDefragmentationSchedule(existingEtcd *druidcorev1alpha1.Etcd) *string {
	defragmentationSchedule := e.values.DefragmentationSchedule
	if existingEtcd != nil && existingEtcd.Spec.Etcd.DefragmentationSchedule != nil {
		defragmentationSchedule = existingEtcd.Spec.Etcd.DefragmentationSchedule
	}
	return defragmentationSchedule
}

func (e *etcd) computeFullSnapshotSchedule(existingEtcd *druidcorev1alpha1.Etcd) *string {
	fullSnapshotSchedule := &e.values.BackupConfig.FullSnapshotSchedule
	if existingEtcd != nil && existingEtcd.Spec.Backup.FullSnapshotSchedule != nil {
		fullSnapshotSchedule = existingEtcd.Spec.Backup.FullSnapshotSchedule
	}
	return fullSnapshotSchedule
}

func (e *etcd) handlePeerCertificates(ctx context.Context) (caSecretName, peerSecretName string, err error) {
	// TODO(timuthy): Remove this once https://github.com/gardener/etcd-backup-restore/issues/538 is resolved.
	if !e.values.HighAvailabilityEnabled {
		return
	}

	return GeneratePeerCertificates(ctx, e.secretsManager, e.values.Role, e.peerServiceDNSNames(), nil)
}

// GeneratePeerCertificates generates the peer certificates for the etcd cluster.
func GeneratePeerCertificates(
	ctx context.Context,
	secretsManager secretsmanager.Interface,
	role string,
	dnsNames []string,
	ipAddresses []net.IP,
) (string, string, error) {
	etcdPeerCASecret, found := secretsManager.Get(v1beta1constants.SecretNameCAETCDPeer)
	if !found {
		return "", "", fmt.Errorf("secret %q not found", v1beta1constants.SecretNameCAETCDPeer)
	}

	peerServerSecret, err := secretsManager.Generate(ctx, &secretsutils.CertificateSecretConfig{
		Name:                        secretNamePrefixPeerServer + role,
		CommonName:                  "etcd-server",
		DNSNames:                    dnsNames,
		IPAddresses:                 ipAddresses,
		CertType:                    secretsutils.ServerClientCert,
		SkipPublishingCACertificate: true,
	}, secretsmanager.SignedByCA(v1beta1constants.SecretNameCAETCDPeer, secretsmanager.UseCurrentCA), secretsmanager.Rotate(secretsmanager.InPlace))
	if err != nil {
		return "", "", fmt.Errorf("failed to generate secret %q: %w", secretNamePrefixPeerServer+role, err)
	}

	return etcdPeerCASecret.Name, peerServerSecret.Name, nil
}

// GenerateClientServerCertificates generates client and server certificates for the etcd cluster.
func GenerateClientServerCertificates(
	ctx context.Context,
	secretsManager secretsmanager.Interface,
	role string,
	dnsNames []string,
	ipAddresses []net.IP,
) (*corev1.Secret, *corev1.Secret, *corev1.Secret, error) {
	etcdCASecret, found := secretsManager.Get(v1beta1constants.SecretNameCAETCD)
	if !found {
		return nil, nil, nil, fmt.Errorf("secret %q not found", v1beta1constants.SecretNameCAETCD)
	}

	serverSecret, err := secretsManager.Generate(ctx, &secretsutils.CertificateSecretConfig{
		Name:                        secretNamePrefixServer + role,
		CommonName:                  "etcd-server",
		DNSNames:                    dnsNames,
		IPAddresses:                 ipAddresses,
		CertType:                    secretsutils.ServerClientCert,
		SkipPublishingCACertificate: true,
	}, secretsmanager.SignedByCA(v1beta1constants.SecretNameCAETCD), secretsmanager.Rotate(secretsmanager.InPlace))
	if err != nil {
		return nil, nil, nil, fmt.Errorf("failed to generate secret %q: %w", secretNamePrefixServer+role, err)
	}

	clientSecret, err := secretsManager.Generate(ctx, &secretsutils.CertificateSecretConfig{
		Name:                        SecretNameClient,
		CommonName:                  "etcd-client",
		CertType:                    secretsutils.ClientCert,
		SkipPublishingCACertificate: true,
	}, secretsmanager.SignedByCA(v1beta1constants.SecretNameCAETCD), secretsmanager.Rotate(secretsmanager.InPlace))
	if err != nil {
		return nil, nil, nil, fmt.Errorf("failed to generate secret %q: %w", SecretNameClient, err)
	}

	return etcdCASecret, serverSecret, clientSecret, nil
}

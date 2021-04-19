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
	"context"
	"fmt"
	"time"

	druidv1alpha1 "github.com/gardener/etcd-druid/api/v1alpha1"
	hvpav1alpha1 "github.com/gardener/hvpa-controller/api/v1alpha1"
	appsv1 "k8s.io/api/apps/v1"
	autoscalingv2beta1 "k8s.io/api/autoscaling/v2beta1"
	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/selection"
	"k8s.io/apimachinery/pkg/util/intstr"
	autoscalingv1beta2 "k8s.io/autoscaler/vertical-pod-autoscaler/pkg/apis/autoscaling.k8s.io/v1beta2"
	"k8s.io/utils/pointer"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	"github.com/gardener/gardener/pkg/operation/botanist/component"
	"github.com/gardener/gardener/pkg/utils"
	kutil "github.com/gardener/gardener/pkg/utils/kubernetes"

	"sigs.k8s.io/controller-runtime/pkg/client"
)

// Class is a string type alias for etcd classes.
type Class string

const (
	// ClassNormal is a constant for a normal etcd (without extensive metrics or higher resource settings, etc.)
	ClassNormal Class = "normal"
	// ClassImportant is a constant for an important etcd (with extensive metrics or higher resource settings, etc.).
	// Such etcds are also unsafe to evict (from the PoV of the cluster-autoscaler when trying to scale down).
	ClassImportant Class = "important"

	// SecretNameCA is the name of the secret containing the CA certificate and key for the etcd.
	SecretNameCA = v1beta1constants.SecretNameCAETCD
	// SecretNameServer is the name of the secret containing the server certificate and key for the etcd.
	SecretNameServer = "etcd-server-cert"
	// SecretNameClient is the name of the secret containing the client certificate and key for the etcd.
	SecretNameClient = "etcd-client-tls"

	// LabelAppValue is the value of a label whose key is 'app'.
	LabelAppValue = "etcd-statefulset"

	// NetworkPolicyName is the name of a network policy that allows ingress traffic to etcd from certain sources.
	NetworkPolicyName = "allow-etcd"

	portNameClient        = "client"
	portNameBackupRestore = "backuprestore"

	statefulSetNamePrefix      = "etcd"
	containerNameEtcd          = "etcd"
	containerNameBackupRestore = "backup-restore"
)

var (
	// TimeNow is a function returning the current time exposed for testing.
	TimeNow = time.Now

	// PortEtcdServer is the port exposed by etcd for server-to-server communication.
	PortEtcdServer = 2380
	// PortEtcdClient is the port exposed by etcd for client communication.
	PortEtcdClient = 2379
	// PortBackupRestore is the client port exposed by the backup-restore sidecar container.
	PortBackupRestore = 8080
)

// Name returns the name of the Etcd object for the given role.
func Name(role string) string {
	return "etcd-" + role
}

// ServiceName returns the service name for an etcd for the given role.
func ServiceName(role string) string {
	return fmt.Sprintf("etcd-%s-client", role)
}

// Etcd contains functions for a etcd deployer.
type Etcd interface {
	component.DeployWaiter
	component.MonitoringComponent
	// ServiceDNSNames returns the service DNS names for the etcd.
	ServiceDNSNames() []string
	// Snapshot triggers the backup-restore sidecar to perform a full snapshot in case backup configuration is provided.
	Snapshot(context.Context, kubernetes.PodExecutor) error
	// SetSecrets sets the secrets.
	SetSecrets(Secrets)
	// SetBackupConfig sets the backup configuration.
	SetBackupConfig(config *BackupConfig)
	// SetHVPAConfig sets the HVPA configuration.
	SetHVPAConfig(config *HVPAConfig)
}

// New creates a new instance of DeployWaiter for the Etcd.
func New(
	client client.Client,
	namespace string,
	role string,
	class Class,
	retainReplicas bool,
	storageCapacity string,
	defragmentationSchedule *string,
) Etcd {
	return &etcd{
		client:                  client,
		namespace:               namespace,
		role:                    role,
		class:                   class,
		retainReplicas:          retainReplicas,
		storageCapacity:         storageCapacity,
		defragmentationSchedule: defragmentationSchedule,
	}
}

type etcd struct {
	client                  client.Client
	namespace               string
	role                    string
	class                   Class
	retainReplicas          bool
	storageCapacity         string
	defragmentationSchedule *string

	secrets      Secrets
	backupConfig *BackupConfig
	hvpaConfig   *HVPAConfig
}

func (e *etcd) Deploy(ctx context.Context) error {
	if e.secrets.CA.Name == "" || e.secrets.CA.Checksum == "" {
		return fmt.Errorf("missing CA secret information")
	}
	if e.secrets.Server.Name == "" || e.secrets.Server.Checksum == "" {
		return fmt.Errorf("missing server secret information")
	}
	if e.secrets.Client.Name == "" || e.secrets.Client.Checksum == "" {
		return fmt.Errorf("missing client secret information")
	}

	var (
		networkPolicy = e.emptyNetworkPolicy()
		etcd          = e.emptyEtcd()
		hvpa          = e.emptyHVPA()
	)

	existingEtcd, foundEtcd, err := e.getExistingEtcd(ctx, Name(e.role))
	if err != nil {
		return err
	}

	stsName := Name(e.role)
	if foundEtcd && existingEtcd.Status.Etcd.Name != "" {
		stsName = existingEtcd.Status.Etcd.Name
	}

	existingSts, foundSts, err := e.getExistingStatefulSet(ctx, stsName)
	if err != nil {
		return err
	}

	var (
		replicas = e.computeReplicas(foundEtcd, existingEtcd)

		protocolTCP             = corev1.ProtocolTCP
		intStrPortEtcdClient    = intstr.FromInt(PortEtcdClient)
		intStrPortBackupRestore = intstr.FromInt(PortBackupRestore)

		resourcesEtcd, resourcesBackupRestore = e.computeContainerResources(foundSts, existingSts)
		quota                                 = resource.MustParse("8Gi")
		storageCapacity                       = resource.MustParse(e.storageCapacity)
		garbageCollectionPolicy               = druidv1alpha1.GarbageCollectionPolicy(druidv1alpha1.GarbageCollectionPolicyExponential)
		garbageCollectionPeriod               = metav1.Duration{Duration: 12 * time.Hour}

		annotations = map[string]string{
			"checksum/secret-etcd-ca":          e.secrets.CA.Checksum,
			"checksum/secret-etcd-server-cert": e.secrets.Server.Checksum,
			"checksum/secret-etcd-client-tls":  e.secrets.Client.Checksum,
		}
		metrics             = druidv1alpha1.Basic
		volumeClaimTemplate = Name(e.role)
		minAllowed          = corev1.ResourceList{
			corev1.ResourceCPU:    resource.MustParse("50m"),
			corev1.ResourceMemory: resource.MustParse("200M"),
		}
	)

	if e.class == ClassImportant {
		annotations["cluster-autoscaler.kubernetes.io/safe-to-evict"] = "false"
		metrics = druidv1alpha1.Extensive
		volumeClaimTemplate = e.role + "-etcd"
		minAllowed = corev1.ResourceList{
			corev1.ResourceCPU:    resource.MustParse("200m"),
			corev1.ResourceMemory: resource.MustParse("700M"),
		}
	}

	if _, err := controllerutil.CreateOrUpdate(ctx, e.client, networkPolicy, func() error {
		networkPolicy.Annotations = map[string]string{
			v1beta1constants.GardenerDescription: "Allows Ingress to etcd pods from the Shoot's Kubernetes API Server.",
		}
		networkPolicy.Labels = map[string]string{
			v1beta1constants.GardenRole: v1beta1constants.GardenRoleControlPlane,
		}
		networkPolicy.Spec.PodSelector = metav1.LabelSelector{
			MatchLabels: map[string]string{
				v1beta1constants.DeprecatedGardenRole: v1beta1constants.GardenRoleControlPlane,
				v1beta1constants.LabelApp:             LabelAppValue,
			},
		}
		networkPolicy.Spec.Ingress = []networkingv1.NetworkPolicyIngressRule{
			{
				From: []networkingv1.NetworkPolicyPeer{
					{
						PodSelector: &metav1.LabelSelector{
							// TODO: Replace below map with a function call to the to-be-introduced kubeapiserver package.
							MatchLabels: map[string]string{
								v1beta1constants.DeprecatedGardenRole: v1beta1constants.GardenRoleControlPlane,
								v1beta1constants.LabelApp:             v1beta1constants.LabelKubernetes,
								v1beta1constants.LabelRole:            v1beta1constants.LabelAPIServer,
							},
						},
					},
					{
						PodSelector: &metav1.LabelSelector{
							// TODO: Replace below map with a function call to the to-be-introduced prometheus package.
							MatchLabels: map[string]string{
								v1beta1constants.DeprecatedGardenRole: "monitoring",
								v1beta1constants.LabelApp:             "prometheus",
								v1beta1constants.LabelRole:            "monitoring",
							},
						},
					},
				},
				Ports: []networkingv1.NetworkPolicyPort{
					{
						Protocol: &protocolTCP,
						Port:     &intStrPortEtcdClient,
					},
					{
						Protocol: &protocolTCP,
						Port:     &intStrPortBackupRestore,
					},
				},
			},
		}
		networkPolicy.Spec.Egress = nil
		networkPolicy.Spec.PolicyTypes = []networkingv1.PolicyType{networkingv1.PolicyTypeIngress}
		return nil
	}); err != nil {
		return err
	}

	if _, err := controllerutil.CreateOrUpdate(ctx, e.client, etcd, func() error {
		etcd.Annotations = map[string]string{
			v1beta1constants.GardenerOperation: v1beta1constants.GardenerOperationReconcile,
			v1beta1constants.GardenerTimestamp: TimeNow().UTC().String(),
		}
		etcd.Labels = map[string]string{
			v1beta1constants.LabelRole:  e.role,
			v1beta1constants.GardenRole: v1beta1constants.GardenRoleControlPlane,
		}
		etcd.Spec.Replicas = replicas
		etcd.Spec.PriorityClassName = pointer.StringPtr(v1beta1constants.PriorityClassNameShootControlPlane)
		etcd.Spec.Annotations = annotations
		etcd.Spec.Labels = utils.MergeStringMaps(e.getLabels(), map[string]string{
			v1beta1constants.LabelApp:                            LabelAppValue,
			v1beta1constants.LabelNetworkPolicyToDNS:             v1beta1constants.LabelNetworkPolicyAllowed,
			v1beta1constants.LabelNetworkPolicyToPublicNetworks:  v1beta1constants.LabelNetworkPolicyAllowed,
			v1beta1constants.LabelNetworkPolicyToPrivateNetworks: v1beta1constants.LabelNetworkPolicyAllowed,
		})
		etcd.Spec.Selector = &metav1.LabelSelector{
			MatchLabels: utils.MergeStringMaps(e.getLabels(), map[string]string{
				v1beta1constants.LabelApp: LabelAppValue,
			}),
		}
		etcd.Spec.Etcd = druidv1alpha1.EtcdConfig{
			Resources: resourcesEtcd,
			TLS: &druidv1alpha1.TLSConfig{
				TLSCASecretRef: corev1.SecretReference{
					Name:      e.secrets.CA.Name,
					Namespace: e.namespace,
				},
				ServerTLSSecretRef: corev1.SecretReference{
					Name:      e.secrets.Server.Name,
					Namespace: e.namespace,
				},
				ClientTLSSecretRef: corev1.SecretReference{
					Name:      e.secrets.Client.Name,
					Namespace: e.namespace,
				},
			},
			ServerPort:              &PortEtcdServer,
			ClientPort:              &PortEtcdClient,
			Metrics:                 metrics,
			DefragmentationSchedule: e.computeDefragmentationSchedule(foundEtcd, existingEtcd),
			Quota:                   &quota,
		}
		etcd.Spec.Backup = druidv1alpha1.BackupSpec{
			Port:                    &PortBackupRestore,
			Resources:               resourcesBackupRestore,
			GarbageCollectionPolicy: &garbageCollectionPolicy,
			GarbageCollectionPeriod: &garbageCollectionPeriod,
		}

		if e.backupConfig != nil {
			var (
				provider                 = druidv1alpha1.StorageProvider(e.backupConfig.Provider)
				deltaSnapshotPeriod      = metav1.Duration{Duration: 5 * time.Minute}
				deltaSnapshotMemoryLimit = resource.MustParse("100Mi")
			)

			etcd.Spec.Backup.Store = &druidv1alpha1.StoreSpec{
				SecretRef: &corev1.SecretReference{Name: e.backupConfig.SecretRefName},
				Container: &e.backupConfig.Container,
				Provider:  &provider,
				Prefix:    fmt.Sprintf("%s/etcd-%s", e.backupConfig.Prefix, e.role),
			}
			etcd.Spec.Backup.FullSnapshotSchedule = e.computeFullSnapshotSchedule(foundEtcd, existingEtcd)
			etcd.Spec.Backup.DeltaSnapshotPeriod = &deltaSnapshotPeriod
			etcd.Spec.Backup.DeltaSnapshotMemoryLimit = &deltaSnapshotMemoryLimit
		}

		etcd.Spec.StorageCapacity = &storageCapacity
		etcd.Spec.VolumeClaimTemplate = &volumeClaimTemplate
		return nil
	}); err != nil {
		return err
	}

	if e.hvpaConfig != nil && e.hvpaConfig.Enabled {
		var (
			hpaLabels                   = map[string]string{v1beta1constants.LabelRole: "etcd-hpa-" + e.role}
			vpaLabels                   = map[string]string{v1beta1constants.LabelRole: "etcd-vpa-" + e.role}
			updateModeAuto              = hvpav1alpha1.UpdateModeAuto
			updateModeMaintenanceWindow = hvpav1alpha1.UpdateModeMaintenanceWindow
			containerPolicyOff          = autoscalingv1beta2.ContainerScalingModeOff
		)

		if _, err := controllerutil.CreateOrUpdate(ctx, e.client, hvpa, func() error {
			hvpa.Labels = utils.MergeStringMaps(e.getLabels(), map[string]string{
				v1beta1constants.LabelApp: LabelAppValue,
			})
			hvpa.Spec.Replicas = pointer.Int32Ptr(1)
			hvpa.Spec.MaintenanceTimeWindow = &hvpav1alpha1.MaintenanceTimeWindow{
				Begin: e.hvpaConfig.MaintenanceTimeWindow.Begin,
				End:   e.hvpaConfig.MaintenanceTimeWindow.End,
			}
			hvpa.Spec.Hpa = hvpav1alpha1.HpaSpec{
				Selector: &metav1.LabelSelector{MatchLabels: hpaLabels},
				Deploy:   false,
				Template: hvpav1alpha1.HpaTemplate{
					ObjectMeta: metav1.ObjectMeta{
						Labels: hpaLabels,
					},
					Spec: hvpav1alpha1.HpaTemplateSpec{
						MinReplicas: pointer.Int32Ptr(int32(replicas)),
						MaxReplicas: int32(replicas),
						Metrics: []autoscalingv2beta1.MetricSpec{
							{
								Type: autoscalingv2beta1.ResourceMetricSourceType,
								Resource: &autoscalingv2beta1.ResourceMetricSource{
									Name:                     corev1.ResourceCPU,
									TargetAverageUtilization: pointer.Int32Ptr(80),
								},
							},
							{
								Type: autoscalingv2beta1.ResourceMetricSourceType,
								Resource: &autoscalingv2beta1.ResourceMetricSource{
									Name:                     corev1.ResourceMemory,
									TargetAverageUtilization: pointer.Int32Ptr(80),
								},
							},
						},
					},
				},
			}
			hvpa.Spec.Vpa = hvpav1alpha1.VpaSpec{
				Selector: &metav1.LabelSelector{MatchLabels: vpaLabels},
				Deploy:   true,
				ScaleUp: hvpav1alpha1.ScaleType{
					UpdatePolicy: hvpav1alpha1.UpdatePolicy{
						UpdateMode: &updateModeAuto,
					},
					StabilizationDuration: pointer.StringPtr("5m"),
					MinChange: hvpav1alpha1.ScaleParams{
						CPU: hvpav1alpha1.ChangeParams{
							Value:      pointer.StringPtr("1"),
							Percentage: pointer.Int32Ptr(80),
						},
						Memory: hvpav1alpha1.ChangeParams{
							Value:      pointer.StringPtr("2G"),
							Percentage: pointer.Int32Ptr(80),
						},
					},
				},
				ScaleDown: hvpav1alpha1.ScaleType{
					UpdatePolicy: hvpav1alpha1.UpdatePolicy{
						UpdateMode: &updateModeMaintenanceWindow,
					},
					StabilizationDuration: pointer.StringPtr("15m"),
					MinChange: hvpav1alpha1.ScaleParams{
						CPU: hvpav1alpha1.ChangeParams{
							Value:      pointer.StringPtr("1"),
							Percentage: pointer.Int32Ptr(80),
						},
						Memory: hvpav1alpha1.ChangeParams{
							Value:      pointer.StringPtr("2G"),
							Percentage: pointer.Int32Ptr(80),
						},
					},
				},
				LimitsRequestsGapScaleParams: hvpav1alpha1.ScaleParams{
					CPU: hvpav1alpha1.ChangeParams{
						Value:      pointer.StringPtr("2"),
						Percentage: pointer.Int32Ptr(40),
					},
					Memory: hvpav1alpha1.ChangeParams{
						Value:      pointer.StringPtr("5G"),
						Percentage: pointer.Int32Ptr(40),
					},
				},
				Template: hvpav1alpha1.VpaTemplate{
					ObjectMeta: metav1.ObjectMeta{
						Labels: vpaLabels,
					},
					Spec: hvpav1alpha1.VpaTemplateSpec{
						ResourcePolicy: &autoscalingv1beta2.PodResourcePolicy{
							ContainerPolicies: []autoscalingv1beta2.ContainerResourcePolicy{
								{
									ContainerName: containerNameEtcd,
									MinAllowed:    minAllowed,
									MaxAllowed: corev1.ResourceList{
										corev1.ResourceCPU:    resource.MustParse("4"),
										corev1.ResourceMemory: resource.MustParse("30G"),
									},
								},
								{
									ContainerName: containerNameBackupRestore,
									Mode:          &containerPolicyOff,
								},
							},
						},
					},
				},
			}
			hvpa.Spec.WeightBasedScalingIntervals = []hvpav1alpha1.WeightBasedScalingInterval{
				{
					VpaWeight:         hvpav1alpha1.VpaOnly,
					StartReplicaCount: int32(replicas),
					LastReplicaCount:  int32(replicas),
				},
			}
			hvpa.Spec.TargetRef = &autoscalingv2beta1.CrossVersionObjectReference{
				APIVersion: appsv1.SchemeGroupVersion.String(),
				Kind:       "StatefulSet",
				Name:       stsName,
			}
			return nil
		}); err != nil {
			return err
		}
	} else {
		if err := kutil.DeleteObjects(ctx, e.client, e.emptyHVPA()); err != nil {
			return err
		}
	}

	return nil
}

func (e *etcd) Destroy(ctx context.Context) error {
	return kutil.DeleteObjects(
		ctx,
		e.client,
		e.emptyHVPA(),
		e.emptyEtcd(),
		e.emptyNetworkPolicy(),
	)
}

func (e *etcd) getLabels() map[string]string {
	return map[string]string{
		v1beta1constants.DeprecatedGardenRole: v1beta1constants.GardenRoleControlPlane,
		v1beta1constants.LabelRole:            e.role,
	}
}

func (e *etcd) getExistingEtcd(ctx context.Context, name string) (*druidv1alpha1.Etcd, bool, error) {
	obj, found, err := e.getExistingResource(ctx, name, &druidv1alpha1.Etcd{})
	if obj != nil {
		return obj.(*druidv1alpha1.Etcd), found, err
	}
	return nil, found, err
}

func (e *etcd) getExistingStatefulSet(ctx context.Context, name string) (*appsv1.StatefulSet, bool, error) {
	obj, found, err := e.getExistingResource(ctx, name, &appsv1.StatefulSet{})
	if obj != nil {
		return obj.(*appsv1.StatefulSet), found, err
	}
	return nil, found, err
}

func (e *etcd) getExistingResource(ctx context.Context, name string, obj client.Object) (client.Object, bool, error) {
	if err := e.client.Get(ctx, kutil.Key(e.namespace, name), obj); err != nil {
		if !apierrors.IsNotFound(err) {
			return nil, false, err
		}
		return nil, false, nil
	}
	return obj, true, nil
}

func (e *etcd) emptyNetworkPolicy() *networkingv1.NetworkPolicy {
	return &networkingv1.NetworkPolicy{ObjectMeta: metav1.ObjectMeta{Name: NetworkPolicyName, Namespace: e.namespace}}
}

func (e *etcd) emptyEtcd() *druidv1alpha1.Etcd {
	return &druidv1alpha1.Etcd{ObjectMeta: metav1.ObjectMeta{Name: Name(e.role), Namespace: e.namespace}}
}

func (e *etcd) emptyHVPA() *hvpav1alpha1.Hvpa {
	return &hvpav1alpha1.Hvpa{ObjectMeta: metav1.ObjectMeta{Name: Name(e.role), Namespace: e.namespace}}
}

func (e *etcd) Snapshot(ctx context.Context, podExecutor kubernetes.PodExecutor) error {
	if e.backupConfig == nil {
		return fmt.Errorf("no backup is configured for this etcd, cannot make a snapshot")
	}

	etcdMainSelector := e.podLabelSelector()

	podsList := &corev1.PodList{}
	if err := e.client.List(ctx, podsList, client.InNamespace(e.namespace), client.MatchingLabelsSelector{Selector: etcdMainSelector}); err != nil {
		return err
	}
	if len(podsList.Items) == 0 {
		return fmt.Errorf("didn't find any pods for selector: %v", etcdMainSelector)
	}
	if len(podsList.Items) > 1 {
		return fmt.Errorf("multiple ETCD Pods found. Pod list found: %v", podsList.Items)
	}

	_, err := podExecutor.Execute(
		e.namespace,
		podsList.Items[0].GetName(),
		containerNameBackupRestore,
		"/bin/sh",
		fmt.Sprintf("curl -k https://etcd-%s-local:%d/snapshot/full", e.role, PortBackupRestore),
	)
	return err
}

func (e *etcd) ServiceDNSNames() []string {
	return append(
		[]string{fmt.Sprintf("etcd-%s-local", e.role)},
		kutil.DNSNamesForService(fmt.Sprintf("etcd-%s-client", e.role), e.namespace)...,
	)
}

func (e *etcd) SetSecrets(secrets Secrets)                 { e.secrets = secrets }
func (e *etcd) SetBackupConfig(backupConfig *BackupConfig) { e.backupConfig = backupConfig }
func (e *etcd) SetHVPAConfig(hvpaConfig *HVPAConfig)       { e.hvpaConfig = hvpaConfig }

func (e *etcd) podLabelSelector() labels.Selector {
	app, _ := labels.NewRequirement(v1beta1constants.LabelApp, selection.Equals, []string{LabelAppValue})
	role, _ := labels.NewRequirement(v1beta1constants.LabelRole, selection.Equals, []string{e.role})
	return labels.NewSelector().Add(*role, *app)
}

func (e *etcd) computeContainerResources(foundSts bool, existingSts *appsv1.StatefulSet) (*corev1.ResourceRequirements, *corev1.ResourceRequirements) {
	var (
		resourcesEtcd = &corev1.ResourceRequirements{
			Requests: corev1.ResourceList{
				corev1.ResourceCPU:    resource.MustParse("300m"),
				corev1.ResourceMemory: resource.MustParse("1G"),
			},
			Limits: corev1.ResourceList{
				corev1.ResourceCPU:    resource.MustParse("2300m"),
				corev1.ResourceMemory: resource.MustParse("6G"),
			},
		}
		resourcesBackupRestore = &corev1.ResourceRequirements{
			Requests: corev1.ResourceList{
				corev1.ResourceCPU:    resource.MustParse("23m"),
				corev1.ResourceMemory: resource.MustParse("128Mi"),
			},
			Limits: corev1.ResourceList{
				corev1.ResourceCPU:    resource.MustParse("1"),
				corev1.ResourceMemory: resource.MustParse("10G"),
			},
		}
	)

	if foundSts && e.hvpaConfig != nil && e.hvpaConfig.Enabled {
		for k := range existingSts.Spec.Template.Spec.Containers {
			v := existingSts.Spec.Template.Spec.Containers[k]
			switch v.Name {
			case containerNameEtcd:
				resourcesEtcd = v.Resources.DeepCopy()
			case containerNameBackupRestore:
				resourcesBackupRestore = v.Resources.DeepCopy()
			}
		}
	}

	return resourcesEtcd, resourcesBackupRestore
}

func (e *etcd) computeReplicas(foundEtcd bool, existingEtcd *druidv1alpha1.Etcd) int {
	if !e.retainReplicas {
		return 1
	}

	if foundEtcd {
		return existingEtcd.Spec.Replicas
	}

	return 0
}

func (e *etcd) computeDefragmentationSchedule(foundEtcd bool, existingEtcd *druidv1alpha1.Etcd) *string {
	defragmentationSchedule := e.defragmentationSchedule
	if foundEtcd && existingEtcd.Spec.Etcd.DefragmentationSchedule != nil {
		defragmentationSchedule = existingEtcd.Spec.Etcd.DefragmentationSchedule
	}
	return defragmentationSchedule
}

func (e *etcd) computeFullSnapshotSchedule(foundEtcd bool, existingEtcd *druidv1alpha1.Etcd) *string {
	fullSnapshotSchedule := &e.backupConfig.FullSnapshotSchedule
	if foundEtcd && existingEtcd.Spec.Backup.FullSnapshotSchedule != nil {
		fullSnapshotSchedule = existingEtcd.Spec.Backup.FullSnapshotSchedule
	}
	return fullSnapshotSchedule
}

// Secrets is collection of secrets for the etcd.
type Secrets struct {
	// CA is a secret containing the CA certificate and key.
	CA component.Secret
	// Server is a secret containing the server certificate and key.
	Server component.Secret
	// Client is a secret containing the client certificate and key.
	Client component.Secret
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
}

// HVPAConfig contains information for configuring the HVPA object for the etcd.
type HVPAConfig struct {
	// Enabled states whether an HVPA object shall be deployed.
	Enabled bool
	// MaintenanceTimeWindow contains begin and end of a time window that allows down-scaling the etcd in case its
	// resource requests/limits are unnecessarily high.
	MaintenanceTimeWindow gardencorev1beta1.MaintenanceTimeWindow
}

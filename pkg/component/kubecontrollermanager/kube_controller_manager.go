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

package kubecontrollermanager

import (
	"context"
	"fmt"
	"net"
	"strings"
	"time"

	"github.com/Masterminds/semver"
	hvpav1alpha1 "github.com/gardener/hvpa-controller/api/v1alpha1"
	"github.com/go-logr/logr"
	appsv1 "k8s.io/api/apps/v1"
	autoscalingv1 "k8s.io/api/autoscaling/v1"
	autoscalingv2beta1 "k8s.io/api/autoscaling/v2beta1"
	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	policyv1 "k8s.io/api/policy/v1"
	policyv1beta1 "k8s.io/api/policy/v1beta1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	vpaautoscalingv1 "k8s.io/autoscaler/vertical-pod-autoscaler/pkg/apis/autoscaling.k8s.io/v1"
	"k8s.io/utils/pointer"
	"sigs.k8s.io/controller-runtime/pkg/client"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	resourcesv1alpha1 "github.com/gardener/gardener/pkg/apis/resources/v1alpha1"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	"github.com/gardener/gardener/pkg/component"
	kubeapiserverconstants "github.com/gardener/gardener/pkg/component/kubeapiserver/constants"
	"github.com/gardener/gardener/pkg/controllerutils"
	"github.com/gardener/gardener/pkg/utils"
	gardenerutils "github.com/gardener/gardener/pkg/utils/gardener"
	kubernetesutils "github.com/gardener/gardener/pkg/utils/kubernetes"
	"github.com/gardener/gardener/pkg/utils/managedresources"
	"github.com/gardener/gardener/pkg/utils/secrets"
	secretsmanager "github.com/gardener/gardener/pkg/utils/secrets/manager"
	versionutils "github.com/gardener/gardener/pkg/utils/version"
)

const (
	// LabelRole is a constant for the value of a label with key 'role'.
	LabelRole = "controller-manager"

	serviceName         = "kube-controller-manager"
	containerName       = v1beta1constants.DeploymentNameKubeControllerManager
	managedResourceName = "shoot-core-kube-controller-manager"
	secretNameServer    = "kube-controller-manager-server"
	portNameMetrics     = "metrics"

	volumeNameServer            = "server"
	volumeNameServiceAccountKey = "service-account-key"
	volumeNameCA                = "ca"
	volumeNameCAClient          = "ca-client"
	volumeNameCAKubelet         = "ca-kubelet"

	volumeMountPathCA                = "/srv/kubernetes/ca"
	volumeMountPathCAClient          = "/srv/kubernetes/ca-client"
	volumeMountPathCAKubelet         = "/srv/kubernetes/ca-kubelet"
	volumeMountPathServiceAccountKey = "/srv/kubernetes/service-account-key"
	volumeMountPathServer            = "/var/lib/kube-controller-manager-server"
)

// Interface contains functions for a kube-controller-manager deployer.
type Interface interface {
	component.DeployWaiter
	component.MonitoringComponent
	// SetReplicaCount sets the replica count for the kube-controller-manager.
	SetReplicaCount(replicas int32)
	// WaitForControllerToBeActive checks whether kube-controller-manager has
	// recently written to the Endpoint object holding the leader information. If yes, it is active.
	WaitForControllerToBeActive(ctx context.Context) error
	// SetShootClient sets the shoot client used to deploy resources into the Shoot API server.
	SetShootClient(c client.Client)
}

// HVPAConfig contains information for configuring the HVPA object for the etcd.
type HVPAConfig struct {
	// Enabled states whether an HVPA object shall be deployed.
	Enabled bool
	// The update mode to use for scale down.
	ScaleDownUpdateMode *string
}

// New creates a new instance of DeployWaiter for the kube-controller-manager.
func New(
	log logr.Logger,
	seedClient kubernetes.Interface,
	namespace string,
	secretsManager secretsmanager.Interface,
	values Values,
) Interface {
	return &kubeControllerManager{
		log:            log,
		seedClient:     seedClient,
		namespace:      namespace,
		secretsManager: secretsManager,
		values:         values,
	}
}

type kubeControllerManager struct {
	log            logr.Logger
	seedClient     kubernetes.Interface
	shootClient    client.Client
	namespace      string
	secretsManager secretsmanager.Interface
	values         Values
}

// Values are the values for the kube-controller-manager deployment.
type Values struct {
	// RuntimeVersion is the Kubernetes version of the runtime cluster.
	RuntimeVersion *semver.Version
	// TargetVersion is the Kubernetes version of the target cluster.
	TargetVersion *semver.Version
	// Image is the image of the kube-controller-manager.
	Image string
	// Replicas is the number of replicas for the kube-controller-manager deployment.
	Replicas int32
	// PriorityClassName is the name of the priority class.
	PriorityClassName string
	// Config is the configuration of the kube-controller-manager.
	Config *gardencorev1beta1.KubeControllerManagerConfig
	// NamePrefix is the prefix for the resource names.
	NamePrefix string
	// HVPAConfig is the configuration for HVPA.
	HVPAConfig *HVPAConfig
	// IsWorkerless specifies whether the cluster has worker nodes.
	IsWorkerless bool
	// PodNetwork is the pod CIDR of the target cluster.
	PodNetwork *net.IPNet
	// ServiceNetwork is the service CIDR of the target cluster.
	ServiceNetwork *net.IPNet
	// ClusterSigningDuration is the value for the `--cluster-signing-duration` flag.
	ClusterSigningDuration *time.Duration
	// ControllerWorkers is used for configuring the workers for controllers.
	ControllerWorkers ControllerWorkers
	// ControllerSyncPeriods is used for configuring the sync periods for controllers.
	ControllerSyncPeriods ControllerSyncPeriods
}

// ControllerWorkers is used for configuring the workers for controllers.
type ControllerWorkers struct {
	// StatefulSet is the number of workers for the StatefulSet controller.
	StatefulSet *int
	// Deployment is the number of workers for the Deployment controller.
	Deployment *int
	// ReplicaSet is the number of workers for the ReplicaSet controller.
	ReplicaSet *int
	// Endpoint is the number of workers for the Endpoint controller.
	Endpoint *int
	// GarbageCollector is the number of workers for the GarbageCollector controller.
	GarbageCollector *int
	// Namespace is the number of workers for the Namespace controller. Set it to '0' in order to disable the controller
	// (only works when cluster is workerless).
	Namespace *int
	// ResourceQuota is the number of workers for the ResourceQuota controller. Set it to '0' in order to disable the
	// controller (only works when cluster is workerless).
	ResourceQuota *int
	// ServiceEndpoint is the number of workers for the ServiceEndpoint controller.
	ServiceEndpoint *int
	// ServiceAccountToken is the number of workers for the ServiceAccountToken controller. Set it to '0' in order to
	// disable the controller (only works when cluster is workerless).
	ServiceAccountToken *int
}

// ControllerSyncPeriods is used for configuring the sync periods for controllers.
type ControllerSyncPeriods struct {
	// ResourceQuota is the sync period for the ResourceQuota controller.
	ResourceQuota *time.Duration
}

const (
	defaultControllerWorkersDeployment          = 50
	defaultControllerWorkersReplicaSet          = 50
	defaultControllerWorkersStatefulSet         = 15
	defaultControllerWorkersEndpoint            = 15
	defaultControllerWorkersGarbageCollector    = 30
	defaultControllerWorkersServiceEndpoint     = 15
	defaultControllerWorkersNamespace           = 30
	defaultControllerWorkersResourceQuota       = 15
	defaultControllerWorkersServiceAccountToken = 15
)

func (k *kubeControllerManager) Deploy(ctx context.Context) error {
	serverSecret, err := k.secretsManager.Generate(ctx, &secrets.CertificateSecretConfig{
		Name:                        secretNameServer,
		CommonName:                  k.values.NamePrefix + v1beta1constants.DeploymentNameKubeControllerManager,
		DNSNames:                    kubernetesutils.DNSNamesForService(k.values.NamePrefix+serviceName, k.namespace),
		CertType:                    secrets.ServerCert,
		SkipPublishingCACertificate: true,
	}, secretsmanager.SignedByCA(v1beta1constants.SecretNameCACluster), secretsmanager.Rotate(secretsmanager.InPlace))
	if err != nil {
		return err
	}

	secretCACluster, found := k.secretsManager.Get(v1beta1constants.SecretNameCACluster)
	if !found {
		return fmt.Errorf("secret %q not found", v1beta1constants.SecretNameCACluster)
	}

	secretCAClient, found := k.secretsManager.Get(v1beta1constants.SecretNameCAClient, secretsmanager.Current)
	if !found {
		return fmt.Errorf("secret %q not found", v1beta1constants.SecretNameCAClient)
	}

	var secretCAKubelet *corev1.Secret
	if !k.values.IsWorkerless {
		secretCAKubelet, found = k.secretsManager.Get(v1beta1constants.SecretNameCAKubelet, secretsmanager.Current)
		if !found {
			return fmt.Errorf("secret %q not found", v1beta1constants.SecretNameCAKubelet)
		}
	}

	genericTokenKubeconfigSecret, found := k.secretsManager.Get(v1beta1constants.SecretNameGenericTokenKubeconfig)
	if !found {
		return fmt.Errorf("secret %q not found", v1beta1constants.SecretNameGenericTokenKubeconfig)
	}

	serviceAccountKeySecret, found := k.secretsManager.Get(v1beta1constants.SecretNameServiceAccountKey, secretsmanager.Current)
	if !found {
		return fmt.Errorf("secret %q not found", v1beta1constants.SecretNameServiceAccountKey)
	}

	var (
		vpa                 = k.emptyVPA()
		hvpa                = k.emptyHVPA()
		service             = k.emptyService()
		shootAccessSecret   = k.newShootAccessSecret()
		deployment          = k.emptyDeployment()
		podDisruptionBudget = k.emptyPodDisruptionBudget()

		port               int32 = 10257
		probeURIScheme           = corev1.URISchemeHTTPS
		command                  = k.computeCommand(port)
		controlledValues         = vpaautoscalingv1.ContainerControlledValuesRequestsOnly
		pdbMaxUnavailable        = intstr.FromInt(1)
		hvpaResourcePolicy       = &vpaautoscalingv1.PodResourcePolicy{
			ContainerPolicies: []vpaautoscalingv1.ContainerResourcePolicy{{
				ContainerName: containerName,
				MinAllowed: corev1.ResourceList{
					corev1.ResourceMemory: resource.MustParse("100Mi"),
				},
				MaxAllowed: corev1.ResourceList{
					corev1.ResourceCPU:    resource.MustParse("4"),
					corev1.ResourceMemory: resource.MustParse("10G"),
				},
				ControlledValues: &controlledValues,
			}},
		}
		vpaResourcePolicy = &vpaautoscalingv1.PodResourcePolicy{
			ContainerPolicies: []vpaautoscalingv1.ContainerResourcePolicy{{
				ContainerName: containerName,
				MinAllowed: corev1.ResourceList{
					corev1.ResourceMemory: resource.MustParse("100Mi"),
				},
				MaxAllowed: corev1.ResourceList{
					corev1.ResourceCPU:    resource.MustParse("4"),
					corev1.ResourceMemory: resource.MustParse("10G"),
				},
				ControlledValues: &controlledValues,
			}},
		}
	)

	resourceRequirements, err := k.computeResourceRequirements(ctx)
	if err != nil {
		return err
	}

	if _, err := controllerutils.GetAndCreateOrMergePatch(ctx, k.seedClient.Client(), service, func() error {
		service.Labels = getLabels()

		utilruntime.Must(gardenerutils.InjectNetworkPolicyAnnotationsForScrapeTargets(service, networkingv1.NetworkPolicyPort{
			Port:     utils.IntStrPtrFromInt(int(port)),
			Protocol: utils.ProtocolPtr(corev1.ProtocolTCP),
		}))

		service.Spec.Selector = getLabels()
		service.Spec.Type = corev1.ServiceTypeClusterIP
		service.Spec.ClusterIP = corev1.ClusterIPNone
		desiredPorts := []corev1.ServicePort{
			{
				Name:     portNameMetrics,
				Protocol: corev1.ProtocolTCP,
				Port:     port,
			},
		}
		service.Spec.Ports = kubernetesutils.ReconcileServicePorts(service.Spec.Ports, desiredPorts, corev1.ServiceTypeClusterIP)
		return nil
	}); err != nil {
		return err
	}

	if err := shootAccessSecret.Reconcile(ctx, k.seedClient.Client()); err != nil {
		return err
	}

	if _, err := controllerutils.GetAndCreateOrMergePatch(ctx, k.seedClient.Client(), deployment, func() error {
		deployment.Labels = utils.MergeStringMaps(getLabels(), map[string]string{
			v1beta1constants.GardenRole:                  v1beta1constants.GardenRoleControlPlane,
			resourcesv1alpha1.HighAvailabilityConfigType: resourcesv1alpha1.HighAvailabilityConfigTypeController,
		})
		deployment.Spec.Replicas = &k.values.Replicas
		deployment.Spec.RevisionHistoryLimit = pointer.Int32(1)
		deployment.Spec.Selector = &metav1.LabelSelector{MatchLabels: getLabels()}
		deployment.Spec.Template = corev1.PodTemplateSpec{
			ObjectMeta: metav1.ObjectMeta{
				Labels: utils.MergeStringMaps(getLabels(), map[string]string{
					v1beta1constants.GardenRole:                 v1beta1constants.GardenRoleControlPlane,
					v1beta1constants.LabelPodMaintenanceRestart: "true",
					v1beta1constants.LabelNetworkPolicyToDNS:    v1beta1constants.LabelNetworkPolicyAllowed,
					gardenerutils.NetworkPolicyLabel(k.values.NamePrefix+v1beta1constants.DeploymentNameKubeAPIServer, kubeapiserverconstants.Port): v1beta1constants.LabelNetworkPolicyAllowed,
				}),
			},
			Spec: corev1.PodSpec{
				AutomountServiceAccountToken: pointer.Bool(false),
				PriorityClassName:            k.values.PriorityClassName,
				Containers: []corev1.Container{
					{
						Name:            containerName,
						Image:           k.values.Image,
						ImagePullPolicy: corev1.PullIfNotPresent,
						Command:         command,
						LivenessProbe: &corev1.Probe{
							ProbeHandler: corev1.ProbeHandler{
								HTTPGet: &corev1.HTTPGetAction{
									Path:   "/healthz",
									Scheme: probeURIScheme,
									Port:   intstr.FromInt(int(port)),
								},
							},
							SuccessThreshold:    1,
							FailureThreshold:    2,
							InitialDelaySeconds: 15,
							PeriodSeconds:       10,
							TimeoutSeconds:      15,
						},
						Ports: []corev1.ContainerPort{
							{
								Name:          portNameMetrics,
								ContainerPort: port,
								Protocol:      corev1.ProtocolTCP,
							},
						},
						Resources: resourceRequirements,
						VolumeMounts: []corev1.VolumeMount{
							{
								Name:      volumeNameCA,
								MountPath: volumeMountPathCA,
							},
							{
								Name:      volumeNameCAClient,
								MountPath: volumeMountPathCAClient,
							},
							{
								Name:      volumeNameServiceAccountKey,
								MountPath: volumeMountPathServiceAccountKey,
							},
							{
								Name:      volumeNameServer,
								MountPath: volumeMountPathServer,
							},
						},
					},
				},
				Volumes: []corev1.Volume{
					{
						Name: volumeNameCA,
						VolumeSource: corev1.VolumeSource{
							Secret: &corev1.SecretVolumeSource{
								SecretName: secretCACluster.Name,
							},
						},
					},
					{
						Name: volumeNameCAClient,
						VolumeSource: corev1.VolumeSource{
							Secret: &corev1.SecretVolumeSource{
								SecretName: secretCAClient.Name,
							},
						},
					},
					{
						Name: volumeNameServiceAccountKey,
						VolumeSource: corev1.VolumeSource{
							Secret: &corev1.SecretVolumeSource{
								SecretName: serviceAccountKeySecret.Name,
							},
						},
					},
					{
						Name: volumeNameServer,
						VolumeSource: corev1.VolumeSource{
							Secret: &corev1.SecretVolumeSource{
								SecretName: serverSecret.Name,
							},
						},
					},
				},
			},
		}

		if !k.values.IsWorkerless {
			deployment.Spec.Template.Spec.Containers[0].VolumeMounts = append(deployment.Spec.Template.Spec.Containers[0].VolumeMounts, corev1.VolumeMount{
				Name:      volumeNameCAKubelet,
				MountPath: volumeMountPathCAKubelet,
			})

			deployment.Spec.Template.Spec.Volumes = append(deployment.Spec.Template.Spec.Volumes, corev1.Volume{
				Name: volumeNameCAKubelet,
				VolumeSource: corev1.VolumeSource{
					Secret: &corev1.SecretVolumeSource{
						SecretName: secretCAKubelet.Name,
					},
				},
			})
		}

		utilruntime.Must(gardenerutils.InjectGenericKubeconfig(deployment, genericTokenKubeconfigSecret.Name, shootAccessSecret.Secret.Name))
		return nil
	}); err != nil {
		return err
	}

	if _, err := controllerutils.GetAndCreateOrMergePatch(ctx, k.seedClient.Client(), podDisruptionBudget, func() error {
		switch pdb := podDisruptionBudget.(type) {
		case *policyv1.PodDisruptionBudget:
			pdb.Labels = getLabels()
			pdb.Spec = policyv1.PodDisruptionBudgetSpec{
				MaxUnavailable: &pdbMaxUnavailable,
				Selector:       deployment.Spec.Selector,
			}
		case *policyv1beta1.PodDisruptionBudget:
			pdb.Labels = getLabels()
			pdb.Spec = policyv1beta1.PodDisruptionBudgetSpec{
				MaxUnavailable: &pdbMaxUnavailable,
				Selector:       deployment.Spec.Selector,
			}
		}
		return nil
	}); err != nil {
		return err
	}

	if k.values.HVPAConfig != nil && k.values.HVPAConfig.Enabled {
		if err := kubernetesutils.DeleteObject(ctx, k.seedClient.Client(), vpa); err != nil {
			return err
		}

		var (
			updateModeAuto = hvpav1alpha1.UpdateModeAuto
			vpaLabels      = map[string]string{v1beta1constants.LabelRole: "kube-controller-manager-vpa"}
		)

		scaleDownUpdateMode := k.values.HVPAConfig.ScaleDownUpdateMode
		if scaleDownUpdateMode == nil {
			scaleDownUpdateMode = pointer.String(hvpav1alpha1.UpdateModeAuto)
		}

		if _, err := controllerutils.GetAndCreateOrMergePatch(ctx, k.seedClient.Client(), hvpa, func() error {
			hvpa.Labels = utils.MergeStringMaps(
				hvpa.Labels,
				getLabels(),
				map[string]string{
					resourcesv1alpha1.HighAvailabilityConfigType: resourcesv1alpha1.HighAvailabilityConfigTypeController,
				},
			)
			hvpa.Spec.Replicas = pointer.Int32(1)
			hvpa.Spec.Hpa = hvpav1alpha1.HpaSpec{
				Deploy:   false,
				Selector: &metav1.LabelSelector{MatchLabels: getLabels()},
				Template: hvpav1alpha1.HpaTemplate{
					ObjectMeta: metav1.ObjectMeta{
						Labels: getLabels(),
					},
					Spec: hvpav1alpha1.HpaTemplateSpec{
						MinReplicas: pointer.Int32(int32(1)),
						MaxReplicas: int32(1),
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
				},
				ScaleDown: hvpav1alpha1.ScaleType{
					UpdatePolicy: hvpav1alpha1.UpdatePolicy{
						UpdateMode: scaleDownUpdateMode,
					},
				},
				Template: hvpav1alpha1.VpaTemplate{
					ObjectMeta: metav1.ObjectMeta{
						Labels: vpaLabels,
					},
					Spec: hvpav1alpha1.VpaTemplateSpec{
						ResourcePolicy: hvpaResourcePolicy,
					},
				},
			}
			hvpa.Spec.WeightBasedScalingIntervals = []hvpav1alpha1.WeightBasedScalingInterval{
				{
					VpaWeight:         hvpav1alpha1.VpaOnly,
					StartReplicaCount: 1,
					LastReplicaCount:  1,
				},
			}
			hvpa.Spec.TargetRef = &autoscalingv2beta1.CrossVersionObjectReference{
				APIVersion: appsv1.SchemeGroupVersion.String(),
				Kind:       "Deployment",
				Name:       k.values.NamePrefix + v1beta1constants.DeploymentNameKubeControllerManager,
			}
			return nil
		}); err != nil {
			return err
		}
	} else {
		if err := kubernetesutils.DeleteObject(ctx, k.seedClient.Client(), hvpa); err != nil {
			return err
		}

		vpaUpdateMode := vpaautoscalingv1.UpdateModeAuto

		if _, err := controllerutils.GetAndCreateOrMergePatch(ctx, k.seedClient.Client(), vpa, func() error {
			vpa.Spec.TargetRef = &autoscalingv1.CrossVersionObjectReference{
				APIVersion: appsv1.SchemeGroupVersion.String(),
				Kind:       "Deployment",
				Name:       k.values.NamePrefix + v1beta1constants.DeploymentNameKubeControllerManager,
			}
			vpa.Spec.UpdatePolicy = &vpaautoscalingv1.PodUpdatePolicy{
				UpdateMode: &vpaUpdateMode,
			}
			vpa.Spec.ResourcePolicy = vpaResourcePolicy
			return nil
		}); err != nil {
			return err
		}
	}

	return k.reconcileShootResources(ctx, shootAccessSecret.ServiceAccountName)
}

func (k *kubeControllerManager) Destroy(ctx context.Context) error {
	return kubernetesutils.DeleteObjects(ctx, k.seedClient.Client(),
		k.emptyManagedResource(),
		k.emptyManagedResourceSecret(),
		k.emptyVPA(),
		k.emptyHVPA(),
		k.emptyService(),
		k.emptyPodDisruptionBudget(),
		k.emptyDeployment(),
		k.newShootAccessSecret().Secret,
	)
}

func (k *kubeControllerManager) SetShootClient(c client.Client) { k.shootClient = c }
func (k *kubeControllerManager) SetReplicaCount(replicas int32) { k.values.Replicas = replicas }

func (k *kubeControllerManager) emptyVPA() *vpaautoscalingv1.VerticalPodAutoscaler {
	return &vpaautoscalingv1.VerticalPodAutoscaler{ObjectMeta: metav1.ObjectMeta{Name: k.values.NamePrefix + "kube-controller-manager-vpa", Namespace: k.namespace}}
}

func (k *kubeControllerManager) emptyHVPA() *hvpav1alpha1.Hvpa {
	return &hvpav1alpha1.Hvpa{ObjectMeta: metav1.ObjectMeta{Name: k.values.NamePrefix + v1beta1constants.DeploymentNameKubeControllerManager, Namespace: k.namespace}}
}

func (k *kubeControllerManager) emptyService() *corev1.Service {
	return &corev1.Service{ObjectMeta: metav1.ObjectMeta{Name: k.values.NamePrefix + serviceName, Namespace: k.namespace}}
}

func (k *kubeControllerManager) emptyDeployment() *appsv1.Deployment {
	return &appsv1.Deployment{ObjectMeta: metav1.ObjectMeta{Name: k.values.NamePrefix + v1beta1constants.DeploymentNameKubeControllerManager, Namespace: k.namespace}}
}

func (k *kubeControllerManager) emptyPodDisruptionBudget() client.Object {
	objectMeta := metav1.ObjectMeta{Name: k.values.NamePrefix + v1beta1constants.DeploymentNameKubeControllerManager, Namespace: k.namespace}

	if versionutils.ConstraintK8sGreaterEqual121.Check(k.values.RuntimeVersion) {
		return &policyv1.PodDisruptionBudget{ObjectMeta: objectMeta}
	}
	return &policyv1beta1.PodDisruptionBudget{ObjectMeta: objectMeta}
}

func (k *kubeControllerManager) newShootAccessSecret() *gardenerutils.ShootAccessSecret {
	return gardenerutils.NewShootAccessSecret(v1beta1constants.DeploymentNameKubeControllerManager, k.namespace)
}

func (k *kubeControllerManager) emptyManagedResource() *resourcesv1alpha1.ManagedResource {
	return &resourcesv1alpha1.ManagedResource{ObjectMeta: metav1.ObjectMeta{Name: managedResourceName, Namespace: k.namespace}}
}

func (k *kubeControllerManager) emptyManagedResourceSecret() *corev1.Secret {
	return &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: managedresources.SecretName(managedResourceName, true), Namespace: k.namespace}}
}

func getLabels() map[string]string {
	return map[string]string{
		v1beta1constants.LabelApp:  v1beta1constants.LabelKubernetes,
		v1beta1constants.LabelRole: LabelRole,
	}
}

func (k *kubeControllerManager) computeCommand(port int32) []string {
	var (
		defaultHorizontalPodAutoscalerConfig = k.getHorizontalPodAutoscalerConfig()
		podEvictionTimeout                   = metav1.Duration{Duration: 2 * time.Minute}
		nodeMonitorGracePeriod               = metav1.Duration{Duration: 2 * time.Minute}
		command                              = []string{
			"/usr/local/bin/kube-controller-manager",
			"--authentication-kubeconfig=" + gardenerutils.PathGenericKubeconfig,
			"--authorization-kubeconfig=" + gardenerutils.PathGenericKubeconfig,
			"--kubeconfig=" + gardenerutils.PathGenericKubeconfig,
		}
	)

	if versionutils.ConstraintK8sGreaterEqual127.Check(k.values.TargetVersion) {
		nodeMonitorGracePeriod = metav1.Duration{Duration: 40 * time.Second}
	}

	if !k.values.IsWorkerless {
		if v := k.values.Config.NodeMonitorGracePeriod; v != nil {
			nodeMonitorGracePeriod = *v
		}
		if k.values.Config.NodeCIDRMaskSize != nil {
			command = append(command, fmt.Sprintf("--node-cidr-mask-size=%d", *k.values.Config.NodeCIDRMaskSize))
		}

		command = append(command,
			"--allocate-node-cidrs=true",
			"--attach-detach-reconcile-sync-period=1m0s",
			"--controllers=*,bootstrapsigner,tokencleaner",
			fmt.Sprintf("--cluster-cidr=%s", k.values.PodNetwork.String()),
			fmt.Sprintf("--cluster-signing-kubelet-client-cert-file=%s/%s", volumeMountPathCAClient, secrets.DataKeyCertificateCA),
			fmt.Sprintf("--cluster-signing-kubelet-client-key-file=%s/%s", volumeMountPathCAClient, secrets.DataKeyPrivateKeyCA),
			fmt.Sprintf("--cluster-signing-kubelet-serving-cert-file=%s/%s", volumeMountPathCAKubelet, secrets.DataKeyCertificateCA),
			fmt.Sprintf("--cluster-signing-kubelet-serving-key-file=%s/%s", volumeMountPathCAKubelet, secrets.DataKeyPrivateKeyCA),
			fmt.Sprintf("--horizontal-pod-autoscaler-downscale-stabilization=%s", defaultHorizontalPodAutoscalerConfig.DownscaleStabilization.Duration.String()),
			fmt.Sprintf("--horizontal-pod-autoscaler-initial-readiness-delay=%s", defaultHorizontalPodAutoscalerConfig.InitialReadinessDelay.Duration.String()),
			fmt.Sprintf("--horizontal-pod-autoscaler-cpu-initialization-period=%s", defaultHorizontalPodAutoscalerConfig.CPUInitializationPeriod.Duration.String()),
			fmt.Sprintf("--horizontal-pod-autoscaler-sync-period=%s", defaultHorizontalPodAutoscalerConfig.SyncPeriod.Duration.String()),
			fmt.Sprintf("--horizontal-pod-autoscaler-tolerance=%v", *defaultHorizontalPodAutoscalerConfig.Tolerance),
			"--leader-elect=true",
			fmt.Sprintf("--node-monitor-grace-period=%s", nodeMonitorGracePeriod.Duration),
		)

		if versionutils.ConstraintK8sLess127.Check(k.values.TargetVersion) {
			if v := k.values.Config.PodEvictionTimeout; v != nil {
				podEvictionTimeout = *v
			}

			command = append(command, fmt.Sprintf("--pod-eviction-timeout=%s", podEvictionTimeout.Duration))
		}

		command = append(command,
			fmt.Sprintf("--concurrent-deployment-syncs=%d", pointer.IntDeref(k.values.ControllerWorkers.Deployment, defaultControllerWorkersDeployment)),
			fmt.Sprintf("--concurrent-replicaset-syncs=%d", pointer.IntDeref(k.values.ControllerWorkers.ReplicaSet, defaultControllerWorkersReplicaSet)),
			fmt.Sprintf("--concurrent-statefulset-syncs=%d", pointer.IntDeref(k.values.ControllerWorkers.StatefulSet, defaultControllerWorkersStatefulSet)),
		)
	} else {
		var controllers []string

		if v := pointer.IntDeref(k.values.ControllerWorkers.Namespace, defaultControllerWorkersNamespace); v != 0 {
			controllers = append(controllers, "namespace")
		}

		controllers = append(controllers, "serviceaccount")

		if v := pointer.IntDeref(k.values.ControllerWorkers.ServiceAccountToken, defaultControllerWorkersServiceAccountToken); v != 0 {
			controllers = append(controllers, "serviceaccount-token")
		}

		controllers = append(controllers,
			"clusterrole-aggregation",
			"garbagecollector",
			"csrapproving",
			"csrcleaner",
			"csrsigning",
			"bootstrapsigner",
			"tokencleaner",
		)

		if v := pointer.IntDeref(k.values.ControllerWorkers.ResourceQuota, defaultControllerWorkersResourceQuota); v != 0 {
			controllers = append(controllers, "resourcequota")
		}

		command = append(command, "--controllers="+strings.Join(controllers, ","))
	}

	command = append(command,
		fmt.Sprintf("--cluster-name=%s", k.namespace),
		fmt.Sprintf("--cluster-signing-kube-apiserver-client-cert-file=%s/%s", volumeMountPathCAClient, secrets.DataKeyCertificateCA),
		fmt.Sprintf("--cluster-signing-kube-apiserver-client-key-file=%s/%s", volumeMountPathCAClient, secrets.DataKeyPrivateKeyCA),
		fmt.Sprintf("--cluster-signing-legacy-unknown-cert-file=%s/%s", volumeMountPathCAClient, secrets.DataKeyCertificateCA),
		fmt.Sprintf("--cluster-signing-legacy-unknown-key-file=%s/%s", volumeMountPathCAClient, secrets.DataKeyPrivateKeyCA),
		"--cluster-signing-duration="+pointer.DurationDeref(k.values.ClusterSigningDuration, 720*time.Hour).String(),
		fmt.Sprintf("--concurrent-endpoint-syncs=%d", pointer.IntDeref(k.values.ControllerWorkers.Endpoint, defaultControllerWorkersEndpoint)),
		fmt.Sprintf("--concurrent-gc-syncs=%d", pointer.IntDeref(k.values.ControllerWorkers.GarbageCollector, defaultControllerWorkersGarbageCollector)),
		fmt.Sprintf("--concurrent-service-endpoint-syncs=%d", pointer.IntDeref(k.values.ControllerWorkers.ServiceEndpoint, defaultControllerWorkersServiceEndpoint)),
	)

	if v := pointer.IntDeref(k.values.ControllerWorkers.Namespace, defaultControllerWorkersNamespace); v != 0 {
		command = append(command, fmt.Sprintf("--concurrent-namespace-syncs=%d", v))
	}

	if v := pointer.IntDeref(k.values.ControllerWorkers.ResourceQuota, defaultControllerWorkersResourceQuota); v != 0 {
		command = append(command, fmt.Sprintf("--concurrent-resource-quota-syncs=%d", v))
		if k.values.ControllerSyncPeriods.ResourceQuota != nil {
			command = append(command, "--resource-quota-sync-period="+k.values.ControllerSyncPeriods.ResourceQuota.String())
		}
	}

	if v := pointer.IntDeref(k.values.ControllerWorkers.ServiceAccountToken, defaultControllerWorkersServiceAccountToken); v != 0 {
		command = append(command, fmt.Sprintf("--concurrent-serviceaccount-token-syncs=%d", v))
	}

	if k.values.Config != nil && len(k.values.Config.FeatureGates) > 0 {
		command = append(command, kubernetesutils.FeatureGatesToCommandLineParameter(k.values.Config.FeatureGates))
	}

	if versionutils.ConstraintK8sLess124.Check(k.values.TargetVersion) {
		command = append(command, "--port=0")
	}

	command = append(command,
		fmt.Sprintf("--root-ca-file=%s/%s", volumeMountPathCA, secrets.DataKeyCertificateBundle),
		fmt.Sprintf("--service-account-private-key-file=%s/%s", volumeMountPathServiceAccountKey, secrets.DataKeyRSAPrivateKey),
		fmt.Sprintf("--secure-port=%d", port),
	)

	if k.values.ServiceNetwork != nil {
		command = append(command,
			fmt.Sprintf("--service-cluster-ip-range=%s", k.values.ServiceNetwork.String()),
		)
	}

	command = append(command,
		"--profiling=false",
		fmt.Sprintf("--tls-cert-file=%s/%s", volumeMountPathServer, secrets.DataKeyCertificate),
		fmt.Sprintf("--tls-private-key-file=%s/%s", volumeMountPathServer, secrets.DataKeyPrivateKey),
		fmt.Sprintf("--tls-cipher-suites=%s", strings.Join(kubernetesutils.TLSCipherSuites(k.values.TargetVersion), ",")),
		"--use-service-account-credentials=true",
		"--v=2",
	)

	return command
}

func (k *kubeControllerManager) getHorizontalPodAutoscalerConfig() gardencorev1beta1.HorizontalPodAutoscalerConfig {
	defaultHPATolerance := gardencorev1beta1.DefaultHPATolerance
	horizontalPodAutoscalerConfig := gardencorev1beta1.HorizontalPodAutoscalerConfig{
		CPUInitializationPeriod: &metav1.Duration{Duration: gardencorev1beta1.DefaultCPUInitializationPeriod},
		DownscaleStabilization:  &metav1.Duration{Duration: gardencorev1beta1.DefaultDownscaleStabilization},
		InitialReadinessDelay:   &metav1.Duration{Duration: gardencorev1beta1.DefaultInitialReadinessDelay},
		SyncPeriod:              &metav1.Duration{Duration: gardencorev1beta1.DefaultHPASyncPeriod},
		Tolerance:               &defaultHPATolerance,
	}

	if k.values.Config != nil && k.values.Config.HorizontalPodAutoscalerConfig != nil {
		if v := k.values.Config.HorizontalPodAutoscalerConfig.CPUInitializationPeriod; v != nil {
			horizontalPodAutoscalerConfig.CPUInitializationPeriod = v
		}
		if v := k.values.Config.HorizontalPodAutoscalerConfig.DownscaleStabilization; v != nil {
			horizontalPodAutoscalerConfig.DownscaleStabilization = v
		}
		if v := k.values.Config.HorizontalPodAutoscalerConfig.InitialReadinessDelay; v != nil {
			horizontalPodAutoscalerConfig.InitialReadinessDelay = v
		}
		if v := k.values.Config.HorizontalPodAutoscalerConfig.SyncPeriod; v != nil {
			horizontalPodAutoscalerConfig.SyncPeriod = v
		}
		if v := k.values.Config.HorizontalPodAutoscalerConfig.Tolerance; v != nil {
			horizontalPodAutoscalerConfig.Tolerance = v
		}
	}
	return horizontalPodAutoscalerConfig
}

func (k *kubeControllerManager) computeResourceRequirements(ctx context.Context) (corev1.ResourceRequirements, error) {
	defaultResources := corev1.ResourceRequirements{
		Requests: corev1.ResourceList{
			corev1.ResourceCPU:    resource.MustParse("100m"),
			corev1.ResourceMemory: resource.MustParse("128Mi"),
		},
	}

	if k.values.HVPAConfig == nil || !k.values.HVPAConfig.Enabled {
		return defaultResources, nil
	}

	existingDeployment := k.emptyDeployment()
	if err := k.seedClient.Client().Get(ctx, client.ObjectKeyFromObject(existingDeployment), existingDeployment); err != nil {
		if !apierrors.IsNotFound(err) {
			return corev1.ResourceRequirements{}, err
		}
		return defaultResources, nil // Deployment was not found, hence, use the default resources
	}

	if len(existingDeployment.Spec.Template.Spec.Containers) > 0 {
		// Copy requests only, effectively removing limits
		return corev1.ResourceRequirements{Requests: existingDeployment.Spec.Template.Spec.Containers[0].Resources.Requests}, nil
	}

	return defaultResources, nil
}

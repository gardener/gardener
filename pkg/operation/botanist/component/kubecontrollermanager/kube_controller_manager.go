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

package kubecontrollermanager

import (
	"context"
	"fmt"
	"net"
	"strings"
	"time"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	resourcesv1alpha1 "github.com/gardener/gardener/pkg/apis/resources/v1alpha1"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	"github.com/gardener/gardener/pkg/controllerutils"
	"github.com/gardener/gardener/pkg/operation/botanist/component"
	"github.com/gardener/gardener/pkg/utils"
	gutil "github.com/gardener/gardener/pkg/utils/gardener"
	kutil "github.com/gardener/gardener/pkg/utils/kubernetes"
	"github.com/gardener/gardener/pkg/utils/managedresources"
	"github.com/gardener/gardener/pkg/utils/secrets"
	secretsmanager "github.com/gardener/gardener/pkg/utils/secrets/manager"
	"github.com/gardener/gardener/pkg/utils/version"
	"github.com/go-logr/logr"

	"github.com/Masterminds/semver"
	hvpav1alpha1 "github.com/gardener/hvpa-controller/api/v1alpha1"
	appsv1 "k8s.io/api/apps/v1"
	autoscalingv1 "k8s.io/api/autoscaling/v1"
	autoscalingv2beta1 "k8s.io/api/autoscaling/v2beta1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	vpaautoscalingv1 "k8s.io/autoscaler/vertical-pod-autoscaler/pkg/apis/autoscaling.k8s.io/v1"
	"k8s.io/utils/pointer"
	"sigs.k8s.io/controller-runtime/pkg/client"
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

	volumeMountPathCA                = "/srv/kubernetes/ca"
	volumeMountPathCAClient          = "/srv/kubernetes/ca-client"
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
	version *semver.Version,
	image string,
	config *gardencorev1beta1.KubeControllerManagerConfig,
	podNetwork *net.IPNet,
	serviceNetwork *net.IPNet,
	hvpaConfig *HVPAConfig,
) Interface {
	return &kubeControllerManager{
		log:            log,
		seedClient:     seedClient,
		namespace:      namespace,
		secretsManager: secretsManager,
		version:        version,
		image:          image,
		config:         config,
		podNetwork:     podNetwork,
		serviceNetwork: serviceNetwork,
		hvpaConfig:     hvpaConfig,
	}
}

type kubeControllerManager struct {
	log            logr.Logger
	seedClient     kubernetes.Interface
	shootClient    client.Client
	namespace      string
	secretsManager secretsmanager.Interface
	version        *semver.Version
	image          string
	replicas       int32
	config         *gardencorev1beta1.KubeControllerManagerConfig
	podNetwork     *net.IPNet
	serviceNetwork *net.IPNet
	hvpaConfig     *HVPAConfig
}

func (k *kubeControllerManager) Deploy(ctx context.Context) error {
	serverSecret, err := k.secretsManager.Generate(ctx, &secrets.CertificateSecretConfig{
		Name:                        secretNameServer,
		CommonName:                  v1beta1constants.DeploymentNameKubeControllerManager,
		DNSNames:                    kutil.DNSNamesForService(serviceName, k.namespace),
		CertType:                    secrets.ServerCert,
		SkipPublishingCACertificate: true,
	}, secretsmanager.SignedByCA(v1beta1constants.SecretNameCACluster), secretsmanager.Rotate(secretsmanager.InPlace))
	if err != nil {
		return err
	}

	clusterCASecret, found := k.secretsManager.Get(v1beta1constants.SecretNameCACluster)
	if !found {
		return fmt.Errorf("secret %q not found", v1beta1constants.SecretNameCACluster)
	}

	clientCASecret, found := k.secretsManager.Get(v1beta1constants.SecretNameCAClient, secretsmanager.Current)
	if !found {
		return fmt.Errorf("secret %q not found", v1beta1constants.SecretNameCAClient)
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
		vpa               = k.emptyVPA()
		hvpa              = k.emptyHVPA()
		service           = k.emptyService()
		shootAccessSecret = k.newShootAccessSecret()
		deployment        = k.emptyDeployment()

		port               int32 = 10257
		probeURIScheme           = corev1.URISchemeHTTPS
		command                  = k.computeCommand(port)
		controlledValues         = vpaautoscalingv1.ContainerControlledValuesRequestsOnly
		hvpaResourcePolicy       = &vpaautoscalingv1.PodResourcePolicy{
			ContainerPolicies: []vpaautoscalingv1.ContainerResourcePolicy{{
				ContainerName: containerName,
				MinAllowed: corev1.ResourceList{
					corev1.ResourceCPU:    resource.MustParse("100m"),
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
					corev1.ResourceCPU:    resource.MustParse("100m"),
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
		service.Spec.Ports = kutil.ReconcileServicePorts(service.Spec.Ports, desiredPorts, corev1.ServiceTypeClusterIP)
		return nil
	}); err != nil {
		return err
	}

	if err := shootAccessSecret.Reconcile(ctx, k.seedClient.Client()); err != nil {
		return err
	}

	if _, err := controllerutils.GetAndCreateOrMergePatch(ctx, k.seedClient.Client(), deployment, func() error {
		deployment.Labels = utils.MergeStringMaps(getLabels(), map[string]string{
			v1beta1constants.GardenRole: v1beta1constants.GardenRoleControlPlane,
		})
		deployment.Spec.Replicas = &k.replicas
		deployment.Spec.RevisionHistoryLimit = pointer.Int32(1)
		deployment.Spec.Selector = &metav1.LabelSelector{MatchLabels: getLabels()}
		deployment.Spec.Template = corev1.PodTemplateSpec{
			ObjectMeta: metav1.ObjectMeta{
				Labels: utils.MergeStringMaps(getLabels(), map[string]string{
					v1beta1constants.GardenRole:                         v1beta1constants.GardenRoleControlPlane,
					v1beta1constants.LabelPodMaintenanceRestart:         "true",
					v1beta1constants.LabelNetworkPolicyToDNS:            v1beta1constants.LabelNetworkPolicyAllowed,
					v1beta1constants.LabelNetworkPolicyToShootAPIServer: v1beta1constants.LabelNetworkPolicyAllowed,
					v1beta1constants.LabelNetworkPolicyFromPrometheus:   v1beta1constants.LabelNetworkPolicyAllowed,
				}),
			},
			Spec: corev1.PodSpec{
				AutomountServiceAccountToken: pointer.Bool(false),
				PriorityClassName:            v1beta1constants.PriorityClassNameShootControlPlane300,
				Containers: []corev1.Container{
					{
						Name:            containerName,
						Image:           k.image,
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
								SecretName: clusterCASecret.Name,
							},
						},
					},
					{
						Name: volumeNameCAClient,
						VolumeSource: corev1.VolumeSource{
							Secret: &corev1.SecretVolumeSource{
								SecretName: clientCASecret.Name,
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

		utilruntime.Must(gutil.InjectGenericKubeconfig(deployment, genericTokenKubeconfigSecret.Name, shootAccessSecret.Secret.Name))
		return nil
	}); err != nil {
		return err
	}

	if k.hvpaConfig != nil && k.hvpaConfig.Enabled {
		if err := kutil.DeleteObject(ctx, k.seedClient.Client(), vpa); err != nil {
			return err
		}

		var (
			updateModeAuto = hvpav1alpha1.UpdateModeAuto
			vpaLabels      = map[string]string{v1beta1constants.LabelRole: "kube-controller-manager-vpa"}
		)

		scaleDownUpdateMode := k.hvpaConfig.ScaleDownUpdateMode
		if scaleDownUpdateMode == nil {
			scaleDownUpdateMode = pointer.String(hvpav1alpha1.UpdateModeAuto)
		}

		if _, err := controllerutils.GetAndCreateOrMergePatch(ctx, k.seedClient.Client(), hvpa, func() error {
			hvpa.Labels = getLabels()
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
				Name:       v1beta1constants.DeploymentNameKubeControllerManager,
			}
			return nil
		}); err != nil {
			return err
		}
	} else {
		if err := kutil.DeleteObject(ctx, k.seedClient.Client(), hvpa); err != nil {
			return err
		}

		vpaUpdateMode := vpaautoscalingv1.UpdateModeAuto

		if _, err := controllerutils.GetAndCreateOrMergePatch(ctx, k.seedClient.Client(), vpa, func() error {
			vpa.Spec.TargetRef = &autoscalingv1.CrossVersionObjectReference{
				APIVersion: appsv1.SchemeGroupVersion.String(),
				Kind:       "Deployment",
				Name:       v1beta1constants.DeploymentNameKubeControllerManager,
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

	if err := k.reconcileShootResources(ctx, shootAccessSecret.ServiceAccountName); err != nil {
		return err
	}

	// TODO(rfranzke): Remove in a future release.
	return kutil.DeleteObject(ctx, k.seedClient.Client(), &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: "kube-controller-manager-server", Namespace: k.namespace}})
}

func (k *kubeControllerManager) SetShootClient(c client.Client)  { k.shootClient = c }
func (k *kubeControllerManager) SetReplicaCount(replicas int32)  { k.replicas = replicas }
func (k *kubeControllerManager) Destroy(_ context.Context) error { return nil }

func (k *kubeControllerManager) emptyVPA() *vpaautoscalingv1.VerticalPodAutoscaler {
	return &vpaautoscalingv1.VerticalPodAutoscaler{ObjectMeta: metav1.ObjectMeta{Name: "kube-controller-manager-vpa", Namespace: k.namespace}}
}

func (k *kubeControllerManager) emptyHVPA() *hvpav1alpha1.Hvpa {
	return &hvpav1alpha1.Hvpa{ObjectMeta: metav1.ObjectMeta{Name: v1beta1constants.DeploymentNameKubeControllerManager, Namespace: k.namespace}}
}

func (k *kubeControllerManager) emptyService() *corev1.Service {
	return &corev1.Service{ObjectMeta: metav1.ObjectMeta{Name: serviceName, Namespace: k.namespace}}
}

func (k *kubeControllerManager) emptyDeployment() *appsv1.Deployment {
	return &appsv1.Deployment{ObjectMeta: metav1.ObjectMeta{Name: v1beta1constants.DeploymentNameKubeControllerManager, Namespace: k.namespace}}
}

func (k *kubeControllerManager) newShootAccessSecret() *gutil.ShootAccessSecret {
	return gutil.NewShootAccessSecret(v1beta1constants.DeploymentNameKubeControllerManager, k.namespace)
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
		command                              []string
		defaultHorizontalPodAutoscalerConfig = k.getHorizontalPodAutoscalerConfig()
	)

	command = append(command,
		"/usr/local/bin/kube-controller-manager",
		"--allocate-node-cidrs=true",
		"--attach-detach-reconcile-sync-period=1m0s",
		"--controllers=*,bootstrapsigner,tokencleaner",
		"--authentication-kubeconfig="+gutil.PathGenericKubeconfig,
		"--authorization-kubeconfig="+gutil.PathGenericKubeconfig,
		"--kubeconfig="+gutil.PathGenericKubeconfig,
	)

	if k.config.NodeCIDRMaskSize != nil {
		command = append(command, fmt.Sprintf("--node-cidr-mask-size=%d", *k.config.NodeCIDRMaskSize))
	}

	command = append(command,
		fmt.Sprintf("--cluster-cidr=%s", k.podNetwork.String()),
		fmt.Sprintf("--cluster-name=%s", k.namespace),
		fmt.Sprintf("--cluster-signing-cert-file=%s/%s", volumeMountPathCAClient, secrets.DataKeyCertificateCA),
		fmt.Sprintf("--cluster-signing-key-file=%s/%s", volumeMountPathCAClient, secrets.DataKeyPrivateKeyCA),
	)

	if version.ConstraintK8sGreaterEqual119.Check(k.version) {
		command = append(command, "--cluster-signing-duration=720h")
	} else {
		command = append(command, "--experimental-cluster-signing-duration=720h")
	}

	command = append(command,
		"--concurrent-deployment-syncs=50",
		"--concurrent-endpoint-syncs=15",
		"--concurrent-gc-syncs=30",
		"--concurrent-namespace-syncs=50",
		"--concurrent-replicaset-syncs=50",
		"--concurrent-resource-quota-syncs=15",
		"--concurrent-service-endpoint-syncs=15",
		"--concurrent-statefulset-syncs=15",
		"--concurrent-serviceaccount-token-syncs=15",
	)

	if len(k.config.FeatureGates) > 0 {
		command = append(command, kutil.FeatureGatesToCommandLineParameter(k.config.FeatureGates))
	}

	podEvictionTimeout := metav1.Duration{Duration: 2 * time.Minute}
	if v := k.config.PodEvictionTimeout; v != nil {
		podEvictionTimeout = *v
	}

	nodeMonitorGracePeriod := metav1.Duration{Duration: 2 * time.Minute}
	if v := k.config.NodeMonitorGracePeriod; v != nil {
		nodeMonitorGracePeriod = *v
	}

	command = append(command,
		fmt.Sprintf("--horizontal-pod-autoscaler-sync-period=%s", defaultHorizontalPodAutoscalerConfig.SyncPeriod.Duration.String()),
		fmt.Sprintf("--horizontal-pod-autoscaler-tolerance=%v", *defaultHorizontalPodAutoscalerConfig.Tolerance),
		"--leader-elect=true",
		fmt.Sprintf("--node-monitor-grace-period=%s", nodeMonitorGracePeriod.Duration),
		fmt.Sprintf("--pod-eviction-timeout=%s", podEvictionTimeout.Duration),
		fmt.Sprintf("--root-ca-file=%s/%s", volumeMountPathCA, secrets.DataKeyCertificateBundle),
		fmt.Sprintf("--service-account-private-key-file=%s/%s", volumeMountPathServiceAccountKey, secrets.DataKeyRSAPrivateKey),
		fmt.Sprintf("--service-cluster-ip-range=%s", k.serviceNetwork.String()),
		fmt.Sprintf("--secure-port=%d", port),
	)

	if version.ConstraintK8sLess124.Check(k.version) {
		command = append(command, "--port=0")
	}

	command = append(command,
		fmt.Sprintf("--horizontal-pod-autoscaler-downscale-stabilization=%s", defaultHorizontalPodAutoscalerConfig.DownscaleStabilization.Duration.String()),
		fmt.Sprintf("--horizontal-pod-autoscaler-initial-readiness-delay=%s", defaultHorizontalPodAutoscalerConfig.InitialReadinessDelay.Duration.String()),
		fmt.Sprintf("--horizontal-pod-autoscaler-cpu-initialization-period=%s", defaultHorizontalPodAutoscalerConfig.CPUInitializationPeriod.Duration.String()),
		fmt.Sprintf("--tls-cert-file=%s/%s", volumeMountPathServer, secrets.DataKeyCertificate),
		fmt.Sprintf("--tls-private-key-file=%s/%s", volumeMountPathServer, secrets.DataKeyPrivateKey),
		fmt.Sprintf("--tls-cipher-suites=%s", strings.Join(kutil.TLSCipherSuites(k.version), ",")),
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

	if k.config.HorizontalPodAutoscalerConfig != nil {
		if v := k.config.HorizontalPodAutoscalerConfig.CPUInitializationPeriod; v != nil {
			horizontalPodAutoscalerConfig.CPUInitializationPeriod = v
		}
		if v := k.config.HorizontalPodAutoscalerConfig.DownscaleStabilization; v != nil {
			horizontalPodAutoscalerConfig.DownscaleStabilization = v
		}
		if v := k.config.HorizontalPodAutoscalerConfig.InitialReadinessDelay; v != nil {
			horizontalPodAutoscalerConfig.InitialReadinessDelay = v
		}
		if v := k.config.HorizontalPodAutoscalerConfig.SyncPeriod; v != nil {
			horizontalPodAutoscalerConfig.SyncPeriod = v
		}
		if v := k.config.HorizontalPodAutoscalerConfig.Tolerance; v != nil {
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

	if k.hvpaConfig == nil || !k.hvpaConfig.Enabled {
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

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
	"time"

	"github.com/sirupsen/logrus"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	"github.com/gardener/gardener/pkg/operation/botanist/component"
	"github.com/gardener/gardener/pkg/operation/common"
	"github.com/gardener/gardener/pkg/utils"
	kutil "github.com/gardener/gardener/pkg/utils/kubernetes"
	"github.com/gardener/gardener/pkg/utils/secrets"

	"github.com/Masterminds/semver"
	resourcesv1alpha1 "github.com/gardener/gardener-resource-manager/pkg/apis/resources/v1alpha1"
	appsv1 "k8s.io/api/apps/v1"
	autoscalingv1 "k8s.io/api/autoscaling/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	autoscalingv1beta2 "k8s.io/autoscaler/vertical-pod-autoscaler/pkg/apis/autoscaling.k8s.io/v1beta2"
	"k8s.io/utils/pointer"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

const (
	// ServiceName is the name of the service of the kube-controller-manager.
	ServiceName   = "kube-controller-manager"
	containerName = v1beta1constants.DeploymentNameKubeControllerManager

	// SecretName is a constant for the secret name for the kube-controller-manager's kubeconfig secret.
	SecretName = "kube-controller-manager"
	// SecretNameServer is the name of the kube-controller-manager server certificate secret.
	SecretNameServer = "kube-controller-manager-server"

	// LabelRole is a constant for the value of a label with key 'role'.
	LabelRole = "controller-manager"
	// portNameMetrics is a constant for the name of the metrics port of the kube-controller-manager.
	portNameMetrics = "metrics"

	// volumeMountPathCA is the volume mount path for the CA certificate used by the kube controller manager.
	volumeMountPathCA = "/srv/kubernetes/ca"
	// volumeMountPathServiceAccountKey is the volume mount path for the service account key that is a PEM-encoded private RSA or ECDSA key used to sign service account tokens.
	volumeMountPathServiceAccountKey = "/srv/kubernetes/service-account-key"
	// volumeMountPathKubeconfig is the volume mount path for the kubeconfig which can be used by the kube-controller-manager to communicate with the kube-apiserver.
	volumeMountPathKubeconfig = "/var/lib/kube-controller-manager"
	// volumeMountPathServer is the volume mount path for the x509 TLS server certificate and key for the HTTPS server inside the kube-controller-manager (which is used for metrics and health checks).
	volumeMountPathServer = "/var/lib/kube-controller-manager-server"

	// managedResourceName is the name of the managed resource that contains the resources to be deployed into the Shoot cluster.
	managedResourceName = "shoot-core-kube-controller-manager"
)

// KubeControllerManager contains functions for a kube-controller-manager deployer.
type KubeControllerManager interface {
	component.DeployWaiter
	component.MonitoringComponent
	// SetSecrets sets the secrets for the kube-controller-manager.
	SetSecrets(Secrets)
	// SetReplicaCount sets the replica count for the kube-controller-manager.
	SetReplicaCount(replicas int32)
	// WaitForControllerToBeActive checks whether kube-controller-manager has
	// recently written to the Endpoint object holding the leader information. If yes, it is active.
	WaitForControllerToBeActive(ctx context.Context) error
	// SetShootClient sets the shoot client used to deploy resources into the Shoot API server.
	SetShootClient(c client.Client)
}

// New creates a new instance of DeployWaiter for the kube-controller-manager.
func New(
	logger logrus.FieldLogger,
	seedClient client.Client,
	namespace string,
	version *semver.Version,
	image string,
	config *gardencorev1beta1.KubeControllerManagerConfig,
	podNetwork *net.IPNet,
	serviceNetwork *net.IPNet,
) KubeControllerManager {
	return &kubeControllerManager{
		log:            logger,
		seedClient:     seedClient,
		namespace:      namespace,
		version:        version,
		image:          image,
		config:         config,
		podNetwork:     podNetwork,
		serviceNetwork: serviceNetwork,
	}
}

type kubeControllerManager struct {
	log            logrus.FieldLogger
	seedClient     client.Client
	shootClient    client.Client
	namespace      string
	version        *semver.Version
	image          string
	replicas       int32
	config         *gardencorev1beta1.KubeControllerManagerConfig
	secrets        Secrets
	podNetwork     *net.IPNet
	serviceNetwork *net.IPNet
}

func (k *kubeControllerManager) Deploy(ctx context.Context) error {
	if k.secrets.Kubeconfig.Name == "" || k.secrets.Kubeconfig.Checksum == "" {
		return fmt.Errorf("missing kubeconfig secret information")
	}
	if k.secrets.Server.Name == "" || k.secrets.Server.Checksum == "" {
		return fmt.Errorf("missing server secret information")
	}
	if k.secrets.CA.Name == "" || k.secrets.CA.Checksum == "" {
		return fmt.Errorf("missing CA secret information")
	}
	if k.secrets.ServiceAccountKey.Name == "" || k.secrets.ServiceAccountKey.Checksum == "" {
		return fmt.Errorf("missing ServiceAccountKey secret information")
	}

	var (
		vpa           = k.emptyVPA()
		service       = k.emptyService()
		deployment    = k.emptyDeployment()
		vpaUpdateMode = autoscalingv1beta2.UpdateModeAuto

		port           = k.computeServerPort()
		probeURIScheme = k.computeServerURIScheme()
		command        = k.computeCommand(port)
	)

	if _, err := controllerutil.CreateOrUpdate(ctx, k.seedClient, service, func() error {
		service.Labels = getLabels()
		service.Spec.Selector = getLabels()
		service.Spec.Type = corev1.ServiceTypeClusterIP
		service.Spec.ClusterIP = corev1.ClusterIPNone
		service.Spec.Ports = kutil.ReconcileServicePorts(service.Spec.Ports, []corev1.ServicePort{
			{
				Name:     portNameMetrics,
				Protocol: corev1.ProtocolTCP,
				Port:     port,
			},
		})
		return nil
	}); err != nil {
		return err
	}

	if _, err := controllerutil.CreateOrUpdate(ctx, k.seedClient, deployment, func() error {
		deployment.Labels = utils.MergeStringMaps(getLabels(), map[string]string{
			v1beta1constants.GardenRole:           v1beta1constants.GardenRoleControlPlane,
			v1beta1constants.DeprecatedGardenRole: v1beta1constants.GardenRoleControlPlane,
		})
		deployment.Spec.Replicas = &k.replicas
		deployment.Spec.RevisionHistoryLimit = pointer.Int32Ptr(0)
		deployment.Spec.Selector = &metav1.LabelSelector{MatchLabels: getLabels()}
		deployment.Spec.Template = corev1.PodTemplateSpec{
			ObjectMeta: metav1.ObjectMeta{
				Annotations: map[string]string{
					"checksum/secret-" + k.secrets.CA.Name:                k.secrets.CA.Checksum,
					"checksum/secret-" + k.secrets.ServiceAccountKey.Name: k.secrets.ServiceAccountKey.Checksum,
					"checksum/secret-" + k.secrets.Kubeconfig.Name:        k.secrets.Kubeconfig.Checksum,
					"checksum/secret-" + k.secrets.Server.Name:            k.secrets.Server.Checksum,
				},
				Labels: utils.MergeStringMaps(getLabels(), map[string]string{
					v1beta1constants.DeprecatedGardenRole:               v1beta1constants.GardenRoleControlPlane,
					v1beta1constants.LabelPodMaintenanceRestart:         "true",
					v1beta1constants.LabelNetworkPolicyToDNS:            v1beta1constants.LabelNetworkPolicyAllowed,
					v1beta1constants.LabelNetworkPolicyToShootAPIServer: v1beta1constants.LabelNetworkPolicyAllowed,
					v1beta1constants.LabelNetworkPolicyFromPrometheus:   v1beta1constants.LabelNetworkPolicyAllowed,
				}),
			},
			Spec: corev1.PodSpec{
				Containers: []corev1.Container{
					{
						Name:            containerName,
						Image:           k.image,
						ImagePullPolicy: corev1.PullIfNotPresent,
						Command:         command,
						LivenessProbe: &corev1.Probe{
							Handler: corev1.Handler{
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
						Resources: corev1.ResourceRequirements{
							Requests: corev1.ResourceList{
								corev1.ResourceCPU:    resource.MustParse("100m"),
								corev1.ResourceMemory: resource.MustParse("128Mi"),
							},
							Limits: corev1.ResourceList{
								corev1.ResourceCPU:    resource.MustParse("400m"),
								corev1.ResourceMemory: resource.MustParse("512Mi"),
							},
						},
						VolumeMounts: []corev1.VolumeMount{
							{
								Name:      k.secrets.CA.Name,
								MountPath: volumeMountPathCA,
							},
							{
								Name:      k.secrets.ServiceAccountKey.Name,
								MountPath: volumeMountPathServiceAccountKey,
							},
							{
								Name:      k.secrets.Kubeconfig.Name,
								MountPath: volumeMountPathKubeconfig,
							},
							{
								Name:      k.secrets.Server.Name,
								MountPath: volumeMountPathServer,
							},
						},
					},
				},
				Volumes: []corev1.Volume{
					{
						Name: k.secrets.CA.Name,
						VolumeSource: corev1.VolumeSource{
							Secret: &corev1.SecretVolumeSource{
								SecretName: k.secrets.CA.Name,
							},
						},
					},
					{
						Name: k.secrets.ServiceAccountKey.Name,
						VolumeSource: corev1.VolumeSource{
							Secret: &corev1.SecretVolumeSource{
								SecretName: k.secrets.ServiceAccountKey.Name,
							},
						},
					},
					{
						Name: k.secrets.Kubeconfig.Name,
						VolumeSource: corev1.VolumeSource{
							Secret: &corev1.SecretVolumeSource{
								SecretName: k.secrets.Kubeconfig.Name,
							},
						},
					},
					{
						Name: k.secrets.Server.Name,
						VolumeSource: corev1.VolumeSource{
							Secret: &corev1.SecretVolumeSource{
								SecretName: k.secrets.Server.Name,
							},
						},
					},
				},
			},
		}
		return nil
	}); err != nil {
		return err
	}

	if _, err := controllerutil.CreateOrUpdate(ctx, k.seedClient, vpa, func() error {
		vpa.Spec.TargetRef = &autoscalingv1.CrossVersionObjectReference{
			APIVersion: appsv1.SchemeGroupVersion.String(),
			Kind:       "Deployment",
			Name:       v1beta1constants.DeploymentNameKubeControllerManager,
		}
		vpa.Spec.UpdatePolicy = &autoscalingv1beta2.PodUpdatePolicy{
			UpdateMode: &vpaUpdateMode,
		}
		vpa.Spec.ResourcePolicy = &autoscalingv1beta2.PodResourcePolicy{
			ContainerPolicies: []autoscalingv1beta2.ContainerResourcePolicy{{
				ContainerName: containerName,
				MinAllowed: corev1.ResourceList{
					corev1.ResourceCPU:    resource.MustParse("100m"),
					corev1.ResourceMemory: resource.MustParse("100Mi"),
				},
			}},
		}
		return nil
	}); err != nil {
		return err
	}

	// create managed resource that deploys resources to the Shoot API Server
	return k.deployShootManagedResource(ctx)
}

func (k *kubeControllerManager) SetSecrets(secrets Secrets) { k.secrets = secrets }
func (k *kubeControllerManager) SetShootClient(c client.Client) {
	k.shootClient = c
}
func (k *kubeControllerManager) SetReplicaCount(replicas int32)  { k.replicas = replicas }
func (k *kubeControllerManager) Destroy(_ context.Context) error { return nil }

func (k *kubeControllerManager) emptyVPA() *autoscalingv1beta2.VerticalPodAutoscaler {
	return &autoscalingv1beta2.VerticalPodAutoscaler{ObjectMeta: metav1.ObjectMeta{Name: "kube-controller-manager-vpa", Namespace: k.namespace}}
}

func (k *kubeControllerManager) emptyService() *corev1.Service {
	return &corev1.Service{ObjectMeta: metav1.ObjectMeta{Name: ServiceName, Namespace: k.namespace}}
}

func (k *kubeControllerManager) emptyDeployment() *appsv1.Deployment {
	return &appsv1.Deployment{ObjectMeta: metav1.ObjectMeta{Name: v1beta1constants.DeploymentNameKubeControllerManager, Namespace: k.namespace}}
}

func (k *kubeControllerManager) emptyManagedResource() *resourcesv1alpha1.ManagedResource {
	return &resourcesv1alpha1.ManagedResource{ObjectMeta: metav1.ObjectMeta{Name: managedResourceName, Namespace: k.namespace}}
}

func (k *kubeControllerManager) emptyManagedResourceSecret() *corev1.Secret {
	return &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: common.ManagedResourceSecretName(managedResourceName), Namespace: k.namespace}}
}

func (k *kubeControllerManager) computeServerPort() int32 {
	if versionConstraintK8sGreaterEqual113.Check(k.version) {
		return 10257
	}
	return 10252
}

func (k *kubeControllerManager) computeServerURIScheme() corev1.URIScheme {
	if versionConstraintK8sGreaterEqual113.Check(k.version) {
		return corev1.URISchemeHTTPS
	}
	return corev1.URISchemeHTTP
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

	if versionConstraintK8sGreaterEqual117.Check(k.version) {
		command = append(command, "/usr/local/bin/kube-controller-manager")
	} else if versionConstraintK8sGreaterEqual115.Check(k.version) {
		command = append(command, "/hyperkube", "kube-controller-manager")
	} else {
		command = append(command, "/hyperkube", "controller-manager")
	}

	command = append(command,
		"--allocate-node-cidrs=true",
		"--attach-detach-reconcile-sync-period=1m0s",
		"--controllers=*,bootstrapsigner,tokencleaner",
	)

	if k.config.NodeCIDRMaskSize != nil {
		command = append(command, fmt.Sprintf("--node-cidr-mask-size=%d", *k.config.NodeCIDRMaskSize))
	}

	command = append(command,
		fmt.Sprintf("--cluster-cidr=%s", k.podNetwork.String()),
		fmt.Sprintf("--cluster-name=%s", k.namespace),
		fmt.Sprintf("--cluster-signing-cert-file=%s/%s", volumeMountPathCA, secrets.DataKeyCertificateCA),
		fmt.Sprintf("--cluster-signing-key-file=%s/%s", volumeMountPathCA, secrets.DataKeyPrivateKeyCA),
		"--concurrent-deployment-syncs=50",
		"--concurrent-endpoint-syncs=15",
		"--concurrent-gc-syncs=30",
		"--concurrent-namespace-syncs=50",
		"--concurrent-replicaset-syncs=50",
		"--concurrent-resource-quota-syncs=15",
	)

	if versionConstraintK8sGreaterEqual116.Check(k.version) {
		command = append(command,
			"--concurrent-service-endpoint-syncs=15",
			"--concurrent-statefulset-syncs=15",
		)
	}

	command = append(command, "--concurrent-serviceaccount-token-syncs=15")

	if len(k.config.FeatureGates) > 0 {
		command = append(command, kutil.FeatureGatesToCommandLineParameter(k.config.FeatureGates))
	}

	if versionConstraintK8sSmaller112.Check(k.version) {
		command = append(command,
			fmt.Sprintf("--horizontal-pod-autoscaler-downscale-delay=%s", defaultHorizontalPodAutoscalerConfig.DownscaleDelay.Duration.String()),
			fmt.Sprintf("--horizontal-pod-autoscaler-upscale-delay=%s", defaultHorizontalPodAutoscalerConfig.UpscaleDelay.Duration.String()),
		)
	}

	podEvictionTimeout := metav1.Duration{Duration: 2 * time.Minute}
	if v := k.config.PodEvictionTimeout; v != nil {
		podEvictionTimeout = *v
	}

	command = append(command,
		fmt.Sprintf("--horizontal-pod-autoscaler-sync-period=%s", defaultHorizontalPodAutoscalerConfig.SyncPeriod.Duration.String()),
		fmt.Sprintf("--horizontal-pod-autoscaler-tolerance=%v", *defaultHorizontalPodAutoscalerConfig.Tolerance),
		fmt.Sprintf("--kubeconfig=%s/%s", volumeMountPathKubeconfig, secrets.DataKeyKubeconfig),
		"--leader-elect=true",
		"--node-monitor-grace-period=120s",
		fmt.Sprintf("--pod-eviction-timeout=%s", podEvictionTimeout.Duration),
		fmt.Sprintf("--root-ca-file=%s/%s", volumeMountPathCA, secrets.DataKeyCertificateCA),
		fmt.Sprintf("--service-account-private-key-file=%s/%s", volumeMountPathServiceAccountKey, secrets.DataKeyRSAPrivateKey),
		fmt.Sprintf("--service-cluster-ip-range=%s", k.serviceNetwork.String()),
	)

	if versionConstraintK8sGreaterEqual113.Check(k.version) {
		command = append(command,
			fmt.Sprintf("--secure-port=%d", port),
			"--port=0",
		)
	}

	if versionConstraintK8sGreaterEqual112.Check(k.version) {
		command = append(command,
			fmt.Sprintf("--horizontal-pod-autoscaler-downscale-stabilization=%s", defaultHorizontalPodAutoscalerConfig.DownscaleStabilization.Duration.String()),
			fmt.Sprintf("--horizontal-pod-autoscaler-initial-readiness-delay=%s", defaultHorizontalPodAutoscalerConfig.InitialReadinessDelay.Duration.String()),
			fmt.Sprintf("--horizontal-pod-autoscaler-cpu-initialization-period=%s", defaultHorizontalPodAutoscalerConfig.CPUInitializationPeriod.Duration.String()),
			fmt.Sprintf("--authentication-kubeconfig=%s/%s", volumeMountPathKubeconfig, secrets.DataKeyKubeconfig),
			fmt.Sprintf("--authorization-kubeconfig=%s/%s", volumeMountPathKubeconfig, secrets.DataKeyKubeconfig),
			fmt.Sprintf("--tls-cert-file=%s/%s", volumeMountPathServer, secrets.ControlPlaneSecretDataKeyCertificatePEM(SecretNameServer)),
			fmt.Sprintf("--tls-private-key-file=%s/%s", volumeMountPathServer, secrets.ControlPlaneSecretDataKeyPrivateKey(SecretNameServer)),
		)
	}

	command = append(command,
		"--tls-cipher-suites=TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256,TLS_ECDHE_RSA_WITH_AES_256_GCM_SHA384,TLS_ECDHE_RSA_WITH_CHACHA20_POLY1305,TLS_RSA_WITH_AES_128_CBC_SHA,TLS_RSA_WITH_AES_256_CBC_SHA,TLS_ECDHE_RSA_WITH_AES_256_CBC_SHA",
		"--use-service-account-credentials=true",
		"--v=2",
	)
	return command
}

func (k *kubeControllerManager) getHorizontalPodAutoscalerConfig() gardencorev1beta1.HorizontalPodAutoscalerConfig {
	defaultHPATolerance := gardencorev1beta1.DefaultHPATolerance
	horizontalPodAutoscalerConfig := gardencorev1beta1.HorizontalPodAutoscalerConfig{
		CPUInitializationPeriod: &metav1.Duration{Duration: gardencorev1beta1.DefaultCPUInitializationPeriod},
		DownscaleDelay:          &metav1.Duration{Duration: gardencorev1beta1.DefaultHPADownscaleDelay},
		DownscaleStabilization:  &metav1.Duration{Duration: gardencorev1beta1.DefaultDownscaleStabilization},
		InitialReadinessDelay:   &metav1.Duration{Duration: gardencorev1beta1.DefaultInitialReadinessDelay},
		SyncPeriod:              &metav1.Duration{Duration: gardencorev1beta1.DefaultHPASyncPeriod},
		Tolerance:               &defaultHPATolerance,
		UpscaleDelay:            &metav1.Duration{Duration: gardencorev1beta1.DefaultHPAUpscaleDelay},
	}

	if k.config.HorizontalPodAutoscalerConfig != nil {
		if v := k.config.HorizontalPodAutoscalerConfig.CPUInitializationPeriod; v != nil {
			horizontalPodAutoscalerConfig.CPUInitializationPeriod = v
		}
		if v := k.config.HorizontalPodAutoscalerConfig.DownscaleDelay; v != nil {
			horizontalPodAutoscalerConfig.DownscaleDelay = v
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
		if v := k.config.HorizontalPodAutoscalerConfig.UpscaleDelay; v != nil {
			horizontalPodAutoscalerConfig.UpscaleDelay = v
		}
	}
	return horizontalPodAutoscalerConfig
}

var (
	versionConstraintK8sGreaterEqual112 *semver.Constraints
	versionConstraintK8sSmaller112      *semver.Constraints
	versionConstraintK8sGreaterEqual113 *semver.Constraints
	versionConstraintK8sSmaller114      *semver.Constraints
	versionConstraintK8sGreaterEqual115 *semver.Constraints
	versionConstraintK8sGreaterEqual116 *semver.Constraints
	versionConstraintK8sGreaterEqual117 *semver.Constraints
	versionConstraintK8sGreaterEqual120 *semver.Constraints
)

func init() {
	var err error

	versionConstraintK8sSmaller112, err = semver.NewConstraint("< 1.12")
	utilruntime.Must(err)
	versionConstraintK8sGreaterEqual112, err = semver.NewConstraint(">= 1.12")
	utilruntime.Must(err)
	versionConstraintK8sGreaterEqual113, err = semver.NewConstraint(">= 1.13")
	utilruntime.Must(err)
	versionConstraintK8sSmaller114, err = semver.NewConstraint("< 1.14")
	utilruntime.Must(err)
	versionConstraintK8sGreaterEqual115, err = semver.NewConstraint(">= 1.15")
	utilruntime.Must(err)
	versionConstraintK8sGreaterEqual116, err = semver.NewConstraint(">= 1.16")
	utilruntime.Must(err)
	versionConstraintK8sGreaterEqual117, err = semver.NewConstraint(">= 1.17")
	utilruntime.Must(err)
	versionConstraintK8sGreaterEqual120, err = semver.NewConstraint(">= 1.20")
	utilruntime.Must(err)
}

// Secrets is collection of secrets for the kube-controller-manager.
type Secrets struct {
	// Kubeconfig is a secret that contains a kubeconfig which can be used by the kube-controller-manager to communicate with the kube-apiserver.
	Kubeconfig component.Secret
	// Server is a secret containing a x509 TLS server certificate and key for the HTTPS server inside the kube-controller-manager (which is used for metrics and health checks).
	Server component.Secret
	// CA is a secret containing a root CA x509 certificate and key that is used for the flags.
	// --cluster-signing-cert-file
	// --cluster-signing-key-file
	// --root-ca-file
	CA component.Secret
	// ServiceAccountKey is a secret containing a PEM-encoded private RSA or ECDSA key used to sign service account tokens.
	// used for the flag: --service-account-private-key-file
	ServiceAccountKey component.Secret
}

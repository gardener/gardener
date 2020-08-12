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

package controlplane

import (
	"context"
	"fmt"
	"strings"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	"github.com/gardener/gardener/pkg/operation/botanist/component"
	"github.com/gardener/gardener/pkg/operation/common"
	"github.com/gardener/gardener/pkg/utils"
	kutil "github.com/gardener/gardener/pkg/utils/kubernetes"
	"github.com/gardener/gardener/pkg/utils/secrets"
	versionutils "github.com/gardener/gardener/pkg/utils/version"

	appsv1 "k8s.io/api/apps/v1"
	autoscalingv1 "k8s.io/api/autoscaling/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	autoscalingv1beta2 "k8s.io/autoscaler/vertical-pod-autoscaler/pkg/apis/autoscaling.k8s.io/v1beta2"
	componentbaseconfigv1alpha1 "k8s.io/component-base/config/v1alpha1"
	kubeschedulerv1alpha1 "k8s.io/kube-scheduler/config/v1alpha1"
	"k8s.io/utils/pointer"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

const (
	// LabelScheduler is a constant for the value of a label with key 'role' whose value is 'scheduler'.
	LabelScheduler = "scheduler"
	// KubeSchedulerDataKeyComponentConfig is a constant for the key of the data map in a ConfigMap whose value is the
	// component configuration of the kube-scheduler.
	KubeSchedulerDataKeyComponentConfig = "config.yaml"
	// KubeSchedulerPortNameMetrics is a constant for the name of the metrics port of the kube-scheduler.
	KubeSchedulerPortNameMetrics = "metrics"

	kubeSchedulerVolumeMountPathKubeconfig = "/var/lib/kube-scheduler"
	kubeSchedulerVolumeMountPathServer     = "/var/lib/kube-scheduler-server"
	kubeSchedulerVolumeMountPathConfig     = "/var/lib/kube-scheduler-config"
)

// KubeScheduler contains function for a kube-scheduler deployer.
type KubeScheduler interface {
	component.DeployWaiter
	// SetSecrets sets the secrets for the kube-scheduler.
	SetSecrets(KubeSchedulerSecrets)
}

// NewKubeScheduler creates a new instance of DeployWaiter for the kube-scheduler.
func NewKubeScheduler(
	client client.Client,
	namespace string,
	version string,
	image string,
	replicas int32,
	config *gardencorev1beta1.KubeSchedulerConfig,
) (KubeScheduler, error) {
	k := &kubeScheduler{
		client:    client,
		namespace: namespace,
		version:   version,
		image:     image,
		replicas:  replicas,
		config:    config,
	}

	versionConstraints, err := k.computeVersionConstraints()
	if err != nil {
		return nil, err
	}
	k.versionConstraints = *versionConstraints

	return k, nil
}

type kubeScheduler struct {
	client             client.Client
	namespace          string
	version            string
	image              string
	replicas           int32
	config             *gardencorev1beta1.KubeSchedulerConfig
	secrets            KubeSchedulerSecrets
	versionConstraints kubeSchedulerVersionConstraints
}

func (k *kubeScheduler) Deploy(ctx context.Context) error {
	if k.secrets.Kubeconfig.Name == "" || k.secrets.Kubeconfig.Checksum == "" {
		return fmt.Errorf("missing kubeconfig secret information")
	}
	if k.secrets.Server.Name == "" || k.secrets.Server.Checksum == "" {
		return fmt.Errorf("missing server secret information")
	}

	var (
		configMap  = k.emptyConfigMap()
		vpa        = k.emptyVPA()
		service    = k.emptyService()
		deployment = k.emptyDeployment()

		labels = map[string]string{
			v1beta1constants.LabelApp:  v1beta1constants.LabelKubernetes,
			v1beta1constants.LabelRole: LabelScheduler,
		}
		labelsWithControlPlaneRole = utils.MergeStringMaps(labels, map[string]string{
			v1beta1constants.DeprecatedGardenRole: v1beta1constants.GardenRoleControlPlane,
		})
		vpaUpdateMode = autoscalingv1beta2.UpdateModeAuto

		port           = k.computeServerPort()
		probeURIScheme = k.computeServerURIScheme()
		env            = k.computeEnvironmentVariables()
		command        = k.computeCommandLineFlags(port)
	)

	componentConfigYAML, componentConfigChecksum, err := k.computeComponentConfig()
	if err != nil {
		return err
	}

	if _, err := controllerutil.CreateOrUpdate(ctx, k.client, configMap, func() error {
		configMap.Data = map[string]string{KubeSchedulerDataKeyComponentConfig: componentConfigYAML}
		return nil
	}); err != nil {
		return err
	}

	if _, err := controllerutil.CreateOrUpdate(ctx, k.client, vpa, func() error {
		vpa.Spec.TargetRef = &autoscalingv1.CrossVersionObjectReference{
			APIVersion: appsv1.SchemeGroupVersion.String(),
			Kind:       "Deployment",
			Name:       v1beta1constants.DeploymentNameKubeScheduler,
		}
		vpa.Spec.UpdatePolicy = &autoscalingv1beta2.PodUpdatePolicy{
			UpdateMode: &vpaUpdateMode,
		}
		return nil
	}); err != nil {
		return err
	}

	if _, err := controllerutil.CreateOrUpdate(ctx, k.client, service, func() error {
		service.Labels = labels
		service.Spec.Selector = labels
		service.Spec.Type = corev1.ServiceTypeClusterIP
		service.Spec.Ports = kutil.ReconcileServicePorts(service.Spec.Ports, []corev1.ServicePort{
			{
				Name:     KubeSchedulerPortNameMetrics,
				Protocol: corev1.ProtocolTCP,
				Port:     port,
			},
		})
		return nil
	}); err != nil {
		return err
	}

	if _, err := controllerutil.CreateOrUpdate(ctx, k.client, deployment, func() error {
		deployment.Labels = labelsWithControlPlaneRole
		deployment.Spec.Replicas = &k.replicas
		deployment.Spec.RevisionHistoryLimit = pointer.Int32Ptr(0)
		deployment.Spec.Selector = &metav1.LabelSelector{MatchLabels: labels}
		deployment.Spec.Template = corev1.PodTemplateSpec{
			ObjectMeta: metav1.ObjectMeta{
				Annotations: map[string]string{
					"checksum/configmap-componentconfig":           componentConfigChecksum,
					"checksum/secret-" + k.secrets.Kubeconfig.Name: k.secrets.Kubeconfig.Checksum,
					"checksum/secret-" + k.secrets.Server.Name:     k.secrets.Server.Checksum,
				},
				Labels: utils.MergeStringMaps(labelsWithControlPlaneRole, map[string]string{
					v1beta1constants.LabelNetworkPolicyToDNS:            v1beta1constants.LabelNetworkPolicyAllowed,
					v1beta1constants.LabelNetworkPolicyToShootAPIServer: v1beta1constants.LabelNetworkPolicyAllowed,
					v1beta1constants.LabelNetworkPolicyFromPrometheus:   v1beta1constants.LabelNetworkPolicyAllowed,
				}),
			},
			Spec: corev1.PodSpec{
				Containers: []corev1.Container{
					{
						Name:            v1beta1constants.DeploymentNameKubeScheduler,
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
								Name:          KubeSchedulerPortNameMetrics,
								ContainerPort: port,
								Protocol:      corev1.ProtocolTCP,
							},
						},
						Env:                      env,
						TerminationMessagePath:   corev1.TerminationMessagePathDefault,
						TerminationMessagePolicy: corev1.TerminationMessageReadFile,
						Resources: corev1.ResourceRequirements{
							Requests: corev1.ResourceList{
								corev1.ResourceCPU:    resource.MustParse("23m"),
								corev1.ResourceMemory: resource.MustParse("64Mi"),
							},
							Limits: corev1.ResourceList{
								corev1.ResourceCPU:    resource.MustParse("400m"),
								corev1.ResourceMemory: resource.MustParse("512Mi"),
							},
						},
						VolumeMounts: []corev1.VolumeMount{
							{
								Name:      k.secrets.Kubeconfig.Name,
								MountPath: kubeSchedulerVolumeMountPathKubeconfig,
							},
							{
								Name:      k.secrets.Server.Name,
								MountPath: kubeSchedulerVolumeMountPathServer,
							},
							{
								Name:      configMap.Name,
								MountPath: kubeSchedulerVolumeMountPathConfig,
							},
						},
					},
				},
				DNSPolicy:                     corev1.DNSClusterFirst,
				RestartPolicy:                 corev1.RestartPolicyAlways,
				SchedulerName:                 corev1.DefaultSchedulerName,
				TerminationGracePeriodSeconds: pointer.Int64Ptr(30),
				Volumes: []corev1.Volume{
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
					{
						Name: configMap.Name,
						VolumeSource: corev1.VolumeSource{
							ConfigMap: &corev1.ConfigMapVolumeSource{
								LocalObjectReference: corev1.LocalObjectReference{
									Name: configMap.Name,
								},
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

	return nil
}

func (k *kubeScheduler) SetSecrets(secrets KubeSchedulerSecrets) { k.secrets = secrets }
func (k *kubeScheduler) Destroy(_ context.Context) error         { return nil }
func (k *kubeScheduler) Wait(_ context.Context) error            { return nil }
func (k *kubeScheduler) WaitCleanup(_ context.Context) error     { return nil }

func (k *kubeScheduler) emptyConfigMap() *corev1.ConfigMap {
	return &corev1.ConfigMap{ObjectMeta: metav1.ObjectMeta{Name: "kube-scheduler-config", Namespace: k.namespace}}
}

func (k *kubeScheduler) emptyVPA() *autoscalingv1beta2.VerticalPodAutoscaler {
	return &autoscalingv1beta2.VerticalPodAutoscaler{ObjectMeta: metav1.ObjectMeta{Name: "kube-scheduler-vpa", Namespace: k.namespace}}
}

func (k *kubeScheduler) emptyService() *corev1.Service {
	return &corev1.Service{ObjectMeta: metav1.ObjectMeta{Name: "kube-scheduler", Namespace: k.namespace}}
}

func (k *kubeScheduler) emptyDeployment() *appsv1.Deployment {
	return &appsv1.Deployment{ObjectMeta: metav1.ObjectMeta{Name: v1beta1constants.DeploymentNameKubeScheduler, Namespace: k.namespace}}
}

func (k *kubeScheduler) computeServerPort() int32 {
	if k.versionConstraints.k8sGreaterEqual113 {
		return 10259
	}
	return 10251
}

func (k *kubeScheduler) computeServerURIScheme() corev1.URIScheme {
	if k.versionConstraints.k8sGreaterEqual113 {
		return corev1.URISchemeHTTPS
	}
	return corev1.URISchemeHTTP
}

func (k *kubeScheduler) computeEnvironmentVariables() []corev1.EnvVar {
	if k.config != nil && k.config.KubeMaxPDVols != nil {
		return []corev1.EnvVar{{
			Name:  "KUBE_MAX_PD_VOLS",
			Value: *k.config.KubeMaxPDVols,
		}}
	}
	return nil
}

func (k *kubeScheduler) computeComponentConfig() (string, string, error) {
	var apiVersion string
	if k.versionConstraints.k8sGreaterEqual118 {
		apiVersion = "kubescheduler.config.k8s.io/v1alpha2"
	} else if k.versionConstraints.k8sGreaterEqual112 {
		apiVersion = kubeschedulerv1alpha1.SchemeGroupVersion.String()
	} else if k.versionConstraints.k8sGreaterEqual110 {
		apiVersion = "componentconfig/v1alpha1"
	}

	componentConfig := &kubeschedulerv1alpha1.KubeSchedulerConfiguration{
		TypeMeta: metav1.TypeMeta{
			APIVersion: apiVersion,
			Kind:       "KubeSchedulerConfiguration",
		},
		ClientConnection: componentbaseconfigv1alpha1.ClientConnectionConfiguration{
			Kubeconfig: kubeSchedulerVolumeMountPathKubeconfig + "/" + secrets.DataKeyKubeconfig,
		},
		LeaderElection: kubeschedulerv1alpha1.KubeSchedulerLeaderElectionConfiguration{
			LeaderElectionConfiguration: componentbaseconfigv1alpha1.LeaderElectionConfiguration{
				LeaderElect: pointer.BoolPtr(true),
			},
		},
	}

	componentConfigYAML, err := NewKubeSchedulerConfigCodec().Encode(componentConfig)
	if err != nil {
		return "", "", err
	}
	componentConfigYAML = strings.Replace(componentConfigYAML, kubeschedulerv1alpha1.SchemeGroupVersion.String(), apiVersion, -1)

	return componentConfigYAML, utils.ComputeChecksum(componentConfig), nil
}

func (k *kubeScheduler) computeCommandLineFlags(port int32) []string {
	var command []string

	if k.versionConstraints.k8sGreaterEqual117 {
		command = append(command, "/usr/local/bin/kube-scheduler")
	} else if k.versionConstraints.k8sGreaterEqual115 {
		command = append(command, "/hyperkube", "kube-scheduler")
	} else {
		command = append(command, "/hyperkube", "scheduler")
	}

	command = append(command, fmt.Sprintf("--config=%s/%s", kubeSchedulerVolumeMountPathConfig, KubeSchedulerDataKeyComponentConfig))

	if k.versionConstraints.k8sGreaterEqual113 {
		command = append(command,
			fmt.Sprintf("--authentication-kubeconfig=%s/%s", kubeSchedulerVolumeMountPathKubeconfig, secrets.DataKeyKubeconfig),
			fmt.Sprintf("--authorization-kubeconfig=%s/%s", kubeSchedulerVolumeMountPathKubeconfig, secrets.DataKeyKubeconfig),
			fmt.Sprintf("--client-ca-file=%s/%s", kubeSchedulerVolumeMountPathServer, secrets.DataKeyCertificateCA),
			fmt.Sprintf("--tls-cert-file=%s/%s", kubeSchedulerVolumeMountPathServer, secrets.ControlPlaneSecretDataKeyCertificatePEM(common.KubeSchedulerServerName)),
			fmt.Sprintf("--tls-private-key-file=%s/%s", kubeSchedulerVolumeMountPathServer, secrets.ControlPlaneSecretDataKeyPrivateKey(common.KubeSchedulerServerName)),
			fmt.Sprintf("--secure-port=%d", port),
			"--port=0",
		)
	}

	if k.config != nil {
		command = append(command, kutil.FeatureGatesToCommandLineParameter(k.config.FeatureGates))
	}
	if k.versionConstraints.k8sEqual110 {
		command = append(command, kutil.FeatureGatesToCommandLineParameter(map[string]bool{"PodPriority": true}))
	}

	command = append(command, "--v=2")
	return command
}

func (k *kubeScheduler) computeVersionConstraints() (*kubeSchedulerVersionConstraints, error) {
	k8sEqual110, err := versionutils.CompareVersions(k.version, "~", "1.10")
	if err != nil {
		return nil, err
	}
	k8sVersionGreaterEqual110, err := versionutils.CompareVersions(k.version, ">=", "1.10")
	if err != nil {
		return nil, err
	}
	k8sVersionGreaterEqual112, err := versionutils.CompareVersions(k.version, ">=", "1.12")
	if err != nil {
		return nil, err
	}
	k8sVersionGreaterEqual113, err := versionutils.CompareVersions(k.version, ">=", "1.13")
	if err != nil {
		return nil, err
	}
	k8sVersionGreaterEqual115, err := versionutils.CompareVersions(k.version, ">=", "1.15")
	if err != nil {
		return nil, err
	}
	k8sVersionGreaterEqual117, err := versionutils.CompareVersions(k.version, ">=", "1.17")
	if err != nil {
		return nil, err
	}
	k8sVersionGreaterEqual118, err := versionutils.CompareVersions(k.version, ">=", "1.18")
	if err != nil {
		return nil, err
	}

	return &kubeSchedulerVersionConstraints{
		k8sEqual110,
		k8sVersionGreaterEqual110,
		k8sVersionGreaterEqual112,
		k8sVersionGreaterEqual113,
		k8sVersionGreaterEqual115,
		k8sVersionGreaterEqual117,
		k8sVersionGreaterEqual118,
	}, nil
}

type kubeSchedulerVersionConstraints struct {
	k8sEqual110        bool
	k8sGreaterEqual110 bool
	k8sGreaterEqual112 bool
	k8sGreaterEqual113 bool
	k8sGreaterEqual115 bool
	k8sGreaterEqual117 bool
	k8sGreaterEqual118 bool
}

// KubeSchedulerSecrets is collection of secrets for the kube-scheduler.
type KubeSchedulerSecrets struct {
	// Kubeconfig is a secret which can be used by the kube-scheduler to communicate to the kube-apiserver.
	Kubeconfig Secret
	// Server is a secret for the HTTPS server inside the kube-scheduler (which is used for metrics and health checks).
	Server Secret
}

// Secret is a structure that contains information about a Kubernetes secret which is managed externally.
type Secret struct {
	// Name is the name of the Kubernetes secret object.
	Name string
	// Checksum is the checksum of the secret's data.
	Checksum string
}

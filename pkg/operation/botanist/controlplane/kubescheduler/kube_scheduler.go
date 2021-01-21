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

package kubescheduler

import (
	"bytes"
	"context"
	"fmt"
	"text/template"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	"github.com/gardener/gardener/pkg/operation/botanist/component"
	"github.com/gardener/gardener/pkg/operation/common"
	"github.com/gardener/gardener/pkg/utils"
	kutil "github.com/gardener/gardener/pkg/utils/kubernetes"
	"github.com/gardener/gardener/pkg/utils/managedresources"
	"github.com/gardener/gardener/pkg/utils/secrets"

	"github.com/Masterminds/semver"
	resourcesv1alpha1 "github.com/gardener/gardener-resource-manager/pkg/apis/resources/v1alpha1"
	appsv1 "k8s.io/api/apps/v1"
	autoscalingv1 "k8s.io/api/autoscaling/v1"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apiserver/pkg/authentication/user"
	autoscalingv1beta2 "k8s.io/autoscaler/vertical-pod-autoscaler/pkg/apis/autoscaling.k8s.io/v1beta2"
	"k8s.io/utils/pointer"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

const (
	// ServiceName is the name of the service of the kube-scheduler.
	ServiceName = "kube-scheduler"
	// SecretName is a constant for the secret name for the kube-scheduler's kubeconfig secret.
	SecretName = "kube-scheduler"
	// SecretNameServer is the name of the kube-scheduler server certificate secret.
	SecretNameServer = "kube-scheduler-server"

	// LabelRole is a constant for the value of a label with key 'role'.
	LabelRole = "scheduler"

	managedResourceName    = "shoot-core-kube-scheduler"
	containerName          = v1beta1constants.DeploymentNameKubeScheduler
	portNameMetrics        = "metrics"
	dataKeyComponentConfig = "config.yaml"

	volumeMountPathKubeconfig = "/var/lib/kube-scheduler"
	volumeMountPathServer     = "/var/lib/kube-scheduler-server"
	volumeMountPathConfig     = "/var/lib/kube-scheduler-config"

	componentConfigTmpl = `apiVersion: {{ .apiVersion }}
kind: KubeSchedulerConfiguration
clientConnection:
  kubeconfig: ` + volumeMountPathKubeconfig + "/" + secrets.DataKeyKubeconfig + `
leaderElection:
  leaderElect: true`
)

// KubeScheduler contains functions for a kube-scheduler deployer.
type KubeScheduler interface {
	component.DeployWaiter
	component.MonitoringComponent
	// SetSecrets sets the secrets.
	SetSecrets(Secrets)
}

// New creates a new instance of DeployWaiter for the kube-scheduler.
func New(
	client client.Client,
	namespace string,
	version *semver.Version,
	image string,
	replicas int32,
	config *gardencorev1beta1.KubeSchedulerConfig,
) KubeScheduler {
	return &kubeScheduler{
		client:    client,
		namespace: namespace,
		version:   version,
		image:     image,
		replicas:  replicas,
		config:    config,
	}
}

type kubeScheduler struct {
	client    client.Client
	namespace string
	version   *semver.Version
	image     string
	replicas  int32
	config    *gardencorev1beta1.KubeSchedulerConfig

	secrets Secrets
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

		vpaUpdateMode = autoscalingv1beta2.UpdateModeAuto

		port           = k.computeServerPort()
		probeURIScheme = k.computeServerURIScheme()
		env            = k.computeEnvironmentVariables()
		command        = k.computeCommand(port)
	)

	componentConfigYAML, componentConfigChecksum, err := k.computeComponentConfig()
	if err != nil {
		return err
	}

	if _, err := controllerutil.CreateOrUpdate(ctx, k.client, configMap, func() error {
		configMap.Data = map[string]string{dataKeyComponentConfig: componentConfigYAML}
		return nil
	}); err != nil {
		return err
	}

	if _, err := controllerutil.CreateOrUpdate(ctx, k.client, service, func() error {
		service.Labels = getLabels()
		service.Spec.Selector = getLabels()
		service.Spec.Type = corev1.ServiceTypeClusterIP
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

	if _, err := controllerutil.CreateOrUpdate(ctx, k.client, deployment, func() error {
		deployment.Labels = utils.MergeStringMaps(getLabels(), map[string]string{
			v1beta1constants.GardenRole: v1beta1constants.GardenRoleControlPlane,
		})
		deployment.Spec.Replicas = &k.replicas
		deployment.Spec.RevisionHistoryLimit = pointer.Int32Ptr(1)
		deployment.Spec.Selector = &metav1.LabelSelector{MatchLabels: getLabels()}
		deployment.Spec.Template = corev1.PodTemplateSpec{
			ObjectMeta: metav1.ObjectMeta{
				Annotations: map[string]string{
					"checksum/configmap-componentconfig":           componentConfigChecksum,
					"checksum/secret-" + k.secrets.Kubeconfig.Name: k.secrets.Kubeconfig.Checksum,
					"checksum/secret-" + k.secrets.Server.Name:     k.secrets.Server.Checksum,
				},
				Labels: utils.MergeStringMaps(getLabels(), map[string]string{
					v1beta1constants.GardenRole:                         v1beta1constants.GardenRoleControlPlane,
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
						Env: env,
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
								MountPath: volumeMountPathKubeconfig,
							},
							{
								Name:      k.secrets.Server.Name,
								MountPath: volumeMountPathServer,
							},
							{
								Name:      configMap.Name,
								MountPath: volumeMountPathConfig,
							},
						},
					},
				},
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

	return k.reconcileShootResources(ctx)
}

func getLabels() map[string]string {
	return map[string]string{
		v1beta1constants.LabelApp:  v1beta1constants.LabelKubernetes,
		v1beta1constants.LabelRole: LabelRole,
	}
}

func (k *kubeScheduler) Destroy(_ context.Context) error     { return nil }
func (k *kubeScheduler) Wait(_ context.Context) error        { return nil }
func (k *kubeScheduler) WaitCleanup(_ context.Context) error { return nil }
func (k *kubeScheduler) SetSecrets(secrets Secrets)          { k.secrets = secrets }

func (k *kubeScheduler) emptyConfigMap() *corev1.ConfigMap {
	return &corev1.ConfigMap{ObjectMeta: metav1.ObjectMeta{Name: "kube-scheduler-config", Namespace: k.namespace}}
}

func (k *kubeScheduler) emptyVPA() *autoscalingv1beta2.VerticalPodAutoscaler {
	return &autoscalingv1beta2.VerticalPodAutoscaler{ObjectMeta: metav1.ObjectMeta{Name: "kube-scheduler-vpa", Namespace: k.namespace}}
}

func (k *kubeScheduler) emptyService() *corev1.Service {
	return &corev1.Service{ObjectMeta: metav1.ObjectMeta{Name: ServiceName, Namespace: k.namespace}}
}

func (k *kubeScheduler) emptyDeployment() *appsv1.Deployment {
	return &appsv1.Deployment{ObjectMeta: metav1.ObjectMeta{Name: v1beta1constants.DeploymentNameKubeScheduler, Namespace: k.namespace}}
}

func (k *kubeScheduler) emptyManagedResource() *resourcesv1alpha1.ManagedResource {
	return &resourcesv1alpha1.ManagedResource{ObjectMeta: metav1.ObjectMeta{Name: managedResourceName, Namespace: k.namespace}}
}

func (k *kubeScheduler) emptyManagedResourceSecret() *corev1.Secret {
	return &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: common.ManagedResourceSecretName(managedResourceName), Namespace: k.namespace}}
}

func (k *kubeScheduler) reconcileShootResources(ctx context.Context) error {
	if versionConstraintK8sEqual113.Check(k.version) {
		data, err := k.computeShootResourcesData()
		if err != nil {
			return err
		}
		return common.DeployManagedResourceForShoot(ctx, k.client, managedResourceName, k.namespace, false, data)
	}

	return kutil.DeleteObjects(ctx, k.client, k.emptyManagedResource(), k.emptyManagedResourceSecret())
}

func (k *kubeScheduler) computeServerPort() int32 {
	if versionConstraintK8sGreaterEqual113.Check(k.version) {
		return 10259
	}
	return 10251
}

func (k *kubeScheduler) computeServerURIScheme() corev1.URIScheme {
	if versionConstraintK8sGreaterEqual113.Check(k.version) {
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
	if versionConstraintK8sGreaterEqual119.Check(k.version) {
		apiVersion = "kubescheduler.config.k8s.io/v1beta1"
	} else if versionConstraintK8sGreaterEqual118.Check(k.version) {
		apiVersion = "kubescheduler.config.k8s.io/v1alpha2"
	} else if versionConstraintK8sGreaterEqual112.Check(k.version) {
		apiVersion = "kubescheduler.config.k8s.io/v1alpha1"
	} else if versionConstraintK8sGreaterEqual110.Check(k.version) {
		apiVersion = "componentconfig/v1alpha1"
	}

	var componentConfigYAML bytes.Buffer
	if err := componentConfigTemplate.Execute(&componentConfigYAML, map[string]string{"apiVersion": apiVersion}); err != nil {
		return "", "", err
	}

	return componentConfigYAML.String(), utils.ComputeSHA256Hex(componentConfigYAML.Bytes()), nil
}

func (k *kubeScheduler) computeCommand(port int32) []string {
	var command []string

	if versionConstraintK8sGreaterEqual117.Check(k.version) {
		command = append(command, "/usr/local/bin/kube-scheduler")
	} else if versionConstraintK8sGreaterEqual115.Check(k.version) {
		command = append(command, "/hyperkube", "kube-scheduler")
	} else {
		command = append(command, "/hyperkube", "scheduler")
	}

	command = append(command, fmt.Sprintf("--config=%s/%s", volumeMountPathConfig, dataKeyComponentConfig))

	if versionConstraintK8sGreaterEqual113.Check(k.version) {
		command = append(command,
			fmt.Sprintf("--authentication-kubeconfig=%s/%s", volumeMountPathKubeconfig, secrets.DataKeyKubeconfig),
			fmt.Sprintf("--authorization-kubeconfig=%s/%s", volumeMountPathKubeconfig, secrets.DataKeyKubeconfig),
			fmt.Sprintf("--client-ca-file=%s/%s", volumeMountPathServer, secrets.DataKeyCertificateCA),
			fmt.Sprintf("--tls-cert-file=%s/%s", volumeMountPathServer, secrets.ControlPlaneSecretDataKeyCertificatePEM(SecretNameServer)),
			fmt.Sprintf("--tls-private-key-file=%s/%s", volumeMountPathServer, secrets.ControlPlaneSecretDataKeyPrivateKey(SecretNameServer)),
			fmt.Sprintf("--secure-port=%d", port),
			"--port=0",
		)
	}

	if k.config != nil {
		command = append(command, kutil.FeatureGatesToCommandLineParameter(k.config.FeatureGates))
	}
	if versionConstraintK8sEqual110.Check(k.version) {
		command = append(command, kutil.FeatureGatesToCommandLineParameter(map[string]bool{"PodPriority": true}))
	}

	command = append(command, "--v=2")
	return command
}

func (k *kubeScheduler) computeShootResourcesData() (map[string][]byte, error) {
	var (
		registry = managedresources.NewRegistry(kubernetes.ShootScheme, kubernetes.ShootCodec, kubernetes.ShootSerializer)

		subjects = []rbacv1.Subject{{
			Kind: rbacv1.UserKind,
			Name: user.KubeScheduler,
		}}

		clusterRoleBinding = &rbacv1.ClusterRoleBinding{
			ObjectMeta: metav1.ObjectMeta{
				Name: "system:controller:kube-scheduler",
			},
			RoleRef: rbacv1.RoleRef{
				APIGroup: rbacv1.GroupName,
				Kind:     "ClusterRole",
				Name:     "system:auth-delegator",
			},
			Subjects: subjects,
		}

		roleBinding = &rbacv1.RoleBinding{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "system:controller:kube-scheduler:auth-reader",
				Namespace: metav1.NamespaceSystem,
			},
			RoleRef: rbacv1.RoleRef{
				APIGroup: rbacv1.GroupName,
				Kind:     "Role",
				Name:     "extension-apiserver-authentication-reader",
			},
			Subjects: subjects,
		}
	)

	return registry.AddAllAndSerialize(
		clusterRoleBinding,
		roleBinding,
	)
}

var (
	componentConfigTemplate *template.Template

	versionConstraintK8sEqual110        *semver.Constraints
	versionConstraintK8sEqual113        *semver.Constraints
	versionConstraintK8sGreaterEqual110 *semver.Constraints
	versionConstraintK8sGreaterEqual112 *semver.Constraints
	versionConstraintK8sGreaterEqual113 *semver.Constraints
	versionConstraintK8sGreaterEqual115 *semver.Constraints
	versionConstraintK8sGreaterEqual117 *semver.Constraints
	versionConstraintK8sGreaterEqual118 *semver.Constraints
	versionConstraintK8sGreaterEqual119 *semver.Constraints
)

func init() {
	var err error

	componentConfigTemplate, err = template.New("config").Parse(componentConfigTmpl)
	utilruntime.Must(err)

	versionConstraintK8sEqual110, err = semver.NewConstraint("~ 1.10")
	utilruntime.Must(err)
	versionConstraintK8sEqual113, err = semver.NewConstraint("~ 1.13")
	utilruntime.Must(err)
	versionConstraintK8sGreaterEqual110, err = semver.NewConstraint(">= 1.10")
	utilruntime.Must(err)
	versionConstraintK8sGreaterEqual112, err = semver.NewConstraint(">= 1.12")
	utilruntime.Must(err)
	versionConstraintK8sGreaterEqual113, err = semver.NewConstraint(">= 1.13")
	utilruntime.Must(err)
	versionConstraintK8sGreaterEqual115, err = semver.NewConstraint(">= 1.15")
	utilruntime.Must(err)
	versionConstraintK8sGreaterEqual117, err = semver.NewConstraint(">= 1.17")
	utilruntime.Must(err)
	versionConstraintK8sGreaterEqual118, err = semver.NewConstraint(">= 1.18")
	utilruntime.Must(err)
	versionConstraintK8sGreaterEqual119, err = semver.NewConstraint(">= 1.19")
	utilruntime.Must(err)
}

// Secrets is collection of secrets for the kube-scheduler.
type Secrets struct {
	// Kubeconfig is a secret which can be used by the kube-scheduler to communicate to the kube-apiserver.
	Kubeconfig component.Secret
	// Server is a secret for the HTTPS server inside the kube-scheduler (which is used for metrics and health checks).
	Server component.Secret
}

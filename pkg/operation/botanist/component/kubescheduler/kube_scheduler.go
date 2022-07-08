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
	"github.com/gardener/gardener/pkg/controllerutils"
	"github.com/gardener/gardener/pkg/operation/botanist/component"
	"github.com/gardener/gardener/pkg/resourcemanager/controller/garbagecollector/references"
	"github.com/gardener/gardener/pkg/utils"
	gutil "github.com/gardener/gardener/pkg/utils/gardener"
	kutil "github.com/gardener/gardener/pkg/utils/kubernetes"
	"github.com/gardener/gardener/pkg/utils/managedresources"
	"github.com/gardener/gardener/pkg/utils/secrets"
	secretsmanager "github.com/gardener/gardener/pkg/utils/secrets/manager"
	"github.com/gardener/gardener/pkg/utils/version"

	"github.com/Masterminds/semver"
	appsv1 "k8s.io/api/apps/v1"
	autoscalingv1 "k8s.io/api/autoscaling/v1"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
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
	LabelRole = "scheduler"
	// BinPackingSchedulerName is the scheduler name that is used when the "bin-packing"
	// scheduling profile is configured.
	BinPackingSchedulerName = "bin-packing-scheduler"

	serviceName         = "kube-scheduler"
	secretNameServer    = "kube-scheduler-server"
	managedResourceName = "shoot-core-kube-scheduler"

	containerName   = v1beta1constants.DeploymentNameKubeScheduler
	portNameMetrics = "metrics"

	volumeNameClientCA      = "client-ca"
	volumeMountPathClientCA = "/var/lib/kube-scheduler-client-ca"
	fileNameClientCA        = "bundle.crt"

	volumeNameServer      = "kube-scheduler-server"
	volumeMountPathServer = "/var/lib/kube-scheduler-server"

	volumeNameConfig      = "kube-scheduler-config"
	volumeMountPathConfig = "/var/lib/kube-scheduler-config"

	dataKeyComponentConfig = "config.yaml"

	componentConfigTmpl = `apiVersion: {{ .apiVersion }}
kind: KubeSchedulerConfiguration
clientConnection:
  kubeconfig: ` + gutil.PathGenericKubeconfig + `
leaderElection:
  leaderElect: true
{{- if eq .profile "bin-packing" }}
profiles:
{{- if or (eq .apiVersion "kubescheduler.config.k8s.io/v1alpha2") (eq .apiVersion "kubescheduler.config.k8s.io/v1beta1") }}
- schedulerName: ` + BinPackingSchedulerName + `
  plugins:
    score:
      disabled:
      - name: NodeResourcesLeastAllocated
      - name: NodeResourcesBalancedAllocation
      enabled:
      - name: NodeResourcesMostAllocated
{{- else if or (eq .apiVersion "kubescheduler.config.k8s.io/v1beta2") (eq .apiVersion "kubescheduler.config.k8s.io/v1beta3") }}
- schedulerName: ` + BinPackingSchedulerName + `
  pluginConfig:
  - name: NodeResourcesFit
    args:
      scoringStrategy:
        type: MostAllocated
  plugins:
    score:
      disabled:
      - name: NodeResourcesBalancedAllocation
{{- end }}
{{- end }}`
)

// Interface contains functions for a kube-scheduler deployer.
type Interface interface {
	component.DeployWaiter
	component.MonitoringComponent
}

// New creates a new instance of DeployWaiter for the kube-scheduler.
func New(
	client client.Client,
	namespace string,
	secretsManager secretsmanager.Interface,
	version *semver.Version,
	image string,
	replicas int32,
	config *gardencorev1beta1.KubeSchedulerConfig,
) Interface {
	return &kubeScheduler{
		client:         client,
		namespace:      namespace,
		secretsManager: secretsManager,
		version:        version,
		image:          image,
		replicas:       replicas,
		config:         config,
	}
}

type kubeScheduler struct {
	client         client.Client
	namespace      string
	secretsManager secretsmanager.Interface
	version        *semver.Version
	image          string
	replicas       int32
	config         *gardencorev1beta1.KubeSchedulerConfig
}

func (k *kubeScheduler) Deploy(ctx context.Context) error {
	serverSecret, err := k.secretsManager.Generate(ctx, &secrets.CertificateSecretConfig{
		Name:                        secretNameServer,
		CommonName:                  v1beta1constants.DeploymentNameKubeScheduler,
		DNSNames:                    kutil.DNSNamesForService(serviceName, k.namespace),
		CertType:                    secrets.ServerCert,
		SkipPublishingCACertificate: true,
	}, secretsmanager.SignedByCA(v1beta1constants.SecretNameCACluster), secretsmanager.Rotate(secretsmanager.InPlace))
	if err != nil {
		return err
	}

	genericTokenKubeconfigSecret, found := k.secretsManager.Get(v1beta1constants.SecretNameGenericTokenKubeconfig)
	if !found {
		return fmt.Errorf("secret %q not found", v1beta1constants.SecretNameGenericTokenKubeconfig)
	}

	clientCASecret, found := k.secretsManager.Get(v1beta1constants.SecretNameCAClient)
	if !found {
		return fmt.Errorf("secret %q not found", v1beta1constants.SecretNameCAClient)
	}

	componentConfigYAML, err := k.computeComponentConfig()
	if err != nil {
		return err
	}

	configMap := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "kube-scheduler-config",
			Namespace: k.namespace,
		},
		Data: map[string]string{dataKeyComponentConfig: componentConfigYAML},
	}
	utilruntime.Must(kutil.MakeUnique(configMap))

	var (
		vpa               = k.emptyVPA()
		service           = k.emptyService()
		shootAccessSecret = k.newShootAccessSecret()
		deployment        = k.emptyDeployment()

		vpaUpdateMode    = vpaautoscalingv1.UpdateModeAuto
		controlledValues = vpaautoscalingv1.ContainerControlledValuesRequestsOnly

		port           int32 = 10259
		probeURIScheme       = corev1.URISchemeHTTPS
		env                  = k.computeEnvironmentVariables()
		command              = k.computeCommand(port)
	)

	if err := k.client.Create(ctx, configMap); kutil.IgnoreAlreadyExists(err) != nil {
		return err
	}

	if _, err := controllerutils.GetAndCreateOrMergePatch(ctx, k.client, service, func() error {
		service.Labels = getLabels()
		service.Spec.Selector = getLabels()
		service.Spec.Type = corev1.ServiceTypeClusterIP
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

	if err := shootAccessSecret.Reconcile(ctx, k.client); err != nil {
		return err
	}

	if _, err := controllerutils.GetAndCreateOrMergePatch(ctx, k.client, deployment, func() error {
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
						Env: env,
						Resources: corev1.ResourceRequirements{
							Requests: corev1.ResourceList{
								corev1.ResourceCPU:    resource.MustParse("23m"),
								corev1.ResourceMemory: resource.MustParse("64Mi"),
							},
						},
						VolumeMounts: []corev1.VolumeMount{
							{
								Name:      volumeNameClientCA,
								MountPath: volumeMountPathClientCA,
							},
							{
								Name:      volumeNameServer,
								MountPath: volumeMountPathServer,
							},
							{
								Name:      volumeNameConfig,
								MountPath: volumeMountPathConfig,
							},
						},
					},
				},
				Volumes: []corev1.Volume{
					{
						Name: volumeNameClientCA,
						VolumeSource: corev1.VolumeSource{
							Projected: &corev1.ProjectedVolumeSource{
								DefaultMode: pointer.Int32(420),
								Sources: []corev1.VolumeProjection{
									{
										Secret: &corev1.SecretProjection{
											LocalObjectReference: corev1.LocalObjectReference{
												Name: clientCASecret.Name,
											},
											Items: []corev1.KeyToPath{{
												Key:  secrets.DataKeyCertificateBundle,
												Path: fileNameClientCA,
											}},
										},
									},
								},
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
					{
						Name: volumeNameConfig,
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

		utilruntime.Must(gutil.InjectGenericKubeconfig(deployment, genericTokenKubeconfigSecret.Name, shootAccessSecret.Secret.Name))
		utilruntime.Must(references.InjectAnnotations(deployment))
		return nil
	}); err != nil {
		return err
	}

	if _, err := controllerutils.GetAndCreateOrMergePatch(ctx, k.client, vpa, func() error {
		vpa.Spec.TargetRef = &autoscalingv1.CrossVersionObjectReference{
			APIVersion: appsv1.SchemeGroupVersion.String(),
			Kind:       "Deployment",
			Name:       v1beta1constants.DeploymentNameKubeScheduler,
		}
		vpa.Spec.UpdatePolicy = &vpaautoscalingv1.PodUpdatePolicy{
			UpdateMode: &vpaUpdateMode,
		}
		vpa.Spec.ResourcePolicy = &vpaautoscalingv1.PodResourcePolicy{
			ContainerPolicies: []vpaautoscalingv1.ContainerResourcePolicy{
				{
					ContainerName: vpaautoscalingv1.DefaultContainerResourcePolicy,
					MinAllowed: corev1.ResourceList{
						corev1.ResourceCPU:    resource.MustParse("20m"),
						corev1.ResourceMemory: resource.MustParse("50Mi"),
					},
					MaxAllowed: corev1.ResourceList{
						corev1.ResourceCPU:    resource.MustParse("4"),
						corev1.ResourceMemory: resource.MustParse("10G"),
					},
					ControlledValues: &controlledValues,
				},
			},
		}
		return nil
	}); err != nil {
		return err
	}

	if err := k.reconcileShootResources(ctx, shootAccessSecret.ServiceAccountName); err != nil {
		return err
	}

	// TODO(rfranzke): Remove in a future release.
	return kutil.DeleteObject(ctx, k.client, &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: "kube-scheduler-server", Namespace: k.namespace}})
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

func (k *kubeScheduler) emptyVPA() *vpaautoscalingv1.VerticalPodAutoscaler {
	return &vpaautoscalingv1.VerticalPodAutoscaler{ObjectMeta: metav1.ObjectMeta{Name: "kube-scheduler-vpa", Namespace: k.namespace}}
}

func (k *kubeScheduler) emptyService() *corev1.Service {
	return &corev1.Service{ObjectMeta: metav1.ObjectMeta{Name: serviceName, Namespace: k.namespace}}
}

func (k *kubeScheduler) emptyDeployment() *appsv1.Deployment {
	return &appsv1.Deployment{ObjectMeta: metav1.ObjectMeta{Name: v1beta1constants.DeploymentNameKubeScheduler, Namespace: k.namespace}}
}

func (k *kubeScheduler) newShootAccessSecret() *gutil.ShootAccessSecret {
	return gutil.NewShootAccessSecret(v1beta1constants.DeploymentNameKubeScheduler, k.namespace)
}

func (k *kubeScheduler) reconcileShootResources(ctx context.Context, serviceAccountName string) error {
	var (
		registry = managedresources.NewRegistry(kubernetes.ShootScheme, kubernetes.ShootCodec, kubernetes.ShootSerializer)

		clusterRoleBinding1 = &rbacv1.ClusterRoleBinding{
			ObjectMeta: metav1.ObjectMeta{
				Name: "gardener.cloud:target:kube-scheduler",
			},
			RoleRef: rbacv1.RoleRef{
				APIGroup: rbacv1.GroupName,
				Kind:     "ClusterRole",
				Name:     "system:kube-scheduler",
			},
			Subjects: []rbacv1.Subject{{
				Kind:      rbacv1.ServiceAccountKind,
				Name:      serviceAccountName,
				Namespace: metav1.NamespaceSystem,
			}},
		}
		clusterRoleBinding2 = &rbacv1.ClusterRoleBinding{
			ObjectMeta: metav1.ObjectMeta{
				Name: "gardener.cloud:target:kube-scheduler-volume",
			},
			RoleRef: rbacv1.RoleRef{
				APIGroup: rbacv1.GroupName,
				Kind:     "ClusterRole",
				Name:     "system:volume-scheduler",
			},
			Subjects: []rbacv1.Subject{{
				Kind:      rbacv1.ServiceAccountKind,
				Name:      serviceAccountName,
				Namespace: metav1.NamespaceSystem,
			}},
		}
	)

	data, err := registry.AddAllAndSerialize(clusterRoleBinding1, clusterRoleBinding2)
	if err != nil {
		return err
	}

	return managedresources.CreateForShoot(ctx, k.client, k.namespace, managedResourceName, false, data)
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

func (k *kubeScheduler) computeComponentConfig() (string, error) {
	var apiVersion string
	if version.ConstraintK8sGreaterEqual123.Check(k.version) {
		apiVersion = "kubescheduler.config.k8s.io/v1beta3"
	} else if version.ConstraintK8sGreaterEqual122.Check(k.version) {
		apiVersion = "kubescheduler.config.k8s.io/v1beta2"
	} else if version.ConstraintK8sGreaterEqual119.Check(k.version) {
		apiVersion = "kubescheduler.config.k8s.io/v1beta1"
	} else if version.ConstraintK8sGreaterEqual118.Check(k.version) {
		apiVersion = "kubescheduler.config.k8s.io/v1alpha2"
	} else {
		apiVersion = "kubescheduler.config.k8s.io/v1alpha1"
	}

	profile := gardencorev1beta1.SchedulingProfileBalanced
	if k.config != nil && k.config.Profile != nil {
		profile = *k.config.Profile
	}

	var (
		componentConfigYAML bytes.Buffer
		values              = map[string]string{
			"apiVersion": apiVersion,
			"profile":    string(profile),
		}
	)
	if err := componentConfigTemplate.Execute(&componentConfigYAML, values); err != nil {
		return "", err
	}

	return componentConfigYAML.String(), nil
}

func (k *kubeScheduler) computeCommand(port int32) []string {
	var command []string

	command = append(command,
		"/usr/local/bin/kube-scheduler",
		fmt.Sprintf("--config=%s/%s", volumeMountPathConfig, dataKeyComponentConfig),
		"--authentication-kubeconfig="+gutil.PathGenericKubeconfig,
		"--authorization-kubeconfig="+gutil.PathGenericKubeconfig,
		fmt.Sprintf("--client-ca-file=%s/%s", volumeMountPathClientCA, fileNameClientCA),
		fmt.Sprintf("--tls-cert-file=%s/%s", volumeMountPathServer, secrets.DataKeyCertificate),
		fmt.Sprintf("--tls-private-key-file=%s/%s", volumeMountPathServer, secrets.DataKeyPrivateKey),
		fmt.Sprintf("--secure-port=%d", port),
	)

	if version.ConstraintK8sLessEqual122.Check(k.version) {
		command = append(command, "--port=0")
	}

	if k.config != nil {
		command = append(command, kutil.FeatureGatesToCommandLineParameter(k.config.FeatureGates))
	}

	command = append(command, "--v=2")
	return command
}

var componentConfigTemplate *template.Template

func init() {
	var err error

	componentConfigTemplate, err = template.New("config").Parse(componentConfigTmpl)
	utilruntime.Must(err)
}

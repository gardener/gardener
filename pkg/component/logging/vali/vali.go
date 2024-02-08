// Copyright 2023 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

package vali

import (
	"bytes"
	"context"
	_ "embed"
	"fmt"
	"text/template"

	"github.com/Masterminds/sprig/v3"
	hvpav1alpha1 "github.com/gardener/hvpa-controller/api/v1alpha1"
	appsv1 "k8s.io/api/apps/v1"
	autoscalingv2beta1 "k8s.io/api/autoscaling/v2beta1"
	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	vpaautoscalingv1 "k8s.io/autoscaler/vertical-pod-autoscaler/pkg/apis/autoscaling.k8s.io/v1"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"

	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	resourcesv1alpha1 "github.com/gardener/gardener/pkg/apis/resources/v1alpha1"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	"github.com/gardener/gardener/pkg/component"
	kubeapiserverconstants "github.com/gardener/gardener/pkg/component/kubeapiserver/constants"
	"github.com/gardener/gardener/pkg/resourcemanager/controller/garbagecollector/references"
	"github.com/gardener/gardener/pkg/utils"
	gardenerutils "github.com/gardener/gardener/pkg/utils/gardener"
	kubernetesutils "github.com/gardener/gardener/pkg/utils/kubernetes"
	"github.com/gardener/gardener/pkg/utils/managedresources"
	"github.com/gardener/gardener/pkg/utils/secrets"
	secretsmanager "github.com/gardener/gardener/pkg/utils/secrets/manager"
)

const (
	// ValiPort is the port exposed by the vali.
	ValiPort = 3100
	// ServiceName is the name of the logging service.
	ServiceName = "logging"

	// ManagedResourceNameRuntime is the name of the managed resource which deploys Vali statefulSet.
	ManagedResourceNameRuntime = "vali"
	managedResourceNameTarget  = "vali-target"

	valiName                      = "vali"
	valiServiceName               = "logging"
	valiMetricsPortName           = "metrics"
	valiUserAndGroupId      int64 = 10001
	valiConfigMapVolumeName       = "config"
	valiPVCName                   = "vali"
	valiDataKeyConfig             = "vali.yaml"
	valiDataKeyInitScript         = "vali-init.sh"
	valiMountPathData             = "/data"
	valiMountPathConfig           = "/etc/vali"
	valiMountPathInitScript       = "/"

	valitailName            = "gardener-valitail"
	valitailClusterRoleName = "gardener.cloud:logging:valitail"
	// ValitailTokenSecretName is the name of a secret in the kube-system namespace in the target cluster containing
	// valitail's token for communication with the kube-apiserver.
	ValitailTokenSecretName = valitailName

	curatorName            = "curator"
	curatorPort            = 2718
	curatorMetricsPortName = "curatormetrics"
	curatorDataKeyConfig   = "curator.yaml"

	telegrafName               = "telegraf"
	telegrafServicePort        = 9273
	telegrafDataKeyConfig      = "telegraf.conf"
	telegrafDataKeyStartScript = "start.sh"
	telegrafVolumeMountPath    = "/etc/telegraf"
	telegrafVolumeName         = "telegraf-config-volume"

	kubeRBACProxyName = "kube-rbac-proxy"
	kubeRBACProxyPort = 8080

	initLargeDirName = "init-large-dir"
)

var (
	//go:embed templates/curator-config.yaml
	curatorConfig string

	//go:embed templates/vali-init.sh
	valiInitScript string

	//go:embed templates/vali-config.yaml
	valiConfig string

	//go:embed templates/telegraf-config.tpl
	telegrafConfigTplContent string
	telegrafConfigTemplate   *template.Template

	//go:embed templates/telegraf-start.sh.tpl
	telegrafStartScriptTplContent string
	telegrafStartScriptTemplate   *template.Template
)

func init() {
	telegrafStartScriptTemplate = template.Must(template.New("telegraf-config").Funcs(sprig.TxtFuncMap()).Parse(telegrafStartScriptTplContent))
	telegrafConfigTemplate = template.Must(template.New("telegraf-start").Funcs(sprig.TxtFuncMap()).Parse(telegrafConfigTplContent))
}

// Interface contains functions for a vali deployer.
type Interface interface {
	component.Deployer
	component.MonitoringComponent
}

// Values are the values for the Vali.
type Values struct {
	ValiImage          string
	CuratorImage       string
	TelegrafImage      string
	KubeRBACProxyImage string
	InitLargeDirImage  string

	ClusterType             component.ClusterType
	Replicas                int32
	PriorityClassName       string
	IngressHost             string
	ShootNodeLoggingEnabled bool
	HVPAEnabled             bool
	Storage                 *resource.Quantity
	MaintenanceTimeWindow   *hvpav1alpha1.MaintenanceTimeWindow
}

type vali struct {
	client         client.Client
	namespace      string
	secretsManager secretsmanager.Interface
	values         Values
}

// New creates a new instance of Vali deployer.
func New(
	client client.Client,
	namespace string,
	secretsManager secretsmanager.Interface,
	values Values,
) Interface {
	return &vali{
		client:         client,
		namespace:      namespace,
		secretsManager: secretsManager,
		values:         values,
	}
}

func (v *vali) Deploy(ctx context.Context) error {
	var (
		registry  = managedresources.NewRegistry(kubernetes.SeedScheme, kubernetes.SeedCodec, kubernetes.SeedSerializer)
		resources []client.Object
	)

	if v.values.Storage != nil {
		if err := v.resizeOrDeleteValiDataVolumeIfStorageNotTheSame(ctx); err != nil {
			return err
		}
	}

	if v.values.HVPAEnabled && v.values.Replicas > 0 {
		resources = append(resources, v.getHVPA())
	}

	var (
		telegrafConfigMapName            string
		genericTokenKubeconfigSecretName string
		valitailShootAccessSecret        = v.newValitailShootAccessSecret()
		kubeRBACProxyShootAccessSecret   = v.newKubeRBACProxyShootAccessSecret()
	)

	if v.values.ShootNodeLoggingEnabled {
		if err := valitailShootAccessSecret.Reconcile(ctx, v.client); err != nil {
			return err
		}
		if err := kubeRBACProxyShootAccessSecret.Reconcile(ctx, v.client); err != nil {
			return err
		}

		ingressTLSSecret, err := v.secretsManager.Generate(ctx, &secrets.CertificateSecretConfig{
			Name:                        "vali-tls",
			CommonName:                  v.values.IngressHost,
			Organization:                []string{"gardener.cloud:monitoring:ingress"},
			DNSNames:                    []string{v.values.IngressHost},
			CertType:                    secrets.ServerCert,
			Validity:                    ptr.To(v1beta1constants.IngressTLSCertificateValidity),
			SkipPublishingCACertificate: true,
		}, secretsmanager.SignedByCA(v1beta1constants.SecretNameCACluster))
		if err != nil {
			return err
		}

		telegrafConfigMap, err := v.getTelegrafConfigMap()
		if err != nil {
			return err
		}
		telegrafConfigMapName = telegrafConfigMap.Name

		genericTokenKubeconfigSecret, found := v.secretsManager.Get(v1beta1constants.SecretNameGenericTokenKubeconfig)
		if !found {
			return fmt.Errorf("secret %q not found", v1beta1constants.SecretNameGenericTokenKubeconfig)
		}
		genericTokenKubeconfigSecretName = genericTokenKubeconfigSecret.Name

		resources = append(resources,
			v.getIngress(ingressTLSSecret.Name),
			telegrafConfigMap,
		)

		resourcesTarget, err := managedresources.
			NewRegistry(kubernetes.ShootScheme, kubernetes.ShootCodec, kubernetes.ShootSerializer).
			AddAllAndSerialize(
				v.getKubeRBACProxyClusterRoleBinding(kubeRBACProxyShootAccessSecret.ServiceAccountName),
				v.getValitailClusterRole(),
				v.getValitailClusterRoleBinding(valitailShootAccessSecret.ServiceAccountName),
			)
		if err != nil {
			return err
		}

		if err := managedresources.CreateForShoot(ctx, v.client, v.namespace, managedResourceNameTarget, managedresources.LabelValueGardener, false, resourcesTarget); err != nil {
			return err
		}
	} else {
		if err := managedresources.DeleteForShoot(ctx, v.client, v.namespace, managedResourceNameTarget); err != nil {
			return err
		}

		if err := kubernetesutils.DeleteObjects(ctx, v.client,
			valitailShootAccessSecret.Secret,
			kubeRBACProxyShootAccessSecret.Secret,
		); err != nil {
			return err
		}
	}

	valiConfigMap := v.getValiConfigMap()

	resources = append(resources,
		valiConfigMap,
		v.getService(),
		v.getStatefulSet(valiConfigMap.Name, telegrafConfigMapName, genericTokenKubeconfigSecretName),
	)

	if err := registry.Add(resources...); err != nil {
		return err
	}

	return managedresources.CreateForSeed(ctx, v.client, v.namespace, ManagedResourceNameRuntime, false, registry.SerializedObjects())
}

func (v *vali) Destroy(ctx context.Context) error {
	if err := managedresources.DeleteForShoot(ctx, v.client, v.namespace, managedResourceNameTarget); err != nil {
		return err
	}

	if err := managedresources.DeleteForSeed(ctx, v.client, v.namespace, ManagedResourceNameRuntime); err != nil {
		return err
	}

	return kubernetesutils.DeleteObjects(ctx, v.client,
		v.newValitailShootAccessSecret().Secret,
		v.newKubeRBACProxyShootAccessSecret().Secret,
	)
}

func (v *vali) newValitailShootAccessSecret() *gardenerutils.AccessSecret {
	return gardenerutils.NewShootAccessSecret("valitail", v.namespace).
		WithServiceAccountName(valitailName).
		WithTokenExpirationDuration("720h").
		WithTargetSecret(ValitailTokenSecretName, metav1.NamespaceSystem)
}

func (v *vali) newKubeRBACProxyShootAccessSecret() *gardenerutils.AccessSecret {
	return gardenerutils.NewShootAccessSecret(kubeRBACProxyName, v.namespace)
}

func (v *vali) getHVPA() *hvpav1alpha1.Hvpa {
	var (
		controlledValues   = vpaautoscalingv1.ContainerControlledValuesRequestsOnly
		containerPolicyOff = vpaautoscalingv1.ContainerScalingModeOff

		hvpa = &hvpav1alpha1.Hvpa{
			ObjectMeta: metav1.ObjectMeta{
				Name:      valiName,
				Namespace: v.namespace,
				Labels:    getLabels(),
			},
			Spec: hvpav1alpha1.HvpaSpec{
				Replicas: ptr.To(v.values.Replicas),
				Hpa: hvpav1alpha1.HpaSpec{
					Selector: &metav1.LabelSelector{
						MatchLabels: map[string]string{
							v1beta1constants.LabelRole: valiName + "-hpa",
						},
					},
					Deploy: false,
					Template: hvpav1alpha1.HpaTemplate{
						ObjectMeta: metav1.ObjectMeta{
							Labels: map[string]string{
								v1beta1constants.LabelRole: valiName + "-hpa",
							},
						},
						Spec: hvpav1alpha1.HpaTemplateSpec{
							MinReplicas: ptr.To(v.values.Replicas),
							MaxReplicas: v.values.Replicas,
							Metrics: []autoscalingv2beta1.MetricSpec{
								{
									Type: autoscalingv2beta1.ResourceMetricSourceType,
									Resource: &autoscalingv2beta1.ResourceMetricSource{
										Name:                     corev1.ResourceCPU,
										TargetAverageUtilization: ptr.To(int32(80)),
									},
								},
								{
									Type: autoscalingv2beta1.ResourceMetricSourceType,
									Resource: &autoscalingv2beta1.ResourceMetricSource{
										Name:                     corev1.ResourceMemory,
										TargetAverageUtilization: ptr.To(int32(80)),
									},
								},
							},
						},
					},
				},
				Vpa: hvpav1alpha1.VpaSpec{
					Selector: &metav1.LabelSelector{
						MatchLabels: map[string]string{
							v1beta1constants.LabelRole: valiName + "vpa",
						},
					},
					Deploy: true,
					ScaleUp: hvpav1alpha1.ScaleType{
						UpdatePolicy: hvpav1alpha1.UpdatePolicy{
							UpdateMode: ptr.To(hvpav1alpha1.UpdateModeAuto),
						},
						StabilizationDuration: ptr.To("5m"),
						MinChange: hvpav1alpha1.ScaleParams{
							CPU: hvpav1alpha1.ChangeParams{
								Value:      ptr.To("100m"),
								Percentage: ptr.To(int32(80)),
							},
							Memory: hvpav1alpha1.ChangeParams{
								Value:      ptr.To("300M"),
								Percentage: ptr.To(int32(80)),
							},
						},
					},
					ScaleDown: hvpav1alpha1.ScaleType{
						UpdatePolicy: hvpav1alpha1.UpdatePolicy{
							UpdateMode: ptr.To(hvpav1alpha1.UpdateModeAuto),
						},
						StabilizationDuration: ptr.To("168h"),
						MinChange: hvpav1alpha1.ScaleParams{
							CPU: hvpav1alpha1.ChangeParams{
								Value:      ptr.To("200m"),
								Percentage: ptr.To(int32(80)),
							},
							Memory: hvpav1alpha1.ChangeParams{
								Value:      ptr.To("500M"),
								Percentage: ptr.To(int32(80)),
							},
						},
					},
					LimitsRequestsGapScaleParams: hvpav1alpha1.ScaleParams{
						CPU: hvpav1alpha1.ChangeParams{
							Value:      ptr.To("300m"),
							Percentage: ptr.To(int32(40)),
						},
						Memory: hvpav1alpha1.ChangeParams{
							Value:      ptr.To("1G"),
							Percentage: ptr.To(int32(40)),
						},
					},
					Template: hvpav1alpha1.VpaTemplate{
						ObjectMeta: metav1.ObjectMeta{
							Labels: map[string]string{
								v1beta1constants.LabelRole: valiName + "vpa",
							},
						},
						Spec: hvpav1alpha1.VpaTemplateSpec{
							ResourcePolicy: &vpaautoscalingv1.PodResourcePolicy{
								ContainerPolicies: []vpaautoscalingv1.ContainerResourcePolicy{
									{
										ContainerName: valiName,
										MinAllowed: corev1.ResourceList{
											corev1.ResourceMemory: resource.MustParse("300M"),
										},
										MaxAllowed: corev1.ResourceList{
											corev1.ResourceCPU:    resource.MustParse("800m"),
											corev1.ResourceMemory: resource.MustParse("3Gi"),
										},
										ControlledValues: &controlledValues,
									},
									{
										ContainerName:    curatorName,
										Mode:             &containerPolicyOff,
										ControlledValues: &controlledValues,
									},
									{
										ContainerName:    initLargeDirName,
										Mode:             &containerPolicyOff,
										ControlledValues: &controlledValues,
									},
								},
							},
						},
					},
				},
				WeightBasedScalingIntervals: []hvpav1alpha1.WeightBasedScalingInterval{{
					VpaWeight:         hvpav1alpha1.VpaOnly,
					StartReplicaCount: v.values.Replicas,
					LastReplicaCount:  v.values.Replicas,
				}},
				TargetRef: &autoscalingv2beta1.CrossVersionObjectReference{
					APIVersion: appsv1.SchemeGroupVersion.String(),
					Kind:       "StatefulSet",
					Name:       valiName,
				},
			},
		}
	)

	if v.values.MaintenanceTimeWindow != nil {
		hvpa.Spec.MaintenanceTimeWindow = v.values.MaintenanceTimeWindow
		hvpa.Spec.Vpa.ScaleDown.UpdatePolicy.UpdateMode = ptr.To(hvpav1alpha1.UpdateModeMaintenanceWindow)
	}

	if v.values.ShootNodeLoggingEnabled {
		hvpa.Spec.Vpa.Template.Spec.ResourcePolicy.ContainerPolicies = append(hvpa.Spec.Vpa.Template.Spec.ResourcePolicy.ContainerPolicies,
			vpaautoscalingv1.ContainerResourcePolicy{
				ContainerName:    kubeRBACProxyName,
				Mode:             &containerPolicyOff,
				ControlledValues: &controlledValues,
			},
			vpaautoscalingv1.ContainerResourcePolicy{
				ContainerName:    telegrafName,
				Mode:             &containerPolicyOff,
				ControlledValues: &controlledValues,
			},
		)
	}

	return hvpa
}

func (v *vali) getIngress(secretName string) *networkingv1.Ingress {
	pathType := networkingv1.PathTypePrefix

	ingress := &networkingv1.Ingress{
		ObjectMeta: metav1.ObjectMeta{
			Name:      valiName,
			Namespace: v.namespace,
			Labels:    getLabels(),
		},
		Spec: networkingv1.IngressSpec{
			IngressClassName: ptr.To(v1beta1constants.SeedNginxIngressClass),
			TLS: []networkingv1.IngressTLS{{
				SecretName: secretName,
				Hosts:      []string{v.values.IngressHost},
			}},
			Rules: []networkingv1.IngressRule{{
				Host: v.values.IngressHost,
				IngressRuleValue: networkingv1.IngressRuleValue{
					HTTP: &networkingv1.HTTPIngressRuleValue{
						Paths: []networkingv1.HTTPIngressPath{{
							Backend: networkingv1.IngressBackend{
								Service: &networkingv1.IngressServiceBackend{
									Name: valiServiceName,
									Port: networkingv1.ServiceBackendPort{Number: kubeRBACProxyPort},
								},
							},
							Path:     "/vali/api/v1/push",
							PathType: &pathType,
						}},
					},
				},
			}},
		},
	}

	return ingress
}

func (v *vali) getService() *corev1.Service {
	var (
		service = &corev1.Service{
			ObjectMeta: metav1.ObjectMeta{
				Name:        ServiceName,
				Namespace:   v.namespace,
				Labels:      getLabels(),
				Annotations: map[string]string{},
			},
			Spec: corev1.ServiceSpec{
				Type:     corev1.ServiceTypeClusterIP,
				Selector: getLabels(),
				Ports: []corev1.ServicePort{{
					Name:       valiMetricsPortName,
					Port:       ValiPort,
					Protocol:   corev1.ProtocolTCP,
					TargetPort: intstr.FromInt32(ValiPort),
				}},
			},
		}

		networkPolicyPorts = []networkingv1.NetworkPolicyPort{{
			Port:     utils.IntStrPtrFromInt32(ValiPort),
			Protocol: ptr.To(corev1.ProtocolTCP),
		}}
	)

	if v.values.ShootNodeLoggingEnabled {
		service.Spec.Ports = append(service.Spec.Ports,
			corev1.ServicePort{
				Port:       kubeRBACProxyPort,
				Protocol:   corev1.ProtocolTCP,
				TargetPort: intstr.FromInt32(kubeRBACProxyPort),
				Name:       "external",
			},
			corev1.ServicePort{
				Port:       telegrafServicePort,
				Protocol:   corev1.ProtocolTCP,
				TargetPort: intstr.FromInt32(telegrafServicePort),
				Name:       telegrafName,
			},
		)

		networkPolicyPorts = append(networkPolicyPorts, networkingv1.NetworkPolicyPort{
			Port:     utils.IntStrPtrFromInt32(telegrafServicePort),
			Protocol: ptr.To(corev1.ProtocolTCP),
		})
	}

	switch v.values.ClusterType {
	case component.ClusterTypeSeed:
		utilruntime.Must(gardenerutils.InjectNetworkPolicyAnnotationsForSeedScrapeTargets(service, networkPolicyPorts...))
	case component.ClusterTypeShoot:
		utilruntime.Must(gardenerutils.InjectNetworkPolicyAnnotationsForScrapeTargets(service, networkPolicyPorts...))
		utilruntime.Must(gardenerutils.InjectNetworkPolicyNamespaceSelectors(service, metav1.LabelSelector{MatchLabels: map[string]string{corev1.LabelMetadataName: v1beta1constants.GardenNamespace}}))
		metav1.SetMetaDataAnnotation(&service.ObjectMeta, resourcesv1alpha1.NetworkingPodLabelSelectorNamespaceAlias, v1beta1constants.LabelNetworkPolicyShootNamespaceAlias)
	}

	return service
}

func (v *vali) getValiConfigMap() *corev1.ConfigMap {
	configMap := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "vali-config",
			Namespace: v.namespace,
			Labels:    getLabels(),
		},
		Data: map[string]string{
			valiDataKeyConfig:     valiConfig,
			curatorDataKeyConfig:  curatorConfig,
			valiDataKeyInitScript: valiInitScript,
		},
	}

	utilruntime.Must(kubernetesutils.MakeUnique(configMap))
	return configMap
}

func (v *vali) getTelegrafConfigMap() (*corev1.ConfigMap, error) {
	var telegrafConfig bytes.Buffer
	if err := telegrafConfigTemplate.Execute(&telegrafConfig, map[string]interface{}{"ListenPort": telegrafServicePort}); err != nil {
		return nil, fmt.Errorf("failed to render telegraf configuration: %w", err)
	}

	var telegrafStartScript bytes.Buffer
	if err := telegrafStartScriptTemplate.Execute(&telegrafStartScript, map[string]interface{}{"KubeRBACProxyPort": kubeRBACProxyPort}); err != nil {
		return nil, fmt.Errorf("failed to render telegraf start script: %w", err)
	}

	configMap := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "telegraf-config",
			Namespace: v.namespace,
			Labels:    getLabels(),
		},
		Data: map[string]string{
			telegrafDataKeyConfig:      telegrafConfig.String(),
			telegrafDataKeyStartScript: telegrafStartScript.String(),
		},
	}

	utilruntime.Must(kubernetesutils.MakeUnique(configMap))
	return configMap, nil
}

func (v *vali) getStatefulSet(valiConfigMapName, telegrafConfigMapName, genericTokenKubeconfigSecretName string) *appsv1.StatefulSet {
	var (
		fsGroupChangeOnRootMismatch = corev1.FSGroupChangeOnRootMismatch

		statefulSet = &appsv1.StatefulSet{
			ObjectMeta: metav1.ObjectMeta{
				Name:      valiName,
				Namespace: v.namespace,
				Labels:    getLabels(),
			},
			Spec: appsv1.StatefulSetSpec{
				Replicas: ptr.To(v.values.Replicas),
				Selector: &metav1.LabelSelector{
					MatchLabels: getLabels(),
				},
				Template: corev1.PodTemplateSpec{
					ObjectMeta: metav1.ObjectMeta{
						Labels: getLabels(),
					},
					Spec: corev1.PodSpec{
						AutomountServiceAccountToken: ptr.To(false),
						SecurityContext: &corev1.PodSecurityContext{
							FSGroup:             ptr.To(valiUserAndGroupId),
							FSGroupChangePolicy: &fsGroupChangeOnRootMismatch,
						},
						PriorityClassName: v.values.PriorityClassName,
						InitContainers: []corev1.Container{
							{
								Name:  initLargeDirName,
								Image: v.values.InitLargeDirImage,
								Command: []string{
									"bash",
									"-c",
									valiMountPathInitScript + valiDataKeyInitScript + " || true",
								},
								SecurityContext: &corev1.SecurityContext{
									Privileged:   ptr.To(true),
									RunAsUser:    ptr.To(int64(0)),
									RunAsNonRoot: ptr.To(false),
									RunAsGroup:   ptr.To(int64(0)),
								},
								VolumeMounts: []corev1.VolumeMount{
									{
										MountPath: valiMountPathData,
										Name:      valiPVCName,
									},
									{
										MountPath: valiMountPathInitScript + valiDataKeyInitScript,
										SubPath:   valiDataKeyInitScript,
										Name:      valiConfigMapVolumeName,
									},
								},
							},
						},
						Containers: []corev1.Container{
							{
								Name:  valiName,
								Image: v.values.ValiImage,
								Args:  []string{fmt.Sprintf("-config.file=%s/%s", valiMountPathConfig, valiDataKeyConfig)},
								VolumeMounts: []corev1.VolumeMount{
									{
										Name:      valiConfigMapVolumeName,
										MountPath: valiMountPathConfig + "/" + valiDataKeyConfig,
										SubPath:   valiDataKeyConfig,
									},
									{
										Name:      valiPVCName,
										MountPath: valiMountPathData,
									},
								},
								Ports: []corev1.ContainerPort{{
									Name:          valiMetricsPortName,
									ContainerPort: ValiPort,
									Protocol:      corev1.ProtocolTCP,
								}},
								LivenessProbe: &corev1.Probe{
									ProbeHandler: corev1.ProbeHandler{
										HTTPGet: &corev1.HTTPGetAction{
											Path: "/ready",
											Port: intstr.FromString(valiMetricsPortName),
										},
									},
									InitialDelaySeconds: 120,
									FailureThreshold:    5,
								},
								ReadinessProbe: &corev1.Probe{
									ProbeHandler: corev1.ProbeHandler{
										HTTPGet: &corev1.HTTPGetAction{
											Path: "/ready",
											Port: intstr.FromString(valiMetricsPortName),
										},
									},
									FailureThreshold: 7,
								},
								Resources: corev1.ResourceRequirements{
									Requests: corev1.ResourceList{
										corev1.ResourceCPU:    resource.MustParse("200m"),
										corev1.ResourceMemory: resource.MustParse("300Mi"),
									},
									Limits: corev1.ResourceList{
										corev1.ResourceMemory: resource.MustParse("3Gi"),
									},
								},
								SecurityContext: &corev1.SecurityContext{
									RunAsUser:              ptr.To(valiUserAndGroupId),
									RunAsGroup:             ptr.To(valiUserAndGroupId),
									RunAsNonRoot:           ptr.To(true),
									ReadOnlyRootFilesystem: ptr.To(true),
								},
							},
							{
								Name:  curatorName,
								Image: v.values.CuratorImage,
								Args:  []string{fmt.Sprintf("-config=%s/%s", valiMountPathConfig, curatorDataKeyConfig)},
								Ports: []corev1.ContainerPort{{
									Name:          curatorMetricsPortName,
									ContainerPort: curatorPort,
									Protocol:      corev1.ProtocolTCP,
								}},
								VolumeMounts: []corev1.VolumeMount{
									{
										Name:      valiConfigMapVolumeName,
										MountPath: valiMountPathConfig + "/" + curatorDataKeyConfig,
										SubPath:   curatorDataKeyConfig,
									},
									{
										Name:      valiPVCName,
										MountPath: valiMountPathData,
									},
								},
								Resources: corev1.ResourceRequirements{
									Requests: corev1.ResourceList{
										corev1.ResourceCPU:    resource.MustParse("10m"),
										corev1.ResourceMemory: resource.MustParse("12Mi"),
									},
									Limits: corev1.ResourceList{
										corev1.ResourceMemory: resource.MustParse("700Mi"),
									},
								},
								SecurityContext: &corev1.SecurityContext{
									RunAsUser:              ptr.To(valiUserAndGroupId),
									RunAsGroup:             ptr.To(valiUserAndGroupId),
									RunAsNonRoot:           ptr.To(true),
									ReadOnlyRootFilesystem: ptr.To(true),
								},
							},
						},
						Volumes: []corev1.Volume{{
							Name: valiConfigMapVolumeName,
							VolumeSource: corev1.VolumeSource{
								ConfigMap: &corev1.ConfigMapVolumeSource{
									LocalObjectReference: corev1.LocalObjectReference{
										Name: valiConfigMapName,
									},
									DefaultMode: ptr.To(int32(0520)),
								},
							},
						}},
					},
				},
				VolumeClaimTemplates: []corev1.PersistentVolumeClaim{{
					ObjectMeta: metav1.ObjectMeta{
						Name: valiPVCName,
					},
					Spec: corev1.PersistentVolumeClaimSpec{
						AccessModes: []corev1.PersistentVolumeAccessMode{
							corev1.ReadWriteOnce,
						},
						Resources: corev1.ResourceRequirements{
							Requests: map[corev1.ResourceName]resource.Quantity{
								corev1.ResourceStorage: resource.MustParse("30Gi"),
							},
						},
					},
				}},
			},
		}
	)

	if v.values.Storage != nil {
		statefulSet.Spec.VolumeClaimTemplates[0].Spec.Resources.Requests[corev1.ResourceStorage] = *v.values.Storage
	}

	if v.values.ShootNodeLoggingEnabled {
		statefulSet.Spec.Template.Labels[v1beta1constants.LabelNetworkPolicyToDNS] = v1beta1constants.LabelNetworkPolicyAllowed
		statefulSet.Spec.Template.Labels[gardenerutils.NetworkPolicyLabel(v1beta1constants.DeploymentNameKubeAPIServer, kubeapiserverconstants.Port)] = v1beta1constants.LabelNetworkPolicyAllowed
		statefulSet.Spec.Template.Spec.Containers = append(statefulSet.Spec.Template.Spec.Containers,
			corev1.Container{
				Name:  kubeRBACProxyName,
				Image: v.values.KubeRBACProxyImage,
				Args: []string{
					fmt.Sprintf("--insecure-listen-address=0.0.0.0:%d", kubeRBACProxyPort),
					fmt.Sprintf("--upstream=http://127.0.0.1:%d/", ValiPort),
					"--kubeconfig=" + gardenerutils.PathGenericKubeconfig,
					"--logtostderr=true",
					"--v=6",
				},
				Resources: corev1.ResourceRequirements{
					Requests: corev1.ResourceList{
						corev1.ResourceCPU:    resource.MustParse("50m"),
						corev1.ResourceMemory: resource.MustParse("50Mi"),
					},
					Limits: corev1.ResourceList{
						corev1.ResourceMemory: resource.MustParse("150Mi"),
					},
				},
				Ports: []corev1.ContainerPort{{
					Name:          kubeRBACProxyName,
					ContainerPort: kubeRBACProxyPort,
					Protocol:      corev1.ProtocolTCP,
				}},
				SecurityContext: &corev1.SecurityContext{
					RunAsUser:              ptr.To(int64(65532)),
					RunAsGroup:             ptr.To(int64(65534)),
					RunAsNonRoot:           ptr.To(true),
					ReadOnlyRootFilesystem: ptr.To(true),
				},
			},
			corev1.Container{
				Name:  telegrafName,
				Image: v.values.TelegrafImage,
				Command: []string{
					"/bin/bash",
					"-c",
					`
trap 'kill %1; wait' SIGTERM
bash ` + telegrafVolumeMountPath + `/` + telegrafDataKeyStartScript + ` &
wait
`,
				},
				Resources: corev1.ResourceRequirements{
					Requests: corev1.ResourceList{
						corev1.ResourceCPU:    resource.MustParse("5m"),
						corev1.ResourceMemory: resource.MustParse("35Mi"),
					},
					Limits: corev1.ResourceList{
						corev1.ResourceMemory: resource.MustParse("350Mi"),
					},
				},
				SecurityContext: &corev1.SecurityContext{
					Capabilities: &corev1.Capabilities{
						Add: []corev1.Capability{"NET_ADMIN"},
					},
				},
				Ports: []corev1.ContainerPort{{
					Name:          telegrafName,
					ContainerPort: telegrafServicePort,
					Protocol:      corev1.ProtocolTCP,
				}},
				VolumeMounts: []corev1.VolumeMount{
					{
						Name:      telegrafVolumeName,
						MountPath: telegrafVolumeMountPath + "/" + telegrafDataKeyConfig,
						SubPath:   telegrafDataKeyConfig,
						ReadOnly:  true,
					},
					{
						Name:      telegrafVolumeName,
						MountPath: telegrafVolumeMountPath + "/" + telegrafDataKeyStartScript,
						SubPath:   telegrafDataKeyStartScript,
						ReadOnly:  true,
					},
				},
			},
		)
		statefulSet.Spec.Template.Spec.Volumes = append(statefulSet.Spec.Template.Spec.Volumes, corev1.Volume{
			Name: telegrafVolumeName,
			VolumeSource: corev1.VolumeSource{
				ConfigMap: &corev1.ConfigMapVolumeSource{
					LocalObjectReference: corev1.LocalObjectReference{
						Name: telegrafConfigMapName,
					},
				},
			},
		})

		utilruntime.Must(gardenerutils.InjectGenericKubeconfig(statefulSet, genericTokenKubeconfigSecretName, "shoot-access-"+kubeRBACProxyName, kubeRBACProxyName))
	}

	utilruntime.Must(references.InjectAnnotations(statefulSet))
	return statefulSet
}

func (v *vali) getKubeRBACProxyClusterRoleBinding(serviceAccountName string) *rbacv1.ClusterRoleBinding {
	return &rbacv1.ClusterRoleBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name:   "gardener.cloud:logging:kube-rbac-proxy",
			Labels: map[string]string{v1beta1constants.LabelApp: kubeRBACProxyName},
		},
		RoleRef: rbacv1.RoleRef{
			APIGroup: rbacv1.GroupName,
			Kind:     "ClusterRole",
			Name:     "system:auth-delegator",
		},
		Subjects: []rbacv1.Subject{{
			Kind:      rbacv1.ServiceAccountKind,
			Name:      serviceAccountName,
			Namespace: metav1.NamespaceSystem,
		}},
	}
}

func (v *vali) getValitailClusterRole() *rbacv1.ClusterRole {
	return &rbacv1.ClusterRole{
		ObjectMeta: metav1.ObjectMeta{
			Name:   valitailClusterRoleName,
			Labels: map[string]string{v1beta1constants.LabelApp: valitailName},
		},
		Rules: []rbacv1.PolicyRule{
			{
				APIGroups: []string{""},
				Resources: []string{
					"nodes",
					"nodes/proxy",
					"services",
					"endpoints",
					"pods",
				},
				Verbs: []string{
					"get",
					"list",
					"watch",
				},
			},
			{
				NonResourceURLs: []string{"/vali/api/v1/push"},
				Verbs:           []string{"create"},
			},
		},
	}
}

func (v *vali) getValitailClusterRoleBinding(serviceAccountName string) *rbacv1.ClusterRoleBinding {
	return &rbacv1.ClusterRoleBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name:   "gardener.cloud:logging:valitail",
			Labels: map[string]string{v1beta1constants.LabelApp: valitailName},
		},
		RoleRef: rbacv1.RoleRef{
			APIGroup: rbacv1.GroupName,
			Kind:     "ClusterRole",
			Name:     valitailClusterRoleName,
		},
		Subjects: []rbacv1.Subject{{
			Kind:      rbacv1.ServiceAccountKind,
			Name:      serviceAccountName,
			Namespace: metav1.NamespaceSystem,
		}},
	}
}

func getLabels() map[string]string {
	return map[string]string{
		v1beta1constants.GardenRole: v1beta1constants.GardenRoleLogging,
		v1beta1constants.LabelRole:  "logging",
		v1beta1constants.LabelApp:   valiName,
	}
}

// resizeOrDeleteValiDataVolumeIfStorageNotTheSame updates the Vali PVC if passed storage value is not the same as the
// current one.
// Caution: If the passed storage capacity is less than the current one the existing PVC and its PV will be deleted.
func (v *vali) resizeOrDeleteValiDataVolumeIfStorageNotTheSame(ctx context.Context) error {
	managedResource := &resourcesv1alpha1.ManagedResource{ObjectMeta: metav1.ObjectMeta{Name: ManagedResourceNameRuntime, Namespace: v.namespace}}
	addOrRemoveIgnoreAnnotationFromManagedResource := func(addIgnoreAnnotation bool) error {
		// In order to not create the managed resource here first check if exists.
		if err := v.client.Get(ctx, client.ObjectKeyFromObject(managedResource), managedResource); err != nil {
			if !apierrors.IsNotFound(err) {
				return err
			}
			return nil
		}
		patch := client.MergeFrom(managedResource.DeepCopy())

		if addIgnoreAnnotation {
			metav1.SetMetaDataAnnotation(&managedResource.ObjectMeta, resourcesv1alpha1.Ignore, "true")
		} else {
			delete(managedResource.Annotations, resourcesv1alpha1.Ignore)
		}
		return v.client.Patch(ctx, managedResource, patch)
	}

	pvc := &corev1.PersistentVolumeClaim{}
	if err := v.client.Get(ctx, kubernetesutils.Key(v.namespace, "vali-vali-0"), pvc); err != nil {
		if !apierrors.IsNotFound(err) {
			return err
		}
		return addOrRemoveIgnoreAnnotationFromManagedResource(false)
	}

	// Check if we need resizing
	storageCmpResult := v.values.Storage.Cmp(*pvc.Spec.Resources.Requests.Storage())
	if storageCmpResult == 0 {
		return addOrRemoveIgnoreAnnotationFromManagedResource(false)
	}

	// Annotate managed resource to skip reconciliation.
	if err := addOrRemoveIgnoreAnnotationFromManagedResource(true); err != nil {
		return err
	}

	if err := kubernetes.ScaleStatefulSetAndWaitUntilScaled(ctx, v.client, client.ObjectKey{Namespace: v1beta1constants.GardenNamespace, Name: v1beta1constants.StatefulSetNameVali}, 0); client.IgnoreNotFound(err) != nil {
		return err
	}

	switch {
	case storageCmpResult > 0:
		patch := client.MergeFrom(pvc.DeepCopy())
		pvc.Spec.Resources.Requests = corev1.ResourceList{corev1.ResourceStorage: *v.values.Storage}
		if err := v.client.Patch(ctx, pvc, patch); client.IgnoreNotFound(err) != nil {
			return err
		}

	case storageCmpResult < 0:
		if err := client.IgnoreNotFound(v.client.Delete(ctx, pvc)); err != nil {
			return err
		}
	}

	return addOrRemoveIgnoreAnnotationFromManagedResource(false)
}

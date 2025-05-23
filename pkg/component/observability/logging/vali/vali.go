// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package vali

import (
	"bytes"
	"context"
	_ "embed"
	"fmt"
	"text/template"

	"github.com/Masterminds/sprig/v3"
	monitoringv1 "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1"
	appsv1 "k8s.io/api/apps/v1"
	autoscalingv1 "k8s.io/api/autoscaling/v1"
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
	kubeapiserverconstants "github.com/gardener/gardener/pkg/component/kubernetes/apiserver/constants"
	valiconstants "github.com/gardener/gardener/pkg/component/observability/logging/vali/constants"
	"github.com/gardener/gardener/pkg/component/observability/monitoring/prometheus/aggregate"
	"github.com/gardener/gardener/pkg/component/observability/monitoring/prometheus/shoot"
	monitoringutils "github.com/gardener/gardener/pkg/component/observability/monitoring/utils"
	"github.com/gardener/gardener/pkg/resourcemanager/controller/garbagecollector/references"
	"github.com/gardener/gardener/pkg/utils"
	gardenerutils "github.com/gardener/gardener/pkg/utils/gardener"
	kubernetesutils "github.com/gardener/gardener/pkg/utils/kubernetes"
	"github.com/gardener/gardener/pkg/utils/managedresources"
	"github.com/gardener/gardener/pkg/utils/secrets"
	secretsmanager "github.com/gardener/gardener/pkg/utils/secrets/manager"
)

const (
	managedResourceNameTarget = "vali-target"

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

// Values are the values for the Vali.
type Values struct {
	ValiImage          string
	CuratorImage       string
	TelegrafImage      string
	KubeRBACProxyImage string
	WithRBACProxy      bool
	InitLargeDirImage  string

	ClusterType             component.ClusterType
	Replicas                int32
	PriorityClassName       string
	IngressHost             string
	ShootNodeLoggingEnabled bool
	Storage                 *resource.Quantity
}

// Interface is the interface for the Vali deployer.
type Interface interface {
	component.Deployer
	WithAuthenticationProxy(bool)
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

func (v *vali) WithAuthenticationProxy(b bool) {
	v.values.WithRBACProxy = b
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
		v.getVPA(),
		v.getStatefulSet(valiConfigMap.Name, telegrafConfigMapName, genericTokenKubeconfigSecretName),
		v.getServiceMonitor(),
		v.getPrometheusRule(),
	)

	if err := registry.Add(resources...); err != nil {
		return err
	}

	serializedObjects, err := registry.SerializedObjects()
	if err != nil {
		return err
	}

	return managedresources.CreateForSeedWithLabels(ctx, v.client, v.namespace, valiconstants.ManagedResourceNameRuntime, false, map[string]string{v1beta1constants.LabelCareConditionType: v1beta1constants.ObservabilityComponentsHealthy}, serializedObjects)
}

func (v *vali) Destroy(ctx context.Context) error {
	if err := managedresources.DeleteForShoot(ctx, v.client, v.namespace, managedResourceNameTarget); err != nil {
		return err
	}

	if err := managedresources.DeleteForSeed(ctx, v.client, v.namespace, valiconstants.ManagedResourceNameRuntime); err != nil {
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
		WithTargetSecret(valiconstants.ValitailTokenSecretName, metav1.NamespaceSystem)
}

func (v *vali) newKubeRBACProxyShootAccessSecret() *gardenerutils.AccessSecret {
	return gardenerutils.NewShootAccessSecret(kubeRBACProxyName, v.namespace)
}

func (v *vali) getVPA() *vpaautoscalingv1.VerticalPodAutoscaler {
	vpa := &vpaautoscalingv1.VerticalPodAutoscaler{
		ObjectMeta: metav1.ObjectMeta{
			Name:      valiName + "-vpa",
			Namespace: v.namespace,
			Labels: utils.MergeStringMaps(getLabels(), map[string]string{
				v1beta1constants.LabelObservabilityApplication: valiName,
			}),
		},
		Spec: vpaautoscalingv1.VerticalPodAutoscalerSpec{
			TargetRef: &autoscalingv1.CrossVersionObjectReference{
				Kind:       "StatefulSet",
				Name:       valiName,
				APIVersion: appsv1.SchemeGroupVersion.String(),
			},
			UpdatePolicy: &vpaautoscalingv1.PodUpdatePolicy{
				UpdateMode: ptr.To(vpaautoscalingv1.UpdateModeAuto),
			},
			ResourcePolicy: &vpaautoscalingv1.PodResourcePolicy{
				ContainerPolicies: []vpaautoscalingv1.ContainerResourcePolicy{
					{
						ContainerName:    valiName,
						ControlledValues: ptr.To(vpaautoscalingv1.ContainerControlledValuesRequestsOnly),
					},
					{
						ContainerName:    curatorName,
						Mode:             ptr.To(vpaautoscalingv1.ContainerScalingModeOff),
						ControlledValues: ptr.To(vpaautoscalingv1.ContainerControlledValuesRequestsOnly),
					},
					{
						ContainerName:    initLargeDirName,
						Mode:             ptr.To(vpaautoscalingv1.ContainerScalingModeOff),
						ControlledValues: ptr.To(vpaautoscalingv1.ContainerControlledValuesRequestsOnly),
					},
				},
			},
		},
	}

	if v.values.ShootNodeLoggingEnabled {
		vpa.Spec.ResourcePolicy.ContainerPolicies = append(vpa.Spec.ResourcePolicy.ContainerPolicies,
			vpaautoscalingv1.ContainerResourcePolicy{
				ContainerName:    kubeRBACProxyName,
				Mode:             ptr.To(vpaautoscalingv1.ContainerScalingModeOff),
				ControlledValues: ptr.To(vpaautoscalingv1.ContainerControlledValuesRequestsOnly),
			},
			vpaautoscalingv1.ContainerResourcePolicy{
				ContainerName:    telegrafName,
				Mode:             ptr.To(vpaautoscalingv1.ContainerScalingModeOff),
				ControlledValues: ptr.To(vpaautoscalingv1.ContainerControlledValuesRequestsOnly),
			},
		)
	}

	return vpa
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
				Name:        valiconstants.ServiceName,
				Namespace:   v.namespace,
				Labels:      getLabels(),
				Annotations: map[string]string{},
			},
			Spec: corev1.ServiceSpec{
				Type:     corev1.ServiceTypeClusterIP,
				Selector: getLabels(),
				Ports: []corev1.ServicePort{{
					Name:       valiMetricsPortName,
					Port:       valiconstants.ValiPort,
					Protocol:   corev1.ProtocolTCP,
					TargetPort: intstr.FromInt32(valiconstants.ValiPort),
				}},
			},
		}

		networkPolicyPorts = []networkingv1.NetworkPolicyPort{{
			Port:     ptr.To(intstr.FromInt32(valiconstants.ValiPort)),
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
			Port:     ptr.To(intstr.FromInt32(telegrafServicePort)),
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
	if err := telegrafConfigTemplate.Execute(&telegrafConfig, map[string]any{"ListenPort": telegrafServicePort}); err != nil {
		return nil, fmt.Errorf("failed to render telegraf configuration: %w", err)
	}

	var telegrafStartScript bytes.Buffer
	if err := telegrafStartScriptTemplate.Execute(&telegrafStartScript, map[string]any{"KubeRBACProxyPort": kubeRBACProxyPort}); err != nil {
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
						Labels: utils.MergeStringMaps(getLabels(), map[string]string{
							v1beta1constants.LabelObservabilityApplication: valiName,
						}),
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
									RunAsUser:    ptr.To[int64](0),
									RunAsNonRoot: ptr.To(false),
									RunAsGroup:   ptr.To[int64](0),
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
									ContainerPort: valiconstants.ValiPort,
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
										corev1.ResourceCPU:    resource.MustParse("10m"),
										corev1.ResourceMemory: resource.MustParse("100M"),
									},
								},
								SecurityContext: &corev1.SecurityContext{
									AllowPrivilegeEscalation: ptr.To(false),
									RunAsUser:                ptr.To(valiUserAndGroupId),
									RunAsGroup:               ptr.To(valiUserAndGroupId),
									RunAsNonRoot:             ptr.To(true),
									ReadOnlyRootFilesystem:   ptr.To(true),
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
										corev1.ResourceCPU:    resource.MustParse("5m"),
										corev1.ResourceMemory: resource.MustParse("15Mi"),
									},
								},
								SecurityContext: &corev1.SecurityContext{
									AllowPrivilegeEscalation: ptr.To(false),
									RunAsUser:                ptr.To(valiUserAndGroupId),
									RunAsGroup:               ptr.To(valiUserAndGroupId),
									RunAsNonRoot:             ptr.To(true),
									ReadOnlyRootFilesystem:   ptr.To(true),
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
									DefaultMode: ptr.To[int32](0520),
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
						Resources: corev1.VolumeResourceRequirements{
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
	if !v.values.ShootNodeLoggingEnabled {
		utilruntime.Must(references.InjectAnnotations(statefulSet))
		return statefulSet
	}

	statefulSet.Spec.Template.Labels[v1beta1constants.LabelNetworkPolicyToDNS] = v1beta1constants.LabelNetworkPolicyAllowed
	statefulSet.Spec.Template.Labels[gardenerutils.NetworkPolicyLabel(v1beta1constants.DeploymentNameKubeAPIServer, kubeapiserverconstants.Port)] = v1beta1constants.LabelNetworkPolicyAllowed
	statefulSet.Spec.Template.Spec.Containers = append(statefulSet.Spec.Template.Spec.Containers,
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
					corev1.ResourceMemory: resource.MustParse("45Mi"),
				},
			},
			SecurityContext: &corev1.SecurityContext{
				AllowPrivilegeEscalation: ptr.To(false),
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

	if v.values.WithRBACProxy {
		statefulSet.Spec.Template.Spec.Containers = append(statefulSet.Spec.Template.Spec.Containers,
			corev1.Container{
				Name:  kubeRBACProxyName,
				Image: v.values.KubeRBACProxyImage,
				Args: []string{
					fmt.Sprintf("--insecure-listen-address=0.0.0.0:%d", kubeRBACProxyPort),
					fmt.Sprintf("--upstream=http://127.0.0.1:%d/", valiconstants.ValiPort),
					"--kubeconfig=" + gardenerutils.PathGenericKubeconfig,
					"--logtostderr=true",
					"--v=6",
				},
				Resources: corev1.ResourceRequirements{
					Requests: corev1.ResourceList{
						corev1.ResourceCPU:    resource.MustParse("5m"),
						corev1.ResourceMemory: resource.MustParse("30Mi"),
					},
				},
				SecurityContext: &corev1.SecurityContext{
					AllowPrivilegeEscalation: ptr.To(false),
					RunAsUser:                ptr.To[int64](65532),
					RunAsGroup:               ptr.To[int64](65534),
					RunAsNonRoot:             ptr.To(true),
					ReadOnlyRootFilesystem:   ptr.To(true),
				},
				Ports: []corev1.ContainerPort{{
					Name:          kubeRBACProxyName,
					ContainerPort: kubeRBACProxyPort,
					Protocol:      corev1.ProtocolTCP,
				}},
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

func (v *vali) getPrometheusLabel() string {
	if v.values.ClusterType == component.ClusterTypeShoot {
		return shoot.Label
	}
	return aggregate.Label
}

func (v *vali) getServiceMonitor() *monitoringv1.ServiceMonitor {
	obj := &monitoringv1.ServiceMonitor{
		ObjectMeta: monitoringutils.ConfigObjectMeta("vali", v.namespace, v.getPrometheusLabel()),
		Spec: monitoringv1.ServiceMonitorSpec{
			Selector: metav1.LabelSelector{MatchLabels: getLabels()},
			Endpoints: []monitoringv1.Endpoint{{
				Port: valiMetricsPortName,
				RelabelConfigs: []monitoringv1.RelabelConfig{
					// This service monitor is targeting the logging service. Without explicitly overriding the
					// job label, prometheus-operator would choose job=logging (service name).
					{
						Action:      "replace",
						Replacement: ptr.To("vali"),
						TargetLabel: "job",
					},
					{
						Action: "labelmap",
						Regex:  `__meta_kubernetes_service_label_(.+)`,
					},
				},
				MetricRelabelConfigs: monitoringutils.StandardMetricRelabelConfig(
					"vali_ingester_blocks_per_chunk_sum",
					"vali_ingester_blocks_per_chunk_count",
					"vali_ingester_chunk_age_seconds_sum",
					"vali_ingester_chunk_age_seconds_count",
					"vali_ingester_chunk_bounds_hours_sum",
					"vali_ingester_chunk_bounds_hours_count",
					"vali_ingester_chunk_compression_ratio_sum",
					"vali_ingester_chunk_compression_ratio_count",
					"vali_ingester_chunk_encode_time_seconds_sum",
					"vali_ingester_chunk_encode_time_seconds_count",
					"vali_ingester_chunk_entries_sum",
					"vali_ingester_chunk_entries_count",
					"vali_ingester_chunk_size_bytes_sum",
					"vali_ingester_chunk_size_bytes_count",
					"vali_ingester_chunk_utilization_sum",
					"vali_ingester_chunk_utilization_count",
					"vali_ingester_memory_chunks",
					"vali_ingester_received_chunks",
					"vali_ingester_samples_per_chunk_sum",
					"vali_ingester_samples_per_chunk_count",
					"vali_ingester_sent_chunks",
					"vali_panic_total",
					"vali_logql_querystats_duplicates_total",
					"vali_logql_querystats_ingester_sent_lines_total",
					"prometheus_target_scrapes_sample_out_of_order_total",
				),
			}},
		},
	}

	if v.values.ShootNodeLoggingEnabled {
		obj.Spec.Endpoints = append(obj.Spec.Endpoints, monitoringv1.Endpoint{
			Port: telegrafName,
			RelabelConfigs: []monitoringv1.RelabelConfig{
				// This service monitor is targeting the logging service. Without explicitly overriding the
				// job label, prometheus-operator would choose job=logging (service name).
				{
					Action:      "replace",
					Replacement: ptr.To("vali-" + telegrafName),
					TargetLabel: "job",
				},
				{
					Action: "labelmap",
					Regex:  `__meta_kubernetes_service_label_(.+)`,
				},
			},
			MetricRelabelConfigs: []monitoringv1.RelabelConfig{{
				SourceLabels: []monitoringv1.LabelName{"__name__"},
				TargetLabel:  "__name__",
				Regex:        `iptables_(.+)`,
				Action:       "replace",
				Replacement:  ptr.To("shoot_node_logging_incoming_$1"),
			}},
		})
	}

	return obj
}

func (v *vali) getPrometheusRule() *monitoringv1.PrometheusRule {
	description := "There are no vali pods running on seed: {{ .ExternalLabels.seed }}. No logs will be collected."
	if v.values.ClusterType == component.ClusterTypeShoot {
		description = "There are no vali pods running. No logs will be collected."
	}

	return &monitoringv1.PrometheusRule{
		ObjectMeta: monitoringutils.ConfigObjectMeta("vali", v.namespace, v.getPrometheusLabel()),
		Spec: monitoringv1.PrometheusRuleSpec{
			Groups: []monitoringv1.RuleGroup{{
				Name: "vali.rules",
				Rules: []monitoringv1.Rule{{
					Alert: "ValiDown",
					Expr:  intstr.FromString(`absent(up{job="vali"} == 1)`),
					For:   ptr.To(monitoringv1.Duration("30m")),
					Labels: map[string]string{
						"service":    "logging",
						"severity":   "warning",
						"type":       "seed",
						"visibility": "operator",
					},
					Annotations: map[string]string{
						"description": description,
						"summary":     "Vali is down",
					},
				}},
			}},
		},
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
	managedResource := &resourcesv1alpha1.ManagedResource{ObjectMeta: metav1.ObjectMeta{Name: valiconstants.ManagedResourceNameRuntime, Namespace: v.namespace}}
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
	if err := v.client.Get(ctx, client.ObjectKey{Namespace: v.namespace, Name: "vali-vali-0"}, pvc); err != nil {
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

	if err := kubernetesutils.ScaleStatefulSetAndWaitUntilScaled(ctx, v.client, client.ObjectKey{Namespace: v1beta1constants.GardenNamespace, Name: v1beta1constants.StatefulSetNameVali}, 0); client.IgnoreNotFound(err) != nil {
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

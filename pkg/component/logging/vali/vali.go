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
	"errors"
	"fmt"
	"io"
	"text/template"
	"time"

	"github.com/Masterminds/semver"
	"github.com/Masterminds/sprig"
	hvpav1alpha1 "github.com/gardener/hvpa-controller/api/v1alpha1"

	//"github.com/go-logr/logr"
	appsv1 "k8s.io/api/apps/v1"
	autoscalingv2beta1 "k8s.io/api/autoscaling/v2beta1"
	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	vpaautoscalingv1 "k8s.io/autoscaler/vertical-pod-autoscaler/pkg/apis/autoscaling.k8s.io/v1"
	"k8s.io/utils/pointer"
	"sigs.k8s.io/controller-runtime/pkg/client"

	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	resourcesv1alpha1 "github.com/gardener/gardener/pkg/apis/resources/v1alpha1"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	"github.com/gardener/gardener/pkg/component"
	"github.com/gardener/gardener/pkg/controllerutils"
	"github.com/gardener/gardener/pkg/resourcemanager/controller/garbagecollector/references"
	kubernetesutils "github.com/gardener/gardener/pkg/utils/kubernetes"
	"github.com/gardener/gardener/pkg/utils/managedresources"
	"github.com/gardener/gardener/pkg/utils/secrets"
	secretsmanager "github.com/gardener/gardener/pkg/utils/secrets/manager"
	"github.com/gardener/gardener/pkg/utils/version"
)

const (
	// ManagedResourceControlName is the name of the Vali managed resource for seeds.
	ManagedResourceControlName = "vali"
	valiName                   = "vali"
	curatorName                = "curator"
	kubeRBACProxyName          = "kube-rbac-proxy"
	telegrafName               = "telegraf"
	initLargeDirName           = "init-large-dir"
	serviceName                = "vali"
	//RBAC Proxy configuration
	serviceRBACProxyPortNumber = 8080
	serviceTelegrafPortNumber  = 9273
	backendPath                = "/vali/api/v1/push"
	//Vali configuration
	serviceValiPortNumber   = 3100
	curatorPortNumber       = 2718
	valiMetricsPortName     = "metrics"
	valiUserAndGroupId      = 10001
	valiConfigMapVolumeName = "config"
	valiPVCName             = "vali"
)

var (
	//go:embed templates/curator-config.yaml
	curatorConfigBytes []byte
	//go:embed templates/vali-init.sh
	valiInitBytes []byte

	tplNameValiConfig = "vali-config"
	//go:embed templates/vali-config.tpl
	tplContentValiConfig string
	valiConfigTmpl       *template.Template
	//valiConfigYAMLTmpl     = template.Must(template.ParseFS(valiConfigYAMLTmplFile, "templates/vali-config.tpl"))

	tplNameTelegrafConfig = "telegraf-config"
	//go:embed templates/telegraf-config.tpl
	tplContentTelegrafConfig string
	telegrafConfigTmpl       *template.Template
	//telegrafConfigYAMLTmpl   = template.Must(template.ParseFS(telegrafConfigYAMLTmplFile, "telegraf-config.tpl"))

	tplNameTelegrafStart = "telegraf-start"
	//go:embed templates/telegraf-start.sh.tpl
	tplContentTelegrafStart string
	telegrafStartTmpl       *template.Template
	//telegrafStartTmpl     = template.Must(template.ParseFS(telegrafStartTmplFile, "templates/telegraf-start.sh.tpl"))

	controlledValues            = vpaautoscalingv1.ContainerControlledValuesRequestsOnly
	containerPolicyOff          = vpaautoscalingv1.ContainerScalingModeOff
	fsGroupChangeOnRootMismatch = corev1.FSGroupChangeOnRootMismatch
	pathType                    = networkingv1.PathTypePrefix

	ingressTLSCertificateValidity = 730 * 24 * time.Hour // ~2 years, see https://support.apple.com/en-us/HT210176
)

func init() {
	var err error
	valiConfigTmpl, err = template.
		New(tplNameValiConfig).
		Funcs(sprig.TxtFuncMap()).
		Parse(tplContentValiConfig)
	if err != nil {
		panic(err)
	}

	telegrafConfigTmpl, err = template.
		New(tplNameTelegrafStart).
		Funcs(sprig.TxtFuncMap()).
		Parse(tplContentTelegrafConfig)
	if err != nil {
		panic(err)
	}

	telegrafStartTmpl, err = template.
		New(tplNameTelegrafConfig).
		Funcs(sprig.TxtFuncMap()).
		Parse(tplContentTelegrafStart)
	if err != nil {
		panic(err)
	}
}

// Values are the values for the Vali.
type Values struct {
	Replicas              int32
	AuthEnabled           bool
	Storage               *resource.Quantity
	HvpaEnabled           bool
	MaintenanceBegin      string
	MaintenanceEnd        string
	ValiImage             string
	CuratorImage          string
	RenameLokiToValiImage string
	InitLargeDirImage     string
	PriorityClassName     string
	TelegrafImage         string
	KubeRBACProxyImage    string
	ClusterType           string
	IngressClass          string
	ValiHost              string
}

type vali struct {
	client         client.Client
	namespace      string
	secretsManager secretsmanager.Interface
	k8sVersion     *semver.Version
	values         Values
}

var _ component.Deployer = &vali{}

// New creates a new instance of Vali deployer.
func New(
	client client.Client,
	namespace string,
	secretsManager secretsmanager.Interface,
	k8sVersion *semver.Version,
	values Values) (component.Deployer, error) {

	switch {
	case client == nil:
		return nil, errors.New("client cannot be nil")
	case len(namespace) == 0:
		return nil, errors.New("namespace cannot be empty")
	case len(values.ValiImage) == 0:
		return nil, errors.New("vali image cannot be empty")
	case len(values.CuratorImage) == 0:
		return nil, errors.New("vali curator image cannot be empty")
	case len(values.InitLargeDirImage) == 0:
		return nil, errors.New("init-large-dir image cannot be empty")
	case len(values.KubeRBACProxyImage) > 0 && len(values.TelegrafImage) == 0:
		return nil, errors.New("when kube-RBAC-proxy image is present the telegraf image cannot be empty")
	case len(values.TelegrafImage) > 0 && len(values.KubeRBACProxyImage) == 0:
		return nil, errors.New("when telegraf image is present the kube-RBAC-proxy image cannot be empty")
	case len(values.KubeRBACProxyImage) > 0 && len(values.TelegrafImage) > 0 && secretsManager == nil:
		return nil, errors.New("secretManager cannot be nil")
	case len(values.KubeRBACProxyImage) > 0 && len(values.TelegrafImage) > 0 && k8sVersion == nil:
		return nil, errors.New("kubernetes version cannot be nil")
	}

	return &vali{
		client:         client,
		namespace:      namespace,
		secretsManager: secretsManager,
		k8sVersion:     k8sVersion,
		values:         values,
	}, nil
}

func (v *vali) Deploy(ctx context.Context) error {
	var (
		valiConfigMapName                string
		telegrafConfigMapName            string
		genericTokenKubeconfigSecretName string
		valiResources                    *corev1.ResourceRequirements
		registry                         = managedresources.NewRegistry(kubernetes.SeedScheme, kubernetes.SeedCodec, kubernetes.SeedSerializer)
		resources                        = []client.Object{}
	)
	if v.values.HvpaEnabled && v.values.Replicas > 0 {
		resources = append(resources, v.getHVPA())

		//If HVPA is enabled then we have to set the resources as same the current ones to avoid redundant scale up.
		currentResources, err := kubernetesutils.GetContainerResourcesInStatefulSet(ctx, v.client, kubernetesutils.Key(v.namespace, v1beta1constants.StatefulSetNameVali))
		if err != nil {
			return err
		}
		if len(currentResources) != 0 && currentResources[valiName] != nil {
			valiResources = &corev1.ResourceRequirements{
				Requests: currentResources[valiName].Requests,
				Limits:   currentResources[valiName].Limits,
			}
		}
	}

	if v.isRBACProxyEnabled() {
		ingressTLSSecret, err := v.secretsManager.Generate(ctx, &secrets.CertificateSecretConfig{
			Name:                        "vali-tls",
			CommonName:                  v.values.ValiHost,
			Organization:                []string{"gardener.cloud:monitoring:ingress"},
			DNSNames:                    []string{v.values.ValiHost},
			CertType:                    secrets.ServerCert,
			Validity:                    &ingressTLSCertificateValidity,
			SkipPublishingCACertificate: true,
		}, secretsmanager.SignedByCA(v1beta1constants.SecretNameCACluster))
		if err != nil {
			return err
		}
		resources = append(resources, v.getKubeRBACProxyIngress(ingressTLSSecret.Name))

		telegrafConfig, err := v.getTelegrafConfig()
		if err != nil {
			return err
		}
		telegrafConfigMapName = telegrafConfig.Name
		resources = append(resources, telegrafConfig)

		genericTokenKubeconfigSecret, found := v.secretsManager.Get(v1beta1constants.SecretNameGenericTokenKubeconfig)
		if !found {
			return fmt.Errorf("secret %q not found", v1beta1constants.SecretNameGenericTokenKubeconfig)
		}
		genericTokenKubeconfigSecretName = genericTokenKubeconfigSecret.Name
	}

	resources = append(resources, v.getService())

	valiConfigMap, err := v.getValiConfig()
	if err != nil {
		return err
	}
	valiConfigMapName = valiConfigMap.Name
	resources = append(resources, valiConfigMap)

	resources = append(resources, v.getStatefulset(valiConfigMapName, telegrafConfigMapName, genericTokenKubeconfigSecretName, valiResources))

	serializedResources, err := registry.AddAllAndSerialize(resources...)
	if err != nil {
		return err
	}

	if v.values.Storage != nil {
		if err := v.resizeOrDeleteValiDataVolumeIfStorageNotTheSame(ctx); err != nil {
			return err
		}
	}

	return managedresources.CreateForSeed(ctx, v.client, v.namespace, ManagedResourceControlName, false, serializedResources)
}

func (v *vali) Destroy(ctx context.Context) error {
	return managedresources.DeleteForSeed(ctx, v.client, v.namespace, ManagedResourceControlName)
}

func (v *vali) getHVPA() *hvpav1alpha1.Hvpa {
	obj := &hvpav1alpha1.Hvpa{
		ObjectMeta: metav1.ObjectMeta{
			Name:      valiName,
			Namespace: v.namespace,
			Labels:    getLabels(),
		},
		Spec: hvpav1alpha1.HvpaSpec{
			Replicas: pointer.Int32(v.values.Replicas),
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
						MinReplicas: pointer.Int32(v.values.Replicas),
						MaxReplicas: v.values.Replicas,
						Metrics: []autoscalingv2beta1.MetricSpec{
							{
								Type: autoscalingv2beta1.ResourceMetricSourceType,
								Resource: &autoscalingv2beta1.ResourceMetricSource{
									Name:                     corev1.ResourceCPU,
									TargetAverageUtilization: pointer.Int32(80),
								},
							},
							{
								Type: autoscalingv2beta1.ResourceMetricSourceType,
								Resource: &autoscalingv2beta1.ResourceMetricSource{
									Name:                     corev1.ResourceMemory,
									TargetAverageUtilization: pointer.Int32(80),
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
						UpdateMode: pointer.String(hvpav1alpha1.UpdateModeAuto),
					},
					StabilizationDuration: pointer.String("5m"),
					MinChange: hvpav1alpha1.ScaleParams{
						CPU: hvpav1alpha1.ChangeParams{
							Value:      pointer.String("100m"),
							Percentage: pointer.Int32(80),
						},
						Memory: hvpav1alpha1.ChangeParams{
							Value:      pointer.String("300M"),
							Percentage: pointer.Int32(80),
						},
					},
				},
				ScaleDown: hvpav1alpha1.ScaleType{
					UpdatePolicy: hvpav1alpha1.UpdatePolicy{
						UpdateMode: pointer.String(hvpav1alpha1.UpdateModeAuto),
					},
					StabilizationDuration: pointer.String("168h"),
					MinChange: hvpav1alpha1.ScaleParams{
						CPU: hvpav1alpha1.ChangeParams{
							Value:      pointer.String("200m"),
							Percentage: pointer.Int32(80),
						},
						Memory: hvpav1alpha1.ChangeParams{
							Value:      pointer.String("500M"),
							Percentage: pointer.Int32(80),
						},
					},
				},
				LimitsRequestsGapScaleParams: hvpav1alpha1.ScaleParams{
					CPU: hvpav1alpha1.ChangeParams{
						Value:      pointer.String("300m"),
						Percentage: pointer.Int32(40),
					},
					Memory: hvpav1alpha1.ChangeParams{
						Value:      pointer.String("1G"),
						Percentage: pointer.Int32(40),
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
			WeightBasedScalingIntervals: []hvpav1alpha1.WeightBasedScalingInterval{
				{
					VpaWeight:         hvpav1alpha1.VpaOnly,
					StartReplicaCount: v.values.Replicas,
					LastReplicaCount:  v.values.Replicas,
				},
			},
			TargetRef: &autoscalingv2beta1.CrossVersionObjectReference{
				APIVersion: "apps/v1",
				Kind:       "StatefulSet",
				Name:       valiName,
			},
		},
	}

	if v.values.MaintenanceBegin != "" && v.values.MaintenanceEnd != "" {
		obj.Spec.MaintenanceTimeWindow = &hvpav1alpha1.MaintenanceTimeWindow{
			Begin: v.values.MaintenanceBegin,
			End:   v.values.MaintenanceEnd,
		}

		obj.Spec.Vpa.ScaleDown.UpdatePolicy.UpdateMode = pointer.String(hvpav1alpha1.UpdateModeMaintenanceWindow)
	}

	if v.isRBACProxyEnabled() {
		obj.Spec.Vpa.Template.Spec.ResourcePolicy.ContainerPolicies = append(obj.Spec.Vpa.Template.Spec.ResourcePolicy.ContainerPolicies,
			[]vpaautoscalingv1.ContainerResourcePolicy{
				{
					ContainerName:    kubeRBACProxyName,
					Mode:             &containerPolicyOff,
					ControlledValues: &controlledValues,
				},
				{
					ContainerName:    telegrafName,
					Mode:             &containerPolicyOff,
					ControlledValues: &controlledValues,
				},
			}...)
	}
	return obj
}

func (v *vali) getKubeRBACProxyIngress(secretName string) *networkingv1.Ingress {
	ingress := &networkingv1.Ingress{
		ObjectMeta: metav1.ObjectMeta{
			Name:      valiName,
			Namespace: v.namespace,
			Annotations: map[string]string{
				"nginx.ingress.kubernetes.io/configuration-snippet": "proxy_set_header X-Scope-OrgID operator;",
			},
			Labels: getLabels(),
		},
		Spec: networkingv1.IngressSpec{
			TLS: []networkingv1.IngressTLS{
				{
					SecretName: secretName,
					Hosts:      []string{v.values.ValiHost},
				},
			},
			Rules: []networkingv1.IngressRule{
				{
					Host: v.values.ValiHost,
					IngressRuleValue: networkingv1.IngressRuleValue{
						HTTP: &networkingv1.HTTPIngressRuleValue{
							Paths: []networkingv1.HTTPIngressPath{
								{
									Backend: networkingv1.IngressBackend{
										Service: &networkingv1.IngressServiceBackend{
											Name: serviceName,
											Port: networkingv1.ServiceBackendPort{
												Number: serviceRBACProxyPortNumber,
											},
										},
									},
									Path:     backendPath,
									PathType: &pathType,
								},
							},
						},
					},
				},
			},
		},
	}

	if version.ConstraintK8sGreaterEqual122.Check(v.k8sVersion) {
		ingress.Spec.IngressClassName = pointer.String(v.values.IngressClass)
	} else {
		ingress.Annotations["kubernetes.io/ingress.class"] = v.values.IngressClass
	}

	return ingress
}

func (v *vali) getService() *corev1.Service {
	svc := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:        "logging",
			Namespace:   v.namespace,
			Labels:      getLabels(),
			Annotations: map[string]string{},
		},
		Spec: corev1.ServiceSpec{
			Type: corev1.ServiceTypeClusterIP,
			Ports: []corev1.ServicePort{
				{
					Port:       serviceValiPortNumber,
					Protocol:   corev1.ProtocolTCP,
					TargetPort: intstr.FromInt(serviceValiPortNumber),
					Name:       valiMetricsPortName,
				},
			},
			Selector: getLabels(),
		},
	}

	if v.isRBACProxyEnabled() {
		svc.Spec.Ports = append(svc.Spec.Ports, []corev1.ServicePort{
			{
				Port:       serviceRBACProxyPortNumber,
				Protocol:   corev1.ProtocolTCP,
				TargetPort: intstr.FromInt(serviceRBACProxyPortNumber),
				Name:       "external",
			},
			{
				Port:       serviceTelegrafPortNumber,
				Protocol:   corev1.ProtocolTCP,
				TargetPort: intstr.FromInt(serviceTelegrafPortNumber),
				Name:       telegrafName,
			},
		}...)
	}

	switch v.values.ClusterType {
	case "seed":
		svc.Annotations["networking.resources.gardener.cloud/from-all-seed-scrape-targets-allowed-ports"] = fromAllSeedScrapeTargetsAllowedPorts(v.isRBACProxyEnabled())
	case "shoot":
		svc.Annotations["networking.resources.gardener.cloud/from-all-scrape-targets-allowed-ports"] = fromAllScrapeTargetsAllowedPorts(v.isRBACProxyEnabled())
		svc.Annotations[resourcesv1alpha1.NetworkingPodLabelSelectorNamespaceAlias] = v1beta1constants.LabelNetworkPolicyShootNamespaceAlias
		svc.Annotations[resourcesv1alpha1.NetworkingNamespaceSelectors] = `[{"matchLabels":{"kubernetes.io/metadata.name":"garden"}}]`
	}

	return svc
}

func (v *vali) getValiConfig() (*corev1.ConfigMap, error) {
	valiConfigBytes, err := buildValiConfiguration(v.values.AuthEnabled)
	if err != nil {
		return nil, err
	}

	configMap := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "vali-config",
			Namespace: v.namespace,
			Labels:    getLabels(),
		},
		BinaryData: map[string][]byte{
			"vali.yaml":    valiConfigBytes,
			"curator.yaml": curatorConfigBytes,
			"vali-init.sh": valiInitBytes,
		},
	}

	utilruntime.Must(kubernetesutils.MakeUnique(configMap))

	return configMap, nil
}

func (v *vali) getTelegrafConfig() (*corev1.ConfigMap, error) {
	telegrafConfigBytes, err := buildTelegrafConfiguration()
	if err != nil {
		return nil, err
	}
	telegrafStartScriptBytes, err := buildTelegrafStartScript()
	if err != nil {
		return nil, err
	}

	configMap := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "telegraf-config",
			Namespace: v.namespace,
			Labels:    getLabels(),
		},
		BinaryData: map[string][]byte{
			"telegraf.conf": telegrafConfigBytes,
			"start.sh":      telegrafStartScriptBytes,
		},
	}

	utilruntime.Must(kubernetesutils.MakeUnique(configMap))

	return configMap, nil
}

func (v *vali) getStatefulset(valiConfigMapName, telegrafConfigMapName, genericTokenKubeconfigSecretName string, valiResources *corev1.ResourceRequirements) *appsv1.StatefulSet {
	sts := &appsv1.StatefulSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:      valiName,
			Namespace: v.namespace,
			Labels:    getLabels(),
		},
		Spec: appsv1.StatefulSetSpec{
			Replicas: pointer.Int32(v.values.Replicas),
			Selector: &metav1.LabelSelector{
				MatchLabels: getLabels(),
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: getLabels(),
				},
				Spec: corev1.PodSpec{
					AutomountServiceAccountToken: pointer.Bool(false),
					SecurityContext: &corev1.PodSecurityContext{
						FSGroup:             pointer.Int64(valiUserAndGroupId),
						FSGroupChangePolicy: &fsGroupChangeOnRootMismatch,
					},
					InitContainers: []corev1.Container{
						{
							Name:  initLargeDirName,
							Image: v.values.InitLargeDirImage,
							Command: []string{
								"bash",
								"-c",
								"/vali-init.sh || true",
							},
							SecurityContext: &corev1.SecurityContext{
								Privileged:   pointer.Bool(true),
								RunAsUser:    pointer.Int64(0),
								RunAsNonRoot: pointer.Bool(false),
								RunAsGroup:   pointer.Int64(0),
							},
							VolumeMounts: []corev1.VolumeMount{
								{
									MountPath: "/data",
									Name:      valiPVCName,
								},
								{
									MountPath: "/vali-init.sh",
									SubPath:   "vali-init.sh",
									Name:      valiConfigMapVolumeName,
								},
							},
						},
						{
							Name:  "rename-loki-to-vali",
							Image: v.values.RenameLokiToValiImage,
							Command: []string{
								"sh",
								"-c",
								`set -x
								# TODO (istvanballok): remove in release v1.77
								if [[ -d /data/loki ]]; then
								  echo "Renaming loki folder to vali"
								  time mv /data/loki /data/vali
								else
								  echo "No loki folder found"
								fi`,
							},
							VolumeMounts: []corev1.VolumeMount{
								{
									MountPath: "/data",
									Name:      valiPVCName,
								},
							},
						},
					},
					Containers: []corev1.Container{
						{
							Name:  valiName,
							Image: v.values.ValiImage,
							Args: []string{
								"-config.file=/etc/vali/vali.yaml",
							},
							VolumeMounts: []corev1.VolumeMount{
								{
									Name:      valiConfigMapVolumeName,
									MountPath: "/etc/vali/vali.yaml",
									SubPath:   "vali.yaml",
								},
								{
									Name:      valiPVCName,
									MountPath: "/data",
								},
							},
							Ports: []corev1.ContainerPort{
								{
									Name:          valiMetricsPortName,
									ContainerPort: serviceValiPortNumber,
									Protocol:      corev1.ProtocolTCP,
								},
							},
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
								RunAsUser:              pointer.Int64(valiUserAndGroupId),
								RunAsGroup:             pointer.Int64(valiUserAndGroupId),
								RunAsNonRoot:           pointer.Bool(true),
								ReadOnlyRootFilesystem: pointer.Bool(true),
							},
						},
						{
							Name:  curatorName,
							Image: v.values.CuratorImage,
							Args: []string{
								"-config=/etc/vali/curator.yaml",
							},
							Ports: []corev1.ContainerPort{
								{
									//TODO:(vlvasilev) why the curator and the vali ports are named metrics?
									Name:          valiMetricsPortName,
									ContainerPort: curatorPortNumber,
									Protocol:      corev1.ProtocolTCP,
								},
							},
							VolumeMounts: []corev1.VolumeMount{
								{
									Name:      valiConfigMapVolumeName,
									MountPath: "/etc/vali/curator.yaml",
									SubPath:   "curator.yaml",
								},
								{
									Name:      valiPVCName,
									MountPath: "/data",
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
								RunAsUser:              pointer.Int64(valiUserAndGroupId),
								RunAsGroup:             pointer.Int64(valiUserAndGroupId),
								RunAsNonRoot:           pointer.Bool(true),
								ReadOnlyRootFilesystem: pointer.Bool(true),
							},
						},
					},
					Volumes: []corev1.Volume{
						{
							Name: valiConfigMapVolumeName,
							VolumeSource: corev1.VolumeSource{
								ConfigMap: &corev1.ConfigMapVolumeSource{
									LocalObjectReference: corev1.LocalObjectReference{
										Name: valiConfigMapName,
									},
									DefaultMode: pointer.Int32(0520),
								},
							},
						},
					},
				},
			},
			VolumeClaimTemplates: []corev1.PersistentVolumeClaim{
				{
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
				},
			},
		},
	}

	if valiResources != nil {
		for i, container := range sts.Spec.Template.Spec.Containers {
			if container.Name == valiName {
				sts.Spec.Template.Spec.Containers[i].Resources = *valiResources
			}
		}
	}

	if v.values.Storage != nil {
		//TODO: (vlvasilev) move here the resize check
		sts.Spec.VolumeClaimTemplates[0].Spec.Resources.Requests[corev1.ResourceStorage] = *v.values.Storage
	}

	if v.values.PriorityClassName != "" {
		sts.Spec.Template.Spec.PriorityClassName = v.values.PriorityClassName
	}

	if v.isRBACProxyEnabled() {
		sts.Spec.Template.ObjectMeta.Labels[v1beta1constants.LabelNetworkPolicyToDNS] = v1beta1constants.LabelNetworkPolicyAllowed
		sts.Spec.Template.ObjectMeta.Labels["networking.resources.gardener.cloud/to-kube-apiserver-tcp-443"] = v1beta1constants.LabelNetworkPolicyAllowed

		sts.Spec.Template.Spec.Containers = append(sts.Spec.Template.Spec.Containers, []corev1.Container{
			{
				Name:  kubeRBACProxyName,
				Image: v.values.KubeRBACProxyImage,
				Args: []string{
					fmt.Sprintf("--insecure-listen-address=0.0.0.0:%d", serviceRBACProxyPortNumber),
					fmt.Sprintf("--upstream=http://127.0.0.1:%d/", serviceValiPortNumber),
					"--kubeconfig=/var/run/secrets/gardener.cloud/shoot/generic-kubeconfig/kubeconfig",
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
				Ports: []corev1.ContainerPort{
					{
						Name:          kubeRBACProxyName,
						ContainerPort: serviceRBACProxyPortNumber,
						Protocol:      corev1.ProtocolTCP,
					},
				},
				VolumeMounts: []corev1.VolumeMount{
					{
						Name:      "kubeconfig",
						MountPath: "/var/run/secrets/gardener.cloud/shoot/generic-kubeconfig",
						ReadOnly:  true,
					},
				},
				SecurityContext: &corev1.SecurityContext{
					RunAsUser:              pointer.Int64(65532),
					RunAsGroup:             pointer.Int64(65534),
					RunAsNonRoot:           pointer.Bool(true),
					ReadOnlyRootFilesystem: pointer.Bool(true),
				},
			},
			{
				Name:  telegrafName,
				Image: v.values.TelegrafImage,
				Command: []string{
					"/bin/bash",
					"-c",
					`            trap 'kill %1; wait' SIGTERM
					bash /etc/telegraf/start.sh &
					wait`,
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
						Add: []corev1.Capability{
							"NET_ADMIN",
						},
					},
				},
				Ports: []corev1.ContainerPort{
					{
						Name:          telegrafName,
						ContainerPort: serviceTelegrafPortNumber,
						Protocol:      corev1.ProtocolTCP,
					},
				},
				VolumeMounts: []corev1.VolumeMount{
					{
						Name:      "telegraf-config-volume",
						MountPath: "/etc/telegraf/telegraf.conf",
						SubPath:   "telegraf.conf",
						ReadOnly:  true,
					},
					{
						Name:      "telegraf-config-volume",
						MountPath: "/etc/telegraf/start.sh",
						SubPath:   "start.sh",
						ReadOnly:  true,
					},
				},
			},
		}...)

		sts.Spec.Template.Spec.Volumes = append(sts.Spec.Template.Spec.Volumes, []corev1.Volume{
			{
				Name: "kubeconfig",
				VolumeSource: corev1.VolumeSource{
					Projected: &corev1.ProjectedVolumeSource{
						DefaultMode: pointer.Int32(420),
						Sources: []corev1.VolumeProjection{
							{
								Secret: &corev1.SecretProjection{
									Items: []corev1.KeyToPath{
										{
											Key:  "kubeconfig",
											Path: "kubeconfig",
										},
									},
									LocalObjectReference: corev1.LocalObjectReference{
										Name: genericTokenKubeconfigSecretName,
									},
									Optional: pointer.Bool(false),
								},
							},
							{
								Secret: &corev1.SecretProjection{
									Items: []corev1.KeyToPath{
										{
											Key:  "token",
											Path: "token",
										},
									},
									LocalObjectReference: corev1.LocalObjectReference{
										Name: "shoot-access-" + kubeRBACProxyName,
									},
									Optional: pointer.Bool(false),
								},
							},
						},
					},
				},
			},
			{
				Name: "telegraf-config-volume",
				VolumeSource: corev1.VolumeSource{
					ConfigMap: &corev1.ConfigMapVolumeSource{
						LocalObjectReference: corev1.LocalObjectReference{
							Name: telegrafConfigMapName,
						},
					},
				},
			},
		}...)
	}

	utilruntime.Must(references.InjectAnnotations(sts))

	return sts
}

func (v *vali) isRBACProxyEnabled() bool {
	return v.values.KubeRBACProxyImage != "" && v.values.TelegrafImage != ""
}

func getLabels() map[string]string {
	return map[string]string{
		v1beta1constants.GardenRole: v1beta1constants.GardenRoleLogging,
		v1beta1constants.LabelRole:  "logging",
		v1beta1constants.LabelApp:   valiName,
	}
}

func fromAllSeedScrapeTargetsAllowedPorts(rbacProxyEnabled bool) string {
	if rbacProxyEnabled {
		return fmt.Sprintf(`[{"port":%d,"protocol":"TCP"},{"port":%d,"protocol":"TCP"}]`, serviceValiPortNumber, serviceTelegrafPortNumber)
	}
	return fmt.Sprintf(`[{"port":%d,"protocol":"TCP"}]`, serviceValiPortNumber)
}

func fromAllScrapeTargetsAllowedPorts(rbacProxyEnabled bool) string {
	if rbacProxyEnabled {
		return fmt.Sprintf(`[{"port":%d,"protocol":"TCP"},{"port":%d,"protocol":"TCP"}]`, serviceValiPortNumber, serviceTelegrafPortNumber)
	}
	return fmt.Sprintf(`[{"port":%d,"protocol":"TCP"}]`, serviceValiPortNumber)
}

func buildValiConfiguration(authEnabled bool) ([]byte, error) {
	w := bytes.NewBuffer(nil)
	err := valiConfigTmpl.Execute(w, map[string]interface{}{"AuthEnabled": authEnabled})
	if err != nil {
		return nil, fmt.Errorf("failed to create vali configuration: %w", err)
	}
	return io.ReadAll(w)
}

func buildTelegrafConfiguration() ([]byte, error) {
	w := bytes.NewBuffer(nil)
	err := telegrafConfigTmpl.Execute(w, map[string]interface{}{"ListenPort": serviceTelegrafPortNumber})
	if err != nil {
		return nil, fmt.Errorf("failed to create telegraf configuration: %w", err)
	}
	return io.ReadAll(w)
}

func buildTelegrafStartScript() ([]byte, error) {
	w := bytes.NewBuffer(nil)
	err := telegrafStartTmpl.Execute(w, map[string]interface{}{"KubeRBACProxyPort": serviceRBACProxyPortNumber})
	if err != nil {
		return nil, fmt.Errorf("failed to create telegraf start script: %w", err)
	}
	return io.ReadAll(w)
}

// TODO: (vlvasilev) adjust the function to use managed resource instead of working directly on the pvc and sts!
// ResizeOrDeleteValiDataVolumeIfStorageNotTheSame updates the garden Vali PVC if passed storage value is not the same as the current one.
// Caution: If the passed storage capacity is less than the current one the existing PVC and its PV will be deleted.
func (v *vali) resizeOrDeleteValiDataVolumeIfStorageNotTheSame(ctx context.Context) error {
	// Check if we need resizing
	pvc := &corev1.PersistentVolumeClaim{}
	if err := v.client.Get(ctx, kubernetesutils.Key(v1beta1constants.GardenNamespace, "vali-vali-0"), pvc); err != nil {
		return client.IgnoreNotFound(err)
	}

	storageCmpResult := v.values.Storage.Cmp(*pvc.Spec.Resources.Requests.Storage())
	if storageCmpResult == 0 {
		return nil
	}

	//Annotate managed resource to skip reconciliation.
	managedResource := &resourcesv1alpha1.ManagedResource{
		ObjectMeta: metav1.ObjectMeta{
			Name:      ManagedResourceControlName,
			Namespace: v.namespace,
		},
	}
	if _, err := controllerutils.GetAndCreateOrMergePatch(ctx, v.client, managedResource, func() error {
		if managedResource.Annotations == nil {
			managedResource.Annotations = map[string]string{}
		}
		managedResource.Annotations[resourcesv1alpha1.Ignore] = "true"
		return nil
	}); err != nil {
		return err
	}

	defer func() {
		_, _ = controllerutils.GetAndCreateOrMergePatch(ctx, v.client, managedResource, func() error {
			delete(managedResource.Annotations, resourcesv1alpha1.Ignore)
			return nil
		})
	}()

	statefulSetKey := client.ObjectKey{Namespace: v1beta1constants.GardenNamespace, Name: v1beta1constants.StatefulSetNameVali}
	if err := kubernetes.ScaleStatefulSetAndWaitUntilScaled(ctx, v.client, statefulSetKey, 0); client.IgnoreNotFound(err) != nil {
		return err
	}

	switch {
	case storageCmpResult > 0:
		patch := client.MergeFrom(pvc.DeepCopy())
		pvc.Spec.Resources.Requests = corev1.ResourceList{
			corev1.ResourceStorage: *v.values.Storage,
		}
		if err := v.client.Patch(ctx, pvc, patch); client.IgnoreNotFound(err) != nil {
			return err
		}
	case storageCmpResult < 0:
		if err := client.IgnoreNotFound(v.client.Delete(ctx, pvc)); err != nil {
			return err
		}
	}

	return nil
}

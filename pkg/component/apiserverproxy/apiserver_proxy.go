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

package apiserverproxy

import (
	"bytes"
	"context"
	_ "embed"
	"fmt"
	"text/template"
	"time"

	"github.com/Masterminds/sprig/v3"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"

	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	"github.com/gardener/gardener/pkg/component"
	"github.com/gardener/gardener/pkg/resourcemanager/controller/garbagecollector/references"
	"github.com/gardener/gardener/pkg/utils"
	kubernetesutils "github.com/gardener/gardener/pkg/utils/kubernetes"
	"github.com/gardener/gardener/pkg/utils/managedresources"
	secretsmanager "github.com/gardener/gardener/pkg/utils/secrets/manager"
)

const (
	managedResourceName   = "shoot-core-apiserver-proxy"
	configMapName         = "apiserver-proxy-config"
	serviceAcountName     = "apiserver-proxy"
	daemonSetName         = "apiserver-proxy"
	mutatingWebhookName   = "apiserver-proxy.networking.gardener.cloud"
	webhookExpressionsKey = "apiserver-proxy.networking.gardener.cloud/inject"

	adminPort           = 16910
	proxySeedServerPort = 8443

	volumeNameConfig   = "proxy-config"
	volumeNameAdminUDS = "admin-uds"

	volumeMountPathConfig = "/etc/apiserver-proxy"
	dataKeyConfig         = "envoy.yaml"
)

var (
	tplNameEnvoy = "envoy.yaml.tpl"
	//go:embed templates/envoy.yaml.tpl
	tplContentEnvoy string
	tplEnvoy        *template.Template
)

func init() {
	var err error
	tplEnvoy, err = template.
		New(tplNameEnvoy).
		Funcs(sprig.TxtFuncMap()).
		Parse(tplContentEnvoy)
	utilruntime.Must(err)
}

// Values is a set of configuration values for the apiserver-proxy component.
type Values struct {
	ProxySeedServerHost string
	Image               string
	SidecarImage        string
	DNSLookupFamily     string

	advertiseIPAddress string
}

// New creates a new instance of DeployWaiter for apiserver-proxy
func New(client client.Client, namespace string, secretsManager secretsmanager.Interface, values Values) Interface {
	return &apiserverProxy{
		client:         client,
		namespace:      namespace,
		secretsManager: secretsManager,
		values:         values,
	}
}

// Interface contains functions for deploying apiserver-proxy.
type Interface interface {
	component.DeployWaiter
	component.MonitoringComponent
	SetAdvertiseIPAddress(string)
}

type apiserverProxy struct {
	client         client.Client
	namespace      string
	secretsManager secretsmanager.Interface
	values         Values
}

func (a *apiserverProxy) Deploy(ctx context.Context) error {
	if a.values.advertiseIPAddress == "" {
		return fmt.Errorf("run SetAdvertiseIPAddress before deploying")
	}

	data, err := a.computeResourcesData()
	if err != nil {
		return err
	}

	return managedresources.CreateForShoot(ctx, a.client, a.namespace, managedResourceName, managedresources.LabelValueGardener, false, data)
}

func (a *apiserverProxy) Destroy(ctx context.Context) error {
	return managedresources.DeleteForShoot(ctx, a.client, a.namespace, managedResourceName)
}

// TimeoutWaitForManagedResource is the timeout used while waiting for the ManagedResources to become healthy
// or deleted.
var TimeoutWaitForManagedResource = 2 * time.Minute

func (a *apiserverProxy) Wait(ctx context.Context) error {
	timeoutCtx, cancel := context.WithTimeout(ctx, TimeoutWaitForManagedResource)
	defer cancel()

	return managedresources.WaitUntilHealthy(timeoutCtx, a.client, a.namespace, managedResourceName)
}

func (a *apiserverProxy) WaitCleanup(ctx context.Context) error {
	timeoutCtx, cancel := context.WithTimeout(ctx, TimeoutWaitForManagedResource)
	defer cancel()

	return managedresources.WaitUntilDeleted(timeoutCtx, a.client, a.namespace, managedResourceName)
}

func (a *apiserverProxy) SetAdvertiseIPAddress(advertiseIPAddress string) {
	a.values.advertiseIPAddress = advertiseIPAddress
}

func (a *apiserverProxy) computeResourcesData() (map[string][]byte, error) {
	var envoyYAML bytes.Buffer
	if err := tplEnvoy.Execute(&envoyYAML, map[string]interface{}{
		"advertiseIPAddress":  a.values.advertiseIPAddress,
		"dnsLookupFamily":     a.values.DNSLookupFamily,
		"adminPort":           adminPort,
		"proxySeedServerHost": a.values.ProxySeedServerHost,
		"proxySeedServerPort": proxySeedServerPort,
	}); err != nil {
		return nil, err
	}

	configMap := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      configMapName,
			Labels:    getDefaultLabels(),
			Namespace: metav1.NamespaceSystem,
		},
		Data: map[string]string{dataKeyConfig: envoyYAML.String()},
	}
	utilruntime.Must(kubernetesutils.MakeUnique(configMap))

	var (
		registry = managedresources.NewRegistry(kubernetes.SeedScheme, kubernetes.SeedCodec, kubernetes.SeedSerializer)

		serviceAccount = &corev1.ServiceAccount{
			ObjectMeta: metav1.ObjectMeta{
				Name:      serviceAcountName,
				Namespace: metav1.NamespaceSystem,
				Labels:    getDefaultLabels(),
			},
			AutomountServiceAccountToken: ptr.To(false),
		}
		service = &corev1.Service{
			ObjectMeta: metav1.ObjectMeta{
				Name:      serviceAcountName,
				Namespace: metav1.NamespaceSystem,
				Labels:    getDefaultLabels(),
			},
			Spec: corev1.ServiceSpec{
				Type:      corev1.ServiceTypeClusterIP,
				ClusterIP: "None",
				Ports: []corev1.ServicePort{
					{
						Name:       "metrics",
						Port:       adminPort,
						Protocol:   corev1.ProtocolTCP,
						TargetPort: intstr.FromInt32(adminPort),
					},
				},
				Selector: getSelector(),
			},
		}
		daemonSet = &appsv1.DaemonSet{
			ObjectMeta: metav1.ObjectMeta{
				Name:      daemonSetName,
				Namespace: metav1.NamespaceSystem,
				Labels: utils.MergeStringMaps(
					getDefaultLabels(),
					map[string]string{
						v1beta1constants.LabelNodeCriticalComponent: "true",
					},
				),
			},
			Spec: appsv1.DaemonSetSpec{
				UpdateStrategy: appsv1.DaemonSetUpdateStrategy{
					Type: appsv1.RollingUpdateDaemonSetStrategyType,
				},
				Selector: &metav1.LabelSelector{
					MatchLabels: getSelector(),
				},
				Template: corev1.PodTemplateSpec{
					ObjectMeta: metav1.ObjectMeta{
						Labels: utils.MergeStringMaps(
							getDefaultLabels(),
							map[string]string{
								v1beta1constants.LabelNodeCriticalComponent:         "true",
								v1beta1constants.LabelNetworkPolicyToDNS:            v1beta1constants.LabelNetworkPolicyAllowed,
								v1beta1constants.LabelNetworkPolicyShootToAPIServer: v1beta1constants.LabelNetworkPolicyAllowed,
								v1beta1constants.LabelNetworkPolicyShootFromSeed:    v1beta1constants.LabelNetworkPolicyAllowed,
							},
						),
					},
					Spec: corev1.PodSpec{
						ServiceAccountName: serviceAcountName,
						PriorityClassName:  "system-node-critical",
						Tolerations: []corev1.Toleration{
							{Effect: corev1.TaintEffectNoSchedule, Operator: corev1.TolerationOpExists},
							{Effect: corev1.TaintEffectNoExecute, Operator: corev1.TolerationOpExists},
						},
						HostNetwork:                  true,
						AutomountServiceAccountToken: ptr.To(false),
						SecurityContext: &corev1.PodSecurityContext{
							SeccompProfile: &corev1.SeccompProfile{
								Type: corev1.SeccompProfileTypeRuntimeDefault,
							},
						},
						InitContainers: []corev1.Container{
							{
								Name:            "setup",
								Image:           a.values.SidecarImage,
								ImagePullPolicy: corev1.PullIfNotPresent,
								Args: []string{
									fmt.Sprintf("--ip-address=%s", a.values.advertiseIPAddress),
									"--daemon=false",
									"--interface=lo",
								},
								SecurityContext: &corev1.SecurityContext{
									Capabilities: &corev1.Capabilities{
										Add: []corev1.Capability{
											"NET_ADMIN",
										},
									},
								},
								Resources: corev1.ResourceRequirements{
									Requests: corev1.ResourceList{
										corev1.ResourceCPU:    resource.MustParse("20m"),
										corev1.ResourceMemory: resource.MustParse("20Mi"),
									},
									Limits: corev1.ResourceList{
										corev1.ResourceMemory: resource.MustParse("200Mi"),
									},
								},
							},
						},
						Containers: []corev1.Container{
							{
								Name:            "sidecar",
								Image:           a.values.SidecarImage,
								ImagePullPolicy: corev1.PullIfNotPresent,
								Args: []string{
									fmt.Sprintf("--ip-address=%s", a.values.advertiseIPAddress),
									"--interface=lo",
								},
								SecurityContext: &corev1.SecurityContext{
									Capabilities: &corev1.Capabilities{
										Add: []corev1.Capability{
											"NET_ADMIN",
										},
									},
								},
								Resources: corev1.ResourceRequirements{
									Requests: corev1.ResourceList{
										corev1.ResourceCPU:    resource.MustParse("20m"),
										corev1.ResourceMemory: resource.MustParse("20Mi"),
									},
									Limits: corev1.ResourceList{
										corev1.ResourceMemory: resource.MustParse("90Mi"),
									},
								},
							},
							{
								Name:            "proxy",
								Image:           a.values.Image,
								ImagePullPolicy: corev1.PullIfNotPresent,
								Command: []string{
									"envoy",
									"--concurrency",
									"2",
									"--use-dynamic-base-id",
									"-c",
									fmt.Sprintf("%s/%s", volumeMountPathConfig, dataKeyConfig),
								},
								SecurityContext: &corev1.SecurityContext{
									Capabilities: &corev1.Capabilities{
										Add: []corev1.Capability{
											"NET_BIND_SERVICE",
										},
									},
									RunAsUser: ptr.To(int64(0)),
								},
								Resources: corev1.ResourceRequirements{
									Requests: corev1.ResourceList{
										corev1.ResourceCPU:    resource.MustParse("20m"),
										corev1.ResourceMemory: resource.MustParse("20Mi"),
									},
									Limits: corev1.ResourceList{
										corev1.ResourceMemory: resource.MustParse("1Gi"),
									},
								},
								ReadinessProbe: &corev1.Probe{
									ProbeHandler: corev1.ProbeHandler{
										HTTPGet: &corev1.HTTPGetAction{
											Path: "/ready",
											Port: intstr.FromInt32(adminPort),
										},
									},
									InitialDelaySeconds: 1,
									PeriodSeconds:       2,
									SuccessThreshold:    1,
									TimeoutSeconds:      1,
								},
								LivenessProbe: &corev1.Probe{
									ProbeHandler: corev1.ProbeHandler{
										HTTPGet: &corev1.HTTPGetAction{
											Path: "/ready",
											Port: intstr.FromInt32(adminPort),
										},
									},
									InitialDelaySeconds: 1,
									PeriodSeconds:       10,
									SuccessThreshold:    1,
									TimeoutSeconds:      1,
									FailureThreshold:    3,
								},
								Ports: []corev1.ContainerPort{
									{
										Name:          "metrics",
										ContainerPort: adminPort,
										HostPort:      adminPort,
									},
								},
								VolumeMounts: []corev1.VolumeMount{
									{
										Name:      volumeNameConfig,
										MountPath: volumeMountPathConfig,
									},
									{
										Name:      volumeNameAdminUDS,
										MountPath: "/etc/admin-uds",
									},
								},
							},
						},
						Volumes: []corev1.Volume{
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
							{
								Name: volumeNameAdminUDS,
								VolumeSource: corev1.VolumeSource{
									EmptyDir: &corev1.EmptyDirVolumeSource{},
								},
							},
						},
					},
				},
			},
		}
	)

	utilruntime.Must(references.InjectAnnotations(daemonSet))

	return registry.AddAllAndSerialize(
		configMap,
		serviceAccount,
		service,
		daemonSet,
	)
}

func getDefaultLabels() map[string]string {
	return utils.MergeStringMaps(
		map[string]string{
			v1beta1constants.GardenRole:     v1beta1constants.GardenRoleSystemComponent,
			managedresources.LabelKeyOrigin: managedresources.LabelValueGardener,
		}, getSelector())
}

func getSelector() map[string]string {
	return map[string]string{
		v1beta1constants.LabelApp:  v1beta1constants.LabelKubernetes,
		v1beta1constants.LabelRole: "apiserver-proxy",
	}
}

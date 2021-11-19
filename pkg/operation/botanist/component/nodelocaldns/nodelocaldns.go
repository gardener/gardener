// Copyright (c) 2021 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

package nodelocaldns

import (
	"context"
	"strconv"
	"time"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	resourcesv1alpha1 "github.com/gardener/gardener/pkg/apis/resources/v1alpha1"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	"github.com/gardener/gardener/pkg/operation/botanist/component"
	"github.com/gardener/gardener/pkg/operation/common"
	"github.com/gardener/gardener/pkg/resourcemanager/controller/garbagecollector/references"
	kutil "github.com/gardener/gardener/pkg/utils/kubernetes"
	"github.com/gardener/gardener/pkg/utils/managedresources"

	appsv1 "k8s.io/api/apps/v1"
	autoscalingv1 "k8s.io/api/autoscaling/v1"
	corev1 "k8s.io/api/core/v1"
	policyv1beta1 "k8s.io/api/policy/v1beta1"
	rbacv1 "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	autoscalingv1beta2 "k8s.io/autoscaler/vertical-pod-autoscaler/pkg/apis/autoscaling.k8s.io/v1beta2"
	"k8s.io/utils/pointer"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	// LabelKey is the key of a label used for the identification of NodeLocalDNS pods.
	LabelKey = "k8s-app"
	// LabelValue is the value of a label used for the identification of NodeLocalDNS pods.
	LabelValue = "node-local-dns"
	// PortServiceServer is the service port used for the DNS server.
	PortServiceServer = 53
	// PortServer is the target port used for the DNS server.
	PortServer = 8053
	// ManagedResourceName is the name of the ManagedResource containing the resource specifications.
	ManagedResourceName = "shoot-core-nodelocaldns"

	// prometheus configuration fot node-local-dns
	prometheusPort   = 9253
	prometheusScrape = true

	nodeLocal     = common.NodeLocalIPVSAddress
	domain        = gardencorev1beta1.DefaultDomain
	serviceName   = "kube-dns-upstream"
	configDataKey = "Corefile"
)

// Interface contains functions for a NodeLocalDNS deployer.
type Interface interface {
	component.DeployWaiter
	component.MonitoringComponent
}

// Values is a set of configuration values for the nodelocaldns component.
type Values struct {
	// Image is the container image used for NodeLocalDNS.
	Image string
	// VPAEnabled marks whether VerticalPodAutoscaler is enabled for the shoot.
	VPAEnabled bool
	// ForceTcpToClusterDNS enforces upgrade to tcp connections for communication between node local and cluster dns.
	ForceTcpToClusterDNS bool
	// ForceTcpToUpstreamDNS enforces upgrade to tcp connections for communication between node local and upstream dns.
	ForceTcpToUpstreamDNS bool
	// ClusterDNS
	ClusterDNS string
	// DNSServer
	DNSServer string
}

// New creates a new instance of DeployWaiter for nodelocaldns.
func New(
	client client.Client,
	namespace string,
	values Values,
) Interface {
	return &nodeLocalDNS{
		client:    client,
		namespace: namespace,
		values:    values,
	}
}

type nodeLocalDNS struct {
	client    client.Client
	namespace string
	values    Values
}

func (c *nodeLocalDNS) Deploy(ctx context.Context) error {
	data, err := c.computeResourcesData()
	if err != nil {
		return err
	}
	return managedresources.CreateForShoot(ctx, c.client, c.namespace, ManagedResourceName, false, data)
}

func (c *nodeLocalDNS) Destroy(ctx context.Context) error {
	return managedresources.DeleteForShoot(ctx, c.client, c.namespace, ManagedResourceName)
}

// TimeoutWaitForManagedResource is the timeout used while waiting for the ManagedResources to become healthy
// or deleted.
var TimeoutWaitForManagedResource = 2 * time.Minute

func (c *nodeLocalDNS) Wait(ctx context.Context) error {
	timeoutCtx, cancel := context.WithTimeout(ctx, TimeoutWaitForManagedResource)
	defer cancel()

	return managedresources.WaitUntilHealthy(timeoutCtx, c.client, c.namespace, ManagedResourceName)
}

func (c *nodeLocalDNS) WaitCleanup(ctx context.Context) error {
	timeoutCtx, cancel := context.WithTimeout(ctx, TimeoutWaitForManagedResource)
	defer cancel()

	return managedresources.WaitUntilDeleted(timeoutCtx, c.client, c.namespace, ManagedResourceName)
}

func (c *nodeLocalDNS) computeResourcesData() (map[string][]byte, error) {
	var (
		hostPathFileOrCreate = corev1.HostPathFileOrCreate
		registry             = managedresources.NewRegistry(kubernetes.ShootScheme, kubernetes.ShootCodec, kubernetes.ShootSerializer)

		serviceAccount = &corev1.ServiceAccount{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "node-local-dns",
				Namespace: metav1.NamespaceSystem,
			},
		}

		podSecurityPolicy = &policyv1beta1.PodSecurityPolicy{
			ObjectMeta: metav1.ObjectMeta{
				Name: "gardener.kube-system.node-local-dns",
				Labels: map[string]string{
					v1beta1constants.LabelApp: LabelValue,
				},
			},
			Spec: policyv1beta1.PodSecurityPolicySpec{
				AllowedHostPaths: []policyv1beta1.AllowedHostPath{
					{
						PathPrefix: "/run/xtables.lock",
					},
				},
				FSGroup: policyv1beta1.FSGroupStrategyOptions{
					Rule: policyv1beta1.FSGroupStrategyRunAsAny,
				},
				HostNetwork: true,
				HostPorts: []policyv1beta1.HostPortRange{
					{
						Min: int32(53),
						Max: int32(53),
					},
					{
						Min: prometheusPort,
						Max: prometheusPort,
					},
				},
				Privileged: true,
				RunAsUser: policyv1beta1.RunAsUserStrategyOptions{
					Rule: policyv1beta1.RunAsUserStrategyRunAsAny,
				},
				SELinux: policyv1beta1.SELinuxStrategyOptions{
					Rule: policyv1beta1.SELinuxStrategyRunAsAny,
				},
				SupplementalGroups: policyv1beta1.SupplementalGroupsStrategyOptions{
					Rule: policyv1beta1.SupplementalGroupsStrategyRunAsAny,
				},
				Volumes: []policyv1beta1.FSType{
					"secret",
					"hostPath",
					"configMap",
				},
			},
		}

		clusterRole = &rbacv1.ClusterRole{
			ObjectMeta: metav1.ObjectMeta{
				Name: "gardener.cloud:psp:kube-system:node-local-dns",
				Labels: map[string]string{
					v1beta1constants.LabelApp: LabelValue,
				},
			},
			Rules: []rbacv1.PolicyRule{
				{
					APIGroups:     []string{"policy", "extensions"},
					ResourceNames: []string{"gardener.kube-system.node-local-dns"},
					Resources:     []string{"podsecuritypolicies"},
					Verbs:         []string{"use"},
				},
			},
		}

		roleBinding = &rbacv1.RoleBinding{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "gardener.cloud:psp:node-local-dns",
				Namespace: metav1.NamespaceSystem,
				Labels: map[string]string{
					v1beta1constants.LabelApp: LabelValue,
				},
				Annotations: map[string]string{resourcesv1alpha1.DeleteOnInvalidUpdate: "true"},
			},
			RoleRef: rbacv1.RoleRef{
				APIGroup: rbacv1.GroupName,
				Kind:     "ClusterRole",
				Name:     clusterRole.Name,
			},
			Subjects: []rbacv1.Subject{{
				Kind:      rbacv1.ServiceAccountKind,
				Name:      serviceAccount.Name,
				Namespace: serviceAccount.Namespace,
			}},
		}

		configMap = &corev1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "node-local-dns",
				Namespace: metav1.NamespaceSystem,
			},
			Data: map[string]string{
				configDataKey: domain + `:53 {
  errors
  cache {
          success 9984 30
          denial 9984 5
  }
  reload
  loop
  bind ` + c.bindIP() + `
  forward . ` + c.values.ClusterDNS + ` {
          ` + c.forceTcpToClusterDNS() + `
  }
  prometheus :` + strconv.Itoa(prometheusPort) + `
  health ` + nodeLocal + `:8080
  }
in-addr.arpa:53 {
  errors
  cache 30
  reload
  loop
  bind ` + c.bindIP() + `
  forward . ` + c.values.ClusterDNS + ` {
          ` + c.forceTcpToClusterDNS() + `
  }
  prometheus :` + strconv.Itoa(prometheusPort) + `
  }
ip6.arpa:53 {
  errors
  cache 30
  reload
  loop
  bind ` + c.bindIP() + `
  forward . ` + c.values.ClusterDNS + ` {
          ` + c.forceTcpToClusterDNS() + `
  }
  prometheus :` + strconv.Itoa(prometheusPort) + `
  }
.:53 {
  errors
  cache 30
  reload
  loop
  bind ` + c.bindIP() + `
  forward . __PILLAR__UPSTREAM__SERVERS__ {
          ` + c.forceTcpToUpstreamDNS() + `
  }
  prometheus :` + strconv.Itoa(prometheusPort) + `
  }
`,
			},
		}
	)
	utilruntime.Must(kutil.MakeUnique(configMap))

	var (
		service = &corev1.Service{
			ObjectMeta: metav1.ObjectMeta{
				Name:      serviceName,
				Namespace: metav1.NamespaceSystem,
				Labels: map[string]string{
					"k8s-app":                       "kube-dns-upstream",
					"kubernetes.io/cluster-service": "true",
					"kubernetes.io/name":            "NodeLocalDNS",
				},
			},
			Spec: corev1.ServiceSpec{
				Selector: map[string]string{"k8s-app": "kube-dns"},
				Ports: []corev1.ServicePort{
					{
						Name:       "dns",
						Port:       int32(PortServiceServer),
						TargetPort: intstr.FromInt(PortServer),
						Protocol:   corev1.ProtocolUDP,
					},
					{
						Name:       "dns-tcp",
						Port:       int32(PortServiceServer),
						TargetPort: intstr.FromInt(PortServer),
						Protocol:   corev1.ProtocolTCP,
					},
				},
			},
		}

		daemonset = &appsv1.DaemonSet{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "node-local-dns",
				Namespace: metav1.NamespaceSystem,
				Labels: map[string]string{
					LabelKey:                        LabelValue,
					v1beta1constants.GardenRole:     v1beta1constants.GardenRoleSystemComponent,
					managedresources.LabelKeyOrigin: managedresources.LabelValueGardener,
				},
			},
			Spec: appsv1.DaemonSetSpec{
				UpdateStrategy: appsv1.DaemonSetUpdateStrategy{
					RollingUpdate: &appsv1.RollingUpdateDaemonSet{
						MaxUnavailable: &intstr.IntOrString{IntVal: 10},
					},
				},
				Selector: &metav1.LabelSelector{
					MatchLabels: map[string]string{
						LabelKey: LabelValue,
					},
				},
				Template: corev1.PodTemplateSpec{
					ObjectMeta: metav1.ObjectMeta{
						Labels: map[string]string{
							LabelKey:                                 LabelValue,
							v1beta1constants.LabelNetworkPolicyToDNS: "allowed",
						},
						Annotations: map[string]string{
							"prometheus.io/port":   strconv.Itoa(prometheusPort),
							"prometheus.io/scrape": strconv.FormatBool(prometheusScrape),
						},
					},
					Spec: corev1.PodSpec{
						PriorityClassName:  "system-node-critical",
						ServiceAccountName: serviceAccount.Name,
						HostNetwork:        true,
						DNSPolicy:          corev1.DNSDefault,
						Tolerations: []corev1.Toleration{
							{
								Key:      "CriticalAddonsOnly",
								Operator: corev1.TolerationOpExists,
							},
							{
								Operator: corev1.TolerationOpExists,
								Effect:   corev1.TaintEffectNoExecute,
							},
							{
								Operator: corev1.TolerationOpExists,
								Effect:   corev1.TaintEffectNoSchedule,
							},
						},
						Containers: []corev1.Container{
							{
								Name:  "node-cache",
								Image: c.values.Image,
								Resources: corev1.ResourceRequirements{
									Limits: corev1.ResourceList{
										corev1.ResourceCPU:    resource.MustParse("100m"),
										corev1.ResourceMemory: resource.MustParse("100Mi"),
									},
									Requests: corev1.ResourceList{
										corev1.ResourceCPU:    resource.MustParse("25m"),
										corev1.ResourceMemory: resource.MustParse("25Mi"),
									},
								},
								Args: []string{
									"-localip",
									c.containerArg(),
									"-conf",
									"/etc/Corefile",
									"-upstreamsvc",
									"kube-dns-upstream",
								},
								SecurityContext: &corev1.SecurityContext{
									Privileged: pointer.Bool(true),
								},
								Ports: []corev1.ContainerPort{
									{
										ContainerPort: int32(53),
										Name:          "dns",
										Protocol:      corev1.ProtocolUDP,
									},
									{
										ContainerPort: int32(53),
										Name:          "dns-tcp",
										Protocol:      corev1.ProtocolTCP,
									},
									{
										ContainerPort: int32(prometheusPort),
										Name:          "metrics",
										Protocol:      corev1.ProtocolTCP,
									},
								},
								LivenessProbe: &corev1.Probe{
									Handler: corev1.Handler{
										HTTPGet: &corev1.HTTPGetAction{
											Host: nodeLocal,
											Path: "/health",
											Port: intstr.FromInt(8080),
										},
									},
									InitialDelaySeconds: int32(60),
									TimeoutSeconds:      int32(5),
								},
								VolumeMounts: []corev1.VolumeMount{
									{
										MountPath: "/run/xtables.lock",
										Name:      "xtables-lock",
										ReadOnly:  false,
									},
									{
										MountPath: "/etc/coredns",
										Name:      "config-volume",
									},
									{
										MountPath: "/etc/kube-dns",
										Name:      "kube-dns-config",
									},
								},
							},
						},
						Volumes: []corev1.Volume{
							{
								Name: "xtables-lock",
								VolumeSource: corev1.VolumeSource{
									HostPath: &corev1.HostPathVolumeSource{
										Path: "/run/xtables.lock",
										Type: &hostPathFileOrCreate,
									},
								},
							},
							{
								Name: "kube-dns-config",
								VolumeSource: corev1.VolumeSource{
									ConfigMap: &corev1.ConfigMapVolumeSource{
										LocalObjectReference: corev1.LocalObjectReference{
											Name: "kube-dns",
										},
										Optional: pointer.Bool(true),
									},
								},
							},
							{
								Name: "config-volume",
								VolumeSource: corev1.VolumeSource{
									ConfigMap: &corev1.ConfigMapVolumeSource{
										LocalObjectReference: corev1.LocalObjectReference{
											Name: configMap.Name,
										},
										Items: []corev1.KeyToPath{
											{
												Key:  configDataKey,
												Path: "Corefile.base",
											},
										},
									},
								},
							},
						},
					},
				},
			},
		}
		vpa *autoscalingv1beta2.VerticalPodAutoscaler
	)
	utilruntime.Must(references.InjectAnnotations(daemonset))

	if c.values.VPAEnabled {
		vpaUpdateMode := autoscalingv1beta2.UpdateModeAuto
		vpa = &autoscalingv1beta2.VerticalPodAutoscaler{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "node-local-dns",
				Namespace: metav1.NamespaceSystem,
			},
			Spec: autoscalingv1beta2.VerticalPodAutoscalerSpec{
				TargetRef: &autoscalingv1.CrossVersionObjectReference{
					APIVersion: appsv1.SchemeGroupVersion.String(),
					Kind:       "DaemonSet",
					Name:       daemonset.Name,
				},
				UpdatePolicy: &autoscalingv1beta2.PodUpdatePolicy{
					UpdateMode: &vpaUpdateMode,
				},
				ResourcePolicy: &autoscalingv1beta2.PodResourcePolicy{
					ContainerPolicies: []autoscalingv1beta2.ContainerResourcePolicy{
						{
							ContainerName: autoscalingv1beta2.DefaultContainerResourcePolicy,
							MinAllowed: corev1.ResourceList{
								corev1.ResourceCPU:    resource.MustParse("50m"),
								corev1.ResourceMemory: resource.MustParse("150Mi"),
							},
						},
					},
				},
			},
		}
	}

	return registry.AddAllAndSerialize(
		serviceAccount,
		podSecurityPolicy,
		clusterRole,
		roleBinding,
		configMap,
		service,
		daemonset,
		vpa,
	)
}

func (c *nodeLocalDNS) bindIP() string {
	if c.values.DNSServer != "" {
		return nodeLocal + " " + c.values.DNSServer
	}
	return nodeLocal
}
func (c *nodeLocalDNS) containerArg() string {
	if c.values.DNSServer != "" {
		return nodeLocal + "," + c.values.DNSServer
	}
	return nodeLocal
}
func (c *nodeLocalDNS) forceTcpToClusterDNS() string {
	if c.values.ForceTcpToClusterDNS {
		return "force_tcp"
	}
	return "prefer_udp"
}
func (c *nodeLocalDNS) forceTcpToUpstreamDNS() string {
	if c.values.ForceTcpToUpstreamDNS {
		return "force_tcp"
	}
	return "prefer_udp"
}

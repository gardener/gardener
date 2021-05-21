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

package konnectivity

import (
	"context"
	"errors"
	"fmt"
	"time"

	gardenercorev1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	"github.com/gardener/gardener/pkg/operation/botanist/component"
	"github.com/gardener/gardener/pkg/utils"
	"github.com/gardener/gardener/pkg/utils/managedresources"

	"github.com/gogo/protobuf/types"
	istionetworkingv1beta1 "istio.io/api/networking/v1beta1"
	networkingv1beta1 "istio.io/client-go/pkg/apis/networking/v1beta1"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	policyv1beta1 "k8s.io/api/policy/v1beta1"
	rbacv1 "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/utils/pointer"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	// ServerName is the name of kubernetes resources associated with konnectivity-server.
	ServerName = "konnectivity-server"
	// SecretNameServerKubeconfig is the name for the konnectivity-server's kubeconfig secret.
	SecretNameServerKubeconfig = "konnectivity-server-kubeconfig"
	// SecretNameServerTLS is the name of the konnectivity-server server certificate secret.
	SecretNameServerTLS = ServerName
	// SecretNameServerCA is the name of the konnectivity-server server certificate authority secret.
	SecretNameServerCA = ServerName + "-ca"
	// ServerCASecretName is the name of the konnectivity-server server certificate authority secret.
	SecretNameServerTLSClient = ServerName + "-client-tls"
	// ServerAudience is the audience of the konnectivity-server used for the token
	ServerAudience = "system:konnectivity-server"
	// ServerHTTPSPort is the port on which konnectivity-server receives traffic from the kube-apiserver.
	ServerHTTPSPort int32 = 9443
	// ServerAgentPort is the port on which konnectivity-server receives traffic from konnectivity-agents.
	ServerAgentPort int32 = 8132
)

// Interface contains functions for a konnectivity-server deployer.
type Interface interface {
	component.DeployWaiter
	// SetSecrets sets the konnectivity-server's secrets.
	SetSecrets(ServerSecrets)
}

// Prober probes specific resource.
type Prober func(ctx context.Context, client client.Client, namespace, name string) error

// ServerSecrets contains secrets used for the konnectivity-server
type ServerSecrets struct {
	// Kubeconfig is a secret which can be used by the konnectivity-server to communicate to the kube-apiserver.
	Kubeconfig component.Secret
	// Server is a secret for the HTTPS server.
	Server component.Secret
	// ClientCA is a secret with the CA used by kube-apiserver to authenticate to konnectivity-server.
	ClientCA component.Secret
}

// ServerOptions are the options for the konnectivity-server.
type ServerOptions struct {
	// Client to create resources with.
	Client client.Client
	// Namespace in the seed cluster.
	Namespace string
	// Image of the konnectivity-server.
	Image string
	// Replica count of the konnectivity-server.
	Replicas int32
	// Hosts used for SNI.
	Hosts []string
	// IstioIngressLabels are the istio-ingressgateway's labels.
	IstioIngressLabels map[string]string
	// Healthy probes that konnectivity-server is healthy.
	// Defaults to managedresources.WaitUntilHealthy.
	Healthy Prober
	// Removed probes that konnectivity-server is removed.
	// Defaults to managedresources.WaitUntilDeleted.
	Removed Prober
}

// NewServer creates a new instance of Interface for the konnectivity-server.
func NewServer(so *ServerOptions) (Interface, error) {
	if so == nil {
		return nil, errors.New("serveroptions are required")
	}

	if so.Client == nil {
		return nil, errors.New("client cannot be nil")
	}

	if len(so.Namespace) == 0 {
		return nil, errors.New("namespace cannot be empty")
	}

	if so.Healthy == nil {
		so.Healthy = managedresources.WaitUntilHealthy
	}

	if so.Removed == nil {
		so.Removed = managedresources.WaitUntilDeleted
	}

	return &konnectivityServer{ServerOptions: so}, nil
}

// OpDestroy destroys components created by connectivity server instead of creating them.
func OpDestroy(ks Interface) Interface {
	return &opDestroy{component.OpDestroy(ks)}
}

type konnectivityServer struct {
	*ServerOptions
	secrets ServerSecrets
}

func (k *konnectivityServer) Deploy(ctx context.Context) error {
	if k.secrets.Kubeconfig.Name == "" || k.secrets.Kubeconfig.Checksum == "" {
		return errors.New("missing kubeconfig secret information")
	}
	if k.secrets.Server.Name == "" || k.secrets.Server.Checksum == "" {
		return errors.New("missing server secret information")
	}
	if k.secrets.ClientCA.Name == "" || k.secrets.ClientCA.Checksum == "" {
		return errors.New("missing client-ca secret information")
	}
	if len(k.Hosts) == 0 {
		return errors.New("hosts cannot be empty")
	}
	if len(k.Image) == 0 {
		return errors.New("image cannot be empty")
	}
	if len(k.IstioIngressLabels) == 0 {
		return errors.New("istioingreslabels cannot be empty")
	}

	const (
		name                          = ServerName
		adminPort               int32 = 8133
		healthPort              int32 = 8134
		serverAgentPortUnsigned       = uint32(ServerAgentPort)
		serverMountPath               = "/certs/konnectivity-server"
		clientCAMountPath             = "/certs/client-ca"
		kubeconfigMountPath           = "/auth"
	)

	var (
		minAvailable                  = intstr.FromInt(1)
		tcpProto                      = corev1.ProtocolTCP
		konnectivityServerServiceHost = fmt.Sprintf("%s.%s.svc.%s", name, k.Namespace, gardenercorev1.DefaultDomain)

		serviceAccount = &corev1.ServiceAccount{ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: k.Namespace,
			Labels:    getLabels(),
		}}
		deployment = &appsv1.Deployment{
			ObjectMeta: metav1.ObjectMeta{
				Name:      name,
				Namespace: k.Namespace,
				Labels:    getLabels(),
			},
			Spec: appsv1.DeploymentSpec{
				Replicas:             pointer.Int32Ptr(k.Replicas),
				RevisionHistoryLimit: pointer.Int32Ptr(1),
				Selector:             &metav1.LabelSelector{MatchLabels: getLabels()},
				Strategy: appsv1.DeploymentStrategy{
					Type: appsv1.RollingUpdateDeploymentStrategyType,
					RollingUpdate: &appsv1.RollingUpdateDeployment{
						MaxUnavailable: &intstr.IntOrString{Type: intstr.Int, IntVal: 1},
						MaxSurge:       &intstr.IntOrString{Type: intstr.Int, IntVal: 1},
					},
				},
				Template: corev1.PodTemplateSpec{
					ObjectMeta: metav1.ObjectMeta{
						Annotations: map[string]string{
							"checksum/secret-" + k.secrets.Kubeconfig.Name: k.secrets.Kubeconfig.Checksum,
							"checksum/secret-" + k.secrets.Server.Name:     k.secrets.Server.Checksum,
							"checksum/secret-" + k.secrets.ClientCA.Name:   k.secrets.ClientCA.Checksum,
						},
						Labels: utils.MergeStringMaps(getLabels(), map[string]string{
							v1beta1constants.LabelNetworkPolicyToDNS:              v1beta1constants.LabelNetworkPolicyAllowed,
							v1beta1constants.LabelNetworkPolicyFromShootAPIServer: v1beta1constants.LabelNetworkPolicyAllowed,
							v1beta1constants.LabelNetworkPolicyToShootAPIServer:   v1beta1constants.LabelNetworkPolicyAllowed,
							v1beta1constants.LabelNetworkPolicyToSeedAPIServer:    v1beta1constants.LabelNetworkPolicyAllowed,
						}),
					},
					Spec: corev1.PodSpec{
						Affinity: &corev1.Affinity{
							PodAntiAffinity: &corev1.PodAntiAffinity{
								PreferredDuringSchedulingIgnoredDuringExecution: []corev1.WeightedPodAffinityTerm{{
									Weight: 100,
									PodAffinityTerm: corev1.PodAffinityTerm{
										TopologyKey:   corev1.LabelHostname,
										LabelSelector: &metav1.LabelSelector{MatchLabels: getLabels()},
									},
								}},
							},
						},
						ServiceAccountName: name,
						Containers: []corev1.Container{{
							Name:            name,
							Image:           k.Image,
							ImagePullPolicy: corev1.PullIfNotPresent,
							Args: []string{
								fmt.Sprintf("--namespace=%s", k.Namespace),
								fmt.Sprintf("--deployment-name=%s", name),
								"--jitter=10s",
								"--jitter-factor=5",
								"--v=2",
								"--",
								"/proxy-server",
								"--logtostderr=true",
								fmt.Sprintf("--cluster-cert=%s/konnectivity-server.crt", serverMountPath),
								fmt.Sprintf("--cluster-key=%s/konnectivity-server.key", serverMountPath),
								fmt.Sprintf("--server-cert=%s/konnectivity-server.crt", serverMountPath),
								fmt.Sprintf("--server-key=%s/konnectivity-server.key", serverMountPath),
								fmt.Sprintf("--server-ca-cert=%s/ca.crt", clientCAMountPath),
								fmt.Sprintf("--agent-namespace=%s", metav1.NamespaceSystem),
								fmt.Sprintf("--agent-service-account=%s", AgentName),
								fmt.Sprintf("--kubeconfig=%s/kubeconfig", kubeconfigMountPath),
								fmt.Sprintf("--authentication-audience=%s", ServerAudience),
								"--keepalive-time=1m",
								"--log-file-max-size=0",
								"--delete-existing-uds-file=true",
								"--mode=http-connect",
								fmt.Sprintf("--server-port=%d", ServerHTTPSPort),
								fmt.Sprintf("--agent-port=%d", ServerAgentPort),
								fmt.Sprintf("--admin-port=%d", adminPort),
								fmt.Sprintf("--health-port=%d", healthPort),
								"--v=2",
								"--server-count",
							},
							LivenessProbe: &corev1.Probe{
								Handler: corev1.Handler{
									HTTPGet: &corev1.HTTPGetAction{
										Path:   "/healthz",
										Port:   intstr.IntOrString{Type: intstr.Int, IntVal: healthPort},
										Scheme: corev1.URISchemeHTTP,
									},
								},
							},
							Resources: corev1.ResourceRequirements{
								Requests: corev1.ResourceList{
									corev1.ResourceCPU:    resource.MustParse("30m"),
									corev1.ResourceMemory: resource.MustParse("40Mi"),
								},
								Limits: corev1.ResourceList{
									corev1.ResourceCPU:    resource.MustParse("300m"),
									corev1.ResourceMemory: resource.MustParse("400Mi"),
								},
							},
							Ports: []corev1.ContainerPort{{
								Name:          "server",
								ContainerPort: ServerHTTPSPort,
							}, {
								Name:          "agent",
								ContainerPort: ServerAgentPort,
							}, {
								Name:          "admin",
								ContainerPort: adminPort,
							}, {
								Name:          "health",
								ContainerPort: healthPort,
							}},
							VolumeMounts: []corev1.VolumeMount{
								{
									Name:      k.secrets.Server.Name,
									MountPath: serverMountPath,
								},
								{
									Name:      k.secrets.ClientCA.Name,
									MountPath: clientCAMountPath,
								},
								{
									Name:      k.secrets.Kubeconfig.Name,
									MountPath: kubeconfigMountPath,
								},
							},
						}},
						Volumes: []corev1.Volume{
							{
								Name: k.secrets.Server.Name,
								VolumeSource: corev1.VolumeSource{
									Secret: &corev1.SecretVolumeSource{
										SecretName: k.secrets.Server.Name,
									},
								},
							},
							{
								Name: k.secrets.ClientCA.Name,
								VolumeSource: corev1.VolumeSource{
									Secret: &corev1.SecretVolumeSource{
										SecretName: k.secrets.ClientCA.Name,
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
						},
					},
				},
			},
		}
		gateway = &networkingv1beta1.Gateway{
			ObjectMeta: metav1.ObjectMeta{
				Name:      name,
				Namespace: k.Namespace,
				Labels:    getLabels(),
			},
			Spec: istionetworkingv1beta1.Gateway{
				Selector: k.IstioIngressLabels,
				Servers: []*istionetworkingv1beta1.Server{{
					Hosts: k.Hosts,
					Port: &istionetworkingv1beta1.Port{
						Number:   serverAgentPortUnsigned,
						Protocol: "TLS",
						Name:     "tls-tunnel",
					},
					Tls: &istionetworkingv1beta1.ServerTLSSettings{
						Mode: istionetworkingv1beta1.ServerTLSSettings_PASSTHROUGH,
					},
				}},
			},
		}
		virtualService = &networkingv1beta1.VirtualService{
			ObjectMeta: metav1.ObjectMeta{
				Name:      name,
				Namespace: k.Namespace,
				Labels:    getLabels(),
			},
			Spec: istionetworkingv1beta1.VirtualService{
				Hosts:    k.Hosts,
				Gateways: []string{name},
				ExportTo: []string{"*"},
				Tls: []*istionetworkingv1beta1.TLSRoute{{
					Match: []*istionetworkingv1beta1.TLSMatchAttributes{{
						SniHosts: k.Hosts,
						Port:     serverAgentPortUnsigned,
					}},
					Route: []*istionetworkingv1beta1.RouteDestination{{
						Destination: &istionetworkingv1beta1.Destination{
							Host: konnectivityServerServiceHost,
							Port: &istionetworkingv1beta1.PortSelector{
								Number: serverAgentPortUnsigned,
							},
						},
					}},
				}},
			},
		}
		destinationRule = &networkingv1beta1.DestinationRule{
			ObjectMeta: metav1.ObjectMeta{
				Name:      name,
				Namespace: k.Namespace,
				Labels:    getLabels(),
			},
			Spec: istionetworkingv1beta1.DestinationRule{
				ExportTo: []string{"*"},
				Host:     konnectivityServerServiceHost,
				TrafficPolicy: &istionetworkingv1beta1.TrafficPolicy{
					ConnectionPool: &istionetworkingv1beta1.ConnectionPoolSettings{
						Tcp: &istionetworkingv1beta1.ConnectionPoolSettings_TCPSettings{
							MaxConnections: 5000,
							TcpKeepalive: &istionetworkingv1beta1.ConnectionPoolSettings_TCPSettings_TcpKeepalive{
								Time:     &types.Duration{Seconds: 7200},
								Interval: &types.Duration{Seconds: 75},
							},
						},
					},
					Tls: &istionetworkingv1beta1.ClientTLSSettings{
						Mode: istionetworkingv1beta1.ClientTLSSettings_DISABLE,
					},
				},
			},
		}
		podDisruptionBudget = &policyv1beta1.PodDisruptionBudget{
			ObjectMeta: metav1.ObjectMeta{
				Name:      name,
				Namespace: k.Namespace,
				Labels:    getLabels(),
			},
			Spec: policyv1beta1.PodDisruptionBudgetSpec{
				MinAvailable: &minAvailable,
				Selector: &metav1.LabelSelector{
					MatchLabels: getLabels(),
				},
			},
		}
		service = &corev1.Service{
			ObjectMeta: metav1.ObjectMeta{
				Name:        name,
				Namespace:   k.Namespace,
				Annotations: map[string]string{"networking.istio.io/exportTo": "*"},
				Labels:      getLabels(),
			},
			Spec: corev1.ServiceSpec{
				Selector: getLabels(),
				Type:     corev1.ServiceTypeClusterIP,
				Ports: []corev1.ServicePort{
					{
						Name:     "server",
						Protocol: corev1.ProtocolTCP,
						Port:     ServerHTTPSPort,
					}, {
						Name:     "agent",
						Protocol: corev1.ProtocolTCP,
						Port:     ServerAgentPort,
					},
				},
			},
		}
		role = &rbacv1.Role{
			ObjectMeta: metav1.ObjectMeta{
				Name:      name,
				Namespace: k.Namespace,
				Labels:    getLabels(),
			},
			Rules: []rbacv1.PolicyRule{{
				Verbs:         []string{"get", "list", "watch"},
				APIGroups:     []string{appsv1.SchemeGroupVersion.Group},
				Resources:     []string{"deployments"},
				ResourceNames: []string{name},
			}},
		}
		rolebinding = &rbacv1.RoleBinding{
			ObjectMeta: metav1.ObjectMeta{
				Name:      name,
				Namespace: k.Namespace,
				Labels:    getLabels(),
			},
			RoleRef: rbacv1.RoleRef{
				APIGroup: rbacv1.GroupName,
				Kind:     "Role",
				Name:     name,
			},
			Subjects: []rbacv1.Subject{{
				Kind:      rbacv1.ServiceAccountKind,
				Name:      name,
				Namespace: k.Namespace,
			}},
		}
		networkPolicy = &networkingv1.NetworkPolicy{
			ObjectMeta: metav1.ObjectMeta{
				Name:      name,
				Namespace: k.Namespace,
				Labels:    getLabels(),
			},
			Spec: networkingv1.NetworkPolicySpec{
				PodSelector: metav1.LabelSelector{
					MatchLabels: getLabels(),
				},
				PolicyTypes: []networkingv1.PolicyType{networkingv1.PolicyTypeIngress},
				Egress:      []networkingv1.NetworkPolicyEgressRule{},
				Ingress: []networkingv1.NetworkPolicyIngressRule{{
					From: []networkingv1.NetworkPolicyPeer{{
						PodSelector: &metav1.LabelSelector{
							MatchLabels: map[string]string{"app": "istio-ingressgateway"},
						},
						// match all namespaces since there is no guarantee about the labels of the istio-ingress namespace
						NamespaceSelector: &metav1.LabelSelector{},
					}},
					Ports: []networkingv1.NetworkPolicyPort{{
						Protocol: &tcpProto,
						Port:     &intstr.IntOrString{Type: intstr.Int, IntVal: ServerAgentPort},
					}},
				}},
			},
		}

		registry = managedresources.NewRegistry(kubernetes.SeedScheme, kubernetes.SeedCodec, kubernetes.SeedSerializer)
	)

	resources, err := registry.AddAllAndSerialize(
		podDisruptionBudget,
		serviceAccount,
		role,
		rolebinding,
		networkPolicy,
		deployment,
		service,
		gateway,
		virtualService,
		destinationRule,
	)
	if err != nil {
		return err
	}

	return managedresources.CreateForSeed(ctx, k.Client, k.Namespace, ServerName, false, resources)
}

func (k *konnectivityServer) Destroy(ctx context.Context) error {
	return managedresources.DeleteForSeed(ctx, k.Client, k.Namespace, ServerName)
}

func (k *konnectivityServer) Wait(ctx context.Context) error {
	timeoutCtx, cancel := context.WithTimeout(ctx, time.Minute*2)
	defer cancel()

	return k.Healthy(timeoutCtx, k.Client, k.Namespace, ServerName)
}

func (k *konnectivityServer) WaitCleanup(ctx context.Context) error {
	timeoutCtx, cancel := context.WithTimeout(ctx, time.Minute*2)
	defer cancel()

	return k.Removed(timeoutCtx, k.Client, k.Namespace, ServerName)
}

func (k *konnectivityServer) SetSecrets(secrets ServerSecrets) { k.secrets = secrets }

type opDestroy struct {
	component.DeployWaiter
}

func (_ *opDestroy) SetSecrets(ServerSecrets) {}

func getLabels() map[string]string {
	return map[string]string{
		"app": ServerName,
	}
}

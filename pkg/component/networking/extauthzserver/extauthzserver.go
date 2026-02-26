// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package extauthzserver

import (
	"context"
	"fmt"
	"path"
	"strings"
	"time"

	"google.golang.org/protobuf/types/known/structpb"
	istioapinetworkingv1alpha3 "istio.io/api/networking/v1alpha3"
	istionetworkingv1beta1 "istio.io/client-go/pkg/apis/networking/v1beta1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/apiutil"

	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	operatorv1alpha1 "github.com/gardener/gardener/pkg/apis/operator/v1alpha1"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	"github.com/gardener/gardener/pkg/component"
	gardenerutils "github.com/gardener/gardener/pkg/utils/gardener"
	kubernetesutils "github.com/gardener/gardener/pkg/utils/kubernetes"
	"github.com/gardener/gardener/pkg/utils/managedresources"
	secretsutils "github.com/gardener/gardener/pkg/utils/secrets"
	secretsmanager "github.com/gardener/gardener/pkg/utils/secrets/manager"
)

const (
	// Port is the port exposed by the ext-authz-server.
	Port = 10000

	name                = v1beta1constants.DeploymentNameExtAuthzServer
	managedResourceName = name
	svcName             = name

	rootMountPath = "/secrets"
	tlsMountPath  = "/tls"

	tlsServerCertificateName = "tls-server-certificate"

	timeoutWaitForManagedResources = 2 * time.Minute
)

// Values is the values for ext-authz-server configuration.
type Values struct {
	// Image is the ext-authz-server container image.
	Image string
	// PriorityClassName is the name of the priority class of the ext-authz-server.
	PriorityClassName string
	// Replicas is the number of pod replicas for the ext-authz-server.
	Replicas int32
	// IsGardenCluster specifies whether the cluster is garden cluster.
	IsGardenCluster bool
}

type extAuthzServer struct {
	client         client.Client
	namespace      string
	secretsManager secretsmanager.Interface
	values         Values
}

// New creates a new instance of an ext-authz-server deployer.
func New(
	client client.Client,
	namespace string,
	secretsManager secretsmanager.Interface,
	values Values,
) component.DeployWaiter {
	return &extAuthzServer{
		client:         client,
		namespace:      namespace,
		secretsManager: secretsManager,
		values:         values,
	}
}

func (e *extAuthzServer) Deploy(ctx context.Context) error {
	registry := managedresources.NewRegistry(kubernetes.SeedScheme, kubernetes.SeedCodec, kubernetes.SeedSerializer)
	destinationHost := kubernetesutils.FQDNForService(e.getPrefix()+svcName, e.namespace)
	caName := fmt.Sprintf("ca-%s%s", e.getPrefix(), name)

	caSecret, err := e.secretsManager.Generate(ctx,
		&secretsutils.CertificateSecretConfig{
			Name:       caName,
			CommonName: "ext-authz-server-ca",
			CertType:   secretsutils.CACert,
		},
		secretsmanager.Rotate(secretsmanager.InPlace),
		secretsmanager.Namespace(e.namespace),
	)
	if err != nil {
		return fmt.Errorf("failed to generate ca certificate: %w", err)
	}

	serverSecret, err := e.secretsManager.Generate(ctx,
		&secretsutils.CertificateSecretConfig{
			Name:       e.getPrefix() + name,
			CommonName: destinationHost,
			DNSNames:   kubernetesutils.DNSNamesForService(e.getPrefix()+svcName, e.namespace),
			CertType:   secretsutils.ServerCert,
		},
		secretsmanager.SignedByCA(caName),
		secretsmanager.Rotate(secretsmanager.InPlace),
		secretsmanager.Namespace(e.namespace),
	)
	if err != nil {
		return fmt.Errorf("failed to generate server certificate: %w", err)
	}

	secretNameInIstioNamespace := fmt.Sprintf("%s-%s", e.namespace, caSecret.Name)

	ownerNamespace := &corev1.Namespace{}
	if err := e.client.Get(ctx, client.ObjectKey{Name: e.namespace}, ownerNamespace); err != nil {
		return fmt.Errorf("failed to get namespace %q: %w", e.namespace, err)
	}
	ownerNamespaceGVK, err := apiutil.GVKForObject(ownerNamespace, kubernetes.SeedScheme)
	if err != nil {
		return fmt.Errorf("failed to get GVK for namespace %q: %w", ownerNamespace.Name, err)
	}
	ownerReference := &metav1.OwnerReference{
		APIVersion:         ownerNamespaceGVK.GroupVersion().String(),
		Kind:               ownerNamespaceGVK.Kind,
		Name:               ownerNamespace.Name,
		UID:                ownerNamespace.UID,
		BlockOwnerDeletion: ptr.To(true),
	}

	volumes, volumeMounts, configPatches, err := e.calculateConfiguration(ctx, serverSecret)
	if err != nil {
		return fmt.Errorf("failed to calculate configuration for ext-authz-server: %w", err)
	}

	destinationRule, err := e.getDestinationRule(destinationHost, secretNameInIstioNamespace)
	if err != nil {
		return fmt.Errorf("failed to create destination rule for ext-authz-server: %w", err)
	}

	isShootNamespace, err := gardenerutils.IsShootNamespace(ctx, e.client, e.namespace)
	if err != nil {
		return fmt.Errorf("failed checking if namespace is a shoot namespace: %w", err)
	}

	serializedResources, err := registry.AddAllAndSerialize(
		e.getDeployment(volumes, volumeMounts),
		e.getService(isShootNamespace),
		destinationRule,
		e.getEnvoyFilter(configPatches, ownerReference),
		e.getTLSSecret(caSecret, secretNameInIstioNamespace, ownerReference),
		e.getVPA(),
	)
	if err != nil {
		return fmt.Errorf("failed to serialize resources: %w", err)
	}

	return managedresources.CreateForSeed(ctx, e.client, e.namespace, e.getPrefix()+managedResourceName, false, serializedResources)
}

func (e *extAuthzServer) Destroy(ctx context.Context) error {
	return managedresources.DeleteForSeed(ctx, e.client, e.namespace, e.getPrefix()+managedResourceName)
}

func (e *extAuthzServer) Wait(ctx context.Context) error {
	timeoutCtx, cancel := context.WithTimeout(ctx, timeoutWaitForManagedResources)
	defer cancel()

	return managedresources.WaitUntilHealthy(timeoutCtx, e.client, e.namespace, e.getPrefix()+managedResourceName)
}

func (e *extAuthzServer) WaitCleanup(ctx context.Context) error {
	timeoutCtx, cancel := context.WithTimeout(ctx, timeoutWaitForManagedResources)
	defer cancel()

	return managedresources.WaitUntilDeleted(timeoutCtx, e.client, e.namespace, e.getPrefix()+managedResourceName)
}

func (e *extAuthzServer) calculateConfiguration(
	ctx context.Context,
	tlsSecret *corev1.Secret,
) ([]corev1.Volume, []corev1.VolumeMount, []*istioapinetworkingv1alpha3.EnvoyFilter_EnvoyConfigObjectPatch, error) {
	virtualServiceList := &istionetworkingv1beta1.VirtualServiceList{}
	err := e.client.List(ctx, virtualServiceList, client.InNamespace(e.namespace), client.HasLabels{v1beta1constants.LabelBasicAuthSecretName})
	if err != nil {
		return nil, nil, nil, fmt.Errorf("unable to list virtual services: %w", err)
	}

	var (
		volumes = []corev1.Volume{{
			Name: tlsServerCertificateName,
			VolumeSource: corev1.VolumeSource{
				Secret: &corev1.SecretVolumeSource{
					SecretName: tlsSecret.Name,
				},
			},
		}}
		volumeMounts = []corev1.VolumeMount{{
			Name:      tlsServerCertificateName,
			MountPath: tlsMountPath,
			ReadOnly:  true,
		}}
		configPatches []*istioapinetworkingv1alpha3.EnvoyFilter_EnvoyConfigObjectPatch
	)

	for _, virtualService := range virtualServiceList.Items {
		for _, host := range virtualService.Spec.Hosts {
			// Use the first subdomain as the filename for the basic authentication data. Domains without '.' are ignored.
			// The full domain is used to identify the filter chain via SNI in the EnvoyFilter configuration patch.
			subdomain, _, found := strings.Cut(host, ".")
			if !found {
				continue
			}

			volumes = append(volumes, corev1.Volume{
				Name: subdomain,
				VolumeSource: corev1.VolumeSource{
					Secret: &corev1.SecretVolumeSource{
						SecretName: virtualService.Labels[v1beta1constants.LabelBasicAuthSecretName],
						Items: []corev1.KeyToPath{
							{
								Key:  secretsutils.DataKeyAuth,
								Path: subdomain,
							},
						},
					},
				},
			})

			volumeMounts = append(volumeMounts, corev1.VolumeMount{
				Name:      subdomain,
				MountPath: path.Join(rootMountPath, subdomain),
				SubPath:   subdomain,
			})

			configPatches = append(configPatches, &istioapinetworkingv1alpha3.EnvoyFilter_EnvoyConfigObjectPatch{
				ApplyTo: istioapinetworkingv1alpha3.EnvoyFilter_HTTP_FILTER,
				Match: &istioapinetworkingv1alpha3.EnvoyFilter_EnvoyConfigObjectMatch{
					Context: istioapinetworkingv1alpha3.EnvoyFilter_GATEWAY,
					ObjectTypes: &istioapinetworkingv1alpha3.EnvoyFilter_EnvoyConfigObjectMatch_Listener{
						Listener: &istioapinetworkingv1alpha3.EnvoyFilter_ListenerMatch{
							PortNumber: 9443,
							FilterChain: &istioapinetworkingv1alpha3.EnvoyFilter_ListenerMatch_FilterChainMatch{
								Filter: &istioapinetworkingv1alpha3.EnvoyFilter_ListenerMatch_FilterMatch{
									Name: "envoy.filters.network.http_connection_manager",
								},
								Sni: host,
							},
						},
					},
				},
				Patch: &istioapinetworkingv1alpha3.EnvoyFilter_Patch{
					Operation:   istioapinetworkingv1alpha3.EnvoyFilter_Patch_INSERT_BEFORE,
					FilterClass: istioapinetworkingv1alpha3.EnvoyFilter_Patch_AUTHZ,
					Value: &structpb.Struct{
						Fields: map[string]*structpb.Value{
							"name": structpb.NewStringValue("envoy.filters.http.ext_authz"),
							"typed_config": structpb.NewStructValue(&structpb.Struct{
								Fields: map[string]*structpb.Value{
									"@type":                 structpb.NewStringValue("type.googleapis.com/envoy.extensions.filters.http.ext_authz.v3.ExtAuthz"),
									"transport_api_version": structpb.NewStringValue("V3"),
									"grpc_service": structpb.NewStructValue(&structpb.Struct{
										Fields: map[string]*structpb.Value{
											"timeout": structpb.NewStringValue("2s"),
											"envoy_grpc": structpb.NewStructValue(&structpb.Struct{
												Fields: map[string]*structpb.Value{
													"cluster_name": structpb.NewStringValue(fmt.Sprintf("outbound|%d||%s%s.%s.svc.cluster.local", Port, e.getPrefix(), svcName, e.namespace)),
												},
											}),
										},
									}),
								},
							}),
						},
					},
				},
			},
			)
		}
	}

	return volumes, volumeMounts, configPatches, nil
}

func (e *extAuthzServer) getPrefix() string {
	if e.values.IsGardenCluster {
		return operatorv1alpha1.VirtualGardenNamePrefix
	}

	return ""
}

func (e *extAuthzServer) getLabels() map[string]string {
	return map[string]string{
		v1beta1constants.LabelApp: e.getPrefix() + name,
	}
}

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

	istionetworkingv1beta1 "istio.io/client-go/pkg/apis/networking/v1beta1"
	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	operatorv1alpha1 "github.com/gardener/gardener/pkg/apis/operator/v1alpha1"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	"github.com/gardener/gardener/pkg/component"
	kubernetesutils "github.com/gardener/gardener/pkg/utils/kubernetes"
	"github.com/gardener/gardener/pkg/utils/managedresources"
	secretsutils "github.com/gardener/gardener/pkg/utils/secrets"
	secretsmanager "github.com/gardener/gardener/pkg/utils/secrets/manager"
)

const (
	// Port is the port exposed by the ext-authz-server.
	Port = 10000

	name                = "ext-authz-server"
	managedResourceName = name
	svcName             = name

	rootMountPath = "/secrets"
	tlsMountPath  = "/tls"

	tlsServerCertificateName = "tls-server-certificate"
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

	_, err := e.secretsManager.Generate(ctx,
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

	volumes, volumeMounts, err := e.calculateConfiguration(ctx, serverSecret)
	if err != nil {
		return fmt.Errorf("failed to calculate configuration for ext-authz-server: %w", err)
	}

	serializedResources, err := registry.AddAllAndSerialize(
		e.getDeployment(volumes, volumeMounts),
	)
	if err != nil {
		return fmt.Errorf("failed to serialize resources: %w", err)
	}

	return managedresources.CreateForSeed(ctx, e.client, e.namespace, e.getPrefix()+managedResourceName, false, serializedResources)
}

func (e *extAuthzServer) Destroy(ctx context.Context) error {
	return managedresources.DeleteForSeed(ctx, e.client, e.namespace, e.getPrefix()+managedResourceName)
}

var timeoutWaitForManagedResources = 2 * time.Minute

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

func (e *extAuthzServer) calculateConfiguration(ctx context.Context, tlsSecret *corev1.Secret) ([]corev1.Volume, []corev1.VolumeMount, error) {
	virtualServiceList := &istionetworkingv1beta1.VirtualServiceList{}
	err := e.client.List(ctx, virtualServiceList, client.InNamespace(e.namespace), client.HasLabels{v1beta1constants.LabelBasicAuthSecretName})
	if err != nil {
		return nil, nil, fmt.Errorf("unable to list virtual services: %w", err)
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
	)

	for _, virtualService := range virtualServiceList.Items {
		for _, host := range virtualService.Spec.Hosts {
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
		}
	}

	return volumes, volumeMounts, nil
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

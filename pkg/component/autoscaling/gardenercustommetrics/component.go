// Copyright 2024 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

// Package gardenercustommetrics implements the gardener-custom-metrics seed component (aka GCMx).
// For details, see the gardenerCustomMetrics type.
package gardenercustommetrics

import (
	"context"
	"fmt"
	"time"

	"github.com/Masterminds/semver/v3"
	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	"github.com/gardener/gardener/pkg/component"
	kubernetesutils "github.com/gardener/gardener/pkg/utils/kubernetes"
	"github.com/gardener/gardener/pkg/utils/managedresources"
	secretsutils "github.com/gardener/gardener/pkg/utils/secrets"
	secretsmanager "github.com/gardener/gardener/pkg/utils/secrets/manager"
)

// gardenerCustomMetrics manages an instance of the gardener-custom-metrics component (aka GCMx). The component is
// deployed on a seed, scrapes the metrics from all shoot kube-apiserver pods, and provides custom metrics by
// registering as APIService at the custom metrics extension point of the seed kube-apiserver.
// For information about individual fields, see the New function.
type gardenerCustomMetrics struct {
	namespaceName string
	values        Values

	seedClient     client.Client
	secretsManager secretsmanager.Interface
}

// Values is a set of configuration values for the GardenerCustomMetrics component.
type Values struct {
	// Image is the container image for the GCMx pods.
	Image string
	// KubernetesVersion is the version of the runtime cluster.
	KubernetesVersion *semver.Version
}

// New creates a new gardenerCustomMetrics instance tied to a specific server connection.
//
// namespace is where the server-side artefacts (e.g. pods) will be deployed (usually the 'garden' namespace).
// containerImageName points to the binary for the gardener-custom-metrics pods. The exact version to be used, is
// determined by contextual configuration, e.g. image vector overrides.
// If enabled is true, this instance strives to bring the component to an installed, working state. If enabled is
// false, this instance strives to uninstall the component.
// seedClient represents the connection to the seed API server.
// secretsManager is used to interact with secrets on the seed.
func New(
	namespace string,
	values Values,
	seedClient client.Client,
	secretsManager secretsmanager.Interface) component.DeployWaiter {
	return &gardenerCustomMetrics{
		namespaceName:  namespace,
		values:         values,
		seedClient:     seedClient,
		secretsManager: secretsManager,
	}
}

// Deploy implements [component.Deployer.Deploy]()
func (gcmx *gardenerCustomMetrics) Deploy(ctx context.Context) error {
	baseErrorMessage :=
		fmt.Sprintf(
			"An error occurred while deploying gardener-custom-metrics component in namespace '%s' of the seed server",
			gcmx.namespaceName)

	serverCertificateSecret, err := gcmx.deployServerCertificate(ctx)
	if err != nil {
		return fmt.Errorf(baseErrorMessage+
			" - failed to deploy the gardener-custom-metrics server TLS certificate to the seed server. "+
			"The error message reported by the underlying operation follows: %w",
			err)
	}

	registry := managedresources.NewRegistry(kubernetes.ShootScheme, kubernetes.ShootCodec, kubernetes.ShootSerializer)

	resources, err := registry.AddAllAndSerialize(
		gcmx.serviceAccount(),
		gcmx.role(),
		gcmx.roleBinding(),
		gcmx.clusterRole(),
		gcmx.clusterRoleBinding(),
		gcmx.authDelegatorClusterRoleBinding(),
		gcmx.authReaderRoleBinding(),
		gcmx.deployment(serverCertificateSecret.Name),
		gcmx.service(),
		gcmx.apiService(),
		gcmx.pdb(),
		gcmx.vpa(),
	)
	if err != nil {
		return fmt.Errorf(baseErrorMessage+
			" - failed to create the K8s object definitions which describe the individual "+
			"k8s objects comprising the application deployment arrangement. "+
			"The error message reported by the underlying operation follows: %w",
			err)
	}

	err = managedresources.CreateForSeed(
		ctx,
		gcmx.seedClient,
		gcmx.namespaceName,
		managedResourceName,
		false,
		resources)
	if err != nil {
		return fmt.Errorf(baseErrorMessage+
			" - failed to deploy the necessary resource config objects as a ManagedResource named '%s' to the server. "+
			"The error message reported by the underlying operation follows: %w",
			managedResourceName,
			err)
	}

	return nil
}

// Destroy implements [component.Deployer.Destroy]()
func (gcmx *gardenerCustomMetrics) Destroy(ctx context.Context) error {
	if err := managedresources.DeleteForSeed(ctx, gcmx.seedClient, gcmx.namespaceName, managedResourceName); err != nil {
		return fmt.Errorf(
			"An error occurred while removing the gardener-custom-metrics component in namespace '%s' from the seed server"+
				" - failed to remove ManagedResource '%s'. "+
				"The error message reported by the underlying operation follows: %w",
			gcmx.namespaceName,
			managedResourceName,
			err)
	}

	return nil
}

// Wait implements [component.Waiter.Wait]()
func (gcmx *gardenerCustomMetrics) Wait(ctx context.Context) error {
	timeoutCtx, cancel := context.WithTimeout(ctx, managedResourceTimeout)
	defer cancel()

	if err := managedresources.WaitUntilHealthy(timeoutCtx, gcmx.seedClient, gcmx.namespaceName, managedResourceName); err != nil {
		return fmt.Errorf(
			"An error occurred while waiting for the deployment process of the gardener-custom-metrics component to "+
				"'%s' namespace in the seed server to finish and for the component to report ready status"+
				" - the wait for ManagedResource '%s' to become healty failed. "+
				"The error message reported by the underlying operation follows: %w",
			gcmx.namespaceName,
			managedResourceName,
			err)
	}

	return nil
}

// WaitCleanup implements [component.Waiter.WaitCleanup]()
func (gcmx *gardenerCustomMetrics) WaitCleanup(ctx context.Context) error {
	timeoutCtx, cancel := context.WithTimeout(ctx, managedResourceTimeout)
	defer cancel()

	if err := managedresources.WaitUntilDeleted(timeoutCtx, gcmx.seedClient, gcmx.namespaceName, managedResourceName); err != nil {
		return fmt.Errorf(
			"An error occurred while waiting for the gardener-custom-metrics component to be fully removed from the "+
				"'%s' namespace in the seed server"+
				" - the wait for ManagedResource '%s' to be removed failed. "+
				"The error message reported by the underlying operation follows: %w",
			gcmx.namespaceName,
			managedResourceName,
			err)
	}

	return nil
}

const (
	// The implementing artifacts are deployed to the seed via this MR
	managedResourceName = "gardener-custom-metrics"
	// GCMx's HTTPS serving certificate
	serverCertificateSecretName = "gardener-custom-metrics-tls"
	// Timeout for ManagedResources to become healthy or deleted
	managedResourceTimeout = 2 * time.Minute
)

// Deploys the GCMx server TLS certificate to a secret and returns the name of the created secret
func (gcmx *gardenerCustomMetrics) deployServerCertificate(ctx context.Context) (*corev1.Secret, error) {
	const baseErrorMessage = "An error occurred while deploying server TLS certificate for gardener-custom-metrics"

	_, found := gcmx.secretsManager.Get(v1beta1constants.SecretNameCASeed)
	if !found {
		return nil, fmt.Errorf(
			baseErrorMessage+
				" - the CA certificate, which is required to sign said server certificate, is missing. "+
				"The CA certificate was expected in the '%s' secret, but that secret was not found",
			v1beta1constants.SecretNameCASeed)
	}

	serverCertificateSecret, err := gcmx.secretsManager.Generate(
		ctx,
		&secretsutils.CertificateSecretConfig{
			Name:                        serverCertificateSecretName,
			CommonName:                  serviceName,
			DNSNames:                    kubernetesutils.DNSNamesForService(serviceName, gcmx.namespaceName),
			CertType:                    secretsutils.ServerCert,
			SkipPublishingCACertificate: true,
		},
		secretsmanager.SignedByCA(v1beta1constants.SecretNameCASeed, secretsmanager.UseCurrentCA),
		secretsmanager.Rotate(secretsmanager.InPlace))
	if err != nil {
		return nil, fmt.Errorf(
			baseErrorMessage+
				" - the attept to generate the certificate and store it in a secret named '%s' failed. "+
				"The error message reported by the underlying operation follows: %w",
			serverCertificateSecretName,
			err)
	}

	return serverCertificateSecret, nil
}

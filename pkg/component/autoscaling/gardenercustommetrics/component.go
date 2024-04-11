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
// For details, see the GardenerCustomMetrics type.
package gardenercustommetrics

import (
	"context"
	"fmt"
	"time"

	"github.com/Masterminds/semver/v3"
	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	"github.com/gardener/gardener/pkg/component/autoscaling/gardenercustommetrics/kubeobjects"
	kubernetesutils "github.com/gardener/gardener/pkg/utils/kubernetes"
	"github.com/gardener/gardener/pkg/utils/managedresources"
	secretsutils "github.com/gardener/gardener/pkg/utils/secrets"
	secretsmanager "github.com/gardener/gardener/pkg/utils/secrets/manager"
)

// ComponentName is the component name.
const ComponentName = componentBaseName

// GardenerCustomMetrics manages an instance of the gardener-custom-metrics component (aka GCMx). The component is
// deployed on a seed, scrapes the metrics from all shoot kube-apiserver pods, and provides custom metrics by
// registering as APIService at the custom metrics extension point of the seed kube-apiserver.
// For information about individual fields, see the NewGardenerCustomMetrics function.
type GardenerCustomMetrics struct {
	namespaceName      string
	containerImageName string
	kubernetesVersion  *semver.Version

	seedClient     client.Client
	secretsManager secretsmanager.Interface

	testIsolation gardenerCustomMetricsTestIsolation // Provides indirections necessary to isolate the unit during tests
}

// NewGardenerCustomMetrics creates a new GardenerCustomMetrics instance tied to a specific server connection.
//
// namespace is where the server-side artefacts (e.g. pods) will be deployed (usually the 'garden' namespace).
// containerImageName points to the binary for the gardener-custom-metrics pods. The exact version to be used, is
// determined by contextual configuration, e.g. image vector overrides.
// If enabled is true, this instance strives to bring the component to an installed, working state. If enabled is
// false, this instance strives to uninstall the component.
// seedClient represents the connection to the seed API server.
// secretsManager is used to interact with secrets on the seed.
func NewGardenerCustomMetrics(
	namespace string,
	containerImageName string,
	kubernetesVersion *semver.Version,
	seedClient client.Client,
	secretsManager secretsmanager.Interface) *GardenerCustomMetrics {
	return &GardenerCustomMetrics{
		namespaceName:      namespace,
		containerImageName: containerImageName,
		kubernetesVersion:  kubernetesVersion,
		seedClient:         seedClient,
		secretsManager:     secretsManager,

		testIsolation: gardenerCustomMetricsTestIsolation{
			CreateForSeed: managedresources.CreateForSeed,
			DeleteForSeed: managedresources.DeleteForSeed,
		},
	}
}

// Deploy implements [component.Deployer.Deploy]()
func (gcmx *GardenerCustomMetrics) Deploy(ctx context.Context) error {
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

	kubeObjects, err := kubeobjects.GetKubeObjectsAsYamlBytes(
		deploymentName, gcmx.namespaceName, gcmx.containerImageName, serverCertificateSecret.Name, gcmx.kubernetesVersion)
	if err != nil {
		return fmt.Errorf(baseErrorMessage+
			" - failed to create the K8s object definitions which describe the individual "+
			"k8s objects comprising the application deployment arrangement. "+
			"The error message reported by the underlying operation follows: %w",
			err)
	}

	err = gcmx.testIsolation.CreateForSeed(
		ctx,
		gcmx.seedClient,
		gcmx.namespaceName,
		managedResourceName,
		false,
		kubeObjects)
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
func (gcmx *GardenerCustomMetrics) Destroy(ctx context.Context) error {
	if err := gcmx.testIsolation.DeleteForSeed(ctx, gcmx.seedClient, gcmx.namespaceName, managedResourceName); err != nil {
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
func (gcmx *GardenerCustomMetrics) Wait(ctx context.Context) error {
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
func (gcmx *GardenerCustomMetrics) WaitCleanup(ctx context.Context) error {
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
	componentBaseName           = "gardener-custom-metrics"
	deploymentName              = componentBaseName
	managedResourceName         = componentBaseName // The implementing artifacts are deployed to the seed via this MR
	serviceName                 = componentBaseName
	serverCertificateSecretName = componentBaseName + "-tls" // GCMx's HTTPS serving certificate
	managedResourceTimeout      = 2 * time.Minute            // Timeout for ManagedResources to become healthy or deleted
)

// gardenerCustomMetricsTestIsolation contains all points of indirection necessary to isolate GardenerCustomMetrics'
// dependencies on external static functions during test.
type gardenerCustomMetricsTestIsolation struct {
	// Points to [managedresources.CreateForSeed]()
	CreateForSeed func(
		ctx context.Context, client client.Client, namespace, name string, keepObjects bool, data map[string][]byte) error
	// Points to [managedresources.DeleteForSeed]()
	DeleteForSeed func(ctx context.Context, client client.Client, namespace, name string) error
}

// Deploys the GCMx server TLS certificate to a secret and returns the name of the created secret
func (gcmx *GardenerCustomMetrics) deployServerCertificate(ctx context.Context) (*corev1.Secret, error) {
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

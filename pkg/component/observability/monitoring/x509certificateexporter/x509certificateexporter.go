// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package x509certificateexporter

import (
	"context"
	"errors"
	"fmt"
	"time"

	"sigs.k8s.io/controller-runtime/pkg/client"

	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	"github.com/gardener/gardener/pkg/component"
	operatorclient "github.com/gardener/gardener/pkg/operator/client"
	"github.com/gardener/gardener/pkg/utils/managedresources"
	secretsmanager "github.com/gardener/gardener/pkg/utils/secrets/manager"
)

const (
	containerName       = "x509-certificate-exporter"
	managedResourceName = "x509-certificate-exporter"
	// promRuleName is the suffix for the PrometheusRule resource name, prefix is target
	promRuleName                 = "-x509-certificate-exporter"
	inClusterManagedResourceName = "x509-certificate-exporter"
	nodeManagedResourceName      = "x509-certificate-exporter-node"
	clusterRoleName              = "gardener-cloud:x509-certificate-exporter"
	clusterRoleBindingName       = "gardener-cloud:x509-certificate-exporter"
	// port on which the x509-certificate-exporter exposes metrics
	port = 9793
	// portName is the name of the port on which the x509-certificate-exporter exposes metrics and is scraped on
	portName      = "metrics"
	SuffixSeed    = "-seed"
	SuffixRuntime = "-runtime"
	SuffixShoot   = "-shoot"
	// labelComponent is the component label value for the `role` label
	labelComponent = "x509-certificate-exporter"
	// defaultCertificateRenewalDays is the default number of days before expiration that will trigger a warning alert
	defaultCertificateRenewalDays = 14
	// defaultCertificateExpirationDays is the default number of days before expiration that will trigger a critical alert
	defaultCertificateExpirationDays = 7
	// defaultReplicas is the default number of replicas for the x509-certificate-exporter deployment
	defaultReplicas uint32 = 1
	// defaultCertCacheDuration is the default duration for which certificates are cached
	defaultCertCacheDuration = 24 * time.Hour
	// defaultKubeApiBurst is the default burst for the kube api client
	defaultKubeApiBurst uint32 = 30
	// defaultKubeApiRateLimit is the default rate limit for the kube api client
	defaultKubeApiRateLimit uint32 = 20
)

func New(
	client client.Client,
	secretsManager secretsmanager.Interface,
	namespace string,
	values Values,
) component.DeployWaiter {
	// if values.CertificateRenewalDays == 0 {
	// 	values.CertificateRenewalDays = defaultCertificateRenewalDays
	// }
	// if values.CertificateExpirationDays == 0 {
	// 	values.CertificateExpirationDays = defaultCertificateExpirationDays
	// }
	// if values.Replicas == 0 {
	// 	values.Replicas = defaultReplicas
	// }
	// if values.CacheDuration.Duration == 0 {
	// 	values.CacheDuration.Duration = defaultCertCacheDuration
	// }
	return &x509CertificateExporter{
		client:         client,
		secretsManager: secretsManager,
		namespace:      namespace,
		values:         values,
	}
}

func (x *x509CertificateExporter) Deploy(ctx context.Context) error {
	if x.values.NameSuffix != SuffixRuntime {
		return errors.New("x509CertificateExporter is currently supported only on the runtime cluster")
	}

	var (
		res []client.Object
		// hostResources       []client.Object
		registry            *managedresources.Registry
		serializedResources map[string][]byte
		err                 error
	)

	if res, err = x.getInClusterCertificateMonitoringResources(); err != nil {
		return fmt.Errorf("failed to get in-cluster certificate monitoring resources: %w", err)
	}
	// res = append(res, x.prometheusRule(x.getGenericLabels("any"), x.values.CertificateRenewalDays, x.values.CertificateExpirationDays))

	// if x.values.WorkerGroups != nil {
	// 	if hostResources, err = x.getHostCertificateMonitoringResources(); err != nil {
	// 		return fmt.Errorf("failed to get host certificate monitoring resources: %w", err)
	// 	}
	// 	res = append(res, hostResources...)
	// }
	if x.values.NameSuffix == SuffixSeed {
		registry = managedresources.NewRegistry(kubernetes.GardenScheme, kubernetes.GardenCodec, kubernetes.GardenSerializer)
	}

	if x.values.NameSuffix == SuffixRuntime {
		registry = managedresources.NewRegistry(operatorclient.RuntimeScheme, operatorclient.RuntimeCodec, operatorclient.RuntimeSerializer)
	}

	if err := registry.Add(res...); err != nil {
		return err
	}

	serializedResources, err = registry.SerializedObjects()
	if err != nil {
		return err
	}

	if err := managedresources.CreateForSeedWithLabels(
		ctx,
		x.client,
		x.namespace,
		managedResourceName+x.values.NameSuffix,
		false,
		map[string]string{v1beta1constants.LabelCareConditionType: v1beta1constants.ObservabilityComponentsHealthy},
		serializedResources,
	); err != nil {
		return err
	}

	return nil
}

func (x *x509CertificateExporter) Destroy(ctx context.Context) error {
	if err := managedresources.DeleteForSeed(ctx, x.client, x.namespace, managedResourceName); err != nil {
		return err
	}
	return nil
}

// TimeoutWaitForManagedResource is the timeout used while waiting for the ManagedResources to become healthy
// or deleted.
var TimeoutWaitForManagedResource = 2 * time.Minute

func (x *x509CertificateExporter) Wait(ctx context.Context) error {
	timeoutCtx, cancel := context.WithTimeout(ctx, TimeoutWaitForManagedResource)
	defer cancel()

	return managedresources.WaitUntilHealthy(timeoutCtx, x.client, x.namespace, managedResourceName)
}

func (x *x509CertificateExporter) WaitCleanup(ctx context.Context) error {
	timeoutCtx, cancel := context.WithTimeout(ctx, TimeoutWaitForManagedResource)
	defer cancel()

	return managedresources.WaitUntilDeleted(timeoutCtx, x.client, x.namespace, managedResourceName)

}

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

func New(
	client client.Client,
	secretsManager secretsmanager.Interface,
	namespace string,
	values Values,
) (component.DeployWaiter, error) {

	var conf x509certificateExporterConfig

	if err := parseConfig(values.ConfigData, &conf); err != nil {
		return nil, fmt.Errorf("failed to parse x509CertificateExporter config: %w", err)
	}

	return &x509CertificateExporter{
		client:         client,
		secretsManager: secretsManager,
		namespace:      namespace,
		values:         values,
		conf:           conf,
	}, nil
}

func (x *x509CertificateExporter) Deploy(ctx context.Context) error {
	if x.values.NameSuffix != SuffixRuntime {
		return errors.New("x509CertificateExporter is currently supported only on the runtime cluster")
	}

	var (
		res                 []client.Object
		hostResources       []client.Object
		registry            *managedresources.Registry
		serializedResources map[string][]byte
		err                 error
	)
	if x.conf.IsInclusterEnabled() {
		res = x.getInClusterCertificateMonitoringResources()
	}
	res = append(res, x.prometheusRule(x.getGenericLabels("any")))

	if x.conf.IsWorkerGroupsEnabled() {
		if hostResources, err = x.getHostCertificateMonitoringResources(); err != nil {
			return fmt.Errorf("failed to get host certificate monitoring resources: %w", err)
		}
		res = append(res, hostResources...)
	}
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

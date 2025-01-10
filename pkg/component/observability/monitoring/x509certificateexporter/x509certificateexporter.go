// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package x509certificateexporter

import (
	"context"
	"errors"
	"fmt"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	operatorv1alpha1 "github.com/gardener/gardener/pkg/apis/operator/v1alpha1"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	"github.com/gardener/gardener/pkg/component"
	operatorclient "github.com/gardener/gardener/pkg/operator/client"
	"github.com/gardener/gardener/pkg/utils/managedresources"
	secretsmanager "github.com/gardener/gardener/pkg/utils/secrets/manager"
)

const (
	containerName                    = "x509-certificate-exporter"
	managedResourceName              = "x509-certificate-exporter"
	promRuleName                     = "-x509-certificate-exporter"
	inClusterManagedResourceName     = "x509-certificate-exporter"
	nodeManagedResourceName          = "x509-certificate-exporter-node"
	clusterRoleName                  = "gardener-cloud:x509-certificate-exporter"
	clusterRoleBindingName           = "gardener-cloud:x509-certificate-exporter"
	port                             = 9793
	portName                         = "metrics"
	certificateSourceLabelName       = "certificate-source"
	inClusterCertificateLabelValue   = "api"
	nodeCertificateLabelValue        = "node"
	SuffixSeed                       = "-seed"
	SuffixRuntime                    = "-runtime"
	SuffixShoot                      = "-shoot"
	labelComponent                   = "x509-certificate-exporter"
	defaultCertificateRenewalDays   = 14
	defaultCertificateExpirationDays = 7
	defaultReplicas                  = 1
	defaultCertCacheDuration         = 24 * time.Hour
)

type x509CertificateExporter struct {
	client         client.Client
	secretsManager secretsmanager.Interface
	namespace      string
	values         Values
}

// Configurations for the x509 certificate exporter
type Values struct {
	// SecretTypes that should be watched by the exporter.
	SecretTypes SecretTypeList
	// ConfigMapKeys that should be watched by the exporter.
	ConfigMapKeys ConfigMapKeys
	// CacheDuration sets cache lifespan, usually cache is
	// regenerated a bit more than half that value.
	CacheDuration metav1.Duration
	// Image sets container image.
	Image string
	// PriorityClassName is the name of the priority class.
	PriorityClassName string
	// Replicas sets the number of replicas.
	Replicas int32
	// NameSuffix is attached to the deployment name and related resources.
	NameSuffix string
	// IncludeNamespaces are namespaces from which secrets are monitored.
	// If non-zero length excludes all else.
	IncludeNamespaces IncludeNamespaces
	// ExcludeNamespaces namespaces from which secrets are not monitored.
	// If non-zero length includes all else.
	ExcludeNamespaces ExcludeNamespaces
	// IncludeLabels includes labels, similar to the namespaces vars.
	IncludeLabels IncludeLabels
	// ExcludeLabels exludes labels, similar to the namespaces vars.
	ExcludeLabels ExcludeLabels
	// WorkerGroups that should be monitored from nodes
	WorkerGroups map[string]operatorv1alpha1.WorkerGroup
	// CertificateExpirationDays is the number of days before expiration that will trigger a critical alert
	CertificateExpirationDays uint
	// CertificateRenewalDays is the number of days before expiration that will trigger a warning alert
	CertificateRenewalDays uint
	// PrometheusInstance is the label for the prometheus instance
	PrometheusInstance string
}

func New(
	client client.Client,
	secretsManager secretsmanager.Interface,
	namespace string,
	values Values,
) component.DeployWaiter {
	if values.CertificateRenewalDays == 0 {
		values.CertificateRenewalDays = defaultCertificateRenewalDays
	}
	if values.CertificateExpirationDays == 0 {
		values.CertificateExpirationDays = defaultCertificateExpirationDays
	}
	if values.Replicas == 0 {
		values.Replicas = defaultReplicas
	}
	if values.CacheDuration.Duration == 0 {
		values.CacheDuration.Duration = defaultCertCacheDuration
	}
	return &x509CertificateExporter{
		client:         client,
		secretsManager: secretsManager,
		namespace:      namespace,
		values:         values,
	}
}

func (x *x509CertificateExporter) Deploy(ctx context.Context) error {
	if x.values.NameSuffix != SuffixRuntime && x.values.NameSuffix != SuffixSeed {
		return errors.New("x509CertificateExporter is currently supported only on the seed and runtime clusters")
	}

	if x.values.WorkerGroups == nil && len(x.values.SecretTypes) == 0 && len(x.values.ConfigMapKeys) == 0 {
		return fmt.Errorf("no secret types, configmap keys and worker groups provided, nothing to monitor")
	}
	var (
		res                 []client.Object
		hostResources       []client.Object
		registry            *managedresources.Registry
		serializedResources map[string][]byte
		err                 error
	)

	if res, err = x.getInClusterCertificateMonitoringResources(); err != nil {
		return fmt.Errorf("failed to get in-cluster certificate monitoring resources: %w", err)
	}
	res = append(res, x.prometheusRule(x.getGenericLabels("any"), x.values.CertificateRenewalDays, x.values.CertificateExpirationDays))

	if x.values.WorkerGroups != nil {
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

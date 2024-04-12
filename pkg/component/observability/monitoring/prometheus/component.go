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

package prometheus

import (
	"context"
	"strings"
	"time"

	"github.com/Masterminds/semver/v3"
	"github.com/go-logr/logr"
	monitoringv1 "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1"
	monitoringv1alpha1 "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	utilerrors "k8s.io/apimachinery/pkg/util/errors"
	"sigs.k8s.io/controller-runtime/pkg/client"

	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	"github.com/gardener/gardener/pkg/component"
	"github.com/gardener/gardener/pkg/component/observability/monitoring"
	monitoringutils "github.com/gardener/gardener/pkg/component/observability/monitoring/utils"
	"github.com/gardener/gardener/pkg/utils"
	"github.com/gardener/gardener/pkg/utils/managedresources"
	secretsmanager "github.com/gardener/gardener/pkg/utils/secrets/manager"
)

const (
	dataKeyAdditionalScrapeConfigs       = "prometheus.yaml"
	dataKeyAdditionalAlertRelabelConfigs = "configs.yaml"
	port                                 = 9090
	servicePort                          = 80
	// ServicePortName is the name of the port in the Service specification.
	ServicePortName = "web"
)

// Interface contains functions for a Prometheus deployer.
type Interface interface {
	component.DeployWaiter
	// SetIngressAuthSecret sets the ingress authentication secret name.
	SetIngressAuthSecret(*corev1.Secret)
	// SetIngressWildcardCertSecret sets the ingress wildcard certificate secret name.
	SetIngressWildcardCertSecret(*corev1.Secret)
	// SetCentralScrapeConfigs sets the central scrape configs.
	SetCentralScrapeConfigs([]*monitoringv1alpha1.ScrapeConfig)
}

// Values contains configuration values for the prometheus resources.
type Values struct {
	// Name is the name of the prometheus. It will be used for the resource names of Prometheus and ManagedResource.
	Name string
	// Image defines the container image of prometheus.
	Image string
	// Version is the version of prometheus.
	Version string
	// PriorityClassName is the name of the priority class for the deployment.
	PriorityClassName string
	// StorageCapacity is the storage capacity of Prometheus.
	StorageCapacity resource.Quantity
	// Replicas is the number of replicas.
	Replicas int32
	// Retention is the duration for the data retention.
	Retention *monitoringv1.Duration
	// RetentionSize is the size for the data retention.
	RetentionSize monitoringv1.ByteSize
	// RuntimeVersion is the Kubernetes version of the runtime cluster.
	RuntimeVersion *semver.Version
	// ScrapeTimeout is the timeout duration when scraping targets.
	ScrapeTimeout monitoringv1.Duration
	// VPAMinAllowed defines the resource list for the minAllowed field for the prometheus container resource policy.
	VPAMinAllowed *corev1.ResourceList
	// VPAMaxAllowed defines the resource list for the maxAllowed field for the prometheus container resource policy.
	VPAMaxAllowed *corev1.ResourceList
	// ExternalLabels is the set of external labels for the Prometheus configuration.
	ExternalLabels map[string]string
	// AdditionalPodLabels is a map containing additional labels for the created pods.
	AdditionalPodLabels map[string]string
	// CentralConfigs contains configuration for this Prometheus instance that is created together with it. This should
	// only contain configuration that cannot be directly assigned to another component package.
	CentralConfigs CentralConfigs
	// IngressValues contains configuration for exposing this Prometheus instance via an Ingress resource.
	Ingress *IngressValues
	// Alerting contains alerting configuration for this Prometheus instance.
	Alerting *AlertingValues
	// AdditionalResources contains any additional resources which get added to the ManagedResource.
	AdditionalResources []client.Object

	// DataMigration is a struct for migrating data from existing disks.
	// TODO(rfranzke): Remove this as soon as the PV migration code is removed.
	DataMigration monitoring.DataMigration
}

// CentralConfigs contains configuration for this Prometheus instance that is created together with it. This should
// only contain configuration that cannot be directly assigned to another component package.
type CentralConfigs struct {
	// AdditionalScrapeConfigs are additional scrape configs which cannot be modelled with the CRDs of the Prometheus
	// operator.
	AdditionalScrapeConfigs []string
	// PrometheusRules is a list of central PrometheusRule objects for this prometheus instance.
	PrometheusRules []*monitoringv1.PrometheusRule
	// ScrapeConfigs is a list of central ScrapeConfig objects for this prometheus instance.
	ScrapeConfigs []*monitoringv1alpha1.ScrapeConfig
	// ServiceMonitors is a list of central ServiceMonitor objects for this prometheus instance.
	ServiceMonitors []*monitoringv1.ServiceMonitor
	// PodMonitors is a list of central PodMonitor objects for this prometheus instance.
	PodMonitors []*monitoringv1.PodMonitor
}

// AlertingValues contains alerting configuration for this Prometheus instance.
type AlertingValues struct {
	// AlertmanagerName is the name of the alertmanager to which alerts should be sent.
	AlertmanagerName string
}

// IngressValues contains configuration for exposing this Prometheus instance via an Ingress resource.
type IngressValues struct {
	// AuthSecretName is the name of the auth secret.
	AuthSecretName string
	// Host is the hostname under which the Prometheus instance should be exposed.
	Host string
	// SecretsManager is the secrets manager used for generating the TLS certificate if no wildcard certificate is
	// provided.
	SecretsManager secretsmanager.Interface
	// SigningCA is the name of the CA that should be used the sign a self-signed server certificate. Only needed when
	// no wildcard certificate secret is provided.
	SigningCA string
	// WildcardCertSecretName is name of a secret containing the wildcard TLS certificate which is issued for the
	// ingress domain. If not provided, a self-signed server certificate will be created.
	WildcardCertSecretName *string
}

// New creates a new instance of DeployWaiter for the prometheus.
func New(log logr.Logger, client client.Client, namespace string, values Values) Interface {
	return &prometheus{
		log:       log,
		client:    client,
		namespace: namespace,
		values:    values,
	}
}

type prometheus struct {
	log       logr.Logger
	client    client.Client
	namespace string
	values    Values
}

func (p *prometheus) Deploy(ctx context.Context) error {
	var (
		log      = p.log.WithName("prometheus-deployer").WithValues("name", p.values.Name)
		registry = managedresources.NewRegistry(kubernetes.SeedScheme, kubernetes.SeedCodec, kubernetes.SeedSerializer)
	)

	// TODO(rfranzke): Remove this migration code after all Prometheis have been migrated.
	takeOverExistingPV, pvs, oldPVCs, err := p.values.DataMigration.ExistingPVTakeOverPrerequisites(ctx, log)
	if err != nil {
		return err
	}

	if err := p.addCentralConfigsToRegistry(registry); err != nil {
		return err
	}

	if err := registry.Add(p.values.AdditionalResources...); err != nil {
		return err
	}

	ingress, err := p.ingress(ctx)
	if err != nil {
		return err
	}

	if p.values.Alerting != nil {
		if err := registry.Add(p.secretAdditionalAlertRelabelConfigs()); err != nil {
			return err
		}
	}

	resources, err := registry.AddAllAndSerialize(
		p.serviceAccount(),
		p.service(),
		p.clusterRoleBinding(),
		p.secretAdditionalScrapeConfigs(),
		p.prometheus(takeOverExistingPV),
		p.vpa(),
		p.podDisruptionBudget(),
		ingress,
	)
	if err != nil {
		return err
	}

	if takeOverExistingPV {
		if err := p.values.DataMigration.PrepareExistingPVTakeOver(ctx, log, pvs, oldPVCs); err != nil {
			return err
		}

		log.Info("Deploy new Prometheus (with init container for renaming the data directory)")
	}

	if err := managedresources.CreateForSeedWithLabels(ctx, p.client, p.namespace, p.name(), false, map[string]string{v1beta1constants.LabelCareConditionType: v1beta1constants.ObservabilityComponentsHealthy}, resources); err != nil {
		return err
	}

	if takeOverExistingPV {
		if err := p.values.DataMigration.FinalizeExistingPVTakeOver(ctx, log, pvs); err != nil {
			return err
		}

		log.Info("Deploy new Prometheus again (to remove the migration init container)")
		return p.Deploy(ctx)
	}

	return nil
}

func (p *prometheus) Destroy(ctx context.Context) error {
	return managedresources.DeleteForSeed(ctx, p.client, p.namespace, p.name())
}

// TimeoutWaitForManagedResource is the timeout used while waiting for the ManagedResources to become healthy or
// deleted.
var TimeoutWaitForManagedResource = 5 * time.Minute

func (p *prometheus) Wait(ctx context.Context) error {
	timeoutCtx, cancel := context.WithTimeout(ctx, TimeoutWaitForManagedResource)
	defer cancel()

	return managedresources.WaitUntilHealthy(timeoutCtx, p.client, p.namespace, p.name())
}

func (p *prometheus) WaitCleanup(ctx context.Context) error {
	timeoutCtx, cancel := context.WithTimeout(ctx, TimeoutWaitForManagedResource)
	defer cancel()

	return managedresources.WaitUntilDeleted(timeoutCtx, p.client, p.namespace, p.name())
}

func (p *prometheus) SetIngressAuthSecret(secret *corev1.Secret) {
	if p.values.Ingress != nil && secret != nil {
		p.values.Ingress.AuthSecretName = secret.Name
	}
}

func (p *prometheus) SetIngressWildcardCertSecret(secret *corev1.Secret) {
	if p.values.Ingress != nil && secret != nil {
		p.values.Ingress.WildcardCertSecretName = &secret.Name
	}
}

func (p *prometheus) SetCentralScrapeConfigs(configs []*monitoringv1alpha1.ScrapeConfig) {
	p.values.CentralConfigs.ScrapeConfigs = configs
}

func (p *prometheus) name() string {
	return "prometheus-" + p.values.Name
}

func (p *prometheus) addCentralConfigsToRegistry(registry *managedresources.Registry) error {
	var errs []error

	add := func(obj client.Object) {
		if !strings.HasPrefix(obj.GetName(), p.values.Name+"-") {
			obj.SetName(p.values.Name + "-" + obj.GetName())
		}

		if obj.GetNamespace() == "" {
			obj.SetNamespace(p.namespace)
		}

		obj.SetLabels(utils.MergeStringMaps(obj.GetLabels(), monitoringutils.Labels(p.values.Name)))

		if err := registry.Add(obj); err != nil {
			errs = append(errs, err)
		}
	}

	for _, obj := range p.values.CentralConfigs.PrometheusRules {
		add(obj)
	}
	for _, obj := range p.values.CentralConfigs.ScrapeConfigs {
		add(obj)
	}
	for _, obj := range p.values.CentralConfigs.ServiceMonitors {
		add(obj)
	}
	for _, obj := range p.values.CentralConfigs.PodMonitors {
		add(obj)
	}

	return utilerrors.NewAggregate(errs)
}

func (p *prometheus) getLabels() map[string]string {
	return map[string]string{
		v1beta1constants.LabelApp:  "prometheus",
		v1beta1constants.LabelRole: v1beta1constants.LabelMonitoring,
		"name":                     p.values.Name,
	}
}

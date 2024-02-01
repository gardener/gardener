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

	monitoringv1 "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	utilerrors "k8s.io/apimachinery/pkg/util/errors"
	"sigs.k8s.io/controller-runtime/pkg/client"

	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	"github.com/gardener/gardener/pkg/component"
	monitoringutils "github.com/gardener/gardener/pkg/component/monitoring/utils"
	"github.com/gardener/gardener/pkg/utils"
	"github.com/gardener/gardener/pkg/utils/managedresources"
)

const (
	port = 9090
)

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
	// CentralConfigs contains configuration for this Prometheus instance that is created together with it. This should
	// only contain configuration that cannot be directly assigned to another component package.
	CentralConfigs CentralConfigs
}

// CentralConfigs contains configuration for this Prometheus instance that is created together with it. This should
// only contain configuration that cannot be directly assigned to another component package.
type CentralConfigs struct {
	// PrometheusRules is a list of central PrometheusRule objects for this prometheus instance.
	PrometheusRules []*monitoringv1.PrometheusRule
}

// New creates a new instance of DeployWaiter for the prometheus.
func New(client client.Client, namespace string, values Values) component.DeployWaiter {
	return &prometheus{
		client:    client,
		namespace: namespace,
		values:    values,
	}
}

type prometheus struct {
	client    client.Client
	namespace string
	values    Values
}

func (p *prometheus) Deploy(ctx context.Context) error {
	registry := managedresources.NewRegistry(kubernetes.SeedScheme, kubernetes.SeedCodec, kubernetes.SeedSerializer)

	if err := p.addCentralConfigsToRegistry(registry); err != nil {
		return err
	}

	resources, err := registry.AddAllAndSerialize(
		p.serviceAccount(),
		p.service(),
		p.clusterRoleBinding(),
		p.prometheus(),
		p.vpa(),
	)
	if err != nil {
		return err
	}

	return managedresources.CreateForSeed(ctx, p.client, p.namespace, p.name(), false, resources)
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

	return utilerrors.NewAggregate(errs)
}

func (p *prometheus) getLabels() map[string]string {
	return map[string]string{
		v1beta1constants.LabelApp:  "prometheus",
		v1beta1constants.LabelRole: v1beta1constants.LabelMonitoring,
		"name":                     p.values.Name,
	}
}

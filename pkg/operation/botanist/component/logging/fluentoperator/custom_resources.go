// Copyright (c) 2023 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

package fluentoperator

import (
	"context"

	fluentbitv1alpha2 "github.com/fluent/fluent-operator/v2/apis/fluentbit/v1alpha2"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	"github.com/gardener/gardener/pkg/operation/botanist/component"
	"github.com/gardener/gardener/pkg/operation/botanist/component/logging/fluentoperator/customresources"
	kubernetesutils "github.com/gardener/gardener/pkg/utils/kubernetes"
	"github.com/gardener/gardener/pkg/utils/managedresources"
)

const (
	// CustomResourcesManagedResourceName is the name of the managed resource which deploys the custom resources of the operator.
	CustomResourcesManagedResourceName = OperatorManagedResourceName + "-custom-resources"
)

// CustomResourcesValues are the values for the custom resources.
type CustomResourcesValues struct {
	// FluentBitImage is the fluent-bit image.
	FluentBitImage string
	// FluentBitInitImage is the fluent-bit init container image.
	FluentBitInitImage string
	// FluentBitPriorityClass is the name of the priority class of the fluent-bit.
	FluentBitPriorityClass string
}

type customResources struct {
	client            client.Client
	namespace         string
	values            CustomResourcesValues
	additionalInputs  []*fluentbitv1alpha2.ClusterInput
	additionalFilters []*fluentbitv1alpha2.ClusterFilter
	additionalParsers []*fluentbitv1alpha2.ClusterParser
}

// NewCustomResources creates a new instance of Fluent Operator Custom Resources.
func NewCustomResources(
	client client.Client,
	namespace string,
	values CustomResourcesValues,
	additionalInputs []*fluentbitv1alpha2.ClusterInput,
	additionalFilters []*fluentbitv1alpha2.ClusterFilter,
	additionalParsers []*fluentbitv1alpha2.ClusterParser,
) component.DeployWaiter {
	return &customResources{
		client:            client,
		namespace:         namespace,
		values:            values,
		additionalInputs:  additionalInputs,
		additionalFilters: additionalFilters,
		additionalParsers: additionalParsers,
	}
}

func (c *customResources) Deploy(ctx context.Context) error {
	var (
		registry = managedresources.NewRegistry(kubernetes.SeedScheme, kubernetes.SeedCodec, kubernetes.SeedSerializer)

		configMap = &corev1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{
				Name:      v1beta1constants.DaemonSetNameFluentBit + "-lua-config",
				Namespace: c.namespace,
			},
			Data: map[string]string{
				"modify_severity.lua": `
function cb_modify(tag, timestamp, record)
  local unified_severity = cb_modify_unify_severity(record)

  if not unified_severity then
    return 0, 0, 0
  end

  return 1, timestamp, record
end

function cb_modify_unify_severity(record)
  local modified = false
  local severity = record["severity"]
  if severity == nil or severity == "" then
	return modified
  end

  severity = trim(severity):upper()

  if severity == "I" or severity == "INF" or severity == "INFO" then
    record["severity"] = "INFO"
    modified = true
  elseif severity == "W" or severity == "WRN" or severity == "WARN" or severity == "WARNING" then
    record["severity"] = "WARN"
    modified = true
  elseif severity == "E" or severity == "ERR" or severity == "ERROR" or severity == "EROR" then
    record["severity"] = "ERR"
    modified = true
  elseif severity == "D" or severity == "DBG" or severity == "DEBUG" then
    record["severity"] = "DBG"
    modified = true
  elseif severity == "N" or severity == "NOTICE" then
    record["severity"] = "NOTICE"
    modified = true
  elseif severity == "F" or severity == "FATAL" then
    record["severity"] = "FATAL"
    modified = true
  end

  return modified
end

function trim(s)
  return (s:gsub("^%s*(.-)%s*$", "%1"))
end`,
				"add_tag_to_record.lua": `
function add_tag_to_record(tag, timestamp, record)
  record["tag"] = tag
  return 1, timestamp, record
end`,
			},
		}
	)

	utilruntime.Must(kubernetesutils.MakeUnique(configMap))

	resources := []client.Object{configMap}

	fluentBit := customresources.GetFluentBit(getFluentBitLabels(), v1beta1constants.DaemonSetNameFluentBit, c.namespace, c.values.FluentBitImage, c.values.FluentBitInitImage, c.values.FluentBitPriorityClass)
	resources = append(resources, fluentBit)

	clusterFluentBitConfig := customresources.GetClusterFluentBitConfig(v1beta1constants.DaemonSetNameFluentBit, getCustomResourcesLabels())
	resources = append(resources, clusterFluentBitConfig)

	for _, clusterInput := range customresources.GetClusterInputs(getCustomResourcesLabels()) {
		resources = append(resources, clusterInput)
	}

	for _, clusterFilter := range customresources.GetClusterFilters(configMap.Name, getCustomResourcesLabels()) {
		resources = append(resources, clusterFilter)
	}

	for _, clusterParser := range customresources.GetClusterParsers(getCustomResourcesLabels()) {
		resources = append(resources, clusterParser)
	}

	for _, clusterOutput := range customresources.GetClusterOutputs(getCustomResourcesLabels()) {
		resources = append(resources, clusterOutput)
	}

	for _, clusterInput := range c.additionalInputs {
		resources = append(resources, clusterInput)
	}

	for _, clusterFilter := range c.additionalFilters {
		resources = append(resources, clusterFilter)
	}

	for _, clusterParser := range c.additionalParsers {
		resources = append(resources, clusterParser)
	}

	serializedResources, err := registry.AddAllAndSerialize(resources...)
	if err != nil {
		return err
	}

	return managedresources.CreateForSeed(ctx, c.client, c.namespace, CustomResourcesManagedResourceName, false, serializedResources)
}

func (c *customResources) Destroy(ctx context.Context) error {
	return managedresources.DeleteForSeed(ctx, c.client, c.namespace, CustomResourcesManagedResourceName)
}

func (c *customResources) Wait(ctx context.Context) error {
	timeoutCtx, cancel := context.WithTimeout(ctx, timeoutWaitForManagedResources)
	defer cancel()

	return managedresources.WaitUntilHealthy(timeoutCtx, c.client, c.namespace, CustomResourcesManagedResourceName)
}

func (c *customResources) WaitCleanup(ctx context.Context) error {
	timeoutCtx, cancel := context.WithTimeout(ctx, timeoutWaitForManagedResources)
	defer cancel()

	return managedresources.WaitUntilDeleted(timeoutCtx, c.client, c.namespace, CustomResourcesManagedResourceName)
}

func getCustomResourcesLabels() map[string]string {
	return map[string]string{
		v1beta1constants.LabelKeyCustomLoggingResource: v1beta1constants.LabelValueCustomLoggingResource,
	}
}

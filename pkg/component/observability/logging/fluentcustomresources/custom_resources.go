// Copyright 2023 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

package fluentcustomresources

import (
	"context"
	"time"

	fluentbitv1alpha2 "github.com/fluent/fluent-operator/v2/apis/fluentbit/v1alpha2"
	"sigs.k8s.io/controller-runtime/pkg/client"

	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	"github.com/gardener/gardener/pkg/component"
	"github.com/gardener/gardener/pkg/utils/managedresources"
)

const (
	managedResourceName = "fluent-operator-custom-resources"
)

// Values are the values for the custom resources.
type Values struct {
	Suffix  string
	Inputs  []*fluentbitv1alpha2.ClusterInput
	Filters []*fluentbitv1alpha2.ClusterFilter
	Parsers []*fluentbitv1alpha2.ClusterParser
	Outputs []*fluentbitv1alpha2.ClusterOutput
}

type customResources struct {
	client    client.Client
	namespace string
	values    Values
}

// New creates a new instance of Fluent Operator Custom Resources.
func New(
	client client.Client,
	namespace string,
	values Values,
) component.DeployWaiter {
	return &customResources{
		client:    client,
		namespace: namespace,
		values:    values,
	}
}

func (c *customResources) Deploy(ctx context.Context) error {
	var (
		registry  = managedresources.NewRegistry(kubernetes.SeedScheme, kubernetes.SeedCodec, kubernetes.SeedSerializer)
		resources []client.Object
	)

	for _, clusterInput := range c.values.Inputs {
		resources = append(resources, clusterInput)
	}

	for _, clusterFilter := range c.values.Filters {
		resources = append(resources, clusterFilter)
	}

	for _, clusterParser := range c.values.Parsers {
		resources = append(resources, clusterParser)
	}

	for _, clusterOutput := range c.values.Outputs {
		resources = append(resources, clusterOutput)
	}

	serializedResources, err := registry.AddAllAndSerialize(resources...)
	if err != nil {
		return err
	}

	return managedresources.CreateForSeedWithLabels(ctx, c.client, c.namespace, c.getManagedResourceName(), false, map[string]string{v1beta1constants.LabelCareConditionType: v1beta1constants.ObservabilityComponentsHealthy}, serializedResources)
}

func (c *customResources) Destroy(ctx context.Context) error {
	return managedresources.DeleteForSeed(ctx, c.client, c.namespace, c.getManagedResourceName())
}

var timeoutWaitForManagedResources = 2 * time.Minute

func (c *customResources) Wait(ctx context.Context) error {
	timeoutCtx, cancel := context.WithTimeout(ctx, timeoutWaitForManagedResources)
	defer cancel()

	return managedresources.WaitUntilHealthy(timeoutCtx, c.client, c.namespace, c.getManagedResourceName())
}

func (c *customResources) WaitCleanup(ctx context.Context) error {
	timeoutCtx, cancel := context.WithTimeout(ctx, timeoutWaitForManagedResources)
	defer cancel()

	return managedresources.WaitUntilDeleted(timeoutCtx, c.client, c.namespace, c.getManagedResourceName())
}

func (c *customResources) getManagedResourceName() string {
	return managedResourceName + c.values.Suffix
}

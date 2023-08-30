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

package fluentoperator

import (
	"context"

	fluentbitv1alpha2 "github.com/fluent/fluent-operator/v2/apis/fluentbit/v1alpha2"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	resourcesv1alpha1 "github.com/gardener/gardener/pkg/apis/resources/v1alpha1"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	"github.com/gardener/gardener/pkg/component"
	"github.com/gardener/gardener/pkg/utils/managedresources"
)

const (
	// CustomResourcesManagedResourceName is the name of the managed resource which deploys the custom resources of the operator.
	CustomResourcesManagedResourceName = OperatorManagedResourceName + "-custom-resources"
)

// CustomResourcesValues are the values for the custom resources.
type CustomResourcesValues struct {
	Suffix  string
	Inputs  []*fluentbitv1alpha2.ClusterInput
	Filters []*fluentbitv1alpha2.ClusterFilter
	Parsers []*fluentbitv1alpha2.ClusterParser
	Outputs []*fluentbitv1alpha2.ClusterOutput
}

type customResources struct {
	client    client.Client
	namespace string
	values    CustomResourcesValues
}

// NewCustomResources creates a new instance of Fluent Operator Custom Resources.
func NewCustomResources(
	client client.Client,
	namespace string,
	values CustomResourcesValues,
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

	// TODO(rfranzke): Remove this block after v1.77 has been released.
	{
		resources = append(resources,
			&fluentbitv1alpha2.ClusterFluentBitConfig{ObjectMeta: metav1.ObjectMeta{Name: "fluent-bit-config", Annotations: map[string]string{resourcesv1alpha1.Mode: resourcesv1alpha1.ModeIgnore}}},

			&fluentbitv1alpha2.ClusterFilter{ObjectMeta: metav1.ObjectMeta{Name: "01-docker", Annotations: map[string]string{resourcesv1alpha1.Mode: resourcesv1alpha1.ModeIgnore}}},
			&fluentbitv1alpha2.ClusterFilter{ObjectMeta: metav1.ObjectMeta{Name: "02-containerd", Annotations: map[string]string{resourcesv1alpha1.Mode: resourcesv1alpha1.ModeIgnore}}},
			&fluentbitv1alpha2.ClusterFilter{ObjectMeta: metav1.ObjectMeta{Name: "03-add-tag-to-record", Annotations: map[string]string{resourcesv1alpha1.Mode: resourcesv1alpha1.ModeIgnore}}},
			&fluentbitv1alpha2.ClusterFilter{ObjectMeta: metav1.ObjectMeta{Name: "zz-modify-severity", Annotations: map[string]string{resourcesv1alpha1.Mode: resourcesv1alpha1.ModeIgnore}}},

			&fluentbitv1alpha2.ClusterParser{ObjectMeta: metav1.ObjectMeta{Name: "docker-parser", Annotations: map[string]string{resourcesv1alpha1.Mode: resourcesv1alpha1.ModeIgnore}}},
			&fluentbitv1alpha2.ClusterParser{ObjectMeta: metav1.ObjectMeta{Name: "containerd-parser", Annotations: map[string]string{resourcesv1alpha1.Mode: resourcesv1alpha1.ModeIgnore}}},

			&fluentbitv1alpha2.ClusterInput{ObjectMeta: metav1.ObjectMeta{Name: "tail-kubernetes", Annotations: map[string]string{resourcesv1alpha1.Mode: resourcesv1alpha1.ModeIgnore}}},

			&fluentbitv1alpha2.ClusterOutput{ObjectMeta: metav1.ObjectMeta{Name: "journald", Annotations: map[string]string{resourcesv1alpha1.Mode: resourcesv1alpha1.ModeIgnore}}},
		)
	}

	for _, clusterInput := range c.values.Inputs {
		resources = append(resources, clusterInput)
	}

	for _, clusterFilter := range c.values.Filters {
		resources = append(resources, clusterFilter)
	}

	for _, clusterParser := range c.values.Parsers {
		resources = append(resources, clusterParser)
	}

	for _, clusterParser := range c.values.Outputs {
		resources = append(resources, clusterParser)
	}

	serializedResources, err := registry.AddAllAndSerialize(resources...)
	if err != nil {
		return err
	}

	return managedresources.CreateForSeed(ctx, c.client, c.namespace, c.getManagedResourceName(), false, serializedResources)
}

func (c *customResources) Destroy(ctx context.Context) error {
	return managedresources.DeleteForSeed(ctx, c.client, c.namespace, c.getManagedResourceName())
}

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
	return CustomResourcesManagedResourceName + c.values.Suffix
}

func getCustomResourcesLabels() map[string]string {
	return map[string]string{
		v1beta1constants.LabelKeyCustomLoggingResource: v1beta1constants.LabelValueCustomLoggingResource,
	}
}

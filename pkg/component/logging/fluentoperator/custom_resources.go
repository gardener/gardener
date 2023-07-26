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
	apierrors "k8s.io/apimachinery/pkg/api/errors"
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

	if err := c.deleteOldManagedResource(ctx); err != nil {
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
	if len(c.values.Suffix) > 0 {
		return CustomResourcesManagedResourceName + "-" + c.values.Suffix
	}
	return CustomResourcesManagedResourceName
}

// TODO: remove this in next release.
func (c *customResources) deleteOldManagedResource(ctx context.Context) error {
	mr := &resourcesv1alpha1.ManagedResource{}

	if err := c.client.Get(ctx, client.ObjectKey{Namespace: c.namespace, Name: CustomResourcesManagedResourceName}, mr); err != nil {
		if apierrors.IsNotFound(err) {
			return nil
		}
		return err
	}

	// Check if the ManagedResource is responsible for FluentBit.
	// If not nothing has to be done.
	var foundFluentBit bool
	for _, resource := range mr.Status.Resources {
		if resource.Kind == "FluentBit" {
			foundFluentBit = true
		}
	}

	if !foundFluentBit {
		return nil
	}

	// Remove the finalizers from the managed resource to delete it
	beforePatch := mr.DeepCopyObject().(client.Object)
	metav1.SetMetaDataAnnotation(&mr.ObjectMeta, resourcesv1alpha1.Ignore, "true")
	mr.SetFinalizers([]string{})
	if err := c.client.Patch(ctx, mr, client.MergeFromWithOptions(beforePatch, client.MergeFromWithOptimisticLock{})); client.IgnoreNotFound(err) != nil {
		return err
	}

	return client.IgnoreNotFound(c.client.Delete(ctx, mr))
}

func getCustomResourcesLabels() map[string]string {
	return map[string]string{
		v1beta1constants.LabelKeyCustomLoggingResource: v1beta1constants.LabelValueCustomLoggingResource,
	}
}

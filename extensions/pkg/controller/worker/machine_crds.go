// Copyright (c) 2019 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

package worker

import (
	"context"

	"github.com/gardener/gardener/pkg/utils"
	"github.com/gardener/gardener/pkg/utils/flow"
	gutil "github.com/gardener/gardener/pkg/utils/gardener"

	apiextensionsv1beta1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1beta1"
	apiextensionsscheme "k8s.io/apiextensions-apiserver/pkg/client/clientset/clientset/scheme"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/client-go/rest"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

const (
	machineGroup   = "machine.sapcloud.io"
	machineVersion = "v1alpha1"
)

var (
	machineCRDs              []*apiextensionsv1beta1.CustomResourceDefinition
	apiextensionsScheme      = runtime.NewScheme()
	deletionProtectionLabels = map[string]string{
		gutil.DeletionProtected: "true",
	}
)

func init() {
	agePrinterColumn := apiextensionsv1beta1.CustomResourceColumnDefinition{
		Name:        "Age",
		Type:        "date",
		Description: metav1.ObjectMeta{}.SwaggerDoc()["creationTimestamp"],
		JSONPath:    ".metadata.creationTimestamp",
	}

	machineCRDs = []*apiextensionsv1beta1.CustomResourceDefinition{
		{
			ObjectMeta: metav1.ObjectMeta{
				Name: "machinedeployments.machine.sapcloud.io",
			},
			Spec: apiextensionsv1beta1.CustomResourceDefinitionSpec{
				Group:   machineGroup,
				Version: machineVersion,
				Scope:   apiextensionsv1beta1.NamespaceScoped,
				Names: apiextensionsv1beta1.CustomResourceDefinitionNames{
					Kind:       "MachineDeployment",
					Plural:     "machinedeployments",
					Singular:   "machinedeployment",
					ShortNames: []string{"machdeploy"},
				},
				Subresources: &apiextensionsv1beta1.CustomResourceSubresources{
					Status: &apiextensionsv1beta1.CustomResourceSubresourceStatus{},
				},
				AdditionalPrinterColumns: []apiextensionsv1beta1.CustomResourceColumnDefinition{
					{
						Name:        "Ready",
						Type:        "integer",
						Description: "Total number of ready machines targeted by this machine deployment.",
						JSONPath:    ".status.readyReplicas",
					},
					{
						Name:        "Desired",
						Type:        "integer",
						Description: "Number of desired machines.",
						JSONPath:    ".spec.replicas",
					},
					{
						Name:        "Up-to-date",
						Type:        "integer",
						Description: "Total number of non-terminated machines targeted by this machine deployment that have the desired template spec.",
						JSONPath:    ".status.updatedReplicas",
					},
					{
						Name:        "Available",
						Type:        "integer",
						Description: "Total number of available machines (ready for at least minReadySeconds) targeted by this machine deployment.",
						JSONPath:    ".status.availableReplicas",
					},
					agePrinterColumn,
				},
			},
		},
		{
			ObjectMeta: metav1.ObjectMeta{
				Name: "machinesets.machine.sapcloud.io",
			},
			Spec: apiextensionsv1beta1.CustomResourceDefinitionSpec{
				Group:   machineGroup,
				Version: machineVersion,
				Scope:   apiextensionsv1beta1.NamespaceScoped,
				Names: apiextensionsv1beta1.CustomResourceDefinitionNames{
					Kind:       "MachineSet",
					Plural:     "machinesets",
					Singular:   "machineset",
					ShortNames: []string{"machset"},
				},
				Subresources: &apiextensionsv1beta1.CustomResourceSubresources{
					Status: &apiextensionsv1beta1.CustomResourceSubresourceStatus{},
				},
				AdditionalPrinterColumns: []apiextensionsv1beta1.CustomResourceColumnDefinition{
					{
						Name:        "Desired",
						Type:        "integer",
						Description: "Number of desired replicas.",
						JSONPath:    ".spec.replicas",
					},
					{
						Name:        "Current",
						Type:        "integer",
						Description: "Number of actual replicas.",
						JSONPath:    ".status.replicas",
					},
					{
						Name:        "Ready",
						Type:        "integer",
						Description: "Number of ready replicas for this machine set.",
						JSONPath:    ".status.readyReplicas",
					},
					agePrinterColumn,
				},
			},
		},
		{
			ObjectMeta: metav1.ObjectMeta{
				Name: "machines.machine.sapcloud.io",
			},
			Spec: apiextensionsv1beta1.CustomResourceDefinitionSpec{
				Group:   machineGroup,
				Version: machineVersion,
				Scope:   apiextensionsv1beta1.NamespaceScoped,
				Names: apiextensionsv1beta1.CustomResourceDefinitionNames{
					Kind:       "Machine",
					Plural:     "machines",
					Singular:   "machine",
					ShortNames: []string{"mach"},
				},
				Subresources: &apiextensionsv1beta1.CustomResourceSubresources{
					Status: &apiextensionsv1beta1.CustomResourceSubresourceStatus{},
				},
				AdditionalPrinterColumns: []apiextensionsv1beta1.CustomResourceColumnDefinition{
					{
						Name:        "Status",
						Type:        "string",
						Description: "Current status of the machine.",
						JSONPath:    ".status.currentStatus.phase",
					},
					agePrinterColumn,
				},
			},
		},
		{
			ObjectMeta: metav1.ObjectMeta{
				Name: "machineclasses.machine.sapcloud.io",
			},
			Spec: apiextensionsv1beta1.CustomResourceDefinitionSpec{
				Group:   machineGroup,
				Version: machineVersion,
				Scope:   apiextensionsv1beta1.NamespaceScoped,
				Names: apiextensionsv1beta1.CustomResourceDefinitionNames{
					Kind:       "MachineClass",
					Plural:     "machineclasses",
					Singular:   "machineclass",
					ShortNames: []string{"machcls"},
				},
				AdditionalPrinterColumns: []apiextensionsv1beta1.CustomResourceColumnDefinition{
					{
						Name:        "Provider",
						Type:        "string",
						JSONPath:    ".provider",
						Description: "Name of the provider which acts on the machine class object",
					},
					agePrinterColumn,
				},
			},
		},
	}

	type machineClass struct {
		name                     string
		kind                     string
		additionalPrinterColumns []apiextensionsv1beta1.CustomResourceColumnDefinition
	}

	machineClasses := []machineClass{
		{
			name: "alicloud",
			kind: "Alicloud",
			additionalPrinterColumns: []apiextensionsv1beta1.CustomResourceColumnDefinition{
				{
					Name:     "Instance Type",
					Type:     "string",
					JSONPath: ".spec.instanceType",
				},
				{
					Name:     "Region",
					Type:     "string",
					Priority: 1,
					JSONPath: ".spec.region",
				},
				agePrinterColumn,
			},
		},
		{
			name: "aws",
			kind: "AWS",
			additionalPrinterColumns: []apiextensionsv1beta1.CustomResourceColumnDefinition{
				{
					Name:     "Machine Type",
					Type:     "string",
					JSONPath: ".spec.machineType",
				},
				{
					Name:     "AMI",
					Type:     "string",
					JSONPath: ".spec.ami",
				},
				{
					Name:     "Region",
					Type:     "string",
					Priority: 1,
					JSONPath: ".spec.region",
				},
				agePrinterColumn,
			},
		},
		{
			name: "azure",
			kind: "Azure",
			additionalPrinterColumns: []apiextensionsv1beta1.CustomResourceColumnDefinition{
				{
					Name:     "VM Size",
					Type:     "string",
					JSONPath: ".spec.properties.hardwareProfile.vmSize",
				},
				{
					Name:     "Location",
					Type:     "string",
					Priority: 1,
					JSONPath: ".spec.location",
				},
				agePrinterColumn,
			},
		},
		{
			name: "gcp",
			kind: "GCP",
			additionalPrinterColumns: []apiextensionsv1beta1.CustomResourceColumnDefinition{
				{
					Name:     "Machine Type",
					Type:     "string",
					JSONPath: ".spec.machineType",
				},
				{
					Name:     "Region",
					Type:     "string",
					Priority: 1,
					JSONPath: ".spec.region",
				},
				agePrinterColumn,
			},
		},
		{
			name: "openstack",
			kind: "OpenStack",
			additionalPrinterColumns: []apiextensionsv1beta1.CustomResourceColumnDefinition{
				{
					Name:     "Flavor",
					Type:     "string",
					JSONPath: ".spec.flavorName",
				},
				{
					Name:     "Image",
					Type:     "string",
					JSONPath: ".spec.imageName",
				},
				{
					Name:     "Region",
					Type:     "string",
					Priority: 1,
					JSONPath: ".spec.region",
				},
				agePrinterColumn,
			},
		},
		{
			name: "packet",
			kind: "Packet",
			additionalPrinterColumns: []apiextensionsv1beta1.CustomResourceColumnDefinition{
				agePrinterColumn,
			},
		},
	}

	for _, current := range machineClasses {
		machineCRDs = append(machineCRDs, &apiextensionsv1beta1.CustomResourceDefinition{
			ObjectMeta: metav1.ObjectMeta{
				Name: current.name + "machineclasses.machine.sapcloud.io",
			},
			Spec: apiextensionsv1beta1.CustomResourceDefinitionSpec{
				Group:   machineGroup,
				Version: machineVersion,
				Scope:   apiextensionsv1beta1.NamespaceScoped,
				Names: apiextensionsv1beta1.CustomResourceDefinitionNames{
					Kind:       current.kind + "MachineClass",
					Plural:     current.name + "machineclasses",
					Singular:   current.name + "machineclass",
					ShortNames: []string{current.name + "cls"},
				},
				Subresources: &apiextensionsv1beta1.CustomResourceSubresources{
					Status: &apiextensionsv1beta1.CustomResourceSubresourceStatus{},
				},
				AdditionalPrinterColumns: current.additionalPrinterColumns,
			},
		})
	}

	utilruntime.Must(apiextensionsscheme.AddToScheme(apiextensionsScheme))
}

// ApplyMachineResourcesForConfig ensures that all well-known machine CRDs are created or updated.
func ApplyMachineResourcesForConfig(ctx context.Context, config *rest.Config) error {
	c, err := client.New(config, client.Options{Scheme: apiextensionsScheme})
	if err != nil {
		return err
	}

	return ApplyMachineResources(ctx, c)
}

// ApplyMachineResources ensures that all well-known machine CRDs are created or updated.
func ApplyMachineResources(ctx context.Context, c client.Client) error {
	fns := make([]flow.TaskFn, 0, len(machineCRDs))

	for _, crd := range machineCRDs {
		obj := &apiextensionsv1beta1.CustomResourceDefinition{
			ObjectMeta: metav1.ObjectMeta{
				Name: crd.Name,
			},
		}
		spec := crd.Spec.DeepCopy()

		fns = append(fns, func(ctx context.Context) error {
			_, err := controllerutil.CreateOrUpdate(ctx, c, obj, func() error {
				obj.Labels = utils.MergeStringMaps(obj.Labels, deletionProtectionLabels)
				obj.Spec = *spec
				return nil
			})
			return err
		})
	}

	return flow.Parallel(fns...)(ctx)
}

// Copyright (c) 2018 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

package kubernetes

import (
	machinev1alpha1 "github.com/gardener/machine-controller-manager/pkg/apis/machine/v1alpha1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/labels"
)

// MachineDeploymentSource is a function that produces a slice of MachineDeployments or an error.
type MachineDeploymentSource func() ([]*machinev1alpha1.MachineDeployment, error)

// MachineDeploymentLister is a lister of MachineDeployments for a specific namespace.
type MachineDeploymentLister interface {
	// List lists all MachineDeployments that match the given selector in the current namespace.
	List(selector labels.Selector) ([]*machinev1alpha1.MachineDeployment, error)
	// MachineDeployments yields a MachineDeploymentNamespaceLister for the given namespace.
	MachineDeployments(namespace string) MachineDeploymentNamespaceLister
}

// MachineDeploymentNamespaceLister is  a lister of MachineDeployments for a specific namespace.
type MachineDeploymentNamespaceLister interface {
	// List lists all MachineDeployments that match the given selector in the current namespace.
	List(selector labels.Selector) ([]*machinev1alpha1.MachineDeployment, error)
	// Get retrieves the MachineDeployment with the given name in the current namespace.
	Get(name string) (*machinev1alpha1.MachineDeployment, error)
}

type machineDeploymentLister struct {
	source MachineDeploymentSource
}

type machineDeploymentNamespaceLister struct {
	source    MachineDeploymentSource
	namespace string
}

// NewMachineDeploymentLister creates a new MachineDeploymentLister from the given MachineDeploymentSource.
func NewMachineDeploymentLister(source MachineDeploymentSource) MachineDeploymentLister {
	return &machineDeploymentLister{source: source}
}

func filterMachineDeployments(source MachineDeploymentSource, filter func(*machinev1alpha1.MachineDeployment) bool) ([]*machinev1alpha1.MachineDeployment, error) {
	machineDeployments, err := source()
	if err != nil {
		return nil, err
	}

	var out []*machinev1alpha1.MachineDeployment
	for _, machineDeployment := range machineDeployments {
		if filter(machineDeployment) {
			out = append(out, machineDeployment)
		}
	}
	return out, nil
}

func (d *machineDeploymentLister) List(selector labels.Selector) ([]*machinev1alpha1.MachineDeployment, error) {
	return filterMachineDeployments(d.source, func(machineDeployment *machinev1alpha1.MachineDeployment) bool {
		return selector.Matches(labels.Set(machineDeployment.Labels))
	})
}

func (d *machineDeploymentLister) MachineDeployments(namespace string) MachineDeploymentNamespaceLister {
	return &machineDeploymentNamespaceLister{
		source:    d.source,
		namespace: namespace,
	}
}

func (d *machineDeploymentNamespaceLister) Get(name string) (*machinev1alpha1.MachineDeployment, error) {
	machineDeployments, err := filterMachineDeployments(d.source, func(machineDeployment *machinev1alpha1.MachineDeployment) bool {
		return machineDeployment.Namespace == d.namespace && machineDeployment.Name == name
	})
	if err != nil {
		return nil, err
	}

	if len(machineDeployments) == 0 {
		return nil, apierrors.NewNotFound(machinev1alpha1.Resource("MachineDeployments"), name)
	}
	return machineDeployments[0], nil
}

func (d *machineDeploymentNamespaceLister) List(selector labels.Selector) ([]*machinev1alpha1.MachineDeployment, error) {
	return filterMachineDeployments(d.source, func(machineDeployment *machinev1alpha1.MachineDeployment) bool {
		return machineDeployment.Namespace == d.namespace && selector.Matches(labels.Set(machineDeployment.Labels))
	})
}

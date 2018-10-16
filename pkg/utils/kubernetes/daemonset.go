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
	appsv1 "k8s.io/api/apps/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/labels"
)

// DaemonSetSource is a function that produces a slice of DaemonSets or an error.
type DaemonSetSource func() ([]*appsv1.DaemonSet, error)

// DaemonSetLister is a lister of DaemonSets.
type DaemonSetLister interface {
	// List lists all DaemonSets that match the given selector.
	List(selector labels.Selector) ([]*appsv1.DaemonSet, error)
	// DaemonSets yields a DaemonSetNamespaceLister for the given namespace.
	DaemonSets(namespace string) DaemonSetNamespaceLister
}

// DaemonSetNamespaceLister is a lister of deployments for a specific namespace.
type DaemonSetNamespaceLister interface {
	// List lists all DaemonSets that match the given selector in the current namespace.
	List(selector labels.Selector) ([]*appsv1.DaemonSet, error)
	// Get retrieves the DaemonSet with the given name in the current namespace.
	Get(name string) (*appsv1.DaemonSet, error)
}

type daemonSetLister struct {
	source DaemonSetSource
}

type daemonSetNamespaceLister struct {
	source    DaemonSetSource
	namespace string
}

// NewDaemonSetLister creates a new DaemonSetLister from the given DaemonSetSource.
func NewDaemonSetLister(source DaemonSetSource) DaemonSetLister {
	return &daemonSetLister{source: source}
}

func filterDaemonSets(source DaemonSetSource, filter func(*appsv1.DaemonSet) bool) ([]*appsv1.DaemonSet, error) {
	daemonSets, err := source()
	if err != nil {
		return nil, err
	}

	var out []*appsv1.DaemonSet
	for _, daemonSet := range daemonSets {
		if filter(daemonSet) {
			out = append(out, daemonSet)
		}
	}
	return out, nil
}

func (d *daemonSetLister) List(selector labels.Selector) ([]*appsv1.DaemonSet, error) {
	return filterDaemonSets(d.source, func(daemonSet *appsv1.DaemonSet) bool {
		return selector.Matches(labels.Set(daemonSet.Labels))
	})
}

func (d *daemonSetLister) DaemonSets(namespace string) DaemonSetNamespaceLister {
	return &daemonSetNamespaceLister{
		source:    d.source,
		namespace: namespace,
	}
}

func (d *daemonSetNamespaceLister) Get(name string) (*appsv1.DaemonSet, error) {
	daemonSets, err := filterDaemonSets(d.source, func(daemonSet *appsv1.DaemonSet) bool {
		return daemonSet.Namespace == d.namespace && daemonSet.Name == name
	})
	if err != nil {
		return nil, err
	}

	if len(daemonSets) == 0 {
		return nil, apierrors.NewNotFound(appsv1.Resource("DaemonSets"), name)
	}
	return daemonSets[0], nil
}

func (d *daemonSetNamespaceLister) List(selector labels.Selector) ([]*appsv1.DaemonSet, error) {
	return filterDaemonSets(d.source, func(daemonSet *appsv1.DaemonSet) bool {
		return daemonSet.Namespace == d.namespace && selector.Matches(labels.Set(daemonSet.Labels))
	})
}

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

// StatefulSetSource is a function that produces a slice of StatefulSets or an error.
type StatefulSetSource func() ([]*appsv1.StatefulSet, error)

// StatefulSetLister is a lister of StatefulSets.
type StatefulSetLister interface {
	// List lists all StatefulSets that match the given selector.
	List(selector labels.Selector) ([]*appsv1.StatefulSet, error)
	// StatefulSets yields a StatefulSetNamespaceLister for the given namespace.
	StatefulSets(namespace string) StatefulSetNamespaceLister
}

// StatefulSetNamespaceLister is a lister of StatefulSets for a specific namespace.
type StatefulSetNamespaceLister interface {
	// List lists all StatefulSets that match the given selector in the current namespace.
	List(selector labels.Selector) ([]*appsv1.StatefulSet, error)
	// Get retrieves the StatefulSet with the given name in the current namespace.
	Get(name string) (*appsv1.StatefulSet, error)
}

type statefulSetLister struct {
	source StatefulSetSource
}

type statefulSetNamespaceLister struct {
	source    StatefulSetSource
	namespace string
}

// NewStatefulSetLister creates a new StatefulSetLister form the given StatefulSetSource.
func NewStatefulSetLister(source StatefulSetSource) StatefulSetLister {
	return &statefulSetLister{source: source}
}

func filterStatefulSets(source StatefulSetSource, filter func(*appsv1.StatefulSet) bool) ([]*appsv1.StatefulSet, error) {
	statefulSets, err := source()
	if err != nil {
		return nil, err
	}

	var out []*appsv1.StatefulSet
	for _, statefulSet := range statefulSets {
		if filter(statefulSet) {
			out = append(out, statefulSet)
		}
	}
	return out, nil
}

func (d *statefulSetLister) List(selector labels.Selector) ([]*appsv1.StatefulSet, error) {
	return filterStatefulSets(d.source, func(statefulSet *appsv1.StatefulSet) bool {
		return selector.Matches(labels.Set(statefulSet.Labels))
	})
}

func (d *statefulSetLister) StatefulSets(namespace string) StatefulSetNamespaceLister {
	return &statefulSetNamespaceLister{
		source:    d.source,
		namespace: namespace,
	}
}

func (d *statefulSetNamespaceLister) Get(name string) (*appsv1.StatefulSet, error) {
	statefulSets, err := filterStatefulSets(d.source, func(statefulSet *appsv1.StatefulSet) bool {
		return statefulSet.Namespace == d.namespace && statefulSet.Name == name
	})
	if err != nil {
		return nil, err
	}

	if len(statefulSets) == 0 {
		return nil, apierrors.NewNotFound(appsv1.Resource("StatefulSets"), name)
	}
	return statefulSets[0], nil
}

func (d *statefulSetNamespaceLister) List(selector labels.Selector) ([]*appsv1.StatefulSet, error) {
	return filterStatefulSets(d.source, func(statefulSet *appsv1.StatefulSet) bool {
		return statefulSet.Namespace == d.namespace && selector.Matches(labels.Set(statefulSet.Labels))
	})
}

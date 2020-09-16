// SPDX-FileCopyrightText: 2018 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package kubernetes

import (
	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/labels"
)

// WorkerSource is a function that produces a slice of Workers or an error.
type WorkerSource func() ([]*extensionsv1alpha1.Worker, error)

// WorkerLister is a lister of Workers for a specific namespace.
type WorkerLister interface {
	// List lists all Workers that match the given selector in the current namespace.
	List(selector labels.Selector) ([]*extensionsv1alpha1.Worker, error)
	// Workers yields a WorkerNamespaceLister for the given namespace.
	Workers(namespace string) WorkerNamespaceLister
}

// WorkerNamespaceLister is  a lister of Workers for a specific namespace.
type WorkerNamespaceLister interface {
	// List lists all Workers that match the given selector in the current namespace.
	List(selector labels.Selector) ([]*extensionsv1alpha1.Worker, error)
	// Get retrieves the MachineDeployment with the given name in the current namespace.
	Get(name string) (*extensionsv1alpha1.Worker, error)
}

type workerLister struct {
	source WorkerSource
}

type workerNamespaceLister struct {
	source    WorkerSource
	namespace string
}

// NewWorkerLister creates a new WorkerLister from the given WorkerSource.
func NewWorkerLister(source WorkerSource) WorkerLister {
	return &workerLister{source: source}
}

func filterWorkers(source WorkerSource, filter func(worker *extensionsv1alpha1.Worker) bool) ([]*extensionsv1alpha1.Worker, error) {
	workers, err := source()
	if err != nil {
		return nil, err
	}

	var out []*extensionsv1alpha1.Worker
	for _, worker := range workers {
		if filter(worker) {
			out = append(out, worker)
		}
	}
	return out, nil
}

func (d *workerLister) List(selector labels.Selector) ([]*extensionsv1alpha1.Worker, error) {
	return filterWorkers(d.source, func(worker *extensionsv1alpha1.Worker) bool {
		return selector.Matches(labels.Set(worker.Labels))
	})
}

func (d *workerLister) Workers(namespace string) WorkerNamespaceLister {
	return &workerNamespaceLister{
		source:    d.source,
		namespace: namespace,
	}
}

func (d *workerNamespaceLister) Get(name string) (*extensionsv1alpha1.Worker, error) {
	workers, err := filterWorkers(d.source, func(worker *extensionsv1alpha1.Worker) bool {
		return worker.Namespace == d.namespace && worker.Name == name
	})
	if err != nil {
		return nil, err
	}

	if len(workers) == 0 {
		return nil, apierrors.NewNotFound(extensionsv1alpha1.Resource("worker"), name)
	}
	return workers[0], nil
}

func (d *workerNamespaceLister) List(selector labels.Selector) ([]*extensionsv1alpha1.Worker, error) {
	return filterWorkers(d.source, func(worker *extensionsv1alpha1.Worker) bool {
		return worker.Namespace == d.namespace && selector.Matches(labels.Set(worker.Labels))
	})
}

// SPDX-FileCopyrightText: 2018 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package kubernetes

import (
	druidv1alpha1 "github.com/gardener/etcd-druid/api/v1alpha1"
	appsv1 "k8s.io/api/apps/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/labels"
)

// EtcdSource is a function that produces a slice of Etcds or an error.
type EtcdSource func() ([]*druidv1alpha1.Etcd, error)

// EtcdLister is a lister of Etcds.
type EtcdLister interface {
	// List lists all Etcds that match the given selector.
	List(selector labels.Selector) ([]*druidv1alpha1.Etcd, error)
	// Etcds yields a EtcdNamespaceLister for the given namespace.
	Etcds(namespace string) EtcdNamespaceLister
}

// EtcdNamespaceLister is a lister of etcds for a specific namespace.
type EtcdNamespaceLister interface {
	// List lists all Etcds that match the given selector in the current namespace.
	List(selector labels.Selector) ([]*druidv1alpha1.Etcd, error)
	// Get retrieves the Etcd with the given name in the current namespace.
	Get(name string) (*druidv1alpha1.Etcd, error)
}

type etcdLister struct {
	source EtcdSource
}

type etcdNamespaceLister struct {
	source    EtcdSource
	namespace string
}

// NewEtcdLister creates a new EtcdLister from the given EtcdSource.
func NewEtcdLister(source EtcdSource) EtcdLister {
	return &etcdLister{source: source}
}

func filterEtcds(source EtcdSource, filter func(*druidv1alpha1.Etcd) bool) ([]*druidv1alpha1.Etcd, error) {
	etcds, err := source()
	if err != nil {
		return nil, err
	}

	var out []*druidv1alpha1.Etcd
	for _, etcd := range etcds {
		if filter(etcd) {
			out = append(out, etcd)
		}
	}
	return out, nil
}

func (d *etcdLister) List(selector labels.Selector) ([]*druidv1alpha1.Etcd, error) {
	return filterEtcds(d.source, func(node *druidv1alpha1.Etcd) bool {
		return selector.Matches(labels.Set(node.Labels))
	})
}

func (d *etcdLister) Etcds(namespace string) EtcdNamespaceLister {
	return &etcdNamespaceLister{
		source:    d.source,
		namespace: namespace,
	}
}

func (d *etcdNamespaceLister) Get(name string) (*druidv1alpha1.Etcd, error) {
	etcds, err := filterEtcds(d.source, func(etcd *druidv1alpha1.Etcd) bool {
		return etcd.Namespace == d.namespace && etcd.Name == name
	})
	if err != nil {
		return nil, err
	}

	if len(etcds) == 0 {
		return nil, apierrors.NewNotFound(appsv1.Resource("Etcds"), name)
	}
	return etcds[0], nil
}

func (d *etcdNamespaceLister) List(selector labels.Selector) ([]*druidv1alpha1.Etcd, error) {
	return filterEtcds(d.source, func(etcd *druidv1alpha1.Etcd) bool {
		return etcd.Namespace == d.namespace && selector.Matches(labels.Set(etcd.Labels))
	})
}

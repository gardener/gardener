// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
// SPDX-License-Identifier: Apache-2.0

// Code generated by lister-gen. DO NOT EDIT.

package v1beta1

import (
	v1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/client-go/tools/cache"
)

// QuotaLister helps list Quotas.
// All objects returned here must be treated as read-only.
type QuotaLister interface {
	// List lists all Quotas in the indexer.
	// Objects returned here must be treated as read-only.
	List(selector labels.Selector) (ret []*v1beta1.Quota, err error)
	// Quotas returns an object that can list and get Quotas.
	Quotas(namespace string) QuotaNamespaceLister
	QuotaListerExpansion
}

// quotaLister implements the QuotaLister interface.
type quotaLister struct {
	indexer cache.Indexer
}

// NewQuotaLister returns a new QuotaLister.
func NewQuotaLister(indexer cache.Indexer) QuotaLister {
	return &quotaLister{indexer: indexer}
}

// List lists all Quotas in the indexer.
func (s *quotaLister) List(selector labels.Selector) (ret []*v1beta1.Quota, err error) {
	err = cache.ListAll(s.indexer, selector, func(m interface{}) {
		ret = append(ret, m.(*v1beta1.Quota))
	})
	return ret, err
}

// Quotas returns an object that can list and get Quotas.
func (s *quotaLister) Quotas(namespace string) QuotaNamespaceLister {
	return quotaNamespaceLister{indexer: s.indexer, namespace: namespace}
}

// QuotaNamespaceLister helps list and get Quotas.
// All objects returned here must be treated as read-only.
type QuotaNamespaceLister interface {
	// List lists all Quotas in the indexer for a given namespace.
	// Objects returned here must be treated as read-only.
	List(selector labels.Selector) (ret []*v1beta1.Quota, err error)
	// Get retrieves the Quota from the indexer for a given namespace and name.
	// Objects returned here must be treated as read-only.
	Get(name string) (*v1beta1.Quota, error)
	QuotaNamespaceListerExpansion
}

// quotaNamespaceLister implements the QuotaNamespaceLister
// interface.
type quotaNamespaceLister struct {
	indexer   cache.Indexer
	namespace string
}

// List lists all Quotas in the indexer for a given namespace.
func (s quotaNamespaceLister) List(selector labels.Selector) (ret []*v1beta1.Quota, err error) {
	err = cache.ListAllByNamespace(s.indexer, s.namespace, selector, func(m interface{}) {
		ret = append(ret, m.(*v1beta1.Quota))
	})
	return ret, err
}

// Get retrieves the Quota from the indexer for a given namespace and name.
func (s quotaNamespaceLister) Get(name string) (*v1beta1.Quota, error) {
	obj, exists, err := s.indexer.GetByKey(s.namespace + "/" + name)
	if err != nil {
		return nil, err
	}
	if !exists {
		return nil, errors.NewNotFound(v1beta1.Resource("quota"), name)
	}
	return obj.(*v1beta1.Quota), nil
}

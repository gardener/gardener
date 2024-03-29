// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

// Code generated by lister-gen. DO NOT EDIT.

package v1alpha1

import (
	v1alpha1 "github.com/gardener/gardener/pkg/apis/settings/v1alpha1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/client-go/tools/cache"
)

// OpenIDConnectPresetLister helps list OpenIDConnectPresets.
// All objects returned here must be treated as read-only.
type OpenIDConnectPresetLister interface {
	// List lists all OpenIDConnectPresets in the indexer.
	// Objects returned here must be treated as read-only.
	List(selector labels.Selector) (ret []*v1alpha1.OpenIDConnectPreset, err error)
	// OpenIDConnectPresets returns an object that can list and get OpenIDConnectPresets.
	OpenIDConnectPresets(namespace string) OpenIDConnectPresetNamespaceLister
	OpenIDConnectPresetListerExpansion
}

// openIDConnectPresetLister implements the OpenIDConnectPresetLister interface.
type openIDConnectPresetLister struct {
	indexer cache.Indexer
}

// NewOpenIDConnectPresetLister returns a new OpenIDConnectPresetLister.
func NewOpenIDConnectPresetLister(indexer cache.Indexer) OpenIDConnectPresetLister {
	return &openIDConnectPresetLister{indexer: indexer}
}

// List lists all OpenIDConnectPresets in the indexer.
func (s *openIDConnectPresetLister) List(selector labels.Selector) (ret []*v1alpha1.OpenIDConnectPreset, err error) {
	err = cache.ListAll(s.indexer, selector, func(m interface{}) {
		ret = append(ret, m.(*v1alpha1.OpenIDConnectPreset))
	})
	return ret, err
}

// OpenIDConnectPresets returns an object that can list and get OpenIDConnectPresets.
func (s *openIDConnectPresetLister) OpenIDConnectPresets(namespace string) OpenIDConnectPresetNamespaceLister {
	return openIDConnectPresetNamespaceLister{indexer: s.indexer, namespace: namespace}
}

// OpenIDConnectPresetNamespaceLister helps list and get OpenIDConnectPresets.
// All objects returned here must be treated as read-only.
type OpenIDConnectPresetNamespaceLister interface {
	// List lists all OpenIDConnectPresets in the indexer for a given namespace.
	// Objects returned here must be treated as read-only.
	List(selector labels.Selector) (ret []*v1alpha1.OpenIDConnectPreset, err error)
	// Get retrieves the OpenIDConnectPreset from the indexer for a given namespace and name.
	// Objects returned here must be treated as read-only.
	Get(name string) (*v1alpha1.OpenIDConnectPreset, error)
	OpenIDConnectPresetNamespaceListerExpansion
}

// openIDConnectPresetNamespaceLister implements the OpenIDConnectPresetNamespaceLister
// interface.
type openIDConnectPresetNamespaceLister struct {
	indexer   cache.Indexer
	namespace string
}

// List lists all OpenIDConnectPresets in the indexer for a given namespace.
func (s openIDConnectPresetNamespaceLister) List(selector labels.Selector) (ret []*v1alpha1.OpenIDConnectPreset, err error) {
	err = cache.ListAllByNamespace(s.indexer, s.namespace, selector, func(m interface{}) {
		ret = append(ret, m.(*v1alpha1.OpenIDConnectPreset))
	})
	return ret, err
}

// Get retrieves the OpenIDConnectPreset from the indexer for a given namespace and name.
func (s openIDConnectPresetNamespaceLister) Get(name string) (*v1alpha1.OpenIDConnectPreset, error) {
	obj, exists, err := s.indexer.GetByKey(s.namespace + "/" + name)
	if err != nil {
		return nil, err
	}
	if !exists {
		return nil, errors.NewNotFound(v1alpha1.Resource("openidconnectpreset"), name)
	}
	return obj.(*v1alpha1.OpenIDConnectPreset), nil
}

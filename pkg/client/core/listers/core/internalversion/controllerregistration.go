// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
// SPDX-License-Identifier: Apache-2.0

// Code generated by lister-gen. DO NOT EDIT.

package internalversion

import (
	core "github.com/gardener/gardener/pkg/apis/core"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/client-go/tools/cache"
)

// ControllerRegistrationLister helps list ControllerRegistrations.
// All objects returned here must be treated as read-only.
type ControllerRegistrationLister interface {
	// List lists all ControllerRegistrations in the indexer.
	// Objects returned here must be treated as read-only.
	List(selector labels.Selector) (ret []*core.ControllerRegistration, err error)
	// Get retrieves the ControllerRegistration from the index for a given name.
	// Objects returned here must be treated as read-only.
	Get(name string) (*core.ControllerRegistration, error)
	ControllerRegistrationListerExpansion
}

// controllerRegistrationLister implements the ControllerRegistrationLister interface.
type controllerRegistrationLister struct {
	indexer cache.Indexer
}

// NewControllerRegistrationLister returns a new ControllerRegistrationLister.
func NewControllerRegistrationLister(indexer cache.Indexer) ControllerRegistrationLister {
	return &controllerRegistrationLister{indexer: indexer}
}

// List lists all ControllerRegistrations in the indexer.
func (s *controllerRegistrationLister) List(selector labels.Selector) (ret []*core.ControllerRegistration, err error) {
	err = cache.ListAll(s.indexer, selector, func(m interface{}) {
		ret = append(ret, m.(*core.ControllerRegistration))
	})
	return ret, err
}

// Get retrieves the ControllerRegistration from the index for a given name.
func (s *controllerRegistrationLister) Get(name string) (*core.ControllerRegistration, error) {
	obj, exists, err := s.indexer.GetByKey(name)
	if err != nil {
		return nil, err
	}
	if !exists {
		return nil, errors.NewNotFound(core.Resource("controllerregistration"), name)
	}
	return obj.(*core.ControllerRegistration), nil
}

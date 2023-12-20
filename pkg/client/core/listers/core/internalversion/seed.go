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

// SeedLister helps list Seeds.
// All objects returned here must be treated as read-only.
type SeedLister interface {
	// List lists all Seeds in the indexer.
	// Objects returned here must be treated as read-only.
	List(selector labels.Selector) (ret []*core.Seed, err error)
	// Get retrieves the Seed from the index for a given name.
	// Objects returned here must be treated as read-only.
	Get(name string) (*core.Seed, error)
	SeedListerExpansion
}

// seedLister implements the SeedLister interface.
type seedLister struct {
	indexer cache.Indexer
}

// NewSeedLister returns a new SeedLister.
func NewSeedLister(indexer cache.Indexer) SeedLister {
	return &seedLister{indexer: indexer}
}

// List lists all Seeds in the indexer.
func (s *seedLister) List(selector labels.Selector) (ret []*core.Seed, err error) {
	err = cache.ListAll(s.indexer, selector, func(m interface{}) {
		ret = append(ret, m.(*core.Seed))
	})
	return ret, err
}

// Get retrieves the Seed from the index for a given name.
func (s *seedLister) Get(name string) (*core.Seed, error) {
	obj, exists, err := s.indexer.GetByKey(name)
	if err != nil {
		return nil, err
	}
	if !exists {
		return nil, errors.NewNotFound(core.Resource("seed"), name)
	}
	return obj.(*core.Seed), nil
}

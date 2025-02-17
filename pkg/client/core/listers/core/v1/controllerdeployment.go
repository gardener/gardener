// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

// Code generated by lister-gen. DO NOT EDIT.

package v1

import (
	corev1 "github.com/gardener/gardener/pkg/apis/core/v1"
	labels "k8s.io/apimachinery/pkg/labels"
	listers "k8s.io/client-go/listers"
	cache "k8s.io/client-go/tools/cache"
)

// ControllerDeploymentLister helps list ControllerDeployments.
// All objects returned here must be treated as read-only.
type ControllerDeploymentLister interface {
	// List lists all ControllerDeployments in the indexer.
	// Objects returned here must be treated as read-only.
	List(selector labels.Selector) (ret []*corev1.ControllerDeployment, err error)
	// Get retrieves the ControllerDeployment from the index for a given name.
	// Objects returned here must be treated as read-only.
	Get(name string) (*corev1.ControllerDeployment, error)
	ControllerDeploymentListerExpansion
}

// controllerDeploymentLister implements the ControllerDeploymentLister interface.
type controllerDeploymentLister struct {
	listers.ResourceIndexer[*corev1.ControllerDeployment]
}

// NewControllerDeploymentLister returns a new ControllerDeploymentLister.
func NewControllerDeploymentLister(indexer cache.Indexer) ControllerDeploymentLister {
	return &controllerDeploymentLister{listers.New[*corev1.ControllerDeployment](indexer, corev1.Resource("controllerdeployment"))}
}

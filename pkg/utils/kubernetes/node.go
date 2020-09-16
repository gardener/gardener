// SPDX-FileCopyrightText: 2018 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package kubernetes

import (
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/labels"
)

// NodeSource is a function that produces a slice of Nodes or an error.
type NodeSource func() ([]*corev1.Node, error)

// NodeLister is a lister of Nodes.
type NodeLister interface {
	// List lists all Nodes that match the given selector.
	List(selector labels.Selector) ([]*corev1.Node, error)
}

type nodeLister struct {
	source NodeSource
}

// NewNodeLister creates a new NodeLister from the given NodeSource.
func NewNodeLister(source NodeSource) NodeLister {
	return &nodeLister{source: source}
}

func filterNodes(source NodeSource, filter func(*corev1.Node) bool) ([]*corev1.Node, error) {
	nodes, err := source()
	if err != nil {
		return nil, err
	}

	var out []*corev1.Node
	for _, node := range nodes {
		if filter(node) {
			out = append(out, node)
		}
	}
	return out, nil
}

func (d *nodeLister) List(selector labels.Selector) ([]*corev1.Node, error) {
	return filterNodes(d.source, func(node *corev1.Node) bool {
		return selector.Matches(labels.Set(node.Labels))
	})
}

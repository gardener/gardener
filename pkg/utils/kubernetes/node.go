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

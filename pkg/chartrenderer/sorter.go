/*
  Copyright The Helm Authors.
  Licensed under the Apache License, Version 2.0 (the "License");
  you may not use this file except in compliance with the License.
  You may obtain a copy of the License at
      http://www.apache.org/licenses/LICENSE-2.0
  Unless required by applicable law or agreed to in writing, software
  distributed under the License is distributed on an "AS IS" BASIS,
  WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
  See the License for the specific language governing permissions and
  limitations under the License.
*/

// @rfranzke, @zanetworker:
// Origin of this file: https://github.com/helm/helm/blob/bed4054c412f95d140c8c98b6387f40df7f3139e/pkg/tiller/kind_sorter.go
// We have copied this to our repository to not depend on k8s.io/helm/pkg/tiller.
// This package is depending on k8s.io/helm/pkg/kube which transitively depends on k8s.io/kubernetes.
// To prevent vendor hell we don't want to depend on k8s.io/kubernetes.

package chartrenderer

import (
	"sort"

	"k8s.io/helm/pkg/manifest"
)

// SortOrder is an ordering of Kinds.
type SortOrder []string

// InstallOrder is the order in which manifests should be installed (by Kind).
//
// Those occurring earlier in the list get installed before those occurring later in the list.
var InstallOrder SortOrder = []string{
	"Namespace",
	"ResourceQuota",
	"LimitRange",
	"PodSecurityPolicy",
	"PodDisruptionBudget",
	"Secret",
	"ConfigMap",
	"StorageClass",
	"PersistentVolume",
	"PersistentVolumeClaim",
	"ServiceAccount",
	"CustomResourceDefinition",
	"ClusterRole",
	"ClusterRoleBinding",
	"Role",
	"RoleBinding",
	"Service",
	"DaemonSet",
	"Pod",
	"ReplicationController",
	"ReplicaSet",
	"Deployment",
	"StatefulSet",
	"Job",
	"CronJob",
	"Ingress",
	"APIService",
}

type kindSorter struct {
	ordering  map[string]int
	manifests []manifest.Manifest
}

func newKindSorter(m []manifest.Manifest, s SortOrder) *kindSorter {
	o := make(map[string]int, len(s))
	for v, k := range s {
		o[k] = v
	}

	return &kindSorter{
		manifests: m,
		ordering:  o,
	}
}

func (k *kindSorter) Len() int { return len(k.manifests) }

func (k *kindSorter) Swap(i, j int) { k.manifests[i], k.manifests[j] = k.manifests[j], k.manifests[i] }

func (k *kindSorter) Less(i, j int) bool {
	a := k.manifests[i]
	b := k.manifests[j]
	first, aok := k.ordering[a.Head.Kind]
	second, bok := k.ordering[b.Head.Kind]

	if !aok && !bok {
		// if both are unknown then sort alphabetically by kind and name
		if a.Head.Kind != b.Head.Kind {
			return a.Head.Kind < b.Head.Kind
		}
		return a.Name < b.Name
	}

	// unknown kind is last
	if !aok {
		return false
	}
	if !bok {
		return true
	}

	// if same kind sub sort alphanumeric
	if first == second {
		return a.Name < b.Name
	}
	// sort different kinds
	return first < second
}

// SortByKind sorts manifests in InstallOrder
func SortByKind(manifests []manifest.Manifest) []manifest.Manifest {
	ordering := InstallOrder
	ks := newKindSorter(manifests, ordering)
	sort.Sort(ks)
	return ks.manifests
}

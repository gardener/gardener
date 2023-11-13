// Copyright 2023 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package kubernetes

import versionutils "github.com/gardener/gardener/pkg/utils/version"

// APIGroupControllerMap is a map for the Kubernetes API groups and the corresponding controllers for them.
var APIGroupControllerMap = map[string]map[string]versionutils.VersionRange{
	"internal/v1alpha1": {
		"storage-version-gc": {},
	},
	"apps/v1": {
		"daemonset":   {},
		"deployment":  {},
		"replicaset":  {},
		"statefulset": {},
	},
	"apps/v1beta1": {
		"disruption": {},
	},
	"authentication/v1": {
		"attachdetach":              {},
		"persistentvolume-expander": {},
	},
	"authorization/v1": {
		"csrapproving": {},
	},
	"autoscaling/v1": {
		"horizontalpodautoscaling": {},
	},
	"autoscaling/v2": {
		"horizontalpodautoscaling": {},
	},
	"batch/v1": {
		"cronjob":            {},
		"job":                {},
		"ttl-after-finished": {},
	},
	"certificates/v1": {
		"csrapproving": {},
		"csrcleaner":   {},
		"csrsigning":   {},
	},
	"certificates/v1beta1": {
		"csrsigning": {},
	},
	"coordination/v1": {
		"nodelifecycle":      {},
		"storage-version-gc": {},
	},
	"discovery/v1": {
		"endpointslice":          {},
		"endpointslicemirroring": {},
	},
	"extensions/v1beta1": {
		"disruption": {},
	},
	"policy/v1": {
		"disruption": {},
	},
	"rbac/v1": {
		"clusterrole-aggregation": {},
	},
	"resource/v1alpha2": {
		"resource-claim-controller": {AddedInVersion: "1.27"},
	},
	"v1": {
		"attachdetach":                         {},
		"bootstrapsigner":                      {},
		"cloud-node-lifecycle":                 {},
		"cronjob":                              {},
		"csrapproving":                         {},
		"csrsigning":                           {},
		"daemonset":                            {},
		"deployment":                           {},
		"disruption":                           {},
		"endpoint":                             {},
		"endpointslice":                        {},
		"endpointslicemirroring":               {},
		"ephemeral-volume":                     {},
		"garbagecollector":                     {},
		"horizontalpodautoscaling":             {},
		"job":                                  {},
		"legacy-service-account-token-cleaner": {AddedInVersion: "1.28"},
		"namespace":                            {},
		"nodelifecycle":                        {},
		"persistentvolume-binder":              {},
		"persistentvolume-expander":            {},
		"podgc":                                {},
		"pv-protection":                        {},
		"pvc-protection":                       {},
		"replicaset":                           {},
		"replicationcontroller":                {},
		"resource-claim-controller":            {AddedInVersion: "1.27"},
		"resourcequota":                        {},
		"root-ca-cert-publisher":               {},
		"route":                                {},
		"service":                              {},
		"serviceaccount":                       {},
		"serviceaccount-token":                 {},
		"statefulset":                          {},
		"tokencleaner":                         {},
		"ttl":                                  {},
		"ttl-after-finished":                   {},
	},
}

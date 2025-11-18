// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package kubernetes

import versionutils "github.com/gardener/gardener/pkg/utils/version"

// APIGroupControllerMap is a map for the Kubernetes API groups and the corresponding controllers for them.
var APIGroupControllerMap = map[string]map[string]versionutils.VersionRange{
	"internal/v1alpha1": {
		"storage-version-gc": {},
	},
	"admissionregistration/v1": {
		"validatingadmissionpolicy-status-controller": {},
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
	"certificates/v1alpha1": {
		"kube-apiserver-serving-clustertrustbundle-publisher-controller": {AddedInVersion: "1.32"},
		"podcertificaterequest-cleaner-controller":                       {AddedInVersion: "1.34"},
	},
	"certificates/v1beta1": {
		"csrsigning": {},
		"kube-apiserver-serving-clustertrustbundle-publisher-controller": {AddedInVersion: "1.33"},
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
	"networking/v1": {
		"service-cidr-controller": {AddedInVersion: "1.33"},
	},
	"networking/v1alpha1": {
		"service-cidr-controller": {RemovedInVersion: "1.31"},
	},
	"networking/v1beta1": {
		"service-cidr-controller": {AddedInVersion: "1.31", RemovedInVersion: "1.33"},
	},
	"policy/v1": {
		"disruption": {},
	},
	"rbac/v1": {
		"clusterrole-aggregation": {},
	},
	"resource/v1": {
		"device-taint-eviction-controller": {AddedInVersion: "1.34"},
		"resourceclaim-controller":         {AddedInVersion: "1.34"},
	},
	"resource/v1alpha2": {
		"resource-claim-controller": {RemovedInVersion: "1.31"},
	},
	"resource/v1alpha3": {
		"resource-claim-controller": {AddedInVersion: "1.31", RemovedInVersion: "1.32"},
	},
	"resource/v1beta1": {
		"device-taint-eviction-controller": {AddedInVersion: "1.33", RemovedInVersion: "1.34"},
		"resource-claim-controller":        {AddedInVersion: "1.32", RemovedInVersion: "1.34"},
	},
	"storage/v1": {
		"selinux-warning-controller":                  {AddedInVersion: "1.32"},
		"volumeattributesclass-protection-controller": {AddedInVersion: "1.34"},
	},
	"storage/v1beta1": {
		"volumeattributesclass-protection-controller": {AddedInVersion: "1.32", RemovedInVersion: "1.34"},
	},
	"storagemigration/v1alpha1": {
		"storage-version-migrator-controller": {},
	},
	"v1": {
		"attachdetach":                         {},
		"bootstrapsigner":                      {},
		"cloud-node":                           {},
		"cloud-node-lifecycle":                 {},
		"cronjob":                              {},
		"csrapproving":                         {},
		"csrsigning":                           {},
		"daemonset":                            {},
		"deployment":                           {},
		"device-taint-eviction-controller":     {AddedInVersion: "1.33"},
		"disruption":                           {},
		"endpoint":                             {},
		"endpointslice":                        {},
		"endpointslicemirroring":               {},
		"ephemeral-volume":                     {},
		"horizontalpodautoscaling":             {},
		"job":                                  {},
		"legacy-service-account-token-cleaner": {},
		"namespace":                            {},
		"nodelifecycle":                        {},
		"persistentvolume-binder":              {},
		"persistentvolume-expander":            {},
		"podgc":                                {},
		"pv-protection":                        {},
		"pvc-protection":                       {},
		"replicaset":                           {},
		"replicationcontroller":                {},
		"resource-claim-controller":            {},
		"resourcequota":                        {},
		"root-ca-cert-publisher":               {},
		"route":                                {},
		"selinux-warning-controller":           {AddedInVersion: "1.32"},
		"service":                              {},
		"service-cidr-controller":              {},
		"serviceaccount":                       {},
		"serviceaccount-token":                 {},
		"statefulset":                          {},
		"taint-eviction-controller":            {},
		"tokencleaner":                         {},
		"ttl":                                  {},
		"ttl-after-finished":                   {},
		"volumeattributesclass-protection-controller": {AddedInVersion: "1.32"},
	},
}

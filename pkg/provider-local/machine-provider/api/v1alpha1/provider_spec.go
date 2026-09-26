// SPDX-FileCopyrightText: Contributors to the Gardener project
//
// SPDX-License-Identifier: Apache-2.0

package v1alpha1

import (
	localv1alpha1 "github.com/gardener/gardener/pkg/provider-local/apis/local/v1alpha1"
)

const (
	// V1Alpha1 is the API version.
	V1Alpha1 = "mcm.gardener.cloud/v1alpha1"
	// Provider is a constant for the provider name.
	Provider = "local"
)

// ProviderSpec is the spec to be used while parsing the calls.
type ProviderSpec struct {
	// APIVersion determines the API version for the provider APIs.
	APIVersion string `json:"apiVersion,omitempty"`
	// Namespace is the namespace in which the machine pods should be created.
	Namespace string `json:"namespace,omitempty"`
	// Image is the container image to use for the node.
	Image string `json:"image,omitempty"`
	// IPPoolNameV4 is the name of the crd.projectcalico.org/v1.IPPool that should be used for machine pods for IPv4
	// addresses.
	IPPoolNameV4 string `json:"ipPoolNameV4,omitempty"`
	// IPPoolNameV6 is the name of the crd.projectcalico.org/v1.IPPool that should be used for machine pods for IPv6
	// addresses.
	IPPoolNameV6 string `json:"ipPoolNameV6,omitempty"`
}

const (
	// LabelZone is the label key on machine pods for the zone of the machine (taken from the node template of the
	// machine class). The well-known topology.kubernetes.io/zone label key cannot be used because the kube-apiserver
	// overwrites it with the zone of the node the pod is scheduled to (PodTopologyLabels admission plugin).
	LabelZone = localv1alpha1.GroupName + "/zone"
	// LabelRegion is the label key on machine pods for the region of the machine (taken from the node template of the
	// machine class).
	LabelRegion = localv1alpha1.GroupName + "/region"
	// LabelInstanceType is the label key on machine pods for the instance type of the machine (taken from the node
	// template of the machine class).
	LabelInstanceType = localv1alpha1.GroupName + "/instance-type"
)

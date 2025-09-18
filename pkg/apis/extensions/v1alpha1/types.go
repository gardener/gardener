// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/sets"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
)

// AllExtensionKinds contains all supported extension kinds.
var AllExtensionKinds = sets.New[string](
	BackupBucketResource,
	BackupEntryResource,
	BastionResource,
	ContainerRuntimeResource,
	ControlPlaneResource,
	DNSRecordResource,
	ExtensionResource,
	InfrastructureResource,
	NetworkResource,
	OperatingSystemConfigResource,
	WorkerResource,
)

// Status is the status of an Object.
type Status interface {
	// GetProviderStatus retrieves the provider status.
	GetProviderStatus() *runtime.RawExtension
	// GetConditions retrieves the Conditions of a status.
	// Conditions may be nil.
	GetConditions() []gardencorev1beta1.Condition
	// SetConditions sets the Conditions of a status.
	SetConditions([]gardencorev1beta1.Condition)
	// GetLastOperation retrieves the LastOperation of a status.
	// LastOperation may be nil.
	GetLastOperation() *gardencorev1beta1.LastOperation
	// SetLastOperation sets the LastOperation of a status.
	SetLastOperation(*gardencorev1beta1.LastOperation)
	// GetObservedGeneration retrieves the last generation observed by the extension controller.
	GetObservedGeneration() int64
	// SetObservedGeneration sets the ObservedGeneration of a status.
	SetObservedGeneration(int64)
	// GetLastError retrieves the LastError of a status.
	// LastError may be nil.
	GetLastError() *gardencorev1beta1.LastError
	// SetLastError sets the LastError of a status.
	SetLastError(*gardencorev1beta1.LastError)
	// GetState retrieves the State of the extension
	GetState() *runtime.RawExtension
	// SetState sets the State of the extension
	SetState(state *runtime.RawExtension)
	// GetResources retrieves the list of named resource references referred to in the State by their names.
	GetResources() []gardencorev1beta1.NamedResourceReference
	// SetResources sets a list of named resource references in the Status, that are referred by
	// their names in the State.
	SetResources(namedResourceReferences []gardencorev1beta1.NamedResourceReference)
}

// Spec is the spec section of an Object.
type Spec interface {
	// GetExtensionType retrieves the extension type.
	GetExtensionType() string
	// GetExtensionClass retrieves the extension class.
	GetExtensionClass() *ExtensionClass
	// GetExtensionPurpose retrieves the extension purpose.
	GetExtensionPurpose() *string
	// GetProviderConfig retrieves the provider config.
	GetProviderConfig() *runtime.RawExtension
}

// Object is an extension object resource.
type Object interface {
	metav1.Object
	runtime.Object

	// GetExtensionSpec retrieves the object's spec.
	GetExtensionSpec() Spec
	// GetExtensionStatus retrieves the object's status.
	GetExtensionStatus() Status
}

// ShootAlphaCSIMigrationKubernetesVersion is a constant for an annotation on the Shoot resource stating the Kubernetes
// version for which the CSI migration shall be enabled.
// Note that this annotation is alpha and can be removed anytime without further notice. Only use it if you know
// what you do.
const ShootAlphaCSIMigrationKubernetesVersion = "alpha.csimigration.shoot.extensions.gardener.cloud/kubernetes-version"

// IPFamily is a type for specifying an IP protocol version to use in Gardener clusters.
type IPFamily string

const (
	// IPFamilyIPv4 is the IPv4 IP family.
	IPFamilyIPv4 IPFamily = "IPv4"
	// IPFamilyIPv6 is the IPv6 IP family.
	IPFamilyIPv6 IPFamily = "IPv6"
)

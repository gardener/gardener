// SPDX-FileCopyrightText: 2019 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package v1alpha1

import (
	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"

	dnsv1alpha1 "github.com/gardener/external-dns-management/pkg/apis/dns/v1alpha1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/sets"
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
	// GetObservedGeneration retrieves the last generation observed by the extension controller.
	GetObservedGeneration() int64
	// GetLastError retrieves the LastError of a status.
	// LastError may be nil.
	GetLastError() *gardencorev1beta1.LastError
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

// ExtensionKinds contains all supported extension kinds.
var ExtensionKinds = sets.NewString(
	BackupBucketResource,
	BackupEntryResource,
	ContainerRuntimeResource,
	ControlPlaneResource,
	dnsv1alpha1.DNSProviderKind,
	ExtensionResource,
	InfrastructureResource,
	NetworkResource,
	OperatingSystemConfigResource,
	WorkerResource,
)

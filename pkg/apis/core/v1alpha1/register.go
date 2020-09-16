// SPDX-FileCopyrightText: 2018 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

// GroupName is the name of the core API group.
const GroupName = "core.gardener.cloud"

// SchemeGroupVersion is group version used to register these objects
var SchemeGroupVersion = schema.GroupVersion{Group: GroupName, Version: "v1alpha1"}

// Kind takes an unqualified kind and returns a Group qualified GroupKind.
func Kind(kind string) schema.GroupKind {
	return SchemeGroupVersion.WithKind(kind).GroupKind()
}

// Resource takes an unqualified resource and returns a Group qualified GroupResource.
func Resource(resource string) schema.GroupResource {
	return SchemeGroupVersion.WithResource(resource).GroupResource()
}

var (
	// SchemeBuilder is a new Scheme Builder which registers our API.
	SchemeBuilder      = runtime.NewSchemeBuilder(addKnownTypes, addDefaultingFuncs, addConversionFuncs)
	localSchemeBuilder = &SchemeBuilder
	// AddToScheme is a reference to the Scheme Builder's AddToScheme function.
	AddToScheme = SchemeBuilder.AddToScheme
)

// Adds the list of known types to the given scheme.
func addKnownTypes(scheme *runtime.Scheme) error {
	scheme.AddKnownTypes(SchemeGroupVersion,
		&BackupBucket{},
		&BackupBucketList{},
		&BackupEntry{},
		&BackupEntryList{},
		&CloudProfile{},
		&CloudProfileList{},
		&ControllerRegistration{},
		&ControllerRegistrationList{},
		&ControllerInstallation{},
		&ControllerInstallationList{},
		&Plant{},
		&PlantList{},
		&Project{},
		&ProjectList{},
		&Quota{},
		&QuotaList{},
		&SecretBinding{},
		&SecretBindingList{},
		&Seed{},
		&SeedList{},
		&ShootState{},
		&ShootStateList{},
		&Shoot{},
		&ShootList{},
	)
	metav1.AddToGroupVersion(scheme, SchemeGroupVersion)
	return nil
}

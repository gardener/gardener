// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"

	"github.com/gardener/gardener/pkg/apis/extensions"
)

// SchemeGroupVersion is group version used to register these objects
var SchemeGroupVersion = schema.GroupVersion{Group: extensions.GroupName, Version: "v1alpha1"}

// Kind takes an unqualified kind and returns back a Group qualified GroupKind
func Kind(kind string) schema.GroupKind {
	return SchemeGroupVersion.WithKind(kind).GroupKind()
}

// Resource takes an unqualified resource and returns a Group qualified GroupResource
func Resource(resource string) schema.GroupResource {
	return SchemeGroupVersion.WithResource(resource).GroupResource()
}

var (
	// SchemeBuilder is a new Scheme Builder which registers our API.
	SchemeBuilder = runtime.NewSchemeBuilder(addKnownTypes)
	// AddToScheme is a reference to the Scheme Builder's AddToScheme function.
	AddToScheme = SchemeBuilder.AddToScheme
)

// Adds the list of known types to Scheme.
func addKnownTypes(scheme *runtime.Scheme) error {
	scheme.AddKnownTypes(SchemeGroupVersion,
		&BackupBucket{},
		&BackupBucketList{},
		&BackupEntry{},
		&BackupEntryList{},
		&Bastion{},
		&BastionList{},
		&Cluster{},
		&ClusterList{},
		&ContainerRuntime{},
		&ContainerRuntimeList{},
		&ControlPlane{},
		&ControlPlaneList{},
		&DNSRecord{},
		&DNSRecordList{},
		&Extension{},
		&ExtensionList{},
		&Infrastructure{},
		&InfrastructureList{},
		&Network{},
		&NetworkList{},
		&OperatingSystemConfig{},
		&OperatingSystemConfigList{},
		&Worker{},
		&WorkerList{},
	)
	metav1.AddToGroupVersion(scheme, SchemeGroupVersion)

	return nil
}

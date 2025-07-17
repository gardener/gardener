// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package graph

// VertexType is a type for specific vertices.
type VertexType byte

const (
	// VertexTypeBackupBucket is a constant for a 'BackupBucket' vertex.
	VertexTypeBackupBucket VertexType = iota
	// VertexTypeBackupEntry is a constant for a 'BackupEntry' vertex.
	VertexTypeBackupEntry
	// VertexTypeBastion is a constant for a 'Bastion' vertex.
	VertexTypeBastion
	// VertexTypeCertificateSigningRequest is a constant for a 'CertificateSigningRequest' vertex.
	VertexTypeCertificateSigningRequest
	// VertexTypeCloudProfile is a constant for a 'CloudProfile' vertex.
	VertexTypeCloudProfile
	// VertexTypeNamespacedCloudProfile is a constant for a 'NamespacedCloudProfile' vertex.
	VertexTypeNamespacedCloudProfile
	// VertexTypeClusterRoleBinding is a constant for a 'ClusterRoleBinding' vertex.
	VertexTypeClusterRoleBinding
	// VertexTypeConfigMap is a constant for a 'ConfigMap' vertex.
	VertexTypeConfigMap
	// VertexTypeControllerDeployment is a constant for a 'ControllerDeployment' vertex.
	VertexTypeControllerDeployment
	// VertexTypeControllerInstallation is a constant for a 'ControllerInstallation' vertex.
	VertexTypeControllerInstallation
	// VertexTypeControllerRegistration is a constant for a 'ControllerRegistration' vertex.
	VertexTypeControllerRegistration
	// VertexTypeExposureClass is a constant for a 'ExposureClass' vertex.
	VertexTypeExposureClass
	// VertexTypeInternalSecret is a constant for a 'InternalSecret' vertex.
	VertexTypeInternalSecret
	// VertexTypeLease is a constant for a 'Lease' vertex.
	VertexTypeLease
	// VertexTypeManagedSeed is a constant for a 'ManagedSeed' vertex.
	VertexTypeManagedSeed
	// VertexTypeNamespace is a constant for a 'Namespace' vertex.
	VertexTypeNamespace
	// VertexTypeProject is a constant for a 'Project' vertex.
	VertexTypeProject
	// VertexTypeSecret is a constant for a 'Secret' vertex.
	VertexTypeSecret
	// VertexTypeSecretBinding is a constant for a 'SecretBinding' vertex.
	VertexTypeSecretBinding
	// VertexTypeSeed is a constant for a 'Seed' vertex.
	VertexTypeSeed
	// VertexTypeServiceAccount is a constant for a 'ServiceAccount' vertex.
	VertexTypeServiceAccount
	// VertexTypeShoot is a constant for a 'Shoot' vertex.
	VertexTypeShoot
	// VertexTypeShootState is a constant for a 'ShootState' vertex.
	VertexTypeShootState
	// VertexTypeGardenlet is a constant for a 'Gardenlet' vertex.
	VertexTypeGardenlet
	// VertexTypeCredentialsBinding is a constant for a 'CredentialsBinding' vertex.
	VertexTypeCredentialsBinding
	// VertexTypeWorkloadIdentity is a constant for a 'WorkloadIdentity' vertex.
	VertexTypeWorkloadIdentity
)

var vertexTypes = map[VertexType]string{
	VertexTypeBackupBucket:              "BackupBucket",
	VertexTypeBackupEntry:               "BackupEntry",
	VertexTypeBastion:                   "Bastion",
	VertexTypeCertificateSigningRequest: "CertificateSigningRequest",
	VertexTypeCloudProfile:              "CloudProfile",
	VertexTypeNamespacedCloudProfile:    "NamespacedCloudProfile",
	VertexTypeClusterRoleBinding:        "ClusterRoleBinding",
	VertexTypeConfigMap:                 "ConfigMap",
	VertexTypeControllerDeployment:      "ControllerDeployment",
	VertexTypeControllerInstallation:    "ControllerInstallation",
	VertexTypeControllerRegistration:    "ControllerRegistration",
	VertexTypeExposureClass:             "ExposureClass",
	VertexTypeGardenlet:                 "Gardenlet",
	VertexTypeLease:                     "Lease",
	VertexTypeManagedSeed:               "ManagedSeed",
	VertexTypeNamespace:                 "Namespace",
	VertexTypeProject:                   "Project",
	VertexTypeSecret:                    "Secret",
	VertexTypeSecretBinding:             "SecretBinding",
	VertexTypeSeed:                      "Seed",
	VertexTypeServiceAccount:            "ServiceAccount",
	VertexTypeShoot:                     "Shoot",
	VertexTypeShootState:                "ShootState",
	VertexTypeCredentialsBinding:        "CredentialsBinding",
	VertexTypeWorkloadIdentity:          "WorkloadIdentity",
}

type vertex struct {
	vertexType VertexType
	namespace  string
	name       string
	id         int64
}

func newVertex(vertexType VertexType, namespace, name string, id int64) *vertex {
	return &vertex{
		vertexType: vertexType,
		name:       name,
		namespace:  namespace,
		id:         id,
	}
}

func (v *vertex) ID() int64 {
	return v.id
}

func (v *vertex) String() string {
	var namespace string
	if len(v.namespace) > 0 {
		namespace = v.namespace + "/"
	}
	return vertexTypes[v.vertexType] + ":" + namespace + v.name
}

// typeVertexMapping is a map from type -> namespace -> name -> vertex.
type typeVertexMapping map[VertexType]namespaceVertexMapping

// namespaceVertexMapping is a map of namespace -> name -> vertex.
type namespaceVertexMapping map[string]nameVertexMapping

// nameVertexMapping is a map of name -> vertex.
type nameVertexMapping map[string]*vertex

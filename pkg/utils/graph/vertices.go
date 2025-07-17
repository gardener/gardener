// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package graph

import (
	certificatesv1 "k8s.io/api/certificates/v1"
	coordinationv1 "k8s.io/api/coordination/v1"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	operationsv1alpha1 "github.com/gardener/gardener/pkg/apis/operations/v1alpha1"
	securityv1alpha1 "github.com/gardener/gardener/pkg/apis/security/v1alpha1"
	seedmanagementv1alpha1 "github.com/gardener/gardener/pkg/apis/seedmanagement/v1alpha1"
)

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

// KindObject contains the object kind and a function for creating a new client.Object.
type KindObject struct {
	Kind          string
	NewObjectFunc func() client.Object
}

// VertexTypes maps a vertex type to its kind and a function for creation of a new client.Object.
var VertexTypes = map[VertexType]KindObject{
	VertexTypeBackupBucket:              {Kind: "BackupBucket", NewObjectFunc: func() client.Object { return &gardencorev1beta1.BackupBucket{} }},
	VertexTypeBackupEntry:               {Kind: "BackupEntry", NewObjectFunc: func() client.Object { return &gardencorev1beta1.BackupEntry{} }},
	VertexTypeBastion:                   {Kind: "Bastion", NewObjectFunc: func() client.Object { return &operationsv1alpha1.Bastion{} }},
	VertexTypeCertificateSigningRequest: {Kind: "CertificateSigningRequest", NewObjectFunc: func() client.Object { return &certificatesv1.CertificateSigningRequest{} }},
	VertexTypeCredentialsBinding:        {Kind: "CredentialsBinding", NewObjectFunc: func() client.Object { return &securityv1alpha1.CredentialsBinding{} }},
	VertexTypeCloudProfile:              {Kind: "CloudProfile", NewObjectFunc: func() client.Object { return &gardencorev1beta1.CloudProfile{} }},
	VertexTypeNamespacedCloudProfile:    {Kind: "NamespacedCloudProfile", NewObjectFunc: func() client.Object { return &gardencorev1beta1.NamespacedCloudProfile{} }},
	VertexTypeClusterRoleBinding:        {Kind: "ClusterRoleBinding", NewObjectFunc: func() client.Object { return &rbacv1.ClusterRoleBinding{} }},
	VertexTypeConfigMap:                 {Kind: "ConfigMap", NewObjectFunc: func() client.Object { return &corev1.ConfigMap{} }},
	VertexTypeControllerDeployment:      {Kind: "ControllerDeployment", NewObjectFunc: func() client.Object { return &gardencorev1beta1.ControllerDeployment{} }},
	VertexTypeControllerInstallation:    {Kind: "ControllerInstallation", NewObjectFunc: func() client.Object { return &gardencorev1beta1.ControllerInstallation{} }},
	VertexTypeControllerRegistration:    {Kind: "ControllerRegistration", NewObjectFunc: func() client.Object { return &gardencorev1beta1.ControllerRegistration{} }},
	VertexTypeExposureClass:             {Kind: "ExposureClass", NewObjectFunc: func() client.Object { return &gardencorev1beta1.ExposureClass{} }},
	VertexTypeGardenlet:                 {Kind: "Gardenlet", NewObjectFunc: func() client.Object { return &seedmanagementv1alpha1.Gardenlet{} }},
	VertexTypeInternalSecret:            {Kind: "InternalSecret", NewObjectFunc: func() client.Object { return &gardencorev1beta1.InternalSecret{} }},
	VertexTypeLease:                     {Kind: "Lease", NewObjectFunc: func() client.Object { return &coordinationv1.Lease{} }},
	VertexTypeManagedSeed:               {Kind: "ManagedSeed", NewObjectFunc: func() client.Object { return &seedmanagementv1alpha1.ManagedSeed{} }},
	VertexTypeNamespace:                 {Kind: "Namespace", NewObjectFunc: func() client.Object { return &corev1.Namespace{} }},
	VertexTypeProject:                   {Kind: "Project", NewObjectFunc: func() client.Object { return &gardencorev1beta1.Project{} }},
	VertexTypeSecret:                    {Kind: "Secret", NewObjectFunc: func() client.Object { return &corev1.Secret{} }},
	VertexTypeSecretBinding:             {Kind: "SecretBinding", NewObjectFunc: func() client.Object { return &gardencorev1beta1.SecretBinding{} }},
	VertexTypeSeed:                      {Kind: "Seed", NewObjectFunc: func() client.Object { return &gardencorev1beta1.Seed{} }},
	VertexTypeServiceAccount:            {Kind: "ServiceAccount", NewObjectFunc: func() client.Object { return &corev1.ServiceAccount{} }},
	VertexTypeShoot:                     {Kind: "Shoot", NewObjectFunc: func() client.Object { return &gardencorev1beta1.Shoot{} }},
	VertexTypeShootState:                {Kind: "ShootState", NewObjectFunc: func() client.Object { return &gardencorev1beta1.ShootState{} }},
	VertexTypeWorkloadIdentity:          {Kind: "WorkloadIdentity", NewObjectFunc: func() client.Object { return &securityv1alpha1.WorkloadIdentity{} }},
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
	return VertexTypes[v.vertexType].Kind + ":" + namespace + v.name
}

// typeVertexMapping is a map from type -> namespace -> name -> vertex.
type typeVertexMapping map[VertexType]namespaceVertexMapping

// namespaceVertexMapping is a map of namespace -> name -> vertex.
type namespaceVertexMapping map[string]nameVertexMapping

// nameVertexMapping is a map of name -> vertex.
type nameVertexMapping map[string]*vertex

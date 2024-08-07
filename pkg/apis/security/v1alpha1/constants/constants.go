// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package constants

const (
	groupName              = "security.gardener.cloud"
	workloadIdentityPrefix = "workloadidentity." + groupName

	// AnnotationWorkloadIdentityNamespace is an annotation key used to indicate the namespace of the origin WorkloadIdentity.
	AnnotationWorkloadIdentityNamespace = workloadIdentityPrefix + "/namespace"
	// AnnotationWorkloadIdentityName is an annotation key used to indicate the name of the origin WorkloadIdentity.
	AnnotationWorkloadIdentityName = workloadIdentityPrefix + "/name"
	// AnnotationWorkloadIdentityContextObject is an annotation key used to indicate the context object for which the origin WorkloadIdentity will be used.
	AnnotationWorkloadIdentityContextObject = workloadIdentityPrefix + "/context-object"

	// LabelPurpose is a label used to indicate the purpose of the labeled resource.
	// Specific values might cause controllers to act on the said object.
	LabelPurpose = groupName + "/purpose"

	// LabelPurposeWorkloadIdentityTokenRequestor is a label value set on secrets indicating that a token should for a specific workload identity
	// should be requested and populated as data in the labeled secret.
	LabelPurposeWorkloadIdentityTokenRequestor = "workload-identity-token-requestor"

	// LabelWorkloadIdentityProvider is a label key indicating the target system type before which workload identity tokens will be presented.
	LabelWorkloadIdentityProvider = workloadIdentityPrefix + "/provider"
)

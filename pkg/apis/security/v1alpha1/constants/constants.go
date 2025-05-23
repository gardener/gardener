// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package constants

const (
	groupName = "security.gardener.cloud"

	// WorkloadIdentityPrefix is used to prefix label and annotation keys used in relation to WorkloadIdentities.
	WorkloadIdentityPrefix = "workloadidentity." + groupName + "/"

	// AnnotationWorkloadIdentityNamespace is an annotation key used to indicate the namespace of the origin WorkloadIdentity.
	AnnotationWorkloadIdentityNamespace = WorkloadIdentityPrefix + "namespace"
	// AnnotationWorkloadIdentityName is an annotation key used to indicate the name of the origin WorkloadIdentity.
	AnnotationWorkloadIdentityName = WorkloadIdentityPrefix + "name"
	// AnnotationWorkloadIdentityContextObject is an annotation key used to indicate the context object for which the origin WorkloadIdentity will be used.
	AnnotationWorkloadIdentityContextObject = WorkloadIdentityPrefix + "context-object"
	// AnnotationWorkloadIdentityTokenRenewTimestamp is an annotation key used to indicate
	// the timestamp after which the workload identity token has to be renewed.
	AnnotationWorkloadIdentityTokenRenewTimestamp = WorkloadIdentityPrefix + "token-renew-timestamp"

	// DataKeyToken is the data key of a secret whose value contains a workload identity token.
	DataKeyToken = "token"
	// DataKeyConfig is the data key of a secret whose value contains a workload identity provider configuration.
	DataKeyConfig = "config"

	// LabelPurpose is a label used to indicate the purpose of the labeled resource.
	// Specific values might cause controllers to act on the said object.
	LabelPurpose = groupName + "/purpose"

	// LabelPurposeWorkloadIdentityTokenRequestor is a label value set on secrets indicating that a token should for a specific workload identity
	// should be requested and populated as data in the labeled secret.
	LabelPurposeWorkloadIdentityTokenRequestor = "workload-identity-token-requestor"

	// LabelWorkloadIdentityProvider is a label key indicating the target system type before which workload identity tokens will be presented.
	LabelWorkloadIdentityProvider = WorkloadIdentityPrefix + "provider"
)

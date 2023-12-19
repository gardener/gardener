//go:build !ignore_autogenerated
// +build !ignore_autogenerated

// SPDX-FileCopyrightText: 2023 SAP SE or an SAP affiliate company and Gardener contributors
// SPDX-License-Identifier: Apache-2.0

// Code generated by defaulter-gen. DO NOT EDIT.

package v1alpha1

import (
	runtime "k8s.io/apimachinery/pkg/runtime"
)

// RegisterDefaults adds defaulters functions to the given scheme.
// Public to allow building arbitrary schemes.
// All generated defaulters are covering - they call all nested defaulters.
func RegisterDefaults(scheme *runtime.Scheme) error {
	scheme.AddTypeDefaultingFunc(&ResourceManagerConfiguration{}, func(obj interface{}) {
		SetObjectDefaults_ResourceManagerConfiguration(obj.(*ResourceManagerConfiguration))
	})
	return nil
}

func SetObjectDefaults_ResourceManagerConfiguration(in *ResourceManagerConfiguration) {
	SetDefaults_ResourceManagerConfiguration(in)
	SetDefaults_ClientConnection(&in.SourceClientConnection)
	SetDefaults_ClientConnectionConfiguration(&in.SourceClientConnection.ClientConnectionConfiguration)
	if in.TargetClientConnection != nil {
		SetDefaults_ClientConnection(in.TargetClientConnection)
		SetDefaults_ClientConnectionConfiguration(&in.TargetClientConnection.ClientConnectionConfiguration)
	}
	SetDefaults_LeaderElectionConfiguration(&in.LeaderElection)
	SetDefaults_ServerConfiguration(&in.Server)
	SetDefaults_ResourceManagerControllerConfiguration(&in.Controllers)
	SetDefaults_GarbageCollectorControllerConfig(&in.Controllers.GarbageCollector)
	SetDefaults_HealthControllerConfig(&in.Controllers.Health)
	SetDefaults_KubeletCSRApproverControllerConfig(&in.Controllers.KubeletCSRApprover)
	SetDefaults_ManagedResourceControllerConfig(&in.Controllers.ManagedResource)
	SetDefaults_NetworkPolicyControllerConfig(&in.Controllers.NetworkPolicy)
	SetDefaults_NodeControllerConfig(&in.Controllers.Node)
	SetDefaults_SecretControllerConfig(&in.Controllers.Secret)
	SetDefaults_TokenInvalidatorControllerConfig(&in.Controllers.TokenInvalidator)
	SetDefaults_TokenRequestorControllerConfig(&in.Controllers.TokenRequestor)
	SetDefaults_PodSchedulerNameWebhookConfig(&in.Webhooks.PodSchedulerName)
	SetDefaults_ProjectedTokenMountWebhookConfig(&in.Webhooks.ProjectedTokenMount)
}

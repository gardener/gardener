//go:build !ignore_autogenerated
// +build !ignore_autogenerated

// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
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
	scheme.AddTypeDefaultingFunc(&SchedulerConfiguration{}, func(obj interface{}) { SetObjectDefaults_SchedulerConfiguration(obj.(*SchedulerConfiguration)) })
	return nil
}

func SetObjectDefaults_SchedulerConfiguration(in *SchedulerConfiguration) {
	SetDefaults_SchedulerConfiguration(in)
	SetDefaults_ClientConnectionConfiguration(&in.ClientConnection)
	if in.LeaderElection != nil {
		SetDefaults_LeaderElectionConfiguration(in.LeaderElection)
	}
	SetDefaults_ServerConfiguration(&in.Server)
	SetDefaults_SchedulerControllerConfiguration(&in.Schedulers)
}

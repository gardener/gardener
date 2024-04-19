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
	scheme.AddTypeDefaultingFunc(&ManagedSeed{}, func(obj interface{}) { SetObjectDefaults_ManagedSeed(obj.(*ManagedSeed)) })
	scheme.AddTypeDefaultingFunc(&ManagedSeedList{}, func(obj interface{}) { SetObjectDefaults_ManagedSeedList(obj.(*ManagedSeedList)) })
	scheme.AddTypeDefaultingFunc(&ManagedSeedSet{}, func(obj interface{}) { SetObjectDefaults_ManagedSeedSet(obj.(*ManagedSeedSet)) })
	scheme.AddTypeDefaultingFunc(&ManagedSeedSetList{}, func(obj interface{}) { SetObjectDefaults_ManagedSeedSetList(obj.(*ManagedSeedSetList)) })
	return nil
}

func SetObjectDefaults_ManagedSeed(in *ManagedSeed) {
	SetDefaults_ManagedSeed(in)
	if in.Spec.Gardenlet != nil {
		if in.Spec.Gardenlet.Deployment != nil {
			SetDefaults_GardenletDeployment(in.Spec.Gardenlet.Deployment)
			if in.Spec.Gardenlet.Deployment.Image != nil {
				SetDefaults_Image(in.Spec.Gardenlet.Deployment.Image)
			}
		}
	}
}

func SetObjectDefaults_ManagedSeedList(in *ManagedSeedList) {
	for i := range in.Items {
		a := &in.Items[i]
		SetObjectDefaults_ManagedSeed(a)
	}
}

func SetObjectDefaults_ManagedSeedSet(in *ManagedSeedSet) {
	SetDefaults_ManagedSeedSet(in)
	if in.Spec.Template.Spec.Gardenlet != nil {
		if in.Spec.Template.Spec.Gardenlet.Deployment != nil {
			SetDefaults_GardenletDeployment(in.Spec.Template.Spec.Gardenlet.Deployment)
			if in.Spec.Template.Spec.Gardenlet.Deployment.Image != nil {
				SetDefaults_Image(in.Spec.Template.Spec.Gardenlet.Deployment.Image)
			}
		}
	}
	if in.Spec.UpdateStrategy != nil {
		SetDefaults_UpdateStrategy(in.Spec.UpdateStrategy)
		if in.Spec.UpdateStrategy.RollingUpdate != nil {
			SetDefaults_RollingUpdateStrategy(in.Spec.UpdateStrategy.RollingUpdate)
		}
	}
}

func SetObjectDefaults_ManagedSeedSetList(in *ManagedSeedSetList) {
	for i := range in.Items {
		a := &in.Items[i]
		SetObjectDefaults_ManagedSeedSet(a)
	}
}
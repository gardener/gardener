//go:build !ignore_autogenerated
// +build !ignore_autogenerated

/*
Copyright (c) SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

     http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

// Code generated by conversion-gen. DO NOT EDIT.

package v1alpha1

import (
	unsafe "unsafe"

	conversion "k8s.io/apimachinery/pkg/conversion"
	runtime "k8s.io/apimachinery/pkg/runtime"

	core "github.com/gardener/gardener/pkg/apis/core"
	v1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	shoottolerationrestriction "github.com/gardener/gardener/plugin/pkg/shoot/tolerationrestriction/apis/shoottolerationrestriction"
)

func init() {
	localSchemeBuilder.Register(RegisterConversions)
}

// RegisterConversions adds conversion functions to the given scheme.
// Public to allow building arbitrary schemes.
func RegisterConversions(s *runtime.Scheme) error {
	if err := s.AddGeneratedConversionFunc((*Configuration)(nil), (*shoottolerationrestriction.Configuration)(nil), func(a, b interface{}, scope conversion.Scope) error {
		return Convert_v1alpha1_Configuration_To_shoottolerationrestriction_Configuration(a.(*Configuration), b.(*shoottolerationrestriction.Configuration), scope)
	}); err != nil {
		return err
	}
	if err := s.AddGeneratedConversionFunc((*shoottolerationrestriction.Configuration)(nil), (*Configuration)(nil), func(a, b interface{}, scope conversion.Scope) error {
		return Convert_shoottolerationrestriction_Configuration_To_v1alpha1_Configuration(a.(*shoottolerationrestriction.Configuration), b.(*Configuration), scope)
	}); err != nil {
		return err
	}
	return nil
}

func autoConvert_v1alpha1_Configuration_To_shoottolerationrestriction_Configuration(in *Configuration, out *shoottolerationrestriction.Configuration, s conversion.Scope) error {
	out.Defaults = *(*[]core.Toleration)(unsafe.Pointer(&in.Defaults))
	out.Whitelist = *(*[]core.Toleration)(unsafe.Pointer(&in.Whitelist))
	return nil
}

// Convert_v1alpha1_Configuration_To_shoottolerationrestriction_Configuration is an autogenerated conversion function.
func Convert_v1alpha1_Configuration_To_shoottolerationrestriction_Configuration(in *Configuration, out *shoottolerationrestriction.Configuration, s conversion.Scope) error {
	return autoConvert_v1alpha1_Configuration_To_shoottolerationrestriction_Configuration(in, out, s)
}

func autoConvert_shoottolerationrestriction_Configuration_To_v1alpha1_Configuration(in *shoottolerationrestriction.Configuration, out *Configuration, s conversion.Scope) error {
	out.Defaults = *(*[]v1beta1.Toleration)(unsafe.Pointer(&in.Defaults))
	out.Whitelist = *(*[]v1beta1.Toleration)(unsafe.Pointer(&in.Whitelist))
	return nil
}

// Convert_shoottolerationrestriction_Configuration_To_v1alpha1_Configuration is an autogenerated conversion function.
func Convert_shoottolerationrestriction_Configuration_To_v1alpha1_Configuration(in *shoottolerationrestriction.Configuration, out *Configuration, s conversion.Scope) error {
	return autoConvert_shoottolerationrestriction_Configuration_To_v1alpha1_Configuration(in, out, s)
}

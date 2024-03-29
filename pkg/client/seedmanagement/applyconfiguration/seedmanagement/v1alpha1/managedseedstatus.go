/*
Copyright SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file

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

// Code generated by applyconfiguration-gen. DO NOT EDIT.

package v1alpha1

import (
	v1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
)

// ManagedSeedStatusApplyConfiguration represents an declarative configuration of the ManagedSeedStatus type for use
// with apply.
type ManagedSeedStatusApplyConfiguration struct {
	Conditions         []v1beta1.Condition `json:"conditions,omitempty"`
	ObservedGeneration *int64              `json:"observedGeneration,omitempty"`
}

// ManagedSeedStatusApplyConfiguration constructs an declarative configuration of the ManagedSeedStatus type for use with
// apply.
func ManagedSeedStatus() *ManagedSeedStatusApplyConfiguration {
	return &ManagedSeedStatusApplyConfiguration{}
}

// WithConditions adds the given value to the Conditions field in the declarative configuration
// and returns the receiver, so that objects can be build by chaining "With" function invocations.
// If called multiple times, values provided by each call will be appended to the Conditions field.
func (b *ManagedSeedStatusApplyConfiguration) WithConditions(values ...v1beta1.Condition) *ManagedSeedStatusApplyConfiguration {
	for i := range values {
		b.Conditions = append(b.Conditions, values[i])
	}
	return b
}

// WithObservedGeneration sets the ObservedGeneration field in the declarative configuration to the given value
// and returns the receiver, so that objects can be built by chaining "With" function invocations.
// If called multiple times, the ObservedGeneration field is set to the value of the last call.
func (b *ManagedSeedStatusApplyConfiguration) WithObservedGeneration(value int64) *ManagedSeedStatusApplyConfiguration {
	b.ObservedGeneration = &value
	return b
}

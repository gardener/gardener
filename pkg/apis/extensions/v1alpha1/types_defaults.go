// Copyright (c) 2019 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package v1alpha1

import (
	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	runtime "k8s.io/apimachinery/pkg/runtime"
)

// DefaultSpec contains common status fields for every extension resource.
type DefaultSpec struct {
	// Type contains the instance of the resource's kind.
	Type string `json:"type"`
	// ProviderConfig is the provider specific configuration.
	// +optional

	ProviderConfig *runtime.RawExtension `json:"providerConfig,omitempty"`
}

// GetExtensionType implements Spec.
func (d *DefaultSpec) GetExtensionType() string {
	return d.Type
}

// GetExtensionPurpose implements Spec.
func (d *DefaultSpec) GetExtensionPurpose() *string {
	return nil
}

// GetProviderConfig implements Spec.
func (d *DefaultSpec) GetProviderConfig() *runtime.RawExtension {
	return d.ProviderConfig
}

// DefaultStatus contains common status fields for every extension resource.
type DefaultStatus struct {
	// ProviderStatus contains provider-specific status.
	// +optional
	ProviderStatus *runtime.RawExtension `json:"providerStatus,omitempty"`
	// Conditions represents the latest available observations of a Seed's current state.
	// +optional
	Conditions []gardencorev1beta1.Condition `json:"conditions,omitempty"`
	// LastError holds information about the last occurred error during an operation.
	// +optional
	LastError *gardencorev1beta1.LastError `json:"lastError,omitempty"`
	// LastOperation holds information about the last operation on the resource.
	// +optional
	LastOperation *gardencorev1beta1.LastOperation `json:"lastOperation,omitempty"`
	// ObservedGeneration is the most recent generation observed for this resource.
	ObservedGeneration int64 `json:"observedGeneration,omitempty"`
	// State can be filled by the operating controller with what ever data it needs.
	// +optional
	State *runtime.RawExtension `json:"state,omitempty"`
}

// GetProviderStatus implements Status.
func (d *DefaultStatus) GetProviderStatus() *runtime.RawExtension {
	return d.ProviderStatus
}

// GetConditions implements Status.
func (d *DefaultStatus) GetConditions() []gardencorev1beta1.Condition {
	return d.Conditions
}

// SetConditions implements Status.
func (d *DefaultStatus) SetConditions(c []gardencorev1beta1.Condition) {
	d.Conditions = c
}

// GetLastOperation implements Status.
func (d *DefaultStatus) GetLastOperation() LastOperation {
	if d.LastOperation == nil {
		return nil
	}
	return d.LastOperation
}

// GetLastError implements Status.
func (d *DefaultStatus) GetLastError() LastError {
	if d.LastError == nil {
		return nil
	}
	return d.LastError
}

// GetObservedGeneration implements Status.
func (d *DefaultStatus) GetObservedGeneration() int64 {
	return d.ObservedGeneration
}

// GetState implements Status.
func (d *DefaultStatus) GetState() *runtime.RawExtension {
	return d.State
}

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

import gardencorev1alpha1 "github.com/gardener/gardener/pkg/apis/core/v1alpha1"

// DefaultSpec contains common status fields for every extension resource.
type DefaultSpec struct {
	// Type contains the instance of the resource's kind.
	Type string `json:"type"`
}

// DefaultStatus contains common status fields for every extension resource.
type DefaultStatus struct {
	// Conditions represents the latest available observations of a Seed's current state.
	// +optional
	Conditions []gardencorev1alpha1.Condition `json:"conditions,omitempty"`
	// LastError holds information about the last occurred error during an operation.
	// +optional
	LastError *gardencorev1alpha1.LastError `json:"lastError,omitempty"`
	// LastOperation holds information about the last operation on the resource.
	// +optional
	LastOperation *gardencorev1alpha1.LastOperation `json:"lastOperation,omitempty"`
	// ObservedGeneration is the most recent generation observed for this resource.
	ObservedGeneration int64 `json:"observedGeneration,omitempty"`
	// State can be filled by the operating controller with what ever data it needs.
	State string `json:"state,omitempty"`
}

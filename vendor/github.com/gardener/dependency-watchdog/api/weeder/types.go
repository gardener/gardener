// Copyright 2022 SAP SE or an SAP affiliate company
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

package weeder

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// Config provides typed access weeder configuration
type Config struct {
	// WatchDuration is the duration for which all dependent pods for a service under surveillance will be watched after the service has recovered.
	// If the dependent pods have not transitioned to CrashLoopBackOff in this duration then it is assumed that they will not enter that state.
	WatchDuration *metav1.Duration `json:"watchDuration,omitempty"`
	// ServicesAndDependantSelectors is a map whose key is the service name and the value is a DependantSelectors
	ServicesAndDependantSelectors map[string]DependantSelectors `json:"servicesAndDependantSelectors"`
}

// DependantSelectors encapsulates LabelSelector's used to identify dependants for a service.
// [Trivia]: Dependent is used as an adjective and dependant is used as a noun. This explains the choice of the variant.
type DependantSelectors struct {
	// PodSelectors is a slice of LabelSelector's used to identify dependant pods
	PodSelectors []*metav1.LabelSelector `json:"podSelectors"`
}

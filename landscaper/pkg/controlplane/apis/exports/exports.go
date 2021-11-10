// Copyright (c) 2021 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

package api

// Exports defines the structure for the exported data which might be consumed by other components.
type Exports struct {
	GardenerIdentity        string `json:"gardenerIdentity" yaml:"gardenerIdentity"`
	OpenVPNDiffieHellmanKey string `json:"openVPNDiffieHellmanKey" yaml:"openVPNDiffieHellmanKey"`

	GardenerAPIServerCACrt string `json:"gardenerAPIServerCACrt" yaml:"gardenerAPIServerCACrt"`
	GardenerAPIServerCAKey string `json:"gardenerAPIServerCAKey" yaml:"gardenerAPIServerCAKey"`

	GardenerAPIServerTLSServingCrt string `json:"gardenerAPIServerTLSServingCrt" yaml:"gardenerAPIServerTLSServingCrt"`
	GardenerAPIServerTLSServingKey string `json:"gardenerAPIServerTLSServingKey" yaml:"gardenerAPIServerTLSServingKey"`

	GardenerAPIServerEncryptionConfiguration string `json:"gardenerAPIServerEncryptionConfiguration" yaml:"gardenerAPIServerEncryptionConfiguration"`

	GardenerAdmissionControllerCACrt string `json:"gardenerAdmissionControllerCACrt,omitempty" yaml:"gardenerAdmissionControllerCACrt,omitempty"`
	GardenerAdmissionControllerCAKey string `json:"gardenerAdmissionControllerCAKey,omitempty" yaml:"gardenerAdmissionControllerCAKey,omitempty"`

	GardenerControllerManagerTLSServingCrt string `json:"gardenerControllerManagerTLSServingCrt" yaml:"gardenerControllerManagerTLSServingCrt"`
	GardenerControllerManagerTLSServingKey string `json:"gardenerControllerManagerTLSServingKey" yaml:"gardenerControllerManagerTLSServingKey"`

	GardenerAdmissionControllerTLSServingCrt string `json:"gardenerAdmissionControllerTLSServingCrt,omitempty" yaml:"gardenerAdmissionControllerTLSServingCrt,omitempty"`
	GardenerAdmissionControllerTLSServingKey string `json:"gardenerAdmissionControllerTLSServingKey,omitempty" yaml:"gardenerAdmissionControllerTLSServingKey,omitempty"`
}

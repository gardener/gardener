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

package exports

// Exports defines the structure for the exported data which might be consumed by other components.
type Exports struct {
	GardenerIdentity                         string `json:"gardenerIdentity" yaml:"gardenerIdentity"`
	GardenerAPIServerEncryptionConfiguration string `json:"gardenerAPIServerEncryptionConfiguration" yaml:"gardenerAPIServerEncryptionConfiguration"`
	OpenVPNDiffieHellmanKey                  string `json:"openVPNDiffieHellmanKey" yaml:"openVPNDiffieHellmanKey"`

	GardenerAPIServerCA           Certificate  `json:"gardenerAPIServerCA" yaml:"gardenerAPIServerCA"`
	GardenerAdmissionControllerCA *Certificate `json:"gardenerAdmissionControllerCA,omitempty" yaml:"gardenerAdmissionControllerCA,omitempty"`

	GardenerAPIServerTLSServing           Certificate  `json:"gardenerAPIServerTLSServing" yaml:"gardenerAPIServerTLSServing"`
	GardenerControllerManagerTLSServing   Certificate  `json:"gardenerControllerManagerTLSServing" yaml:"gardenerControllerManagerTLSServing"`
	GardenerAdmissionControllerTLSServing *Certificate `json:"gardenerAdmissionControllerTLSServing,omitempty" yaml:"gardenerAdmissionControllerTLSServing,omitempty"`
}

type Certificate struct {
	Rotated bool   `json:"rotated" yaml:"rotated"`
	Crt     string `json:"crt" yaml:"crt"`
	Key     string `json:"key" yaml:"key"`
}

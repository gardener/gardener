// Copyright (c) 2022 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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
	// GardenerIdentity is the identity of the Gardener installation
	GardenerIdentity string `json:"gardenerIdentity"`
	// GardenerAPIServerEncryptionConfiguration is the encryption configuration of the Gardener API Server
	GardenerAPIServerEncryptionConfiguration string `json:"gardenerAPIServerEncryptionConfiguration"`
	// OpenVPNDiffieHellmanKey is the Diffie-Hellman-Key used for the Shoot<->Seed VPN
	OpenVPNDiffieHellmanKey string `json:"openVPNDiffieHellmanKey"`

	// GardenerAPIServerCA is the PEM encoded CA certificate of the Gardener API Server
	GardenerAPIServerCA Certificate `json:"gardenerAPIServerCA"`
	// GardenerAPIServerCA is the PEM encoded CA certificate of the Gardener Admission Controller
	GardenerAdmissionControllerCA *Certificate `json:"gardenerAdmissionControllerCA,omitempty"`

	// GardenerAPIServerTLSServing is the TLS serving certificate of the Gardener API Server
	GardenerAPIServerTLSServing Certificate `json:"gardenerAPIServerTLSServing"`
	// GardenerControllerManagerTLSServing is the TLS serving certificate of the Gardener Controller Manager
	GardenerControllerManagerTLSServing Certificate `json:"gardenerControllerManagerTLSServing"`
	// GardenerAdmissionControllerTLSServing is the TLS serving certificate of the Gardener Admission Controller
	GardenerAdmissionControllerTLSServing *Certificate `json:"gardenerAdmissionControllerTLSServing,omitempty"`
}

// Certificate represents an exported certificate
type Certificate struct {
	// Rotated defines if the certificate has been rotated due to expiration during execution
	Rotated bool `json:"rotated"`
	// Crt is the x509 certificate
	Crt string `json:"crt"`
	// Key is the RSA private key
	Key string `json:"key"`
}

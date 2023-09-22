// Copyright 2021 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

package kubernetes

// TLSCipherSuites is a list of the wanted and acceptable cipher suits.
var TLSCipherSuites = []string{
	"TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256",
	"TLS_ECDHE_RSA_WITH_AES_256_GCM_SHA384",
	"TLS_AES_128_GCM_SHA256",
	"TLS_AES_256_GCM_SHA384",
	"TLS_ECDHE_ECDSA_WITH_AES_128_GCM_SHA256",
	"TLS_ECDHE_ECDSA_WITH_AES_256_GCM_SHA384",
	"TLS_CHACHA20_POLY1305_SHA256",
	"TLS_ECDHE_ECDSA_WITH_CHACHA20_POLY1305",
	"TLS_ECDHE_RSA_WITH_CHACHA20_POLY1305",
}

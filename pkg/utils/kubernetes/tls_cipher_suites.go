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

package kubernetes

import (
	"github.com/Masterminds/semver"

	"github.com/gardener/gardener/pkg/utils/version"
)

// TLSCipherSuites returns the wanted and acceptable cipher suits depending on the passed Kubernetes version.
func TLSCipherSuites(k8sVersion *semver.Version) []string {
	var (
		commonSuites = []string{
			"TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256",
			"TLS_ECDHE_RSA_WITH_AES_256_GCM_SHA384",
		}
		tlsV13Suites = append(commonSuites,
			"TLS_AES_128_GCM_SHA256",
			"TLS_AES_256_GCM_SHA384",
			"TLS_ECDHE_ECDSA_WITH_AES_128_GCM_SHA256",
			"TLS_ECDHE_ECDSA_WITH_AES_256_GCM_SHA384",
		)
	)

	if version.ConstraintK8sLessEqual121.Check(k8sVersion) {
		return append(commonSuites,
			"TLS_ECDHE_RSA_WITH_CHACHA20_POLY1305",
			"TLS_RSA_WITH_AES_128_CBC_SHA",
			"TLS_RSA_WITH_AES_256_CBC_SHA",
			"TLS_ECDHE_RSA_WITH_AES_256_CBC_SHA",
		)
	}

	// For Kubernetes 1.22 Gardener only allows suites permissible for TLS 1.3
	// see https://github.com/gardener/gardener/issues/4300#issuecomment-885498872
	if version.ConstraintK8sLessEqual122.Check(k8sVersion) {
		return tlsV13Suites
	}

	// For Kubernetes >= 1.23 the Cipher list was again adapted as described in
	// https://github.com/gardener/gardener/issues/4823#issue-1022865330
	return append(tlsV13Suites,
		"TLS_CHACHA20_POLY1305_SHA256",
		"TLS_ECDHE_ECDSA_WITH_CHACHA20_POLY1305",
		"TLS_ECDHE_RSA_WITH_CHACHA20_POLY1305",
	)
}

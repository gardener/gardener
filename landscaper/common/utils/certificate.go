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

package utils

import (
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"time"
)

// NowFunc is a function returning the current time.
// Exposed for testing.
var NowFunc = time.Now

// CertificateNeedsRenewal returns true in case the certificate is not (yet) valid or in case the given validityThresholdPercentage is exceeded.
// A validityThresholdPercentage lower than 1 (100%) should be given in case the certificate should be renewed well in advance before the certificate expires.
// Based on: https://github.com/gardener/gardenlogin-controller-manager/tree/master/.landscaper/container/internal/util
func CertificateNeedsRenewal(certificate *x509.Certificate, validityThresholdPercentage float64) (bool, time.Duration) {
	notBefore := certificate.NotBefore.UTC()
	notAfter := certificate.NotAfter.UTC()

	validNotBefore := NowFunc().UTC().After(notBefore) || NowFunc().UTC().Equal(notBefore)
	validNotAfter := NowFunc().UTC().Before(notAfter) || NowFunc().UTC().Equal(notAfter)

	isValid := validNotBefore && validNotAfter
	if !isValid {
		return true, 0
	}

	validityTimespan := notAfter.Sub(notBefore).Seconds()
	elapsedValidity := NowFunc().UTC().Sub(notBefore).Seconds()

	validityThreshold := validityTimespan * validityThresholdPercentage

	return elapsedValidity > validityThreshold, time.Duration(validityThreshold-elapsedValidity) * time.Second
}

// ParseX509Certificate parses a given input string as a x509 certificate
func ParseX509Certificate(input string) (*x509.Certificate, error) {
	block, _ := pem.Decode([]byte(input))
	if block == nil {
		return nil, fmt.Errorf("the TLS certificate is not PEM encoded")
	}

	cert, err := x509.ParseCertificate(block.Bytes)
	if err != nil {
		return nil, fmt.Errorf("the TLS certificate provided cannot be parses as a X509 certificate: %s", err.Error())
	}

	return cert, nil
}

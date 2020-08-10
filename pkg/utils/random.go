// Copyright (c) 2018 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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
	cryptorand "crypto/rand"
	"math/big"
	mathrand "math/rand"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// GenerateRandomString uses crypto/rand to generate a random string of the specified length <n>.
// The set of allowed characters is [0-9a-zA-Z], thus no special characters are included in the output.
// Returns error if there was a problem during the random generation.
func GenerateRandomString(n int) (string, error) {
	allowedCharacters := "0123456789abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ"
	return GenerateRandomStringFromCharset(n, allowedCharacters)
}

// GenerateRandomStringFromCharset generates a cryptographically secure random string of the specified length <n>.
// The set of allowed characters can be specified. Returns error if there was a problem during the random generation.
func GenerateRandomStringFromCharset(n int, allowedCharacters string) (string, error) {
	output := make([]byte, n)
	max := new(big.Int).SetInt64(int64(len(allowedCharacters)))
	for i := range output {
		randomCharacter, err := cryptorand.Int(cryptorand.Reader, max)
		if err != nil {
			return "", err
		}
		output[i] = allowedCharacters[randomCharacter.Int64()]
	}
	return string(output), nil
}

// RandomDuration takes a time.Duration and computes a non-negative pseudo-random duration in [0,max).
// It returns 0ns if max is <= 0ns.
func RandomDuration(max time.Duration) time.Duration {
	if max.Nanoseconds() <= 0 {
		return time.Duration(0)
	}
	return time.Duration(mathrand.Int63n(max.Nanoseconds()))
}

// RandomDurationWithMetaDuration takes a *metav1.Duration and computes a non-negative pseudo-random duration in [0,max).
// It returns 0ns if max is nil or <= 0ns.
func RandomDurationWithMetaDuration(max *metav1.Duration) time.Duration {
	if max == nil {
		return time.Duration(0)
	}
	return RandomDuration(max.Duration)
}

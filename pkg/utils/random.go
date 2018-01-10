// Copyright 2018 The Gardener Authors.
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
	"math/rand"
	"time"
)

// GenerateRandomString generates a random string of the specified length <n>. The set of allowed characters
// is [0-9a-zA-Z], thus no special characters are included in the output.
func GenerateRandomString(n int) string {
	allowedCharacters := "0123456789abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ"
	return GenerateRandomStringFromCharset(n, allowedCharacters)
}

// GenerateRandomStringFromCharset generates a random string of the specified length <n>. The set of allowed characters
// can be specified.
func GenerateRandomStringFromCharset(n int, allowedCharacters string) string {
	seed := time.Now().UnixNano()
	randomSource := rand.NewSource(seed)
	randomGenerator := rand.New(randomSource)
	output := make([]byte, n)
	for i := range output {
		randomCharacter := randomGenerator.Intn(len(allowedCharacters))
		output[i] = allowedCharacters[randomCharacter]
	}
	return string(output)
}

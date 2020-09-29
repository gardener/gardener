// Copyright (c) 2020 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

package matchers

import (
	"fmt"

	"github.com/onsi/gomega/format"
)

type kubernetesErrors struct {
	checkFunc func(error) bool
	message   string
}

func (k *kubernetesErrors) Match(actual interface{}) (success bool, err error) {
	// is purely nil?
	if actual == nil {
		return false, nil
	}

	actualErr, actualOk := actual.(error)
	if !actualOk {
		return false, fmt.Errorf("expected an error-type.  got:\n%s", format.Object(actual, 1))
	}

	return k.checkFunc(actualErr), nil
}

func (k *kubernetesErrors) FailureMessage(actual interface{}) (message string) {
	return format.Message(actual, fmt.Sprintf("to be %s error", k.message))
}
func (k *kubernetesErrors) NegatedFailureMessage(actual interface{}) (message string) {
	return format.Message(actual, fmt.Sprintf("to not be %s error", k.message))
}

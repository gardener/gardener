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

package util

import (
	"errors"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	gardencorev1beta1helper "github.com/gardener/gardener/pkg/apis/core/v1beta1/helper"

	"k8s.io/apimachinery/pkg/util/sets"
)

// DetermineError determines the Gardener error codes for the given error and returns an ErrorWithCodes with the error and codes.
func DetermineError(err error, knownCodes map[gardencorev1beta1.ErrorCode]func(string) bool) error {
	if err == nil {
		return nil
	}

	// try to re-use codes from error
	var coder gardencorev1beta1helper.Coder
	if errors.As(err, &coder) {
		return err
	}

	codes := DetermineErrorCodes(err, knownCodes)
	if len(codes) == 0 {
		return err
	}

	return gardencorev1beta1helper.NewErrorWithCodes(err, codes...)
}

// DetermineErrorCodes determines error codes based on the given error.
func DetermineErrorCodes(err error, knownCodes map[gardencorev1beta1.ErrorCode]func(string) bool) []gardencorev1beta1.ErrorCode {
	var (
		coder   gardencorev1beta1helper.Coder
		message = err.Error()
		codes   = sets.NewString()
	)

	if err == nil {
		return nil
	}

	// try to re-use codes from error
	if errors.As(err, &coder) {
		for _, code := range coder.Codes() {
			codes.Insert(string(code))
		}
	}

	// determine error codes
	for code, matchFn := range knownCodes {
		if !codes.Has(string(code)) && matchFn(message) {
			codes.Insert(string(code))
		}
	}

	// compute error code list based on code string set
	var out []gardencorev1beta1.ErrorCode
	for _, c := range codes.List() {
		out = append(out, gardencorev1beta1.ErrorCode(c))
	}
	return out
}

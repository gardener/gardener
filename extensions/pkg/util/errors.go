// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package util

import (
	"errors"

	"k8s.io/apimachinery/pkg/util/sets"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	v1beta1helper "github.com/gardener/gardener/pkg/apis/core/v1beta1/helper"
)

// DetermineError determines the Gardener error codes for the given error and returns an ErrorWithCodes with the error and codes.
func DetermineError(err error, knownCodes map[gardencorev1beta1.ErrorCode]func(string) bool) error {
	if err == nil {
		return nil
	}

	// try to re-use codes from error
	var coder v1beta1helper.Coder
	if errors.As(err, &coder) {
		return err
	}

	codes := DetermineErrorCodes(err, knownCodes)
	if len(codes) == 0 {
		return err
	}

	return v1beta1helper.NewErrorWithCodes(err, codes...)
}

// DetermineErrorCodes determines error codes based on the given error.
func DetermineErrorCodes(err error, knownCodes map[gardencorev1beta1.ErrorCode]func(string) bool) []gardencorev1beta1.ErrorCode {
	var (
		coder   v1beta1helper.Coder
		message = err.Error()
		codes   = sets.New[string]()
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
	for _, c := range sets.List(codes) {
		out = append(out, gardencorev1beta1.ErrorCode(c))
	}
	return out
}

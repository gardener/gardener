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

package errors

import "errors"

// Unwraps and returns the root error. Multiple wrappings either via `fmt.Errorf` or via `causer` implementations are properly taken into account.
func Unwrap(err error) error {
	var (
		cdone      bool
		res, udone = unwrapErr(err)
	)

	for !(udone && cdone) {
		res, cdone = unwrapCauser(res)
		res, udone = unwrapErr(res)
	}

	return res
}

func unwrapErr(err error) (error, bool) {
	if err == nil || errors.Unwrap(err) == nil {
		// this most likely is the root error
		return err, true
	}

	for err != nil {
		var e = errors.Unwrap(err)
		if e == nil {
			break
		}
		err = e
	}
	return err, false
}

type causer interface {
	Cause() error
}

func unwrapCauser(err error) (error, bool) {
	if _, ok := err.(causer); !ok {
		// this most likely is the root cause error
		return err, true
	}

	for err != nil {
		cause, ok := err.(causer)
		if !ok {
			break
		}
		err = cause.Cause()
	}

	return err, false
}

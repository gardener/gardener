// SPDX-FileCopyrightText: 2018 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package test

import (
	"fmt"
	"os"
	"reflect"

	"github.com/onsi/ginkgo"
)

// WithVar sets the given var to the src value and returns a function to revert to the original state.
// The type of `dst` has to be a settable pointer.
// The value of `src` has to be assignable to the type of `dst`.
//
// Example usage:
//   v := "foo"
//   defer WithVar(&v, "bar")()
func WithVar(dst, src interface{}) func() {
	dstValue := reflect.ValueOf(dst)
	if dstValue.Type().Kind() != reflect.Ptr {
		ginkgo.Fail(fmt.Sprintf("destination value %T is not a pointer", dst))
	}

	if dstValue.CanSet() {
		ginkgo.Fail(fmt.Sprintf("value %T cannot be set", dst))
	}

	srcValue := reflect.ValueOf(src)
	if srcValue.Type().AssignableTo(dstValue.Type()) {
		ginkgo.Fail(fmt.Sprintf("cannot write %T into %T", src, dst))
	}

	tmp := dstValue.Elem().Interface()
	dstValue.Elem().Set(srcValue)
	return func() {
		dstValue.Elem().Set(reflect.ValueOf(tmp))
	}
}

// WithVars sets the given vars to the given values and returns a function to revert back.
// dstsAndSrcs have to appear in pairs of 2, otherwise there will be a runtime panic.
//
// Example usage:
//  defer WithVars(&v, "foo", &x, "bar")()
func WithVars(dstsAndSrcs ...interface{}) func() {
	if len(dstsAndSrcs)%2 != 0 {
		ginkgo.Fail(fmt.Sprintf("dsts and srcs are not of equal length: %v", dstsAndSrcs))
	}
	reverts := make([]func(), 0, len(dstsAndSrcs)/2)

	for i := 0; i < len(dstsAndSrcs); i += 2 {
		dst := dstsAndSrcs[i]
		src := dstsAndSrcs[i+1]

		reverts = append(reverts, WithVar(dst, src))
	}

	return func() {
		for _, revert := range reverts {
			revert()
		}
	}
}

// WithEnvVar sets the env variable to the given environment variable and returns a function to revert.
// If the value is empty, the environment variable will be unset.
func WithEnvVar(key, value string) func() {
	tmp := os.Getenv(key)

	var err error
	if value == "" {
		err = os.Unsetenv(key)
	} else {
		err = os.Setenv(key, value)
	}
	if err != nil {
		ginkgo.Fail(fmt.Sprintf("Could not set the env variable %q to %q: %v", key, value, err))
	}

	return func() {
		var err error
		if tmp == "" {
			err = os.Unsetenv(key)
		} else {
			err = os.Setenv(key, tmp)
		}
		if err != nil {
			ginkgo.Fail(fmt.Sprintf("Could not revert the env variable %q to %q: %v", key, value, err))
		}
	}
}

// WithWd sets the working directory and returns a function to revert to the previous one.
func WithWd(path string) func() {
	oldPath, err := os.Getwd()
	if err != nil {
		ginkgo.Fail(fmt.Sprintf("Could not obtain current working diretory: %v", err))
	}

	if err := os.Chdir(path); err != nil {
		ginkgo.Fail(fmt.Sprintf("Could not change working diretory: %v", err))
	}

	return func() {
		if err := os.Chdir(oldPath); err != nil {
			ginkgo.Fail(fmt.Sprintf("Could not revert working diretory: %v", err))
		}
	}
}

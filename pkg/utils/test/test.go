// Copyright 2018 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

package test

import (
	"context"
	"fmt"
	"os"
	"reflect"
	"time"

	"github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"go.uber.org/mock/gomock"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/component-base/featuregate"
	"sigs.k8s.io/controller-runtime/pkg/client"

	mockclient "github.com/gardener/gardener/pkg/mock/controller-runtime/client"
	. "github.com/gardener/gardener/pkg/utils/test/matchers"
)

// WithVar sets the given var to the src value and returns a function to revert to the original state.
// The type of `dst` has to be a settable pointer.
// The value of `src` has to be assignable to the type of `dst`.
//
// Example usage:
//
//	v := "foo"
//	DeferCleanup(WithVar(&v, "bar"))
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
//
//	DeferCleanup(WithVars(&v, "foo", &x, "bar"))
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

// WithFeatureGate sets the specified gate to the specified value, and returns a function that restores the original value.
// Failures to set or restore cause the test to fail.
// Example use:
//
//	DeferCleanup(WithFeatureGate(features.DefaultFeatureGate, features.<FeatureName>, true))
func WithFeatureGate(gate featuregate.FeatureGate, f featuregate.Feature, value bool) func() {
	originalValue := gate.Enabled(f)

	if err := gate.(featuregate.MutableFeatureGate).Set(fmt.Sprintf("%s=%v", f, value)); err != nil {
		ginkgo.Fail(fmt.Sprintf("could not set feature gate %s=%v: %v", f, value, err))
	}

	return func() {
		if err := gate.(featuregate.MutableFeatureGate).Set(fmt.Sprintf("%s=%v", f, originalValue)); err != nil {
			ginkgo.Fail(fmt.Sprintf("could not restore feature gate %s=%v: %v", f, originalValue, err))
		}
	}
}

// WithTempFile creates a temporary file with the given dir and pattern, writes the given content to it,
// and returns a function to delete it. Failures to create, open, close, or delete the file case the test to fail.
//
// The filename is generated by taking pattern and adding a random string to the end. If pattern includes a "*",
// the random string replaces the last "*". If dir is the empty string, WriteTempFile uses the default directory for
// temporary files (see ioutil.TempFile). The caller can use the value of fileName to find the pathname of the file.
//
// Example usage:
//
//	var fileName string
//	DeferCleanup(WithTempFile("", "test", []byte("test file content"), &fileName))
func WithTempFile(dir, pattern string, content []byte, fileName *string) func() {
	file, err := os.CreateTemp(dir, pattern)
	if err != nil {
		ginkgo.Fail(fmt.Sprintf("could not create temp file in directory %s: %v", dir, err))
	}

	*fileName = file.Name()

	if _, err := file.Write(content); err != nil {
		ginkgo.Fail(fmt.Sprintf("could not write to temp file %s: %v", file.Name(), err))
	}
	if err := file.Close(); err != nil {
		ginkgo.Fail(fmt.Sprintf("could not close temp file %s: %v", file.Name(), err))
	}

	return func() {
		if err := os.Remove(file.Name()); err != nil {
			ginkgo.Fail(fmt.Sprintf("could not delete temp file %s: %v", file.Name(), err))
		}
	}
}

// EXPECTPatch is a helper function for a GoMock call expecting a patch with the mock client.
func EXPECTPatch(ctx interface{}, c *mockclient.MockClient, expectedObj, mergeFrom client.Object, patchType types.PatchType, rets ...interface{}) *gomock.Call {
	var expectedPatch client.Patch

	switch patchType {
	case types.MergePatchType:
		expectedPatch = client.MergeFrom(mergeFrom)
	case types.StrategicMergePatchType:
		expectedPatch = client.StrategicMergeFrom(mergeFrom.DeepCopyObject().(client.Object))
	}

	return expectPatch(ctx, c, expectedObj, expectedPatch, rets...)
}

// EXPECTStatusPatch is a helper function for a GoMock call expecting a status patch with the mock client.
func EXPECTStatusPatch(ctx interface{}, c *mockclient.MockStatusWriter, expectedObj, mergeFrom client.Object, patchType types.PatchType, rets ...interface{}) *gomock.Call {
	var expectedPatch client.Patch

	switch patchType {
	case types.MergePatchType:
		expectedPatch = client.MergeFrom(mergeFrom)
	case types.StrategicMergePatchType:
		expectedPatch = client.StrategicMergeFrom(mergeFrom.DeepCopyObject().(client.Object))
	}

	return expectStatusPatch(ctx, c, expectedObj, expectedPatch, rets...)
}

// EXPECTPatchWithOptimisticLock is a helper function for a GoMock call with the mock client
// expecting a merge patch with optimistic lock.
func EXPECTPatchWithOptimisticLock(ctx interface{}, c *mockclient.MockClient, expectedObj, mergeFrom client.Object, patchType types.PatchType, rets ...interface{}) *gomock.Call {
	var expectedPatch client.Patch

	switch patchType {
	case types.MergePatchType:
		expectedPatch = client.MergeFromWithOptions(mergeFrom, client.MergeFromWithOptimisticLock{})
	case types.StrategicMergePatchType:
		expectedPatch = client.StrategicMergeFrom(mergeFrom.DeepCopyObject().(client.Object), client.MergeFromWithOptimisticLock{})
	}

	return expectPatch(ctx, c, expectedObj, expectedPatch, rets...)
}

func expectPatch(ctx interface{}, c *mockclient.MockClient, expectedObj client.Object, expectedPatch client.Patch, rets ...interface{}) *gomock.Call {
	expectedData, expectedErr := expectedPatch.Data(expectedObj)
	Expect(expectedErr).To(BeNil())

	if rets == nil {
		rets = []interface{}{nil}
	}

	// match object key here, but verify contents only inside DoAndReturn.
	// This is to tell gomock, for which object we expect the given patch, but to enable rich yaml diff between
	// actual and expected via `DeepEqual`.
	return c.
		EXPECT().
		Patch(ctx, HasObjectKeyOf(expectedObj), gomock.Any()).
		DoAndReturn(func(_ context.Context, obj client.Object, patch client.Patch, _ ...client.PatchOption) error {
			// if one of these Expects fails and Patch is called in some goroutine (e.g. via flow.Parallel)
			// the failures will not be shown, as the ginkgo panic is not recovered, so the test is hard to fix
			defer ginkgo.GinkgoRecover()

			Expect(obj).To(DeepEqual(expectedObj))
			data, err := patch.Data(obj)
			Expect(err).To(BeNil())
			Expect(patch.Type()).To(Equal(expectedPatch.Type()))
			Expect(string(data)).To(Equal(string(expectedData)))
			return nil
		}).
		Return(rets...)
}

func expectStatusPatch(ctx interface{}, c *mockclient.MockStatusWriter, expectedObj client.Object, expectedPatch client.Patch, rets ...interface{}) *gomock.Call {
	expectedData, expectedErr := expectedPatch.Data(expectedObj)
	Expect(expectedErr).To(BeNil())

	if rets == nil {
		rets = []interface{}{nil}
	}

	// match object key here, but verify contents only inside DoAndReturn.
	// This is to tell gomock, for which object we expect the given patch, but to enable rich yaml diff between
	// actual and expected via `DeepEqual`.
	return c.
		EXPECT().
		Patch(ctx, HasObjectKeyOf(expectedObj), gomock.Any()).
		DoAndReturn(func(_ context.Context, obj client.Object, patch client.Patch, _ ...client.PatchOption) error {
			// if one of these Expects fails and Patch is called in some goroutine (e.g. via flow.Parallel)
			// the failures will not be shown, as the ginkgo panic is not recovered, so the test is hard to fix
			defer ginkgo.GinkgoRecover()

			Expect(obj).To(DeepEqual(expectedObj))
			data, err := patch.Data(obj)
			Expect(err).To(BeNil())
			Expect(patch.Type()).To(Equal(expectedPatch.Type()))
			Expect(string(data)).To(Equal(string(expectedData)))
			return nil
		}).
		Return(rets...)
}

// CEventually is like gomega.Eventually but with a context.Context. When it has a deadline then the gomega.Eventually
// call with be configured with a the respective timeout.
func CEventually(ctx context.Context, actual interface{}) AsyncAssertion {
	deadline, ok := ctx.Deadline()
	if !ok {
		return Eventually(actual)
	}
	return Eventually(actual).WithTimeout(time.Until(deadline))
}

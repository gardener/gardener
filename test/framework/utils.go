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

package framework

import (
	"fmt"
	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	"github.com/onsi/ginkgo"
	"io/ioutil"
	apimachineryRuntime "k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/serializer"
	"os"
	"reflect"
	"sigs.k8s.io/yaml"
)

// Must errors with `GinkgoT().Fatal` if the error is non-nil.
func Must(err error) {
	if err != nil {
		ginkgo.GinkgoT().Fatal(err)
	}
}

func checkPtr(v reflect.Value) error {
	if v.Type().Kind() != reflect.Ptr {
		return fmt.Errorf("value has to be a pointer-type but got %T", v.Interface())
	}
	return nil
}

func checkAssignable(src, dst reflect.Value) error {
	if !src.Type().AssignableTo(dst.Type().Elem()) {
		return fmt.Errorf("src of type %T cannot be assigned to dst of type %T", src.Interface(), dst.Interface())
	}
	return nil
}

func dereference(v interface{}) interface{} {
	dstValue := reflect.ValueOf(v)
	Must(checkPtr(dstValue))

	return dstValue.Elem().Interface()
}

// RevertableSet sets the element of dst to src and returns a function that can revert back to the original values.
func RevertableSet(dst, src interface{}) (revert func()) {
	tmp := dereference(dst)
	Set(dst, src)
	return func() { Set(dst, tmp) }
}

// Set sets the pointer dst to the value of src.
//
// dst has to be a pointer, src has to be assignable to the element type of dst.
func Set(dst, src interface{}) {
	dstValue := reflect.ValueOf(dst)
	Must(checkPtr(dstValue))

	srcValue := reflect.ValueOf(src)
	Must(checkAssignable(srcValue, dstValue))

	dstValue.Elem().Set(srcValue)
}

func computeTechnicalID(projectName string, shoot *gardencorev1beta1.Shoot) string {
	// Use the stored technical ID in the Shoot's status field if it's there.
	// For backwards compatibility we keep the pattern as it was before we had to change it
	// (double hyphens).
	if len(shoot.Status.TechnicalID) > 0 {
		return shoot.Status.TechnicalID
	}

	// New clusters shall be created with the new technical id (double hyphens).
	return fmt.Sprintf("shoot--%s--%s", projectName, shoot.Name)
}

// Exists checks if a path exists
func Exists(path string) (bool, error) {
	if _, err := os.Stat(path); err != nil {
		if os.IsNotExist(err) {
			return false, nil
		}
		return false, err
	}
	return true, nil
}

// StringSet checks if a string is set
func StringSet(s string) bool {
	return len(s) != 0
}

// FileExists Checks if a file path exists and fail otherwise
func FileExists(kc string) bool {
	ok, err := Exists(kc)
	if err != nil {
		ginkgo.Fail(err.Error())
	}
	return ok
}

// ReadObject loads the contents of file and decodes it as an object.
func ReadObject(file string, into apimachineryRuntime.Object) error {
	data, err := ioutil.ReadFile(file)
	if err != nil {
		return err
	}

	_, _, err = serializer.NewCodecFactory(kubernetes.GardenScheme).UniversalDecoder().Decode(data, nil, into)
	return err
}

// ParseFileAsProviderConfig parses a file as a ProviderConfig
func ParseFileAsProviderConfig(filepath string) (*gardencorev1beta1.ProviderConfig, error) {
	data, err := ioutil.ReadFile(filepath)
	if err != nil {
		return nil, err
	}

	// apiServer needs JSON for the Raw data
	jsonData, err := yaml.YAMLToJSON(data)
	if err != nil {
		return nil, fmt.Errorf("unable to decode ProviderConfig: %v", err)
	}
	return &gardencorev1beta1.ProviderConfig{RawExtension: apimachineryRuntime.RawExtension{Raw: jsonData}}, nil
}

// ParseFileAsWorkers parses a file as a Worker configuration
func ParseFileAsWorkers(filepath string) ([]gardencorev1beta1.Worker, error) {
	data, err := ioutil.ReadFile(filepath)
	if err != nil {
		return nil, err
	}

	workers := []gardencorev1beta1.Worker{}
	if err := yaml.Unmarshal(data, &workers); err != nil {
		return nil, fmt.Errorf("unable to decode workers: %v", err)
	}
	return workers, nil
}

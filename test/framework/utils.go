// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package framework

import (
	"errors"
	"fmt"
	"os"
	"reflect"
	"regexp"

	"github.com/hashicorp/go-multierror"
	"github.com/onsi/ginkgo/v2"
	apimachineryRuntime "k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/serializer"
	"sigs.k8s.io/yaml"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	"github.com/gardener/gardener/pkg/client/kubernetes"
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

func dereference(v any) any {
	dstValue := reflect.ValueOf(v)
	Must(checkPtr(dstValue))

	return dstValue.Elem().Interface()
}

// RevertableSet sets the element of dst to src and returns a function that can revert back to the original values.
func RevertableSet(dst, src any) (revert func()) {
	tmp := dereference(dst)
	Set(dst, src)
	return func() { Set(dst, tmp) }
}

// Set sets the pointer dst to the value of src.
//
// dst has to be a pointer, src has to be assignable to the element type of dst.
func Set(dst, src any) {
	dstValue := reflect.ValueOf(dst)
	Must(checkPtr(dstValue))

	srcValue := reflect.ValueOf(src)
	Must(checkAssignable(srcValue, dstValue))

	dstValue.Elem().Set(srcValue)
}

// ComputeTechnicalID computes the technical ID of a shoot
func ComputeTechnicalID(projectName string, shoot *gardencorev1beta1.Shoot) string {
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
	data, err := os.ReadFile(file) // #nosec: G304 -- Test only.
	if err != nil {
		return err
	}

	_, _, err = serializer.NewCodecFactory(kubernetes.GardenScheme).UniversalDeserializer().Decode(data, nil, into)
	return err
}

// ParseFileAsProviderConfig parses a file as a ProviderConfig
func ParseFileAsProviderConfig(filepath string) (*apimachineryRuntime.RawExtension, error) {
	data, err := os.ReadFile(filepath) // #nosec: G304 -- Test only.
	if err != nil {
		return nil, err
	}

	// apiServer needs JSON for the Raw data
	jsonData, err := yaml.YAMLToJSON(data)
	if err != nil {
		return nil, fmt.Errorf("unable to decode ProviderConfig: %v", err)
	}
	return &apimachineryRuntime.RawExtension{Raw: jsonData}, nil
}

// ParseFileAsWorkers parses a file as a Worker configuration
func ParseFileAsWorkers(filepath string) ([]gardencorev1beta1.Worker, error) {
	data, err := os.ReadFile(filepath) // #nosec: G304 -- Test only.
	if err != nil {
		return nil, err
	}

	workers := []gardencorev1beta1.Worker{}
	if err := yaml.Unmarshal(data, &workers); err != nil {
		return nil, fmt.Errorf("unable to decode workers: %v", err)
	}
	return workers, nil
}

// GetTestRunID returns the current testmachinery testrun ID.
func GetTestRunID() string {
	return os.Getenv(TestMachineryTestRunIDEnvVarName)
}

// TextValidation is a map of regular expression to description
// that is used to validate texts based on allowed or denied regexps.
type TextValidation map[string]string

// ValidateAsAllowlist validates that all allowed regular expressions
// are in the given text.
func (v *TextValidation) ValidateAsAllowlist(text []byte) error {
	return v.validate(text, func(matches [][]byte) error {
		if len(matches) == 0 {
			return errors.New("allowed RegExp not found")
		}
		return nil
	})
}

// ValidateAsDenylist validates that no denied regular expressions
// are in the given text.
func (v *TextValidation) ValidateAsDenylist(text []byte) error {
	return v.validate(text, func(matches [][]byte) error {
		if len(matches) != 0 {
			return errors.New("denied RegExp found")
		}
		return nil
	})
}

// validate compiles all given regular expressions strings and finds all matches in the given text.
func (v *TextValidation) validate(text []byte, validationFunc func([][]byte) error) error {
	var allErrs error

	for reString, description := range *v {
		re, err := regexp.Compile(reString)
		if err != nil {
			allErrs = multierror.Append(allErrs, err)
			continue
		}

		matches := re.FindAll(text, -1)
		if err := validationFunc(matches); err != nil {
			allErrs = multierror.Append(allErrs, fmt.Errorf("RegExp %s validation failed: %s: %w", reString, description, err))
		}
	}

	return allErrs
}

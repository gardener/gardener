// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"fmt"
	"maps"
	"slices"

	flag "github.com/spf13/pflag"
	"k8s.io/apimachinery/pkg/util/sets"
)

// Options contain the generate configuration.
type Options struct {
	ExtensionName       string
	ProviderType        string
	ComponentCategories []string
	Destination         string

	ExtensionOCIRepository            string
	AdmissionRuntimeOCIRepository     string
	AdmissionApplicationOCIRepository string

	InjectGardenKubeconfig bool
}

var validCategories = sets.New(slices.Collect(maps.Keys(categoryToEnsurer))...)

// AddFlags adds the cmd flags to the given FlagSet.
func (o *Options) AddFlags(flags *flag.FlagSet) {
	flags.StringVar(&o.ExtensionName, "name", "", "Name is the name of the extension")
	flags.StringVar(&o.ProviderType, "provider-type", "", "Type of the provider")
	flags.StringArrayVar(&o.ComponentCategories, "component-category", nil, fmt.Sprintf("Category of the component, one of %v", validCategories.UnsortedList()))
	flags.StringVar(&o.Destination, "destination", "", "The path the extension manifest is written to")
	flags.StringVar(&o.ExtensionOCIRepository, "extension-oci-repository", "", "URL to OCI image containing the extension chart")
	flags.StringVar(&o.AdmissionRuntimeOCIRepository, "admission-runtime-oci-repository", "", "OPTIONAL: URL to OCI image containing the admission runtime chart")
	flags.StringVar(&o.AdmissionApplicationOCIRepository, "admission-application-oci-repository", "", "OPTIONAL: URL to OCI image containing the admission application chart")
	flags.BoolVar(&o.InjectGardenKubeconfig, "inject-garden-kubeconfig", false, "OPTIONAL: When set, the `.spec.deployment.extension.injectGardenKubeconfig: true` field is added to the generated extension")
}

// Validate returns an error if the Options configuration is incomplete.
func (o *Options) Validate() error {
	var errs []error

	if len(o.ExtensionName) == 0 {
		errs = append(errs, fmt.Errorf("extension name is required"))
	}

	if len(o.ProviderType) == 0 {
		errs = append(errs, fmt.Errorf("provider type is required"))
	}

	if o.ComponentCategories == nil {
		errs = append(errs, fmt.Errorf("component categories is required"))
	} else if !validCategories.HasAll(o.ComponentCategories...) {
		errs = append(errs, fmt.Errorf("at least one component category is invalid, must be one of %v", validCategories.UnsortedList()))
	}

	if len(o.Destination) == 0 {
		errs = append(errs, fmt.Errorf("destination is required"))
	}

	if len(o.ExtensionOCIRepository) == 0 {
		errs = append(errs, fmt.Errorf("extension oci repository is required"))
	}

	if len(errs) > 0 {
		return fmt.Errorf("%v", errs)
	}
	return nil
}

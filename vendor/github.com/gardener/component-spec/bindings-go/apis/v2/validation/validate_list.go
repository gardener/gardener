// Copyright 2020 Copyright (c) 2020 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file.
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

package validation

import (
	"k8s.io/apimachinery/pkg/util/validation/field"

	v2 "github.com/gardener/component-spec/bindings-go/apis/v2"
)

// Validate validates a parsed v2 component descriptor
func ValidateList(list *v2.ComponentDescriptorList) error {
	if err := validateList(list); err != nil {
		return err.ToAggregate()
	}
	return nil
}

func validateList(list *v2.ComponentDescriptorList) field.ErrorList {
	if list == nil {
		return nil
	}
	allErrs := field.ErrorList{}

	if len(list.Metadata.Version) == 0 {
		allErrs = append(allErrs, field.Required(field.NewPath("meta").Child("schemaVersion"), "must specify a version"))
	}

	compsPath := field.NewPath("components")
	for i, comp := range list.Components {
		compPath := compsPath.Index(i)
		allErrs = append(allErrs, validate(compPath, &comp)...)
	}

	return allErrs
}

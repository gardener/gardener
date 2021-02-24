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
	"regexp"
	"unicode"

	"k8s.io/apimachinery/pkg/util/validation/field"

	v2 "github.com/gardener/component-spec/bindings-go/apis/v2"
)

// Validate validates a parsed v2 component descriptor
func Validate(component *v2.ComponentDescriptor) error {
	if err := validate(nil, component); err != nil {
		return err.ToAggregate()
	}
	return nil
}

func validate(fldPath *field.Path, component *v2.ComponentDescriptor) field.ErrorList {
	if component == nil {
		return nil
	}
	allErrs := field.ErrorList{}

	if len(component.Metadata.Version) == 0 {
		metaPath := field.NewPath("meta").Child("schemaVersion")
		if fldPath != nil {
			metaPath = fldPath.Child("meta").Child("schemaVersion")
		}
		allErrs = append(allErrs, field.Required(metaPath, "must specify a version"))
	}

	compPath := field.NewPath("component")
	if fldPath != nil {
		compPath = fldPath.Child("component")
	}

	if err := validateProvider(compPath.Child("provider"), component.Provider); err != nil {
		allErrs = append(allErrs, err)
	}

	allErrs = append(allErrs, ValidateObjectMeta(compPath, component)...)

	srcPath := compPath.Child("sources")
	allErrs = append(allErrs, ValidateSources(srcPath, component.Sources)...)

	refPath := compPath.Child("componentReferences")
	allErrs = append(allErrs, ValidateComponentReferences(refPath, component.ComponentReferences)...)

	resourcePath := compPath.Child("resources")
	allErrs = append(allErrs, ValidateResources(resourcePath, component.Resources, component.GetVersion())...)

	return allErrs
}

// ValidateObjectMeta validate the metadata of an object.
func ValidateObjectMeta(fldPath *field.Path, om v2.ObjectMetaAccessor) field.ErrorList {
	allErrs := field.ErrorList{}
	if len(om.GetName()) == 0 {
		allErrs = append(allErrs, field.Required(fldPath.Child("name"), "must specify a name"))
	}
	if len(om.GetVersion()) == 0 {
		allErrs = append(allErrs, field.Required(fldPath.Child("version"), "must specify a version"))
	}
	if len(om.GetLabels()) != 0 {
		allErrs = append(allErrs, ValidateLabels(fldPath.Child("labels"), om.GetLabels())...)
	}
	return allErrs
}

// ValidateIdentity validates the identity of object.
func ValidateIdentity(fldPath *field.Path, id v2.Identity) field.ErrorList {
	allErrs := field.ErrorList{}

	for key := range id {
		if key == v2.SystemIdentityName {
			allErrs = append(allErrs, field.Forbidden(fldPath.Key(v2.SystemIdentityName), "name is a reserved system identity label"))
		}

		if !IsASCII(key) {
			allErrs = append(allErrs, field.Forbidden(fldPath.Key(key), "key contains non-ascii characters"))
		}
		if !identityKeyValidationRegexp.Match([]byte(key)) {
			allErrs = append(allErrs, field.Invalid(fldPath.Key(key), key, identityKeyValidationErrMsg))
		}
	}
	return allErrs
}

// ValidateSources validates a list of sources.
// It makes sure that no duplicate sources are present.
func ValidateSources(fldPath *field.Path, sources []v2.Source) field.ErrorList {
	allErrs := field.ErrorList{}
	sourceIDs := make(map[string]struct{})
	for i, src := range sources {
		srcPath := fldPath.Index(i)
		allErrs = append(allErrs, ValidateSource(srcPath, src)...)

		id := string(src.GetIdentityDigest())
		if _, ok := sourceIDs[id]; ok {
			allErrs = append(allErrs, field.Duplicate(srcPath, "duplicate source"))
			continue
		}
		sourceIDs[id] = struct{}{}
	}
	return allErrs
}

// ValidateSource validates the a component's source object.
func ValidateSource(fldPath *field.Path, src v2.Source) field.ErrorList {
	allErrs := field.ErrorList{}
	if len(src.GetName()) == 0 {
		allErrs = append(allErrs, field.Required(fldPath.Child("name"), "must specify a name"))
	}
	if len(src.GetType()) == 0 {
		allErrs = append(allErrs, field.Required(fldPath.Child("type"), "must specify a type"))
	}
	allErrs = append(allErrs, ValidateIdentity(fldPath.Child("extraIdentity"), src.ExtraIdentity)...)
	return allErrs
}

// ValidateResource validates a components resource
func ValidateResource(fldPath *field.Path, res v2.Resource) field.ErrorList {
	allErrs := field.ErrorList{}
	allErrs = append(allErrs, ValidateObjectMeta(fldPath, &res)...)

	if !identityKeyValidationRegexp.Match([]byte(res.Name)) {
		allErrs = append(allErrs, field.Invalid(fldPath.Child("name"), res.Name, identityKeyValidationErrMsg))
	}

	if len(res.GetType()) == 0 {
		allErrs = append(allErrs, field.Required(fldPath.Child("type"), "must specify a type"))
	}
	if res.Access == nil {
		allErrs = append(allErrs, field.Required(fldPath.Child("access"), "must specify a access"))
	}
	allErrs = append(allErrs, ValidateIdentity(fldPath.Child("extraIdentity"), res.ExtraIdentity)...)

	return allErrs
}

func validateProvider(fldPath *field.Path, provider v2.ProviderType) *field.Error {
	if len(provider) == 0 {
		return field.Required(fldPath, "provider must be set and one of (internal, external)")
	}
	if provider != v2.InternalProvider && provider != v2.ExternalProvider {
		return field.Invalid(fldPath, "unknown provider type", "provider must be one of (internal, external)")
	}
	return nil
}

// ValidateLabels validates a list of labels.
func ValidateLabels(fldPath *field.Path, labels []v2.Label) field.ErrorList {
	allErrs := field.ErrorList{}
	labelNames := make(map[string]struct{})
	for i, label := range labels {
		labelPath := fldPath.Index(i)
		if len(label.Name) == 0 {
			allErrs = append(allErrs, field.Required(labelPath.Child("name"), "must specify a name"))
			continue
		}

		if _, ok := labelNames[label.Name]; ok {
			allErrs = append(allErrs, field.Duplicate(labelPath, "duplicate label name"))
			continue
		}
		labelNames[label.Name] = struct{}{}
	}
	return allErrs
}

// ValidateComponentReference validates a component reference.
func ValidateComponentReference(fldPath *field.Path, cr v2.ComponentReference) field.ErrorList {
	allErrs := field.ErrorList{}
	if len(cr.ComponentName) == 0 {
		allErrs = append(allErrs, field.Required(fldPath.Child("componentName"), "must specify a component name"))
	}
	allErrs = append(allErrs, ValidateObjectMeta(fldPath, &cr)...)
	return allErrs
}

// ValidateComponentReferences validates a list of component references.
// It makes sure that no duplicate sources are present.
func ValidateComponentReferences(fldPath *field.Path, refs []v2.ComponentReference) field.ErrorList {
	allErrs := field.ErrorList{}
	refIDs := make(map[string]struct{})
	for i, ref := range refs {
		refPath := fldPath.Index(i)
		allErrs = append(allErrs, ValidateComponentReference(refPath, ref)...)

		id := string(ref.GetIdentityDigest())
		if _, ok := refIDs[id]; ok {
			allErrs = append(allErrs, field.Duplicate(refPath, "duplicate component reference name"))
			continue
		}
		refIDs[id] = struct{}{}
	}
	return allErrs
}

// ValidateResources validates a list of resources.
// It makes sure that no duplicate sources are present.
func ValidateResources(fldPath *field.Path, resources []v2.Resource, componentVersion string) field.ErrorList {
	allErrs := field.ErrorList{}
	resourceIDs := make(map[string]struct{})
	for i, res := range resources {
		localPath := fldPath.Index(i)
		allErrs = append(allErrs, ValidateResource(localPath, res)...)

		// only validate the component version if it is defined
		if res.Relation == v2.LocalRelation && len(componentVersion) != 0 {
			if res.GetVersion() != componentVersion {
				allErrs = append(allErrs, field.Invalid(localPath.Child("version"), "invalid version",
					"version of local resources must match the component version"))
			}
		}

		id := string(res.GetIdentityDigest())
		if _, ok := resourceIDs[id]; ok {
			allErrs = append(allErrs, field.Duplicate(localPath, "duplicated resource"))
			continue
		}
		resourceIDs[id] = struct{}{}
	}
	return allErrs
}

const identityKeyValidationErrMsg string = "a identity label or name must consist of lower case alphanumeric characters, '-', '_' or '+', and must start and end with an alphanumeric character"

var identityKeyValidationRegexp = regexp.MustCompile("^[a-z0-9]([-_+a-z0-9]*[a-z0-9])?$")

// IsAscii checks whether a string only contains ascii characters
func IsASCII(s string) bool {
	for i := 0; i < len(s); i++ {
		if s[i] > unicode.MaxASCII {
			return false
		}
	}
	return true
}

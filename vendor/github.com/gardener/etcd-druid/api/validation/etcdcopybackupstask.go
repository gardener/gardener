// Copyright (c) 2021 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package validation

import (
	"github.com/gardener/etcd-druid/api/v1alpha1"
	apiequality "k8s.io/apimachinery/pkg/api/equality"
	apivalidation "k8s.io/apimachinery/pkg/api/validation"
	"k8s.io/apimachinery/pkg/util/validation/field"
)

// ValidateEtcdCopyBackupsTask validates a EtcdCopyBackupsTask object.
func ValidateEtcdCopyBackupsTask(task *v1alpha1.EtcdCopyBackupsTask) field.ErrorList {
	var allErrs field.ErrorList

	allErrs = append(allErrs, apivalidation.ValidateObjectMeta(&task.ObjectMeta, true, apivalidation.NameIsDNSSubdomain, field.NewPath("metadata"))...)
	allErrs = append(allErrs, ValidateEtcdCopyBackupsTaskSpec(&task.Spec, task.Name, task.Namespace, field.NewPath("spec"))...)

	return allErrs
}

// ValidateEtcdCopyBackupsTaskUpdate validates a EtcdCopyBackupsTask object before an update.
func ValidateEtcdCopyBackupsTaskUpdate(new, old *v1alpha1.EtcdCopyBackupsTask) field.ErrorList {
	var allErrs field.ErrorList

	allErrs = append(allErrs, apivalidation.ValidateObjectMetaUpdate(&new.ObjectMeta, &old.ObjectMeta, field.NewPath("metadata"))...)
	allErrs = append(allErrs, ValidateEtcdCopyBackupsTaskSpecUpdate(&new.Spec, &old.Spec, new.DeletionTimestamp != nil, field.NewPath("spec"))...)
	allErrs = append(allErrs, ValidateEtcdCopyBackupsTask(new)...)

	return allErrs
}

// ValidateEtcdCopyBackupsTaskSpec validates the specification of a EtcdCopyBackupsTask object.
func ValidateEtcdCopyBackupsTaskSpec(spec *v1alpha1.EtcdCopyBackupsTaskSpec, name, namespace string, path *field.Path) field.ErrorList {
	var allErrs field.ErrorList

	allErrs = append(allErrs, validateStore(&spec.SourceStore, name, namespace, path.Child("sourceStore"))...)
	allErrs = append(allErrs, validateStore(&spec.TargetStore, name, namespace, path.Child("targetStore"))...)

	return allErrs
}

// ValidateEtcdCopyBackupsTaskSpecUpdate validates the specification of a EtcdCopyBackupsTask object before an update.
func ValidateEtcdCopyBackupsTaskSpecUpdate(new, old *v1alpha1.EtcdCopyBackupsTaskSpec, deletionTimestampSet bool, path *field.Path) field.ErrorList {
	var allErrs field.ErrorList

	if deletionTimestampSet && !apiequality.Semantic.DeepEqual(new, old) {
		allErrs = append(allErrs, apivalidation.ValidateImmutableField(new, old, path)...)
		return allErrs
	}

	allErrs = append(allErrs, validateStoreUpdate(&new.SourceStore, &old.SourceStore, path.Child("sourceStore"))...)
	allErrs = append(allErrs, validateStoreUpdate(&new.TargetStore, &old.TargetStore, path.Child("targetStore"))...)

	return allErrs
}

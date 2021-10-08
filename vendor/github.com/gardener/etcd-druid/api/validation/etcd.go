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
	"strings"

	"github.com/gardener/etcd-druid/api/v1alpha1"

	apiequality "k8s.io/apimachinery/pkg/api/equality"
	apivalidation "k8s.io/apimachinery/pkg/api/validation"
	"k8s.io/apimachinery/pkg/util/validation/field"
)

// ValidateEtcd validates Etcd object.
func ValidateEtcd(etcd *v1alpha1.Etcd) field.ErrorList {
	var allErrs field.ErrorList

	allErrs = append(allErrs, apivalidation.ValidateObjectMeta(&etcd.ObjectMeta, true, apivalidation.NameIsDNSSubdomain, field.NewPath("metadata"))...)
	allErrs = append(allErrs, ValidateEtcdSpec(etcd, field.NewPath("spec"))...)

	return allErrs
}

// ValidateEtcdUpdate validates a Etcd object before an update.
func ValidateEtcdUpdate(new, old *v1alpha1.Etcd) field.ErrorList {
	var allErrs field.ErrorList

	allErrs = append(allErrs, apivalidation.ValidateObjectMetaUpdate(&new.ObjectMeta, &old.ObjectMeta, field.NewPath("metadata"))...)
	allErrs = append(allErrs, ValidateEtcdSpecUpdate(&new.Spec, &old.Spec, new.DeletionTimestamp != nil, field.NewPath("spec"))...)
	allErrs = append(allErrs, ValidateEtcd(new)...)

	return allErrs
}

// ValidateEtcdSpec validates the specification of an Etd object.
func ValidateEtcdSpec(etcd *v1alpha1.Etcd, path *field.Path) field.ErrorList {
	var allErrs field.ErrorList

	if etcd.Spec.Backup.Store != nil {
		prefix := etcd.Spec.Backup.Store.Prefix
		allErrs = append(allErrs, validateBackupStorePrefix(prefix, etcd.Name, etcd.Namespace, path.Child("backup.store"))...)
	}

	return allErrs
}

// ValidateEtcdSpecUpdate validates the specification of an Etcd before an update.
func ValidateEtcdSpecUpdate(new, old *v1alpha1.EtcdSpec, deletionTimestampSet bool, path *field.Path) field.ErrorList {
	var allErrs field.ErrorList

	if deletionTimestampSet && !apiequality.Semantic.DeepEqual(new, old) {
		allErrs = append(allErrs, apivalidation.ValidateImmutableField(new, old, path)...)
		return allErrs
	}

	if new.Backup.Store != nil && old.Backup.Store != nil {
		allErrs = append(allErrs, apivalidation.ValidateImmutableField(new.Backup.Store.Prefix, old.Backup.Store.Prefix, path.Child("backup.store.prefix"))...)
	}

	return allErrs
}

func validateBackupStorePrefix(prefix, name, ns string, path *field.Path) field.ErrorList {
	var allErrs field.ErrorList

	if !(strings.Contains(prefix, ns) && strings.Contains(prefix, name)) {
		allErrs = append(allErrs, field.Invalid(path.Child("prefix"), prefix, "field value must contain object name and namespace"))
	}

	return allErrs
}

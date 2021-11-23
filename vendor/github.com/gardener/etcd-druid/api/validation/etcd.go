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
	"github.com/gardener/etcd-druid/pkg/utils"

	apiequality "k8s.io/apimachinery/pkg/api/equality"
	apivalidation "k8s.io/apimachinery/pkg/api/validation"
	"k8s.io/apimachinery/pkg/util/validation/field"
)

// ValidateEtcd validates a Etcd object.
func ValidateEtcd(etcd *v1alpha1.Etcd) field.ErrorList {
	var allErrs field.ErrorList

	allErrs = append(allErrs, apivalidation.ValidateObjectMeta(&etcd.ObjectMeta, true, apivalidation.NameIsDNSSubdomain, field.NewPath("metadata"))...)
	allErrs = append(allErrs, ValidateEtcdSpec(&etcd.Spec, etcd.Name, etcd.Namespace, field.NewPath("spec"))...)

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

// ValidateEtcdSpec validates the specification of a Etcd object.
func ValidateEtcdSpec(spec *v1alpha1.EtcdSpec, name, namespace string, path *field.Path) field.ErrorList {
	var allErrs field.ErrorList

	if spec.Backup.Store != nil {
		allErrs = append(allErrs, validateStore(spec.Backup.Store, name, namespace, path.Child("backup.store"))...)
	}

	if spec.Backup.OwnerCheck != nil {
		ownerCheckPath := path.Child("backup.ownerCheck")
		if spec.Backup.OwnerCheck.Name == "" {
			allErrs = append(allErrs, field.Required(ownerCheckPath.Child("name"), "field is required"))
		}
		if spec.Backup.OwnerCheck.ID == "" {
			allErrs = append(allErrs, field.Required(ownerCheckPath.Child("id"), "field is required"))
		}
		if spec.Backup.OwnerCheck.Interval != nil {
			allErrs = append(allErrs, apivalidation.ValidateNonnegativeField(int64(spec.Backup.OwnerCheck.Interval.Duration), ownerCheckPath.Child("interval"))...)
		}
		if spec.Backup.OwnerCheck.Timeout != nil {
			allErrs = append(allErrs, apivalidation.ValidateNonnegativeField(int64(spec.Backup.OwnerCheck.Timeout.Duration), ownerCheckPath.Child("timeout"))...)
		}
		if spec.Backup.OwnerCheck.DNSCacheTTL != nil {
			allErrs = append(allErrs, apivalidation.ValidateNonnegativeField(int64(spec.Backup.OwnerCheck.DNSCacheTTL.Duration), ownerCheckPath.Child("dnsCacheTTL"))...)
		}
	}

	return allErrs
}

// ValidateEtcdSpecUpdate validates the specification of a Etcd object before an update.
func ValidateEtcdSpecUpdate(new, old *v1alpha1.EtcdSpec, deletionTimestampSet bool, path *field.Path) field.ErrorList {
	var allErrs field.ErrorList

	if deletionTimestampSet && !apiequality.Semantic.DeepEqual(new, old) {
		allErrs = append(allErrs, apivalidation.ValidateImmutableField(new, old, path)...)
		return allErrs
	}

	if new.Backup.Store != nil && old.Backup.Store != nil {
		allErrs = append(allErrs, validateStoreUpdate(new.Backup.Store, old.Backup.Store, path.Child("backup.store"))...)
	}

	return allErrs
}

func validateStore(store *v1alpha1.StoreSpec, name, namespace string, path *field.Path) field.ErrorList {
	var allErrs field.ErrorList

	if !(strings.Contains(store.Prefix, namespace) && strings.Contains(store.Prefix, name)) {
		allErrs = append(allErrs, field.Invalid(path.Child("prefix"), store.Prefix, "must contain object name and namespace"))
	}

	if store.Provider != nil && *store.Provider != "" {
		if _, err := utils.StorageProviderFromInfraProvider(store.Provider); err != nil {
			allErrs = append(allErrs, field.Invalid(path.Child("provider"), store.Provider, err.Error()))
		}
	}

	return allErrs
}

func validateStoreUpdate(new, old *v1alpha1.StoreSpec, path *field.Path) field.ErrorList {
	var allErrs field.ErrorList

	allErrs = append(allErrs, apivalidation.ValidateImmutableField(new.Prefix, old.Prefix, path.Child("prefix"))...)

	return allErrs
}

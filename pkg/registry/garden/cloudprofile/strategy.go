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

package cloudprofile

import (
	"github.com/gardener/gardener/pkg/api"
	"github.com/gardener/gardener/pkg/apis/garden"
	gardenv1beta1 "github.com/gardener/gardener/pkg/apis/garden/v1beta1"
	"github.com/gardener/gardener/pkg/apis/garden/validation"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/apimachinery/pkg/util/validation/field"
	genericapirequest "k8s.io/apiserver/pkg/endpoints/request"
	"k8s.io/apiserver/pkg/storage/names"
)

type cloudProfileStrategy struct {
	runtime.ObjectTyper
	names.NameGenerator
}

// Strategy defines the storage strategy for CloudProfiles.
var Strategy = cloudProfileStrategy{api.Scheme, names.SimpleNameGenerator}

func (cloudProfileStrategy) NamespaceScoped() bool {
	return false
}

func (cloudProfileStrategy) PrepareForCreate(ctx genericapirequest.Context, obj runtime.Object) {
	cloudprofile := obj.(*garden.CloudProfile)

	finalizers := sets.NewString(cloudprofile.Finalizers...)
	if !finalizers.Has(gardenv1beta1.GardenerName) {
		finalizers.Insert(gardenv1beta1.GardenerName)
	}
	cloudprofile.Finalizers = finalizers.UnsortedList()
}

func (cloudProfileStrategy) Validate(ctx genericapirequest.Context, obj runtime.Object) field.ErrorList {
	cloudprofile := obj.(*garden.CloudProfile)
	return validation.ValidateCloudProfile(cloudprofile)
}

func (cloudProfileStrategy) Canonicalize(obj runtime.Object) {
}

func (cloudProfileStrategy) AllowCreateOnUpdate() bool {
	return false
}

func (cloudProfileStrategy) PrepareForUpdate(ctx genericapirequest.Context, newObj, oldObj runtime.Object) {
	_ = oldObj.(*garden.CloudProfile)
	_ = newObj.(*garden.CloudProfile)
}

func (cloudProfileStrategy) AllowUnconditionalUpdate() bool {
	return true
}

func (cloudProfileStrategy) ValidateUpdate(ctx genericapirequest.Context, newObj, oldObj runtime.Object) field.ErrorList {
	oldProfile, newProfile := oldObj.(*garden.CloudProfile), newObj.(*garden.CloudProfile)
	return validation.ValidateCloudProfileUpdate(newProfile, oldProfile)
}

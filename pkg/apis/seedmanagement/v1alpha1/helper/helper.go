// Copyright (c) 2020 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

package helper

import (
	seedmanagementv1alpha1 "github.com/gardener/gardener/pkg/apis/seedmanagement/v1alpha1"
)

// Bootstrap returns the value of the Bootstrap field of the given gardenlet, or None if the field is nil.
func Bootstrap(gardenlet *seedmanagementv1alpha1.Gardenlet) seedmanagementv1alpha1.Bootstrap {
	if gardenlet.Bootstrap != nil {
		return *gardenlet.Bootstrap
	}
	return seedmanagementv1alpha1.BootstrapNone
}

// MergeWithParent returns the value of the MergeWithParent field of the given gardenlet, or false if the field is nil.
func MergeWithParent(gardenlet *seedmanagementv1alpha1.Gardenlet) bool {
	if gardenlet.MergeWithParent != nil {
		return *gardenlet.MergeWithParent
	}
	return false
}

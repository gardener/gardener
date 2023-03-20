// Copyright 2021 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

package infrastructure

import (
	"context"

	"k8s.io/apimachinery/pkg/util/validation/field"

	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
)

// ConfigValidator validates the provider config of infrastructures resource with the cloud provider.
type ConfigValidator interface {
	// Validate validates the provider config of the given infrastructure resource with the cloud provider.
	// If the returned error list is non-empty, the reconciliation will fail with an error.
	// This error will have the error code ERR_CONFIGURATION_PROBLEM, unless there is at least one error in the list
	// that has its ErrorType field set to field.ErrorTypeInternal.
	Validate(ctx context.Context, infra *extensionsv1alpha1.Infrastructure) field.ErrorList
}

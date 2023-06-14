// Copyright 2019 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

package constants

const (
	// AnnotationProtectFromDeletion is a constant for an annotation on a replica of a ManagedSeedSet
	//(either ManagedSeed or Shoot) to protect it from deletion..
	AnnotationProtectFromDeletion = "seedmanagement.gardener.cloud/protect-from-deletion"

	// AnnotationSeedSecretName is the name of the secret which is referred in the Seed spec of a ManagedSeed.
	AnnotationSeedSecretName = "seedmanagement.gardener.cloud/seed-secret-name"
	// AnnotationSeedSecretNamespace is the namespace of the secret which is referred in the Seed spec of a ManagedSeed.
	AnnotationSeedSecretNamespace = "seedmanagement.gardener.cloud/seed-secret-namespace"
)

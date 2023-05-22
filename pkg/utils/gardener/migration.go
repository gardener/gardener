// Copyright 2022 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

package gardener

// GetResponsibleSeedName returns the seed name which is responsible for the next reconciliation.
func GetResponsibleSeedName(specSeedName, statusSeedName *string) string {
	switch {
	case specSeedName == nil:
		// If the spec.seedName is empty then nobody is responsible.
		return ""

	case statusSeedName == nil:
		// If status.seedName is not set yet, the seed given in spec.seedName is responsible for reconciliation.
		return *specSeedName

	case *specSeedName != *statusSeedName:
		// Migration of the object was triggered, the seed given in status.seedName is responsible for preparing the
		// migration.
		return *statusSeedName

	default:
		return *specSeedName
	}
}

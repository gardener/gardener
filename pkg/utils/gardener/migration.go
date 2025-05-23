// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

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

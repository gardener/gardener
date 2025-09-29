// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package gardenletidentity

// UserType is used for distinguishing between clients running on a seed cluster when authenticating against the garden
// cluster.
type UserType string

const (
	// UserTypeGardenlet is the UserType of a gardenlet client.
	UserTypeGardenlet UserType = "gardenlet"
	// UserTypeExtension is the UserType of an extension client.
	UserTypeExtension UserType = "extension"
)

// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package kubernetes

// DeleteManifestOption is some configuration that modifies options for a delete request.
type DeleteManifestOption interface {
	// MutateDeleteManifestOptions applies this configuration to the given delete options.
	MutateDeleteManifestOptions(opts *DeleteManifestOptions)
}

// DeleteManifestOptions contains options for delete requests.
type DeleteManifestOptions struct {
	// TolerateErrorFuncs are functions for which errors are tolerated.
	TolerateErrorFuncs []TolerateErrorFunc
}

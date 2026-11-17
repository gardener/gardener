// SPDX-FileCopyrightText: Contributors to the Gardener project
//
// SPDX-License-Identifier: Apache-2.0

package gardenlet

import "k8s.io/client-go/rest"

// GardenClientRestConfigWithInlineCA is exported for testing.
func GardenClientRestConfigWithInlineCA(config *rest.Config) (*rest.Config, error) {
	return gardenClientRestConfigWithInlineCA(config)
}

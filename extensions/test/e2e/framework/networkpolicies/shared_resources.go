// SPDX-FileCopyrightText: 2019 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package networkpolicies

import (
	networkingv1 "k8s.io/api/networking/v1"
)

// SharedResources are shared between Ginkgo Nodes.
type SharedResources struct {
	Mirror            string                       `json:"mirror"`
	External          string                       `json:"external"`
	SeedNodeIP        string                       `json:"seedNodeIP"`
	Policies          []networkingv1.NetworkPolicy `json:"policies"`
	SeedCloudProvider string                       `json:"seedCloudProvider"`
}

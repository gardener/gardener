// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package nodeagentauthorizer

import (
	"slices"
	"strings"

	"k8s.io/apiserver/pkg/authentication/user"

	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
)

// GetNodeAgentIdentity returns the node name and a boolean which indicates whether the user is a gardener-node-agent.
func GetNodeAgentIdentity(u user.Info) (string, bool) {
	if u == nil {
		return "", false
	}

	if !strings.HasPrefix(u.GetName(), v1beta1constants.NodeAgentUserNamePrefix) || !slices.Contains(u.GetGroups(), v1beta1constants.NodeAgentsGroup) {
		return "", false
	}

	return strings.TrimPrefix(u.GetName(), v1beta1constants.NodeAgentUserNamePrefix), true
}

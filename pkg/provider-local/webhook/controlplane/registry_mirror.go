// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package controlplane

import (
	"fmt"
)

// RegistryMirror represents a registry mirror for containerd.
type RegistryMirror struct {
	UpstreamHost   string
	UpstreamServer string
	MirrorHost     string
}

// HostsTOML returns hosts.toml configuration.
func (r *RegistryMirror) HostsTOML() string {
	const hostsTOMLTemplate = `server = "%s"

[host."%s"]
  capabilities = ["pull", "resolve"]
`
	return fmt.Sprintf(hostsTOMLTemplate, r.UpstreamServer, r.MirrorHost)
}

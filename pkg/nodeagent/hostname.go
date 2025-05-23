// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package nodeagent

import (
	"os"
	"strings"
)

// Hostname is an alias for os.Hostname.
// Exposed for testing.
var Hostname = os.Hostname

// GetHostName gets the hostname and converts it to lowercase.
func GetHostName() (string, error) {
	hostName, err := Hostname()
	if err != nil {
		return "", err
	}
	return strings.ToLower(hostName), nil
}

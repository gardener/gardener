// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package unmanagedinfra

import (
	"strconv"
)

func machineContainerName(ordinal int) string {
	return "gind-machine-" + strconv.Itoa(ordinal)
}

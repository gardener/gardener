// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package unmanagedinfra

import (
	"strconv"
)

const (
	namespace       = "gardenadm-unmanaged-infra"
	statefulSetName = "machine"
)

func machinePodName(ordinal int) string {
	return statefulSetName + "-" + strconv.Itoa(ordinal)
}

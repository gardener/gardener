// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package hightouch

import (
	"strconv"
)

const (
	namespace       = "gardenadm-high-touch"
	statefulSetName = "machine"
)

func machinePodName(ordinal int) string {
	return statefulSetName + "-" + strconv.Itoa(ordinal)
}

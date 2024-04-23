// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package framework

import (
	"github.com/onsi/gomega"
)

// ExpectNoError checks if an error has occurred
func ExpectNoError(actual interface{}, extra ...interface{}) {
	gomega.ExpectWithOffset(1, actual, extra...).ToNot(gomega.HaveOccurred())
}

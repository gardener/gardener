// SPDX-FileCopyrightText: 2018 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package resourcereferencemanager_test

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"testing"
)

func TestResourcereferencemanager(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Admission ResourceReferenceManager Suite")
}

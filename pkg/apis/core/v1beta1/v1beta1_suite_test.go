// SPDX-FileCopyrightText: 2018 SAP SE or an SAP affiliate company and Gardener contributors
// SPDX-License-Identifier: Apache-2.0

package v1beta1_test

import (
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func TestV1beta1(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "APIs Core V1beta1 Suite")
}

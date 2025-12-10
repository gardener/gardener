// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package kubernetes_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/gardener/gardener/pkg/utils/kubernetes"
	"github.com/gardener/gardener/pkg/utils/test"
)

var _ = Describe("serviceaccount", func() {
	Describe("#ExtractServiceAccountUID", func() {
		It("should extract the ServiceAccount UID from a valid Kubernetes token", func() {
			uid := "uid-abc-123"
			token := test.SampleServiceAccountToken(uid)
			Expect(kubernetes.ExtractServiceAccountUID(token)).To(Equal(uid))
		})

		It("should return an empty uid result if the provided token was invalid", func() {
			Expect(kubernetes.ExtractServiceAccountUID("")).To(Equal(""))
			Expect(kubernetes.ExtractServiceAccountUID("invalid_token")).To(Equal(""))
		})
	})
})

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

var _ = Describe("ServiceAccount", func() {
	Describe("#ExtractServiceAccountUID", func() {
		It("should extract the ServiceAccount UID from a valid Kubernetes token", func() {
			uid := "uid-abc-123"
			token := test.SampleServiceAccountToken(uid)
			extractedUID, err := kubernetes.ExtractServiceAccountUID(token)
			Expect(err).NotTo(HaveOccurred())
			Expect(extractedUID).To(Equal(uid))
		})

		It("should return an empty uid result if the provided token was invalid", func() {
			_, err := kubernetes.ExtractServiceAccountUID("")
			Expect(err).To(HaveOccurred())
			_, err = kubernetes.ExtractServiceAccountUID("invalid_token")
			Expect(err).To(HaveOccurred())
		})
	})
})

// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package kubernetes_test

import (
	"errors"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"go.uber.org/goleak"
	"go.uber.org/mock/gomock"

	. "github.com/gardener/gardener/pkg/client/kubernetes"
	"github.com/gardener/gardener/pkg/client/kubernetes/fake"
)

var _ = Describe("Pods", func() {
	var (
		ctrl *gomock.Controller
	)

	BeforeEach(func() {
		ctrl = gomock.NewController(GinkgoT())
	})

	AfterEach(func() {
		ctrl.Finish()
	})

	Describe("#CheckForwardPodPort", func() {
		It("should create a forward connection successfully", func() {
			defer goleak.VerifyNone(GinkgoT(), goleak.IgnoreCurrent())
			fw := fake.PortForwarder{
				ReadyChan: make(chan struct{}, 1),
				DoneChan:  make(chan struct{}, 1),
			}
			close(fw.ReadyChan)
			defer close(fw.DoneChan)

			Expect(CheckForwardPodPort(fw)).To(Succeed())
		})

		It("should return error if port forward fails", func() {
			defer goleak.VerifyNone(GinkgoT(), goleak.IgnoreCurrent())
			fw := fake.PortForwarder{
				Err:      errors.New("foo"),
				DoneChan: make(chan struct{}, 1),
			}
			close(fw.DoneChan)

			Expect(CheckForwardPodPort(fw)).To(MatchError(ContainSubstring("foo")))
		})
	})
})

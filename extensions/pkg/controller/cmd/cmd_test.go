// SPDX-FileCopyrightText: 2019 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package cmd

import (
	"errors"

	"github.com/gardener/gardener/extensions/pkg/util/test"
	mocklogr "github.com/gardener/gardener/pkg/mock/go-logr/logr"

	"github.com/golang/mock/gomock"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

var _ = Describe("Cmd", func() {
	var (
		ctrl *gomock.Controller
	)

	BeforeEach(func() {
		ctrl = gomock.NewController(GinkgoT())
	})
	AfterEach(func() {
		ctrl.Finish()
	})

	Describe("#LogErrAndExit", func() {
		It("should log the error and exit", func() {
			called := false

			defer test.WithVar(&Exit, func(code int) {
				called = true
				Expect(code).To(Equal(1))
			})()

			defer test.WithVar(&Log, log.NewDelegatingLogger(log.NullLogger{}))()

			logger := mocklogr.NewMockLogger(ctrl)
			err := errors.New("error")
			msg := "msg"

			logger.EXPECT().Error(err, msg, []interface{}{})
			Log.Fulfill(logger)

			LogErrAndExit(err, msg)

			Expect(called).To(BeTrue())
		})
	})
})

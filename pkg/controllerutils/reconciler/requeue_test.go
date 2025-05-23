// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package reconciler_test

import (
	"fmt"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	. "github.com/gardener/gardener/pkg/controllerutils/reconciler"
)

var _ = Describe("Requeue", func() {
	var (
		cause        = fmt.Errorf("cause")
		requeueAfter = time.Hour
	)

	DescribeTable("#Error",
		func(err *RequeueAfterError, expectedMsg string) {
			Expect(err.Error()).To(Equal(expectedMsg))
		},

		Entry("w/o cause", &RequeueAfterError{RequeueAfter: requeueAfter}, "requeue in "+requeueAfter.String()),
		Entry("w/ cause", &RequeueAfterError{Cause: cause, RequeueAfter: requeueAfter}, "requeue in "+requeueAfter.String()+" due to "+cause.Error()),
	)
})

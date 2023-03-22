// Copyright 2021 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package reconciler_test

import (
	"fmt"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	. "github.com/gardener/gardener/pkg/controllerutils/reconciler"
)

var _ = Describe("Reconcile", func() {
	var (
		fakeErr           = fmt.Errorf("fake")
		cause             = fmt.Errorf("cause")
		requeueAfter      = time.Hour
		requeueAfterError = &RequeueAfterError{Cause: cause, RequeueAfter: requeueAfter}
	)

	Describe("#ReconcileErr", func() {
		It("should return the correct result if it's no RequeueAfterError", func() {
			res, err := ReconcileErr(fakeErr)
			Expect(err).To(MatchError(fakeErr))
			Expect(res).To(Equal(reconcile.Result{}))
		})

		It("should return the correct result if it's a RequeueAfterError", func() {
			res, err := ReconcileErr(requeueAfterError)
			Expect(err).To(BeNil())
			Expect(res).To(Equal(reconcile.Result{Requeue: true, RequeueAfter: requeueAfter}))
		})
	})

	Describe("#ReconcileErrCause", func() {
		It("should return the correct result if it's no RequeueAfterError", func() {
			Expect(ReconcileErrCause(fakeErr)).To(MatchError(fakeErr))
		})

		It("should return the correct result if it's a RequeueAfterError", func() {
			Expect(ReconcileErrCause(requeueAfterError)).To(Equal(cause))
		})
	})

	Describe("#ReconcileErrCauseOrErr", func() {
		It("should return the correct result if it's no RequeueAfterError", func() {
			Expect(ReconcileErrCauseOrErr(fakeErr)).To(MatchError(fakeErr))
		})

		It("should return the correct result if it's a RequeueAfterError", func() {
			Expect(ReconcileErrCauseOrErr(requeueAfterError)).To(Equal(cause))
		})
	})
})

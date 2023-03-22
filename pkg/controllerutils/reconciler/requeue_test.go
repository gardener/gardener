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

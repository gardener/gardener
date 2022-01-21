// Copyright (c) 2019 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

package framework

import (
	"context"
	"time"

	"github.com/onsi/ginkgo/v2"
)

// CIt  contextifies Gingko's It
func CIt(text string, body func(context.Context), timeout time.Duration) {
	ginkgo.It(text, contextify(body, timeout), timeout.Seconds())
}

// FCIt contextifies Gingko's FIt
func FCIt(text string, body func(context.Context), timeout time.Duration) {
	ginkgo.FIt(text, contextify(body, timeout), timeout.Seconds())
}

// CAfterSuite contextifies Gingko's FIt
func CAfterSuite(body func(context.Context), timeout time.Duration) {
	ginkgo.AfterSuite(contextify(body, timeout))
}

// CAfterEach contextifies Gingko's AfterEach
func CAfterEach(body func(context.Context), timeout time.Duration) {
	ginkgo.AfterEach(contextify(body, timeout), timeout.Seconds())
}

// CBeforeSuite contextifies Gingko's FIt
func CBeforeSuite(body func(context.Context), timeout time.Duration) {
	ginkgo.BeforeSuite(contextify(body, timeout))
}

// CBeforeEach contextifies Gingko's BeforeEach
func CBeforeEach(body func(ctx context.Context), timeout time.Duration) {
	ginkgo.BeforeEach(contextify(body, timeout), timeout.Seconds())
}

// CJustBeforeEach contextifies Gingko's JustBeforeEach
func CJustBeforeEach(body func(ctx context.Context), timeout time.Duration) {
	ginkgo.JustBeforeEach(contextify(body, timeout), timeout.Seconds())
}

func contextify(body func(context.Context), timeout time.Duration) func() {
	return func() {
		ctx, cancel := context.WithTimeout(context.Background(), timeout)
		defer cancel()

		body(ctx)
	}
}

// Copyright (c) 2020 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

package healthz_test

import (
	"net/http"

	. "github.com/gardener/gardener/pkg/healthz"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Healthz", func() {
	Describe("#HandlerFunc", func() {
		var (
			healthz  Manager
			response *fakeResponse
		)

		BeforeEach(func() {
			healthz = NewDefaultHealthz()
			healthz.Start()
			response = &fakeResponse{}
		})

		It("should return a function that sends 200 OK when the health check passes", func() {
			healthz.Set(true)
			HandlerFunc(healthz)(response, nil)
			Expect(response.status).To(Equal(200))
		})

		It("should return a function that sends 500 Internal Server Error when the health check does not pass", func() {
			healthz.Set(false)
			HandlerFunc(healthz)(response, nil)
			Expect(response.status).To(Equal(500))
		})
	})
})

type fakeResponse struct {
	headers http.Header
	body    []byte
	status  int
}

func (r *fakeResponse) Header() http.Header {
	return r.headers
}

func (r *fakeResponse) Write(body []byte) (int, error) {
	r.body = body
	return len(body), nil
}

func (r *fakeResponse) WriteHeader(status int) {
	r.status = status
}

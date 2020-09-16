// SPDX-FileCopyrightText: 2020 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

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

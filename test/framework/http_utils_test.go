// Copyright 2019 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

package framework_test

import (
	"context"
	"net/http"
	"net/http/httptest"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/gardener/gardener/test/framework"
)

var _ = Describe("HTTP Utils tests", func() {

	var (
		httpHandlerFunc func(http.ResponseWriter, *http.Request)
		server          *httptest.Server
	)

	JustBeforeEach(func() {
		server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			httpHandlerFunc(w, r)
		}))
	})

	AfterEach(func() {
		server.Close()
	})

	It("Should perform a basic http get request", func() {
		var called int
		httpHandlerFunc = func(w http.ResponseWriter, r *http.Request) {
			Expect(r.Method).To(Equal(http.MethodGet))
			called++
			_, err := w.Write(nil)
			Expect(err).ToNot(HaveOccurred())
		}
		_, err := framework.HTTPGet(context.TODO(), server.URL)
		Expect(err).ToNot(HaveOccurred())

		Expect(called).To(Equal(1))
	})

	Context("Basic Auth", func() {

		It("Should succeed if the endpoints accepts the credentials", func() {
			httpHandlerFunc = func(w http.ResponseWriter, r *http.Request) {
				Expect(r.Method).To(Equal(http.MethodGet))
				Expect(r.Header.Get("Authorization")).To(Equal("Basic dGVzdDp0ZXN0"), "credentials should be test test")

				_, err := w.Write(nil)
				Expect(err).ToNot(HaveOccurred())
			}
			err := framework.TestHTTPEndpointWithBasicAuth(context.TODO(), server.URL, "test", "test")
			Expect(err).ToNot(HaveOccurred())
		})

		It("Should fail if the endpoints declines the credentials", func() {
			httpHandlerFunc = func(w http.ResponseWriter, r *http.Request) {
				Expect(r.Method).To(Equal(http.MethodGet))

				w.WriteHeader(http.StatusUnauthorized)
				_, err := w.Write(nil)
				Expect(err).ToNot(HaveOccurred())
			}
			err := framework.TestHTTPEndpointWithBasicAuth(context.TODO(), server.URL, "test", "test")
			Expect(err).To(HaveOccurred())
		})
	})

	Context("Bearer Token", func() {

		It("Should succeed if the endpoints accepts the token", func() {
			httpHandlerFunc = func(w http.ResponseWriter, r *http.Request) {
				Expect(r.Method).To(Equal(http.MethodGet))
				Expect(r.Header.Get("Authorization")).To(Equal("Bearer testtoken"), "the token should be testtoken")

				_, err := w.Write(nil)
				Expect(err).ToNot(HaveOccurred())
			}
			err := framework.TestHTTPEndpointWithToken(context.TODO(), server.URL, "testtoken")
			Expect(err).ToNot(HaveOccurred())
		})

		It("Should fail if the endpoints declines the token", func() {
			httpHandlerFunc = func(w http.ResponseWriter, r *http.Request) {
				Expect(r.Method).To(Equal(http.MethodGet))

				w.WriteHeader(http.StatusUnauthorized)
				_, err := w.Write(nil)
				Expect(err).ToNot(HaveOccurred())
			}
			err := framework.TestHTTPEndpointWithToken(context.TODO(), server.URL, "testtoken")
			Expect(err).To(HaveOccurred())
		})
	})

})

// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

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

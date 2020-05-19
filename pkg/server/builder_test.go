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

package server

import (
	"net/http"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gstruct"
)

var _ = Describe("Builder", func() {
	Describe("#NewBuilder", func() {
		It("should correctly create a builder with defaults", func() {
			b := NewBuilder()
			Expect(b.bindAddress).To(Equal("0.0.0.0"))
			Expect(b.port).To(Equal(8080))
			Expect(b.handlers).To(Equal(make(map[string]http.Handler)))
			Expect(b.handlerFuncs).To(Equal(make(map[string]http.HandlerFunc)))
		})
	})

	Context("<Builder>", func() {
		var b *Builder

		BeforeEach(func() {
			b = NewBuilder()
		})

		Describe("#WithBindAddress", func() {
			It("should correctly set the field", func() {
				value := "foo"
				Expect(b.WithBindAddress(value)).To(BeAssignableToTypeOf(&Builder{}))
				Expect(b.bindAddress).To(Equal(value))
			})
		})

		Describe("#WithPort", func() {
			It("should correctly set the field", func() {
				value := 1234
				Expect(b.WithPort(value)).To(BeAssignableToTypeOf(&Builder{}))
				Expect(b.port).To(Equal(value))
			})
		})

		Describe("#WithTLS", func() {
			It("should correctly set the fields", func() {
				value1, value2 := "foo", "bar"
				Expect(b.WithTLS(value1, value2)).To(BeAssignableToTypeOf(&Builder{}))
				Expect(b.tlsCertPath).To(PointTo(Equal(value1)))
				Expect(b.tlsKeyPath).To(PointTo(Equal(value2)))
			})
		})

		Describe("#WithHandlers", func() {
			It("should correctly set the field", func() {
				value := map[string]http.Handler{"foo": nil}
				Expect(b.WithHandlers(value)).To(BeAssignableToTypeOf(&Builder{}))
				Expect(b.handlers).To(Equal(value))
			})
		})

		Describe("#WithHandler", func() {
			It("should correctly set the field", func() {
				value1 := "foo"
				var value2 http.Handler
				Expect(b.WithHandler(value1, value2)).To(BeAssignableToTypeOf(&Builder{}))
				Expect(b.handlers).To(HaveKeyWithValue(value1, BeNil()))
			})
		})

		Describe("#WithHandlerFuncs", func() {
			It("should correctly set the field", func() {
				value := map[string]http.HandlerFunc{"foo": nil}
				Expect(b.WithHandlerFuncs(value)).To(BeAssignableToTypeOf(&Builder{}))
				Expect(b.handlerFuncs).To(Equal(value))
			})
		})

		Describe("#WithHandlerFunc", func() {
			It("should correctly set the field", func() {
				value1 := "foo"
				var value2 http.HandlerFunc
				Expect(b.WithHandlerFunc(value1, value2)).To(BeAssignableToTypeOf(&Builder{}))
				Expect(b.handlerFuncs).To(HaveKeyWithValue(value1, BeNil()))
			})
		})

		Describe("#Build", func() {
			var (
				bindAddress  = "foo"
				port         = 1234
				handlers     = map[string]http.Handler{"foo": nil}
				handlerFuncs = map[string]http.HandlerFunc{"foo": nil}
			)

			It("should correctly build a HTTP server", func() {
				Expect(b.WithBindAddress(bindAddress)).To(BeAssignableToTypeOf(&Builder{}))
				Expect(b.WithPort(port)).To(BeAssignableToTypeOf(&Builder{}))
				Expect(b.WithHandlers(handlers)).To(BeAssignableToTypeOf(&Builder{}))
				Expect(b.WithHandlerFuncs(handlerFuncs)).To(BeAssignableToTypeOf(&Builder{}))

				server := b.Build()

				Expect(server.bindAddress).To(Equal(bindAddress))
				Expect(server.port).To(Equal(port))
				Expect(server.handlers).To(Equal(handlers))
				Expect(server.handlerFuncs).To(Equal(handlerFuncs))
				Expect(server.tlsCertPath).To(BeNil())
				Expect(server.tlsKeyPath).To(BeNil())
			})

			It("should correctly build a HTTPS server", func() {
				var (
					tlsCertPath = "/some/path/to/cert"
					tlsKeyPath  = "/some/path/to/key"
				)

				Expect(b.WithBindAddress(bindAddress)).To(BeAssignableToTypeOf(&Builder{}))
				Expect(b.WithPort(port)).To(BeAssignableToTypeOf(&Builder{}))
				Expect(b.WithTLS(tlsCertPath, tlsKeyPath)).To(BeAssignableToTypeOf(&Builder{}))
				Expect(b.WithHandlers(handlers)).To(BeAssignableToTypeOf(&Builder{}))
				Expect(b.WithHandlerFuncs(handlerFuncs)).To(BeAssignableToTypeOf(&Builder{}))

				server := b.Build()

				Expect(server.bindAddress).To(Equal(bindAddress))
				Expect(server.port).To(Equal(port))
				Expect(server.handlers).To(Equal(handlers))
				Expect(server.handlerFuncs).To(Equal(handlerFuncs))
				Expect(server.tlsCertPath).To(PointTo(Equal(tlsCertPath)))
				Expect(server.tlsKeyPath).To(PointTo(Equal(tlsKeyPath)))
			})
		})
	})
})

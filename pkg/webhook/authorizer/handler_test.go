// SPDX-FileCopyrightText: Contributors to the Gardener project
//
// SPDX-License-Identifier: Apache-2.0

package authorizer_test

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"go.uber.org/zap/zapcore"
	"k8s.io/apiserver/pkg/authorization/authorizer"
	logzap "sigs.k8s.io/controller-runtime/pkg/log/zap"

	"github.com/gardener/gardener/pkg/logger"
	"github.com/gardener/gardener/pkg/utils/test"
	. "github.com/gardener/gardener/pkg/webhook/authorizer"
)

var _ = Describe("Handler", func() {
	var (
		log = logger.MustNewZapLogger(logger.DebugLevel, logger.FormatJSON, logzap.WriteTo(GinkgoWriter), logzap.Level(zapcore.Level(0)))

		handler      *Handler
		respRecorder *httptest.ResponseRecorder
	)

	BeforeEach(func() {
		handler = &Handler{Logger: log, Authorizer: &test.FakeAuthorizer{Fn: test.Allow}}
		respRecorder = &httptest.ResponseRecorder{
			Body: bytes.NewBuffer(nil),
		}
	})

	Describe("#Handle", func() {
		It("should write an erroneous response because the request body is empty", func() {
			req := &http.Request{Body: nil}

			handler.ServeHTTP(respRecorder, req)

			Expect(respRecorder.Body.String()).To(Equal(`{"kind":"SubjectAccessReview","apiVersion":"authorization.k8s.io/v1","metadata":{},"spec":{},"status":{"allowed":false,"evaluationError":"422 request body is empty"}}
`))
		})

		It("should write an erroneous response because the body cannot be read", func() {
			req := &http.Request{
				Header: http.Header{"Content-Type": []string{"foo"}},
				Body:   &errReader{},
			}

			handler.ServeHTTP(respRecorder, req)

			Expect(respRecorder.Body.String()).To(Equal(`{"kind":"SubjectAccessReview","apiVersion":"authorization.k8s.io/v1","metadata":{},"spec":{},"status":{"allowed":false,"evaluationError":"400 fake-err"}}
`))
		})

		It("should write an erroneous response because the content type is invalid", func() {
			req := &http.Request{
				Header: http.Header{"Content-Type": []string{"foo"}},
				Body:   nopCloser{Reader: bytes.NewBuffer(nil)},
			}

			handler.ServeHTTP(respRecorder, req)

			Expect(respRecorder.Body.String()).To(Equal(`{"kind":"SubjectAccessReview","apiVersion":"authorization.k8s.io/v1","metadata":{},"spec":{},"status":{"allowed":false,"evaluationError":"400 contentType=foo, expected application/json"}}
`))
		})

		It("should write an erroneous response because the body is invalid", func() {
			req := &http.Request{
				Header: http.Header{"Content-Type": []string{"application/json"}},
				Body:   nopCloser{Reader: bytes.NewBufferString("{")},
			}

			handler.ServeHTTP(respRecorder, req)

			Expect(respRecorder.Body.String()).To(Equal(`{"kind":"SubjectAccessReview","apiVersion":"authorization.k8s.io/v1","metadata":{},"spec":{},"status":{"allowed":false,"evaluationError":"400 couldn't get version/kind; json parse error: unexpected end of JSON input"}}
`))
		})

		DescribeTable("authorizer consultation",
			func(fn func(context.Context, authorizer.Attributes) (authorizer.Decision, string, error), timeout time.Duration, expectedStatus string) {
				defer test.WithVar(&DecisionTimeout, timeout)()

				req := &http.Request{
					Header: http.Header{"Content-Type": []string{"application/json"}},
					Body:   nopCloser{Reader: bytes.NewBufferString(`{"apiVersion":"authorization.k8s.io/v1","kind":"SubjectAccessReview"}`)},
				}

				handler = &Handler{Logger: log, Authorizer: &test.FakeAuthorizer{Fn: fn}}
				handler.ServeHTTP(respRecorder, req)

				Expect(respRecorder.Body.String()).To(Equal(`{"kind":"SubjectAccessReview","apiVersion":"authorization.k8s.io/v1","metadata":{},"spec":{},"status":{` + expectedStatus + `}}
`))
			},

			Entry("error", test.Err, DecisionTimeout, `"allowed":false,"evaluationError":"500 fake-err"`),
			Entry("allow", test.Allow, DecisionTimeout, `"allowed":true`),
			Entry("deny", test.Deny, DecisionTimeout, `"allowed":false,"denied":true,"reason":"deny"`),
			Entry("no opinion", test.NoOpinion, DecisionTimeout, `"allowed":false,"reason":"noopinion"`),
			Entry("unexpected decision", test.UnexpectedDecision, DecisionTimeout, `"allowed":false,"evaluationError":"500 unexpected decision: -1"`),
			Entry("timeout", test.Timeout, time.Millisecond, `"allowed":false,"evaluationError":"500 context deadline exceeded"`),
		)

		It("should respect the sent apiVersion in the request", func() {
			req := &http.Request{
				Header: http.Header{"Content-Type": []string{"application/json"}},
				Body:   nopCloser{Reader: bytes.NewBufferString(`{"kind":"SubjectAccessReview","apiVersion":"authorization.k8s.io/v1beta1"}`)},
			}

			handler.ServeHTTP(respRecorder, req)

			Expect(respRecorder.Body.String()).To(Equal(`{"kind":"SubjectAccessReview","apiVersion":"authorization.k8s.io/v1beta1","metadata":{},"spec":{},"status":{"allowed":true}}
`))
		})
	})
})

type nopCloser struct{ io.Reader }

func (nopCloser) Close() error { return nil }

type errReader struct{ nopCloser }

func (errReader) Read([]byte) (n int, err error) {
	return 0, fmt.Errorf("fake-err")
}

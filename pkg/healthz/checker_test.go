// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package healthz_test

import (
	"context"
	"errors"
	"io"
	"net/http"
	"strings"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"k8s.io/apimachinery/pkg/runtime/serializer"
	fakerestclient "k8s.io/client-go/rest/fake"
	"sigs.k8s.io/controller-runtime/pkg/healthz"

	"github.com/gardener/gardener/pkg/client/kubernetes"
	. "github.com/gardener/gardener/pkg/healthz"
)

var _ = Describe("Checker", func() {
	Describe("#NewAPIServerHealthz", func() {
		var (
			ctx            = context.TODO()
			fakeRESTClient *fakerestclient.RESTClient
			checker        healthz.Checker
		)

		BeforeEach(func() {
			fakeRESTClient = &fakerestclient.RESTClient{
				NegotiatedSerializer: serializer.NewCodecFactory(kubernetes.GardenScheme).WithoutConversion(),
				Resp: &http.Response{
					StatusCode: http.StatusOK,
					Body:       io.NopCloser(strings.NewReader("")),
				},
			}
			checker = NewAPIServerHealthz(ctx, fakeRESTClient)
		})

		It("should return no error because the request succeeded", func() {
			Expect(checker(nil)).To(Succeed())
		})

		It("should return an error because the request failed", func() {
			fakeErr := errors.New("fake err")
			fakeRESTClient.Err = fakeErr

			Expect(checker(nil)).To(MatchError(fakeErr))
		})

		It("should return an error because the response was not 200 OK", func() {
			fakeRESTClient.Resp.StatusCode = http.StatusAccepted

			Expect(checker(nil)).To(MatchError(ContainSubstring("failed talking to the source cluster's kube-apiserver")))
		})
	})
})

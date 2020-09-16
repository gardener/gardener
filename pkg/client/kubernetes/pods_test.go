// SPDX-FileCopyrightText: 2018 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package kubernetes_test

import (
	"context"
	"io"
	"net/http"
	"net/url"

	"github.com/golang/mock/gomock"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/client-go/rest"
	fakerestclient "k8s.io/client-go/rest/fake"

	. "github.com/gardener/gardener/pkg/client/kubernetes"
	mockcorev1 "github.com/gardener/gardener/pkg/mock/client-go/core/v1"
	mockio "github.com/gardener/gardener/pkg/mock/go/io"
)

var _ = Describe("Pods", func() {
	var (
		ctx  context.Context
		ctrl *gomock.Controller
		pods *mockcorev1.MockPodInterface
	)

	BeforeEach(func() {
		ctx = context.Background()
		ctrl = gomock.NewController(GinkgoT())
		pods = mockcorev1.NewMockPodInterface(ctrl)
	})

	AfterEach(func() {
		ctrl.Finish()
	})

	Describe("#GetPodLogs", func() {
		It("should read all pod logs and close the stream", func() {
			const name = "name"
			var (
				options = &corev1.PodLogOptions{}
				logs    = []byte("logs")
				body    = mockio.NewMockReadCloser(ctrl)
				client  = fakerestclient.CreateHTTPClient(func(_ *http.Request) (*http.Response, error) {
					return &http.Response{StatusCode: http.StatusOK, Body: body}, nil
				})
			)

			gomock.InOrder(
				pods.EXPECT().GetLogs(name, options).Return(rest.NewRequestWithClient(&url.URL{}, "", rest.ClientContentConfig{}, client)),
				body.EXPECT().Read(gomock.Any()).DoAndReturn(func(data []byte) (int, error) {
					copy(data, logs)
					return len(logs), io.EOF
				}),
				body.EXPECT().Close(),
			)

			actual, err := GetPodLogs(ctx, pods, name, options.DeepCopy())
			Expect(err).NotTo(HaveOccurred())
			Expect(actual).To(Equal(logs))
		})
	})
})

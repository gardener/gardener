// Copyright 2022 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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
	"context"
	"fmt"
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
			fakeErr := fmt.Errorf("fake err")
			fakeRESTClient.Err = fakeErr

			Expect(checker(nil)).To(MatchError(fakeErr))
		})

		It("should return an error because the response was not 200 OK", func() {
			fakeRESTClient.Resp.StatusCode = http.StatusAccepted

			Expect(checker(nil)).To(MatchError(ContainSubstring("failed talking to the source cluster's kube-apiserver")))
		})
	})
})

// Copyright (c) 2021 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

package controller

import (
	"context"

	"github.com/gardener/gardener/landscaper/pkg/controlplane/apis/imports"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	"github.com/gardener/gardener/pkg/client/kubernetes/fake"
	"github.com/gardener/gardener/pkg/logger"
	mockclient "github.com/gardener/gardener/pkg/mock/controller-runtime/client"
	kutil "github.com/gardener/gardener/pkg/utils/kubernetes"
	"github.com/golang/mock/gomock"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/sirupsen/logrus"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var _ = Describe("#GetOrDefaultDiffieHellmannKey", func() {
	var (
		testOperation operation

		expectedDefaultDiffieHellmanKey = `-----BEGIN DH PARAMETERS-----
MIIBCAKCAQEA7cBXxG9an6KRz/sB5uiSOTf7Eg+uWVkhXO4peKDTARzMYa8b7WR8
B/Aw+AyUXtB3tXtrzeC5M3IHnuhFwMo3K4oSOkFJxatLlYKeY15r+Kt5vnOOT3BW
eN5OnWlR5Wi7GZBWbaQgXVR79N4yst43sVhJus6By0lN6Olc9xD/ys9GH/ykJVIh
Z/NLrxAC5lxjwCqJMd8hrryChuDlz597vg6gYFuRV60U/YU4DK71F4H7mI07aGJ9
l+SK8TbkKWF5ITI7kYWbc4zmtfXSXaGjMhM9omQUaTH9csB96hzFJdeZ4XjxybRf
Vc3t7XP5q7afeaKmM3FhSXdeHKCTqQzQuwIBAg==
-----END DH PARAMETERS-----
`
	)

	// mocking
	var (
		ctx               = context.TODO()
		ctrl              *gomock.Controller
		mockRuntimeClient *mockclient.MockClient
		runtimeClient     kubernetes.Interface
		errNotFound       = &apierrors.StatusError{ErrStatus: metav1.Status{Reason: metav1.StatusReasonNotFound}}
	)

	AfterEach(func() {
		ctrl.Finish()
	})

	BeforeEach(func() {
		ctrl = gomock.NewController(GinkgoT())
		mockRuntimeClient = mockclient.NewMockClient(ctrl)

		runtimeClient = fake.NewClientSetBuilder().WithClient(mockRuntimeClient).Build()

		testOperation = operation{
			log:           logrus.NewEntry(logger.NewNopLogger()),
			runtimeClient: runtimeClient,
			imports:       &imports.Imports{},
		}
	})

	It("should return the default Diffie Hellman Key - secret not found", func() {
		mockRuntimeClient.EXPECT().Get(ctx, kutil.Key("garden", "openvpn-diffie-hellman-key"), gomock.AssignableToTypeOf(&corev1.Secret{})).Return(errNotFound)

		Expect(testOperation.GetOrDefaultDiffieHellmannKey(ctx)).ToNot(HaveOccurred())
		Expect(testOperation.imports.OpenVPNDiffieHellmanKey).To(Equal(&expectedDefaultDiffieHellmanKey))
	})

	It("should return the default Diffie Hellman Key - secret does not contain expected key", func() {
		expectedKey := "my diffy"
		mockRuntimeClient.EXPECT().Get(ctx, kutil.Key("garden", "openvpn-diffie-hellman-key"), gomock.AssignableToTypeOf(&corev1.Secret{})).DoAndReturn(
			func(ctx context.Context, _ client.ObjectKey, obj client.Object) error {
				(&corev1.Secret{
					ObjectMeta: metav1.ObjectMeta{
						Generation: 1,
					},
					Data: map[string][]byte{
						"incorrect": []byte(expectedKey),
					},
				}).DeepCopyInto(obj.(*corev1.Secret))
				return nil
			},
		)

		Expect(testOperation.GetOrDefaultDiffieHellmannKey(ctx)).ToNot(HaveOccurred())
		Expect(testOperation.imports.OpenVPNDiffieHellmanKey).To(Equal(&expectedDefaultDiffieHellmanKey))
	})

	It("should reuse existing Diffie Hellman Key", func() {
		expectedKey := "my diffy"
		mockRuntimeClient.EXPECT().Get(ctx, kutil.Key("garden", "openvpn-diffie-hellman-key"), gomock.AssignableToTypeOf(&corev1.Secret{})).DoAndReturn(
			func(ctx context.Context, _ client.ObjectKey, obj client.Object) error {
				(&corev1.Secret{
					ObjectMeta: metav1.ObjectMeta{
						Generation: 1,
					},
					Data: map[string][]byte{
						"dh2048.pem": []byte(expectedKey),
					},
				}).DeepCopyInto(obj.(*corev1.Secret))
				return nil
			},
		)

		Expect(testOperation.GetOrDefaultDiffieHellmannKey(ctx)).ToNot(HaveOccurred())
		Expect(testOperation.imports.OpenVPNDiffieHellmanKey).To(Equal(&expectedKey))
	})
})

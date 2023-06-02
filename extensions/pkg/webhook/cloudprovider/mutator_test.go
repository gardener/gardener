// Copyright 2021 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

package cloudprovider_test

import (
	"context"
	"testing"

	"github.com/golang/mock/gomock"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/log"

	"github.com/gardener/gardener/extensions/pkg/webhook"
	"github.com/gardener/gardener/extensions/pkg/webhook/cloudprovider"
	extensionsmockcloudprovider "github.com/gardener/gardener/extensions/pkg/webhook/cloudprovider/mock"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
)

func TestCloudProvider(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Extensions Webhook CloudProvider Suite")
}

var _ = Describe("Mutator", func() {
	var (
		ctrl   *gomock.Controller
		logger = log.Log.WithName("test")
	)

	BeforeEach(func() {
		ctrl = gomock.NewController(GinkgoT())
	})

	AfterEach(func() {
		ctrl.Finish()
	})

	Describe("#Mutate", func() {
		var (
			ensurer        *extensionsmockcloudprovider.MockEnsurer
			newSecret, old *corev1.Secret
			mutator        webhook.Mutator
		)

		BeforeEach(func() {
			ensurer = extensionsmockcloudprovider.NewMockEnsurer(ctrl)
			mutator = cloudprovider.NewMutator(logger, ensurer)
			newSecret = nil
			old = nil
		})

		It("Should ignore secrets other than cloudprovider", func() {
			newSecret = &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: "test"}}
			err := mutator.Mutate(context.TODO(), newSecret, old)
			Expect(err).NotTo(HaveOccurred())
		})

		It("Should mutate cloudprovider secret", func() {
			newSecret = &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: v1beta1constants.SecretNameCloudProvider}}

			ensurer.EXPECT().EnsureCloudProviderSecret(context.TODO(), gomock.Any(), newSecret, old)
			err := mutator.Mutate(context.TODO(), newSecret, old)
			Expect(err).NotTo(HaveOccurred())
		})
	})
})

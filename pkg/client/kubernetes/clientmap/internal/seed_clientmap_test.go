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

package internal_test

import (
	"context"
	"fmt"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	"github.com/gardener/gardener/pkg/client/kubernetes/clientmap"
	"github.com/gardener/gardener/pkg/client/kubernetes/clientmap/internal"
	"github.com/gardener/gardener/pkg/client/kubernetes/clientmap/keys"
	fakeclientset "github.com/gardener/gardener/pkg/client/kubernetes/fake"
	. "github.com/gardener/gardener/pkg/client/kubernetes/test"
	"github.com/gardener/gardener/pkg/logger"

	"github.com/golang/mock/gomock"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	eventsv1 "k8s.io/api/events/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	baseconfig "k8s.io/component-base/config"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var _ = Describe("SeedClientMap", func() {
	var (
		ctx  context.Context
		ctrl *gomock.Controller

		cm                     clientmap.ClientMap
		key                    clientmap.ClientSetKey
		factory                *internal.SeedClientSetFactory
		clientConnectionConfig baseconfig.ClientConnectionConfiguration

		seed *gardencorev1beta1.Seed
	)

	BeforeEach(func() {
		ctx = context.TODO()
		ctrl = gomock.NewController(GinkgoT())

		seed = &gardencorev1beta1.Seed{
			ObjectMeta: metav1.ObjectMeta{
				Name: "potato-seed",
			},
			Spec: gardencorev1beta1.SeedSpec{
				SecretRef: &corev1.SecretReference{
					Namespace: "backyard",
					Name:      "potato-secret",
				},
			},
		}

		key = keys.ForSeed(seed)

		clientConnectionConfig = baseconfig.ClientConnectionConfiguration{
			Kubeconfig:         "/var/run/secrets/kubeconfig",
			AcceptContentTypes: "application/vnd.kubernetes.protobuf;application/json",
			ContentType:        "application/vnd.kubernetes.protobuf",
			QPS:                42,
			Burst:              43,
		}
		factory = &internal.SeedClientSetFactory{
			ClientConnectionConfig: clientConnectionConfig,
		}
		cm = internal.NewSeedClientMap(factory, logger.NewNopLogger())
	})

	AfterEach(func() {
		ctrl.Finish()
	})

	Context("#GetClient", func() {
		It("should fail if NewClientFromSecret fails", func() {
			fakeErr := fmt.Errorf("fake")
			internal.NewClientFromFile = func(masterURL, kubeconfigPath string, fns ...kubernetes.ConfigFunc) (kubernetes.Interface, error) {
				return nil, fakeErr
			}

			cs, err := cm.GetClient(ctx, key)
			Expect(cs).To(BeNil())
			Expect(err).To(MatchError(fmt.Sprintf("error creating new ClientSet for key %q: fake", key.Key())))
		})

		It("should correctly construct a new ClientSet", func() {
			fakeCS := fakeclientset.NewClientSet()
			internal.NewClientFromFile = func(masterURL, kubeconfigPath string, fns ...kubernetes.ConfigFunc) (kubernetes.Interface, error) {
				Expect(masterURL).To(BeEmpty())
				Expect(kubeconfigPath).To(Equal(clientConnectionConfig.Kubeconfig))
				Expect(fns).To(ConsistOfConfigFuncs(
					kubernetes.WithClientConnectionOptions(clientConnectionConfig),
					kubernetes.WithClientOptions(client.Options{Scheme: kubernetes.SeedScheme}),
					kubernetes.WithUncached(
						&corev1.Event{},
						&eventsv1.Event{},
					),
				))
				return fakeCS, nil
			}

			cs, err := cm.GetClient(ctx, key)
			Expect(err).NotTo(HaveOccurred())
			Expect(cs).To(BeIdenticalTo(fakeCS))
		})
	})

	Context("#CalculateClientSetHash", func() {
		It("should always return the same identifier", func() {
			key = fakeKey{}
			hash, err := factory.CalculateClientSetHash(ctx, key)
			Expect(hash).To(BeEmpty())
			Expect(err).NotTo(HaveOccurred())
		})
	})
})

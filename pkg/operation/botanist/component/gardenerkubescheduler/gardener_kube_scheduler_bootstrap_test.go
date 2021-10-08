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

package gardenerkubescheduler_test

import (
	"context"

	gardenletfeatures "github.com/gardener/gardener/pkg/gardenlet/features"
	"github.com/gardener/gardener/pkg/operation/botanist/component"
	. "github.com/gardener/gardener/pkg/operation/botanist/component/gardenerkubescheduler"
	"github.com/gardener/gardener/pkg/operation/botanist/component/seedadmissioncontroller"
	"github.com/gardener/gardener/pkg/utils/imagevector"
	. "github.com/gardener/gardener/pkg/utils/test/matchers"

	"github.com/Masterminds/semver"
	resourcesv1alpha1 "github.com/gardener/gardener/pkg/apis/resources/v1alpha1"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	admissionregistrationv1 "k8s.io/api/admissionregistration/v1"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/serializer"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/utils/pointer"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

var _ = Describe("Bootstrap", func() {
	var (
		ctx   context.Context
		c     client.Client
		sched component.DeployWaiter
		codec serializer.CodecFactory
		image *imagevector.Image
	)

	BeforeEach(func() {
		ctx = context.TODO()
		image = &imagevector.Image{Name: "foo", Repository: "example.com", Tag: pointer.String("v1.2.3")}

		s := runtime.NewScheme()
		Expect(appsv1.AddToScheme(s)).NotTo(HaveOccurred())
		Expect(corev1.AddToScheme(s)).NotTo(HaveOccurred())
		Expect(rbacv1.AddToScheme(s)).NotTo(HaveOccurred())
		Expect(admissionregistrationv1.AddToScheme(s)).NotTo(HaveOccurred())
		Expect(resourcesv1alpha1.AddToScheme(s)).NotTo(HaveOccurred())

		codec = serializer.NewCodecFactory(s, serializer.EnableStrict)
		c = fake.NewClientBuilder().WithScheme(s).Build()
	})

	Context("fails", func() {
		var err error

		It("when namespace is empty", func() {
			sched, err = Bootstrap(c, "", image, &semver.Version{})
		})

		It("when seed version is nil", func() {
			sched, err = Bootstrap(c, "foo", image, nil)
		})

		AfterEach(func() {
			Expect(err).To(HaveOccurred())
			Expect(sched).To(BeNil())
		})
	})

	Context("succeeds", func() {
		const deployNS = "gardener-kube-scheduler"

		var (
			managedResourceSecret *corev1.Secret
			version               *semver.Version
			err                   error
		)

		BeforeEach(func() {
			managedResourceSecret = &corev1.Secret{}
			version = &semver.Version{}
			err = nil

			gardenletfeatures.RegisterFeatureGates()
			Expect(gardenletfeatures.FeatureGate.SetFromMap(map[string]bool{"SeedKubeScheduler": true})).To(Succeed())
		})

		Describe("it does nothing", func() {
			AfterEach(func() {
				Expect(err).NotTo(HaveOccurred())
				Expect(sched.Deploy(ctx)).To(Succeed(), "deploy succeeds")

				actual := &corev1.Namespace{}
				Expect(c.Get(ctx, types.NamespacedName{Name: deployNS}, actual)).To(BeNotFoundError())
			})

			It("for version with unsupported version", func() {
				sched, err = Bootstrap(c, "foo", image, &semver.Version{})
			})

			It("for supported version 0.18.1, but feature gate disabled", func() {
				Expect(gardenletfeatures.FeatureGate.SetFromMap(map[string]bool{"SeedKubeScheduler": false})).To(Succeed())
				version, err = semver.NewVersion("1.18.1")
				Expect(err).ToNot(HaveOccurred())

				sched, err = Bootstrap(c, "foo", image, version)
			})
		})

		Context("with supported version", func() {
			var (
				config            string
				configMapDataHash string
			)

			JustBeforeEach(func() {
				var (
					cmKey  = "configmap__gardener-kube-scheduler__gardener-kube-scheduler-" + configMapDataHash + ".yaml"
					mwcKey = "mutatingwebhookconfiguration____kube-scheduler.scheduling.gardener.cloud.yaml"
				)

				var (
					actualCM    = &corev1.ConfigMap{}
					actualMWC   = &admissionregistrationv1.MutatingWebhookConfiguration{}
					expectedMWC = &admissionregistrationv1.MutatingWebhookConfiguration{
						Webhooks: []admissionregistrationv1.MutatingWebhook{{
							ClientConfig: admissionregistrationv1.WebhookClientConfig{
								Service: &admissionregistrationv1.ServiceReference{
									Namespace: "foo",
									Name:      "gardener-seed-admission-controller",
									Path:      pointer.String("/webhooks/default-pod-scheduler-name/gardener-shoot-controlplane-scheduler"),
								},
								CABundle: []byte(seedadmissioncontroller.TLSCACert),
							},
						}},
					}
				)

				sched, err = Bootstrap(c, "foo", image, version)
				Expect(err).ToNot(HaveOccurred())

				Expect(sched.Deploy(ctx)).To(Succeed(), "deploy is successful")

				Expect(c.Get(ctx, types.NamespacedName{
					Name:      "managedresource-gardener-kube-scheduler",
					Namespace: deployNS,
				}, managedResourceSecret)).NotTo(HaveOccurred(), "can get managed resource's secret")

				Expect(managedResourceSecret.Data).To(HaveKey(cmKey))
				_, _, err := codec.UniversalDecoder().Decode(managedResourceSecret.Data[cmKey], nil, actualCM)
				Expect(err).NotTo(HaveOccurred())

				Expect(actualCM.Data).To(HaveKey("config.yaml"))
				config = actualCM.Data["config.yaml"]

				Expect(managedResourceSecret.Data).To(HaveKey(mwcKey))
				_, _, err = codec.UniversalDecoder().Decode(managedResourceSecret.Data[mwcKey], nil, actualMWC)
				Expect(err).NotTo(HaveOccurred())

				Expect(actualMWC).To(DeepDerivativeEqual(expectedMWC))
			})

			Context("v1.18", func() {
				BeforeEach(func() {
					version, err = semver.NewVersion("1.18.3")
					Expect(err).ToNot(HaveOccurred())
					configMapDataHash = "7f566643"
				})

				It("has correct config", func() {
					Expect(config).To(Equal(expectedV18Config))
				})
			})

			Context("v1.19", func() {
				BeforeEach(func() {
					version, err = semver.NewVersion("1.19.3")
					Expect(err).ToNot(HaveOccurred())
					configMapDataHash = "add26c73"
				})

				It("has correct config", func() {
					Expect(config).To(Equal(expectedV19Config))
				})
			})

			Context("v1.20", func() {
				BeforeEach(func() {
					version, err = semver.NewVersion("1.20.3")
					Expect(err).ToNot(HaveOccurred())
					configMapDataHash = "add26c73"
				})

				It("has correct config", func() {
					Expect(config).To(Equal(expectedV20Config))
				})
			})
			Context("v1.21", func() {
				BeforeEach(func() {
					version, err = semver.NewVersion("1.21.3")
					Expect(err).ToNot(HaveOccurred())
				})

				It("has correct config", func() {
					Expect(config).To(Equal(expectedV21Config))
				})
			})
			Context("v1.22", func() {
				BeforeEach(func() {
					version, err = semver.NewVersion("1.22.1")
					Expect(err).ToNot(HaveOccurred())
				})

				It("has correct config", func() {
					Expect(config).To(Equal(expectedV22Config))
				})
			})
		})
	})
})

const (
	expectedV18Config = `apiVersion: kubescheduler.config.k8s.io/v1alpha2
bindTimeoutSeconds: null
clientConnection:
  acceptContentTypes: ""
  burst: 0
  contentType: ""
  kubeconfig: ""
  qps: 0
extenders: null
kind: KubeSchedulerConfiguration
leaderElection:
  leaderElect: true
  leaseDuration: 15s
  renewDeadline: 10s
  resourceLock: leases
  resourceName: gardener-kube-scheduler
  resourceNamespace: gardener-kube-scheduler
  retryPeriod: 2s
podInitialBackoffSeconds: null
podMaxBackoffSeconds: null
profiles:
- plugins:
    score:
      disabled:
      - name: NodeResourcesLeastAllocated
      - name: NodeResourcesBalancedAllocation
      enabled:
      - name: NodeResourcesMostAllocated
  schedulerName: gardener-shoot-controlplane-scheduler
`
	expectedV19Config = `apiVersion: kubescheduler.config.k8s.io/v1beta1
clientConnection:
  acceptContentTypes: ""
  burst: 0
  contentType: ""
  kubeconfig: ""
  qps: 0
kind: KubeSchedulerConfiguration
leaderElection:
  leaderElect: true
  leaseDuration: 15s
  renewDeadline: 10s
  resourceLock: leases
  resourceName: gardener-kube-scheduler
  resourceNamespace: gardener-kube-scheduler
  retryPeriod: 2s
profiles:
- plugins:
    score:
      disabled:
      - name: NodeResourcesLeastAllocated
      - name: NodeResourcesBalancedAllocation
      enabled:
      - name: NodeResourcesMostAllocated
  schedulerName: gardener-shoot-controlplane-scheduler
`
	expectedV20Config = expectedV19Config

	expectedV21Config = expectedV19Config

	expectedV22Config = expectedV19Config
)

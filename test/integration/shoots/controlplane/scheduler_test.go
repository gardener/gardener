// Copyright (c) 2019 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

/**
	Overview
		- Tests the control plane of a Shoot cluster

	Test: kube-scheduler
	Expected Output
		- kube-scheduler is configured correctly

 **/

package controlplane

import (
	"context"
	"flag"
	"time"

	gardencorev1alpha1 "github.com/gardener/gardener/pkg/apis/core/v1alpha1"
	"github.com/gardener/gardener/pkg/logger"
	. "github.com/gardener/gardener/test/integration/framework"
	. "github.com/gardener/gardener/test/integration/shoots"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gstruct"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/runtime/serializer"
	schedulerv1alpha1conf "k8s.io/kube-scheduler/config/v1alpha1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var (
	kubeconfig     = flag.String("kubecfg", "", "the path to the kubeconfig  of the garden cluster that will be used for integration tests")
	shootName      = flag.String("shoot-name", "", "the name of the shoot we want to test")
	shootNamespace = flag.String("shoot-namespace", "", "the namespace name that the shoot resides in")
	logLevel       = flag.String("verbose", "", "verbosity level, when set, logging level will be DEBUG")
)

func validateFlags() {
	if !StringSet(*shootName) {
		Fail("You should specify a shootName to test against")
	}

	if !StringSet(*kubeconfig) {
		Fail("you need to specify the correct path for the kubeconfig")
	}

	if !FileExists(*kubeconfig) {
		Fail("kubeconfig path does not exist")
	}
}

var _ = Describe("kube-scheduler testing", func() {
	const (
		seedTimeout           = time.Minute
		initializationTimeout = 10 * time.Second
		dumpStateTimeout      = 5 * time.Minute
	)

	var (
		shootGardenerTest   *ShootGardenerTest
		shootTestOperations *GardenerTestOperation
		shootSeedNamespace  string
	)

	CBeforeSuite(func(ctx context.Context) {
		flag.Parse()
		// validate flags
		validateFlags()

		shootAppTestLogger := logger.AddWriter(logger.NewLogger(*logLevel), GinkgoWriter)

		var err error
		shootGardenerTest, err = NewShootGardenerTest(*kubeconfig, nil, shootAppTestLogger)
		Expect(err).NotTo(HaveOccurred())

		shoot := &gardencorev1alpha1.Shoot{ObjectMeta: metav1.ObjectMeta{Namespace: *shootNamespace, Name: *shootName}}
		shootTestOperations, err = NewGardenTestOperationWithShoot(
			ctx,
			shootGardenerTest.GardenClient,
			shootAppTestLogger,
			shoot)
		Expect(err).NotTo(HaveOccurred())

		shootSeedNamespace = shootTestOperations.ShootSeedNamespace()

	}, initializationTimeout)

	CAfterEach(func(ctx context.Context) {
		shootTestOperations.AfterEach(ctx)
	}, dumpStateTimeout)

	CIt("correct configmap is created", func(ctx context.Context) {
		cm := &corev1.ConfigMap{}
		err := shootTestOperations.SeedClient.Client().Get(
			ctx,
			client.ObjectKey{Namespace: shootSeedNamespace, Name: "kube-scheduler-config"},
			cm)

		Expect(err).ToNot(HaveOccurred(), "scheduler ConfigMap should exist")

		configData, ok := cm.Data["config.yaml"]
		Expect(ok).To(BeTrue(), "config.yaml key should exist in the configmap")

		s := runtime.NewScheme()
		Expect(schedulerv1alpha1conf.AddToScheme(s)).ToNot(HaveOccurred(), "no error should occur when adding to scheme")

		// Add types for K8S clusters <= 1.12
		s.AddKnownTypes(
			schema.GroupVersion{Group: "componentconfig", Version: "v1alpha1"},
			&schedulerv1alpha1conf.KubeSchedulerConfiguration{})

		configObj := &schedulerv1alpha1conf.KubeSchedulerConfiguration{}

		err = runtime.DecodeInto(serializer.NewCodecFactory(s).UniversalDecoder(), []byte(configData), configObj)
		Expect(err).NotTo(HaveOccurred(), "when decoding object")

		Expect(configObj.ClientConnection.Kubeconfig).To(Equal("/var/lib/kube-scheduler/kubeconfig"), "kubeconfig is set")
		Expect(configObj.LeaderElection.LeaderElect).To(PointTo(BeTrue()), "leader election is enabled")

		Expect(configObj.AlgorithmSource.Provider).To(BeNil(), "provider is not set")
		Expect(configObj.AlgorithmSource.Policy).To(BeNil(), "policy is not set")
	}, seedTimeout)
})

// Copyright (c) 2022 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

package rotation

import (
	"context"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"

	"github.com/gardener/gardener/pkg/client/kubernetes"
	"github.com/gardener/gardener/test/e2e/shoot/internal"
	"github.com/gardener/gardener/test/framework"
)

type clients struct {
	staticToken, adminKubeconfig kubernetes.Interface
}

// ShootAccessVerifier uses the static token and admin kubeconfig to access the Shoot.
type ShootAccessVerifier struct {
	*framework.ShootCreationFramework

	clientsBefore, clientsPrepared clients
}

// Before is called before the rotation is started.
func (v *ShootAccessVerifier) Before(ctx context.Context) {
	By("Using old static token kubeconfig with old CA to access shoot")
	Eventually(func(g Gomega) {
		shootClient, err := internal.CreateShootClientFromStaticTokenKubeconfig(ctx, v.GardenClient, v.Shoot)
		g.Expect(err).NotTo(HaveOccurred())

		g.Expect(shootClient.Client().List(ctx, &corev1.NamespaceList{})).To(Succeed())

		v.clientsBefore.staticToken = shootClient
	}).Should(Succeed())

	By("Using admin kubeconfig with old CA to access shoot")
	Eventually(func(g Gomega) {
		shootClient, err := internal.CreateShootClientFromAdminKubeconfig(ctx, v.GardenClient, v.Shoot)
		g.Expect(err).NotTo(HaveOccurred())

		g.Expect(shootClient.Client().List(ctx, &corev1.NamespaceList{})).To(Succeed())

		v.clientsBefore.adminKubeconfig = shootClient
	}).Should(Succeed())
}

// ExpectPreparingStatus is called while waiting for the Preparing status.
func (v *ShootAccessVerifier) ExpectPreparingStatus(g Gomega) {}

// AfterPrepared is called when the Shoot is in Prepared status.
func (v *ShootAccessVerifier) AfterPrepared(ctx context.Context) {
	By("Using old static token kubeconfig with old CA to access shoot")
	Consistently(func(g Gomega) {
		g.Expect(v.clientsBefore.staticToken.Client().List(ctx, &corev1.NamespaceList{})).NotTo(Succeed())
	}).Should(Succeed())

	By("Using admin kubeconfig with old CA to access shoot")
	Eventually(func(g Gomega) {
		g.Expect(v.clientsBefore.adminKubeconfig.Client().List(ctx, &corev1.NamespaceList{})).To(Succeed())
	}).Should(Succeed())

	By("Using rotated static token kubeconfig with CA bundle to access shoot")
	Eventually(func(g Gomega) {
		shootClient, err := internal.CreateShootClientFromStaticTokenKubeconfig(ctx, v.GardenClient, v.Shoot)
		g.Expect(err).NotTo(HaveOccurred())

		g.Expect(shootClient.Client().List(ctx, &corev1.NamespaceList{})).To(Succeed())

		v.clientsPrepared.staticToken = shootClient
	}).Should(Succeed())

	By("Using admin kubeconfig with CA bundle to access shoot")
	Eventually(func(g Gomega) {
		shootClient, err := internal.CreateShootClientFromAdminKubeconfig(ctx, v.GardenClient, v.Shoot)
		g.Expect(err).NotTo(HaveOccurred())

		g.Expect(shootClient.Client().List(ctx, &corev1.NamespaceList{})).To(Succeed())

		v.clientsPrepared.adminKubeconfig = shootClient
	}).Should(Succeed())
}

// ExpectCompletingStatus is called while waiting for the Completing status.
func (v *ShootAccessVerifier) ExpectCompletingStatus(g Gomega) {}

// AfterCompleted is called when the Shoot is in Completed status.
func (v *ShootAccessVerifier) AfterCompleted(ctx context.Context) {
	By("Using old static token kubeconfig with old CA to access shoot")
	Consistently(func(g Gomega) {
		g.Expect(v.clientsBefore.staticToken.Client().List(ctx, &corev1.NamespaceList{})).NotTo(Succeed())
	}).Should(Succeed())

	By("Using admin kubeconfig with old CA to access shoot")
	Consistently(func(g Gomega) {
		g.Expect(v.clientsBefore.adminKubeconfig.Client().List(ctx, &corev1.NamespaceList{})).NotTo(Succeed())
	}).Should(Succeed())

	By("Using rotated static token kubeconfig with CA bundle to access shoot")
	Eventually(func(g Gomega) {
		g.Expect(v.clientsPrepared.staticToken.Client().List(ctx, &corev1.NamespaceList{})).To(Succeed())
	}).Should(Succeed())

	By("Using admin kubeconfig with CA bundle to access shoot")
	Eventually(func(g Gomega) {
		g.Expect(v.clientsPrepared.adminKubeconfig.Client().List(ctx, &corev1.NamespaceList{})).To(Succeed())
	}).Should(Succeed())

	By("Using rotated static token kubeconfig with new CA to access shoot")
	Eventually(func(g Gomega) {
		shootClient, err := internal.CreateShootClientFromStaticTokenKubeconfig(ctx, v.GardenClient, v.Shoot)
		g.Expect(err).NotTo(HaveOccurred())

		g.Expect(shootClient.Client().List(ctx, &corev1.NamespaceList{})).To(Succeed())
	}).Should(Succeed())

	By("Using admin kubeconfig with new CA to access shoot")
	Eventually(func(g Gomega) {
		shootClient, err := internal.CreateShootClientFromAdminKubeconfig(ctx, v.GardenClient, v.Shoot)
		g.Expect(err).NotTo(HaveOccurred())

		g.Expect(shootClient.Client().List(ctx, &corev1.NamespaceList{})).To(Succeed())
	}).Should(Succeed())
}

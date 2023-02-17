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
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	v1beta1helper "github.com/gardener/gardener/pkg/apis/core/v1beta1/helper"
	"github.com/gardener/gardener/test/framework"
	"github.com/gardener/gardener/test/utils/rotation"
)

// ServiceAccountKeyVerifier verifies the service account key rotation.
type ServiceAccountKeyVerifier struct {
	*framework.ShootCreationFramework

	secretsBefore   rotation.SecretConfigNamesToSecrets
	secretsPrepared rotation.SecretConfigNamesToSecrets
}

const (
	serviceAccountKey       = "service-account-key"
	serviceAccountKeyBundle = "service-account-key-bundle"
)

// Before is called before the rotation is started.
func (v *ServiceAccountKeyVerifier) Before(ctx context.Context) {
	seedClient := v.ShootFramework.SeedClient.Client()

	By("Verify old service account key secret")
	Eventually(func(g Gomega) {
		secretList := &corev1.SecretList{}
		g.Expect(seedClient.List(ctx, secretList, client.InNamespace(v.Shoot.Status.TechnicalID), managedByGardenletSecretsManager)).To(Succeed())

		grouped := rotation.GroupByName(secretList.Items)
		g.Expect(grouped[serviceAccountKey]).To(HaveLen(1), "service account key secret should get created, but not rotated yet")
		g.Expect(grouped[serviceAccountKeyBundle]).To(HaveLen(1), "service account key bundle secret should get created, but not rotated yet")
		v.secretsBefore = grouped
	}).Should(Succeed())
}

// ExpectPreparingStatus is called while waiting for the Preparing status.
func (v *ServiceAccountKeyVerifier) ExpectPreparingStatus(g Gomega) {
	g.Expect(v1beta1helper.GetShootServiceAccountKeyRotationPhase(v.Shoot.Status.Credentials)).To(Equal(gardencorev1beta1.RotationPreparing))
	g.Expect(time.Now().UTC().Sub(v.Shoot.Status.Credentials.Rotation.ServiceAccountKey.LastInitiationTime.Time.UTC())).To(BeNumerically("<=", time.Minute))
	g.Expect(v.Shoot.Status.Credentials.Rotation.ServiceAccountKey.LastInitiationFinishedTime).To(BeNil())
	g.Expect(v.Shoot.Status.Credentials.Rotation.ServiceAccountKey.LastCompletionTriggeredTime).To(BeNil())
}

// AfterPrepared is called when the Shoot is in Prepared status.
func (v *ServiceAccountKeyVerifier) AfterPrepared(ctx context.Context) {
	seedClient := v.ShootFramework.SeedClient.Client()

	Expect(v.Shoot.Status.Credentials.Rotation.ServiceAccountKey.Phase).To(Equal(gardencorev1beta1.RotationPrepared), "rotation phase should be 'Prepared'")
	Expect(v.Shoot.Status.Credentials.Rotation.ServiceAccountKey.LastInitiationFinishedTime).NotTo(BeNil())
	Expect(v.Shoot.Status.Credentials.Rotation.ServiceAccountKey.LastInitiationFinishedTime.After(v.Shoot.Status.Credentials.Rotation.ServiceAccountKey.LastInitiationTime.Time)).To(BeTrue())

	By("Verify service account key bundle secret")
	Eventually(func(g Gomega) {
		secretList := &corev1.SecretList{}
		g.Expect(seedClient.List(ctx, secretList, client.InNamespace(v.Shoot.Status.TechnicalID), managedByGardenletSecretsManager)).To(Succeed())

		grouped := rotation.GroupByName(secretList.Items)
		g.Expect(grouped[serviceAccountKey]).To(HaveLen(2), "service account key secret should get rotated, but old service account key is kept")
		g.Expect(grouped[serviceAccountKeyBundle]).To(HaveLen(1))

		g.Expect(grouped[serviceAccountKey]).To(ContainElement(v.secretsBefore[serviceAccountKey][0]), "old service account key secret should be kept")
		g.Expect(grouped[serviceAccountKeyBundle]).To(Not(ContainElement(v.secretsBefore[serviceAccountKeyBundle][0])), "service account key bundle should have changed")
		v.secretsPrepared = grouped
	}).Should(Succeed())
}

// ExpectCompletingStatus is called while waiting for the Completing status.
func (v *ServiceAccountKeyVerifier) ExpectCompletingStatus(g Gomega) {
	g.Expect(v1beta1helper.GetShootServiceAccountKeyRotationPhase(v.Shoot.Status.Credentials)).To(Equal(gardencorev1beta1.RotationCompleting))
	Expect(v.Shoot.Status.Credentials.Rotation.ServiceAccountKey.LastCompletionTriggeredTime).NotTo(BeNil())
	Expect(v.Shoot.Status.Credentials.Rotation.ServiceAccountKey.LastCompletionTriggeredTime.Time.Equal(v.Shoot.Status.Credentials.Rotation.ServiceAccountKey.LastInitiationFinishedTime.Time) ||
		v.Shoot.Status.Credentials.Rotation.ServiceAccountKey.LastCompletionTriggeredTime.After(v.Shoot.Status.Credentials.Rotation.ServiceAccountKey.LastInitiationFinishedTime.Time)).To(BeTrue())
}

// AfterCompleted is called when the Shoot is in Completed status.
func (v *ServiceAccountKeyVerifier) AfterCompleted(ctx context.Context) {
	seedClient := v.ShootFramework.SeedClient.Client()

	serviceAccountKeyRotation := v.Shoot.Status.Credentials.Rotation.ServiceAccountKey
	Expect(v1beta1helper.GetShootServiceAccountKeyRotationPhase(v.Shoot.Status.Credentials)).To(Equal(gardencorev1beta1.RotationCompleted))
	Expect(serviceAccountKeyRotation.LastCompletionTime.Time.UTC().After(serviceAccountKeyRotation.LastInitiationTime.Time.UTC())).To(BeTrue())
	Expect(serviceAccountKeyRotation.LastInitiationFinishedTime).To(BeNil())
	Expect(serviceAccountKeyRotation.LastCompletionTriggeredTime).To(BeNil())

	By("Verify new service account key secret")
	Eventually(func(g Gomega) {
		secretList := &corev1.SecretList{}
		g.Expect(seedClient.List(ctx, secretList, client.InNamespace(v.Shoot.Status.TechnicalID), managedByGardenletSecretsManager)).To(Succeed())

		grouped := rotation.GroupByName(secretList.Items)
		g.Expect(grouped[serviceAccountKey]).To(HaveLen(1), "old service account key secret should get cleaned up")
		g.Expect(grouped[serviceAccountKeyBundle]).To(HaveLen(1))

		g.Expect(grouped[serviceAccountKey]).To(ContainElement(v.secretsPrepared[serviceAccountKey][1]), "new service account key secret should be kept")
		g.Expect(grouped[serviceAccountKeyBundle]).To(Not(ContainElement(v.secretsPrepared[serviceAccountKeyBundle][0])), "service account key bundle should have changed")
	}).Should(Succeed())
}

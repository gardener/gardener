// Copyright 2023 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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
	"bytes"
	"context"
	"crypto/tls"
	"encoding/base64"
	"net/http"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	operatorv1alpha1 "github.com/gardener/gardener/pkg/apis/operator/v1alpha1"
)

// ObservabilityVerifier verifies the observability credentials rotation.
type ObservabilityVerifier struct {
	RuntimeClient client.Client
	Garden        *operatorv1alpha1.Garden

	observabilityEndpoint string
	oldKeypairData        map[string][]byte
}

// Before is called before the rotation is started.
func (v *ObservabilityVerifier) Before(ctx context.Context) {
	By("Verify old observability secret")
	Eventually(func(g Gomega) {
		secretList := &corev1.SecretList{}
		g.Expect(v.RuntimeClient.List(ctx, secretList, client.InNamespace(v1beta1constants.GardenNamespace), ObservabilitySecretManagedByGardenerOperatorSecretsManager)).To(Succeed())
		g.Expect(secretList.Items).To(HaveLen(1))
		g.Expect(secretList.Items[0].Data).To(And(
			HaveKeyWithValue("username", Not(BeEmpty())),
			HaveKeyWithValue("password", Not(BeEmpty())),
		))

		v.observabilityEndpoint = getObservabilityEndpoint(v.Garden)
		v.oldKeypairData = secretList.Items[0].Data
	}).Should(Succeed(), "old observability secret should be present")

	By("Use old credentials to access observability endpoint")
	Eventually(func(g Gomega) {
		response, err := v.accessEndpoint(ctx, v.observabilityEndpoint, v.oldKeypairData["username"], v.oldKeypairData["password"])
		g.Expect(err).NotTo(HaveOccurred())
		g.Expect(response.StatusCode).To(Equal(http.StatusOK))
	}).Should(Succeed())
}

// ExpectPreparingStatus is called while waiting for the Preparing status.
func (v *ObservabilityVerifier) ExpectPreparingStatus(g Gomega) {
	g.Expect(time.Now().UTC().Sub(v.Garden.Status.Credentials.Rotation.Observability.LastInitiationTime.Time.UTC())).To(BeNumerically("<=", time.Minute))
}

// AfterPrepared is called when the Garden is in Prepared status.
func (v *ObservabilityVerifier) AfterPrepared(ctx context.Context) {
	observabilityRotation := v.Garden.Status.Credentials.Rotation.Observability
	Expect(observabilityRotation.LastCompletionTime.Time.UTC().After(observabilityRotation.LastInitiationTime.Time.UTC())).To(BeTrue())

	By("Use old credentials to access observability endpoint")
	Consistently(func(g Gomega) {
		response, err := v.accessEndpoint(ctx, v.observabilityEndpoint, v.oldKeypairData["username"], v.oldKeypairData["password"])
		g.Expect(err).NotTo(HaveOccurred())
		g.Expect(response.StatusCode).To(Equal(http.StatusUnauthorized))
	}).Should(Succeed())

	By("Verify new observability secret")
	secretList := &corev1.SecretList{}
	Eventually(func(g Gomega) {
		g.Expect(v.RuntimeClient.List(ctx, secretList, client.InNamespace(v1beta1constants.GardenNamespace), ObservabilitySecretManagedByGardenerOperatorSecretsManager)).To(Succeed())
		g.Expect(secretList.Items).To(HaveLen(1))
		g.Expect(secretList.Items[0].Data).To(And(
			HaveKeyWithValue("username", Equal(v.oldKeypairData["username"])),
			HaveKeyWithValue("password", Not(Equal(v.oldKeypairData["password"]))),
		))
	}).Should(Succeed(), "observability secret should have been rotated")

	By("Use new credentials to access observability endpoint")
	Eventually(func(g Gomega) {
		response, err := v.accessEndpoint(ctx, v.observabilityEndpoint, secretList.Items[0].Data["username"], secretList.Items[0].Data["password"])
		g.Expect(err).NotTo(HaveOccurred())
		g.Expect(response.StatusCode).To(Equal(http.StatusOK))
	}).Should(Succeed())
}

// observability credentials rotation is completed after one reconciliation (there is no second phase)
// hence, there is nothing to check in the second part of the credentials rotation

// ExpectCompletingStatus is called while waiting for the Completing status.
func (v *ObservabilityVerifier) ExpectCompletingStatus(g Gomega) {}

// AfterCompleted is called when the Garden is in Completed status.
func (v *ObservabilityVerifier) AfterCompleted(ctx context.Context) {}

func (v *ObservabilityVerifier) accessEndpoint(ctx context.Context, url string, username, password []byte) (*http.Response, error) {
	httpClient := &http.Client{
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
		},
	}

	request, err := http.NewRequestWithContext(ctx, "GET", url+":8448", nil)
	if err != nil {
		return nil, err
	}
	request.Header.Add("Authorization", "Basic "+base64.StdEncoding.EncodeToString(bytes.Join([][]byte{username, password}, []byte(":"))))

	return httpClient.Do(request)
}

func getObservabilityEndpoint(garden *operatorv1alpha1.Garden) string {
	if garden != nil {
		return "https://plutono-garden." + garden.Spec.RuntimeCluster.Ingress.Domain
	}

	return ""
}

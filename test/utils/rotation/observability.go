// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

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

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
)

// ObservabilityVerifier verifies the observability credentials rotation.
type ObservabilityVerifier struct {
	GetObservabilitySecretFunc func(context.Context) (*corev1.Secret, error)
	GetObservabilityEndpoint   func(*corev1.Secret) string
	GetObservabilityRotation   func() *gardencorev1beta1.ObservabilityRotation

	observabilityEndpoint string
	oldKeypairData        map[string][]byte
}

// Before is called before the rotation is started.
func (v *ObservabilityVerifier) Before(ctx context.Context) {
	By("Verify old observability secret")
	Eventually(func(g Gomega) {
		secret, err := v.GetObservabilitySecretFunc(ctx)
		g.Expect(err).NotTo(HaveOccurred())
		g.Expect(secret.Data).To(And(
			HaveKeyWithValue("username", Not(BeEmpty())),
			HaveKeyWithValue("password", Not(BeEmpty())),
		))

		v.observabilityEndpoint = v.GetObservabilityEndpoint(secret)
		v.oldKeypairData = secret.Data
	}).Should(Succeed(), "old observability secret should be present")

	By("Use old credentials to access observability endpoint")
	Eventually(func(g Gomega) {
		response, err := accessEndpoint(ctx, v.observabilityEndpoint, v.oldKeypairData["username"], v.oldKeypairData["password"])
		g.Expect(err).NotTo(HaveOccurred())
		g.Expect(response.StatusCode).To(Equal(http.StatusOK))
	}).Should(Succeed())
}

// ExpectPreparingStatus is called while waiting for the Preparing status.
func (v *ObservabilityVerifier) ExpectPreparingStatus(g Gomega) {
	g.Expect(time.Now().UTC().Sub(v.GetObservabilityRotation().LastInitiationTime.Time.UTC())).To(BeNumerically("<=", time.Minute))
}

// ExpectPreparingWithoutWorkersRolloutStatus is called while waiting for the PreparingWithoutWorkersRollout status.
func (v *ObservabilityVerifier) ExpectPreparingWithoutWorkersRolloutStatus(_ Gomega) {}

// ExpectWaitingForWorkersRolloutStatus is called while waiting for the WaitingForWorkersRollout status.
func (v *ObservabilityVerifier) ExpectWaitingForWorkersRolloutStatus(_ Gomega) {}

// AfterPrepared is called when the Shoot is in Prepared status.
func (v *ObservabilityVerifier) AfterPrepared(ctx context.Context) {
	observabilityRotation := v.GetObservabilityRotation()
	Expect(observabilityRotation.LastCompletionTime.Time.UTC().After(observabilityRotation.LastInitiationTime.Time.UTC())).To(BeTrue())

	By("Use old credentials to access observability endpoint")
	Consistently(func(g Gomega) {
		response, err := accessEndpoint(ctx, v.observabilityEndpoint, v.oldKeypairData["username"], v.oldKeypairData["password"])
		g.Expect(err).NotTo(HaveOccurred())
		g.Expect(response.StatusCode).To(Equal(http.StatusUnauthorized))
	}).Should(Succeed())

	By("Verify new observability secret")
	var (
		secret *corev1.Secret
		err    error
	)

	Eventually(func(g Gomega) {
		secret, err = v.GetObservabilitySecretFunc(ctx)
		g.Expect(err).NotTo(HaveOccurred())
		g.Expect(secret.Data).To(And(
			HaveKeyWithValue("username", Equal(v.oldKeypairData["username"])),
			HaveKeyWithValue("password", Not(Equal(v.oldKeypairData["password"]))),
		))
	}).Should(Succeed(), "observability secret should have been rotated")

	By("Use new credentials to access observability endpoint")
	Eventually(func(g Gomega) {
		response, err := accessEndpoint(ctx, v.observabilityEndpoint, secret.Data["username"], secret.Data["password"])
		g.Expect(err).NotTo(HaveOccurred())
		g.Expect(response.StatusCode).To(Equal(http.StatusOK))
	}).Should(Succeed())
}

// observability credentials rotation is completed after one reconciliation (there is no second phase)
// hence, there is nothing to check in the second part of the credentials rotation

// ExpectCompletingStatus is called while waiting for the Completing status.
func (v *ObservabilityVerifier) ExpectCompletingStatus(_ Gomega) {}

// AfterCompleted is called when the Shoot is in Completed status.
func (v *ObservabilityVerifier) AfterCompleted(_ context.Context) {}

func accessEndpoint(ctx context.Context, url string, username, password []byte) (*http.Response, error) {
	httpClient := &http.Client{
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{InsecureSkipVerify: true}, // #nosec: G402 -- Test only.
		},
	}

	request, err := http.NewRequestWithContext(ctx, "GET", url+":443", nil)
	if err != nil {
		return nil, err
	}

	request.Header.Add("Authorization", "Basic "+base64.StdEncoding.EncodeToString(bytes.Join([][]byte{username, password}, []byte(":"))))
	return httpClient.Do(request)
}

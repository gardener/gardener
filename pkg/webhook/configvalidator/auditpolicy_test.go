// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package configvalidator_test

import (
	"net/http"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	. "github.com/gardener/gardener/pkg/webhook/configvalidator"
)

var _ = Describe("AdmitAudtPolicy", func() {
	Describe("valid audit policies", func() {
		It("should accept a valid minimal audit policy", func() {
			auditPolicy := `
apiVersion: audit.k8s.io/v1
kind: Policy
rules:
- level: Metadata
`
			statusCode, err := AdmitAudtPolicy(auditPolicy)
			Expect(err).NotTo(HaveOccurred())
			Expect(statusCode).To(Equal(int32(0)))
		})

		It("should accept a valid audit policy with multiple rules", func() {
			auditPolicy := `
apiVersion: audit.k8s.io/v1
kind: Policy
rules:
- level: None
  users: ["system:kube-proxy"]
  verbs: ["watch"]
  resources:
  - group: ""
    resources: ["endpoints", "services", "services/status"]
- level: None
  users: ["system:unsecured"]
  namespaces: ["kube-system"]
  verbs: ["get"]
  resources:
  - group: ""
    resources: ["configmaps"]
- level: Metadata
  resources:
  - group: ""
    resources: ["secrets", "configmaps"]
- level: RequestResponse
  resources:
  - group: ""
    resources: ["pods"]
`
			statusCode, err := AdmitAudtPolicy(auditPolicy)
			Expect(err).NotTo(HaveOccurred())
			Expect(statusCode).To(Equal(int32(0)))
		})

		It("should accept audit policy with omitStages", func() {
			auditPolicy := `
apiVersion: audit.k8s.io/v1
kind: Policy
omitStages:
  - RequestReceived
rules:
- level: Metadata
`
			statusCode, err := AdmitAudtPolicy(auditPolicy)
			Expect(err).NotTo(HaveOccurred())
			Expect(statusCode).To(Equal(int32(0)))
		})

		It("should accept audit policy with namespaces and objectRef", func() {
			auditPolicy := `
apiVersion: audit.k8s.io/v1
kind: Policy
rules:
- level: None
  namespaces: ["kube-system", "kube-public"]
- level: Metadata
  objectRef:
    resource: "pods"
    namespace: "default"
`
			statusCode, err := AdmitAudtPolicy(auditPolicy)
			Expect(err).NotTo(HaveOccurred())
			Expect(statusCode).To(Equal(int32(0)))
		})
	})

	Describe("invalid audit policies", func() {
		It("should reject invalid YAML", func() {
			auditPolicy := `
invalid: yaml: content
  - missing quotes
`
			statusCode, err := AdmitAudtPolicy(auditPolicy)
			Expect(err).To(HaveOccurred())
			Expect(statusCode).To(Equal(int32(http.StatusUnprocessableEntity)))
			Expect(err.Error()).To(ContainSubstring("failed to decode the provided audit policy"))
		})

		It("should reject empty content", func() {
			auditPolicy := ""
			statusCode, err := AdmitAudtPolicy(auditPolicy)
			Expect(err).To(HaveOccurred())
			Expect(statusCode).To(Equal(int32(http.StatusUnprocessableEntity)))
			Expect(err.Error()).To(ContainSubstring("failed to decode the provided audit policy"))
		})

		It("should reject audit policy without apiVersion", func() {
			auditPolicy := `
kind: Policy
rules:
- level: Metadata
`
			statusCode, err := AdmitAudtPolicy(auditPolicy)
			Expect(err).To(HaveOccurred())
			Expect(statusCode).To(Equal(int32(http.StatusUnprocessableEntity)))
		})

		It("should reject audit policy without kind", func() {
			auditPolicy := `
apiVersion: audit.k8s.io/v1
rules:
- level: Metadata
`
			statusCode, err := AdmitAudtPolicy(auditPolicy)
			Expect(err).To(HaveOccurred())
			Expect(statusCode).To(Equal(int32(http.StatusUnprocessableEntity)))
		})

		It("should reject audit policy with invalid level", func() {
			auditPolicy := `
apiVersion: audit.k8s.io/v1
kind: Policy
rules:
- level: InvalidLevel
`
			statusCode, err := AdmitAudtPolicy(auditPolicy)
			Expect(err).To(HaveOccurred())
			Expect(statusCode).To(Equal(int32(http.StatusUnprocessableEntity)))
			Expect(err.Error()).To(ContainSubstring("provided invalid audit policy"))
		})

		It("should reject audit policy with invalid omitStages", func() {
			auditPolicy := `
apiVersion: audit.k8s.io/v1
kind: Policy
omitStages:
  - InvalidStage
rules:
- level: Metadata
`
			statusCode, err := AdmitAudtPolicy(auditPolicy)
			Expect(err).To(HaveOccurred())
			Expect(statusCode).To(Equal(int32(http.StatusUnprocessableEntity)))
			Expect(err.Error()).To(ContainSubstring("provided invalid audit policy"))
		})
	})
})

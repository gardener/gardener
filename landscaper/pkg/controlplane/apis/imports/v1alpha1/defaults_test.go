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

package v1alpha1_test

import (
	"fmt"

	landscaperv1alpha1 "github.com/gardener/landscaper/apis/core/v1alpha1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/pointer"

	. "github.com/gardener/gardener/landscaper/pkg/controlplane/apis/imports/v1alpha1"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("Defaults", func() {
	Describe("#SetDefaults_Imports", func() {
		It("should enable the Seed restriction webhook if the Seed Authorizer is enabled", func() {
			var obj = &Imports{
				Rbac: &Rbac{
					SeedAuthorizer: &SeedAuthorizer{Enabled: pointer.Bool(true)},
				},
				GardenerAdmissionController: &GardenerAdmissionController{
					Enabled: true,
				},
			}
			SetDefaults_Imports(obj)

			Expect(obj).To(Equal(&Imports{
				Rbac: &Rbac{
					SeedAuthorizer: &SeedAuthorizer{Enabled: pointer.Bool(true)},
				},
				GardenerAPIServer: GardenerAPIServer{
					ComponentConfiguration: APIServerComponentConfiguration{
						CA: &CA{
							Validity: &metav1.Duration{
								Duration: 157680000000000000,
							},
						},
						TLS: &TLSServer{
							Validity: &metav1.Duration{
								Duration: 31536000000000000,
							},
						},
					},
				},
				GardenerAdmissionController: &GardenerAdmissionController{
					Enabled: true,
					SeedRestriction: &SeedRestriction{
						Enabled: true,
					},
					ComponentConfiguration: &AdmissionControllerComponentConfiguration{
						CA: &CA{
							Validity: &metav1.Duration{
								Duration: 157680000000000000,
							},
						},
						TLS: &TLSServer{
							Validity: &metav1.Duration{
								Duration: 31536000000000000,
							},
						},
					},
				},
				GardenerControllerManager: &GardenerControllerManager{
					ComponentConfiguration: &ControllerManagerComponentConfiguration{
						TLS: &TLSServer{
							Validity: &metav1.Duration{
								Duration: 31536000000000000,
							},
						},
					},
				},
			}))
		})

		It("should default the mutating webhook kubeconfig for token volume projection", func() {
			var obj = &Imports{
				GardenerAPIServer: GardenerAPIServer{
					ComponentConfiguration: APIServerComponentConfiguration{
						Admission: &APIServerAdmissionConfiguration{
							MutatingWebhook: &APIServerAdmissionWebhookCredentials{
								TokenProjection: &APIServerAdmissionWebhookCredentialsTokenProjection{
									Enabled: true,
								},
							},
						},
					},
				},
			}
			SetDefaults_Imports(obj)

			Expect(obj).To(Equal(&Imports{
				GardenerAPIServer: GardenerAPIServer{
					ComponentConfiguration: APIServerComponentConfiguration{
						CA: &CA{
							Validity: &metav1.Duration{
								Duration: 157680000000000000,
							},
						},
						TLS: &TLSServer{
							Validity: &metav1.Duration{
								Duration: 31536000000000000,
							},
						},
						Admission: &APIServerAdmissionConfiguration{
							MutatingWebhook: &APIServerAdmissionWebhookCredentials{
								Kubeconfig: &landscaperv1alpha1.Target{Spec: landscaperv1alpha1.TargetSpec{Configuration: landscaperv1alpha1.AnyJSON{
									RawMessage: []byte(getVolumeProjectionKubeconfig("mutating")),
								}}},
								TokenProjection: &APIServerAdmissionWebhookCredentialsTokenProjection{
									Enabled: true,
								},
							},
						},
					},
				},
				GardenerAdmissionController: &GardenerAdmissionController{},
				GardenerControllerManager: &GardenerControllerManager{
					ComponentConfiguration: &ControllerManagerComponentConfiguration{
						TLS: &TLSServer{
							Validity: &metav1.Duration{
								Duration: 31536000000000000,
							},
						},
					},
				},
			}))
		})

		It("should default the validating webhook kubeconfig for token volume projection", func() {
			var obj = &Imports{
				GardenerAPIServer: GardenerAPIServer{
					ComponentConfiguration: APIServerComponentConfiguration{
						Admission: &APIServerAdmissionConfiguration{
							ValidatingWebhook: &APIServerAdmissionWebhookCredentials{
								TokenProjection: &APIServerAdmissionWebhookCredentialsTokenProjection{
									Enabled: true,
								},
							},
						},
					},
				},
			}
			SetDefaults_Imports(obj)

			Expect(obj).To(Equal(&Imports{
				GardenerAPIServer: GardenerAPIServer{
					ComponentConfiguration: APIServerComponentConfiguration{
						CA: &CA{
							Validity: &metav1.Duration{
								Duration: 157680000000000000,
							},
						},
						TLS: &TLSServer{
							Validity: &metav1.Duration{
								Duration: 31536000000000000,
							},
						},
						Admission: &APIServerAdmissionConfiguration{
							ValidatingWebhook: &APIServerAdmissionWebhookCredentials{
								Kubeconfig: &landscaperv1alpha1.Target{Spec: landscaperv1alpha1.TargetSpec{Configuration: landscaperv1alpha1.AnyJSON{
									RawMessage: []byte(getVolumeProjectionKubeconfig("validating")),
								}}},
								TokenProjection: &APIServerAdmissionWebhookCredentialsTokenProjection{
									Enabled: true,
								},
							},
						},
					},
				},
				GardenerAdmissionController: &GardenerAdmissionController{},
				GardenerControllerManager: &GardenerControllerManager{
					ComponentConfiguration: &ControllerManagerComponentConfiguration{
						TLS: &TLSServer{
							Validity: &metav1.Duration{
								Duration: 31536000000000000,
							},
						},
					},
				},
			}))
		})

	})
})

func getVolumeProjectionKubeconfig(name string) string {
	return fmt.Sprintf(`
---
apiVersion: v1
kind: Config
users:
- name: '*'
user:
  tokenFile: /var/run/secrets/admission-tokens/%s-webhook-token`, name)
}

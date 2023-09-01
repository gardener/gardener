// Copyright 2023 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package forcedeletion_test

import (
	"context"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apiserver/pkg/admission"
	"k8s.io/utils/pointer"

	"github.com/gardener/gardener/pkg/apis/core"
	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	. "github.com/gardener/gardener/pkg/utils/test/matchers"
	. "github.com/gardener/gardener/plugin/pkg/shoot/forcedeletion"
)

const (
	name      = "foo"
	namespace = "garden"
)

var _ = Describe("ShootForceDeletion", func() {
	Describe("#Validate", func() {
		var (
			shoot            *core.Shoot
			admissionHandler *ForceDeletion
		)

		BeforeEach(func() {
			shoot = &core.Shoot{
				ObjectMeta: metav1.ObjectMeta{
					Name:      name,
					Namespace: namespace,
				},
				Spec: core.ShootSpec{
					Addons: &core.Addons{
						NginxIngress: &core.NginxIngress{
							Addon: core.Addon{
								Enabled: false,
							},
						},
					},
					Kubernetes: core.Kubernetes{
						VerticalPodAutoscaler: &core.VerticalPodAutoscaler{
							Enabled: true,
						},
					},
					Networking: &core.Networking{
						Type:  pointer.String("foo"),
						Nodes: pointer.String("10.181.0.0/18"),
					},
					Provider: core.Provider{},
				},
			}

			admissionHandler, _ = New()
		})

		It("should do nothing if the resource is not a Shoot", func() {
			attrs := admission.NewAttributesRecord(nil, nil, core.Kind("Foo").WithVersion("version"), shoot.Namespace, shoot.Name, core.Resource("foos").WithVersion("version"), "", admission.Delete, &metav1.DeleteOptions{}, false, nil)

			err := admissionHandler.Validate(context.TODO(), attrs, nil)
			Expect(err).NotTo(HaveOccurred())
		})

		It("should not allow setting the force-deletion annotation if the Shoot does not have a deletionTimestamp", func() {
			oldShoot := shoot.DeepCopy()
			metav1.SetMetaDataAnnotation(&shoot.ObjectMeta, v1beta1constants.AnnotationConfirmationForceDeletion, "1")
			shoot.Status = core.ShootStatus{
				LastErrors: []core.LastError{
					{
						Codes: []core.ErrorCode{core.ErrorConfigurationProblem},
					},
				},
			}

			attrs := admission.NewAttributesRecord(shoot, oldShoot, gardencorev1beta1.Kind("Shoot").WithVersion("v1beta1"), shoot.Namespace, shoot.Name, gardencorev1beta1.Resource("shoots").WithVersion("v1beta1"), "", admission.Update, &metav1.UpdateOptions{}, false, nil)

			err := admissionHandler.Validate(context.TODO(), attrs, nil)
			Expect(err).To(BeForbiddenError())
			Expect(err.Error()).To(ContainSubstring("force-deletion annotation cannot be set when Shoot deletionTimestamp is nil"))
		})

		It("should not allow setting the force-deletion annotation if the Shoot status does not have an ErrorCode", func() {
			metav1.SetMetaDataAnnotation(&shoot.ObjectMeta, v1beta1constants.AnnotationConfirmationForceDeletion, "T")
			shoot.DeletionTimestamp = &metav1.Time{Time: time.Now()}

			attrs := admission.NewAttributesRecord(shoot, nil, gardencorev1beta1.Kind("Shoot").WithVersion("v1beta1"), shoot.Namespace, shoot.Name, gardencorev1beta1.Resource("shoots").WithVersion("v1beta1"), "", admission.Create, &metav1.CreateOptions{}, false, nil)

			err := admissionHandler.Validate(context.TODO(), attrs, nil)
			Expect(err).To(BeForbiddenError())
			Expect(err.Error()).To(ContainSubstring("force-deletion annotation cannot be set when Shoot status does not contain one of these ErrorCode: [ERR_CLEANUP_CLUSTER_RESOURCES ERR_CONFIGURATION_PROBLEM ERR_INFRA_DEPENDENCIES ERR_INFRA_UNAUTHENTICATED ERR_INFRA_UNAUTHORIZED]"))
		})

		It("should not allow setting the force-deletion annotation if the Shoot status does not have a required ErrorCode", func() {
			metav1.SetMetaDataAnnotation(&shoot.ObjectMeta, v1beta1constants.AnnotationConfirmationForceDeletion, "T")
			shoot.DeletionTimestamp = &metav1.Time{Time: time.Now()}
			shoot.Status = core.ShootStatus{
				LastErrors: []core.LastError{
					{
						Codes: []core.ErrorCode{core.ErrorProblematicWebhook},
					},
				},
			}

			attrs := admission.NewAttributesRecord(shoot, nil, gardencorev1beta1.Kind("Shoot").WithVersion("v1beta1"), shoot.Namespace, shoot.Name, gardencorev1beta1.Resource("shoots").WithVersion("v1beta1"), "", admission.Create, &metav1.CreateOptions{}, false, nil)

			err := admissionHandler.Validate(context.TODO(), attrs, nil)
			Expect(err).To(BeForbiddenError())
			Expect(err.Error()).To(ContainSubstring("force-deletion annotation cannot be set when Shoot status does not contain one of these ErrorCode: [ERR_CLEANUP_CLUSTER_RESOURCES ERR_CONFIGURATION_PROBLEM ERR_INFRA_DEPENDENCIES ERR_INFRA_UNAUTHENTICATED ERR_INFRA_UNAUTHORIZED]"))
		})

		It("should not do anything if the both new and old Shoot have the annotation", func() {
			metav1.SetMetaDataAnnotation(&shoot.ObjectMeta, v1beta1constants.AnnotationConfirmationForceDeletion, "true")
			shoot.DeletionTimestamp = &metav1.Time{Time: time.Now()}
			oldShoot := shoot.DeepCopy()

			attrs := admission.NewAttributesRecord(shoot, oldShoot, gardencorev1beta1.Kind("Shoot").WithVersion("v1beta1"), shoot.Namespace, shoot.Name, gardencorev1beta1.Resource("shoots").WithVersion("v1beta1"), "", admission.Update, &metav1.UpdateOptions{}, false, nil)

			err := admissionHandler.Validate(context.TODO(), attrs, nil)
			Expect(err).NotTo(HaveOccurred())
		})

		It("should forbid to remove the annotation once set", func() {
			oldShoot := shoot.DeepCopy()
			metav1.SetMetaDataAnnotation(&oldShoot.ObjectMeta, v1beta1constants.AnnotationConfirmationForceDeletion, "true")
			shoot.DeletionTimestamp = &metav1.Time{Time: time.Now()}

			attrs := admission.NewAttributesRecord(shoot, oldShoot, gardencorev1beta1.Kind("Shoot").WithVersion("v1beta1"), shoot.Namespace, shoot.Name, gardencorev1beta1.Resource("shoots").WithVersion("v1beta1"), "", admission.Update, &metav1.UpdateOptions{}, false, nil)

			err := admissionHandler.Validate(context.TODO(), attrs, nil)
			Expect(err).To(BeForbiddenError())
			Expect(err.Error()).To(ContainSubstring("orce-deletion annotation cannot be removed once set"))
		})

		It("should allow setting the force-deletion annotation if the Shoot has a deletionTimestamp and the status has a required ErrorCode", func() {
			oldShoot := shoot.DeepCopy()
			metav1.SetMetaDataAnnotation(&shoot.ObjectMeta, v1beta1constants.AnnotationConfirmationForceDeletion, "true")
			shoot.DeletionTimestamp = &metav1.Time{Time: time.Now()}
			shoot.Status = core.ShootStatus{
				LastErrors: []core.LastError{
					{
						Codes: []core.ErrorCode{core.ErrorConfigurationProblem},
					},
				},
			}

			attrs := admission.NewAttributesRecord(shoot, oldShoot, gardencorev1beta1.Kind("Shoot").WithVersion("v1beta1"), shoot.Namespace, shoot.Name, gardencorev1beta1.Resource("shoots").WithVersion("v1beta1"), "", admission.Update, &metav1.UpdateOptions{}, false, nil)

			err := admissionHandler.Validate(context.TODO(), attrs, nil)
			Expect(err).NotTo(HaveOccurred())
		})
	})

	Describe("#Register", func() {
		It("should register the plugin", func() {
			plugins := admission.NewPlugins()
			Register(plugins)

			registered := plugins.Registered()
			Expect(registered).To(HaveLen(1))
			Expect(registered).To(ContainElement("ShootForceDeletion"))
		})
	})

	Describe("#New", func() {
		It("should only handle CREATE and UPDATE operations", func() {
			admissionHandler, err := New()
			Expect(err).ToNot(HaveOccurred())
			Expect(admissionHandler.Handles(admission.Create)).To(BeTrue())
			Expect(admissionHandler.Handles(admission.Connect)).NotTo(BeTrue())
			Expect(admissionHandler.Handles(admission.Update)).To(BeTrue())
			Expect(admissionHandler.Handles(admission.Delete)).NotTo(BeTrue())
		})
	})
})

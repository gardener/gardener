// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package project

import (
	"context"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"go.uber.org/mock/gomock"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	controllermanagerconfigv1alpha1 "github.com/gardener/gardener/pkg/controllermanager/apis/config/v1alpha1"
	mockclient "github.com/gardener/gardener/third_party/mock/controller-runtime/client"
)

var _ = Describe("Default Resource Quota", func() {
	var (
		ctrl *gomock.Controller
		c    *mockclient.MockClient
		ctx  = context.TODO()

		namespace   = "namespace"
		projectName = "name"
	)

	BeforeEach(func() {
		ctrl = gomock.NewController(GinkgoT())
		c = mockclient.NewMockClient(ctrl)
	})

	AfterEach(func() {
		ctrl.Finish()
	})

	Describe("#quotaConfigurationForProject", func() {
		var (
			project     *gardencorev1beta1.Project
			conf        controllermanagerconfigv1alpha1.ProjectControllerConfiguration
			fooSelector *metav1.LabelSelector

			resourceQuota, resourceQuota2 corev1.ResourceQuota
		)

		BeforeEach(func() {
			project = &gardencorev1beta1.Project{
				ObjectMeta: metav1.ObjectMeta{
					Name:      projectName,
					Namespace: namespace,
					UID:       "1",
				},
			}
			fooSelector, _ = metav1.ParseToLabelSelector("role = foo")
			resourceQuota = corev1.ResourceQuota{
				Spec: corev1.ResourceQuotaSpec{
					Hard: map[corev1.ResourceName]resource.Quantity{
						"count/foo": resource.MustParse("1"),
					},
				},
			}
			resourceQuota2 = *resourceQuota.DeepCopy()
			resourceQuota2.Spec.Hard["count/bar"] = resource.MustParse("1")
		})

		It("should return no quota configuration because no project controller config is specified", func() {
			Expect(quotaConfigurationForProject(conf, project)).To(BeNil())
		})

		It("should return no quota configuration because no quota config is specified", func() {
			conf = controllermanagerconfigv1alpha1.ProjectControllerConfiguration{}
			Expect(quotaConfigurationForProject(conf, project)).To(BeNil())
		})

		It("should return no quota configuration because label selector does not match project", func() {
			conf = controllermanagerconfigv1alpha1.ProjectControllerConfiguration{
				Quotas: []controllermanagerconfigv1alpha1.QuotaConfiguration{
					{
						ProjectSelector: fooSelector,
					},
				},
			}
			Expect(quotaConfigurationForProject(conf, project)).To(BeNil())
		})

		It("should return no quota configuration because label selector is invalid", func() {
			conf = controllermanagerconfigv1alpha1.ProjectControllerConfiguration{
				Quotas: []controllermanagerconfigv1alpha1.QuotaConfiguration{
					{
						ProjectSelector: &metav1.LabelSelector{
							MatchExpressions: []metav1.LabelSelectorRequirement{
								{},
							},
						},
					},
				},
			}
			quotaConf, err := quotaConfigurationForProject(conf, project)
			Expect(err).To(HaveOccurred())
			Expect(quotaConf).To(BeNil())
		})

		It("should return no quota configuration because label selector is nil", func() {
			conf = controllermanagerconfigv1alpha1.ProjectControllerConfiguration{
				Quotas: []controllermanagerconfigv1alpha1.QuotaConfiguration{
					{
						ProjectSelector: nil,
					},
				},
			}
			Expect(quotaConfigurationForProject(conf, project)).To(BeNil())
		})

		It("should return the quota configuration because label selector matches project", func() {
			conf = controllermanagerconfigv1alpha1.ProjectControllerConfiguration{
				Quotas: []controllermanagerconfigv1alpha1.QuotaConfiguration{
					{
						Config:          resourceQuota2,
						ProjectSelector: fooSelector,
					},
					{
						Config:          resourceQuota,
						ProjectSelector: &metav1.LabelSelector{},
					},
				},
			}
			Expect(quotaConfigurationForProject(conf, project)).To(Equal(&conf.Quotas[1]))
		})

		It("should return the first matching quota configuration", func() {
			additionalQuota := *resourceQuota.DeepCopy()
			additionalQuota.Spec.Hard["count/bar"] = resource.MustParse("2")
			conf = controllermanagerconfigv1alpha1.ProjectControllerConfiguration{
				Quotas: []controllermanagerconfigv1alpha1.QuotaConfiguration{
					{
						Config:          resourceQuota2,
						ProjectSelector: fooSelector,
					},
					{
						Config:          additionalQuota,
						ProjectSelector: &metav1.LabelSelector{},
					},
					{
						Config:          resourceQuota,
						ProjectSelector: &metav1.LabelSelector{},
					},
				},
			}
			Expect(quotaConfigurationForProject(conf, project)).To(Equal(&conf.Quotas[1]))
		})
	})

	Describe("#createOrUpdateResourceQuota", func() {
		var (
			project       *gardencorev1beta1.Project
			ownerRef      *metav1.OwnerReference
			resourceQuota corev1.ResourceQuota
			shoots        corev1.ResourceName
			secrets       corev1.ResourceName
			quantity      resource.Quantity
		)

		BeforeEach(func() {
			project = &gardencorev1beta1.Project{
				ObjectMeta: metav1.ObjectMeta{
					Name:      projectName,
					Namespace: namespace,
					UID:       "1",
				},
			}
			ownerRef = metav1.NewControllerRef(project, gardencorev1beta1.SchemeGroupVersion.WithKind("Project"))
			shoots = "shoots.core.gardener.cloud"
			secrets = "secrets"
			quantity = resource.MustParse("10")
			resourceQuota = corev1.ResourceQuota{
				ObjectMeta: metav1.ObjectMeta{
					Annotations: map[string]string{
						"foo": "bar",
					},
					Labels: map[string]string{
						"bar": "baz",
					},
				},
				Spec: corev1.ResourceQuotaSpec{
					Hard: map[corev1.ResourceName]resource.Quantity{
						shoots:  quantity,
						secrets: quantity,
					},
				},
			}
		})

		It("should create a new ResourceQuota", func() {
			config := controllermanagerconfigv1alpha1.QuotaConfiguration{
				Config: resourceQuota,
			}

			c.EXPECT().Get(gomock.Any(), client.ObjectKey{Namespace: namespace, Name: ResourceQuotaName}, gomock.AssignableToTypeOf(&corev1.ResourceQuota{})).
				Return(apierrors.NewNotFound(corev1.Resource("resourcequota"), "resourcequota"))

			expectedResourceQuota := resourceQuota.DeepCopy()
			expectedResourceQuota.SetOwnerReferences([]metav1.OwnerReference{*ownerRef})
			expectedResourceQuota.Labels = map[string]string{"bar": "baz"}
			expectedResourceQuota.Annotations = map[string]string{"foo": "bar"}
			expectedResourceQuota.SetName(ResourceQuotaName)
			expectedResourceQuota.SetNamespace(namespace)

			c.EXPECT().Create(gomock.Any(), expectedResourceQuota).Return(nil)

			Expect(createOrUpdateResourceQuota(ctx, c, namespace, ownerRef, config)).To(Succeed())
		})

		It("should update a existing ResourceQuota", func() {
			config := controllermanagerconfigv1alpha1.QuotaConfiguration{
				Config: resourceQuota,
			}

			existingOwnerRef := metav1.OwnerReference{Name: "foo"}
			existingResourceQuota := &corev1.ResourceQuota{
				ObjectMeta: metav1.ObjectMeta{
					Name:            ResourceQuotaName,
					Namespace:       namespace,
					OwnerReferences: []metav1.OwnerReference{existingOwnerRef},
				},
				Spec: corev1.ResourceQuotaSpec{
					Hard: map[corev1.ResourceName]resource.Quantity{
						shoots: resource.MustParse("50"),
					},
				},
			}

			c.EXPECT().Get(gomock.Any(), client.ObjectKey{Namespace: namespace, Name: ResourceQuotaName}, gomock.AssignableToTypeOf(&corev1.ResourceQuota{})).
				DoAndReturn(func(_ context.Context, _ client.ObjectKey, resourceQuota *corev1.ResourceQuota, _ ...client.GetOption) error {
					*resourceQuota = *existingResourceQuota
					return nil
				})

			expectedResourceQuota := existingResourceQuota.DeepCopy()
			expectedResourceQuota.SetOwnerReferences([]metav1.OwnerReference{existingOwnerRef, *ownerRef})
			expectedResourceQuota.Labels = map[string]string{"bar": "baz"}
			expectedResourceQuota.Annotations = map[string]string{"foo": "bar"}
			expectedResourceQuota.Spec.Hard[secrets] = quantity

			c.EXPECT().Patch(gomock.Any(), expectedResourceQuota, gomock.Any()).Return(nil)

			Expect(createOrUpdateResourceQuota(ctx, c, namespace, ownerRef, config)).To(Succeed())
		})
	})
})

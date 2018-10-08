// Copyright (c) 2018 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

package namespace_test

import (
	"github.com/gardener/gardener/pkg/apis/garden"
	gardeninformers "github.com/gardener/gardener/pkg/client/garden/informers/internalversion"
	"github.com/gardener/gardener/pkg/operation/common"
	. "github.com/gardener/gardener/plugin/pkg/project/namespace"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apiserver/pkg/admission"
	kubeinformers "k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes/fake"
	"k8s.io/client-go/testing"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gstruct"
)

var _ = Describe("namespace", func() {
	Describe("#Admit", func() {
		var (
			admissionHandler      *Namespace
			gardenInformerFactory gardeninformers.SharedInformerFactory
			kubeInformerFactory   kubeinformers.SharedInformerFactory
			kubeClient            *fake.Clientset

			project *garden.Project

			projectName = "dev"

			projectBase = &garden.Project{
				ObjectMeta: metav1.ObjectMeta{
					Name: projectName,
				},
			}
		)

		BeforeEach(func() {
			admissionHandler, _ = New()
			admissionHandler.AssignReadyFunc(func() bool { return true })

			kubeInformerFactory = kubeinformers.NewSharedInformerFactory(nil, 0)
			admissionHandler.SetKubeInformerFactory(kubeInformerFactory)

			kubeClient = &fake.Clientset{}
			admissionHandler.SetKubeClientset(kubeClient)

			gardenInformerFactory = gardeninformers.NewSharedInformerFactory(nil, 0)
			admissionHandler.SetInternalGardenInformerFactory(gardenInformerFactory)

			project = projectBase
		})

		It("should create a namespace because project does not explicitly set the namespace in its spec", func() {
			namespace := "garden-generated-project"

			kubeClient.AddReactor("create", "namespaces", func(action testing.Action) (bool, runtime.Object, error) {
				return true, &corev1.Namespace{
					ObjectMeta: metav1.ObjectMeta{
						Name: namespace,
					},
				}, nil
			})

			attrs := admission.NewAttributesRecord(project, nil, garden.Kind("Project").WithVersion("version"), project.Namespace, project.Name, garden.Resource("projects").WithVersion("version"), "", admission.Create, false, nil)
			err := admissionHandler.Admit(attrs)

			Expect(err).NotTo(HaveOccurred())
			Expect(project.Spec.Namespace).To(PointTo(Equal(namespace)))
		})

		It("should create the namespace because it does not exist", func() {
			namespace := "garden-foo"
			project.Spec.Namespace = &namespace

			kubeClient.AddReactor("get", "namespaces", func(action testing.Action) (bool, runtime.Object, error) {
				return true, nil, apierrors.NewNotFound(corev1.Resource("Namespace"), namespace)
			})
			kubeClient.AddReactor("create", "namespaces", func(action testing.Action) (bool, runtime.Object, error) {
				return true, &corev1.Namespace{
					ObjectMeta: metav1.ObjectMeta{
						Name: namespace,
					},
				}, nil
			})

			attrs := admission.NewAttributesRecord(project, nil, garden.Kind("Project").WithVersion("version"), project.Namespace, project.Name, garden.Resource("projects").WithVersion("version"), "", admission.Create, false, nil)
			err := admissionHandler.Admit(attrs)

			Expect(err).NotTo(HaveOccurred())
		})

		It("should fail because the referenced namespace does not have the project role label", func() {
			namespace := "kube-system"
			project.Spec.Namespace = &namespace

			kubeInformerFactory.Core().V1().Namespaces().Informer().GetStore().Add(&corev1.Namespace{
				ObjectMeta: metav1.ObjectMeta{
					Name: namespace,
				},
			})

			attrs := admission.NewAttributesRecord(project, nil, garden.Kind("Project").WithVersion("version"), project.Namespace, project.Name, garden.Resource("projects").WithVersion("version"), "", admission.Create, false, nil)
			err := admissionHandler.Admit(attrs)

			Expect(err).To(HaveOccurred())
			Expect(apierrors.IsForbidden(err)).To(BeTrue())
		})

		It("should fail because the referenced namespace is already referenced by another projects", func() {
			namespace := "garden-foo"
			project.Spec.Namespace = &namespace

			gardenInformerFactory.Garden().InternalVersion().Projects().Informer().GetStore().Add(&garden.Project{
				ObjectMeta: metav1.ObjectMeta{
					Name: "conflicting",
				},
				Spec: garden.ProjectSpec{
					Namespace: &namespace,
				},
			})
			kubeInformerFactory.Core().V1().Namespaces().Informer().GetStore().Add(&corev1.Namespace{
				ObjectMeta: metav1.ObjectMeta{
					Name: namespace,
				},
			})

			attrs := admission.NewAttributesRecord(project, nil, garden.Kind("Project").WithVersion("version"), project.Namespace, project.Name, garden.Resource("projects").WithVersion("version"), "", admission.Create, false, nil)
			err := admissionHandler.Admit(attrs)

			Expect(err).To(HaveOccurred())
			Expect(apierrors.IsForbidden(err)).To(BeTrue())
		})

		It("should succeed", func() {
			namespace := "kube-system"
			project.Spec.Namespace = &namespace

			kubeInformerFactory.Core().V1().Namespaces().Informer().GetStore().Add(&corev1.Namespace{
				ObjectMeta: metav1.ObjectMeta{
					Name: namespace,
					Labels: map[string]string{
						common.GardenRole:  common.GardenRoleProject,
						common.ProjectName: project.Name,
					},
				},
			})

			attrs := admission.NewAttributesRecord(project, nil, garden.Kind("Project").WithVersion("version"), project.Namespace, project.Name, garden.Resource("projects").WithVersion("version"), "", admission.Create, false, nil)
			err := admissionHandler.Admit(attrs)

			Expect(err).NotTo(HaveOccurred())
		})
	})

	Describe("#Register", func() {
		It("should register the plugin", func() {
			plugins := admission.NewPlugins()
			Register(plugins)

			registered := plugins.Registered()
			Expect(registered).To(HaveLen(1))
			Expect(registered).To(ContainElement(PluginName))
		})
	})

	Describe("#New", func() {
		It("should only handle CREATE operations", func() {
			handler, err := New()

			Expect(err).ToNot(HaveOccurred())
			Expect(handler.Handles(admission.Create)).To(BeTrue())
			Expect(handler.Handles(admission.Update)).NotTo(BeTrue())
			Expect(handler.Handles(admission.Connect)).NotTo(BeTrue())
			Expect(handler.Handles(admission.Delete)).NotTo(BeTrue())
		})
	})

	Describe("#ValidateInitialization", func() {
		It("should return error if no ProjectLister or NamespaceLister is set", func() {
			handler, _ := New()

			err := handler.ValidateInitialization()

			Expect(err).To(HaveOccurred())
		})

		It("should not return error if ProjectLister or NamespaceLister are set", func() {
			handler, _ := New()
			handler.SetInternalGardenInformerFactory(gardeninformers.NewSharedInformerFactory(nil, 0))
			handler.SetKubeInformerFactory(kubeinformers.NewSharedInformerFactory(nil, 0))
			handler.SetKubeClientset(&fake.Clientset{})

			err := handler.ValidateInitialization()

			Expect(err).ToNot(HaveOccurred())
		})
	})
})

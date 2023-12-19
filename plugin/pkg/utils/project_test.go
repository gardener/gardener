// SPDX-FileCopyrightText: 2022 SAP SE or an SAP affiliate company and Gardener contributors
// SPDX-License-Identifier: Apache-2.0

package utils_test

import (
	"fmt"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime/schema"

	gardencore "github.com/gardener/gardener/pkg/apis/core"
	gardencorelisters "github.com/gardener/gardener/pkg/client/core/listers/core/internalversion"
	"github.com/gardener/gardener/plugin/pkg/utils"
)

var _ = Describe("Project", func() {
	var (
		fakeErr = fmt.Errorf("fake err")

		namespaceName = "foo"

		projectName     = "bar"
		projectInternal = &gardencore.Project{
			ObjectMeta: metav1.ObjectMeta{
				Name: projectName,
			},
			Spec: gardencore.ProjectSpec{
				Namespace: &namespaceName,
			},
		}
	)

	Describe("#ProjectForNamespaceFromInternalLister", func() {
		var lister *fakeInternalLister

		BeforeEach(func() {
			lister = &fakeInternalLister{}
		})

		It("should return an error because listing failed", func() {
			lister.err = fakeErr

			result, err := utils.ProjectForNamespaceFromInternalLister(lister, namespaceName)
			Expect(err).To(MatchError(fakeErr))
			Expect(result).To(BeNil())
		})

		It("should return the found project", func() {
			lister.projects = []*gardencore.Project{projectInternal}

			result, err := utils.ProjectForNamespaceFromInternalLister(lister, namespaceName)
			Expect(err).NotTo(HaveOccurred())
			Expect(result).To(Equal(projectInternal))
		})

		It("should return a 'not found' error", func() {
			result, err := utils.ProjectForNamespaceFromInternalLister(lister, namespaceName)
			Expect(err).To(MatchError(apierrors.NewNotFound(schema.GroupResource{Group: "core.gardener.cloud", Resource: "Project"}, namespaceName)))
			Expect(result).To(BeNil())
		})
	})
})

type fakeInternalLister struct {
	gardencorelisters.ProjectLister
	projects []*gardencore.Project
	err      error
}

func (c *fakeInternalLister) List(labels.Selector) ([]*gardencore.Project, error) {
	return c.projects, c.err
}

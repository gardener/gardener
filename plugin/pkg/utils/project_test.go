// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package utils_test

import (
	"errors"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	gardencorev1beta1listers "github.com/gardener/gardener/pkg/client/core/listers/core/v1beta1"
	. "github.com/gardener/gardener/pkg/utils/test/matchers"
	"github.com/gardener/gardener/plugin/pkg/utils"
)

var _ = Describe("Project", func() {
	var (
		fakeErr = errors.New("fake err")

		namespaceName = "foo"

		projectName     = "bar"
		projectInternal = &gardencorev1beta1.Project{
			ObjectMeta: metav1.ObjectMeta{
				Name: projectName,
			},
			Spec: gardencorev1beta1.ProjectSpec{
				Namespace: &namespaceName,
			},
		}
	)

	Describe("#ProjectForNamespaceFromLister", func() {
		var lister *fakeInternalLister

		BeforeEach(func() {
			lister = &fakeInternalLister{}
		})

		It("should return an error because listing failed", func() {
			lister.err = fakeErr

			result, err := utils.ProjectForNamespaceFromLister(lister, namespaceName)
			Expect(err).To(MatchError(fakeErr))
			Expect(result).To(BeNil())
		})

		It("should return the found project", func() {
			lister.projects = []*gardencorev1beta1.Project{projectInternal}

			result, err := utils.ProjectForNamespaceFromLister(lister, namespaceName)
			Expect(err).NotTo(HaveOccurred())
			Expect(result).To(Equal(projectInternal))
		})

		It("should return a 'not found' error", func() {
			result, err := utils.ProjectForNamespaceFromLister(lister, namespaceName)
			Expect(err).To(BeNotFoundError())
			Expect(result).To(BeNil())
		})
	})
})

type fakeInternalLister struct {
	gardencorev1beta1listers.ProjectLister
	projects []*gardencorev1beta1.Project
	err      error
}

func (c *fakeInternalLister) List(labels.Selector) ([]*gardencorev1beta1.Project, error) {
	return c.projects, c.err
}

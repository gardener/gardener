// SPDX-FileCopyrightText: Contributors to the Gardener project
//
// SPDX-License-Identifier: Apache-2.0

package gardener_test

import (
	"context"
	"errors"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	authorizationv1 "k8s.io/api/authorization/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apiserver/pkg/authentication/user"
	clientauthorizationv1 "k8s.io/client-go/kubernetes/typed/authorization/v1"

	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	. "github.com/gardener/gardener/pkg/utils/gardener"
)

var _ = Describe("Authorization", func() {
	var (
		ctx      context.Context
		reviewer *fakeSubjectAccessReviewer
		u        *user.DefaultInfo
	)

	BeforeEach(func() {
		ctx = context.Background()
		reviewer = &fakeSubjectAccessReviewer{}
		u = &user.DefaultInfo{
			Name:   "test-user",
			UID:    "test-uid",
			Groups: []string{"group-1", "group-2"},
			Extra: map[string][]string{
				"scopes": {"scope-1", "scope-2"},
				"tenant": {"test-tenant"},
			},
		}
	})

	Describe("#GetViewerUserGroups", func() {
		It("should return the system viewers group when access is allowed", func() {
			reviewer.allowed = true

			Expect(GetViewerUserGroups(ctx, u, reviewer)).To(Equal([]string{v1beta1constants.ShootSystemViewersGroupName}))
		})

		It("should return the project viewers group when access is not allowed", func() {
			reviewer.allowed = false

			Expect(GetViewerUserGroups(ctx, u, reviewer)).To(Equal([]string{v1beta1constants.ShootProjectViewersGroupName}))
		})

		It("should return the error from the subject access review", func() {
			reviewer.err = errors.New("subject access review failed")

			groups, err := GetViewerUserGroups(ctx, u, reviewer)
			Expect(err).To(MatchError("subject access review failed"))
			Expect(groups).To(BeNil())
		})

		It("should check whether the user can list projects at cluster scope", func() {
			_, err := GetViewerUserGroups(ctx, u, reviewer)
			Expect(err).NotTo(HaveOccurred())

			Expect(reviewer.reviews).To(HaveLen(1))
			Expect(reviewer.reviews[0].Spec.ResourceAttributes).To(Equal(&authorizationv1.ResourceAttributes{
				Namespace: "",
				Group:     "core.gardener.cloud",
				Resource:  "projects",
				Verb:      "list",
			}))
		})

		It("should forward the user identity to the subject access review", func() {
			_, err := GetViewerUserGroups(ctx, u, reviewer)
			Expect(err).NotTo(HaveOccurred())

			Expect(reviewer.reviews).To(HaveLen(1))
			Expect(reviewer.reviews[0].Spec.User).To(Equal("test-user"))
			Expect(reviewer.reviews[0].Spec.UID).To(Equal("test-uid"))
			Expect(reviewer.reviews[0].Spec.Groups).To(Equal([]string{"group-1", "group-2"}))
			Expect(reviewer.reviews[0].Spec.Extra).To(Equal(map[string]authorizationv1.ExtraValue{
				"scopes": {"scope-1", "scope-2"},
				"tenant": {"test-tenant"},
			}))
		})

		It("should forward nil extra information as nil", func() {
			u.Extra = nil

			_, err := GetViewerUserGroups(ctx, u, reviewer)
			Expect(err).NotTo(HaveOccurred())

			Expect(reviewer.reviews).To(HaveLen(1))
			Expect(reviewer.reviews[0].Spec.Extra).To(BeNil())
		})

		It("should forward empty extra information as empty", func() {
			u.Extra = map[string][]string{}

			_, err := GetViewerUserGroups(ctx, u, reviewer)
			Expect(err).NotTo(HaveOccurred())

			Expect(reviewer.reviews).To(HaveLen(1))
			Expect(reviewer.reviews[0].Spec.Extra).To(Equal(map[string]authorizationv1.ExtraValue{}))
		})
	})

	Describe("#GetAdminUserGroups", func() {
		It("should return the system admins group when access is allowed", func() {
			reviewer.allowed = true

			Expect(GetAdminUserGroups(ctx, u, reviewer)).To(Equal([]string{v1beta1constants.ShootSystemAdminsGroupName}))
		})

		It("should return the project admins group when access is not allowed", func() {
			reviewer.allowed = false

			Expect(GetAdminUserGroups(ctx, u, reviewer)).To(Equal([]string{v1beta1constants.ShootProjectAdminsGroupName}))
		})

		It("should return the error from the subject access review", func() {
			reviewer.err = errors.New("subject access review failed")

			groups, err := GetAdminUserGroups(ctx, u, reviewer)
			Expect(err).To(MatchError("subject access review failed"))
			Expect(groups).To(BeNil())
		})

		It("should check whether the user can list secrets at cluster scope", func() {
			_, err := GetAdminUserGroups(ctx, u, reviewer)
			Expect(err).NotTo(HaveOccurred())

			Expect(reviewer.reviews).To(HaveLen(1))
			Expect(reviewer.reviews[0].Spec.ResourceAttributes).To(Equal(&authorizationv1.ResourceAttributes{
				Namespace: "",
				Group:     "",
				Resource:  "secrets",
				Verb:      "list",
			}))
		})

		It("should forward the user identity to the subject access review", func() {
			_, err := GetAdminUserGroups(ctx, u, reviewer)
			Expect(err).NotTo(HaveOccurred())

			Expect(reviewer.reviews).To(HaveLen(1))
			Expect(reviewer.reviews[0].Spec.User).To(Equal("test-user"))
			Expect(reviewer.reviews[0].Spec.UID).To(Equal("test-uid"))
			Expect(reviewer.reviews[0].Spec.Groups).To(Equal([]string{"group-1", "group-2"}))
			Expect(reviewer.reviews[0].Spec.Extra).To(Equal(map[string]authorizationv1.ExtraValue{
				"scopes": {"scope-1", "scope-2"},
				"tenant": {"test-tenant"},
			}))
		})

		It("should forward nil extra information as nil", func() {
			u.Extra = nil

			_, err := GetAdminUserGroups(ctx, u, reviewer)
			Expect(err).NotTo(HaveOccurred())

			Expect(reviewer.reviews).To(HaveLen(1))
			Expect(reviewer.reviews[0].Spec.Extra).To(BeNil())
		})

		It("should forward empty extra information as empty", func() {
			u.Extra = map[string][]string{}

			_, err := GetAdminUserGroups(ctx, u, reviewer)
			Expect(err).NotTo(HaveOccurred())

			Expect(reviewer.reviews).To(HaveLen(1))
			Expect(reviewer.reviews[0].Spec.Extra).To(Equal(map[string]authorizationv1.ExtraValue{}))
		})
	})
})

type fakeSubjectAccessReviewer struct {
	clientauthorizationv1.SubjectAccessReviewInterface

	allowed bool
	err     error

	// reviews captures all SubjectAccessReviews passed to Create, for assertions.
	reviews []*authorizationv1.SubjectAccessReview
}

func (f *fakeSubjectAccessReviewer) Create(_ context.Context, review *authorizationv1.SubjectAccessReview, _ metav1.CreateOptions) (*authorizationv1.SubjectAccessReview, error) {
	f.reviews = append(f.reviews, review.DeepCopy())

	if f.err != nil {
		return nil, f.err
	}

	ret := review.DeepCopy()
	ret.Status = authorizationv1.SubjectAccessReviewStatus{Allowed: f.allowed}
	return ret, nil
}

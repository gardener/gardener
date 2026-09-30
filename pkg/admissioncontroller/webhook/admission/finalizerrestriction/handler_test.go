// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package finalizerrestriction_test

import (
	"context"
	"encoding/json"
	"errors"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	admissionv1 "k8s.io/api/admission/v1"
	authenticationv1 "k8s.io/api/authentication/v1"
	authorizationv1 "k8s.io/api/authorization/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/sets"
	clientauthorizationv1 "k8s.io/client-go/kubernetes/typed/authorization/v1"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"

	"github.com/gardener/gardener/pkg/admissioncontroller/webhook/admission/finalizerrestriction"
)

var _ = Describe("handler", func() {
	var (
		ctx      context.Context
		reviewer *fakeSubjectAccessReviewer
		handler  *finalizerrestriction.Handler
		req      admission.Request

		// rawWithFinalizers marshals an object carrying the given finalizers into its metadata.
		rawWithFinalizers = func(finalizers ...string) []byte {
			obj := &metav1.PartialObjectMetadata{}
			obj.Finalizers = finalizers
			raw, err := json.Marshal(obj)
			Expect(err).NotTo(HaveOccurred())
			return raw
		}
	)

	BeforeEach(func() {
		ctx = context.TODO()
		reviewer = &fakeSubjectAccessReviewer{allowed: false}
		handler = &finalizerrestriction.Handler{
			ProtectedFinalizers:   sets.New("gardener", "gardener.cloud/reference-protection"),
			SubjectAccessReviewer: reviewer,
		}

		req = admission.Request{}
		req.UserInfo = authenticationv1.UserInfo{Username: "some-project-admin"}
		req.Resource = metav1.GroupVersionResource{Resource: "shoots"}
		req.Operation = admissionv1.Update
		// By default the request removes the protected "gardener" finalizer.
		req.OldObject.Raw = rawWithFinalizers("gardener", "some-other-finalizer")
		req.Object.Raw = rawWithFinalizers("some-other-finalizer")
	})

	Describe("#Handle", func() {
		Context("no protected finalizer is modified", func() {
			It("should allow when only an unprotected finalizer is removed", func() {
				req.OldObject.Raw = rawWithFinalizers("gardener", "some-other-finalizer")
				req.Object.Raw = rawWithFinalizers("gardener")

				resp := handler.Handle(ctx, req)
				Expect(resp.Allowed).To(BeTrue())
				Expect(resp.Result.Message).To(Equal("No protected finalizers were modified"))
			})

			It("should allow when no finalizers change at all", func() {
				req.OldObject.Raw = rawWithFinalizers("gardener")
				req.Object.Raw = rawWithFinalizers("gardener")

				resp := handler.Handle(ctx, req)
				Expect(resp.Allowed).To(BeTrue())
				Expect(resp.Result.Message).To(Equal("No protected finalizers were modified"))
			})

			It("should allow an unprivileged user to add an unprotected finalizer", func() {
				req.OldObject.Raw = rawWithFinalizers()
				req.Object.Raw = rawWithFinalizers("some-other-finalizer")

				resp := handler.Handle(ctx, req)
				Expect(resp.Allowed).To(BeTrue())
				Expect(resp.Result.Message).To(Equal("No protected finalizers were modified"))
			})
		})

		Context("privileged principals may modify a protected finalizer", func() {
			DescribeTable("should allow the request",
				func(userInfo authenticationv1.UserInfo, systemAdmin bool, expectedMessageSubstring string) {
					req.UserInfo = userInfo
					reviewer.allowed = systemAdmin

					resp := handler.Handle(ctx, req)
					Expect(resp.Allowed).To(BeTrue())
					Expect(resp.Result.Message).To(ContainSubstring(expectedMessageSubstring))
					Expect(resp.Result.Message).To(ContainSubstring("gardener"))
				},

				Entry("self-hosted shoot gardenlet (shoot identity)",
					authenticationv1.UserInfo{Username: "gardener.cloud:system:shoot:foo:bar", Groups: []string{"gardener.cloud:system:shoots"}},
					false, "gardenlet is allowed to modify protected finalizer"),
				Entry("gardenadm (shoot identity)",
					authenticationv1.UserInfo{Username: "gardener.cloud:gardenadm:shoot:foo:bar", Groups: []string{"gardener.cloud:system:shoots"}},
					false, "gardenadm is allowed to modify protected finalizer"),
				Entry("extension (shoot identity)",
					authenticationv1.UserInfo{Username: "system:serviceaccount:garden-my-project:extension-shoot--myshoot--myext", Groups: []string{"system:serviceaccounts", "system:serviceaccounts:garden-my-project"}},
					false, "extension is allowed to modify protected finalizer"),
				Entry("seed gardenlet (seed identity)",
					authenticationv1.UserInfo{Username: "gardener.cloud:system:seed:foo", Groups: []string{"gardener.cloud:system:seeds"}},
					false, "gardenlet is allowed to modify protected finalizer"),
				Entry("extension (seed identity)",
					authenticationv1.UserInfo{Username: "system:serviceaccount:seed-foo:extension-bar", Groups: []string{"system:serviceaccounts", "system:serviceaccounts:seed-foo"}},
					false, "extension is allowed to modify protected finalizer"),
				Entry("kube-system service account gardener-internal",
					authenticationv1.UserInfo{Username: "system:serviceaccount:kube-system:gardener-internal", Groups: []string{"system:serviceaccounts", "system:serviceaccounts:kube-system"}},
					false, "kube-system service account"),
				Entry("kube-system service account generic-garbage-collector",
					authenticationv1.UserInfo{Username: "system:serviceaccount:kube-system:generic-garbage-collector", Groups: []string{"system:serviceaccounts", "system:serviceaccounts:kube-system"}},
					false, "kube-system service account"),
				Entry("kube-system service account",
					authenticationv1.UserInfo{Username: "system:serviceaccount:kube-system:some-controller", Groups: []string{"system:serviceaccounts", "system:serviceaccounts:kube-system"}},
					false, "kube-system service account"),
				Entry("cluster administrator (system:masters)",
					authenticationv1.UserInfo{Username: "some-admin", Groups: []string{"system:masters"}},
					false, "cluster administrator"),
				Entry("system administrator via SubjectAccessReview",
					authenticationv1.UserInfo{Username: "some-ops-user"},
					true, "system administrator user"),
			)
		})

		Context("finalizer additions are gated as well as removals", func() {
			It("should deny an unprivileged user adding a protected finalizer", func() {
				req.OldObject.Raw = rawWithFinalizers()
				req.Object.Raw = rawWithFinalizers("gardener")

				resp := handler.Handle(ctx, req)
				Expect(resp.Allowed).To(BeFalse())
				Expect(resp.Result.Code).To(Equal(int32(403)))
				Expect(resp.Result.Message).To(ContainSubstring("not allowed to modify protected finalizer"))
			})

			It("should allow a privileged user adding a protected finalizer", func() {
				req.UserInfo = authenticationv1.UserInfo{Username: "gardener.cloud:system:seed:foo", Groups: []string{"gardener.cloud:system:seeds"}}
				req.OldObject.Raw = rawWithFinalizers()
				req.Object.Raw = rawWithFinalizers("gardener")

				resp := handler.Handle(ctx, req)
				Expect(resp.Allowed).To(BeTrue())
			})
		})

		Context("the reference-protection finalizer is protected too", func() {
			It("should deny an unprivileged user removing the reference-protection finalizer", func() {
				req.OldObject.Raw = rawWithFinalizers("gardener.cloud/reference-protection")
				req.Object.Raw = rawWithFinalizers()

				resp := handler.Handle(ctx, req)
				Expect(resp.Allowed).To(BeFalse())
				Expect(resp.Result.Code).To(Equal(int32(403)))
				Expect(resp.Result.Message).To(ContainSubstring("gardener.cloud/reference-protection"))
			})
		})

		Context("CREATE operations", func() {
			It("should allow a CREATE with a protected finalizer for a privileged user", func() {
				req.Operation = admissionv1.Create
				req.UserInfo = authenticationv1.UserInfo{Username: "gardener.cloud:system:seed:foo", Groups: []string{"gardener.cloud:system:seeds"}}
				req.OldObject.Raw = nil
				req.Object.Raw = rawWithFinalizers("gardener")

				resp := handler.Handle(ctx, req)
				Expect(resp.Allowed).To(BeTrue())
			})

			It("should deny a CREATE with a protected finalizer for an unprivileged user", func() {
				req.Operation = admissionv1.Create
				req.OldObject.Raw = nil
				req.Object.Raw = rawWithFinalizers("gardener")

				resp := handler.Handle(ctx, req)
				Expect(resp.Allowed).To(BeFalse())
				Expect(resp.Result.Code).To(Equal(int32(403)))
			})

			It("should allow a CREATE without any protected finalizer regardless of user", func() {
				req.Operation = admissionv1.Create
				req.OldObject.Raw = nil
				req.Object.Raw = rawWithFinalizers("some-other-finalizer")

				resp := handler.Handle(ctx, req)
				Expect(resp.Allowed).To(BeTrue())
				Expect(resp.Result.Message).To(Equal("No protected finalizers were modified"))
			})
		})

		Context("denials and errors", func() {
			It("should deny a project administrator removing a protected finalizer", func() {
				reviewer.allowed = false

				resp := handler.Handle(ctx, req)
				Expect(resp.Allowed).To(BeFalse())
				Expect(resp.Result.Code).To(Equal(int32(403)))
				Expect(resp.Result.Message).To(ContainSubstring(`user "some-project-admin" is not allowed to modify protected finalizer`))
				Expect(resp.Result.Message).To(ContainSubstring("gardener"))
			})

			It("should error if the SubjectAccessReview fails", func() {
				reviewer.evaluationError = "review failed"

				resp := handler.Handle(ctx, req)
				Expect(resp.Allowed).To(BeFalse())
				Expect(resp.Result.Code).To(Equal(int32(500)))
			})

			It("should error if the new object cannot be decoded", func() {
				req.Object.Raw = []byte("not-json")

				resp := handler.Handle(ctx, req)
				Expect(resp.Allowed).To(BeFalse())
				Expect(resp.Result.Code).To(Equal(int32(500)))
			})

			It("should error if the old object cannot be decoded", func() {
				req.OldObject.Raw = []byte("not-json")

				resp := handler.Handle(ctx, req)
				Expect(resp.Allowed).To(BeFalse())
				Expect(resp.Result.Code).To(Equal(int32(500)))
			})
		})

		Context("the SubjectAccessReview probes cluster-wide secret list", func() {
			It("should ask whether the user can list secrets at cluster scope", func() {
				resp := handler.Handle(ctx, req)
				Expect(resp.Allowed).To(BeFalse())

				Expect(reviewer.lastReview).NotTo(BeNil())
				attrs := reviewer.lastReview.Spec.ResourceAttributes
				Expect(attrs).NotTo(BeNil())
				Expect(attrs.Group).To(Equal(""))
				Expect(attrs.Resource).To(Equal("secrets"))
				Expect(attrs.Verb).To(Equal("list"))
				Expect(attrs.Namespace).To(BeEmpty(), "empty namespace means cluster scope")
				Expect(reviewer.lastReview.Spec.User).To(Equal("some-project-admin"))
			})
		})
	})
})

type fakeSubjectAccessReviewer struct {
	// allowed is true when the user has gardener system-wide permissions.
	allowed         bool
	reason          string
	evaluationError string

	// lastReview captures the most recent SubjectAccessReview passed to Create, for assertions.
	lastReview *authorizationv1.SubjectAccessReview
}

var _ clientauthorizationv1.SubjectAccessReviewInterface = (*fakeSubjectAccessReviewer)(nil)

func (f *fakeSubjectAccessReviewer) Create(_ context.Context, subjectAccessReview *authorizationv1.SubjectAccessReview, _ metav1.CreateOptions) (*authorizationv1.SubjectAccessReview, error) {
	f.lastReview = subjectAccessReview.DeepCopy()

	if f.evaluationError != "" {
		return nil, errors.New(f.evaluationError)
	}

	ret := subjectAccessReview.DeepCopy()
	ret.Status = authorizationv1.SubjectAccessReviewStatus{
		Allowed: f.allowed,
		Denied:  !f.allowed,
		Reason:  f.reason,
	}
	return ret, nil
}

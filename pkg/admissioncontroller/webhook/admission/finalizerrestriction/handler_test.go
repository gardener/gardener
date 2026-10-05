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
	)

	getRawObjWithFinalizers := func(finalizers ...string) []byte {
		obj := &metav1.PartialObjectMetadata{}
		obj.Finalizers = finalizers
		raw, err := json.Marshal(obj)
		Expect(err).NotTo(HaveOccurred())
		return raw
	}

	BeforeEach(func() {
		ctx = context.Background()
		reviewer = &fakeSubjectAccessReviewer{denied: true}
		handler = &finalizerrestriction.Handler{
			ProtectedFinalizers:   sets.New("gardener", "gardener.cloud/reference-protection"),
			SubjectAccessReviewer: reviewer,
		}
	})

	Describe("#Handle", func() {
		Context("trusted identities", func() {
			expectAllowedWithoutReview := func(req admission.Request, message string) {
				resp := handler.Handle(ctx, req)
				Expect(resp.Allowed).To(BeTrue())
				Expect(resp.Result.Message).To(Equal(message))
				Expect(reviewer.lastReview).To(BeNil())
			}

			It("should allow a (self-hosted) shoot gardenlet", func() {
				req := admission.Request{AdmissionRequest: admissionv1.AdmissionRequest{
					UserInfo:  authenticationv1.UserInfo{Username: "gardener.cloud:system:shoot:foo:bar", Groups: []string{"gardener.cloud:system:shoots"}},
					Operation: admissionv1.Update,
				}}
				req.OldObject.Raw = getRawObjWithFinalizers("gardener")
				req.Object.Raw = getRawObjWithFinalizers()
				expectAllowedWithoutReview(req, "gardenlet is allowed to modify protected finalizer(s)")
			})

			It("should allow gardenadm", func() {
				req := admission.Request{AdmissionRequest: admissionv1.AdmissionRequest{
					UserInfo:  authenticationv1.UserInfo{Username: "gardener.cloud:gardenadm:shoot:foo:bar", Groups: []string{"gardener.cloud:system:shoots"}},
					Operation: admissionv1.Update,
				}}
				req.OldObject.Raw = getRawObjWithFinalizers("gardener")
				req.Object.Raw = getRawObjWithFinalizers()
				expectAllowedWithoutReview(req, "gardenadm is allowed to modify protected finalizer(s)")
			})

			It("should allow a shoot extension", func() {
				req := admission.Request{AdmissionRequest: admissionv1.AdmissionRequest{
					UserInfo:  authenticationv1.UserInfo{Username: "system:serviceaccount:garden-my-project:extension-shoot--myshoot--myext", Groups: []string{"system:serviceaccounts", "system:serviceaccounts:garden-my-project"}},
					Operation: admissionv1.Update,
				}}
				req.OldObject.Raw = getRawObjWithFinalizers("gardener")
				req.Object.Raw = getRawObjWithFinalizers()
				expectAllowedWithoutReview(req, "extension is allowed to modify protected finalizer(s)")
			})

			It("should allow a (seed) gardenlet to add a protected finalizer", func() {
				req := admission.Request{AdmissionRequest: admissionv1.AdmissionRequest{
					UserInfo:  authenticationv1.UserInfo{Username: "gardener.cloud:system:seed:foo", Groups: []string{"gardener.cloud:system:seeds"}},
					Operation: admissionv1.Create,
				}}
				req.Object.Raw = getRawObjWithFinalizers("gardener")
				expectAllowedWithoutReview(req, "gardenlet is allowed to modify protected finalizer(s)")
			})

			It("should allow a seed extension", func() {
				req := admission.Request{AdmissionRequest: admissionv1.AdmissionRequest{
					UserInfo:  authenticationv1.UserInfo{Username: "system:serviceaccount:seed-foo:extension-bar", Groups: []string{"system:serviceaccounts", "system:serviceaccounts:seed-foo"}},
					Operation: admissionv1.Update,
				}}
				req.OldObject.Raw = getRawObjWithFinalizers("gardener")
				req.Object.Raw = getRawObjWithFinalizers()
				expectAllowedWithoutReview(req, "extension is allowed to modify protected finalizer(s)")
			})

			It("should allow a kube-system service account", func() {
				req := admission.Request{AdmissionRequest: admissionv1.AdmissionRequest{
					UserInfo:  authenticationv1.UserInfo{Username: "system:serviceaccount:kube-system:some-controller", Groups: []string{"system:serviceaccounts", "system:serviceaccounts:kube-system"}},
					Operation: admissionv1.Update,
				}}
				req.OldObject.Raw = getRawObjWithFinalizers("gardener")
				req.Object.Raw = getRawObjWithFinalizers()
				expectAllowedWithoutReview(req, `"system:serviceaccount:kube-system:some-controller" is allowed to modify protected finalizer(s)`)
			})

			It("should allow a system:masters user", func() {
				req := admission.Request{AdmissionRequest: admissionv1.AdmissionRequest{
					UserInfo:  authenticationv1.UserInfo{Username: "some-admin", Groups: []string{"system:masters"}},
					Operation: admissionv1.Update,
				}}
				req.OldObject.Raw = getRawObjWithFinalizers("gardener")
				req.Object.Raw = getRawObjWithFinalizers()
				reviewer.createError = errors.New("authorization is unavailable")
				expectAllowedWithoutReview(req, `"some-admin" is allowed to modify protected finalizer(s)`)
			})
		})

		Context("C(R)UD operations", func() {
			It("should deny a CREATE of an object with a protected finalizer", func() {
				req := admission.Request{AdmissionRequest: admissionv1.AdmissionRequest{
					UserInfo: authenticationv1.UserInfo{Username: "some-project-admin"}, Operation: admissionv1.Create,
				}}
				req.Object.Raw = getRawObjWithFinalizers("gardener")

				resp := handler.Handle(ctx, req)
				Expect(resp.Allowed).To(BeFalse())
				Expect(resp.Result.Code).To(Equal(int32(403)))
				Expect(resp.Result.Message).To(Equal(`user "some-project-admin" is not allowed to modify protected finalizer(s) [gardener]`))
				Expect(reviewer.lastReview).NotTo(BeNil())
			})

			It("should deny a UPDATE of an object removing a protected finalizer", func() {
				req := admission.Request{AdmissionRequest: admissionv1.AdmissionRequest{
					UserInfo: authenticationv1.UserInfo{Username: "some-project-admin"}, Operation: admissionv1.Update,
				}}
				req.OldObject.Raw = getRawObjWithFinalizers("gardener.cloud/reference-protection")
				req.Object.Raw = getRawObjWithFinalizers()

				resp := handler.Handle(ctx, req)
				Expect(resp.Allowed).To(BeFalse())
				Expect(resp.Result.Code).To(Equal(int32(403)))
				Expect(resp.Result.Message).To(Equal(`user "some-project-admin" is not allowed to modify protected finalizer(s) [gardener.cloud/reference-protection]`))
				Expect(reviewer.lastReview).NotTo(BeNil())
			})

			It("should deny a UPDATE of an object adding a protected finalizer", func() {
				req := admission.Request{AdmissionRequest: admissionv1.AdmissionRequest{
					UserInfo: authenticationv1.UserInfo{Username: "some-project-admin"}, Operation: admissionv1.Update,
				}}
				req.OldObject.Raw = getRawObjWithFinalizers()
				req.Object.Raw = getRawObjWithFinalizers("gardener.cloud/reference-protection")

				resp := handler.Handle(ctx, req)
				Expect(resp.Allowed).To(BeFalse())
				Expect(resp.Result.Code).To(Equal(int32(403)))
				Expect(resp.Result.Message).To(Equal(`user "some-project-admin" is not allowed to modify protected finalizer(s) [gardener.cloud/reference-protection]`))
				Expect(reviewer.lastReview).NotTo(BeNil())
			})

			It("should deny a DELETE of an object with a protected finalizer", func() {
				req := admission.Request{AdmissionRequest: admissionv1.AdmissionRequest{
					UserInfo: authenticationv1.UserInfo{Username: "some-project-admin"}, Operation: admissionv1.Delete,
				}}
				req.OldObject.Raw = getRawObjWithFinalizers("gardener")

				resp := handler.Handle(ctx, req)
				Expect(resp.Allowed).To(BeFalse())
				Expect(resp.Result.Code).To(Equal(int32(403)))
				Expect(resp.Result.Message).To(Equal(`user "some-project-admin" is not allowed to modify protected finalizer(s) [gardener]`))
				Expect(reviewer.lastReview).NotTo(BeNil())
			})
		})

		Context("finalizer changes", func() {
			It("should allow reordering protected finalizers", func() {
				req := admission.Request{AdmissionRequest: admissionv1.AdmissionRequest{
					UserInfo: authenticationv1.UserInfo{Username: "some-project-admin"}, Operation: admissionv1.Update,
				}}
				req.OldObject.Raw = getRawObjWithFinalizers("gardener", "gardener.cloud/reference-protection")
				req.Object.Raw = getRawObjWithFinalizers("gardener.cloud/reference-protection", "gardener")

				resp := handler.Handle(ctx, req)
				Expect(resp.Allowed).To(BeTrue())
				Expect(resp.Result.Message).To(Equal("No protected finalizers were modified"))
				Expect(reviewer.lastReview).To(BeNil())
			})

			It("should deny a project administrator removing a protected finalizer while a non-protected finalizer is also present", func() {
				req := admission.Request{AdmissionRequest: admissionv1.AdmissionRequest{
					UserInfo: authenticationv1.UserInfo{Username: "some-project-admin"}, Operation: admissionv1.Update,
				}}
				req.OldObject.Raw = getRawObjWithFinalizers("gardener", "some-other-finalizer")
				req.Object.Raw = getRawObjWithFinalizers("some-other-finalizer")

				resp := handler.Handle(ctx, req)
				Expect(resp.Allowed).To(BeFalse())
				Expect(resp.Result.Code).To(Equal(int32(403)))
				Expect(resp.Result.Message).To(Equal(`user "some-project-admin" is not allowed to modify protected finalizer(s) [gardener]`))
			})

			It("should deny replacing one protected finalizer with another", func() {
				req := admission.Request{AdmissionRequest: admissionv1.AdmissionRequest{
					UserInfo: authenticationv1.UserInfo{Username: "some-project-admin"}, Operation: admissionv1.Update,
				}}
				req.OldObject.Raw = getRawObjWithFinalizers("gardener", "some-other-finalizer")
				req.Object.Raw = getRawObjWithFinalizers("gardener.cloud/reference-protection")

				resp := handler.Handle(ctx, req)
				Expect(resp.Allowed).To(BeFalse())
				Expect(resp.Result.Code).To(Equal(int32(403)))
				Expect(resp.Result.Message).To(Or(
					Equal(`user "some-project-admin" is not allowed to modify protected finalizer(s) [gardener gardener.cloud/reference-protection]`),
					Equal(`user "some-project-admin" is not allowed to modify protected finalizer(s) [gardener.cloud/reference-protection gardener]`),
				))
			})

		})

		Context("authorization", func() {
			It("should allow changes to unprotected finalizers without consulting the SubjectAccessReviewer", func() {
				req := admission.Request{AdmissionRequest: admissionv1.AdmissionRequest{
					UserInfo: authenticationv1.UserInfo{Username: "some-project-admin"}, Operation: admissionv1.Update,
				}}
				req.OldObject.Raw = getRawObjWithFinalizers("gardener", "old-unprotected")
				req.Object.Raw = getRawObjWithFinalizers("gardener", "new-unprotected")
				reviewer.createError = errors.New("authorization is unavailable")

				resp := handler.Handle(ctx, req)
				Expect(resp.Allowed).To(BeTrue())
				Expect(resp.Result.Message).To(Equal("No protected finalizers were modified"))
				Expect(reviewer.lastReview).To(BeNil())
			})

			It("should not trust a 'gardenlet username' without the appropriate group", func() {
				req := admission.Request{AdmissionRequest: admissionv1.AdmissionRequest{
					UserInfo: authenticationv1.UserInfo{Username: "gardener.cloud:system:seed:foo"}, Operation: admissionv1.Update,
				}}
				req.OldObject.Raw = getRawObjWithFinalizers("gardener")
				req.Object.Raw = getRawObjWithFinalizers()

				resp := handler.Handle(ctx, req)
				Expect(resp.Allowed).To(BeFalse())
				Expect(resp.Result.Code).To(Equal(int32(403)))
				Expect(reviewer.lastReview).NotTo(BeNil())
			})

			It("should not trust 'gardener.cloud:system:admins' group membership without consulting the SubjectAccessReviewer", func() {
				req := admission.Request{AdmissionRequest: admissionv1.AdmissionRequest{
					UserInfo: authenticationv1.UserInfo{Username: "some-user", Groups: []string{"gardener.cloud:system:admins"}}, Operation: admissionv1.Update,
				}}
				req.OldObject.Raw = getRawObjWithFinalizers("gardener")
				req.Object.Raw = getRawObjWithFinalizers()
				reviewer.denied = true

				resp := handler.Handle(ctx, req)
				Expect(resp.Allowed).To(BeFalse())
				Expect(resp.Result.Code).To(Equal(int32(403)))
				Expect(reviewer.lastReview).NotTo(BeNil())
			})

			It("should allow a system administrator and forward the full identity to a cluster-wide Secret list review", func() {
				req := admission.Request{AdmissionRequest: admissionv1.AdmissionRequest{
					UserInfo: authenticationv1.UserInfo{
						Username: "some-ops-user", UID: "user-uid",
						Groups: []string{"ops", "system:authenticated"},
						Extra:  map[string]authenticationv1.ExtraValue{"example.com/scopes": {"read", "write"}},
					},
					Operation: admissionv1.Update,
				}}
				req.OldObject.Raw = getRawObjWithFinalizers("gardener.cloud/reference-protection", "some-other-finalizer")
				req.Object.Raw = getRawObjWithFinalizers("some-other-finalizer")
				reviewer.allowed = true

				resp := handler.Handle(ctx, req)
				Expect(resp.Allowed).To(BeTrue())
				Expect(resp.Result.Message).To(Equal(`system administrator user "some-ops-user" is allowed to modify protected finalizer(s) [gardener.cloud/reference-protection]`))
				Expect(reviewer.lastReview).NotTo(BeNil())
				Expect(reviewer.lastReview.Spec).To(Equal(authorizationv1.SubjectAccessReviewSpec{
					User: "some-ops-user", UID: "user-uid", Groups: req.UserInfo.Groups,
					Extra:              map[string]authorizationv1.ExtraValue{"example.com/scopes": {"read", "write"}},
					ResourceAttributes: &authorizationv1.ResourceAttributes{Resource: "secrets", Verb: "list"},
				}))
			})

			It("should error if the SubjectAccessReview API call fails", func() {
				req := admission.Request{AdmissionRequest: admissionv1.AdmissionRequest{
					UserInfo: authenticationv1.UserInfo{Username: "some-project-admin"}, Operation: admissionv1.Update,
				}}
				req.OldObject.Raw = getRawObjWithFinalizers("gardener")
				req.Object.Raw = getRawObjWithFinalizers()
				reviewer.createError = errors.New("review failed")

				resp := handler.Handle(ctx, req)
				Expect(resp.Allowed).To(BeFalse())
				Expect(resp.Result.Code).To(Equal(int32(500)))
				Expect(resp.Result.Message).To(ContainSubstring("review failed"))
			})

			It("should deny if the SubjectAccessReview cannot establish permission", func() {
				req := admission.Request{AdmissionRequest: admissionv1.AdmissionRequest{
					UserInfo: authenticationv1.UserInfo{Username: "some-project-admin"}, Operation: admissionv1.Update,
				}}
				req.OldObject.Raw = getRawObjWithFinalizers("gardener")
				req.Object.Raw = getRawObjWithFinalizers()
				reviewer.denied = false
				reviewer.evaluationError = "review could not be evaluated"

				resp := handler.Handle(ctx, req)
				Expect(resp.Allowed).To(BeFalse())
				Expect(resp.Result.Code).To(Equal(int32(403)))
			})
		})

		Context("decoding errors", func() {
			It("should error if the old object cannot be decoded", func() {
				req := admission.Request{AdmissionRequest: admissionv1.AdmissionRequest{
					UserInfo: authenticationv1.UserInfo{Username: "some-project-admin"}, Operation: admissionv1.Update,
				}}
				req.OldObject.Raw = []byte("not-json")
				req.Object.Raw = getRawObjWithFinalizers()

				resp := handler.Handle(ctx, req)
				Expect(resp.Allowed).To(BeFalse())
				Expect(resp.Result.Code).To(Equal(int32(500)))
				Expect(reviewer.lastReview).To(BeNil())
			})

			It("should error if the new object cannot be decoded", func() {
				req := admission.Request{AdmissionRequest: admissionv1.AdmissionRequest{
					UserInfo: authenticationv1.UserInfo{Username: "some-project-admin"}, Operation: admissionv1.Update,
				}}
				req.OldObject.Raw = getRawObjWithFinalizers("gardener")
				req.Object.Raw = []byte("not-json")

				resp := handler.Handle(ctx, req)
				Expect(resp.Allowed).To(BeFalse())
				Expect(resp.Result.Code).To(Equal(int32(500)))
				Expect(reviewer.lastReview).To(BeNil())
			})
		})
	})
})

type fakeSubjectAccessReviewer struct {
	allowed         bool
	denied          bool
	reason          string
	evaluationError string
	createError     error
	lastReview      *authorizationv1.SubjectAccessReview
}

var _ clientauthorizationv1.SubjectAccessReviewInterface = (*fakeSubjectAccessReviewer)(nil)

func (f *fakeSubjectAccessReviewer) Create(_ context.Context, subjectAccessReview *authorizationv1.SubjectAccessReview, _ metav1.CreateOptions) (*authorizationv1.SubjectAccessReview, error) {
	f.lastReview = subjectAccessReview.DeepCopy()
	if f.createError != nil {
		return nil, f.createError
	}
	ret := subjectAccessReview.DeepCopy()
	ret.Status = authorizationv1.SubjectAccessReviewStatus{
		Allowed:         f.allowed,
		Denied:          f.denied && !f.allowed,
		Reason:          f.reason,
		EvaluationError: f.evaluationError,
	}
	return ret, nil
}

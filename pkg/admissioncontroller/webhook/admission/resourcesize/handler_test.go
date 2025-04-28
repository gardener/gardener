// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package resourcesize_test

import (
	"context"
	"io"
	"net/http"

	"github.com/go-logr/logr"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gbytes"
	"go.uber.org/zap/zapcore"
	admissionv1 "k8s.io/api/admission/v1"
	authenticationv1 "k8s.io/api/authentication/v1"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/serializer/json"
	"k8s.io/apiserver/pkg/authentication/serviceaccount"
	"sigs.k8s.io/controller-runtime/pkg/client"
	logzap "sigs.k8s.io/controller-runtime/pkg/log/zap"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"

	admissioncontrollerconfigv1alpha1 "github.com/gardener/gardener/pkg/admissioncontroller/apis/config/v1alpha1"
	. "github.com/gardener/gardener/pkg/admissioncontroller/webhook/admission/resourcesize"
	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	"github.com/gardener/gardener/pkg/logger"
)

var _ = Describe("handler", func() {
	var (
		ctx = context.TODO()
		log logr.Logger

		request admission.Request
		handler *Handler

		logBuffer   *gbytes.Buffer
		testEncoder runtime.Encoder

		projectsSizeLimit, _ = resource.ParseQuantity("0M")
		secretSizeLimit, _   = resource.ParseQuantity("1Mi")
		// size of shoot w/ namespace, name, w/o spec
		shootsv1beta1SizeLimit, _ = resource.ParseQuantity("342")
		// size of shoot w/ namespace, name, w/o spec -1 byte
		shootsv1alpha1SizeLimit, _ = resource.ParseQuantity("342")

		restrictedUserName                  = "restrictedUser"
		unrestrictedUserName                = "unrestrictedUser"
		restrictedGroupName                 = "restrictedGroup"
		unrestrictedGroupName               = "unrestrictedGroup"
		restrictedServiceAccountName        = "restrictedServiceAccount"
		unrestrictedServiceAccountName      = "unrestrictedServiceAccount"
		unrestrictedServiceAccountNamespace = "unrestricted"

		config = func() *admissioncontrollerconfigv1alpha1.ResourceAdmissionConfiguration {
			return &admissioncontrollerconfigv1alpha1.ResourceAdmissionConfiguration{
				UnrestrictedSubjects: []rbacv1.Subject{
					{
						Kind: rbacv1.GroupKind,
						Name: unrestrictedGroupName,
					},
					{
						Kind: rbacv1.UserKind,
						Name: unrestrictedUserName,
					},
					{
						Kind:      rbacv1.ServiceAccountKind,
						Name:      unrestrictedServiceAccountName,
						Namespace: unrestrictedServiceAccountNamespace,
					},
				},
				Limits: []admissioncontrollerconfigv1alpha1.ResourceLimit{
					{
						APIGroups:   []string{"*"},
						APIVersions: []string{"*"},
						Resources:   []string{"projects"},
						Size:        projectsSizeLimit,
					},
					{
						APIGroups:   []string{""},
						APIVersions: []string{"v1"},
						Resources:   []string{"secrets"},
						Size:        secretSizeLimit,
					},
					{
						APIGroups:   []string{"core.gardener.cloud"},
						APIVersions: []string{"v1beta1"},
						Resources:   []string{"shoots"},
						Size:        shootsv1beta1SizeLimit,
					},
					{
						APIGroups:   []string{"core.gardener.cloud"},
						APIVersions: []string{"v1alpha1"},
						Resources:   []string{"shoots"},
						Size:        shootsv1alpha1SizeLimit,
					},
				},
			}
		}

		empty = func() runtime.Object {
			return nil
		}

		shootv1beta1 = func() runtime.Object {
			return &gardencorev1beta1.Shoot{
				TypeMeta: metav1.TypeMeta{
					Kind:       "Shoot",
					APIVersion: gardencorev1beta1.SchemeGroupVersion.String(),
				},
				ObjectMeta: metav1.ObjectMeta{
					Namespace: "garden-my-project",
					Name:      "my-shoot",
				},
			}
		}

		project = func() runtime.Object {
			return &gardencorev1beta1.Project{
				TypeMeta: metav1.TypeMeta{
					Kind:       "Project",
					APIVersion: gardencorev1beta1.SchemeGroupVersion.String(),
				},
				ObjectMeta: metav1.ObjectMeta{
					Name: "my-project",
				},
			}
		}

		secret = func() runtime.Object {
			return &corev1.Secret{
				TypeMeta: metav1.TypeMeta{
					Kind:       "Secret",
					APIVersion: corev1.SchemeGroupVersion.String(),
				},
			}
		}

		configMap = func() runtime.Object {
			return &corev1.ConfigMap{
				TypeMeta: metav1.TypeMeta{
					Kind:       "ConfigMap",
					APIVersion: corev1.SchemeGroupVersion.String(),
				},
			}
		}

		unrestrictedUser = func() authenticationv1.UserInfo {
			return authenticationv1.UserInfo{
				Username: unrestrictedUserName,
				Groups:   []string{"test"},
			}
		}

		unrestrictedGroup = func() authenticationv1.UserInfo {
			return authenticationv1.UserInfo{
				Username: "restricted",
				Groups:   []string{unrestrictedGroupName},
			}
		}

		unrestrictedServiceAccount = func() authenticationv1.UserInfo {
			return authenticationv1.UserInfo{
				Username: serviceaccount.MakeUsername(unrestrictedServiceAccountNamespace, unrestrictedServiceAccountName),
				Groups:   serviceaccount.MakeGroupNames(unrestrictedGroupName),
			}
		}

		restrictedServiceAccount = func() authenticationv1.UserInfo {
			return authenticationv1.UserInfo{
				Username: serviceaccount.MakeUsername(unrestrictedServiceAccountNamespace, restrictedServiceAccountName),
				Groups:   serviceaccount.MakeGroupNames(restrictedGroupName),
			}
		}

		restrictedUser = func() authenticationv1.UserInfo {
			return authenticationv1.UserInfo{
				Username: restrictedUserName,
				Groups:   []string{restrictedGroupName},
			}
		}
	)

	BeforeEach(func() {
		logBuffer = gbytes.NewBuffer()
		log = logger.MustNewZapLogger(logger.DebugLevel, logger.FormatJSON, logzap.WriteTo(io.MultiWriter(GinkgoWriter, logBuffer)), logzap.Level(zapcore.Level(0)))

		handler = &Handler{Logger: log, Config: config()}

		testEncoder = &json.Serializer{}
		request = admission.Request{}
		request.Operation = admissionv1.Update
	})

	test := func(objFn func() runtime.Object, userFn func() authenticationv1.UserInfo, expectedAllowed bool) {
		if obj := objFn(); obj != nil {
			objData, err := runtime.Encode(testEncoder, obj)
			Expect(err).NotTo(HaveOccurred())
			request.Object.Raw = objData

			gvr, _ := meta.UnsafeGuessKindToResource(obj.GetObjectKind().GroupVersionKind())
			v1Gvr := metav1.GroupVersionResource{
				Group:    gvr.Group,
				Version:  gvr.Version,
				Resource: gvr.Resource,
			}

			request.Resource = v1Gvr
			request.RequestResource = &v1Gvr
			request.Object = runtime.RawExtension{Raw: objData}

			if o, ok := obj.(client.Object); ok {
				request.Name = o.GetName()
				request.Namespace = o.GetNamespace()
			}
		}

		request.UserInfo = userFn()
		response := handler.Handle(ctx, request)
		Expect(response).To(Not(BeNil()))
		Expect(response.Allowed).To(Equal(expectedAllowed))

		var expectedStatusCode int32 = http.StatusOK
		if !expectedAllowed {
			expectedStatusCode = http.StatusForbidden
		}
		Expect(response.Result.Code).To(Equal(expectedStatusCode))
	}

	Context("ignored requests", func() {
		It("empty resource", func() {
			test(empty, restrictedUser, true)
		})
	})

	It("should pass because size is in range for v1beta1 shoot", func() {
		test(shootv1beta1, restrictedUser, true)
	})

	It("should pass because size is in range for v1beta1 shoot without considering status", func() {
		largeShoot := func() runtime.Object {
			shootWithLargeStatus := &gardencorev1beta1.Shoot{
				TypeMeta: metav1.TypeMeta{
					Kind:       "Shoot",
					APIVersion: gardencorev1beta1.SchemeGroupVersion.String(),
				},
				ObjectMeta: metav1.ObjectMeta{
					Namespace: "garden-my-project",
					Name:      "my-shoot",
				},
				Status: gardencorev1beta1.ShootStatus{
					LastOperation: &gardencorev1beta1.LastOperation{
						Description: "This is a very long description that can happen if e.g. each node's error message is appended and a lot of nodes exist.",
					},
				},
			}
			objData, err := runtime.Encode(testEncoder, shootWithLargeStatus)
			Expect(err).NotTo(HaveOccurred())
			Expect(shootsv1beta1SizeLimit.CmpInt64(int64(len(objData)))).Should(Equal(-1))
			return shootWithLargeStatus
		}
		test(largeShoot, restrictedUser, true)
	})

	It("should pass because size is in range for v1beta1 shoot without considering managed fields", func() {
		largeShoot := func() runtime.Object {
			shootWithLargeStatus := &gardencorev1beta1.Shoot{
				TypeMeta: metav1.TypeMeta{
					Kind:       "Shoot",
					APIVersion: gardencorev1beta1.SchemeGroupVersion.String(),
				},
				ObjectMeta: metav1.ObjectMeta{
					Namespace:     "garden-my-project",
					Name:          "my-shoot",
					ManagedFields: []metav1.ManagedFieldsEntry{{Manager: "manager", Operation: "Update", FieldsV1: &metav1.FieldsV1{Raw: []byte("{\"testPayload\": \"This is some random large payload to mock some managed fields entries that are to be filtered out for size comparison\"}")}}},
				},
			}
			objData, err := runtime.Encode(testEncoder, shootWithLargeStatus)
			Expect(err).NotTo(HaveOccurred())
			Expect(shootsv1beta1SizeLimit.CmpInt64(int64(len(objData)))).Should(Equal(-1))
			return shootWithLargeStatus
		}

		test(largeShoot, restrictedUser, true)
	})

	It("should pass because of unrestricted user", func() {
		test(shootv1beta1, unrestrictedUser, true)
	})

	It("should pass because of unrestricted group", func() {
		test(shootv1beta1, unrestrictedGroup, true)
	})

	It("should pass because size is in range for secret", func() {
		test(secret, restrictedUser, true)
	})

	It("should pass because no limits configured for configMaps", func() {
		test(configMap, restrictedUser, true)
	})

	It("should fail because size is not in range for project", func() {
		test(project, restrictedUser, false)
	})

	It("should pass because of unrestricted user", func() {
		test(project, unrestrictedUser, true)
	})

	It("should pass because of unrestricted group", func() {
		test(project, unrestrictedGroup, true)
	})

	It("should pass because of unrestricted service account", func() {
		test(project, unrestrictedServiceAccount, true)
	})

	It("should fail because of restricted service account", func() {
		test(project, restrictedServiceAccount, false)
	})
})

// SPDX-FileCopyrightText: Contributors to the Gardener project
//
// SPDX-License-Identifier: Apache-2.0

package access_test

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/go-logr/logr"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/spf13/afero"
	authenticationv1 "k8s.io/api/authentication/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/rest"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/cache"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/client/interceptor"
	controllerconfig "sigs.k8s.io/controller-runtime/pkg/config"
	"sigs.k8s.io/controller-runtime/pkg/envtest"
	"sigs.k8s.io/controller-runtime/pkg/event"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	metricsserver "sigs.k8s.io/controller-runtime/pkg/metrics/server"

	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	resourcesv1alpha1 "github.com/gardener/gardener/pkg/apis/resources/v1alpha1"
	"github.com/gardener/gardener/pkg/logger"
	operatorclient "github.com/gardener/gardener/pkg/operator/client"
	"github.com/gardener/gardener/pkg/operator/controller/virtual/access"
	secretsutils "github.com/gardener/gardener/pkg/utils/secrets"
	secretsmanager "github.com/gardener/gardener/pkg/utils/secrets/manager"
	"github.com/gardener/gardener/pkg/utils/test"
)

func TestAccess(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Test Integration Operator Virtual Access Suite")
}

const testID = "garden-access-controller-test"

var (
	ctx = context.Background()
	log logr.Logger

	testEnv   *envtest.Environment
	mgrClient client.Client

	testRunID     string
	testNamespace *corev1.Namespace
	testSecret    *corev1.Secret
	tokenFilePath string

	fs      afero.Fs
	channel chan event.TypedGenericEvent[*rest.Config]
)

// issuedToken is the token that the stubbed virtual cluster's token-requestor hands out when the access controller
// bootstraps or renews the virtual garden token.
const issuedToken = "issued-by-token-requestor"

var _ = BeforeSuite(func() {
	logf.SetLogger(logger.MustNewZapLogger(logger.DebugLevel, logger.FormatJSON, zap.WriteTo(GinkgoWriter)))
	log = logf.Log.WithName(testID)

	By("Start test environment")
	testEnv = &envtest.Environment{
		ErrorIfCRDPathMissing: true,
	}

	restConfig, err := testEnv.Start()
	Expect(err).NotTo(HaveOccurred())
	Expect(restConfig).NotTo(BeNil())

	DeferCleanup(func() {
		By("Stop test environment")
		Expect(testEnv.Stop()).To(Succeed())
	})

	testClient, err := client.New(restConfig, client.Options{Scheme: operatorclient.RuntimeScheme})
	Expect(err).NotTo(HaveOccurred())

	By("Create test Namespaces")
	testNamespace = &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			GenerateName: "garden-",
		},
	}

	Expect(testClient.Create(ctx, testNamespace)).To(Succeed())
	log.Info("Created Namespace for test", "namespaceName", testNamespace.Name)

	testRunID = testNamespace.Name
	testSecret = &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      testNamespace.Name,
			Namespace: testNamespace.Name,
			Labels: map[string]string{
				testID:                                   testRunID,
				resourcesv1alpha1.ResourceManagerPurpose: resourcesv1alpha1.LabelPurposeTokenRequest,
			},
			Annotations: map[string]string{
				resourcesv1alpha1.ServiceAccountName:      "gardener-internal",
				resourcesv1alpha1.ServiceAccountNamespace: metav1.NamespaceSystem,
			},
		},
	}

	By("Setup manager")
	mgr, err := manager.New(restConfig, manager.Options{
		Scheme:  operatorclient.RuntimeScheme,
		Metrics: metricsserver.Options{BindAddress: "0"},
		Cache: cache.Options{
			ByObject: map[client.Object]cache.ByObject{
				&corev1.Secret{}: {
					Label: labels.SelectorFromSet(labels.Set{testID: testRunID}),
				},
			},
		},
		Controller: controllerconfig.Controller{
			SkipNameValidation: new(true),
		},
	})

	Expect(err).NotTo(HaveOccurred())
	mgrClient = mgr.GetClient()

	By("Create ca-client CA secret")
	caCert, err := (&secretsutils.CertificateSecretConfig{
		Name:       v1beta1constants.SecretNameCAClient,
		CommonName: "kubernetes-client",
		CertType:   secretsutils.CACert,
	}).GenerateCertificate()
	Expect(err).NotTo(HaveOccurred())

	caSecret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "ca-client-current",
			Namespace: testNamespace.Name,
			Labels: map[string]string{
				testID:                           testRunID,
				secretsmanager.LabelKeyName:      v1beta1constants.SecretNameCAClient,
				secretsmanager.LabelKeyManagedBy: secretsmanager.LabelValueSecretsManager,
			},
		},
		Data: map[string][]byte{
			secretsutils.DataKeyCertificateCA: caCert.CertificatePEM,
			secretsutils.DataKeyPrivateKeyCA:  caCert.PrivateKeyPEM,
		},
	}
	Expect(testClient.Create(ctx, caSecret)).To(Succeed())

	DeferCleanup(func() {
		Expect(testClient.Delete(ctx, caSecret)).To(Succeed())
	})

	By("Stub virtual cluster client")
	// The access controller runs the token-requestor against the virtual cluster to (re)populate the garden token.
	// Instead of standing up a second API server, stub the virtual client with a fake that issues a static token and
	// authenticates TokenReviews, mirroring the unit test and pkg/controller/tokenrequestor's own test.
	DeferCleanup(test.WithVar(&access.NewVirtualClient, func(_ *rest.Config) (client.Client, error) {
		return fake.NewClientBuilder().WithScheme(operatorclient.VirtualScheme).WithInterceptorFuncs(interceptor.Funcs{
			Create: func(ctx context.Context, c client.WithWatch, obj client.Object, opts ...client.CreateOption) error {
				if tokenReview, ok := obj.(*authenticationv1.TokenReview); ok {
					tokenReview.Status = authenticationv1.TokenReviewStatus{Authenticated: true}
					return nil
				}
				return c.Create(ctx, obj, opts...)
			},
			SubResourceCreate: func(ctx context.Context, c client.Client, _ string, obj client.Object, subResource client.Object, _ ...client.SubResourceCreateOption) error {
				tokenRequest, ok := subResource.(*authenticationv1.TokenRequest)
				if !ok {
					return apierrors.NewBadRequest(fmt.Sprintf("got invalid type %T, expected TokenRequest", subResource))
				}
				if _, ok := obj.(*corev1.ServiceAccount); !ok {
					return apierrors.NewNotFound(schema.GroupResource{}, "")
				}
				tokenRequest.Status.Token = issuedToken
				tokenRequest.Status.ExpirationTimestamp = metav1.Time{Time: time.Now().Add(time.Duration(ptr.Deref(tokenRequest.Spec.ExpirationSeconds, 3600)) * time.Second)}
				return c.Get(ctx, client.ObjectKeyFromObject(obj), obj)
			},
		}).Build(), nil
	}))

	fs = afero.NewMemMapFs()
	tokenFilePath = testRunID + ".test"

	DeferCleanup(test.WithVar(&access.CreateTemporaryFile, func(fs afero.Fs, _, _ string) (afero.File, error) {
		return fs.Create(tokenFilePath)
	}))

	channel = make(chan event.TypedGenericEvent[*rest.Config])

	By("Register controller")
	Expect((&access.Reconciler{
		FS:      fs,
		Channel: channel,
	}).AddToManager(mgr, testSecret.Name, testSecret.Name)).To(Succeed())

	By("Start manager")
	mgrContext, mgrCancel := context.WithCancel(ctx)

	go func() {
		defer GinkgoRecover()
		Expect(mgr.Start(mgrContext)).To(Succeed())
	}()

	DeferCleanup(func() {
		By("Stop manager")
		mgrCancel()
	})
})

// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package botanist_test

import (
	"context"

	"github.com/go-logr/logr"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/spf13/afero"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/rest"
	"sigs.k8s.io/controller-runtime/pkg/client"
	fakeclient "sigs.k8s.io/controller-runtime/pkg/client/fake"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	fakekubernetes "github.com/gardener/gardener/pkg/client/kubernetes/fake"
	. "github.com/gardener/gardener/pkg/gardenadm/botanist"
	"github.com/gardener/gardener/pkg/gardenlet/operation"
	botanistpkg "github.com/gardener/gardener/pkg/gardenlet/operation/botanist"
	"github.com/gardener/gardener/pkg/gardenlet/operation/shoot"
	fakedbus "github.com/gardener/gardener/pkg/nodeagent/dbus/fake"
	secretsmanager "github.com/gardener/gardener/pkg/utils/secrets/manager"
	fakesecretsmanager "github.com/gardener/gardener/pkg/utils/secrets/manager/fake"
)

var _ = Describe("Kubelet", func() {
	var (
		ctx context.Context

		namespace string
		hostName  string

		fakeSeedClient    client.Client
		fakeSecretManager secretsmanager.Interface
		fakeDBus          *fakedbus.DBus

		b *GardenadmBotanist
	)

	BeforeEach(func() {
		ctx = context.Background()
		namespace = "kube-system"
		hostName = "test"

		fakeSeedClient = fakeclient.NewClientBuilder().WithScheme(kubernetes.SeedScheme).Build()
		fakeSecretManager = fakesecretsmanager.New(fakeSeedClient, namespace)
		fakeDBus = fakedbus.New()

		b = &GardenadmBotanist{
			Botanist: &botanistpkg.Botanist{
				Operation: &operation.Operation{
					Logger:         logr.Discard(),
					Shoot:          &shoot.Shoot{},
					SecretsManager: fakeSecretManager,
					SeedClientSet: fakekubernetes.
						NewClientSetBuilder().
						WithClient(fakeSeedClient).
						WithRESTConfig(&rest.Config{}).
						Build(),
				},
			},
			FS:       afero.Afero{Fs: afero.NewMemMapFs()},
			DBus:     fakeDBus,
			HostName: hostName,
		}
		b.Shoot.SetInfo(&gardencorev1beta1.Shoot{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "foo",
				Namespace: namespace,
			},
		})

		Expect(fakeSeedClient.Create(ctx, &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: "ca", Namespace: namespace}})).To(Succeed())
	})

	Describe("#WriteKubeletBootstrapKubeconfig", func() {
		DescribeTable("should write the kubelet bootstrap kubeconfig",
			func(createToken bool) {
				Expect(b.FS.WriteFile("/var/lib/kubelet/kubeconfig-real", []byte{}, 0o600)).To(Succeed())
				Expect(b.FS.Exists("/var/lib/gardener-node-agent/tmp")).To(BeFalse())
				Expect(b.FS.Exists("/var/lib/gardener-node-agent/credentials")).To(BeFalse())
				Expect(b.FS.Exists("/var/lib/kubelet/kubeconfig-real")).To(BeTrue())
				Expect(b.FS.Exists("/var/lib/kubelet/kubeconfig-bootstrap")).To(BeFalse())
				if !createToken {
					Expect(b.FS.WriteFile("/var/lib/gardener-node-agent/credentials/bootstrap-token", []byte{}, 0o600)).To(Succeed())
				} else {
					Expect(b.FS.Exists("/var/lib/gardener-node-agent/credentials/bootstrap-token")).To(BeFalse())
				}
				Expect(b.WriteKubeletBootstrapKubeconfig(ctx)).To(Succeed())
				Expect(b.FS.Exists("/var/lib/gardener-node-agent/tmp")).To(BeTrue())
				Expect(b.FS.Exists("/var/lib/gardener-node-agent/credentials")).To(BeTrue())
				Expect(b.FS.Exists("/var/lib/gardener-node-agent/credentials/bootstrap-token")).To(BeTrue())
				Expect(b.FS.Exists("/var/lib/kubelet/kubeconfig-real")).To(BeFalse())
				Expect(b.FS.Exists("/var/lib/kubelet/kubeconfig-bootstrap")).To(BeTrue())
			},

			Entry("with creation of token file", true),
			Entry("with existing token file", false),
		)
	})
})

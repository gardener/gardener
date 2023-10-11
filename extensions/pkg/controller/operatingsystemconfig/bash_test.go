// Copyright 2023 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

package operatingsystemconfig_test

import (
	"context"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/pointer"
	"sigs.k8s.io/controller-runtime/pkg/client"
	fakeclient "sigs.k8s.io/controller-runtime/pkg/client/fake"

	. "github.com/gardener/gardener/extensions/pkg/controller/operatingsystemconfig"
	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
	. "github.com/gardener/gardener/pkg/utils/test/matchers"
)

var _ = Describe("Bash", func() {
	Describe("#FilesToDiskScript", func() {
		var (
			ctx        = context.Background()
			fakeClient client.Client
			namespace  = "namespace"
		)

		BeforeEach(func() {
			fakeClient = fakeclient.NewClientBuilder().Build()
		})

		It("should fail when a referenced secret cannot be read", func() {
			files := []extensionsv1alpha1.File{{
				Content: extensionsv1alpha1.FileContent{
					SecretRef: &extensionsv1alpha1.FileContentSecretRef{
						Name: "foo",
					},
				},
			}}

			script, err := FilesToDiskScript(ctx, fakeClient, namespace, files)
			Expect(err).To(BeNotFoundError())
			Expect(script).To(BeEmpty())
		})

		It("should generate the expected output", func() {
			Expect(fakeClient.Create(ctx, &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "foo",
					Namespace: namespace,
				},
				Data: map[string][]byte{"bar": []byte("bar-content")},
			})).To(Succeed())

			files := []extensionsv1alpha1.File{
				{
					Path: "/foo/bar.txt",
					Content: extensionsv1alpha1.FileContent{
						SecretRef: &extensionsv1alpha1.FileContentSecretRef{
							Name:    "foo",
							DataKey: "bar",
						},
					},
				},
				{
					Path: "/bar/baz",
					Content: extensionsv1alpha1.FileContent{
						Inline: &extensionsv1alpha1.FileContentInline{
							Encoding: "",
							Data:     "plain-text",
						},
					},
				},
				{
					Path: "/bar/baz",
					Content: extensionsv1alpha1.FileContent{
						Inline: &extensionsv1alpha1.FileContentInline{
							Encoding: "b64",
							Data:     "YmFzZTY0",
						},
					},
				},
				{
					Path: "/baz/foo",
					Content: extensionsv1alpha1.FileContent{
						Inline: &extensionsv1alpha1.FileContentInline{
							Encoding: "",
							Data:     "transmit-unencoded",
						},
						TransmitUnencoded: pointer.Bool(true),
					},
				},
			}

			script, err := FilesToDiskScript(ctx, fakeClient, namespace, files)
			Expect(err).NotTo(HaveOccurred())
			Expect(script).To(Equal(`
mkdir -p "/foo"

cat << EOF | base64 -d > "/foo/bar.txt"
YmFyLWNvbnRlbnQ=
EOF
mkdir -p "/bar"

cat << EOF | base64 -d > "/bar/baz"
cGxhaW4tdGV4dA==
EOF
mkdir -p "/bar"

cat << EOF | base64 -d > "/bar/baz"
YmFzZTY0
EOF
mkdir -p "/baz"

cat << EOF > "/baz/foo"
transmit-unencoded
EOF`))
		})
	})

	Describe("#UnitsToDiskScript", func() {
		It("should generate the expected output", func() {
			units := []extensionsv1alpha1.Unit{
				{
					Name: "unit1",
					DropIns: []extensionsv1alpha1.DropIn{
						{
							Name:    "dropin1",
							Content: "dropdrop",
						},
						{
							Name:    "dropin2",
							Content: "dropeldidrop",
						},
					},
				},
				{
					Name:    "unit2",
					Content: pointer.String("content2"),
				},
			}

			Expect(UnitsToDiskScript(units)).To(Equal(`
mkdir -p "/etc/systemd/system/unit1.d"

cat << EOF | base64 -d > "/etc/systemd/system/unit1.d/dropin1"
ZHJvcGRyb3A=
EOF

cat << EOF | base64 -d > "/etc/systemd/system/unit1.d/dropin2"
ZHJvcGVsZGlkcm9w
EOF

cat << EOF | base64 -d > "/etc/systemd/system/unit2"
Y29udGVudDI=
EOF`))
		})
	})
})

// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package operatingsystemconfig_test

import (
	"context"
	"io/fs"
	"os"
	"os/exec"
	"strings"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"
	fakeclient "sigs.k8s.io/controller-runtime/pkg/client/fake"

	. "github.com/gardener/gardener/extensions/pkg/controller/operatingsystemconfig"
	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
	. "github.com/gardener/gardener/pkg/utils/test/matchers"
)

var _ = Describe("Bash", func() {
	var (
		ctx        = context.Background()
		fakeClient client.Client
		namespace  = "namespace"

		folder1 = "/foo"
		file1   = folder1 + "/bar.txt"
	)

	BeforeEach(func() {
		fakeClient = fakeclient.NewClientBuilder().Build()
	})

	Describe("#FilesToDiskScript", func() {
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
			var (
				folder2 = "/bar"
				file2   = folder2 + "/baz"

				folder3 = "/baz"
				file3   = folder3 + "/foo"

				folder4 = "/foobar"
				file4   = folder4 + "/baz"
			)

			Expect(fakeClient.Create(ctx, &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "foo",
					Namespace: namespace,
				},
				Data: map[string][]byte{"bar": []byte("bar-content")},
			})).To(Succeed())

			files := []extensionsv1alpha1.File{
				{
					Path: file1,
					Content: extensionsv1alpha1.FileContent{
						SecretRef: &extensionsv1alpha1.FileContentSecretRef{
							Name:    "foo",
							DataKey: "bar",
						},
					},
				},
				{
					Path: file2,
					Content: extensionsv1alpha1.FileContent{
						Inline: &extensionsv1alpha1.FileContentInline{
							Encoding: "",
							Data:     "plain-text",
						},
					},
				},
				{
					Path: file3,
					Content: extensionsv1alpha1.FileContent{
						Inline: &extensionsv1alpha1.FileContentInline{
							Encoding: "b64",
							Data:     "YmFzZTY0",
						},
					},
				},
				{
					Path: file4,
					Content: extensionsv1alpha1.FileContent{
						Inline: &extensionsv1alpha1.FileContentInline{
							Encoding: "",
							Data:     "transmit-unencoded",
						},
						TransmitUnencoded: ptr.To(true),
					},
					Permissions: ptr.To(uint32(0777)),
				},
			}

			By("Ensure the function generated the expected bash script")
			script, err := FilesToDiskScript(ctx, fakeClient, namespace, files)
			Expect(err).NotTo(HaveOccurred())
			Expect(script).To(Equal(`
mkdir -p "` + folder1 + `"

cat << EOF | base64 -d > "` + file1 + `"
YmFyLWNvbnRlbnQ=
EOF
mkdir -p "` + folder2 + `"

cat << EOF | base64 -d > "` + file2 + `"
cGxhaW4tdGV4dA==
EOF
mkdir -p "` + folder3 + `"

cat << EOF | base64 -d > "` + file3 + `"
YmFzZTY0
EOF
mkdir -p "` + folder4 + `"

cat << EOF > "` + file4 + `"
transmit-unencoded
EOF
chmod "0777" "` + file4 + `"`))

			By("Ensure that the bash script can be executed and performs the desired operations")
			tempDir, err := os.MkdirTemp("", "tempdir")
			Expect(err).NotTo(HaveOccurred())
			defer os.RemoveAll(tempDir)

			script = strings.ReplaceAll(script, `"`+folder1, `"`+tempDir+folder1)
			script = strings.ReplaceAll(script, `"`+folder2, `"`+tempDir+folder2)
			script = strings.ReplaceAll(script, `"`+folder3, `"`+tempDir+folder3)
			script = strings.ReplaceAll(script, `"`+folder4, `"`+tempDir+folder4)

			runScriptAndCheckFiles(script,
				tempDir+file1,
				tempDir+file2,
				tempDir+file3,
				tempDir+file4,
			)
			checkFilePermissions(tempDir+file4, 0777)
		})
	})

	Describe("#UnitsToDiskScript", func() {
		It("should generate the expected output", func() {
			var (
				unit1        = "unit1"
				unit1DropIn1 = "dropin1"
				unit1DropIn2 = "dropin2"

				unit2 = "unit2"

				units = []extensionsv1alpha1.Unit{
					{
						Name: unit1,
						DropIns: []extensionsv1alpha1.DropIn{
							{
								Name:    unit1DropIn1,
								Content: "dropdrop",
							},
							{
								Name:    unit1DropIn2,
								Content: "dropeldidrop",
							},
						},
					},
					{
						Name:    unit2,
						Content: ptr.To("content2"),
					},
				}
			)

			By("Ensure the function generated the expected bash script")
			script := UnitsToDiskScript(units)
			Expect(script).To(Equal(`
mkdir -p "/etc/systemd/system/` + unit1 + `.d"

cat << EOF | base64 -d > "/etc/systemd/system/` + unit1 + `.d/` + unit1DropIn1 + `"
ZHJvcGRyb3A=
EOF

cat << EOF | base64 -d > "/etc/systemd/system/` + unit1 + `.d/` + unit1DropIn2 + `"
ZHJvcGVsZGlkcm9w
EOF

cat << EOF | base64 -d > "/etc/systemd/system/` + unit2 + `"
Y29udGVudDI=
EOF`))

			By("Ensure that the bash script can be executed and performs the desired operations")
			tempDir, err := os.MkdirTemp("", "tempdir")
			Expect(err).NotTo(HaveOccurred())
			defer os.RemoveAll(tempDir)

			script = strings.ReplaceAll(script, "/etc/systemd/system/", tempDir+"/etc/systemd/system/")
			script = strings.ReplaceAll(script, `"`+folder1, `"`+tempDir+folder1)

			runScriptAndCheckFiles(script,
				tempDir+"/etc/systemd/system/"+unit2,
				tempDir+"/etc/systemd/system/"+unit1+".d/"+unit1DropIn1,
				tempDir+"/etc/systemd/system/"+unit1+".d/"+unit1DropIn2,
			)
		})
	})

	Describe("#WrapProvisionOSCIntoOneshotScript", func() {
		It("should wrap the script into an oneshot script", func() {
			script := `echo "Hello, World!"
`
			Expect(WrapProvisionOSCIntoOneshotScript(script)).To(Equal(`if [ -f "/var/lib/osc/provision-osc-applied" ]; then
  echo "Provision OSC already applied, exiting..."
  exit 0
fi

echo "Hello, World!"


mkdir -p /var/lib/osc
touch /var/lib/osc/provision-osc-applied
`))
		})

		It("should wrap the script with shebang and comments into an oneshot script", func() {
			script := `#/bin/bash
# This is a hello world script
echo "Hello, World!"
`
			Expect(WrapProvisionOSCIntoOneshotScript(script)).To(Equal(`#/bin/bash
# This is a hello world script
if [ -f "/var/lib/osc/provision-osc-applied" ]; then
  echo "Provision OSC already applied, exiting..."
  exit 0
fi

echo "Hello, World!"


mkdir -p /var/lib/osc
touch /var/lib/osc/provision-osc-applied
`))
		})
	})
})

func runScriptAndCheckFiles(script string, filePaths ...string) {
	ExpectWithOffset(1, exec.Command("bash", "-c", script).Run()).To(Succeed())

	for _, filePath := range filePaths {
		fileInfo, err := os.Stat(filePath)
		ExpectWithOffset(1, err).NotTo(HaveOccurred(), "file at path "+filePath)
		ExpectWithOffset(1, fileInfo.Mode().IsRegular()).To(BeTrue(), "file at path "+filePath)
	}
}

func checkFilePermissions(path string, permissions int32) {
	fileInfo, err := os.Stat(path)
	ExpectWithOffset(1, err).NotTo(HaveOccurred(), "file at path "+path)
	ExpectWithOffset(1, fileInfo.Mode().Perm()).To(Equal(fs.FileMode(permissions)), "file at path "+path)
}

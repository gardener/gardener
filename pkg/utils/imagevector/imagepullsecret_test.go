// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package imagevector_test

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"

	dockerconfigfile "github.com/docker/cli/cli/config/configfile"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/gardener/gardener/pkg/utils/imagevector"
)

var _ = Describe("Image pull secret utilities", func() {
	marshalAuths := func(auths map[string]any) []byte {
		raw, err := json.Marshal(map[string]any{"auths": auths})
		Expect(err).NotTo(HaveOccurred())
		return raw
	}

	makeSecret := func(auths map[string]any) *corev1.Secret {
		return &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{Name: "pull-secret"},
			Type:       corev1.SecretTypeDockerConfigJson,
			Data:       map[string][]byte{corev1.DockerConfigJsonKey: marshalAuths(auths)},
		}
	}

	makeConfigFile := func(auths map[string]any) *dockerconfigfile.ConfigFile {
		cf := dockerconfigfile.New("")
		Expect(cf.LoadFromReader(bytes.NewReader(marshalAuths(auths)))).To(Succeed())
		return cf
	}

	authEntry := func(username, password string) map[string]any {
		return map[string]any{"username": username, "password": password}
	}

	authEntryEncoded := func(username, password string) map[string]any {
		encoded := base64.StdEncoding.EncodeToString(fmt.Appendf(nil, "%s:%s", username, password))
		return map[string]any{"auth": encoded}
	}

	Describe("ConfigFileFromImagePullSecret", func() {
		It("returns an error when the secret is missing the .dockerconfigjson key", func() {
			secret := &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{Name: "pull-secret"},
				Data:       map[string][]byte{},
			}
			_, err := imagevector.ConfigFileFromImagePullSecret(secret)
			Expect(err).To(MatchError(ContainSubstring("missing key")))
		})

		It("returns an error when the .dockerconfigjson value is not valid JSON", func() {
			secret := &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{Name: "pull-secret"},
				Data:       map[string][]byte{corev1.DockerConfigJsonKey: []byte("not-json")},
			}
			_, err := imagevector.ConfigFileFromImagePullSecret(secret)
			Expect(err).To(MatchError(ContainSubstring("failed to parse dockerconfigjson")))
		})

		It("returns a populated config file for a valid secret", func() {
			secret := makeSecret(map[string]any{"myregistry.io": authEntry("u", "p")})
			cf, err := imagevector.ConfigFileFromImagePullSecret(secret)
			Expect(err).NotTo(HaveOccurred())
			Expect(cf).NotTo(BeNil())
		})
	})

	Describe("CredentialsFromDockerConfigFile", func() {
		DescribeTable("resolves credentials for the given imageRef",
			func(auths map[string]any, imageRef, wantUser, wantPass string) {
				cf := makeConfigFile(auths)
				username, password, err := imagevector.CredentialsFromDockerConfigFile(cf, imageRef)
				Expect(err).NotTo(HaveOccurred())
				Expect(username).To(Equal(wantUser))
				Expect(password).To(Equal(wantPass))
			},
			Entry("no auth entry matches the image registry",
				map[string]any{"other.registry.io": authEntry("user", "pass")},
				"myregistry.io/img:tag", "", ""),
			Entry("matching auth entry with username and password",
				map[string]any{"myregistry.example.com/v1": authEntry("myuser", "mypassword")},
				"myregistry.example.com/v1/repo/img:tag", "myuser", "mypassword"),
			Entry("matching auth entry with encoded auth field",
				map[string]any{"myregistry.example.com/v1": authEntryEncoded("myuser", "mypassword")},
				"myregistry.example.com/v1/repo/img:tag", "myuser", "mypassword"),

			Entry("full image reference with path and tag",
				map[string]any{"myregistry.example.com": authEntry("u", "p")},
				"myregistry.example.com/path/to/image:tag", "u", "p"),
			Entry("registry with a port (e.g. localhost:5000)",
				map[string]any{"localhost:5000": authEntry("u", "p")},
				"localhost:5000/myimage:latest", "u", "p"),
			Entry("auth key with https:// scheme and /v2/ path",
				map[string]any{"https://myregistry.example.com/v2/": authEntry("u", "p")},
				"myregistry.example.com/repo/img:tag", "u", "p"),

			// Docker Hub fallback: images with no explicit registry resolve to index.docker.io.
			Entry("bare image name with tag (nginx:latest)",
				map[string]any{"https://index.docker.io/v1/": authEntry("hubuser", "hubpass")},
				"nginx:latest", "hubuser", "hubpass"),
			Entry("bare image name without tag (ubuntu)",
				map[string]any{"https://index.docker.io/v1/": authEntry("hubuser", "hubpass")},
				"ubuntu", "hubuser", "hubpass"),
			Entry("docker.io/library/nginx:latest",
				map[string]any{"https://index.docker.io/v1/": authEntry("hubuser", "hubpass")},
				"docker.io/library/nginx:latest", "hubuser", "hubpass"),
			Entry("index.docker.io/library/nginx:latest",
				map[string]any{"https://index.docker.io/v1/": authEntry("hubuser", "hubpass")},
				"index.docker.io/library/nginx:latest", "hubuser", "hubpass"),
			// First path component has no '.' or ':', so it is treated as Docker Hub.
			Entry("short-form user/image (library/ubuntu)",
				map[string]any{"https://index.docker.io/v1/": authEntry("hubuser", "hubpass")},
				"library/ubuntu", "hubuser", "hubpass"),
			Entry("other registry does not match Docker Hub fallback",
				map[string]any{"https://index.docker.io/v1/": authEntry("hubuser", "hubpass")},
				"myregistry.example.com/repo/img:tag", "", ""),
		)
	})

	Describe("CredentialsFromDockerConfigJSON", func() {
		It("propagates parse errors from the secret", func() {
			secret := &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{Name: "pull-secret"},
				Data:       map[string][]byte{corev1.DockerConfigJsonKey: []byte("not-json")},
			}
			_, _, err := imagevector.CredentialsFromDockerConfigJSON(secret, "myregistry.io/img:tag")
			Expect(err).To(MatchError(ContainSubstring("failed to parse dockerconfigjson")))
		})

		It("returns credentials for a valid secret and matching imageRef", func() {
			secret := makeSecret(map[string]any{"myregistry.io": authEntry("u", "p")})
			username, password, err := imagevector.CredentialsFromDockerConfigJSON(secret, "myregistry.io/img:tag")
			Expect(err).NotTo(HaveOccurred())
			Expect(username).To(Equal("u"))
			Expect(password).To(Equal("p"))
		})
	})
})

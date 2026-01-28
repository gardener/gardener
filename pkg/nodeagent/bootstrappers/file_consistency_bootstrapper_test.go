// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package bootstrappers_test

import (
	"context"
	"path/filepath"

	"github.com/go-logr/logr"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/spf13/afero"
	"gopkg.in/yaml.v3"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/tools/record"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	"github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
	"github.com/gardener/gardener/pkg/nodeagent/bootstrappers"
)

var _ = Describe("FileConsistencyBootstrapper (OSCChecker)", func() {
	var (
		ctx context.Context

		fakeFS       afero.Afero
		fakeRecorder *record.FakeRecorder
		fakeClient   client.Client

		checker *bootstrappers.OSCChecker

		nodeName           = "node-1"
		lastAppliedOSCPath = "/var/lib/gardener-node-agent/last-applied-osc.yaml"
	)

	BeforeEach(func() {
		ctx = context.Background()

		fakeFS = afero.Afero{Fs: afero.NewMemMapFs()}
		fakeRecorder = record.NewFakeRecorder(100)

		fakeClient = fake.NewClientBuilder().WithScheme(scheme.Scheme).Build()

		checker = &bootstrappers.OSCChecker{
			Log:      logr.Discard(),
			FS:       fakeFS,
			Client:   fakeClient,
			Recorder: fakeRecorder,
			NodeName: nodeName,
		}
	})

	Describe("Start", func() {

		Context("when the node does not exist yet", func() {
			It("should skip all checks and emit no events", func() {
				// No node created in fake client

				Expect(checker.Start(ctx)).To(Succeed())

				// No events should be emitted
				Consistently(fakeRecorder.Events).ShouldNot(Receive())
			})
		})

		Context("when the node exists but no OSC file exists", func() {
			It("should do nothing", func() {
				// Create node
				node := &corev1.Node{
					ObjectMeta: metav1.ObjectMeta{
						Name: nodeName,
					},
				}
				Expect(fakeClient.Create(ctx, node)).To(Succeed())

				// lastAppliedOSCPath NOT written

				Expect(checker.Start(ctx)).To(Succeed())

				// Still no events
				Consistently(fakeRecorder.Events).ShouldNot(Receive())
			})
		})

		Context("when the node and OSC exist with a missing file", func() {
			It("should perform checks and emit expected events", func() {
				// Create node
				node := &corev1.Node{
					ObjectMeta: metav1.ObjectMeta{
						Name: nodeName,
					},
				}
				Expect(fakeClient.Create(ctx, node)).To(Succeed())

				// Write minimal OSC with a missing file
				osc := v1alpha1.OperatingSystemConfig{
					Spec: v1alpha1.OperatingSystemConfigSpec{
						Files: []v1alpha1.File{
							{
								Path: "/etc/testfile",
								Content: v1alpha1.FileContent{
									Inline: &v1alpha1.FileContentInline{
										Data: "test",
									},
								},
							},
						},
					},
				}

				oscYAML, err := yaml.Marshal(&osc)
				Expect(err).NotTo(HaveOccurred())
				Expect(fakeFS.WriteFile(lastAppliedOSCPath, oscYAML, 0644)).To(Succeed())

				Expect(checker.Start(ctx)).To(Succeed())

				var event string
				Eventually(fakeRecorder.Events).Should(Receive(&event))

				// Sanity check event content
				Expect(event).To(ContainSubstring("FileMissing"))
				Expect(event).To(ContainSubstring("/etc/testfile"))
			})
		})

		Context("when node, OSC and all files exist with no mismatches", func() {
			It("should complete successfully without emitting any events", func() {
				node := &corev1.Node{
					ObjectMeta: metav1.ObjectMeta{
						Name: nodeName,
					},
				}
				Expect(fakeClient.Create(ctx, node)).To(Succeed())

				filePath := "/etc/config-file"
				fileContent := "expected-content"
				unitName := "test.service"
				unitPath := "/etc/systemd/system/" + unitName
				unitContent := "[Unit]\nDescription=Test"

				osc := v1alpha1.OperatingSystemConfig{
					Spec: v1alpha1.OperatingSystemConfigSpec{
						Files: []v1alpha1.File{
							{
								Path: filePath,
								Content: v1alpha1.FileContent{
									Inline: &v1alpha1.FileContentInline{
										Data: fileContent,
									},
								},
							},
						},
						Units: []v1alpha1.Unit{
							{
								Name:    unitName,
								Content: &unitContent,
							},
						},
					},
				}

				oscYAML, err := yaml.Marshal(&osc)
				Expect(err).NotTo(HaveOccurred())
				Expect(fakeFS.WriteFile(lastAppliedOSCPath, oscYAML, 0644)).To(Succeed())

				// Create matching file
				Expect(fakeFS.MkdirAll(filepath.Dir(filePath), 0755)).To(Succeed())
				Expect(fakeFS.WriteFile(filePath, []byte(fileContent), 0644)).To(Succeed())

				// Create matching unit file
				Expect(fakeFS.MkdirAll(filepath.Dir(unitPath), 0755)).To(Succeed())
				Expect(fakeFS.WriteFile(unitPath, []byte(unitContent), 0644)).To(Succeed())

				Expect(checker.Start(ctx)).To(Succeed())

				// No events should be emitted
				Consistently(fakeRecorder.Events).ShouldNot(Receive())
			})
		})

		Describe("File checks", func() {
			BeforeEach(func() {
				// Node exists
				node := &corev1.Node{
					ObjectMeta: metav1.ObjectMeta{Name: nodeName},
				}
				Expect(fakeClient.Create(ctx, node)).To(Succeed())
			})

			Context("when a file is missing", func() {
				It("should emit FileMissing event", func() {
					osc := v1alpha1.OperatingSystemConfig{
						Spec: v1alpha1.OperatingSystemConfigSpec{
							Files: []v1alpha1.File{
								{
									Path: "/etc/missing-file",
									Content: v1alpha1.FileContent{
										Inline: &v1alpha1.FileContentInline{
											Data: "content",
										},
									},
								},
							},
						},
					}

					raw, err := yaml.Marshal(&osc)
					Expect(err).NotTo(HaveOccurred())
					Expect(fakeFS.WriteFile(lastAppliedOSCPath, raw, 0644)).To(Succeed())

					Expect(checker.Start(ctx)).To(Succeed())

					var event string
					Eventually(fakeRecorder.Events).Should(Receive(&event))
					Expect(event).To(ContainSubstring("FileMissing"))
					Expect(event).To(ContainSubstring("/etc/missing-file"))
				})
			})

			Context("when a file content mismatches", func() {
				It("should emit FileMismatch event", func() {
					filePath := "/etc/config-file"

					osc := v1alpha1.OperatingSystemConfig{
						Spec: v1alpha1.OperatingSystemConfigSpec{
							Files: []v1alpha1.File{
								{
									Path: filePath,
									Content: v1alpha1.FileContent{
										Inline: &v1alpha1.FileContentInline{
											Data: "expected-content",
										},
									},
								},
							},
						},
					}

					raw, err := yaml.Marshal(&osc)
					Expect(err).NotTo(HaveOccurred())
					Expect(fakeFS.WriteFile(lastAppliedOSCPath, raw, 0644)).To(Succeed())

					Expect(fakeFS.MkdirAll(filepath.Dir(filePath), 0755)).To(Succeed())
					Expect(fakeFS.WriteFile(filePath, []byte("actual-different-content"), 0644)).To(Succeed())

					Expect(checker.Start(ctx)).To(Succeed())

					var event string
					Eventually(fakeRecorder.Events).Should(Receive(&event))
					Expect(event).To(ContainSubstring("FileMismatch"))
					Expect(event).To(ContainSubstring(filePath))
				})
			})

			Context("when a file matches", func() {
				It("should emit no event", func() {
					filePath := "/etc/matching-file"
					content := "matching-content"

					osc := v1alpha1.OperatingSystemConfig{
						Spec: v1alpha1.OperatingSystemConfigSpec{
							Files: []v1alpha1.File{
								{
									Path: filePath,
									Content: v1alpha1.FileContent{
										Inline: &v1alpha1.FileContentInline{
											Data: content,
										},
									},
								},
							},
						},
					}

					raw, err := yaml.Marshal(&osc)
					Expect(err).NotTo(HaveOccurred())
					Expect(fakeFS.WriteFile(lastAppliedOSCPath, raw, 0644)).To(Succeed())

					Expect(fakeFS.MkdirAll(filepath.Dir(filePath), 0755)).To(Succeed())
					Expect(fakeFS.WriteFile(filePath, []byte(content), 0644)).To(Succeed())

					Expect(checker.Start(ctx)).To(Succeed())

					Consistently(fakeRecorder.Events).ShouldNot(Receive())
				})
			})
		})

		Describe("Unit file checks", func() {
			var (
				unitName    = "test.service"
				unitPath    = "/etc/systemd/system/" + unitName
				unitContent = "UNIT_CONTENT"
			)

			BeforeEach(func() {
				node := &corev1.Node{
					ObjectMeta: metav1.ObjectMeta{Name: nodeName},
				}
				Expect(fakeClient.Create(ctx, node)).To(Succeed())
			})

			Context("when unit file is missing", func() {
				It("should emit UnitFileMissing event", func() {
					content := unitContent
					osc := v1alpha1.OperatingSystemConfig{
						Spec: v1alpha1.OperatingSystemConfigSpec{
							Units: []v1alpha1.Unit{
								{
									Name:    unitName,
									Content: &content,
								},
							},
						},
					}

					raw, err := yaml.Marshal(&osc)
					Expect(err).NotTo(HaveOccurred())
					Expect(fakeFS.WriteFile(lastAppliedOSCPath, raw, 0644)).To(Succeed())

					Expect(checker.Start(ctx)).To(Succeed())

					var event string
					Eventually(fakeRecorder.Events).Should(Receive(&event))
					Expect(event).To(ContainSubstring("UnitFileMissing"))
					Expect(event).To(ContainSubstring(unitName))
				})
			})

			Context("when unit content mismatches", func() {
				It("should emit UnitMismatch event", func() {
					content := unitContent
					osc := v1alpha1.OperatingSystemConfig{
						Spec: v1alpha1.OperatingSystemConfigSpec{
							Units: []v1alpha1.Unit{
								{
									Name:    unitName,
									Content: &content,
								},
							},
						},
					}

					raw, err := yaml.Marshal(&osc)
					Expect(err).NotTo(HaveOccurred())
					Expect(fakeFS.WriteFile(lastAppliedOSCPath, raw, 0644)).To(Succeed())

					Expect(fakeFS.MkdirAll(filepath.Dir(unitPath), 0755)).To(Succeed())
					Expect(fakeFS.WriteFile(unitPath, []byte("DIFFERENT_CONTENT"), 0644)).To(Succeed())

					Expect(checker.Start(ctx)).To(Succeed())

					var event string
					Eventually(fakeRecorder.Events).Should(Receive(&event))
					Expect(event).To(ContainSubstring("UnitMismatch"))
					Expect(event).To(ContainSubstring(unitName))
				})
			})

			Context("when unit content matches", func() {
				It("should emit no event", func() {
					content := unitContent
					osc := v1alpha1.OperatingSystemConfig{
						Spec: v1alpha1.OperatingSystemConfigSpec{
							Units: []v1alpha1.Unit{
								{
									Name:    unitName,
									Content: &content,
								},
							},
						},
					}

					raw, err := yaml.Marshal(&osc)
					Expect(err).NotTo(HaveOccurred())
					Expect(fakeFS.WriteFile(lastAppliedOSCPath, raw, 0644)).To(Succeed())

					Expect(fakeFS.MkdirAll(filepath.Dir(unitPath), 0755)).To(Succeed())
					Expect(fakeFS.WriteFile(unitPath, []byte(content), 0644)).To(Succeed())

					Expect(checker.Start(ctx)).To(Succeed())

					Consistently(fakeRecorder.Events).ShouldNot(Receive())
				})
			})
		})

		Describe("Drop-in checks", func() {
			var (
				unitName      = "test.service"
				unitPath      = "/etc/systemd/system/" + unitName
				dropInName    = "10-override.conf"
				dropInPath    = "/etc/systemd/system/test.service.d/10-override.conf"
				dropInContent = "DROP_IN_CONTENT"
			)

			BeforeEach(func() {
				node := &corev1.Node{
					ObjectMeta: metav1.ObjectMeta{Name: nodeName},
				}
				Expect(fakeClient.Create(ctx, node)).To(Succeed())
			})

			Context("when drop-in file is missing", func() {
				It("should emit DropInMissing event", func() {
					// Create unit file first (required for resolveDropInPath to work)
					Expect(fakeFS.MkdirAll(filepath.Dir(unitPath), 0755)).To(Succeed())
					Expect(fakeFS.WriteFile(unitPath, []byte("[Unit]"), 0644)).To(Succeed())

					osc := v1alpha1.OperatingSystemConfig{
						Spec: v1alpha1.OperatingSystemConfigSpec{
							Units: []v1alpha1.Unit{
								{
									Name: unitName,
									DropIns: []v1alpha1.DropIn{
										{
											Name:    dropInName,
											Content: dropInContent,
										},
									},
								},
							},
						},
					}

					raw, err := yaml.Marshal(&osc)
					Expect(err).NotTo(HaveOccurred())
					Expect(fakeFS.WriteFile(lastAppliedOSCPath, raw, 0644)).To(Succeed())

					Expect(checker.Start(ctx)).To(Succeed())

					var event string
					Eventually(fakeRecorder.Events).Should(Receive(&event))
					Expect(event).To(ContainSubstring("DropInMissing"))
					Expect(event).To(ContainSubstring(dropInName))
					Expect(event).To(ContainSubstring(unitName))
				})
			})

			Context("when drop-in content mismatches", func() {
				It("should emit DropInMismatch event", func() {
					Expect(fakeFS.MkdirAll(filepath.Dir(unitPath), 0755)).To(Succeed())
					Expect(fakeFS.WriteFile(unitPath, []byte("[Unit]"), 0644)).To(Succeed())

					osc := v1alpha1.OperatingSystemConfig{
						Spec: v1alpha1.OperatingSystemConfigSpec{
							Units: []v1alpha1.Unit{
								{
									Name: unitName,
									DropIns: []v1alpha1.DropIn{
										{
											Name:    dropInName,
											Content: dropInContent,
										},
									},
								},
							},
						},
					}

					raw, err := yaml.Marshal(&osc)
					Expect(err).NotTo(HaveOccurred())
					Expect(fakeFS.WriteFile(lastAppliedOSCPath, raw, 0644)).To(Succeed())

					Expect(fakeFS.MkdirAll(filepath.Dir(dropInPath), 0755)).To(Succeed())
					Expect(fakeFS.WriteFile(dropInPath, []byte("DIFFERENT_CONTENT"), 0644)).To(Succeed())

					Expect(checker.Start(ctx)).To(Succeed())

					var event string
					Eventually(fakeRecorder.Events).Should(Receive(&event))
					Expect(event).To(ContainSubstring("DropInMismatch"))
					Expect(event).To(ContainSubstring(dropInName))
					Expect(event).To(ContainSubstring(unitName))
				})
			})

			Context("when drop-in content matches", func() {
				It("should emit no event", func() {
					Expect(fakeFS.MkdirAll(filepath.Dir(unitPath), 0755)).To(Succeed())
					Expect(fakeFS.WriteFile(unitPath, []byte("[Unit]"), 0644)).To(Succeed())

					content := dropInContent
					osc := v1alpha1.OperatingSystemConfig{
						Spec: v1alpha1.OperatingSystemConfigSpec{
							Units: []v1alpha1.Unit{
								{
									Name: unitName,
									DropIns: []v1alpha1.DropIn{
										{
											Name:    dropInName,
											Content: content,
										},
									},
								},
							},
						},
					}

					raw, err := yaml.Marshal(&osc)
					Expect(err).NotTo(HaveOccurred())
					Expect(fakeFS.WriteFile(lastAppliedOSCPath, raw, 0644)).To(Succeed())

					Expect(fakeFS.MkdirAll(filepath.Dir(dropInPath), 0755)).To(Succeed())
					Expect(fakeFS.WriteFile(dropInPath, []byte(content), 0644)).To(Succeed())

					Expect(checker.Start(ctx)).To(Succeed())

					Consistently(fakeRecorder.Events).ShouldNot(Receive())
				})
			})
		})
	})
})

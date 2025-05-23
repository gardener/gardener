// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package project_test

import (
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/rest"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	"github.com/gardener/gardener/pkg/utils"
	kubernetesutils "github.com/gardener/gardener/pkg/utils/kubernetes"
	. "github.com/gardener/gardener/pkg/utils/test/matchers"
)

var _ = Describe("Project controller tests", func() {
	var (
		projectNamespaceKey client.ObjectKey

		project          *gardencorev1beta1.Project
		projectNamespace *corev1.Namespace
		shoot            *gardencorev1beta1.Shoot
	)

	BeforeEach(func() {
		projectName := "test-" + utils.ComputeSHA256Hex([]byte(testRunID + CurrentSpecReport().LeafNodeLocation.String()))[:5]

		project = &gardencorev1beta1.Project{
			ObjectMeta: metav1.ObjectMeta{
				Name:   projectName,
				Labels: map[string]string{testID: testRunID},
			},
			Spec: gardencorev1beta1.ProjectSpec{
				Namespace: ptr.To("garden-" + projectName),
			},
		}

		projectNamespace = nil
		projectNamespaceKey = client.ObjectKey{Name: *project.Spec.Namespace}

		shoot = &gardencorev1beta1.Shoot{
			ObjectMeta: metav1.ObjectMeta{
				GenerateName: "test-",
				Namespace:    projectNamespaceKey.Name,
				Labels:       map[string]string{testID: testRunID},
			},
			Spec: gardencorev1beta1.ShootSpec{
				SecretBindingName: ptr.To("mysecretbinding"),
				CloudProfileName:  ptr.To("cloudprofile1"),
				Region:            "europe-central-1",
				Provider: gardencorev1beta1.Provider{
					Type: "foo-provider",
					Workers: []gardencorev1beta1.Worker{
						{
							Name:    "cpu-worker",
							Minimum: 3,
							Maximum: 3,
							Machine: gardencorev1beta1.Machine{
								Type: "large",
							},
						},
					},
				},
				DNS: &gardencorev1beta1.DNS{
					Domain: ptr.To("some-domain.example.com"),
				},
				Kubernetes: gardencorev1beta1.Kubernetes{
					Version: "1.25.1",
				},
				Networking: &gardencorev1beta1.Networking{
					Type: ptr.To("foo-networking"),
				},
			},
		}
	})

	JustBeforeEach(func() {
		if projectNamespace != nil {
			By("Create project Namespace")
			Expect(testClient.Create(ctx, projectNamespace)).To(Succeed())
			log.Info("Created project namespace", "projectNamespace", projectNamespace)

			DeferCleanup(func() {
				By("Delete project namespace")
				Expect(testClient.Delete(ctx, projectNamespace)).To(Or(Succeed(), BeNotFoundError()))
			})
		} else {
			projectNamespace = &corev1.Namespace{}
		}

		By("Create Project")
		Expect(testClient.Create(ctx, project)).To(Succeed())
		log.Info("Created Project", "project", client.ObjectKeyFromObject(project))

		DeferCleanup(func() {
			By("Delete Project")
			Expect(client.IgnoreNotFound(testClient.Delete(ctx, project))).To(Succeed())

			By("Wait for Project to be gone")
			Eventually(func() error {
				return testClient.Get(ctx, client.ObjectKeyFromObject(project), project)
			}).Should(BeNotFoundError())
		})
	})

	triggerAndWaitForReconciliation := func(project *gardencorev1beta1.Project) {
		By("Trigger Project Reconciliation")
		patch := client.MergeFrom(project.DeepCopy())
		project.Spec.Description = ptr.To(time.Now().UTC().Format(time.RFC3339Nano))
		Expect(testClient.Patch(ctx, project, patch)).To(Succeed())

		By("Wait for Project to be reconciled")
		Eventually(func(g Gomega) {
			g.Expect(testClient.Get(ctx, client.ObjectKeyFromObject(project), project)).To(Succeed())
			g.Expect(project.Status.ObservedGeneration).To(Equal(project.Generation), "project controller should observe generation %d", project.Generation)
		}).Should(Succeed())
	}

	waitForProjectPhase := func(project *gardencorev1beta1.Project, phase gardencorev1beta1.ProjectPhase) {
		By("Wait for Project to be reconciled")
		Eventually(func(g Gomega) {
			g.Expect(testClient.Get(ctx, client.ObjectKeyFromObject(project), project)).To(Succeed())
			g.Expect(project.Status.ObservedGeneration).To(Equal(project.Generation), "project controller should observe generation %d", project.Generation)
			g.Expect(project.Status.Phase).To(Equal(phase), "project should transition to phase %s", phase)
		}).Should(Succeed())
	}

	It("should add the finalizer and release it on deletion", func() {
		waitForProjectPhase(project, gardencorev1beta1.ProjectReady)
		Expect(project.Finalizers).To(ConsistOf("gardener"))

		By("Delete Project")
		Expect(testClient.Delete(ctx, project)).To(Succeed())

		By("Wait for Project to be gone")
		Eventually(func() error {
			return testClient.Get(ctx, client.ObjectKeyFromObject(project), project)
		}).Should(BeNotFoundError())
	})

	It("should not release the project as long as it still contains shoots", func() {
		waitForProjectPhase(project, gardencorev1beta1.ProjectReady)

		By("Create Shoot")
		Expect(testClient.Create(ctx, shoot)).To(Succeed())
		log.Info("Created Shoot for test", "shoot", client.ObjectKeyFromObject(shoot))

		By("Wait until manager has observed Shoot creation")
		Eventually(func() error {
			return mgrClient.Get(ctx, client.ObjectKeyFromObject(shoot), shoot)
		}).Should(Succeed())

		DeferCleanup(func() {
			By("Cleanup Shoot")
			Expect(client.IgnoreNotFound(testClient.Delete(ctx, shoot))).To(Succeed())
			Eventually(func() error {
				return testClient.Get(ctx, client.ObjectKeyFromObject(shoot), shoot)
			}).Should(BeNotFoundError())
		})

		By("Delete Project")
		Expect(testClient.Delete(ctx, project)).To(Succeed())

		waitForProjectPhase(project, gardencorev1beta1.ProjectTerminating)

		By("Ensure Project is not released")
		Consistently(func(g Gomega) {
			g.Expect(testClient.Get(ctx, client.ObjectKeyFromObject(project), project)).To(Succeed())
		}).Should(Succeed())

		By("Delete Shoot")
		Expect(testClient.Delete(ctx, shoot)).To(Succeed())
		Eventually(func() error {
			return testClient.Get(ctx, client.ObjectKeyFromObject(shoot), shoot)
		}).Should(BeNotFoundError())

		By("Wait for Project to be gone")
		Eventually(func() error {
			return testClient.Get(ctx, client.ObjectKeyFromObject(project), project)
		}).Should(BeNotFoundError())
	})

	Describe("Project Namespace", func() {
		testNamespaceLifecycle := func(text string) {
			It(text, Offset(1), func() {
				waitForProjectPhase(project, gardencorev1beta1.ProjectReady)

				By("Wait for project namespace to be created")
				Eventually(func(g Gomega) {
					g.Expect(testClient.Get(ctx, projectNamespaceKey, projectNamespace)).To(Succeed())
					g.Expect(projectNamespace.OwnerReferences).To(ConsistOf(metav1.OwnerReference{
						APIVersion:         "core.gardener.cloud/v1beta1",
						Kind:               "Project",
						Name:               project.Name,
						UID:                project.UID,
						Controller:         ptr.To(true),
						BlockOwnerDeletion: ptr.To(true),
					}))
				}).Should(Succeed())

				By("Delete Project")
				Expect(testClient.Delete(ctx, project)).To(Succeed())

				By("Wait for project namespace to be gone")
				Eventually(func() error {
					return testClient.Get(ctx, projectNamespaceKey, projectNamespace)
				}).Should(BeNotFoundError())
			})
		}

		Context("namespace specified for creation", func() {
			testNamespaceLifecycle("should create and delete the specified namespace")

			It("should keep the namespace if it has the annotation", func() {
				waitForProjectPhase(project, gardencorev1beta1.ProjectReady)

				By("Wait for project namespace to be created")
				Eventually(func(g Gomega) {
					g.Expect(testClient.Get(ctx, projectNamespaceKey, projectNamespace)).To(Succeed())
				}).Should(Succeed())

				By("Annotate project namespace to be kept after Project deletion")
				patch := client.MergeFrom(projectNamespace.DeepCopy())
				metav1.SetMetaDataAnnotation(&projectNamespace.ObjectMeta, "namespace.gardener.cloud/keep-after-project-deletion", "true")
				Expect(testClient.Patch(ctx, projectNamespace, patch)).To(Succeed())

				DeferCleanup(func() {
					By("Delete project namespace")
					Expect(testClient.Delete(ctx, projectNamespace)).To(Or(Succeed(), BeNotFoundError()))
				})

				By("Wait until manager has observed annotation")
				Eventually(func(g Gomega) {
					g.Expect(mgrClient.Get(ctx, projectNamespaceKey, projectNamespace)).To(Succeed())
					g.Expect(projectNamespace.Annotations).To(HaveKey("namespace.gardener.cloud/keep-after-project-deletion"))
				}).Should(Succeed())

				By("Delete Project")
				Expect(testClient.Delete(ctx, project)).To(Succeed())

				By("Wait for Project to be gone")
				Eventually(func() error {
					return testClient.Get(ctx, client.ObjectKeyFromObject(project), project)
				}).Should(BeNotFoundError())

				By("Ensure project namespace is released but not deleted")
				Consistently(func(g Gomega) {
					g.Expect(testClient.Get(ctx, projectNamespaceKey, projectNamespace)).To(Succeed())
					g.Expect(projectNamespace.OwnerReferences).To(BeEmpty())
					g.Expect(projectNamespace.Labels).NotTo(Or(
						HaveKey("project.gardener.cloud/name"),
						HaveKey("gardener.cloud/role"),
					))
				}).Should(Succeed())
			})
		})

		Context("existing namespace specified for adoption", func() {
			BeforeEach(func() {
				projectNamespace = &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{
					Name: projectNamespaceKey.Name,
				}}
			})

			Context("namespace without proper project labels", func() {
				It("should fail to adopt existing namespace", func() {
					waitForProjectPhase(project, gardencorev1beta1.ProjectFailed)

					By("Delete Project")
					Expect(testClient.Delete(ctx, project)).To(Succeed())

					By("Wait for Project to be gone")
					Eventually(func() error {
						return testClient.Get(ctx, client.ObjectKeyFromObject(project), project)
					}).Should(BeNotFoundError())

					By("Ensure project namespace is not deleted")
					Consistently(func() error {
						return testClient.Get(ctx, projectNamespaceKey, projectNamespace)
					}).Should(Succeed())
				})
			})

			Context("namespace correctly labeled", func() {
				BeforeEach(func() {
					metav1.SetMetaDataLabel(&projectNamespace.ObjectMeta, v1beta1constants.GardenRole, v1beta1constants.GardenRoleProject)
					metav1.SetMetaDataLabel(&projectNamespace.ObjectMeta, v1beta1constants.ProjectName, project.Name)
				})

				It("should adopt existing namespace but not delete it", func() {
					waitForProjectPhase(project, gardencorev1beta1.ProjectReady)

					By("Delete Project")
					Expect(testClient.Delete(ctx, project)).To(Succeed())

					By("Wait for Project to be gone")
					Eventually(func() error {
						return testClient.Get(ctx, client.ObjectKeyFromObject(project), project)
					}).Should(BeNotFoundError())

					By("Ensure project namespace is released but not deleted")
					Consistently(func(g Gomega) {
						g.Expect(testClient.Get(ctx, projectNamespaceKey, projectNamespace)).To(Succeed())
						g.Expect(projectNamespace.OwnerReferences).To(BeEmpty())
						g.Expect(projectNamespace.Labels).NotTo(Or(
							HaveKey("project.gardener.cloud/name"),
							HaveKey("gardener.cloud/role"),
						))
					}).Should(Succeed())
				})
			})

			Context("namespace belongs to another project", func() {
				BeforeEach(func() {
					metav1.SetMetaDataLabel(&projectNamespace.ObjectMeta, v1beta1constants.GardenRole, v1beta1constants.GardenRoleProject)
					metav1.SetMetaDataLabel(&projectNamespace.ObjectMeta, v1beta1constants.ProjectName, "foo")
				})

				It("should fail to adopt existing namespace", func() {
					waitForProjectPhase(project, gardencorev1beta1.ProjectFailed)

					By("Delete Project")
					Expect(testClient.Delete(ctx, project)).To(Succeed())

					By("Wait for Project to be gone")
					Eventually(func() error {
						return testClient.Get(ctx, client.ObjectKeyFromObject(project), project)
					}).Should(BeNotFoundError())

					By("Ensure project namespace is not deleted")
					Consistently(func() error {
						return testClient.Get(ctx, projectNamespaceKey, projectNamespace)
					}).Should(Succeed())
				})
			})
		})

		Context("no namespace specified", func() {
			BeforeEach(func() {
				project.Spec.Namespace = nil
			})

			JustBeforeEach(func() {
				waitForProjectPhase(project, gardencorev1beta1.ProjectReady)
				projectNamespaceKey = client.ObjectKey{Name: *project.Spec.Namespace}
				log.Info("Project uses generated project namespace", "projectNamespace", projectNamespaceKey)
			})

			testNamespaceLifecycle("should create and delete a generated project namespace")
		})
	})

	Describe("Default ResourceQuota", func() {
		var resourceQuota *corev1.ResourceQuota

		BeforeEach(func() {
			resourceQuota = &corev1.ResourceQuota{ObjectMeta: metav1.ObjectMeta{
				Name:      "gardener",
				Namespace: projectNamespaceKey.Name,
			}}
		})

		JustBeforeEach(func() {
			waitForProjectPhase(project, gardencorev1beta1.ProjectReady)
		})

		waitForQuota := func(resourceQuota *corev1.ResourceQuota) {
			By("Wait for quota to be created")
			Eventually(func(g Gomega) {
				g.Expect(testClient.Get(ctx, client.ObjectKeyFromObject(resourceQuota), resourceQuota)).To(Succeed())
				g.Expect(resourceQuota.Spec).To(DeepEqual(defaultResourceQuota.Spec))
				g.Expect(resourceQuota.Labels).To(DeepEqual(defaultResourceQuota.Labels))
				g.Expect(resourceQuota.Annotations).To(DeepEqual(defaultResourceQuota.Annotations))
			}).Should(Succeed())
		}

		It("should maintain the configured default quota", func() {
			waitForQuota(resourceQuota)

			By("Modify quota metadata")
			patch := client.MergeFrom(resourceQuota.DeepCopy())
			metav1.SetMetaDataLabel(&resourceQuota.ObjectMeta, "bar", testRunID)
			metav1.SetMetaDataAnnotation(&resourceQuota.ObjectMeta, "bar", testRunID)
			Expect(testClient.Patch(ctx, resourceQuota, patch)).To(Succeed())

			expectedLabels := resourceQuota.DeepCopy().Labels
			expectedAnnotations := resourceQuota.DeepCopy().Annotations

			triggerAndWaitForReconciliation(project)

			By("Ensure quota metadata is not overwritten")
			Consistently(func(g Gomega) {
				g.Expect(testClient.Get(ctx, client.ObjectKeyFromObject(resourceQuota), resourceQuota)).To(Succeed())
				g.Expect(resourceQuota.Labels).To(DeepEqual(expectedLabels))
				g.Expect(resourceQuota.Annotations).To(DeepEqual(expectedAnnotations))
			}).Should(Succeed())
		})

		It("should not overwrite increased quota settings", func() {
			waitForQuota(resourceQuota)

			By("Increase quota")
			patch := client.MergeFrom(resourceQuota.DeepCopy())
			for resourceName, quantity := range resourceQuota.Spec.Hard {
				quantity.Add(resource.MustParse("1"))
				resourceQuota.Spec.Hard[resourceName] = quantity
			}
			Expect(testClient.Patch(ctx, resourceQuota, patch)).To(Succeed())
			increasedQuota := resourceQuota.DeepCopy()

			triggerAndWaitForReconciliation(project)

			By("Ensure increased quota is not overwritten")
			Consistently(func(g Gomega) {
				g.Expect(testClient.Get(ctx, client.ObjectKeyFromObject(resourceQuota), resourceQuota)).To(Succeed())
				g.Expect(resourceQuota.Spec).To(DeepEqual(increasedQuota.Spec))
			}).Should(Succeed())
		})

		It("should add new resources to existing quotas", func() {
			waitForQuota(resourceQuota)

			By("Add new resource to quota config")
			defaultResourceQuota.Spec.Hard["count/secrets"] = resource.MustParse("42")

			triggerAndWaitForReconciliation(project)

			By("Ensure new resource is added")
			expectedQuota := defaultResourceQuota.DeepCopy()
			Eventually(func(g Gomega) {
				g.Expect(testClient.Get(ctx, client.ObjectKeyFromObject(resourceQuota), resourceQuota)).To(Succeed())
				g.Expect(resourceQuota.Spec).To(DeepEqual(expectedQuota.Spec))
			}).Should(Succeed())
		})
	})

	Describe("Member RBAC", func() {
		var (
			testUserName   string
			testUserClient client.Client
		)

		BeforeEach(func() {
			testUserName = project.Name
			testUserConfig := rest.CopyConfig(restConfig)
			// envtest.Environment.AddUser doesn't work when running against an existing cluster
			// use impersonation instead to simulate different user
			testUserConfig.Impersonate = rest.ImpersonationConfig{
				UserName: testUserName,
			}

			var err error
			testUserClient, err = client.New(testUserConfig, client.Options{Scheme: kubernetes.GardenScheme})
			Expect(err).NotTo(HaveOccurred())
		})

		JustBeforeEach(func() {
			waitForProjectPhase(project, gardencorev1beta1.ProjectReady)
		})

		It("should allow admins to access the project namespace", func() {
			By("Ensure non-member doesn't have access to project")
			Consistently(func(g Gomega) {
				g.Expect(testUserClient.Get(ctx, projectNamespaceKey, &corev1.Namespace{})).To(BeForbiddenError())
			}).Should(Succeed())

			By("Add admin to project")
			patch := client.MergeFrom(project.DeepCopy())
			project.Spec.Members = append(project.Spec.Members, gardencorev1beta1.ProjectMember{
				Subject: rbacv1.Subject{
					APIGroup: rbacv1.GroupName,
					Kind:     rbacv1.UserKind,
					Name:     testUserName,
				},
				Role: "admin",
			})
			Expect(testClient.Patch(ctx, project, patch)).To(Succeed())

			By("Ensure new admin has access to project")
			Eventually(func(g Gomega) {
				g.Expect(testUserClient.Get(ctx, projectNamespaceKey, &corev1.Namespace{})).To(Succeed())
			}).Should(Succeed())
		})

		It("should recreate deleted well-known RoleBindings", func() {
			By("Delete RoleBindings")
			var roleBindings []client.Object
			for _, name := range []string{"gardener.cloud:system:project-member", "gardener.cloud:system:project-viewer", "gardener.cloud:system:project-serviceaccountmanager"} {
				roleBindings = append(roleBindings, &rbacv1.RoleBinding{ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: projectNamespaceKey.Name}})
			}
			Expect(kubernetesutils.DeleteObjects(ctx, testClient, roleBindings...)).To(Succeed())

			By("Ensure RoleBindings are recreated")
			Eventually(func(g Gomega) {
				for _, roleBinding := range roleBindings {
					g.Expect(testClient.Get(ctx, client.ObjectKeyFromObject(roleBinding), roleBinding)).To(Succeed(), "should recreate RoleBinding %s", roleBinding.GetName())
				}
			}).Should(Succeed())
		})

		It("should recreate deleted extension RoleBinding", func() {
			By("Add new member with extension role")
			patch := client.MergeFrom(project.DeepCopy())
			project.Spec.Members = append(project.Spec.Members, gardencorev1beta1.ProjectMember{
				Subject: rbacv1.Subject{
					APIGroup: rbacv1.GroupName,
					Kind:     rbacv1.UserKind,
					Name:     testUserName,
				},
				Role: "extension:test",
			})
			Expect(testClient.Patch(ctx, project, patch)).To(Succeed())

			By("Wait until RoleBinding is created")
			roleBinding := &rbacv1.RoleBinding{ObjectMeta: metav1.ObjectMeta{Name: "gardener.cloud:extension:project:" + project.Name + ":test", Namespace: projectNamespaceKey.Name}}
			Eventually(func(g Gomega) {
				g.Expect(testClient.Get(ctx, client.ObjectKeyFromObject(roleBinding), roleBinding)).To(Succeed())
			}).Should(Succeed())

			By("Delete RoleBinding")
			Expect(testClient.Delete(ctx, roleBinding)).To(Succeed())

			By("Ensure RoleBinding is recreated")
			Eventually(func(g Gomega) {
				g.Expect(testClient.Get(ctx, client.ObjectKeyFromObject(roleBinding), roleBinding)).To(Succeed(), "should recreate RoleBinding %s", roleBinding.GetName())
			}).Should(Succeed())
		})

		Describe("NamespacedCloudProfile access", func() {
			var (
				parentCloudProfile        *gardencorev1beta1.CloudProfile
				namespacedCloudProfile    *gardencorev1beta1.NamespacedCloudProfile
				namespacedCloudProfileKey client.ObjectKey

				futureExpirationDate *metav1.Time
			)

			BeforeEach(func() {
				parentCloudProfile = &gardencorev1beta1.CloudProfile{
					ObjectMeta: metav1.ObjectMeta{
						GenerateName: testID + "-cpfl-",
					},
					Spec: gardencorev1beta1.CloudProfileSpec{
						Kubernetes: gardencorev1beta1.KubernetesSettings{
							Versions: []gardencorev1beta1.ExpirableVersion{{Version: "1.26.0"}, {Version: "1.25.1"}},
						},
						MachineImages: []gardencorev1beta1.MachineImage{
							{
								Name: "some-OS",
								Versions: []gardencorev1beta1.MachineImageVersion{
									{
										ExpirableVersion: gardencorev1beta1.ExpirableVersion{Version: "1.1.1"},
										CRI: []gardencorev1beta1.CRI{
											{
												Name: gardencorev1beta1.CRINameContainerD,
											},
										},
									},
								},
							},
						},
						MachineTypes: []gardencorev1beta1.MachineType{{Name: "large"}},
						Regions:      []gardencorev1beta1.Region{{Name: "some-region"}},
						Type:         "provider-type",
						Limits: &gardencorev1beta1.Limits{
							MaxNodesTotal: ptr.To(int32(100)),
						},
					},
				}

				namespacedCloudProfile = &gardencorev1beta1.NamespacedCloudProfile{
					ObjectMeta: metav1.ObjectMeta{
						GenerateName: testID + "-nscpfl-",
						Namespace:    projectNamespaceKey.Name,
					},
					Spec: gardencorev1beta1.NamespacedCloudProfileSpec{
						Parent: gardencorev1beta1.CloudProfileReference{
							Kind: "CloudProfile",
						},
					},
				}

				futureExpirationDate = &metav1.Time{Time: time.Now().Add(48 * time.Hour)}
			})

			JustBeforeEach(func() {
				By("Create parent CloudProfile")
				Expect(testClient.Create(ctx, parentCloudProfile)).To(Succeed())
				DeferCleanup(func() {
					Expect(testClient.Delete(ctx, parentCloudProfile)).To(Succeed())
				})

				By("Create NamespacedCloudProfile")
				namespacedCloudProfile.Spec.Parent.Name = parentCloudProfile.Name
				// Create the NamespacedCloudProfile using Eventually, as it may take time for the client and cache to
				// have the parent CloudProfile available for reading.
				Eventually(func() error {
					return testClient.Create(ctx, namespacedCloudProfile)
				}).Should(Succeed())
				By("Wait until NamespacedCloudProfile is reconciled")
				Eventually(func(g Gomega) {
					g.Expect(testClient.Get(ctx, client.ObjectKeyFromObject(namespacedCloudProfile), namespacedCloudProfile)).To(Succeed())
					g.Expect(namespacedCloudProfile.Status.ObservedGeneration).To(Equal(namespacedCloudProfile.Generation))
				}).Should(Succeed())
				DeferCleanup(func() {
					Expect(testClient.Delete(ctx, namespacedCloudProfile)).To(Succeed())
				})
				namespacedCloudProfileKey = client.ObjectKeyFromObject(namespacedCloudProfile)
			})

			It("should not allow users not associated to a project to read its NamespacedCloudProfiles", func() {
				By("Ensure non-members don't have access to NamespacedCloudProfiles")
				Consistently(func(g Gomega) {
					g.Expect(testUserClient.Get(ctx, namespacedCloudProfileKey, &gardencorev1beta1.NamespacedCloudProfile{})).To(BeForbiddenError())
				}).Should(Succeed())
			})

			It("should allow project viewers to read a project's NamespacedCloudProfile", func() {
				By("Grant project-viewer respective permissions for NamespacedCloudProfiles")
				clusterRoleGardenerMember := &rbacv1.ClusterRole{
					ObjectMeta: metav1.ObjectMeta{
						Name:   "gardener.cloud:system:project-viewer",
						Labels: map[string]string{"gardener.cloud/role": "project-viewer"},
					},
					Rules: []rbacv1.PolicyRule{
						{
							APIGroups: []string{"core.gardener.cloud"},
							Resources: []string{"namespacedcloudprofiles"},
							Verbs:     []string{"get", "list", "watch"},
						},
					},
				}
				Expect(testClient.Create(ctx, clusterRoleGardenerMember)).To(Succeed())
				DeferCleanup(func() {
					Expect(testClient.Delete(ctx, clusterRoleGardenerMember)).To(Succeed())
				})

				By("Add viewer to project")
				patch := client.MergeFrom(project.DeepCopy())
				project.Spec.Members = append(project.Spec.Members, gardencorev1beta1.ProjectMember{
					Subject: rbacv1.Subject{
						APIGroup: rbacv1.GroupName,
						Kind:     rbacv1.UserKind,
						Name:     testUserName,
					},
					Role: "viewer",
				})
				Expect(testClient.Patch(ctx, project, patch)).To(Succeed())

				By("Ensure viewer has access to NamespacedCloudProfile")
				Eventually(func() error {
					return testUserClient.Get(ctx, namespacedCloudProfileKey, &gardencorev1beta1.NamespacedCloudProfile{})
				}).Should(Succeed())

				By("Ensure viewer cannot update NamespacedCloudProfile")
				namespacedCloudProfile.Spec.MachineTypes = []gardencorev1beta1.MachineType{
					{Name: "test-machine-type", CPU: resource.MustParse("2"), Memory: resource.MustParse("1G")},
				}
				Expect(testUserClient.Update(ctx, namespacedCloudProfile)).To(BeForbiddenError())
			})

			It("should allow project admins to read and modify a project's NamespacedCloudProfile for non-special fields", func() {
				By("Grant project-member respective permissions for NamespacedCloudProfiles")
				clusterRoleGardenerMember := &rbacv1.ClusterRole{
					ObjectMeta: metav1.ObjectMeta{
						Name:   "gardener.cloud:system:project-member",
						Labels: map[string]string{"gardener.cloud/role": "project-member"},
					},
					Rules: []rbacv1.PolicyRule{
						{
							APIGroups: []string{"core.gardener.cloud"},
							Resources: []string{"namespacedcloudprofiles"},
							Verbs:     []string{"get", "list", "watch", "create", "patch", "update", "delete"},
						},
					},
				}
				Expect(testClient.Create(ctx, clusterRoleGardenerMember)).To(Succeed())
				DeferCleanup(func() {
					Expect(testClient.Delete(ctx, clusterRoleGardenerMember)).To(Succeed())
				})

				By("Add admin to project")
				patch := client.MergeFrom(project.DeepCopy())
				project.Spec.Members = append(project.Spec.Members, gardencorev1beta1.ProjectMember{
					Subject: rbacv1.Subject{
						APIGroup: rbacv1.GroupName,
						Kind:     rbacv1.UserKind,
						Name:     testUserName,
					},
					Role: "admin",
				})
				Expect(testClient.Patch(ctx, project, patch)).To(Succeed())

				By("Ensure new admin has access to NamespacedCloudProfile")
				Eventually(func() error {
					return testUserClient.Get(ctx, namespacedCloudProfileKey, &gardencorev1beta1.NamespacedCloudProfile{})
				}).Should(Succeed())

				By("Ensure admin without proper role can update NamespacedCloudProfile.Spec.{MachineTypes,VolumeTypes}")
				namespacedCloudProfile.Spec.MachineTypes = []gardencorev1beta1.MachineType{
					{Name: "test-machine-type", CPU: resource.MustParse("2"), Memory: resource.MustParse("1G")},
				}
				namespacedCloudProfile.Spec.VolumeTypes = []gardencorev1beta1.VolumeType{
					{Name: "test-volume-type", Class: "standard"},
				}
				Expect(testUserClient.Update(ctx, namespacedCloudProfile)).To(Succeed())
				Eventually(func(g Gomega) {
					g.Expect(testClient.Get(ctx, client.ObjectKeyFromObject(namespacedCloudProfile), namespacedCloudProfile)).To(Succeed())
					g.Expect(namespacedCloudProfile.Status.ObservedGeneration).To(Equal(namespacedCloudProfile.Generation))
				}).Should(Succeed())

				By("Ensure admin without proper role cannot update NamespacedCloudProfile.Spec.{Kubernetes,MachineImages,ProviderConfig}")
				updatedNamespacedCloudProfile := namespacedCloudProfile.DeepCopy()
				updatedNamespacedCloudProfile.Spec.Kubernetes = &gardencorev1beta1.KubernetesSettings{
					Versions: []gardencorev1beta1.ExpirableVersion{{Version: "1.25.1", ExpirationDate: futureExpirationDate}},
				}
				Expect(testUserClient.Update(ctx, updatedNamespacedCloudProfile)).To(BeForbiddenError())
				updatedNamespacedCloudProfile = namespacedCloudProfile.DeepCopy()
				updatedNamespacedCloudProfile.Spec.MachineImages = []gardencorev1beta1.MachineImage{
					{Name: "some-OS", Versions: []gardencorev1beta1.MachineImageVersion{
						{ExpirableVersion: gardencorev1beta1.ExpirableVersion{Version: "1.1.1", ExpirationDate: futureExpirationDate}},
					}},
				}
				Expect(testUserClient.Update(ctx, updatedNamespacedCloudProfile)).To(BeForbiddenError())
				updatedNamespacedCloudProfile = namespacedCloudProfile.DeepCopy()
				updatedNamespacedCloudProfile.Spec.ProviderConfig = &runtime.RawExtension{Raw: []byte(`{"foo": "bar"}`)}
				Expect(testUserClient.Update(ctx, updatedNamespacedCloudProfile)).To(BeForbiddenError())

				By("Ensure admin without proper role cannot increase NamespacedCloudProfile.Spec.Limits above value from parent CloudProfile")
				updatedNamespacedCloudProfile = namespacedCloudProfile.DeepCopy()
				updatedNamespacedCloudProfile.Spec.Limits = &gardencorev1beta1.Limits{
					MaxNodesTotal: ptr.To(int32(200)),
				}
				Expect(testUserClient.Update(ctx, updatedNamespacedCloudProfile)).To(BeForbiddenError())
			})

			It("should allow gardener operators to modify a project's NamespacedCloudProfiles including special fields", func() {
				By("Grant user operator-like permissions to modify special fields in NamespacedCloudProfile")
				clusterRoleGardenerMember := &rbacv1.ClusterRole{
					ObjectMeta: metav1.ObjectMeta{
						Name:   "gardener.cloud:system:project-member",
						Labels: map[string]string{"gardener.cloud/role": "project-member"},
					},
					Rules: []rbacv1.PolicyRule{
						{
							APIGroups: []string{"core.gardener.cloud"},
							Resources: []string{"namespacedcloudprofiles"},
							Verbs:     []string{"get", "list", "watch", "create", "patch", "update", "delete"},
						},
						{
							APIGroups: []string{"core.gardener.cloud"},
							Resources: []string{"namespacedcloudprofiles"},
							Verbs: []string{
								"modify-spec-kubernetes",
								"modify-spec-machineimages",
								"modify-spec-providerconfig",
								"raise-spec-limits",
							},
						},
					},
				}
				Expect(testClient.Create(ctx, clusterRoleGardenerMember)).To(Succeed())
				DeferCleanup(func() {
					Expect(testClient.Delete(ctx, clusterRoleGardenerMember)).To(Succeed())
				})

				By("Add admin to project")
				patch := client.MergeFrom(project.DeepCopy())
				project.Spec.Members = append(project.Spec.Members, gardencorev1beta1.ProjectMember{
					Subject: rbacv1.Subject{
						APIGroup: rbacv1.GroupName,
						Kind:     rbacv1.UserKind,
						Name:     testUserName,
					},
					Role: "admin",
				})
				Expect(testClient.Patch(ctx, project, patch)).To(Succeed())

				By("Ensure admin with proper role can update NamespacedCloudProfile.Spec.{Kubernetes,MachineImages,ProviderConfig,Limits}")
				namespacedCloudProfile.Spec.Kubernetes = &gardencorev1beta1.KubernetesSettings{
					Versions: []gardencorev1beta1.ExpirableVersion{{Version: "1.25.1", ExpirationDate: futureExpirationDate}},
				}
				namespacedCloudProfile.Spec.MachineImages = []gardencorev1beta1.MachineImage{
					{Name: "some-OS", Versions: []gardencorev1beta1.MachineImageVersion{
						{ExpirableVersion: gardencorev1beta1.ExpirableVersion{Version: "1.1.1", ExpirationDate: futureExpirationDate}},
					}},
				}
				namespacedCloudProfile.Spec.ProviderConfig = &runtime.RawExtension{Raw: []byte(`{"foo": "bar"}`)}
				namespacedCloudProfile.Spec.Limits = &gardencorev1beta1.Limits{
					MaxNodesTotal: ptr.To(int32(200)),
				}
				Eventually(func() error {
					return testUserClient.Update(ctx, namespacedCloudProfile)
				}).Should(Succeed())
				Eventually(func(g Gomega) {
					g.Expect(testClient.Get(ctx, client.ObjectKeyFromObject(namespacedCloudProfile), namespacedCloudProfile)).To(Succeed())
					g.Expect(namespacedCloudProfile.Status.ObservedGeneration).To(Equal(namespacedCloudProfile.Generation))
				}).Should(Succeed())
			})
		})
	})
})

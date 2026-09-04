// SPDX-FileCopyrightText: Contributors to the Gardener project
//
// SPDX-License-Identifier: Apache-2.0

// Package test provides test helpers for the gardenadm discover subcommands.
// It is internal to the discover command tree and not intended for use outside of tests.
package test

import (
	autoscalingv1 "k8s.io/api/autoscaling/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	gardencorev1 "github.com/gardener/gardener/pkg/apis/core/v1"
	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
)

// Resources is the bundle of resources used by the discover subcommand tests.
type Resources struct {
	Namespace                      *corev1.Namespace
	Project                        *gardencorev1beta1.Project
	Secret                         *corev1.Secret
	SecretDNS                      *corev1.Secret
	SecretBinding                  *gardencorev1beta1.SecretBinding
	CloudProfile                   *gardencorev1beta1.CloudProfile
	ControllerDeploymentProvider   *gardencorev1.ControllerDeployment
	ControllerRegistrationProvider *gardencorev1beta1.ControllerRegistration
	ControllerDeploymentNetwork    *gardencorev1.ControllerDeployment
	ControllerRegistrationNetwork  *gardencorev1beta1.ControllerRegistration
	ControllerDeploymentDNS        *gardencorev1.ControllerDeployment
	ControllerRegistrationDNS      *gardencorev1beta1.ControllerRegistration

	// These are referenced by ControllerDeploymentProvider and reside in the garden namespace.
	ReferencedSecret            *corev1.Secret
	ReferencedConfigMap         *corev1.ConfigMap
	ReferencedOCIPullSecret     *corev1.Secret
	ReferencedOCICABundleSecret *corev1.Secret

	Shoot *gardencorev1beta1.Shoot
}

// NewResources constructs the bundle of resources used by both
// `gardenadm discover new` and `gardenadm discover existing` tests.
func NewResources() *Resources {
	var (
		namespaceName         = "garden-test-project"
		extensionTypeProvider = "test-extension-type-provider"
		extensionTypeNetwork  = "test-extension-type-network"
		extensionTypeDNS      = "test-extension-type-dns"
	)

	namespace := &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: namespaceName,
		},
	}
	project := &gardencorev1beta1.Project{
		ObjectMeta: metav1.ObjectMeta{
			Name: "test-project",
		},
		Spec: gardencorev1beta1.ProjectSpec{
			Namespace: &namespaceName,
		},
	}
	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-secret",
			Namespace: namespaceName,
		},
	}
	secretDNS := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-secret-dns",
			Namespace: namespaceName,
		},
	}
	secretBinding := &gardencorev1beta1.SecretBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-secret-binding",
			Namespace: namespaceName,
		},
		SecretRef: corev1.SecretReference{
			Name:      secret.Name,
			Namespace: secret.Namespace,
		},
	}
	cloudProfile := &gardencorev1beta1.CloudProfile{
		ObjectMeta: metav1.ObjectMeta{
			Name: "test-cloud-profile",
		},
	}
	referencedSecret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-referenced-secret",
			Namespace: "garden",
		},
	}
	referencedConfigMap := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-referenced-configmap",
			Namespace: "garden",
		},
	}
	referencedOCIPullSecret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-referenced-oci-pull-secret",
			Namespace: "garden",
		},
	}
	referencedOCICABundleSecret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-referenced-oci-ca-bundle-secret",
			Namespace: "garden",
		},
	}
	controllerDeploymentProvider := &gardencorev1.ControllerDeployment{
		ObjectMeta: metav1.ObjectMeta{
			Name: "test-controller-deployment-provider",
		},
		Helm: &gardencorev1.HelmControllerDeployment{
			OCIRepository: &gardencorev1.OCIRepository{
				Ref:               new("test-registry.example.com/charts/provider:v1.0.0"),
				PullSecretRef:     &corev1.LocalObjectReference{Name: referencedOCIPullSecret.Name},
				CABundleSecretRef: &corev1.LocalObjectReference{Name: referencedOCICABundleSecret.Name},
			},
		},
		Resources: []gardencorev1.NamedResourceReference{
			{
				Name: "config",
				ResourceRef: autoscalingv1.CrossVersionObjectReference{
					APIVersion: "v1",
					Kind:       "Secret",
					Name:       referencedSecret.Name,
				},
			},
			{
				Name: "log-level",
				ResourceRef: autoscalingv1.CrossVersionObjectReference{
					APIVersion: "v1",
					Kind:       "ConfigMap",
					Name:       referencedConfigMap.Name,
				},
			},
		},
	}
	controllerRegistrationProvider := &gardencorev1beta1.ControllerRegistration{
		ObjectMeta: metav1.ObjectMeta{
			Name: "test-controller-registration-provider",
		},
		Spec: gardencorev1beta1.ControllerRegistrationSpec{
			Resources: []gardencorev1beta1.ControllerResource{
				{Kind: "ControlPlane", Type: extensionTypeProvider},
				{Kind: "Infrastructure", Type: extensionTypeProvider},
				{Kind: "Worker", Type: extensionTypeProvider},
			},
			Deployment: &gardencorev1beta1.ControllerRegistrationDeployment{
				DeploymentRefs: []gardencorev1beta1.DeploymentRef{{Name: controllerDeploymentProvider.Name}},
			},
		},
	}
	controllerDeploymentNetwork := &gardencorev1.ControllerDeployment{
		ObjectMeta: metav1.ObjectMeta{
			Name: "test-controller-deployment-network",
		},
	}
	controllerRegistrationNetwork := &gardencorev1beta1.ControllerRegistration{
		ObjectMeta: metav1.ObjectMeta{
			Name: "test-controller-registration-network",
		},
		Spec: gardencorev1beta1.ControllerRegistrationSpec{
			Resources: []gardencorev1beta1.ControllerResource{
				{Kind: "Network", Type: extensionTypeNetwork},
			},
			Deployment: &gardencorev1beta1.ControllerRegistrationDeployment{
				DeploymentRefs: []gardencorev1beta1.DeploymentRef{{Name: controllerDeploymentNetwork.Name}},
			},
		},
	}
	controllerDeploymentDNS := &gardencorev1.ControllerDeployment{
		ObjectMeta: metav1.ObjectMeta{
			Name: "test-controller-deployment-dns",
		},
	}
	controllerRegistrationDNS := &gardencorev1beta1.ControllerRegistration{
		ObjectMeta: metav1.ObjectMeta{
			Name: "test-controller-registration-dns",
		},
		Spec: gardencorev1beta1.ControllerRegistrationSpec{
			Resources: []gardencorev1beta1.ControllerResource{
				{Kind: "DNSRecord", Type: extensionTypeDNS},
			},
			Deployment: &gardencorev1beta1.ControllerRegistrationDeployment{
				DeploymentRefs: []gardencorev1beta1.DeploymentRef{{Name: controllerDeploymentDNS.Name}},
			},
		},
	}

	shoot := &gardencorev1beta1.Shoot{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-shoot",
			Namespace: namespaceName,
		},
		Spec: gardencorev1beta1.ShootSpec{
			SecretBindingName: &secretBinding.Name,
			CloudProfile: &gardencorev1beta1.CloudProfileReference{
				Kind: "CloudProfile",
				Name: cloudProfile.Name,
			},
			Provider: gardencorev1beta1.Provider{
				Type:    extensionTypeProvider,
				Workers: []gardencorev1beta1.Worker{{}},
			},
			Networking: &gardencorev1beta1.Networking{
				Type: &extensionTypeNetwork,
			},
			DNS: &gardencorev1beta1.DNS{
				Providers: []gardencorev1beta1.DNSProvider{
					{
						Type:    new(extensionTypeDNS),
						Primary: new(true),
						CredentialsRef: &autoscalingv1.CrossVersionObjectReference{
							APIVersion: "v1",
							Kind:       "Secret",
							Name:       secretDNS.Name,
						},
					},
					{
						Type: new("unused"),
						CredentialsRef: &autoscalingv1.CrossVersionObjectReference{
							APIVersion: "v1",
							Kind:       "Secret",
							Name:       "dns-credentials-unused",
						},
					},
				},
			},
		},
	}

	return &Resources{
		Namespace:                      namespace,
		Project:                        project,
		Secret:                         secret,
		SecretDNS:                      secretDNS,
		SecretBinding:                  secretBinding,
		CloudProfile:                   cloudProfile,
		ControllerDeploymentProvider:   controllerDeploymentProvider,
		ControllerRegistrationProvider: controllerRegistrationProvider,
		ControllerDeploymentNetwork:    controllerDeploymentNetwork,
		ControllerRegistrationNetwork:  controllerRegistrationNetwork,
		ControllerDeploymentDNS:        controllerDeploymentDNS,
		ControllerRegistrationDNS:      controllerRegistrationDNS,
		ReferencedSecret:               referencedSecret,
		ReferencedConfigMap:            referencedConfigMap,
		ReferencedOCIPullSecret:        referencedOCIPullSecret,
		ReferencedOCICABundleSecret:    referencedOCICABundleSecret,
		Shoot:                          shoot,
	}
}

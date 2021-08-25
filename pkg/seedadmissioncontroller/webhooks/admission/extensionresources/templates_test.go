// Copyright (c) 2021 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

package extensionresources_test

import (
	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"

	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var (
	apiversion  = "extensions.gardener.cloud/v1alpha1"
	defaultSpec = extensionsv1alpha1.DefaultSpec{Type: "gcp"}
	objectMeta  = metav1.ObjectMeta{Name: "entity-external", Namespace: "prjswebhooks"}
	secretRef   = corev1.SecretReference{Name: "secret-external", Namespace: "prjswebhooks"}

	backupBucket = &extensionsv1alpha1.BackupBucket{
		TypeMeta: metav1.TypeMeta{
			Kind:       extensionsv1alpha1.BackupBucketResource,
			APIVersion: apiversion,
		},
		ObjectMeta: metav1.ObjectMeta{Name: "backupbucket-external"},
		Spec: extensionsv1alpha1.BackupBucketSpec{
			DefaultSpec: defaultSpec,
			Region:      "europe-west-1",
			SecretRef:   secretRef,
		},
	}

	backupEntry = &extensionsv1alpha1.BackupEntry{
		TypeMeta: metav1.TypeMeta{
			Kind:       extensionsv1alpha1.BackupEntryResource,
			APIVersion: apiversion,
		},
		ObjectMeta: metav1.ObjectMeta{Name: "backupentry-external"},
		Spec: extensionsv1alpha1.BackupEntrySpec{
			DefaultSpec: defaultSpec,
			Region:      "europe-west-1",
			BucketName:  "cloud--gcp--fg2d6",
			SecretRef:   secretRef,
		},
	}

	bastion = &extensionsv1alpha1.Bastion{
		TypeMeta: metav1.TypeMeta{
			Kind:       extensionsv1alpha1.BastionResource,
			APIVersion: apiversion,
		},
		ObjectMeta: objectMeta,
		Spec: extensionsv1alpha1.BastionSpec{
			DefaultSpec: defaultSpec,
			UserData:    []byte("data"),
			Ingress: []extensionsv1alpha1.BastionIngressPolicy{
				{
					IPBlock: networkingv1.IPBlock{
						CIDR: "1.2.3.4/32",
					},
				},
			},
		},
	}

	controlPlane = &extensionsv1alpha1.ControlPlane{
		TypeMeta: metav1.TypeMeta{
			Kind:       extensionsv1alpha1.ControlPlaneResource,
			APIVersion: apiversion,
		},
		ObjectMeta: objectMeta,
		Spec: extensionsv1alpha1.ControlPlaneSpec{
			DefaultSpec: defaultSpec,
			SecretRef:   secretRef,
			Region:      "europe-west-1",
		},
	}

	dnsrecord = &extensionsv1alpha1.DNSRecord{
		TypeMeta: metav1.TypeMeta{
			Kind:       extensionsv1alpha1.DNSRecordResource,
			APIVersion: apiversion,
		},
		ObjectMeta: objectMeta,
		Spec: extensionsv1alpha1.DNSRecordSpec{
			DefaultSpec: defaultSpec,
			SecretRef:   secretRef,
			Name:        "api.gcp.foobar.shoot.example.com",
			RecordType:  "A",
			Values:      []string{"1.2.3.4"},
		},
	}

	extension = &extensionsv1alpha1.Extension{
		TypeMeta: metav1.TypeMeta{
			Kind:       extensionsv1alpha1.ExtensionResource,
			APIVersion: apiversion,
		},
		ObjectMeta: objectMeta,
		Spec: extensionsv1alpha1.ExtensionSpec{
			DefaultSpec: defaultSpec,
		},
	}

	infrastructure = &extensionsv1alpha1.Infrastructure{
		TypeMeta: metav1.TypeMeta{
			Kind:       extensionsv1alpha1.InfrastructureResource,
			APIVersion: apiversion,
		},
		ObjectMeta: objectMeta,
		Spec: extensionsv1alpha1.InfrastructureSpec{
			DefaultSpec: defaultSpec,
			SecretRef:   secretRef,
			Region:      "europe-west-1",
		},
	}

	network = &extensionsv1alpha1.Network{
		TypeMeta: metav1.TypeMeta{
			Kind:       extensionsv1alpha1.NetworkResource,
			APIVersion: apiversion,
		},
		ObjectMeta: objectMeta,
		Spec: extensionsv1alpha1.NetworkSpec{
			DefaultSpec: defaultSpec,
			PodCIDR:     "100.96.0.0/11",
			ServiceCIDR: "100.64.0.0/13",
		},
	}

	operatingsysconfig = &extensionsv1alpha1.OperatingSystemConfig{
		TypeMeta: metav1.TypeMeta{
			Kind:       extensionsv1alpha1.OperatingSystemConfigResource,
			APIVersion: apiversion,
		},
		ObjectMeta: objectMeta,
		Spec: extensionsv1alpha1.OperatingSystemConfigSpec{
			Purpose:     extensionsv1alpha1.OperatingSystemConfigPurposeProvision,
			DefaultSpec: defaultSpec,
		},
	}

	worker = &extensionsv1alpha1.Worker{
		TypeMeta: metav1.TypeMeta{
			Kind:       extensionsv1alpha1.WorkerResource,
			APIVersion: apiversion,
		},
		ObjectMeta: objectMeta,
		Spec: extensionsv1alpha1.WorkerSpec{
			DefaultSpec: defaultSpec,
			Region:      "europe-west-1",
			SecretRef:   secretRef,
		},
	}
)

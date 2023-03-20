// Copyright 2019 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

package genericactuator_test

import (
	"context"

	"github.com/go-logr/logr"
	"github.com/golang/mock/gomock"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client"
	fakeclient "sigs.k8s.io/controller-runtime/pkg/client/fake"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/runtime/inject"

	"github.com/gardener/gardener/extensions/pkg/controller/backupentry"
	"github.com/gardener/gardener/extensions/pkg/controller/backupentry/genericactuator"
	extensionsmockgenericactuator "github.com/gardener/gardener/extensions/pkg/controller/backupentry/genericactuator/mock"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
)

const (
	providerSecretName      = "backupprovider"
	providerSecretNamespace = "garden"
	shootTechnicalID        = "shoot--foo--bar"
	shootUID                = "asd234-asd-34"
	bucketName              = "test-bucket"
)

var _ = Describe("Actuator", func() {
	var (
		ctrl *gomock.Controller

		be = &extensionsv1alpha1.BackupEntry{
			ObjectMeta: metav1.ObjectMeta{
				Name: shootTechnicalID + "--" + shootUID,
			},
			Spec: extensionsv1alpha1.BackupEntrySpec{
				BucketName: bucketName,
				SecretRef: corev1.SecretReference{
					Name:      providerSecretName,
					Namespace: providerSecretNamespace,
				},
			},
		}

		backupProviderSecretData = map[string][]byte{
			"foo":        []byte("bar"),
			"bucketName": []byte(bucketName),
		}

		beSecret = &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      providerSecretName,
				Namespace: providerSecretNamespace,
			},
			Data: map[string][]byte{
				"foo": []byte("bar"),
			},
		}

		etcdBackupSecretData = map[string][]byte{
			"bucketName": []byte(bucketName),
			"foo":        []byte("bar"),
		}

		etcdBackupSecretKey = client.ObjectKey{Namespace: shootTechnicalID, Name: v1beta1constants.BackupSecretName}
		etcdBackupSecret    = &corev1.Secret{
			TypeMeta: metav1.TypeMeta{Kind: "Secret", APIVersion: "v1"},
			ObjectMeta: metav1.ObjectMeta{
				Name:            v1beta1constants.BackupSecretName,
				Namespace:       shootTechnicalID,
				ResourceVersion: "1",
			},
			Data: etcdBackupSecretData,
		}

		seedNamespace = &corev1.Namespace{
			ObjectMeta: metav1.ObjectMeta{
				Name: shootTechnicalID,
			},
		}

		log = logf.Log.WithName("test")
	)

	BeforeEach(func() {
		ctrl = gomock.NewController(GinkgoT())
	})

	AfterEach(func() {
		ctrl.Finish()
	})

	Describe("#Reconcile", func() {
		var (
			client client.Client
			a      backupentry.Actuator
		)

		Context("seed namespace exist", func() {

			BeforeEach(func() {
				// Create fake client
				client = fakeclient.NewClientBuilder().WithScheme(scheme.Scheme).
					WithObjects(seedNamespace, beSecret).Build()
			})

			It("should create secrets", func() {
				// Create mock values provider
				backupEntryDelegate := extensionsmockgenericactuator.NewMockBackupEntryDelegate(ctrl)
				backupEntryDelegate.EXPECT().GetETCDSecretData(context.TODO(), gomock.AssignableToTypeOf(logr.Logger{}), be, backupProviderSecretData).Return(etcdBackupSecretData, nil)

				// Create actuator
				a = genericactuator.NewActuator(backupEntryDelegate)
				err := a.(inject.Client).InjectClient(client)
				Expect(err).NotTo(HaveOccurred())

				// Call Reconcile method and check the result
				err = a.Reconcile(context.TODO(), log, be)
				Expect(err).NotTo(HaveOccurred())

				deployedSecret := &corev1.Secret{}
				err = client.Get(context.TODO(), etcdBackupSecretKey, deployedSecret)
				Expect(err).NotTo(HaveOccurred())
				Expect(deployedSecret).To(Equal(etcdBackupSecret))
			})
		})

		Context("seed namespace does not exist", func() {
			BeforeEach(func() {
				// Create fake client
				client = fakeclient.NewClientBuilder().WithScheme(scheme.Scheme).
					WithObjects(beSecret).Build()
			})

			It("should not create secrets", func() {
				// Create mock values provider
				backupEntryDelegate := extensionsmockgenericactuator.NewMockBackupEntryDelegate(ctrl)

				// Create actuator
				a = genericactuator.NewActuator(backupEntryDelegate)
				err := a.(inject.Client).InjectClient(client)
				Expect(err).NotTo(HaveOccurred())

				// Call Reconcile method and check the result
				err = a.Reconcile(context.TODO(), log, be)
				Expect(err).NotTo(HaveOccurred())

				deployedSecret := &corev1.Secret{}
				err = client.Get(context.TODO(), etcdBackupSecretKey, deployedSecret)
				Expect(err).To(HaveOccurred())
			})
		})
	})
})

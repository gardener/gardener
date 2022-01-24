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

package controller

import (
	"context"

	"github.com/gardener/gardener/landscaper/pkg/controlplane/apis/imports"
	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	"github.com/gardener/gardener/pkg/client/kubernetes/fake"
	"github.com/gardener/gardener/pkg/logger"
	mockclient "github.com/gardener/gardener/pkg/mock/controller-runtime/client"
	kutil "github.com/gardener/gardener/pkg/utils/kubernetes"
	"github.com/golang/mock/gomock"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/sirupsen/logrus"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	apiregistrationv1 "k8s.io/kube-aggregator/pkg/apis/apiregistration/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var _ = Describe("#VerifyControlplane", func() {
	var testOperation operation

	// mocking
	var (
		ctx               = context.TODO()
		ctrl              *gomock.Controller
		mockGardenClient  *mockclient.MockClient
		mockRuntimeClient *mockclient.MockClient
		gardenClient      kubernetes.Interface
		runtimeClient     kubernetes.Interface
	)

	AfterEach(func() {
		ctrl.Finish()
	})

	BeforeEach(func() {
		ctrl = gomock.NewController(GinkgoT())
		mockGardenClient = mockclient.NewMockClient(ctrl)
		mockRuntimeClient = mockclient.NewMockClient(ctrl)

		gardenClient = fake.NewClientSetBuilder().WithClient(mockGardenClient).Build()
		runtimeClient = fake.NewClientSetBuilder().WithClient(mockRuntimeClient).Build()

		testOperation = operation{
			log:                 logrus.NewEntry(logger.NewNopLogger()),
			runtimeClient:       runtimeClient,
			virtualGardenClient: &gardenClient,
			imports: &imports.Imports{
				GardenerAdmissionController: &imports.GardenerAdmissionController{Enabled: true},
			},
		}
	})

	It("should successfully validate the Gardener control plane", func() {
		var deploymentsToVerify = []string{"gardener-apiserver", "gardener-scheduler", "gardener-controller-manager", "gardener-admission-controller"}

		for _, deploymentName := range deploymentsToVerify {
			mockRuntimeClient.EXPECT().Get(gomock.Any(), kutil.Key("garden", deploymentName), gomock.AssignableToTypeOf(&appsv1.Deployment{})).DoAndReturn(
				func(ctx context.Context, _ client.ObjectKey, obj client.Object) error {
					(getHealthyDeployment()).DeepCopyInto(obj.(*appsv1.Deployment))
					return nil
				},
			)
		}

		mockGardenClient.EXPECT().Get(gomock.Any(), kutil.Key("v1beta1.core.gardener.cloud"), gomock.AssignableToTypeOf(&apiregistrationv1.APIService{})).DoAndReturn(
			func(ctx context.Context, _ client.ObjectKey, obj client.Object) error {
				(getHealthyAPIService()).DeepCopyInto(obj.(*apiregistrationv1.APIService))
				return nil
			},
		)

		mockGardenClient.EXPECT().List(gomock.Any(), gomock.AssignableToTypeOf(&gardencorev1beta1.SeedList{})).DoAndReturn(func(ctx context.Context, list *gardencorev1beta1.SeedList, opts ...client.ListOption) error {
			(&gardencorev1beta1.SeedList{Items: []gardencorev1beta1.Seed{{}}}).DeepCopyInto(list)
			return nil
		})

		Expect(testOperation.VerifyControlplane(ctx)).ToNot(HaveOccurred())
	})

	It("should successfully validate - no Gardener Admission Controller", func() {
		var deploymentsToVerify = []string{"gardener-apiserver", "gardener-scheduler", "gardener-controller-manager"}

		for _, deploymentName := range deploymentsToVerify {
			mockRuntimeClient.EXPECT().Get(gomock.Any(), kutil.Key("garden", deploymentName), gomock.AssignableToTypeOf(&appsv1.Deployment{})).DoAndReturn(
				func(ctx context.Context, _ client.ObjectKey, obj client.Object) error {
					(getHealthyDeployment()).DeepCopyInto(obj.(*appsv1.Deployment))
					return nil
				},
			)
		}

		mockGardenClient.EXPECT().Get(gomock.Any(), kutil.Key("v1beta1.core.gardener.cloud"), gomock.AssignableToTypeOf(&apiregistrationv1.APIService{})).DoAndReturn(
			func(ctx context.Context, _ client.ObjectKey, obj client.Object) error {
				(getHealthyAPIService()).DeepCopyInto(obj.(*apiregistrationv1.APIService))
				return nil
			},
		)

		mockGardenClient.EXPECT().List(gomock.Any(), gomock.AssignableToTypeOf(&gardencorev1beta1.SeedList{})).DoAndReturn(func(ctx context.Context, list *gardencorev1beta1.SeedList, opts ...client.ListOption) error {
			(&gardencorev1beta1.SeedList{Items: []gardencorev1beta1.Seed{{}}}).DeepCopyInto(list)
			return nil
		})

		testOperation.imports.GardenerAdmissionController.Enabled = false
		Expect(testOperation.VerifyControlplane(ctx)).ToNot(HaveOccurred())
	})
})

func getHealthyDeployment() *appsv1.Deployment {
	return &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Generation: 1,
		},
		Status: appsv1.DeploymentStatus{Conditions: []appsv1.DeploymentCondition{
			{
				Type:   appsv1.DeploymentAvailable,
				Status: corev1.ConditionTrue,
			},
		},
			ObservedGeneration: 1,
		},
	}
}

func getHealthyAPIService() *apiregistrationv1.APIService {
	return &apiregistrationv1.APIService{
		ObjectMeta: metav1.ObjectMeta{
			Generation: 1,
		},
		Status: apiregistrationv1.APIServiceStatus{
			Conditions: []apiregistrationv1.APIServiceCondition{
				{
					Status:             apiregistrationv1.ConditionTrue,
					LastTransitionTime: metav1.Time{},
					Reason:             "Even better",
					Message:            "All great",
				},
			},
		},
	}
}

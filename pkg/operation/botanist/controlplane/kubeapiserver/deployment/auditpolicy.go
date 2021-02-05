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

package deployment

import (
	"bytes"
	"fmt"

	"github.com/gardener/gardener/pkg/client/kubernetes"
	"github.com/gardener/gardener/pkg/mock/go/context"
	"github.com/gardener/gardener/pkg/utils"
	kutil "github.com/gardener/gardener/pkg/utils/kubernetes"
	"github.com/gardener/gardener/pkg/utils/version"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/runtime/serializer"
	audit_internal "k8s.io/apiserver/pkg/apis/audit"
	auditv1 "k8s.io/apiserver/pkg/apis/audit/v1"
	auditv1alpha1 "k8s.io/apiserver/pkg/apis/audit/v1alpha1"
	auditv1beta1 "k8s.io/apiserver/pkg/apis/audit/v1beta1"
	auditvalidation "k8s.io/apiserver/pkg/apis/audit/validation"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

const auditPolicyConfigMapDataKey = "policy"

var (
	runtimeScheme             = runtime.NewScheme()
	codecs                    = serializer.NewCodecFactory(runtimeScheme)
	decoder                   = codecs.UniversalDecoder()
	policyEncoderAuditV1      = kubernetes.SeedCodec.EncoderForVersion(kubernetes.SeedSerializer, auditv1.SchemeGroupVersion)
	policyEncoderAuditV1Beta1 = kubernetes.SeedCodec.EncoderForVersion(kubernetes.SeedSerializer, auditv1beta1.SchemeGroupVersion)
)

func init() {
	_ = auditv1alpha1.AddToScheme(runtimeScheme)
	_ = auditv1beta1.AddToScheme(runtimeScheme)
	_ = auditv1.AddToScheme(runtimeScheme)
	_ = audit_internal.AddToScheme(runtimeScheme)
}

func (k *kubeAPIServer) deployAuditPolicyConfigMap(ctx context.Context) (*string, error) {
	var (
		auditPolicy       string
		err               error
		buf               = new(bytes.Buffer)
		auditPolicyConfig = k.emptyConfigMap(cmNameAuditPolicyConfig)
	)

	if k.config != nil &&
		k.config.AuditConfig != nil &&
		k.config.AuditConfig.AuditPolicy != nil &&
		k.config.AuditConfig.AuditPolicy.ConfigMapRef != nil {
		auditPolicy, err = k.getAuditPolicy(ctx, k.config.AuditConfig.AuditPolicy.ConfigMapRef.Name, k.gardenNamespace)
		if err != nil {
			// Ignore missing audit configuration on shoot deletion to prevent failing redeployments of the
			// kube-apiserver in case the end-user deleted the configmap before/simultaneously to the shoot
			// deletion.
			if !apierrors.IsNotFound(err) || !k.shootHasDeletionTimestamp {
				return nil, fmt.Errorf("deploy the Kube API server audit policy config map to the Seed failed. Retrieval of the audit policy from the ConfigMap '%v' in the Garden cluster failed: %+v", k.config.AuditConfig.AuditPolicy.ConfigMapRef.Name, err)
			}
		}
	} else if versionConstraintK8sGreaterEqual112.Check(k.shootKubernetesVersion) {
		policy := auditv1.Policy{
			Rules: []auditv1.PolicyRule{
				{
					Level: auditv1.LevelNone,
				},
			},
		}

		policyEncoderAuditV1 = kubernetes.SeedCodec.EncoderForVersion(kubernetes.SeedSerializer, auditv1.SchemeGroupVersion)
		if err := policyEncoderAuditV1.Encode(&policy, buf); err != nil {
			return nil, fmt.Errorf("failed to encode the Shoot API server Audit Policy config map (%s/%s): %v", k.seedNamespace, cmNameAuditPolicyConfig, err)
		}
		auditPolicy = buf.String()
	} else {
		policy := auditv1beta1.Policy{
			Rules: []auditv1beta1.PolicyRule{
				{
					Level: auditv1beta1.LevelNone,
				},
			},
		}

		if err := policyEncoderAuditV1Beta1.Encode(&policy, buf); err != nil {
			return nil, fmt.Errorf("failed to encode the Shoot API server Audit Policy config map (%s/%s): %v", k.seedNamespace, cmNameAuditPolicyConfig, err)
		}
		auditPolicy = buf.String()
	}

	auditPolicyConfigSHA := auditPolicyConfig
	auditPolicyConfigSHA.Data = map[string]string{
		fileNameAuditPolicyConfig: `---
` + auditPolicy,
	}

	if _, err = controllerutil.CreateOrUpdate(ctx, k.seedClient.Client(), auditPolicyConfig, func() error {
		auditPolicyConfig.Data = auditPolicyConfigSHA.Data
		return nil
	}); err != nil {
		return nil, err
	}

	buf = new(bytes.Buffer)
	if err := encoderCoreV1.Encode(auditPolicyConfigSHA, buf); err != nil {
		return nil, err
	}
	sha256Hex := utils.ComputeSHA256Hex(buf.Bytes())
	return &sha256Hex, nil
}

func (k *kubeAPIServer) getAuditPolicy(ctx context.Context, name, namespace string) (string, error) {
	auditPolicyCm := &corev1.ConfigMap{}
	if err := k.gardenClient.Get(ctx, kutil.Key(namespace, name), auditPolicyCm); err != nil {
		return "", err
	}
	auditPolicy, ok := auditPolicyCm.Data[auditPolicyConfigMapDataKey]
	if !ok {
		return "", fmt.Errorf("missing '.data.policy' in audit policy configmap %v/%v", namespace, name)
	}
	if len(auditPolicy) == 0 {
		return "", fmt.Errorf("empty audit policy. Provide non-empty audit policy")
	}

	auditPolicyObj, schemaVersion, err := decoder.Decode([]byte(auditPolicy), nil, nil)
	if err != nil {
		return "", fmt.Errorf("failed to decode the provided audit policy err=%v", err)
	}

	if isValidVersion, err := IsValidAuditPolicyVersion(k.shootKubernetesVersion.String(), schemaVersion); err != nil {
		return "", err
	} else if !isValidVersion {
		return "", fmt.Errorf("your shoot cluster version %q is not compatible with audit policy version %q", k.shootKubernetesVersion.String(), schemaVersion.GroupVersion().String())
	}

	auditPolicyInternal, ok := auditPolicyObj.(*audit_internal.Policy)
	if !ok {
		return "", fmt.Errorf("failure to cast to audit Policy type: %v", schemaVersion)
	}
	errList := auditvalidation.ValidatePolicy(auditPolicyInternal)
	if len(errList) != 0 {
		return "", fmt.Errorf("provided invalid audit policy err=%v", errList)
	}
	return auditPolicy, nil
}

// IsValidAuditPolicyVersion checks whether the api server support the provided audit policy apiVersion
func IsValidAuditPolicyVersion(shootVersion string, schemaVersion *schema.GroupVersionKind) (bool, error) {
	auditGroupVersion := schemaVersion.GroupVersion().String()

	if auditGroupVersion == "audit.k8s.io/v1" {
		return version.CheckVersionMeetsConstraint(shootVersion, ">= v1.12")
	}
	return true, nil
}

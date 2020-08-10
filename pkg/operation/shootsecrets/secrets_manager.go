// Copyright (c) 2020 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

package shootsecrets

import (
	"context"
	"fmt"
	"reflect"
	"sync"

	gardencorev1alpha1helper "github.com/gardener/gardener/pkg/apis/core/v1alpha1/helper"
	"github.com/gardener/gardener/pkg/operation/common"
	"github.com/gardener/gardener/pkg/utils/infodata"
	"github.com/gardener/gardener/pkg/utils/secrets"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/sets"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// SecretConfigGeneratorFunc is a func used to generate secret configurations
type SecretConfigGeneratorFunc func(*secrets.BasicAuth, *secrets.StaticToken, map[string]*secrets.Certificate) ([]secrets.ConfigInterface, error)

// SecretsManager holds the configurations of all required shoot secrets that have to be preserved in the ShootState.
// It uses these configurations to load infodata from existing secrets into the ShootState, generate new secret infodata and save it into the ShootState
// or create kubernetes secret objects from infodata available in the ShootState and deploy them.
type SecretsManager struct {
	apiServerBasicAuthConfig    *secrets.BasicAuthSecretConfig
	staticTokenConfig           *secrets.StaticTokenSecretConfig
	certificateAuthorityConfigs map[string]*secrets.CertificateSecretConfig
	secretConfigGenerator       SecretConfigGeneratorFunc

	apiServerBasicAuth     *secrets.BasicAuth
	certificateAuthorities map[string]*secrets.Certificate

	existingSecrets map[string]*corev1.Secret

	GardenerResourceDataList gardencorev1alpha1helper.GardenerResourceDataList
	StaticToken              *secrets.StaticToken
	DeployedSecrets          map[string]*corev1.Secret
}

// NewSecretsManager takes in a list of GardenerResourceData items, a static token secret config, a map of certificate authority configs,
// a function which can generate secret configurations and returns a new SecretsManager struct
func NewSecretsManager(
	gardenerResourceDataList gardencorev1alpha1helper.GardenerResourceDataList,
	staticTokenConfig *secrets.StaticTokenSecretConfig,
	certificateAuthorityConfigs map[string]*secrets.CertificateSecretConfig,
	secretConfigGenerator SecretConfigGeneratorFunc,
) *SecretsManager {
	return &SecretsManager{
		GardenerResourceDataList:    gardenerResourceDataList,
		staticTokenConfig:           staticTokenConfig,
		certificateAuthorityConfigs: certificateAuthorityConfigs,
		secretConfigGenerator:       secretConfigGenerator,
		certificateAuthorities:      make(map[string]*secrets.Certificate, len(certificateAuthorityConfigs)),
		existingSecrets:             map[string]*corev1.Secret{},
		DeployedSecrets:             map[string]*corev1.Secret{},
	}
}

// WithExistingSecrets adds the provided map of existing secrets to the SecretsManager
func (s *SecretsManager) WithExistingSecrets(existingSecrets map[string]*corev1.Secret) *SecretsManager {
	s.existingSecrets = existingSecrets
	return s
}

// WithAPIServerBasicAuthConfig adds the provided basic auth secret configuration to the SecretsManager
func (s *SecretsManager) WithAPIServerBasicAuthConfig(config *secrets.BasicAuthSecretConfig) *SecretsManager {
	s.apiServerBasicAuthConfig = config
	return s
}

// Load gets the InfoData from all existing secrets in the shoot's control plane which are managed by gardener
// and adds it to the SecretManager's GardenerResourceDataList
func (s *SecretsManager) Load() error {
	if s.apiServerBasicAuthConfig != nil {
		if err := s.loadExistingSecretInfoDataAndUpdateResourceList(s.apiServerBasicAuthConfig); err != nil {
			return err
		}
	}

	if err := s.loadExistingSecretInfoDataAndUpdateResourceList(s.staticTokenConfig); err != nil {
		return err
	}

	for _, caConfig := range s.certificateAuthorityConfigs {
		if err := s.loadExistingSecretInfoDataAndUpdateResourceList(caConfig); err != nil {
			return err
		}
	}

	secretConfigs, err := s.secretConfigGenerator(nil, nil, nil)
	if err != nil {
		return err
	}

	for _, config := range secretConfigs {
		if err := s.loadExistingSecretInfoDataAndUpdateResourceList(config); err != nil {
			return err
		}
	}

	return nil
}

// Generate generates InfoData for all shoot secrets managed by gardener and adds it to the SecretManager's
// GardenerResourceData
func (s *SecretsManager) Generate() error {
	if s.apiServerBasicAuthConfig != nil {
		if err := s.generateInfoDataAndUpdateResourceList(s.apiServerBasicAuthConfig); err != nil {
			return err
		}
	}

	if err := s.generateStaticTokenAndUpdateResourceList(); err != nil {
		return err
	}

	for _, caConfig := range s.certificateAuthorityConfigs {
		if err := s.generateInfoDataAndUpdateResourceList(caConfig); err != nil {
			return err
		}
	}

	for name, caConfig := range s.certificateAuthorityConfigs {
		cert, err := s.getInfoDataAndGenerateSecret(caConfig)
		if err != nil {
			return err
		}
		s.certificateAuthorities[name] = cert.(*secrets.Certificate)
	}

	secretConfigs, err := s.secretConfigGenerator(nil, nil, s.certificateAuthorities)
	if err != nil {
		return err
	}

	for _, config := range secretConfigs {
		if err := s.generateInfoDataAndUpdateResourceList(config); err != nil {
			return err
		}
	}

	return nil
}

// Deploy gets InfoData for all shoot secrets managed by gardener from the SecretManager's GardenerResourceDataList
// and uses it to generate kubernetes secrets and deploy them in the provided namespace.
func (s *SecretsManager) Deploy(ctx context.Context, k8sClient client.Client, namespace string) error {
	if s.apiServerBasicAuthConfig != nil {
		if err := s.deployBasicAuthSecret(ctx, k8sClient, namespace); err != nil {
			return err
		}
	}

	if err := s.deployStaticToken(ctx, k8sClient, namespace); err != nil {
		return err
	}

	for name, caConfig := range s.certificateAuthorityConfigs {
		cert, err := s.getInfoDataAndGenerateSecret(caConfig)
		if err != nil {
			return err
		}
		s.certificateAuthorities[name] = cert.(*secrets.Certificate)
	}

	if err := s.deployCertificateAuthorities(ctx, k8sClient, namespace); err != nil {
		return err
	}

	if s.secretConfigGenerator == nil {
		return nil
	}

	secretConfigs, err := s.secretConfigGenerator(s.apiServerBasicAuth, s.StaticToken, s.certificateAuthorities)
	if err != nil {
		return err
	}

	deployedSecrets, err := secrets.GenerateClusterSecretsWithFunc(ctx, k8sClient, s.existingSecrets, secretConfigs, namespace, func(c secrets.ConfigInterface) (secrets.DataInterface, error) {
		return s.getInfoDataAndGenerateSecret(c)
	})
	if err != nil {
		return err
	}

	for name, secret := range deployedSecrets {
		s.DeployedSecrets[name] = secret
	}

	return nil
}

func (s *SecretsManager) generateStaticTokenAndUpdateResourceList() error {
	infoData, err := infodata.GetInfoData(s.GardenerResourceDataList, s.staticTokenConfig.Name)
	if err != nil {
		return err
	}

	if infoData == nil {
		data, err := s.staticTokenConfig.GenerateInfoData()
		if err != nil {
			return err
		}
		return infodata.UpsertInfoData(&s.GardenerResourceDataList, s.staticTokenConfig.Name, data)
	}

	staticTokenInfoData, ok := infoData.(*secrets.StaticTokenInfoData)
	if !ok {
		return fmt.Errorf("could not convert InfoData entry %s to StaticTokenInfoData", s.staticTokenConfig.Name)
	}

	newStaticTokenConfig := secrets.StaticTokenSecretConfig{
		Name:   common.StaticTokenSecretName,
		Tokens: make(map[string]secrets.TokenConfig),
	}

	var tokenConfigSet, tokenInfoDataSet = sets.NewString(), sets.NewString()
	for name := range s.staticTokenConfig.Tokens {
		tokenConfigSet.Insert(name)
	}
	for name := range staticTokenInfoData.Tokens {
		tokenInfoDataSet.Insert(name)
	}

	newTokenKeys, outdatedTokenKeys := []string{}, []string{}
	if diff := tokenConfigSet.Difference(tokenInfoDataSet); diff.Len() > 0 {
		newTokenKeys = diff.UnsortedList()
	} else if diff := tokenInfoDataSet.Difference(tokenConfigSet); diff.Len() > 0 {
		outdatedTokenKeys = diff.UnsortedList()
	} else {
		return nil
	}

	switch {
	case len(newTokenKeys) > 0:
		for _, tokenKey := range newTokenKeys {
			newStaticTokenConfig.Tokens[tokenKey] = s.staticTokenConfig.Tokens[tokenKey]
		}
		newStaticTokenInfoData, err := newStaticTokenConfig.GenerateInfoData()
		if err != nil {
			return err
		}
		staticTokenInfoData.Append(newStaticTokenInfoData.(*secrets.StaticTokenInfoData))
	case len(outdatedTokenKeys) > 0:
		staticTokenInfoData.RemoveTokens(outdatedTokenKeys...)
	}

	return infodata.UpsertInfoData(&s.GardenerResourceDataList, s.staticTokenConfig.Name, staticTokenInfoData)
}

func (s *SecretsManager) generateInfoDataAndUpdateResourceList(secretConfig secrets.ConfigInterface) error {
	if s.GardenerResourceDataList.Get(secretConfig.GetName()) != nil {
		return nil
	}
	data, err := secretConfig.GenerateInfoData()
	if err != nil {
		return err
	}
	return infodata.UpsertInfoData(&s.GardenerResourceDataList, secretConfig.GetName(), data)
}

func (s *SecretsManager) getInfoDataAndGenerateSecret(secretConfig secrets.ConfigInterface) (secrets.DataInterface, error) {
	secretInfoData, err := infodata.GetInfoData(s.GardenerResourceDataList, secretConfig.GetName())
	if err != nil {
		return nil, err
	}
	if secretInfoData == nil {
		return secretConfig.Generate()
	}

	return secretConfig.GenerateFromInfoData(secretInfoData)
}

func (s *SecretsManager) loadExistingSecretInfoDataAndUpdateResourceList(secretConfig secrets.ConfigInterface) error {
	name := secretConfig.GetName()
	if s.GardenerResourceDataList.Get(name) != nil {
		return nil
	}

	secret, ok := s.existingSecrets[name]
	if !ok {
		return nil
	}

	infodataLoader := secretConfig.(infodata.Loader)
	infoData, err := infodataLoader.LoadFromSecretData(secret.Data)
	if err != nil {
		return err
	}
	return infodata.UpsertInfoData(&s.GardenerResourceDataList, name, infoData)
}

// DeployBasicAuthSecret deploys the APIServer BasicAuth secret
func (s *SecretsManager) deployBasicAuthSecret(ctx context.Context, k8sClient client.Client, namespace string) error {
	secretInterface, err := s.getInfoDataAndGenerateSecret(s.apiServerBasicAuthConfig)
	if err != nil {
		return err
	}
	s.apiServerBasicAuth = secretInterface.(*secrets.BasicAuth)

	secret, err := s.deploySecret(ctx, k8sClient, namespace, s.apiServerBasicAuth, s.apiServerBasicAuth.Name)
	if err != nil {
		return err
	}

	s.DeployedSecrets[secret.Name] = secret

	return nil
}

func (s *SecretsManager) deployStaticToken(ctx context.Context, k8sClient client.Client, namespace string) error {
	secretInterface, err := s.getInfoDataAndGenerateSecret(s.staticTokenConfig)
	if err != nil {
		return err
	}
	s.StaticToken = secretInterface.(*secrets.StaticToken)

	data := s.StaticToken.SecretData()

	if secret, ok := s.existingSecrets[s.staticTokenConfig.Name]; ok {
		if reflect.DeepEqual(data, secret.Data) {
			s.DeployedSecrets[secret.Name] = secret
			return nil
		}
		secret.Data = data
		if err := k8sClient.Update(ctx, secret); err != nil {
			return err
		}
		s.DeployedSecrets[secret.Name] = secret
		return nil
	}

	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      s.staticTokenConfig.Name,
			Namespace: namespace,
		},
		Type: corev1.SecretTypeOpaque,
		Data: data,
	}

	if err := k8sClient.Create(ctx, secret); err != nil {
		return err
	}

	s.DeployedSecrets[secret.Name] = secret

	return nil
}

func (s *SecretsManager) deployCertificateAuthorities(ctx context.Context, k8sClient client.Client, namespace string) error {
	type caOutput struct {
		secret *corev1.Secret
		err    error
	}

	var (
		wg        sync.WaitGroup
		results   = make(chan *caOutput)
		errorList = []error{}
	)

	for name, certificateAuthority := range s.certificateAuthorities {
		wg.Add(1)
		go func(c *secrets.Certificate, n string) {
			defer wg.Done()
			secret, err := s.deploySecret(ctx, k8sClient, namespace, c, n)
			results <- &caOutput{secret, err}
		}(certificateAuthority, name)
	}
	go func() {
		wg.Wait()
		close(results)
	}()
	for out := range results {
		if out.err != nil {
			errorList = append(errorList, out.err)
			continue
		}
		s.DeployedSecrets[out.secret.Name] = out.secret
	}
	// Wait and check whether an error occurred during the parallel processing of the Secret creation.
	if len(errorList) > 0 {
		return fmt.Errorf("errors occurred during certificate authority generation: %+v", errorList)
	}
	return nil
}

func (s *SecretsManager) deploySecret(ctx context.Context, k8sClient client.Client, namespace string, secretInterface secrets.DataInterface, secretName string) (*corev1.Secret, error) {
	if secret, ok := s.existingSecrets[secretName]; ok {
		return secret, nil
	}

	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      secretName,
			Namespace: namespace,
		},
		Type: corev1.SecretTypeOpaque,
		Data: secretInterface.SecretData(),
	}

	if err := k8sClient.Create(ctx, secret); err != nil {
		return nil, err
	}
	return secret, nil
}

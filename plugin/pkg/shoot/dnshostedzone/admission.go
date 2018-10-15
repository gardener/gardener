// Copyright (c) 2018 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

package dnshostedzone

import (
	"errors"
	"fmt"
	"io"
	"strings"

	"github.com/gardener/gardener/pkg/apis/garden"
	admissioninitializer "github.com/gardener/gardener/pkg/apiserver/admission/initializer"
	gardeninformers "github.com/gardener/gardener/pkg/client/garden/informers/internalversion"
	gardenlisters "github.com/gardener/gardener/pkg/client/garden/listers/garden/internalversion"
	"github.com/gardener/gardener/pkg/operation/cloudbotanist/awsbotanist"

	"github.com/gardener/gardener/pkg/operation/cloudbotanist/gcpbotanist"
	"github.com/gardener/gardener/pkg/operation/cloudbotanist/openstackbotanist"
	"github.com/gardener/gardener/pkg/operation/common"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apiserver/pkg/admission"
	kubeinformers "k8s.io/client-go/informers"
	kubecorev1listers "k8s.io/client-go/listers/core/v1"
)

const (
	// PluginName is the name of this admission plugin.
	PluginName = "ShootDNSHostedZone"
)

// Register registers a plugin.
func Register(plugins *admission.Plugins) {
	plugins.Register(PluginName, func(config io.Reader) (admission.Interface, error) {
		return New()
	})
}

// DNSHostedZone contains listers and and admission handler.
type DNSHostedZone struct {
	*admission.Handler
	secretLister        kubecorev1listers.SecretLister
	secretBindingLister gardenlisters.SecretBindingLister
	readyFunc           admission.ReadyFunc
}

var (
	_ = admissioninitializer.WantsInternalGardenInformerFactory(&DNSHostedZone{})
	_ = admissioninitializer.WantsKubeInformerFactory(&DNSHostedZone{})

	readyFuncs = []admission.ReadyFunc{}
)

// New creates a new DNSHostedZone admission plugin.
func New() (*DNSHostedZone, error) {
	return &DNSHostedZone{
		Handler: admission.NewHandler(admission.Create, admission.Update),
	}, nil
}

// AssignReadyFunc assigns the ready function to the admission handler.
func (d *DNSHostedZone) AssignReadyFunc(f admission.ReadyFunc) {
	d.readyFunc = f
	d.SetReadyFunc(f)
}

// SetInternalGardenInformerFactory gets Lister from SharedInformerFactory.
func (d *DNSHostedZone) SetInternalGardenInformerFactory(f gardeninformers.SharedInformerFactory) {
	secretBindingInformer := f.Garden().InternalVersion().SecretBindings()
	d.secretBindingLister = secretBindingInformer.Lister()

	readyFuncs = append(readyFuncs, secretBindingInformer.Informer().HasSynced)
}

// SetKubeInformerFactory gets Lister from SharedInformerFactory.
func (d *DNSHostedZone) SetKubeInformerFactory(f kubeinformers.SharedInformerFactory) {
	secretInformer := f.Core().V1().Secrets()
	d.secretLister = secretInformer.Lister()

	readyFuncs = append(readyFuncs, secretInformer.Informer().HasSynced)
}

// ValidateInitialization checks whether the plugin was correctly initialized.
func (d *DNSHostedZone) ValidateInitialization() error {
	if d.secretLister == nil {
		return errors.New("missing secret lister")
	}
	if d.secretBindingLister == nil {
		return errors.New("missing secret binding lister")
	}
	return nil
}

// Admit tries to determine a DNS hosted zone for the Shoot's external domain.
func (d *DNSHostedZone) Admit(a admission.Attributes) error {
	// Wait until the caches have been synced
	if d.readyFunc == nil {
		d.AssignReadyFunc(func() bool {
			for _, readyFunc := range readyFuncs {
				if !readyFunc() {
					return false
				}
			}
			return true
		})
	}
	if !d.WaitForReady() {
		return admission.NewForbidden(a, errors.New("not yet ready to handle request"))
	}

	// Ignore all kinds other than Shoot
	if a.GetKind().GroupKind() != garden.Kind("Shoot") {
		return nil
	}
	shoot, ok := a.GetObject().(*garden.Shoot)
	if !ok {
		return apierrors.NewBadRequest("could not convert resource into Shoot object")
	}

	// If the Shoot manifest specifies the 'unmanaged' DNS provider, then we do nothing.
	if shoot.Spec.DNS.Provider == garden.DNSUnmanaged {
		return nil
	}

	// Alicloud DNS needs no HostedZoneID, no need to do check by now
	// As about credentials needed by shoot DNS, they are checked in operation/botanist/dns.go
	if shoot.Spec.DNS.Provider == garden.DNSAlicloud {
		return nil
	}

	// If the Shoot manifest specifies a Hosted Zone ID then we check whether it is an ID of a default domain.
	// If not, we must ensure that the cloud provider secret provided by the user contain credentials for the
	// respective DNS provider.
	if shoot.Spec.DNS.HostedZoneID != nil {
		if err := verifyHostedZoneID(shoot, d.secretBindingLister, d.secretLister); err != nil {
			return admission.NewForbidden(a, err)
		}
		return nil
	}

	hostedZoneID, err := determineHostedZoneID(shoot, d.secretLister)
	if err != nil {
		return admission.NewForbidden(a, err)
	}
	shoot.Spec.DNS.HostedZoneID = &hostedZoneID

	return nil
}

// verifyHostedZoneID verifies that the cloud provider secret for the Shoot cluster contains credentials for the
// respective DNS provider.
func verifyHostedZoneID(shoot *garden.Shoot, secretBindingLister gardenlisters.SecretBindingLister, secretLister kubecorev1listers.SecretLister) error {
	secrets, err := getDefaultDomainSecrets(secretLister)
	if err != nil {
		return err
	}

	// If the dns domain matches a default domain then the specified hosted zone id must also match.
	for _, secret := range secrets {
		if strings.HasSuffix(*(shoot.Spec.DNS.Domain), secret.Annotations[common.DNSDomain]) && shoot.Spec.DNS.Provider == garden.DNSProvider(secret.Annotations[common.DNSProvider]) {
			if *shoot.Spec.DNS.HostedZoneID == secret.Annotations[common.DNSHostedZoneID] {
				return nil
			}
			return errors.New("specified dns domain matches a default domain but the specified hosted zone id is incorrect")
		}
	}

	// If the hosted zone id does not match a default domain then either the cloud provider secret must contain valid
	// credentials for the respective DNS provider, or the spec.dns section must contain a field 'secretName' which is
	// a reference to a secret containing the needed credentials.
	var credentials *corev1.Secret
	if shoot.Spec.DNS.SecretName != nil {
		referencedSecret, err := secretLister.Secrets(shoot.Namespace).Get(*shoot.Spec.DNS.SecretName)
		if err != nil {
			return err
		}
		credentials = referencedSecret
	} else {
		cloudProviderSecret, err := getCloudProviderSecret(shoot, secretBindingLister, secretLister)
		if err != nil {
			return err
		}
		credentials = cloudProviderSecret
	}

	switch shoot.Spec.DNS.Provider {
	case garden.DNSAWSRoute53:
		_, accessKeyFound := credentials.Data[awsbotanist.AccessKeyID]
		_, secretKeyFound := credentials.Data[awsbotanist.SecretAccessKey]
		if !accessKeyFound || !secretKeyFound {
			return fmt.Errorf("specifying the `.spec.dns.hostedZoneID` field is only possible if the cloud provider secret or the secret referenced in .spec.dns.secretName contains credentials for AWS Route53 (%s and %s)", awsbotanist.AccessKeyID, awsbotanist.SecretAccessKey)
		}
	case garden.DNSGoogleCloudDNS:
		_, serviceAccountJSONFound := credentials.Data[gcpbotanist.ServiceAccountJSON]
		if !serviceAccountJSONFound {
			return fmt.Errorf("specifying the `.spec.dns.hostedZoneID` field is only possible if the cloud provider secret or the secret referenced in .spec.dns.secretName contains credentials for Google CloudDNS (%s)", gcpbotanist.ServiceAccountJSON)
		}
	case garden.DNSOpenstackDesignate:
		_, authURLFound := credentials.Data[openstackbotanist.AuthURL]
		_, domainNameFound := credentials.Data[openstackbotanist.DomainName]
		_, tenantNameFound := credentials.Data[openstackbotanist.TenantName]
		_, usernameFound := credentials.Data[openstackbotanist.UserName]
		_, userDomainNameFound := credentials.Data[openstackbotanist.UserDomainName]
		_, passwordFound := credentials.Data[openstackbotanist.Password]
		if !authURLFound || !domainNameFound || !tenantNameFound || !usernameFound || !userDomainNameFound || !passwordFound {
			return fmt.Errorf("specifying the `.spec.dns.hostedZoneID` field is only possible if the cloud provider secret or the secret referenced in .spec.dns.secretName contains credentials for Designate (%s, %s, %s, %s, %s and %s)", openstackbotanist.AuthURL, openstackbotanist.DomainName, openstackbotanist.TenantName, openstackbotanist.UserName, openstackbotanist.UserDomainName, openstackbotanist.Password)
		}

	}

	return nil
}

// determineHostedZoneID reads the default domain secrets and compare their annotations with the specified Shoot
// domain. If both fit, it will return the Hosted Zone of the respective default domain.
func determineHostedZoneID(shoot *garden.Shoot, secretLister kubecorev1listers.SecretLister) (string, error) {
	secrets, err := getDefaultDomainSecrets(secretLister)
	if err != nil {
		return "", err
	}

	if len(secrets) == 0 {
		return "", apierrors.NewInternalError(errors.New("failed to determine a DNS hosted zone: no default domain secrets found"))
	}

	for _, secret := range secrets {
		if strings.HasSuffix(*(shoot.Spec.DNS.Domain), secret.Annotations[common.DNSDomain]) && shoot.Spec.DNS.Provider == garden.DNSProvider(secret.Annotations[common.DNSProvider]) {
			return secret.Annotations[common.DNSHostedZoneID], nil
		}
	}

	return "", apierrors.NewInternalError(errors.New("failed to determine a hosted zone id for the given domain (no default domain secret matches)"))
}

// getDefaultDomainSecrets filters the secrets in the Garden namespace for default domain secrets.
func getDefaultDomainSecrets(secretLister kubecorev1listers.SecretLister) ([]corev1.Secret, error) {
	defaultDomainSecrets := []corev1.Secret{}

	selector, err := labels.Parse(fmt.Sprintf("%s=%s", common.GardenRole, common.GardenRoleDefaultDomain))
	if err != nil {
		return nil, err
	}
	secrets, err := secretLister.Secrets(common.GardenNamespace).List(selector)
	if err != nil {
		return nil, err
	}

	for _, secret := range secrets {
		metadata := secret.ObjectMeta
		if !metav1.HasAnnotation(metadata, common.DNSHostedZoneID) || !metav1.HasAnnotation(metadata, common.DNSProvider) || !metav1.HasAnnotation(metadata, common.DNSDomain) {
			continue
		}
		defaultDomainSecrets = append(defaultDomainSecrets, *secret)
	}

	return defaultDomainSecrets, nil
}

// getCloudProviderSecret reads the cloud provider secret specified by the binding referenced in the Shoot manifest.
func getCloudProviderSecret(shoot *garden.Shoot, secretBindingLister gardenlisters.SecretBindingLister, secretLister kubecorev1listers.SecretLister) (*corev1.Secret, error) {
	binding, err := secretBindingLister.SecretBindings(shoot.Namespace).Get(shoot.Spec.Cloud.SecretBindingRef.Name)
	if err != nil {
		return nil, err
	}
	return secretLister.Secrets(binding.SecretRef.Namespace).Get(binding.SecretRef.Name)
}

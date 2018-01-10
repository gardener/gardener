// Copyright 2018 The Gardener Authors.
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
	"github.com/gardener/gardener/pkg/operation/common"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apiserver/pkg/admission"
	kubeinformers "k8s.io/client-go/informers"
	kubecorev1listers "k8s.io/client-go/listers/core/v1"
)

// Register registers a plugin.
func Register(plugins *admission.Plugins) {
	plugins.Register("ShootDNSHostedZone", func(config io.Reader) (admission.Interface, error) {
		return New()
	})
}

// Finder contains listers and and admission handler.
type Finder struct {
	*admission.Handler
	secretLister               kubecorev1listers.SecretLister
	privateSecretBindingLister gardenlisters.PrivateSecretBindingLister
	crossSecretBindingLister   gardenlisters.CrossSecretBindingLister
}

var _ = admissioninitializer.WantsInternalGardenInformerFactory(&Finder{})
var _ = admissioninitializer.WantsKubeInformerFactory(&Finder{})

// New creates a new Finder admission plugin.
func New() (*Finder, error) {
	return &Finder{
		Handler: admission.NewHandler(admission.Create),
	}, nil
}

// SetInternalGardenInformerFactory gets Lister from SharedInformerFactory.
func (h *Finder) SetInternalGardenInformerFactory(f gardeninformers.SharedInformerFactory) {
	h.privateSecretBindingLister = f.Garden().InternalVersion().PrivateSecretBindings().Lister()
	h.crossSecretBindingLister = f.Garden().InternalVersion().CrossSecretBindings().Lister()
}

// SetKubeInformerFactory gets Lister from SharedInformerFactory.
func (h *Finder) SetKubeInformerFactory(f kubeinformers.SharedInformerFactory) {
	h.secretLister = f.Core().V1().Secrets().Lister()
}

// ValidateInitialization checks whether the plugin was correctly initialized.
func (h *Finder) ValidateInitialization() error {
	if h.secretLister == nil {
		return errors.New("missing secret lister")
	}
	if h.privateSecretBindingLister == nil {
		return errors.New("missing private secret binding lister")
	}
	if h.crossSecretBindingLister == nil {
		return errors.New("missing cross secret binding lister")
	}
	return nil
}

// Admit ensures that the object in-flight is of kind Shoot.
// In addition it tries to find an adequate Seed cluster for the given cloud provider profile and region,
// and writes the name into the Shoot specification.
func (h *Finder) Admit(a admission.Attributes) error {
	// Wait until the caches have been synced
	if !h.WaitForReady() {
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

	// If the Shoot manifest specifies a Hosted Zone ID then we check whether it is an ID of a default domain.
	// If not, we must ensure that the cloud provider secret provided by the user contain credentials for the
	// respective DNS provider.
	if shoot.Spec.DNS.HostedZoneID != nil {
		if err := verifyHostedZoneID(shoot, h.privateSecretBindingLister, h.crossSecretBindingLister, h.secretLister); err != nil {
			return admission.NewForbidden(a, err)
		}
		return nil
	}

	hostedZoneID, err := determineHostedZoneID(shoot, h.secretLister)
	if err != nil {
		return admission.NewForbidden(a, err)
	}
	shoot.Spec.DNS.HostedZoneID = &hostedZoneID

	return nil
}

// verifyHostedZoneID verifies that the cloud provider secret for the Shoot cluster contains credentials for the
// respective DNS provider.
func verifyHostedZoneID(shoot *garden.Shoot, privateSecretBindingLister gardenlisters.PrivateSecretBindingLister, crossSecretBindingLister gardenlisters.CrossSecretBindingLister, secretLister kubecorev1listers.SecretLister) error {
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

	// If the hosted zone id does not match a default domain then the cloud provider secret must contain valid
	// credentials for the respective DNS provider.
	cloudProviderSecret, err := getCloudProviderSecret(shoot, privateSecretBindingLister, crossSecretBindingLister, secretLister)
	if err != nil {
		return err
	}
	switch shoot.Spec.DNS.Provider {
	case garden.DNSAWSRoute53:
		_, accessKeyFound := cloudProviderSecret.Data[awsbotanist.AccessKeyID]
		_, secretKeyFound := cloudProviderSecret.Data[awsbotanist.SecretAccessKey]
		if !accessKeyFound || !secretKeyFound {
			return errors.New("specifying the `.spec.dns.hostedZoneID` field is only possible if the cloud provider secret contains AWS credentials for Route53")
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
func getCloudProviderSecret(shoot *garden.Shoot, privateSecretBindingLister gardenlisters.PrivateSecretBindingLister, crossSecretBindingLister gardenlisters.CrossSecretBindingLister, secretLister kubecorev1listers.SecretLister) (*corev1.Secret, error) {
	bindingRef := shoot.Spec.Cloud.SecretBindingRef

	switch bindingRef.Kind {
	case "PrivateSecretBinding":
		binding, err := privateSecretBindingLister.PrivateSecretBindings(shoot.Namespace).Get(bindingRef.Name)
		if err != nil {
			return nil, err
		}
		return secretLister.Secrets(binding.Namespace).Get(binding.SecretRef.Name)
	case "CrossSecretBinding":
		binding, err := crossSecretBindingLister.CrossSecretBindings(shoot.Namespace).Get(bindingRef.Name)
		if err != nil {
			return nil, err
		}
		return secretLister.Secrets(binding.SecretRef.Namespace).Get(binding.SecretRef.Name)
	default:
		return nil, errors.New("unknown secret binding reference kind")
	}
}

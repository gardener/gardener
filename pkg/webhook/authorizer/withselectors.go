// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package authorizer

import (
	"context"
	"fmt"
	"time"

	"github.com/Masterminds/semver/v3"
	"github.com/go-logr/logr"
	authorizationv1 "k8s.io/api/authorization/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/clock"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/gardener/gardener/pkg/client/kubernetes"
	versionutils "github.com/gardener/gardener/pkg/utils/version"
)

// WithSelectorsChecker checks whether the 'AuthorizeWithSelectors' feature is enabled in the kube-apiserver.
// TODO(rfranzke): Remove this interface once the lowest supported Kubernetes version is 1.34.
//
// Deprecated: This interface will be removed once the lowest supported Kubernetes version is 1.34.
type WithSelectorsChecker interface {
	// IsPossible returns true if the 'AuthorizeWithSelectors' feature is enabled in the kube-apiserver.
	IsPossible() (bool, error)
}

// NewWithSelectorsChecker creates a new WithSelectorsChecker.
func NewWithSelectorsChecker(ctx context.Context, log logr.Logger, clientSet kubernetes.Interface, clock clock.Clock) WithSelectorsChecker {
	return &withSelectorsChecker{
		ctx:       ctx,
		log:       log.WithName("authorize-with-selectors-checker"),
		clientSet: clientSet,
		clock:     clock,

		isPossible:    false,
		nextCheckTime: ptr.To(clock.Now()),
	}
}

type withSelectorsChecker struct {
	ctx       context.Context
	log       logr.Logger
	clientSet kubernetes.Interface
	clock     clock.Clock

	isPossible    bool
	nextCheckTime *time.Time
}

func (w *withSelectorsChecker) IsPossible() (bool, error) {
	if w.nextCheckTime != nil && w.clock.Now().UTC().After(w.nextCheckTime.UTC()) {
		enabled, mustCheckAgain, err := w.isAuthorizeWithSelectorsFeatureEnabled()
		if err != nil {
			return false, fmt.Errorf("failed checking whether 'AuthorizeWithSelectors' feature is enabled: %w", err)
		}

		w.isPossible = enabled
		if mustCheckAgain {
			w.nextCheckTime = ptr.To(w.clock.Now().UTC().Add(10 * time.Minute))
			w.log.Info("Must check again, caching result", "nextCheckTime", w.nextCheckTime.UTC().String())
		} else {
			w.nextCheckTime = nil
			w.log.Info("No need not check again")
		}
	}

	return w.isPossible, nil
}

func (w *withSelectorsChecker) isAuthorizeWithSelectorsFeatureEnabled() (enabled bool, mustCheckAgain bool, err error) {
	version, err := semver.NewVersion(w.clientSet.Version())
	if err != nil {
		return false, true, fmt.Errorf("failed parsing Kubernetes version %q to semver: %w", w.clientSet.Version(), err)
	}

	switch {
	case versionutils.ConstraintK8sLess131.Check(version):
		w.log.Info("Kubernetes version is lower than 1.31 -> feature gate did not exist", "authorizationWithSelectorsPossible", false)
		return false, false, nil

	case versionutils.ConstraintK8sGreaterEqual134.Check(version):
		w.log.Info("Kubernetes version is at least 1.34 -> feature gate is GA and locked to 'enabled'", "authorizationWithSelectorsPossible", true)
		return true, false, nil

	default:
		// Feature gate exists but is alpha/beta and may be disabled - we have to check whether it is enabled by making
		// a dry-run call with a label selector field. If it is still present in the output, then the
		// 'AuthorizeWithSelectors' feature is enabled. If it was disabled, kube-apiserver would have removed it in its
		// response.
		w.log.Info("Kubernetes version is between 1.31 and 1.33 -> feature gate is alpha/beta and may be disabled, must check")

		sar := &authorizationv1.SubjectAccessReview{
			Spec: authorizationv1.SubjectAccessReviewSpec{
				User: "gardener-admission-controller",
				ResourceAttributes: &authorizationv1.ResourceAttributes{
					Namespace:     "default",
					Verb:          "get",
					Resource:      "shoots",
					LabelSelector: &authorizationv1.LabelSelectorAttributes{RawSelector: "is-kubernetes-feature-gate-AuthorizeWithSelectors=enabled?"},
				},
			},
		}

		ctx, cancel := context.WithTimeout(w.ctx, 10*time.Second)
		defer cancel()

		if err := w.clientSet.Client().Create(ctx, sar, &client.CreateOptions{DryRun: []string{metav1.DryRunAll}}); err != nil {
			return false, true, fmt.Errorf("failed creating dry-run SubjectAccessReview: %w", err)
		}

		// If the label selector field is still present in the output, then the AuthorizeWithSelectors feature is enabled.
		// If it was disabled, kube-apiserver would have removed it in its response.
		authorizationWithSelectorsPossible := sar.Spec.ResourceAttributes.LabelSelector != nil
		w.log.Info("Check completed", "authorizationWithSelectorsPossible", authorizationWithSelectorsPossible)

		return authorizationWithSelectorsPossible, true, nil
	}
}

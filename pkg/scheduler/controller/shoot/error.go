package shoot

import (
	"fmt"
	"reflect"
	"strings"
)

const ( //defines possible reasons why a shoot could not be determined

	// Unknown indicates an unknown or non-specific reason for the occurred error.
	Unknown ErrorReason = "ukn"

	// NoMatchingSeed is used when no seed cluster was valid for scheduling the shoot.
	NoMatchingSeed ErrorReason = "nom"

	// LabelSelectorConversionFailed representation a failed conversion of label selectors.
	LabelSelectorConversionFailed ErrorReason = "lsc"

	// NoMatchingLabels is used when no seed was matching the labels defined by seed selector.
	NoMatchingLabels ErrorReason = "nml"

	// NoMatchingProvider indicates no seed had a matching provider.
	NoMatchingProvider ErrorReason = "nmp"

	// NoHASeedInZone is used when none of the seeds has at least 3 zones for hosting a shoot control plane with failure tolerance type 'zone'.
	NoHASeedInZone ErrorReason = "nsz"

	// NoSeedWithAccessRestriction indicates that no seed supports the access restrictions configured in the shoot specification.
	NoSeedWithAccessRestriction ErrorReason = "nar"

	// NoSeedForPurposeAndStrategy shows that no seed could be determined for the provided purpose and strategy.
	NoSeedForPurposeAndStrategy ErrorReason = "nst"

	// NoSeedForProfileAndStrategy is used when no seed could be determined for the provided cloud profile and strategy.
	NoSeedForProfileAndStrategy ErrorReason = "npr"
)

var errorReasons = []ErrorReason{ //contains all possible ErrorReasons
	Unknown,
	NoMatchingSeed,
	LabelSelectorConversionFailed,
	NoMatchingLabels,
	NoMatchingProvider,
	NoHASeedInZone,
	NoSeedWithAccessRestriction,
	NoSeedForPurposeAndStrategy,
	NoSeedForProfileAndStrategy,
}

const errorReasonPrefix = "dse"

// ErrorReason represents a failure case which can occur during the seed determination.
type ErrorReason string

func (er ErrorReason) suffix() string {
	return fmt.Sprintf("[%s-%s]", errorReasonPrefix, string(er))
}

// DetermineSeedError indicates a failure case happening during the seed determination.
type DetermineSeedError struct {
	reason  ErrorReason
	message string
}

func (e *DetermineSeedError) Error() string {
	return e.message
}

func newDetermineSeedError(reason ErrorReason, msg string, args ...any) *DetermineSeedError {
	errMsg := fmt.Sprintf("%s %s", msg, reason.suffix())
	return &DetermineSeedError{
		reason:  reason,
		message: fmt.Sprintf(errMsg, args...),
	}
}

// DetermineSeedErrorFromString converts an error message to a DetermineSeedError.
// If the error message is not related to a DetermineSeedError, an error will be returned.
func DetermineSeedErrorFromString(msg string) (*DetermineSeedError, error) {
	for _, errReason := range errorReasons {
		if strings.HasSuffix(msg, errReason.suffix()) {
			return &DetermineSeedError{
				reason:  errReason,
				message: msg,
			}, nil
		}
	}
	return nil, fmt.Errorf("error message could not be mapped to %s", reflect.TypeOf(DetermineSeedError{}).Name())
}

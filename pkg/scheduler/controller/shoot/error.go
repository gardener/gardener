package shoot

import (
	"fmt"
	"reflect"
	"strings"
)

const ( //defines possible reasons why a shoot could not be determined
	Unknown                       ErrorReason = "ukn"
	NoMatchingSeed                ErrorReason = "nom"
	NoUsableSeed                  ErrorReason = "nou"
	LabelSelectorConversionFailed ErrorReason = "lsc"
	NoMatchingLabels              ErrorReason = "nml"
	NoMatchingProvider            ErrorReason = "nmp"
	NoHASeedInZone                ErrorReason = "nsz"
	NoSeedWithAccessRestriction   ErrorReason = "nar"
	NoSeedForPurposeAndStrategy   ErrorReason = "nst"
	NoSeedForProfileAndStrategy   ErrorReason = "npr"
)

var errorReasons = []ErrorReason{ //contains all possible ErrorReasons
	Unknown,
	NoMatchingSeed,
	NoUsableSeed,
	LabelSelectorConversionFailed,
	NoMatchingLabels,
	NoMatchingProvider,
	NoHASeedInZone,
	NoSeedWithAccessRestriction,
	NoSeedForPurposeAndStrategy,
	NoSeedForProfileAndStrategy,
}

const errorReasonPrefix = "dse"

type ErrorReason string

func (er ErrorReason) suffix() string {
	return fmt.Sprintf("[%s-%s]", errorReasonPrefix, string(er))
}

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

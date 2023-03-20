// Copyright 2018 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

package errors_test

import (
	"fmt"
	"testing"

	"github.com/golang/mock/gomock"
	"github.com/hashicorp/go-multierror"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	errorsutils "github.com/gardener/gardener/pkg/utils/errors"
	errorsmock "github.com/gardener/gardener/pkg/utils/errors/mock"
)

func TestErrors(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Errors Suite")
}

var _ = Describe("Errors", func() {
	var (
		err1, err2 error
	)
	BeforeEach(func() {
		err1 = fmt.Errorf("error 1")
		err2 = fmt.Errorf("error 2")
	})

	Describe("#WithSuppressed", func() {
		It("should return nil if the error is nil", func() {
			Expect(errorsutils.WithSuppressed(nil, err2)).To(BeNil())
		})

		It("should return the error if the suppressed error is nil", func() {
			Expect(errorsutils.WithSuppressed(err1, nil)).To(BeIdenticalTo(err1))
		})

		It("should return an error with cause and suppressed equal to the given errors", func() {
			err := errorsutils.WithSuppressed(err1, err2)

			Expect(errorsutils.Unwrap(err)).To(BeIdenticalTo(err1))
			Expect(errorsutils.Suppressed(err)).To(BeIdenticalTo(err2))
		})
	})

	Describe("#Suppressed", func() {
		It("should retrieve the suppressed error", func() {
			Expect(errorsutils.Suppressed(errorsutils.WithSuppressed(err1, err2))).To(BeIdenticalTo(err2))
		})

		It("should retrieve nil if the error doesn't have a suppressed error", func() {
			Expect(errorsutils.Suppressed(err1)).To(BeNil())
		})
	})

	Context("withSuppressed", func() {
		Describe("#Error", func() {
			It("should return an error message consisting of the two errors", func() {
				Expect(errorsutils.WithSuppressed(err1, err2).Error()).To(Equal("error 1, suppressed: error 2"))
			})
		})

		Describe("#Format", func() {
			It("should correctly format the error in verbose mode", func() {
				Expect(fmt.Sprintf("%+v", errorsutils.WithSuppressed(err1, err2))).
					To(Equal("error 1\nsuppressed: error 2"))
			})
		})
	})

	Describe("Error context", func() {
		It("Should panic with duplicate error IDs", func() {
			defer func() {
				_ = recover()
			}()

			errorContext := errorsutils.NewErrorContext("Test context", nil)
			errorContext.AddErrorID("ID1")
			errorContext.AddErrorID("ID1")
			Fail("Panic should have occurred")
		})
	})

	Describe("Error handling", func() {
		var (
			errorContext *errorsutils.ErrorContext
			ctrl         *gomock.Controller
		)

		BeforeEach(func() {
			errorContext = errorsutils.NewErrorContext("Test context", nil)
			ctrl = gomock.NewController(GinkgoT())
		})

		AfterEach(func() {
			ctrl.Finish()
		})

		It("Should update the error context", func() {
			errID := "x1"
			Expect(errorsutils.HandleErrors(errorContext,
				nil,
				nil,
				errorsutils.ToExecute(errID, func() error {
					return nil
				}),
			)).To(Succeed())
			Expect(errorContext.HasErrorWithID(errID)).To(BeTrue())
		})

		It("Should call default failure handler", func() {
			errorID := "x1"
			errorText := fmt.Sprintf("Error in %s", errorID)
			expectedErr := errorsutils.WithID(errorID, fmt.Errorf("%s failed (%s)", errorID, errorText))
			err := errorsutils.HandleErrors(errorContext,
				nil,
				nil,
				errorsutils.ToExecute(errorID, func() error {
					return fmt.Errorf(errorText)
				}),
			)

			Expect(err).To(Equal(expectedErr))
		})

		It("Should call failure handler on fail", func() {
			errID := "x1"
			errorText := "Error from task"
			expectedErr := fmt.Errorf("Got %s %s", errID, errorText)
			err := errorsutils.HandleErrors(errorContext,
				nil,
				func(errorID string, err error) error {
					return fmt.Errorf(fmt.Sprintf("Got %s %s", errorID, err))
				},
				errorsutils.ToExecute(errID, func() error {
					return fmt.Errorf(errorText)
				}),
			)

			Expect(err).To(Equal(expectedErr))
		})

		It("Should return a cancelError when manually canceled", func() {
			errID := "x1"
			err := errorsutils.HandleErrors(errorContext,
				nil,
				nil,
				errorsutils.ToExecute(errID, func() error {
					return errorsutils.Cancel()
				}),
			)

			Expect(errorsutils.WasCanceled(errorsutils.Unwrap(err))).To(BeTrue())
		})

		It("Should stop execution on error", func() {
			expectedErr := fmt.Errorf("Err1")
			f1 := errorsmock.NewMockTaskFunc(ctrl)
			f2 := errorsmock.NewMockTaskFunc(ctrl)
			f3 := errorsmock.NewMockTaskFunc(ctrl)

			f1.EXPECT().Do(errorContext).Return("x1", nil)
			f2.EXPECT().Do(errorContext).Return("x2", expectedErr)
			f3.EXPECT().Do(errorContext).Times(0)

			err := errorsutils.HandleErrors(errorContext,
				nil,
				func(errorID string, err error) error {
					return err
				},
				f1,
				f2,
				f3,
			)

			Expect(err).To(Equal(expectedErr))
		})

		It("Should call success handler on error resolution", func() {
			errID := "x2"
			errorContext := errorsutils.NewErrorContext("Check success handler", []string{errID})
			Expect(errorsutils.HandleErrors(errorContext,
				func(errorID string) error {
					return nil
				},
				nil,
				errorsutils.ToExecute("x1", func() error {
					return nil
				}),
				errorsutils.ToExecute(errID, func() error {
					return nil
				}),
			)).To(Succeed())
		})

		It("Should execute methods sequentially in the specified order", func() {
			f1 := errorsmock.NewMockTaskFunc(ctrl)
			f2 := errorsmock.NewMockTaskFunc(ctrl)
			f3 := errorsmock.NewMockTaskFunc(ctrl)

			gomock.InOrder(
				f1.EXPECT().Do(errorContext).Return("x1", nil),
				f2.EXPECT().Do(errorContext).Return("x2", nil),
				f3.EXPECT().Do(errorContext).Return("x3", nil),
			)

			err := errorsutils.HandleErrors(errorContext,
				nil,
				func(errorID string, err error) error {
					return err
				},
				f1,
				f2,
				f3,
			)

			Expect(err).To(Succeed())
		})
	})
})

var _ = Describe("Multierrors", func() {
	var (
		allErrs    *multierror.Error
		err1, err2 error
	)

	BeforeEach(func() {
		err1 = fmt.Errorf("error 1")
		err2 = fmt.Errorf("error 2")
	})

	Describe("#NewErrorFormatFuncWithPrefix", func() {
		BeforeEach(func() {
			allErrs = &multierror.Error{
				ErrorFormat: errorsutils.NewErrorFormatFuncWithPrefix("prefix"),
			}
		})

		It("should format a multierror correctly if it contains 1 error", func() {
			allErrs.Errors = []error{err1}
			Expect(allErrs.Error()).To(Equal("prefix: 1 error occurred: error 1"))
		})
		It("should format a multierror correctly if it contains multiple errors", func() {
			allErrs.Errors = []error{err1, err2}
			Expect(allErrs.Error()).To(Equal("prefix: 2 errors occurred: [error 1, error 2]"))
		})
	})
})

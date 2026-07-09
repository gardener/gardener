// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

// This is a test file for executing unit tests for logcheck using golang.org/x/tools/go/analysis/analysistest.

package use_logr

import (
	"errors"
	"fmt"

	"use-logr/helper"

	"github.com/go-logr/logr"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
)

// notLogr has methods named like in logr.Logger but does not implement the interface.
type notLogr struct {
}

func (n notLogr) Enabled() bool                                     { return false }
func (n notLogr) Info(msg string, keysAndValues ...any)             {}
func (n notLogr) Error(err error, msg string, keysAndValues ...any) {}
func (n notLogr) WithValues(keysAndValues ...any) logr.Logger       { return logr.Logger{} }

func Info(msg string, keysAndValues ...any)             {}
func Error(err error, msg string, keysAndValues ...any) {}
func WithValues(keysAndValues ...any) logr.Logger       { return logr.Logger{} }

// begin test cases

func notCallingFuncWithoutSelectorExpr() {
	Info("foo")
	Error(errors.New("foo"), "bar")
	WithValues("foo", "bar")
}

func notCallingFuncWithSelectorExpr() {
	helper.Info("foo")
	helper.Error(errors.New("foo"), "bar")
	helper.WithValues("foo", "bar")
}

func notCallingRelevantMethod() {
	var nl notLogr
	nl.Enabled()

	var lf = logf.Log
	lf.Enabled()

	var l logr.Logger
	l.Enabled()
}

func missingArgs() {
	var nl notLogr
	nl.WithValues()

	var lf = logf.Log
	lf.WithValues() // want `call to "WithValues" without arguments`

	var l logr.Logger
	l.WithValues() // want `call to "WithValues" without arguments`
}

type fooStringer struct{}

// String implements fmt.Stringer
func (fooStringer) String() string { return "Foo" }

func constantStringMessage() {
	var err = errors.New("foo")
	const msg = "A constant string"
	var varMsg = "Not a constant string"
	var stringer fmt.Stringer = fooStringer{}

	var nl notLogr
	nl.Info("A constant string")
	nl.Info("A " + "constant string")
	nl.Info(msg)
	nl.Info("Constant " + msg)
	nl.Info(varMsg)
	nl.Info(varMsg + "foo")
	nl.Info(stringer.String())
	nl.Info(stringer.String() + "foo")

	nl.Error(err, "A constant string")
	nl.Error(err, "A "+"constant string")
	nl.Error(err, msg)
	nl.Error(err, "Constant "+msg)
	nl.Error(err, varMsg)
	nl.Error(err, varMsg+"foo")
	nl.Error(err, stringer.String())
	nl.Error(err, stringer.String()+"foo")

	var lf = logf.Log
	lf.Info("A constant string")
	lf.Info("A " + "constant string")
	lf.Info(msg)
	lf.Info("Constant " + msg)
	lf.Info(varMsg)                    // want `structured logging message should be a constant string expression`
	lf.Info(varMsg + "foo")            // want `structured logging message should be a constant string expression`
	lf.Info(stringer.String())         // want `structured logging message should be a constant string expression`
	lf.Info(stringer.String() + "foo") // want `structured logging message should be a constant string expression`

	lf.Error(err, "A constant string")
	lf.Error(err, "A "+"constant string")
	lf.Error(err, msg)
	lf.Error(err, "Constant "+msg)
	lf.Error(err, varMsg)                  // want `structured logging message should be a constant string expression`
	lf.Error(err, varMsg+"foo")            // want `structured logging message should be a constant string expression`
	lf.Error(err, stringer.String())       // want `structured logging message should be a constant string expression`
	lf.Error(err, stringer.String()+"foo") // want `structured logging message should be a constant string expression`

	var l logr.Logger
	l.Info("A constant string")
	l.Info("A " + "constant string")
	l.Info(msg)
	l.Info("Constant " + msg)
	l.Info(varMsg)                    // want `structured logging message should be a constant string expression`
	l.Info(varMsg + "foo")            // want `structured logging message should be a constant string expression`
	l.Info(stringer.String())         // want `structured logging message should be a constant string expression`
	l.Info(stringer.String() + "foo") // want `structured logging message should be a constant string expression`

	l.Error(err, "A constant string")
	l.Error(err, "A "+"constant string")
	l.Error(err, msg)
	l.Error(err, "Constant "+msg)
	l.Error(err, varMsg)                  // want `structured logging message should be a constant string expression`
	l.Error(err, varMsg+"foo")            // want `structured logging message should be a constant string expression`
	l.Error(err, stringer.String())       // want `structured logging message should be a constant string expression`
	l.Error(err, stringer.String()+"foo") // want `structured logging message should be a constant string expression`
}

func emptyMessage() {
	var err = errors.New("foo")
	const msg = ""

	var nl notLogr
	nl.Info("")
	nl.Info(msg)
	nl.Error(err, "")
	nl.Error(err, msg)

	var lf = logf.Log
	lf.Info("")        // want `structured logging message should not be empty: ""`
	lf.Info(msg)       // want `structured logging message should not be empty: ""`
	lf.Error(err, "")  // want `structured logging message should not be empty: ""`
	lf.Error(err, msg) // want `structured logging message should not be empty: ""`

	var l logr.Logger
	l.Info("")        // want `structured logging message should not be empty: ""`
	l.Info(msg)       // want `structured logging message should not be empty: ""`
	l.Error(err, "")  // want `structured logging message should not be empty: ""`
	l.Error(err, msg) // want `structured logging message should not be empty: ""`
}

func messageUsingFormatSpecifier() {
	var err = errors.New("foo")
	const msg = "Message using %s"

	var nl notLogr
	nl.Info("Message using %s")
	nl.Info(msg)
	nl.Error(err, "Message using %s")
	nl.Error(err, msg)

	var lf = logf.Log
	lf.Info("Message using %s")       // want `structured logging message should not use format specifier "%s"`
	lf.Info(msg)                      // want `structured logging message should not use format specifier "%s"`
	lf.Error(err, "Message using %s") // want `structured logging message should not use format specifier "%s"`
	lf.Error(err, msg)                // want `structured logging message should not use format specifier "%s"`

	var l logr.Logger
	l.Info("Message using %s")       // want `structured logging message should not use format specifier "%s"`
	l.Info(msg)                      // want `structured logging message should not use format specifier "%s"`
	l.Error(err, "Message using %s") // want `structured logging message should not use format specifier "%s"`
	l.Error(err, msg)                // want `structured logging message should not use format specifier "%s"`
}

func messageNotCapitalized() {
	var err = errors.New("foo")
	const msg = "message"

	var nl notLogr
	nl.Info("message")
	nl.Info(msg)
	nl.Error(err, "message")
	nl.Error(err, msg)

	var lf = logf.Log
	lf.Info("message")       // want `structured logging message should be capitalized: "message"`
	lf.Info(msg)             // want `structured logging message should be capitalized: "message"`
	lf.Error(err, "message") // want `structured logging message should be capitalized: "message"`
	lf.Error(err, msg)       // want `structured logging message should be capitalized: "message"`

	var l logr.Logger
	l.Info("message")       // want `structured logging message should be capitalized: "message"`
	l.Info(msg)             // want `structured logging message should be capitalized: "message"`
	l.Error(err, "message") // want `structured logging message should be capitalized: "message"`
	l.Error(err, msg)       // want `structured logging message should be capitalized: "message"`
}

func messageEndingWithPunctuationMark() {
	var err = errors.New("foo")
	const msg = "Message."

	var nl notLogr
	nl.Info("Message.")
	nl.Info(msg)
	nl.Error(err, "Message.")
	nl.Error(err, msg)

	var lf = logf.Log
	lf.Info("Message.")       // want `structured logging message should not end with punctuation mark: "Message."`
	lf.Info(msg)              // want `structured logging message should not end with punctuation mark: "Message."`
	lf.Error(err, "Message.") // want `structured logging message should not end with punctuation mark: "Message."`
	lf.Error(err, msg)        // want `structured logging message should not end with punctuation mark: "Message."`

	var l logr.Logger
	l.Info("Message.")       // want `structured logging message should not end with punctuation mark: "Message."`
	l.Info(msg)              // want `structured logging message should not end with punctuation mark: "Message."`
	l.Error(err, "Message.") // want `structured logging message should not end with punctuation mark: "Message."`
	l.Error(err, msg)        // want `structured logging message should not end with punctuation mark: "Message."`
}

func oddNumberOfArgs() {
	var err = errors.New("foo")

	var nl notLogr
	nl.WithValues("foo")
	nl.WithValues("foo", "bar")
	nl.WithValues("foo", "bar", "baz")
	nl.WithValues("foo", "bar", "baz", "boo")
	nl.Info("Message")
	nl.Info("Message", "foo")
	nl.Info("Message", "foo", "bar")
	nl.Info("Message", "foo", "bar", "baz")
	nl.Info("Message", "foo", "bar", "baz", "boo")
	nl.Error(err, "Message")
	nl.Error(err, "Message", "foo")
	nl.Error(err, "Message", "foo", "bar")
	nl.Error(err, "Message", "foo", "bar", "baz")
	nl.Error(err, "Message", "foo", "bar", "baz", "boo")

	var lf = logf.Log
	lf.WithValues("foo") // want `structured logging arguments to WithValues must be key-value pairs, got odd number of arguments: 1`
	lf.WithValues("foo", "bar")
	lf.WithValues("foo", "bar", "baz") // want `structured logging arguments to WithValues must be key-value pairs, got odd number of arguments: 3`
	lf.WithValues("foo", "bar", "baz", "boo")
	lf.Info("Message")
	lf.Info("Message", "foo") // want `structured logging arguments to Info must be key-value pairs, got odd number of arguments: 1`
	lf.Info("Message", "foo", "bar")
	lf.Info("Message", "foo", "bar", "baz") // want `structured logging arguments to Info must be key-value pairs, got odd number of arguments: 3`
	lf.Info("Message", "foo", "bar", "baz", "boo")
	lf.Error(err, "Message")
	lf.Error(err, "Message", "foo") // want `structured logging arguments to Error must be key-value pairs, got odd number of arguments: 1`
	lf.Error(err, "Message", "foo", "bar")
	lf.Error(err, "Message", "foo", "bar", "baz") // want `structured logging arguments to Error must be key-value pairs, got odd number of arguments: 3`
	lf.Error(err, "Message", "foo", "bar", "baz", "boo")

	var l logr.Logger
	l.WithValues("foo") // want `structured logging arguments to WithValues must be key-value pairs, got odd number of arguments: 1`
	l.WithValues("foo", "bar")
	l.WithValues("foo", "bar", "baz") // want `structured logging arguments to WithValues must be key-value pairs, got odd number of arguments: 3`
	l.WithValues("foo", "bar", "baz", "boo")
	l.Info("Message")
	l.Info("Message", "foo") // want `structured logging arguments to Info must be key-value pairs, got odd number of arguments: 1`
	l.Info("Message", "foo", "bar")
	l.Info("Message", "foo", "bar", "baz") // want `structured logging arguments to Info must be key-value pairs, got odd number of arguments: 3`
	l.Info("Message", "foo", "bar", "baz", "boo")
	l.Error(err, "Message")
	l.Error(err, "Message", "foo") // want `structured logging arguments to Error must be key-value pairs, got odd number of arguments: 1`
	l.Error(err, "Message", "foo", "bar")
	l.Error(err, "Message", "foo", "bar", "baz") // want `structured logging arguments to Error must be key-value pairs, got odd number of arguments: 3`
	l.Error(err, "Message", "foo", "bar", "baz", "boo")
}

func constantStringKey() {
	var err = errors.New("foo")
	const key = "constantKey"
	var varKey = "notConstantKey"
	var stringer fmt.Stringer = fooStringer{}

	var nl notLogr
	nl.Info("Message", "constantKey", "foo")
	nl.Info("Message", "also"+"ConstantKey", "foo")
	nl.Info("Message", key, "foo")
	nl.Info("Message", key+"Foo", "foo")
	nl.Info("Message", varKey, "foo")
	nl.Info("Message", varKey+"Foo", "foo")
	nl.Info("Message", stringer.String(), "foo")
	nl.Info("Message", stringer.String()+"Foo", "foo")

	nl.Error(err, "Message", "constantKey", "foo")
	nl.Error(err, "Message", "also"+"ConstantKey", "foo")
	nl.Error(err, "Message", key, "foo")
	nl.Error(err, "Message", key+"Foo", "foo")
	nl.Error(err, "Message", varKey, "foo")
	nl.Error(err, "Message", varKey+"Foo", "foo")
	nl.Error(err, "Message", stringer.String(), "foo")
	nl.Error(err, "Message", stringer.String()+"Foo", "foo")

	var lf = logf.Log
	lf.Info("Message", "constantKey", "foo")
	lf.Info("Message", "also"+"ConstantKey", "foo")
	lf.Info("Message", key, "foo")
	lf.Info("Message", key+"Foo", "foo")
	lf.Info("Message", varKey, "foo")                  // want `structured logging key should be a constant string expression`
	lf.Info("Message", varKey+"Foo", "foo")            // want `structured logging key should be a constant string expression`
	lf.Info("Message", stringer.String(), "foo")       // want `structured logging key should be a constant string expression`
	lf.Info("Message", stringer.String()+"Foo", "foo") // want `structured logging key should be a constant string expression`

	lf.Error(err, "Message", "constantKey", "foo")
	lf.Error(err, "Message", "also"+"ConstantKey", "foo")
	lf.Error(err, "Message", key, "foo")
	lf.Error(err, "Message", key+"Foo", "foo")
	lf.Error(err, "Message", varKey, "foo")                  // want `structured logging key should be a constant string expression`
	lf.Error(err, "Message", varKey+"Foo", "foo")            // want `structured logging key should be a constant string expression`
	lf.Error(err, "Message", stringer.String(), "foo")       // want `structured logging key should be a constant string expression`
	lf.Error(err, "Message", stringer.String()+"Foo", "foo") // want `structured logging key should be a constant string expression`

	var l logr.Logger
	l.Info("Message", "constantKey", "foo")
	l.Info("Message", "also"+"ConstantKey", "foo")
	l.Info("Message", key, "foo")
	l.Info("Message", key+"Foo", "foo")
	l.Info("Message", varKey, "foo")                  // want `structured logging key should be a constant string expression`
	l.Info("Message", varKey+"Foo", "foo")            // want `structured logging key should be a constant string expression`
	l.Info("Message", stringer.String(), "foo")       // want `structured logging key should be a constant string expression`
	l.Info("Message", stringer.String()+"Foo", "foo") // want `structured logging key should be a constant string expression`

	l.Error(err, "Message", "constantKey", "foo")
	l.Error(err, "Message", "also"+"ConstantKey", "foo")
	l.Error(err, "Message", key, "foo")
	l.Error(err, "Message", key+"Foo", "foo")
	l.Error(err, "Message", varKey, "foo")                  // want `structured logging key should be a constant string expression`
	l.Error(err, "Message", varKey+"Foo", "foo")            // want `structured logging key should be a constant string expression`
	l.Error(err, "Message", stringer.String(), "foo")       // want `structured logging key should be a constant string expression`
	l.Error(err, "Message", stringer.String()+"Foo", "foo") // want `structured logging key should be a constant string expression`
}

func emptyKey() {
	var err = errors.New("foo")
	const emptyKey, key = "", "notEmpty"

	var nl notLogr
	nl.Info("Message", "", "foo")
	nl.Info("Message", "notEmpty", "foo")
	nl.Info("Message", emptyKey, "foo")
	nl.Info("Message", key, "foo")

	nl.Error(err, "Message", "", "foo")
	nl.Error(err, "Message", "notEmpty", "foo")
	nl.Error(err, "Message", emptyKey, "foo")
	nl.Error(err, "Message", key, "foo")

	var lf = logf.Log
	lf.Info("Message", "", "foo") // want `structured logging key should not be empty: ""`
	lf.Info("Message", "notEmpty", "foo")
	lf.Info("Message", emptyKey, "foo") // want `structured logging key should not be empty: emptyKey`
	lf.Info("Message", key, "foo")

	lf.Error(err, "Message", "", "foo") // want `structured logging key should not be empty: ""`
	lf.Error(err, "Message", "notEmpty", "foo")
	lf.Error(err, "Message", emptyKey, "foo") // want `structured logging key should not be empty: emptyKey`
	lf.Error(err, "Message", key, "foo")

	var l logr.Logger
	l.Info("Message", "", "foo") // want `structured logging key should not be empty: ""`
	l.Info("Message", "notEmpty", "foo")
	l.Info("Message", emptyKey, "foo") // want `structured logging key should not be empty: emptyKey`
	l.Info("Message", key, "foo")

	l.Error(err, "Message", "", "foo") // want `structured logging key should not be empty: ""`
	l.Error(err, "Message", "notEmpty", "foo")
	l.Error(err, "Message", emptyKey, "foo") // want `structured logging key should not be empty: emptyKey`
	l.Error(err, "Message", key, "foo")
}

func keyUsingFormatSpecifier() {
	var err = errors.New("foo")
	const key = "keyUsing%s"

	var nl notLogr
	nl.Info("Message", "keyUsing%s", "foo")
	nl.Info("Message", key, "foo")
	nl.Error(err, "Message", "keyUsing%s", "foo")
	nl.Error(err, "Message", key, "foo")

	var lf = logf.Log
	lf.Info("Message", "keyUsing%s", "foo")       // want `structured logging key should not use format specifier "%s"`
	lf.Info("Message", key, "foo")                // want `structured logging key should not use format specifier "%s"`
	lf.Error(err, "Message", "keyUsing%s", "foo") // want `structured logging key should not use format specifier "%s"`
	lf.Error(err, "Message", key, "foo")          // want `structured logging key should not use format specifier "%s"`

	var l logr.Logger
	l.Info("Message", "keyUsing%s", "foo")       // want `structured logging key should not use format specifier "%s"`
	l.Info("Message", key, "foo")                // want `structured logging key should not use format specifier "%s"`
	l.Error(err, "Message", "keyUsing%s", "foo") // want `structured logging key should not use format specifier "%s"`
	l.Error(err, "Message", key, "foo")          // want `structured logging key should not use format specifier "%s"`
}

func capitalizedKey() {
	var err = errors.New("foo")
	const capitalizedKey, key = "UpperCamelCase", "lowerCamelCase"

	var nl notLogr
	nl.Info("Message", "UpperCamelCase", "foo")
	nl.Info("Message", "lowerCamelCase", "foo")
	nl.Info("Message", capitalizedKey, "foo")
	nl.Info("Message", key, "foo")

	nl.Error(err, "Message", "UpperCamelCase", "foo")
	nl.Error(err, "Message", "lowerCamelCase", "foo")
	nl.Error(err, "Message", capitalizedKey, "foo")
	nl.Error(err, "Message", key, "foo")

	var lf = logf.Log
	lf.Info("Message", "UpperCamelCase", "foo") // want `structured logging key should be lowerCamelCase: "UpperCamelCase"`
	lf.Info("Message", "lowerCamelCase", "foo")
	lf.Info("Message", capitalizedKey, "foo") // want `structured logging key should be lowerCamelCase: "UpperCamelCase"`
	lf.Info("Message", key, "foo")

	lf.Error(err, "Message", "UpperCamelCase", "foo") // want `structured logging key should be lowerCamelCase: "UpperCamelCase"`
	lf.Error(err, "Message", "lowerCamelCase", "foo")
	lf.Error(err, "Message", capitalizedKey, "foo") // want `structured logging key should be lowerCamelCase: "UpperCamelCase"`
	lf.Error(err, "Message", key, "foo")

	var l logr.Logger
	l.Info("Message", "UpperCamelCase", "foo") // want `structured logging key should be lowerCamelCase: "UpperCamelCase"`
	l.Info("Message", "lowerCamelCase", "foo")
	l.Info("Message", capitalizedKey, "foo") // want `structured logging key should be lowerCamelCase: "UpperCamelCase"`
	l.Info("Message", key, "foo")

	l.Error(err, "Message", "UpperCamelCase", "foo") // want `structured logging key should be lowerCamelCase: "UpperCamelCase"`
	l.Error(err, "Message", "lowerCamelCase", "foo")
	l.Error(err, "Message", capitalizedKey, "foo") // want `structured logging key should be lowerCamelCase: "UpperCamelCase"`
	l.Error(err, "Message", key, "foo")
}

func asciiKey() {
	var err = errors.New("foo")
	const nonASCIIKey, key = "nonASCIIKeyÄÖÜ", "asciiKey"

	var nl notLogr
	nl.Info("Message", "nonASCIIKeyÄÖÜ", "foo")
	nl.Info("Message", "asciiKey", "foo")
	nl.Info("Message", nonASCIIKey, "foo")
	nl.Info("Message", key, "foo")

	nl.Error(err, "Message", "nonASCIIKeyÄÖÜ", "foo")
	nl.Error(err, "Message", "asciiKey", "foo")
	nl.Error(err, "Message", nonASCIIKey, "foo")
	nl.Error(err, "Message", key, "foo")

	var lf = logf.Log
	lf.Info("Message", "nonASCIIKeyÄÖÜ", "foo") // want `structured logging key should be an ASCII string: "nonASCIIKeyÄÖÜ"`
	lf.Info("Message", "asciiKey", "foo")
	lf.Info("Message", nonASCIIKey, "foo") // want `structured logging key should be an ASCII string: "nonASCIIKeyÄÖÜ"`
	lf.Info("Message", key, "foo")

	lf.Error(err, "Message", "nonASCIIKeyÄÖÜ", "foo") // want `structured logging key should be an ASCII string: "nonASCIIKeyÄÖÜ"`
	lf.Error(err, "Message", "asciiKey", "foo")
	lf.Error(err, "Message", nonASCIIKey, "foo") // want `structured logging key should be an ASCII string: "nonASCIIKeyÄÖÜ"`
	lf.Error(err, "Message", key, "foo")

	var l logr.Logger
	l.Info("Message", "nonASCIIKeyÄÖÜ", "foo") // want `structured logging key should be an ASCII string: "nonASCIIKeyÄÖÜ"`
	l.Info("Message", "asciiKey", "foo")
	l.Info("Message", nonASCIIKey, "foo") // want `structured logging key should be an ASCII string: "nonASCIIKeyÄÖÜ"`
	l.Info("Message", key, "foo")

	l.Error(err, "Message", "nonASCIIKeyÄÖÜ", "foo") // want `structured logging key should be an ASCII string: "nonASCIIKeyÄÖÜ"`
	l.Error(err, "Message", "asciiKey", "foo")
	l.Error(err, "Message", nonASCIIKey, "foo") // want `structured logging key should be an ASCII string: "nonASCIIKeyÄÖÜ"`
	l.Error(err, "Message", key, "foo")
}

func keyContainingSpace() {
	var err = errors.New("foo")
	const keyWithSpaces, key = "key with spaces", "keyWithoutSpaces"

	var nl notLogr
	nl.Info("Message", "key with spaces", "foo")
	nl.Info("Message", "keyWithoutSpaces", "foo")
	nl.Info("Message", keyWithSpaces, "foo")
	nl.Info("Message", key, "foo")

	nl.Error(err, "Message", "key with spaces", "foo")
	nl.Error(err, "Message", "keyWithoutSpaces", "foo")
	nl.Error(err, "Message", keyWithSpaces, "foo")
	nl.Error(err, "Message", key, "foo")

	var lf = logf.Log
	lf.Info("Message", "key with spaces", "foo") // want `structured logging key should not contain spaces: "key with spaces"`
	lf.Info("Message", "keyWithoutSpaces", "foo")
	lf.Info("Message", keyWithSpaces, "foo") // want `structured logging key should not contain spaces: "key with spaces"`
	lf.Info("Message", key, "foo")

	lf.Error(err, "Message", "key with spaces", "foo") // want `structured logging key should not contain spaces: "key with spaces"`
	lf.Error(err, "Message", "keyWithoutSpaces", "foo")
	lf.Error(err, "Message", keyWithSpaces, "foo") // want `structured logging key should not contain spaces: "key with spaces"`
	lf.Error(err, "Message", key, "foo")

	var l logr.Logger
	l.Info("Message", "key with spaces", "foo") // want `structured logging key should not contain spaces: "key with spaces"`
	l.Info("Message", "keyWithoutSpaces", "foo")
	l.Info("Message", keyWithSpaces, "foo") // want `structured logging key should not contain spaces: "key with spaces"`
	l.Info("Message", key, "foo")

	l.Error(err, "Message", "key with spaces", "foo") // want `structured logging key should not contain spaces: "key with spaces"`
	l.Error(err, "Message", "keyWithoutSpaces", "foo")
	l.Error(err, "Message", keyWithSpaces, "foo") // want `structured logging key should not contain spaces: "key with spaces"`
	l.Error(err, "Message", key, "foo")
}

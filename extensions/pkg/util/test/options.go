// SPDX-FileCopyrightText: 2019 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package test

import (
	"fmt"
	"strings"
)

type Flag interface {
	Slice() []string
}

func keyToFlag(key string) string {
	return fmt.Sprintf("--%s", key)
}

type intFlag struct {
	key   string
	value int
}

func (f *intFlag) Slice() []string {
	return []string{keyToFlag(f.key), fmt.Sprintf("%d", f.value)}
}

type stringFlag struct {
	key   string
	value string
}

func (f *stringFlag) Slice() []string {
	return []string{keyToFlag(f.key), f.value}
}

type boolFlag struct {
	key   string
	value bool
}

func (f *boolFlag) Slice() []string {
	var value string
	if f.value {
		value = "true"
	} else {
		value = "false"
	}

	return []string{keyToFlag(f.key), value}
}

type stringSliceFlag struct {
	key   string
	value []string
}

func (f *stringSliceFlag) Slice() []string {
	return []string{keyToFlag(f.key), strings.Join(f.value, ",")}
}

func IntFlag(key string, value int) Flag {
	return &intFlag{key, value}
}

func StringFlag(key, value string) Flag {
	return &stringFlag{key, value}
}

func BoolFlag(key string, value bool) Flag {
	return &boolFlag{key, value}
}

func StringSliceFlag(key string, value ...string) Flag {
	return &stringSliceFlag{key, value}
}

// Command is a command that has a name, a list of flags, and a list of arguments.
type Command struct {
	Name  string
	Flags []Flag
	Args  []string
}

// CommandBuilder is a builder for Command objects.
type CommandBuilder struct {
	command Command
}

// NewCommandBuilder creates and returns a new CommandBuilder with the given name.
func NewCommandBuilder(name string) *CommandBuilder {
	return &CommandBuilder{Command{Name: name}}
}

// Flags appends the given flags to this CommandBuilder.
func (c *CommandBuilder) Flags(flags ...Flag) *CommandBuilder {
	c.command.Flags = append(c.command.Flags, flags...)
	return c
}

// Args appends the given arguments to this CommandBuilder.
func (c *CommandBuilder) Args(args ...string) *CommandBuilder {
	c.command.Args = append(c.command.Args, args...)
	return c
}

// Command returns the Command that has been built by this CommandBuilder.
func (c *CommandBuilder) Command() *Command {
	return &c.command
}

// Slice returns a representation of this Command as a slice of strings.
func (c *Command) Slice() []string {
	out := []string{c.Name}
	for _, flag := range c.Flags {
		out = append(out, flag.Slice()...)
	}
	out = append(out, c.Args...)
	return out
}

// String returns a representation of this Command as a string.
func (c *Command) String() string {
	return strings.Join(c.Slice(), " ")
}

// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package flow

import (
	"slices"
)

// TaskID is an id of a task.
type TaskID string

// TaskIDs retrieves this TaskID as a singleton slice.
func (t TaskID) TaskIDs() []TaskID {
	return []TaskID{t}
}

// TaskIDs is a set of TaskID.
type TaskIDs map[TaskID]struct{}

// TaskIDs retrieves all TaskIDs as an unsorted slice.
func (t TaskIDs) TaskIDs() []TaskID {
	return t.UnsortedList()
}

// TaskIDer can produce a slice of TaskIDs.
// Default implementations of this are
// TaskIDs, TaskID and TaskIDSlice
type TaskIDer interface {
	// TaskIDs reports all TaskIDs of this TaskIDer.
	TaskIDs() []TaskID
}

// NewTaskIDs returns a new set of TaskIDs initialized
// to contain all TaskIDs of the given TaskIDers.
func NewTaskIDs(ids ...TaskIDer) TaskIDs {
	set := make(TaskIDs)
	set.Insert(ids...)
	return set
}

// Insert inserts the TaskIDs of all TaskIDers into
// this TaskIDs.
func (t TaskIDs) Insert(iders ...TaskIDer) TaskIDs {
	for _, ider := range iders {
		for _, id := range ider.TaskIDs() {
			t[id] = struct{}{}
		}
	}
	return t
}

// InsertIf inserts the TaskIDs of all TaskIDers into
// this TaskIDs if the given condition evaluates to true.
func (t TaskIDs) InsertIf(condition bool, iders ...TaskIDer) TaskIDs {
	if condition {
		return t.Insert(iders...)
	}
	return t
}

// Delete deletes the TaskIDs of all TaskIDers from
// this TaskIDs.
func (t TaskIDs) Delete(iders ...TaskIDer) TaskIDs {
	for _, ider := range iders {
		for _, id := range ider.TaskIDs() {
			delete(t, id)
		}
	}
	return t
}

// Len returns the amount of TaskIDs this contains.
func (t TaskIDs) Len() int {
	return len(t)
}

// Has checks if the given TaskID is present in this set.
func (t TaskIDs) Has(id TaskID) bool {
	_, ok := t[id]
	return ok
}

// Copy makes a deep copy of this TaskIDs.
func (t TaskIDs) Copy() TaskIDs {
	out := make(TaskIDs, len(t))
	for k := range t {
		out[k] = struct{}{}
	}
	return out
}

// UnsortedList returns the elements of this in an unordered slice.
func (t TaskIDs) UnsortedList() TaskIDSlice {
	out := make([]TaskID, 0, len(t))
	for k := range t {
		out = append(out, k)
	}
	return out
}

// List returns the elements of this in an ordered slice.
func (t TaskIDs) List() TaskIDSlice {
	out := make(TaskIDSlice, 0, len(t))
	for k := range t {
		out = append(out, k)
	}

	slices.Sort(out)
	return out
}

// UnsortedStringList returns the elements of this in an unordered string slice.
func (t TaskIDs) UnsortedStringList() []string {
	out := make([]string, 0, len(t))
	for k := range t {
		out = append(out, string(k))
	}
	return out
}

// StringList returns the elements of this in an ordered string slice.
func (t TaskIDs) StringList() []string {
	out := t.UnsortedStringList()
	slices.Sort(out)
	return out
}

// TaskIDSlice is a slice of TaskIDs.
type TaskIDSlice []TaskID

// TaskIDs returns this as a slice of TaskIDs.
func (t TaskIDSlice) TaskIDs() []TaskID {
	return t
}

func (t TaskIDSlice) Len() int {
	return len(t)
}

func (t TaskIDSlice) Less(i1, i2 int) bool {
	return t[i1] < t[i2]
}

func (t TaskIDSlice) Swap(i1, i2 int) {
	t[i1], t[i2] = t[i2], t[i1]
}

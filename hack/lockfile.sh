# SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
#
# SPDX-License-Identifier: Apache-2.0

# Try to acquire a lockfile for a subsequent script part.
# E.g. to write a imagevector override patch sequentially as
# else skaffold's concurrent execution can cause conflicting writes.
acquire_lockfile() {
  LOCKFILE=${1-/tmp/gardener-ci.lock}
  LOCKFILE_DEST=""
  # Try to acquire the lock in a loop
  while ! ln -s "$$" "$LOCKFILE" 2>/dev/null; do
    OLD_LOCKFILE_DEST=$LOCKFILE_DEST
    LOCKFILE_DEST=$(readlink $LOCKFILE)
    if [[ $OLD_LOCKFILE_DEST == $LOCKFILE_DEST ]]; then
      echo "Lock file ($LOCKFILE) has not changed recently. Proceeding..."
      break
    fi
    echo "Another instance ($LOCKFILE_DEST, lock file $LOCKFILE) is running. Waiting..."
    sleep 1  # Wait before retrying
  done
  # Ensure lock is removed on exit
  trap 'rm -f "$LOCKFILE"' EXIT
}

#!/usr/bin/env bash

LABEL=KUBEDEV
if ! blkid --label $LABEL >/dev/null; then
  DISK_DEVICES=$(lsblk -dbsnP -o NAME,PARTTYPE,FSTYPE,SIZE,PATH,TYPE | grep 'TYPE="disk"')
  while IFS= read -r line; do
    MATCHING_DEVICE_CANDIDATE=$(echo "$line" | grep 'PARTTYPE="".*FSTYPE="".*SIZE="{{ .kubeletDataVolumeSize }}"')
    if [ -z "$MATCHING_DEVICE_CANDIDATE" ]; then
      continue
    fi

    # Skip device if it's already mounted.
    DEVICE_NAME=$(echo "$MATCHING_DEVICE_CANDIDATE" | cut -f2 -d\")
    DEVICE_MOUNTS=$(lsblk -dbsnP -o NAME,MOUNTPOINT,TYPE | grep -e "^NAME=\"$DEVICE_NAME.*\".*TYPE=\"part\"$")
    if echo "$DEVICE_MOUNTS" | awk '{print $2}' | grep "MOUNTPOINT=\"\/.*\"" > /dev/null; then
      continue
    fi

    TARGET_DEVICE_NAME="$DEVICE_NAME"
    break
  done <<< "$DISK_DEVICES"

  if [ -z "$TARGET_DEVICE_NAME" ]; then
    echo "No kubelet data device found"
    exit 1
  fi

  echo "Matching kubelet data device by size : {{ .kubeletDataVolumeSize }}"
  echo "detected kubelet data device $TARGET_DEVICE_NAME"
  mkfs.ext4 -L $LABEL -O quota -E lazy_itable_init=0,lazy_journal_init=0,quotatype=usrquota:grpquota:prjquota  /dev/$TARGET_DEVICE_NAME
  echo "formatted and labeled data device $TARGET_DEVICE_NAME"
  mkdir /tmp/varlibkubeletcp
  mount LABEL=$LABEL /tmp/varlibkubeletcp
  echo "mounted temp copy dir on data device $TARGET_DEVICE_NAME"
  cp -a /var/lib/kubelet/* /tmp/varlibkubeletcp/
  umount /tmp/varlibkubeletcp
  echo "copied /var/lib/kubelet to data device $TARGET_DEVICE_NAME"
  mount LABEL=$LABEL /var/lib/kubelet -o defaults,prjquota,errors=remount-ro
  echo "mounted /var/lib/kubelet on data device $TARGET_DEVICE_NAME"
fi

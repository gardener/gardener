#!/bin/bash
set -o errexit

function error() {
    exit_code=$?
    echo "${BASH_COMMAND} failed, exit code $exit_code"
}

trap error ERR

tune2fs -O large_dir $(mount | gawk '{if($3=="/data") {print $1}}')

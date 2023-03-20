#!/bin/sh -e
# Copyright 2023 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file.
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#     http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.

trap "kill -s INT 1" TERM
apk add conntrack-tools
sleep 120 & wait
date
# conntrack example:
# tcp      6 113 SYN_SENT src=21.73.193.93 dst=21.71.0.65 sport=1413 dport=443 \
#   [UNREPLIED] src=21.71.0.65 dst=21.73.193.93 sport=443 dport=1413 mark=0 use=1
eval "$(
  conntrack -L -p tcp --state SYN_SENT \
  | sed 's/=/ /g'                      \
  | awk '$6 !~ /^10\./ &&
         $8 !~ /^10\./ &&
         $6  == $17    &&
         $8  == $15    &&
         $10 == $21    &&
         $12 == $19 {
           printf "conntrack -D -p tcp -s %s --sport %s -d %s --dport %s;\n",
                                          $6,        $10,  $8,        $12}'
)"
while true; do
  date
  sleep 3600 & wait
done

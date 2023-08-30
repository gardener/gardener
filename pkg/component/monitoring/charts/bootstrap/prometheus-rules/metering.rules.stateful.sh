#!/bin/bash
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


for NAME in cpu_usage cpu_requests memory_usage working_set_memory memory_requests network_transmit network_receive persistent_volume_usage; do
cat <<EOF
# - metering  :$NAME                   :sum_by_namespace                       :sum_over_time
# - metering  :$NAME                   :sum_by_namespace                       :avg_over_time
# - metering  :$NAME                   :sum_by_namespace                       :avg_over_time             :this_month
EOF
done

echo

for NAME in cpu_usage cpu_requests memory_usage working_set_memory memory_requests network_transmit network_receive persistent_volume_usage; do
cat <<EOF
  - record: metering:$NAME:sum_by_namespace:sum_over_time
    expr: |2
        metering:$NAME:sum_by_namespace
      +
        (
            last_over_time(metering:$NAME:sum_by_namespace:sum_over_time[10m])
          or
            metering:$NAME:sum_by_namespace * 0
        )

  - record: metering:$NAME:sum_by_namespace:avg_over_time
    expr: |2
          metering:$NAME:sum_by_namespace:sum_over_time * 60
        /
          (metering:memory_usage_seconds != 0)
      or
        metering:$NAME:sum_by_namespace:sum_over_time


  - record: metering:$NAME:sum_by_namespace:avg_over_time:this_month
    expr: |2
        metering:$NAME:sum_by_namespace:avg_over_time
      or
          last_over_time(metering:$NAME:sum_by_namespace:avg_over_time:this_month[10m])
        + on (year, month) group_left ()
          _year_month2

EOF
done

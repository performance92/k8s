#!/bin/bash

# Copyright 2015 The Kubernetes Authors All rights reserved.
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

function ensure-basic-networking() {
}

function set-kube-env() {
  local kube_env_yaml="${INSTALL_DIR}/kube_env.yaml"

  # kube-env has all the environment variables we care about, in a flat yaml format
  eval "$(python -c '
import pipes,sys,yaml

for k,v in yaml.load(sys.stdin).iteritems():
  print("""readonly {var}={value}""".format(var = k, value = pipes.quote(str(v))))
  print("""export {var}""".format(var = k))
  ' < """${kube_env_yaml}""")"
}

function remove-docker-artifacts() {
}

# Finds the master PD device
find-master-pd() {
  echo "Waiting for master pd to be attached"
  attempt=0
  while true; do
    echo Attempt "$(($attempt+1))" to check for /dev/xvdb
    if [[ -e /dev/xvdb ]]; then
      echo "Found /dev/xvdb"
      MASTER_PD_DEVICE="/dev/xvdb"
      break
    fi
    attempt=$(($attempt+1))
    sleep 1
  done
}

function fix-apt-sources() {
}

#+GCE
function salt-master-role() {
  cat <<EOF >/etc/salt/minion.d/grains.conf
grains:
  roles:
    - kubernetes-master
  cloud: aws
EOF

  # If the kubelet on the master is enabled, give it the same CIDR range
  # as a generic node.
  if [[ ! -z "${KUBELET_APISERVER:-}" ]] && [[ ! -z "${KUBELET_CERT:-}" ]] && [[ ! -z "${KUBELET_KEY:-}" ]]; then
    cat <<EOF >>/etc/salt/minion.d/grains.conf
  kubelet_api_servers: '${KUBELET_APISERVER}'
  cbr-cidr: 10.123.45.0/30
EOF
  else
    # If the kubelet is running disconnected from a master, give it a fixed
    # CIDR range.
    cat <<EOF >>/etc/salt/minion.d/grains.conf
  cbr-cidr: ${MASTER_IP_RANGE}
EOF
  fi
  if [[ ! -z "${RUNTIME_CONFIG:-}" ]]; then
    cat <<EOF >>/etc/salt/minion.d/grains.conf
  runtime_config: '$(echo "$RUNTIME_CONFIG" | sed -e "s/'/''/g")'
EOF
  fi
}

function salt-node-role() {
  cat <<EOF >/etc/salt/minion.d/grains.conf
grains:
  roles:
    - kubernetes-pool
  cbr-cidr: 10.123.45.0/30
  cloud: aws
  api_servers: '${KUBERNETES_MASTER_NAME}'
EOF
}


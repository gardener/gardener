data "local_file" "ssh_key" {
  filename = pathexpand(var.ssh_key)
}

resource "google_compute_instance" "dev-box" {
  name           = var.name
  machine_type   = var.machine_type
  zone           = var.zone
  can_ip_forward = true
  desired_status = var.desired_status

  boot_disk {
    initialize_params {
      image = "projects/ubuntu-os-cloud/global/images/ubuntu-2204-jammy-v20220924"

      type  = "pd-ssd"
      size  = 100
    }
  }

  tags = ["allow-ssh"]

  network_interface {
    subnetwork = google_compute_subnetwork.dev-subnetwork.name
    stack_type = "IPV4_IPV6"
    access_config {}
  }

  metadata = {
    ssh-keys = "${var.user}:${data.local_file.ssh_key.content}"
  }

  metadata_startup_script = <<EOS
cd /home/${var.user}
startup_script_done_file=.startup_script_done

cat >>.bashrc <<EOF

export PATH="/home/${var.user}/go/src/github.com/gardener/gardener/hack/tools/bin:\$PATH"
alias k=kubectl
EOF

cat >>start-gardener-dev.sh <<EOF
#!/usr/bin/env bash
# guide the user when logging in
if ! [ -e $startup_script_done_file ] ; then
  until [ -e $startup_script_done_file ] ; do
    echo "Required development tools are being installed and configured. Waiting 5 more seconds..."
    sleep 5
  done
  echo "Please reconnect your SSH session to reload group membership (required for docker commands)"
  exit
fi
echo "All required development tools are installed and configured. Bringing you to the gardener/gardener directory."
cd ~/go/src/github.com/gardener/gardener
exec \$SHELL
EOF
chmod +x start-gardener-dev.sh

sudo -u ${var.user} mkdir -p go/src/github.com/gardener/gardener
apt update
apt install -y make docker.io golang jq
gpasswd -a ${var.user} docker

touch $startup_script_done_file
EOS

  # required for some projects by organization policy
  shielded_instance_config {
    enable_secure_boot = true
  }
}

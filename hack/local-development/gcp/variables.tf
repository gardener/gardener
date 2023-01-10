variable "serviceaccount_file" {
  type = string
}

variable "project" {
  type = string
}

variable "region" {
  type    = string
  default = "europe-west3"
}

variable "zone" {
  type    = string
  default = "europe-west3-c"
}

variable "user" {
  type = string
}

variable "name" {
  type = string
}

variable "machine_type" {
  type    = string
  default = "n2d-standard-8"
}

variable "ssh_key" {
  type    = string
  default = "~/.ssh/id_rsa.pub"
}

variable "desired_status" {
  type    = string
  default = "RUNNING"
}

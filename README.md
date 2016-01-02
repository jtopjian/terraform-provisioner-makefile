# terraform-provisioner-makefile

A Makefile / makefile provisioner for Terraform

## Usage

```hcl
provisioner "makefile" {
  directory = "~/infrastructure"
  target = "provision"
  variables {
    HOSTNAME = "${openstack_compute_instance_v2.scratch.name}"
  }
}
```

## Attributes

* `directory`: Change to this directory and run `makefile`.
* `target`: The Makefile target / task.
* `variables`: A list of key/value pairs that will be passed in as variables to the Makefile

## Installation

1. Grab the latest release from the [releases](https://github.com/jtopjian/terraform-provisioner-makefile/releases) page.
2. Copy the binary to the same location as the other Terraform executables.

## Building

```shell
$ go get github.com/jtopjian/terraform-provisioner-makefile
$ cd $GOPATH/src/github.com/jtopjian/terraform-provisioner-makefile
$ go build -v -o ~/path/to/terraform/terraform-provisioner-makefile .
```

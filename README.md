# Linsk

Linsk is a utility that allows you to access Linux-native file system infrastructure, including LVM and LUKS on Windows and macOS. Unlike other solutions created to access Linux filesystems on unsupported operating systems, Linsk does not reimplement any file system. Instead, Linsk utilizes a lightweight Alpine Linux VM (~130 MB only) combined with network share technologies like SMB, AFP, and FTP.

Because Linsk uses a native Linux VM, there are no limitations on what you can access. Anything that works on Linux will work under Linsk too (hence the Linux+Disk name).

# Supported platforms

## CPU architectures
Linsk natively supports both amd64 (aka x86_64, Intel, AMD, etc.) and arm64 (aka aarch64, Apple M1/M2, and others). Although Linsk uses a virtual machine, CPU is never emulated but hardware accelerators like HVF (macOS), WHPX (Windows), and KVM (Linux) are used.

## Operating systems

* Windows
* macOS
* Linux (for development purposes, mostly)

## Network file share backends

Linsk relies on network file shares to expose files to the host machine. Below are the types of network shares Linsk supports:

* SMB - The default for Windows.
* AFP - The default for macOS.
* FTP - An alternative backend.

# Installation
//TODO

# Contributing

//TODO: Make it a separate .md file.

Contributions are the most welcome. Anything including small fixes is greatly appreciated.

Please keep your new code consistent with the existing and follow the best practices writing Go code. Before committing, please make sure that your code passes the linting & security scanning suite.

To install linting tools, execute the following commands:
```sh
make lint-deps
make security-check-deps
```

After that, you will be able to run the linter and the security scanning tool.
```sh
make lint
make security-check
```

# Copyright and License

Copyright (c) 2023 The Linsk Authors.

Provided under the GNU General Public License v3. The copy of the license is included in the LICENSE file.

Use this `git` command to view the list of Linsk Authors:
```sh
git log --format='%aN <%aE>' | sort -uf
```

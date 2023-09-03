# Linsk on Linux

Yes, you read that right. For development purposes, Linsk remains 100% supported natively on Linux as well as on macOS and Windows. This makes it possible to lead the development on Linux without having to compromise the annoyance of any other operating systems.

# Prerequisites

## QEMU

On Ubuntu (and probably on any other Debian-based distro), you can install the required `qemu-system-$(arch)` binary by running the following:
```sh
apt install qemu-system
```

## Go v1.21 or higher

There are many installation options, but so far, the most convenient way to install it is to do so from the official website: https://go.dev/doc/install.

Since Linsk is written in Go (Golang), you will need to have a working Go environment to compile it.

# Build from Source
Clone the repository using `git` and run `go build` to build the Linsk binary.

```sh
git clone https://github.com/AlexSSD7/linsk
cd linsk
go build
```

After that is done, you will be able to find the `linsk` binary in the same directory you ran `go build` in.
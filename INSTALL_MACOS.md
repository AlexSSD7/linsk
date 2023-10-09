# Linsk installation instructions for macOS

Here you will find the instructions on how you can install Linsk on macOS.

# Prerequisites

Linsk aims to have minimal dependencies. In this section, you can find the instructions to install the required dependencies.

## QEMU
QEMU is what Linsk uses to run virtual machines.

The easiest way to install QEMU on macOS is to use `brew` package manager.
```sh
brew install qemu
```

## (Optional) Go v1.21 or higher
**OPTIONAL:** You need to install Go only if you want to use `go install` installation method or build Linsk from the bare Git repository.

You can find the installer on Go's official website: https://go.dev/dl/.

# Installation

## Using Go's `go install`
Assuming that you have an existing Go installation, you should be able to access the `go install` command which will build the project from source and put it to `$GOPATH/bin` directory. By default, `$GOPATH` is `$HOME/go`.

You can run the following command to build and install Linsk:
```sh
go install github.com/AlexSSD7/linsk@latest
```

After that, you should be able to run `linsk`, or `~/go/bin/linsk` if you have not added `~/go/bin` to `$PATH`.

## Package managers

TODO.

## Prebuilt binaries

You can find prebuilt binaries in [Linsk GitHub Releases](https://github.com/AlexSSD7/linsk/releases).

## Build from Source
Clone the repository using `git` and run `go build` to build the Linsk binary.

```sh
git clone https://github.com/AlexSSD7/linsk
cd linsk
go build
```

After that is done, you will be able to find the `linsk` binary in the same directory you ran `go build` in.
# Linsk installation instructions for Windows

Here you will find the instructions on how you can install Linsk on Windows.

# Prerequisites

Linsk aims to have minimal dependencies. In this section, you can find the instructions to install the required dependencies.

## QEMU
QEMU powers the Linsk's virtual machine.

The easiest way to install QEMU on Windows is to use the official installer binaries. You can find them here: https://www.qemu.org/download/#windows.

## OpenVPN Tap Networking Drivers
Linsk takes use of OpenVPN's tap networking drivers to allow for direct host-VM communication over a virtual network.

The easiest way to get these drivers is to use OpenVPN Community installer: https://openvpn.net/community-downloads/.

## (Optional) Go
**OPTIONAL:** You need to install Go only if you want to use `go install` installation method or build Linsk from the bare Git repository.

You can find the installer on Go's official website: https://go.dev/dl/.

# Installation

## Using Go's `go install`

Assuming that you have an existing Go installation, you should be able to access the `go install` command which will build the project from source and put it to `%GOPATH%\bin` folder. By default, `%GOPATH%` is `%USERPROFILE%\go`.

You can run the following command to build and install Linsk:
```sh
go install github.com/AlexSSD7/linsk
```

After that, you should be able to run `linsk`, or `%USERPROFILE%\go\bin\linsk.exe` if you have not added `%USERPROFILE%\go\bin` to PATH.

## Package managers

//TODO.

## Prebuilt binaries

//TODO.

## Build from Source
Clone the repository using `git` and run `go build` to build the Linsk binary.

```sh
git clone https://github.com/AlexSSD7/linsk
cd linsk
go build
```

After that is done, you will be able to find the `linsk.exe` binary in the same folder you ran `go build` in.
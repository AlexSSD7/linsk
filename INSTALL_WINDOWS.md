# Linsk installation instructions for Windows

Here you will find the instructions on how you can install Linsk on Windows.

# Prerequisites

Linsk aims to have minimal dependencies. In this section, you can find the instructions to install the required dependencies.

## Enable virtualization

You will need to enable virtualization in your OS, and in most cases in BIOS too. This is required to run a virtual machine on your computer. You can find the instructions how to do it here: https://support.microsoft.com/en-us/windows/enable-virtualization-on-windows-11-pcs-c5578302-6e43-4b4b-a449-8ced115f58e1.

## QEMU
QEMU is what Linsk uses to run virtual machines.

The easiest way to install QEMU on Windows is to use the official installer binaries. You can find them here: https://www.qemu.org/download/#windows.

After the installation is complete, you will need to add `C:\Program Files\qemu` to PATH. Here's a guide: https://www.howtogeek.com/118594/how-to-edit-your-system-path-for-easy-command-line-access/.

## OpenVPN Tap Networking Drivers
Linsk takes use of OpenVPN's tap networking drivers to allow for direct host-VM communication over a virtual network.

The easiest way to get these drivers is to use OpenVPN Community installer: https://openvpn.net/community-downloads/.

## (Optional) Go v1.21 or higher
**OPTIONAL:** You need to install Go only if you want to use `go install` installation method or build Linsk from the bare Git repository.

You can find the installer on Go's official website: https://go.dev/dl/.

# Installation

## Using Go's `go install`

Assuming that you have an existing Go installation, you should be able to access the `go install` command which will build the project from source and put it to `%GOPATH%\bin` folder. By default, `%GOPATH%` is `%USERPROFILE%\go`.

You can run the following command to build and install Linsk:
```sh
go install github.com/AlexSSD7/linsk@latest
```

After that, you should be able to run `linsk`, or `%USERPROFILE%\go\bin\linsk.exe` if you have not added `%USERPROFILE%\go\bin` to PATH.

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

After that is done, you will be able to find the `linsk.exe` binary in the same folder you ran `go build` in.
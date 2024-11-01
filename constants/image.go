// Linsk - A utility to access Linux-native file systems on non-Linux operating systems.
// Copyright (c) 2023 The Linsk Authors.
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
// GNU General Public License for more details.
//
// You should have received a copy of the GNU General Public License
// along with this program. If not, see <https://www.gnu.org/licenses/>.

package constants

import (
	"runtime"

	"github.com/AlexSSD7/linsk/utils"
)

const baseAlpineVersionMajor = "3.20"
const baseAlpineVersionMinor = "3"
const baseAlpineVersionCombined = baseAlpineVersionMajor + "." + baseAlpineVersionMinor

const LinskVMImageVersion = "1"

var baseAlpineArch string
var baseImageURL string
var alpineBaseImageHash []byte

func init() {
	baseAlpineArch = "x86_64"
	alpineBaseImageHash = utils.MustDecodeHex("81df854fbd7327d293c726b1eeeb82061d3bc8f5a86a6f77eea720f6be372261")
	if runtime.GOARCH == "arm64" {
		baseAlpineArch = "aarch64"
		alpineBaseImageHash = utils.MustDecodeHex("dbd0c2eaa0bfa39e18d075dae07760a9055ffdee0a338c8a35059413b0f76fec")
	}

	baseImageURL = "https://dl-cdn.alpinelinux.org/alpine/v" + baseAlpineVersionMajor + "/releases/" + baseAlpineArch + "/alpine-virt-" + baseAlpineVersionCombined + "-" + baseAlpineArch + ".iso"
}

func GetAlpineBaseImageURL() string {
	return baseImageURL
}

func GetAlpineBaseImageTags() string {
	return baseAlpineVersionCombined + "-" + baseAlpineArch
}

func GetVMImageTags() string {
	return GetAlpineBaseImageTags() + "-linsk" + LinskVMImageVersion
}

func GetAlpineBaseImageFileName() string {
	return "alpine-" + GetAlpineBaseImageTags() + ".img"
}

func GetAlpineBaseImageHash() []byte {
	// Making a copy so that remote caller cannot modify the original variable.
	tmp := make([]byte, len(alpineBaseImageHash))
	copy(tmp, alpineBaseImageHash)
	return tmp
}

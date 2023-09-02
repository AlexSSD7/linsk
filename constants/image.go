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

const baseAlpineVersionMajor = "3.18"
const baseAlpineVersionMinor = "3"
const baseAlpineVersionCombined = baseAlpineVersionMajor + "." + baseAlpineVersionMinor

const LinskVMImageVersion = "1"

var baseAlpineArch string
var baseImageURL string
var alpineBaseImageHash []byte

func init() {
	baseAlpineArch = "x86_64"
	alpineBaseImageHash = utils.MustDecodeHex("925f6bc1039a0abcd0548d2c3054d54dce31cfa03c7eeba22d10d85dc5817c98")
	if runtime.GOARCH == "arm64" {
		baseAlpineArch = "aarch64"
		alpineBaseImageHash = utils.MustDecodeHex("c94593729e4577650d9e73ada28e3cbe56964ab2a27240364f8616e920ed6d4e")
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

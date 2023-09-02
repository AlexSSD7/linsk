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

import "github.com/AlexSSD7/linsk/utils"

const aarch64EFIImageBZ2URL = "https://github.com/qemu/qemu/raw/86305e864191123dcf87c3af639fddfc59352ac6/pc-bios/edk2-aarch64-code.fd.bz2"
const aarch64EFIImageName = "edk2-aarch64-code.fd"

var aarch64EFIImageHash []byte

func init() {
	aarch64EFIImageHash = utils.MustDecodeHex("f7f2c02853fda64cad31d4ab95ef636a7c50aac4829290e7b3a73b17d3483fc1")
}

func GetAarch64EFIImageName() string {
	return aarch64EFIImageName
}

func GetAarch64EFIImageBZ2URL() string {
	return aarch64EFIImageBZ2URL
}

func GetAarch64EFIImageHash() []byte {
	// Making a copy so that remote caller cannot modify the original variable.
	tmp := make([]byte, len(aarch64EFIImageHash))
	copy(tmp, aarch64EFIImageHash)
	return tmp
}

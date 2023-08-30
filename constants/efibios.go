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

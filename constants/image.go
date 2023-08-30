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

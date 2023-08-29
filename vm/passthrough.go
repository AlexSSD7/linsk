package vm

type USBDevicePassthroughConfig struct {
	VendorID  uint16
	ProductID uint16
}

type BlockDevicePassthroughConfig struct {
	Path string
}

type PassthroughConfig struct {
	USB   []USBDevicePassthroughConfig
	Block []BlockDevicePassthroughConfig
}

package share

import (
	"github.com/AlexSSD7/linsk/nettap"
	"github.com/AlexSSD7/linsk/vm"
)

type NetTapRuntimeContext struct {
	Manager *nettap.TapManager
	Name    string
	Net     nettap.TapNet
}

type VMShareOptions struct {
	Ports     []vm.PortForwardingRule
	EnableTap bool
}

type VMShareContext struct {
	Instance    *vm.VM
	FileManager *vm.FileManager
	NetTapCtx   *NetTapRuntimeContext
}

package share

import "context"

type NewBackendFunc func(uc *UserConfiguration) (Backend, *VMShareOptions, error)

type Backend interface {
	Apply(ctx context.Context, sharePWD string, vc *VMShareContext) (string, error)
}

var backends = map[string]NewBackendFunc{
	"ftp": NewFTPBackend,
	"smb": NewSMBBackend,
}

// Will return nil if no backend is found.
func GetBackend(id string) NewBackendFunc {
	return backends[id]
}

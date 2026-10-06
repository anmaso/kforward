package forward

import (
	"github.com/amarin/kforward/internal/discovery"
	"github.com/amarin/kforward/internal/state"
)

func Recreate(f state.Forward, target discovery.Target, kubeContext string) (*state.Forward, error) {
	_ = state.RemoveForward(f.FilePath)

	mapping := Mapping{
		Local:  f.LocalPort,
		Remote: int32(f.RemotePort),
	}
	return Start(target, mapping, kubeContext)
}

package biz

import (
	"taksa-platform-dm/internal/conf"
	"taksa-platform-dm/internal/storage"

	"github.com/google/wire"
)

// ProviderSet is biz providers.
var ProviderSet = wire.NewSet(
	NewAuthUsecase,
	NewInstanceUsecase,
	ProvideDeviceUsecase,
	NewActionUsecase,
)

// ProvideDeviceUsecase provides DeviceUsecase with deployment config
func ProvideDeviceUsecase(store storage.Store, authUc *AuthUsecase, deployConf *conf.Deployment) *DeviceUsecase {
	baseURL := ""
	dockerImage := ""
	if deployConf != nil {
		baseURL = deployConf.BaseUrl
		dockerImage = deployConf.UmhCoreDockerImage
	}
	return NewDeviceUsecase(store, authUc, baseURL, dockerImage)
}

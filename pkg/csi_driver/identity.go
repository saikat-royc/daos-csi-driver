package driver

import (
	csi "github.com/container-storage-interface/spec/lib/go/csi"
	"golang.org/x/net/context"
)

type identityServer struct {
	driver *DaosDriver
}

func newIdentityServer(driver *DaosDriver) csi.IdentityServer {
	return &identityServer{driver: driver}
}

func (s *identityServer) GetPluginInfo(_ context.Context, _ *csi.GetPluginInfoRequest) (*csi.GetPluginInfoResponse, error) {
	return &csi.GetPluginInfoResponse{
		Name:          s.driver.config.Name,
		VendorVersion: s.driver.config.Version,
	}, nil
}

func (s *identityServer) GetPluginCapabilities(_ context.Context, _ *csi.GetPluginCapabilitiesRequest) (*csi.GetPluginCapabilitiesResponse, error) {
	return &csi.GetPluginCapabilitiesResponse{
		Capabilities: []*csi.PluginCapability{},
	}, nil
}

func (s *identityServer) Probe(_ context.Context, _ *csi.ProbeRequest) (*csi.ProbeResponse, error) {
	return &csi.ProbeResponse{}, nil
}

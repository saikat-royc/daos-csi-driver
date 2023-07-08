package driver

import (
	"fmt"

	csi "github.com/container-storage-interface/spec/lib/go/csi"
	"sigs.k8s.io/daos-csi-driver/pkg/cloud_provider/clientset"

	// "github.com/googlecloudplatform/daos-csi-driver/pkg/cloud_provider/auth"
	// "github.com/googlecloudplatform/daos-csi-driver/pkg/cloud_provider/clientset"
	// "github.com/googlecloudplatform/daos-csi-driver/pkg/cloud_provider/storage"
	"k8s.io/klog/v2"
	mount "k8s.io/mount-utils"
)

const DefaultName = "daos.csi.storage.gke.io"

type DaosDriverConfig struct {
	Name    string // Driver name
	Version string // Driver version
	NodeID  string // Node name
	RunNode bool   // Run CSI node service
	// TokenManager auth.TokenManager
	Mounter      mount.Interface
	K8sClients   clientset.Interface
	SidecarImage string
}

type DaosDriver struct {
	config *DaosDriverConfig

	// CSI RPC servers
	ids csi.IdentityServer
	ns  csi.NodeServer
	cs  csi.ControllerServer

	// Plugin capabilities
	vcap  map[csi.VolumeCapability_AccessMode_Mode]*csi.VolumeCapability_AccessMode
	cscap []*csi.ControllerServiceCapability
	nscap []*csi.NodeServiceCapability
}

func NewDaosDriver(config *DaosDriverConfig) (*DaosDriver, error) {
	if config.Name == "" {
		return nil, fmt.Errorf("driver name missing")
	}
	if config.Version == "" {
		return nil, fmt.Errorf("driver version missing")
	}

	driver := &DaosDriver{
		config: config,
		vcap:   map[csi.VolumeCapability_AccessMode_Mode]*csi.VolumeCapability_AccessMode{},
	}

	vcam := []csi.VolumeCapability_AccessMode_Mode{
		csi.VolumeCapability_AccessMode_SINGLE_NODE_WRITER,
		csi.VolumeCapability_AccessMode_SINGLE_NODE_READER_ONLY,
		csi.VolumeCapability_AccessMode_MULTI_NODE_READER_ONLY,
		csi.VolumeCapability_AccessMode_MULTI_NODE_SINGLE_WRITER,
		csi.VolumeCapability_AccessMode_MULTI_NODE_MULTI_WRITER,
	}
	driver.addVolumeCapabilityAccessModes(vcam)

	// Setup RPC servers
	driver.ids = newIdentityServer(driver)
	nscap := []csi.NodeServiceCapability_RPC_Type{}
	driver.ns = newNodeServer(driver, config.Mounter)
	driver.addNodeServiceCapabilities(nscap)
	return driver, nil
}

func (driver *DaosDriver) addVolumeCapabilityAccessModes(vc []csi.VolumeCapability_AccessMode_Mode) {
	for _, c := range vc {
		klog.Infof("Enabling volume access mode: %v", c.String())
		mode := NewVolumeCapabilityAccessMode(c)
		driver.vcap[mode.Mode] = mode
	}
}

func (driver *DaosDriver) validateVolumeCapabilities(caps []*csi.VolumeCapability) error {
	if len(caps) == 0 {
		return fmt.Errorf("volume capabilities must be provided")
	}

	for _, c := range caps {
		if err := driver.validateVolumeCapability(c); err != nil {
			return err
		}
	}

	return nil
}

func (driver *DaosDriver) validateVolumeCapability(c *csi.VolumeCapability) error {
	if c == nil {
		return fmt.Errorf("volume capability must be provided")
	}

	// Validate access mode
	accessMode := c.GetAccessMode()
	if accessMode == nil {
		return fmt.Errorf("volume capability access mode not set")
	}
	if driver.vcap[accessMode.Mode] == nil {
		return fmt.Errorf("driver does not support access mode: %v", accessMode.Mode.String())
	}

	// Validate access type
	accessType := c.GetAccessType()
	if accessType == nil {
		return fmt.Errorf("volume capability access type not set")
	}
	mountType := c.GetMount()
	if mountType == nil {
		return fmt.Errorf("driver only supports mount access type volume capability")
	}

	return nil
}

func (driver *DaosDriver) addNodeServiceCapabilities(nl []csi.NodeServiceCapability_RPC_Type) {
	nsc := []*csi.NodeServiceCapability{}
	for _, n := range nl {
		klog.Infof("Enabling node service capability: %v", n.String())
		nsc = append(nsc, NewNodeServiceCapability(n))
	}
	driver.nscap = nsc
}

func (driver *DaosDriver) Run(endpoint string) {
	klog.Infof("Running driver: %v", driver.config.Name)

	s := NewNonBlockingGRPCServer()
	s.Start(endpoint, driver.ids, driver.cs, driver.ns)
	s.Wait()
}

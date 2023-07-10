package main

import (
	"flag"
	"os"

	"k8s.io/klog/v2"
	"k8s.io/mount-utils"
	"sigs.k8s.io/daos-csi-driver/pkg/cloud_provider/clientset"
	driver "sigs.k8s.io/daos-csi-driver/pkg/csi_driver"
	csimounter "sigs.k8s.io/daos-csi-driver/pkg/csi_mounter"
)

var (
	endpoint = flag.String("endpoint", "unix:/tmp/csi.sock", "CSI endpoint")
	nodeID   = flag.String("nodeid", "", "node id")
	// runController    = flag.Bool("controller", false, "run controller service")
	runNode        = flag.Bool("node", false, "run node service")
	kubeconfigPath = flag.String("kubeconfig-path", "", "The kubeconfig path.")
	// sidecarImage     = flag.String("sidecar-image", "", "The gcsfuse sidecar container image.")
	// identityPool     = flag.String("identity-pool", "", "The Identity Pool to authenticate with GCS API.")
	// identityProvider = flag.String("identity-provider", "", "The Identity Provider to authenticate with GCS API.")

	// These are set at compile time.
	version = "unknown"
)

func main() {
	klog.InitFlags(nil)
	flag.Parse()

	clientset, err := clientset.New(*kubeconfigPath)
	if err != nil {
		klog.Fatal("Failed to configure k8s client")
	}

	// meta, err := metadata.NewMetadataService(clientset)
	// if err != nil {
	// 	klog.Fatalf("Failed to set up metadata service: %v", err)
	// }

	// tm := auth.NewTokenManager(meta, clientset)
	// ssm, err := storage.NewGCSServiceManager()
	// if err != nil {
	// 	klog.Fatalf("Failed to set up storage service manager: %v", err)
	// }

	var mounter mount.Interface
	if *runNode {
		if *nodeID == "" {
			klog.Fatalf("NodeID cannot be empty for node service")
		}

		mounter, err = csimounter.New("")
		if err != nil {
			klog.Fatalf("Failed to prepare CSI mounter: %v", err)
		}
	}

	if err != nil {
		klog.Fatalf("Failed to initialize cloud provider: %v", err)
	}

	config := &driver.DaosDriverConfig{
		Name:    driver.DefaultName,
		Version: version,
		NodeID:  *nodeID,
		// StorageServiceManager: ssm,
		// TokenManager:          tm,
		Mounter:    mounter,
		K8sClients: clientset,
		// SidecarImage:          *sidecarImage,
	}

	gcfsDriver, err := driver.NewDaosDriver(config)
	if err != nil {
		klog.Fatalf("Failed to initialize Google Cloud Storage FUSE CSI Driver: %v", err)
	}

	klog.Infof("Running DAOS CSI driver version %v", version)
	gcfsDriver.Run(*endpoint)

	os.Exit(0)
}

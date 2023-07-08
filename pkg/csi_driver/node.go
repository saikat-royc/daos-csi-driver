package driver

import (
	"os"
	"strings"
	"time"

	csi "github.com/container-storage-interface/spec/lib/go/csi"
	"golang.org/x/net/context"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"k8s.io/klog/v2"
	mount "k8s.io/mount-utils"
	"sigs.k8s.io/daos-csi-driver/pkg/cloud_provider/clientset"
	"sigs.k8s.io/daos-csi-driver/pkg/util"
)

// NodePublishVolume VolumeContext parameters.
const (
	VolumeContextKeyServiceAccountName = "csi.storage.k8s.io/serviceAccount.name"
	//nolint:gosec
	VolumeContextKeyServiceAccountToken = "csi.storage.k8s.io/serviceAccount.tokens"
	VolumeContextKeyPodName             = "csi.storage.k8s.io/pod.name"
	VolumeContextKeyPodNamespace        = "csi.storage.k8s.io/pod.namespace"
	VolumeContextKeyEphemeral           = "csi.storage.k8s.io/ephemeral"
	VolumeContextKeyBucketName          = "bucketName"
	VolumeContextKeyMountOptions        = "mountOptions"

	UmountTimeout = time.Second * 5
)

type nodeServer struct {
	driver      *DaosDriver
	mounter     mount.Interface
	volumeLocks *util.VolumeLocks
	k8sClients  clientset.Interface
}

func newNodeServer(driver *DaosDriver, mounter mount.Interface) csi.NodeServer {
	return &nodeServer{
		driver:      driver,
		mounter:     mounter,
		k8sClients:  driver.config.K8sClients,
		volumeLocks: util.NewVolumeLocks(),
	}
}

func (s *nodeServer) NodeGetInfo(_ context.Context, _ *csi.NodeGetInfoRequest) (*csi.NodeGetInfoResponse, error) {
	return &csi.NodeGetInfoResponse{
		NodeId: s.driver.config.NodeID,
	}, nil
}

func (s *nodeServer) NodeGetCapabilities(_ context.Context, _ *csi.NodeGetCapabilitiesRequest) (*csi.NodeGetCapabilitiesResponse, error) {
	return &csi.NodeGetCapabilitiesResponse{
		Capabilities: s.driver.nscap,
	}, nil
}

func (s *nodeServer) NodePublishVolume(ctx context.Context, req *csi.NodePublishVolumeRequest) (*csi.NodePublishVolumeResponse, error) {
	// klog.Infof("NodePublishVolume no-op success")
	daosContainerName := req.GetVolumeId()
	klog.Infof("daos container name %v", daosContainerName)
	vc := req.GetVolumeContext()
	fuseMountOptions := []string{}
	// TODO: Ready user provided options

	targetPath := req.GetTargetPath()
	if len(targetPath) == 0 {
		return nil, status.Error(codes.InvalidArgument, "NodePublishVolume target path must be provided")
	}

	if acquired := s.volumeLocks.TryAcquire(targetPath); !acquired {
		return nil, status.Errorf(codes.Aborted, util.VolumeOperationAlreadyExistsFmt, targetPath)
	}
	defer s.volumeLocks.Release(targetPath)

	klog.Infof("Got pod %s/%s from volume context", vc[VolumeContextKeyPodNamespace], vc[VolumeContextKeyPodName])
	pod, err := s.k8sClients.GetPod(ctx, vc[VolumeContextKeyPodNamespace], vc[VolumeContextKeyPodName])
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to get pod: %v", err)
	}
	klog.Infof("Got pod %s/%s from volume context", pod.Namespace, pod.Name)
	// TODO: Validate pod has sidecar injected
	// TODO: Check if sidecar needs to exit

	// Prepare the emptyDir path for the mounter to pass the file descriptor
	emptyDirBasePath, err := util.PrepareEmptyDir(targetPath, true)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to prepare emptyDir path: %v", err)
	}
	// TODO: Put an exit file to notify the sidecar container to exit

	// Check if there is any error from the sidecar container
	errMsg, err := os.ReadFile(emptyDirBasePath + "/error")
	if err != nil && !os.IsNotExist(err) {
		return nil, status.Errorf(codes.Internal, "failed to open error file %q: %v", emptyDirBasePath+"/error", err)
	}
	if err == nil && len(errMsg) > 0 {
		errMsgStr := string(errMsg)
		code := codes.Internal
		if strings.Contains(errMsgStr, "Incorrect Usage") {
			code = codes.InvalidArgument
		}

		if strings.Contains(errMsgStr, "signal: killed") {
			code = codes.ResourceExhausted
		}

		return nil, status.Errorf(code, "the sidecar container failed with error: %v", errMsgStr)
	}

	// TODO: Check if the sidecar container terminated

	// Check if the target path is already mounted
	mounted, err := s.isDirMounted(targetPath)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to check if path %q is already mounted: %v", targetPath, err)
	}

	if mounted {
		// Already mounted
		klog.V(4).Infof("NodePublishVolume succeeded on volume %q to target path %q, mount already exists.", daosContainerName, targetPath)

		return &csi.NodePublishVolumeResponse{}, nil
	}

	klog.Infof("NodePublishVolume attempting mkdir for path %q", targetPath)
	if err := os.MkdirAll(targetPath, 0o750); err != nil {
		return nil, status.Errorf(codes.Internal, "mkdir failed for path %q: %v", targetPath, err)
	}

	// Start to mount
	if err = s.mounter.Mount(daosContainerName, targetPath, "fuse", fuseMountOptions); err != nil {
		return nil, status.Errorf(codes.Internal, "failed to mount volume %q to target path %q: %v", daosContainerName, targetPath, err)
	}
	return &csi.NodePublishVolumeResponse{}, nil
}

func (s *nodeServer) NodeUnpublishVolume(_ context.Context, req *csi.NodeUnpublishVolumeRequest) (*csi.NodeUnpublishVolumeResponse, error) {
	klog.Infof("NodeUnpublishVolume no-op success")
	return &csi.NodeUnpublishVolumeResponse{}, nil
}

func (s *nodeServer) isDirMounted(targetPath string) (bool, error) {
	mps, err := s.mounter.List()
	if err != nil {
		return false, err
	}
	for _, m := range mps {
		if m.Path == targetPath {
			return true, nil
		}
	}

	return false, nil
}

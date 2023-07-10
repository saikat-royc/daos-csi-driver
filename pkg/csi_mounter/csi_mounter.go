package csimounter

import (
	"encoding/json"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"sync"
	"syscall"
	"time"

	"k8s.io/klog/v2"
	"k8s.io/mount-utils"
	sidecarmounter "sigs.k8s.io/daos-csi-driver/pkg/sidecar_mounter"
	"sigs.k8s.io/daos-csi-driver/pkg/util"
)

// Mounter provides the Cloud Storage FUSE CSI implementation of mount.Interface
// for the linux platform.
type Mounter struct {
	mount.MounterForceUnmounter
	chdirMu sync.Mutex
}

// New returns a mount.MounterForceUnmounter for the current system.
// It provides options to override the default mounter behavior.
// mounterPath allows using an alternative to `/bin/mount` for mounting.
func New(mounterPath string) (mount.Interface, error) {
	m, ok := mount.New(mounterPath).(mount.MounterForceUnmounter)
	if !ok {
		return nil, fmt.Errorf("failed to cast mounter to MounterForceUnmounter")
	}

	return &Mounter{
		m,
		sync.Mutex{},
	}, nil
}

func (m *Mounter) Mount(source string, target string, fstype string, options []string) error {
	csiMountOptions := prepareMountOptions(options)

	// Prepare the temp emptyDir path
	emptyDirBasePath, err := util.PrepareEmptyDir(target, false)
	if err != nil {
		return fmt.Errorf("failed to prepare emptyDir path: %w", err)
	}

	klog.V(4).Info("opening the device /dev/fuse")
	fd, err := syscall.Open("/dev/fuse", syscall.O_RDWR, 0o644)
	if err != nil {
		return fmt.Errorf("failed to open the device /dev/fuse: %w", err)
	}
	klog.Infof("got fd %d for /dev/fuse", fd)
	csiMountOptions = append(csiMountOptions, fmt.Sprintf("fd=%v", fd))

	klog.V(4).Info("mounting the fuse filesystem")
	err = m.MountSensitiveWithoutSystemdWithMountFlags(source, target, fstype, csiMountOptions, nil, []string{"--internal-only"})
	if err != nil {
		return fmt.Errorf("failed to mount the fuse filesystem: %w", err)
	}

	klog.V(4).Info("passing the descriptor fd %d", fd)
	// Need to change the current working directory to the temp volume base path,
	// because the socket absolute path is longer than 104 characters,
	// which will cause "bind: invalid argument" errors.
	m.chdirMu.Lock()
	exPwd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("failed to get the current directory to %w", err)
	}
	if err = os.Chdir(emptyDirBasePath); err != nil {
		return fmt.Errorf("failed to change directory to %q: %w", emptyDirBasePath, err)
	}

	klog.V(4).Info("creating a listener for the socket")
	l, err := net.Listen("unix", "./socket")
	if err != nil {
		return fmt.Errorf("failed to create the listener for the socket: %w", err)
	}

	// Change the socket ownership
	err = os.Chown(filepath.Dir(emptyDirBasePath), 65534, 65534)
	if err != nil {
		return fmt.Errorf("failed to change ownership on base of emptyDirBasePath: %w", err)
	}
	err = os.Chown(emptyDirBasePath, 65534, 65534)
	if err != nil {
		return fmt.Errorf("failed to change ownership on emptyDirBasePath: %w", err)
	}
	err = os.Chown("./socket", 65534, 65534)
	if err != nil {
		return fmt.Errorf("failed to change ownership on socket: %w", err)
	}

	// Close the listener after 15 minutes
	// TODO: properly handle the socket listener timed out
	go func(l net.Listener) {
		time.Sleep(15 * time.Minute)
		l.Close()
	}(l)

	if err = os.Chdir(exPwd); err != nil {
		return fmt.Errorf("failed to change directory to %q: %w", exPwd, err)
	}
	m.chdirMu.Unlock()

	// Prepare sidecar mounter MountConfig
	mc := sidecarmounter.MountConfig{
		DaosContainerName: source,
		DaosPoolName:      "pool1",
		FileDescriptor:    fd,
	}
	mcb, err := json.Marshal(mc)
	if err != nil {
		return fmt.Errorf("failed to marshal sidecar mounter MountConfig %v: %w", mc, err)
	}

	// Asynchronously waiting for the sidecar container to connect to the listener
	go func(l net.Listener, daosCtrName, target string, msg []byte, fd int) {
		defer syscall.Close(fd)
		defer l.Close()

		podID, volumeName, _ := util.ParsePodIDVolumeFromTargetpath(target)
		logPrefix := fmt.Sprintf("[Pod %v, Volume %v, Daos container %v]", podID, volumeName, daosCtrName)

		klog.V(4).Infof("%v start to accept connections to the listener.", logPrefix)
		a, err := l.Accept()
		if err != nil {
			klog.Errorf("%v failed to accept connections to the listener: %v", logPrefix, err)

			return
		}
		defer a.Close()

		klog.V(4).Infof("%v start to send file descriptor and mount options", logPrefix)
		if err = util.SendMsg(a, fd, msg); err != nil {
			klog.Errorf("%v failed to send file descriptor and mount options: %v", logPrefix, err)
		}

		klog.V(4).Infof("%v exiting the goroutine.", logPrefix)
	}(l, source, target, mcb, fd)

	return nil
}

func prepareMountOptions(options []string) []string {
	csiMountOptions := []string{
		"allow_other",
		"default_permissions",
		"rw",
		"nodev",
		"nosuid",
		"rootmode=40000",
		fmt.Sprintf("user_id=%d", os.Getuid()),
		fmt.Sprintf("group_id=%d", os.Getgid()),
	}

	return csiMountOptions
}

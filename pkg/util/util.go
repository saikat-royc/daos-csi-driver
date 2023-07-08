package util

import (
	"fmt"
	"net/url"
	"os"
	"regexp"

	"k8s.io/klog/v2"
)

const (
	Mb = 1024 * 1024
)

const (
	SidecarContainerName            = "daos-sidecar"
	SidecarContainerVolumeName      = "daos-tmp"
	SidecarContainerVolumeMountPath = "/daos-tmp"

	// See the nonroot user discussion: https://github.com/GoogleContainerTools/distroless/issues/443
	NobodyUID = 65534
	NobodyGID = 65534
)

func ParseEndpoint(endpoint string, cleanupSocket bool) (string, string, error) {
	u, err := url.Parse(endpoint)
	if err != nil {
		klog.Fatal(err.Error())
	}

	var addr string
	switch u.Scheme {
	case "unix":
		addr = u.Path
		if cleanupSocket {
			if err := os.Remove(addr); err != nil && !os.IsNotExist(err) {
				klog.Fatalf("Failed to remove %s, error: %s", addr, err)
			}
		}
	case "tcp":
		addr = u.Host
	default:
		klog.Fatalf("%v endpoint scheme not supported", u.Scheme)
	}

	return u.Scheme, addr, nil
}

func ParsePodIDVolumeFromTargetpath(targetPath string) (string, string, error) {
	r := regexp.MustCompile("/var/lib/kubelet/pods/(.*)/volumes/kubernetes.io~csi/(.*)/mount")
	matched := r.FindStringSubmatch(targetPath)
	if len(matched) < 3 {
		return "", "", fmt.Errorf("targetPath %v does not contain Pod ID or volume information", targetPath)
	}
	podID := matched[1]
	volume := matched[2]

	return podID, volume, nil
}

func PrepareEmptyDir(targetPath string, createEmptyDir bool) (string, error) {
	_, _, err := ParsePodIDVolumeFromTargetpath(targetPath)
	if err != nil {
		return "", fmt.Errorf("failed to parse volume name from target path %q: %w", targetPath, err)
	}

	r := regexp.MustCompile("kubernetes.io~csi/(.*)/mount")
	emptyDirBasePath := r.ReplaceAllString(targetPath, fmt.Sprintf("kubernetes.io~empty-dir/%v/.volumes/$1", SidecarContainerVolumeName))
	klog.Infof("emptyDirBasePath %v", emptyDirBasePath)
	if createEmptyDir {
		if err := os.MkdirAll(emptyDirBasePath, 0o750); err != nil {
			return "", fmt.Errorf("mkdir failed for path %q: %w", emptyDirBasePath, err)
		}
	}

	return emptyDirBasePath, nil
}

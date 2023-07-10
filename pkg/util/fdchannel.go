package util

import (
	"fmt"
	"net"
	"syscall"

	"k8s.io/klog/v2"
)

func SendMsg(via net.Conn, fd int, msg []byte) error {
	klog.V(4).Info("get the underlying socket")
	conn, ok := via.(*net.UnixConn)
	if !ok {
		return fmt.Errorf("failed to cast via to *net.UnixConn")
	}
	connf, err := conn.File()
	if err != nil {
		return err
	}
	socket := int(connf.Fd())
	defer connf.Close()

	klog.V(4).Infof("calling sendmsg...")
	rights := syscall.UnixRights(fd)

	return syscall.Sendmsg(socket, msg, rights, nil, 0)
}

func RecvMsg(via net.Conn) (int, []byte, error) {
	klog.V(4).Info("get the underlying socket")
	conn, ok := via.(*net.UnixConn)
	if !ok {
		return 0, nil, fmt.Errorf("failed to cast via to *net.UnixConn")
	}
	connf, err := conn.File()
	if err != nil {
		return 0, nil, err
	}
	socket := int(connf.Fd())
	defer connf.Close()

	klog.V(4).Info("calling recvmsg...")
	buf := make([]byte, syscall.CmsgSpace(4))
	b := make([]byte, 500)
	//nolint:dogsled
	n, _, _, _, err := syscall.Recvmsg(socket, b, buf, 0)
	if err != nil {
		return 0, nil, err
	}

	klog.V(4).Info("parsing SCM...")
	var msgs []syscall.SocketControlMessage
	msgs, err = syscall.ParseSocketControlMessage(buf)
	if err != nil {
		return 0, nil, err
	}

	klog.V(4).Info("parsing SCM_RIGHTS...")
	fds, err := syscall.ParseUnixRights(&msgs[0])
	if err != nil {
		return 0, nil, err
	}

	return fds[0], b[:n], err
}

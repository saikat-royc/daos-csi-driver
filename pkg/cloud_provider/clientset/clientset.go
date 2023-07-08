package clientset

import (
	"context"

	appsv1 "k8s.io/api/apps/v1"
	authenticationv1 "k8s.io/api/authentication/v1"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/klog/v2"
)

type Interface interface {
	GetPod(ctx context.Context, namespace, name string) (*v1.Pod, error)
	GetDaemonSet(ctx context.Context, namespace, name string) (*appsv1.DaemonSet, error)
	CreateServiceAccountToken(ctx context.Context, namespace, name string, tokenRequest *authenticationv1.TokenRequest) (*authenticationv1.TokenRequest, error)
	// GetGCPServiceAccountName(ctx context.Context, namespace, name string) (string, error)
}

type Clientset struct {
	k8sClients kubernetes.Interface
}

func New(kubeconfigPath string) (Interface, error) {
	var err error
	var rc *rest.Config
	if kubeconfigPath != "" {
		klog.V(4).Infof("using kubeconfig path %q", kubeconfigPath)
		rc, err = clientcmd.BuildConfigFromFlags("", kubeconfigPath)
	} else {
		klog.V(4).Info("using in-cluster kubeconfig")
		rc, err = rest.InClusterConfig()
	}
	if err != nil {
		klog.Fatalf("Failed to read kubeconfig: %v", err)
	}

	clientset, err := kubernetes.NewForConfig(rc)
	if err != nil {
		klog.Fatal("failed to configure k8s client")
	}

	return &Clientset{k8sClients: clientset}, nil
}

func (c *Clientset) GetPod(ctx context.Context, namespace, name string) (*v1.Pod, error) {
	pod, err := c.k8sClients.CoreV1().Pods(namespace).Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		return nil, err
	}

	return pod, nil
}

func (c *Clientset) GetDaemonSet(ctx context.Context, namespace, name string) (*appsv1.DaemonSet, error) {
	ds, err := c.k8sClients.AppsV1().DaemonSets(namespace).Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		return nil, err
	}

	return ds, nil
}

func (c *Clientset) CreateServiceAccountToken(ctx context.Context, namespace, name string, tokenRequest *authenticationv1.TokenRequest) (*authenticationv1.TokenRequest, error) {
	resp, err := c.k8sClients.
		CoreV1().
		ServiceAccounts(namespace).
		CreateToken(
			ctx,
			name,
			tokenRequest,
			metav1.CreateOptions{},
		)

	return resp, err
}

// func (c *Clientset) GetGCPServiceAccountName(ctx context.Context, namespace, name string) (string, error) {
// 	resp, err := c.k8sClients.
// 		CoreV1().
// 		ServiceAccounts(namespace).
// 		Get(
// 			ctx,
// 			name,
// 			metav1.GetOptions{},
// 		)
// 	if err != nil {
// 		return "", fmt.Errorf("failed to call Kubernetes ServiceAccount.Get API: %w", err)
// 	}

// 	return resp.Annotations["iam.gke.io/gcp-service-account"], nil
// }

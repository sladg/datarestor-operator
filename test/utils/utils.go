package utils

import (
	"context"
	"os/exec"
	"strings"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
)

// LoadClient loads a kubernetes client from kubeconfig
func LoadClient(kubeconfig string) (*kubernetes.Clientset, error) {
	config, err := clientcmd.BuildConfigFromFlags("", kubeconfig)
	if err != nil {
		return nil, err
	}

	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		return nil, err
	}

	return clientset, nil
}

// WaitForPodReady waits for a pod to be ready
func WaitForPodReady(clientset *kubernetes.Clientset, namespace, name string, timeout time.Duration) error {
	return wait.PollImmediate(time.Second, timeout, func() (bool, error) {
		pod, err := clientset.CoreV1().Pods(namespace).Get(context.TODO(), name, metav1.GetOptions{})
		if err != nil {
			return false, err
		}

		for _, condition := range pod.Status.Conditions {
			if condition.Type == corev1.PodReady && condition.Status == corev1.ConditionTrue {
				return true, nil
			}
		}

		return false, nil
	})
}

// WaitForPVCBound waits for a PVC to be bound
func WaitForPVCBound(clientset *kubernetes.Clientset, namespace, name string, timeout time.Duration) error {
	return wait.PollImmediate(time.Second, timeout, func() (bool, error) {
		pvc, err := clientset.CoreV1().PersistentVolumeClaims(namespace).Get(context.TODO(), name, metav1.GetOptions{})
		if err != nil {
			return false, err
		}

		return pvc.Status.Phase == corev1.ClaimBound, nil
	})
}

// CreateTestNamespace creates a test namespace
func CreateTestNamespace(clientset *kubernetes.Clientset, name string) error {
	namespace := &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
		},
	}

	_, err := clientset.CoreV1().Namespaces().Create(context.TODO(), namespace, metav1.CreateOptions{})
	return err
}

// DeleteTestNamespace deletes a test namespace
func DeleteTestNamespace(clientset *kubernetes.Clientset, name string) error {
	return clientset.CoreV1().Namespaces().Delete(context.TODO(), name, metav1.DeleteOptions{})
}

// Run executes a command and returns the output
func Run(cmd *exec.Cmd) (string, error) {
	output, err := cmd.CombinedOutput()
	return string(output), err
}

// LoadImageToKindClusterWithName loads a Docker image into a Kind cluster
func LoadImageToKindClusterWithName(imageName string) error {
	cmd := exec.Command("kind", "load", "docker-image", imageName)
	return cmd.Run()
}

// IsCertManagerCRDsInstalled checks if CertManager CRDs are already installed
func IsCertManagerCRDsInstalled() bool {
	cmd := exec.Command("kubectl", "get", "crd", "certificates.cert-manager.io")
	return cmd.Run() == nil
}

// InstallCertManager installs CertManager
func InstallCertManager() error {
	cmd := exec.Command("kubectl", "apply", "-f", "https://github.com/cert-manager/cert-manager/releases/download/v1.13.0/cert-manager.yaml")
	return cmd.Run()
}

// UninstallCertManager uninstalls CertManager
func UninstallCertManager() error {
	cmd := exec.Command("kubectl", "delete", "-f", "https://github.com/cert-manager/cert-manager/releases/download/v1.13.0/cert-manager.yaml")
	return cmd.Run()
}

// GetNonEmptyLines splits output into lines and filters out empty ones
func GetNonEmptyLines(output string) []string {
	lines := strings.Split(output, "\n")
	var nonEmptyLines []string
	for _, line := range lines {
		if strings.TrimSpace(line) != "" {
			nonEmptyLines = append(nonEmptyLines, strings.TrimSpace(line))
		}
	}
	return nonEmptyLines
}

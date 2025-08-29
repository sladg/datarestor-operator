package utils

import (
	"context"
	"fmt"
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

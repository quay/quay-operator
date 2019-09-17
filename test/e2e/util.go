package e2e

import (
	goctx "context"
	"fmt"
	"strings"
	"testing"
	"time"

	framework "github.com/operator-framework/operator-sdk/pkg/test"
	v1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/kubernetes"
)

// Original Source https://github.com/jaegertracing/jaeger-operator/blob/master/test/e2e/utils.go
func GetPod(namespace, namePrefix, containsImage string, kubeclient kubernetes.Interface) v1.Pod {
	pods, err := kubeclient.CoreV1().Pods(namespace).List(metav1.ListOptions{})
	if err != nil {
		return v1.Pod{}
	}
	for _, pod := range pods.Items {
		if strings.HasPrefix(pod.Name, namePrefix) {
			for _, pod := range pods.Items {
				if strings.HasPrefix(pod.Name, namePrefix) {
					for _, c := range pod.Spec.Containers {
						fmt.Printf("Found pod +%v\n", c)
						if strings.Contains(c.Image, containsImage) {
							return pod
						}
					}
				}
			}
		}
	}
	return v1.Pod{}

}

func WaitForPod(t *testing.T, f *framework.Framework, ctx *framework.TestCtx, namespace, name string, retryInterval time.Duration, timeout time.Duration) error {
	err := wait.Poll(retryInterval, timeout, func() (done bool, err error) {
		// Check if the CRD has been created
		pod := &v1.Pod{}
		err = f.Client.Get(goctx.TODO(), types.NamespacedName{Name: name, Namespace: namespace}, pod)
		if err != nil {
			if apierrors.IsNotFound(err) {
				t.Logf("Waiting for availability of %s pod\n", name)
				return false, nil
			}
			return false, err
		}
		if pod.Status.Phase == "Running" {
			return true, nil
		}
		t.Logf("Waiting for full availability of %s pod\n", name)
		return false, nil
	})
	if err != nil {
		return err
	}
	t.Logf("pod available\n")
	return nil
}

func WaitForPodWithImage(t *testing.T, f *framework.Framework, ctx *framework.TestCtx, namespace, name string, image string, retryInterval time.Duration, timeout time.Duration) error {
	err := wait.Poll(retryInterval, timeout, func() (done bool, err error) {
		fmt.Printf("Looking for pod +%v\n", name)
		// Check if the CRD has been created
		pods, err := f.KubeClient.CoreV1().Events(namespace).List(metav1.ListOptions{})
		fmt.Printf("Events +%v\n", pods)
		pod := GetPod(namespace, name, image, f.KubeClient)
		if err != nil {
			if apierrors.IsNotFound(err) {
				t.Logf("Waiting for availability of %s pod\n", name)
				return false, nil
			}
			return false, err
		}
		if pod.Status.Phase == "Running" {
			return true, nil
		}
		t.Logf("Waiting for full availability of %s pod\n", name)
		return false, nil
	})
	if err != nil {
		return err
	}
	t.Logf("pod available\n")
	return nil
}

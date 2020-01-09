package k8sutils

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"time"

	"github.com/redhat-cop/quay-operator/pkg/controller/quayecosystem/logging"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/remotecommand"
	client "sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/config"
)

func GetK8sClient(config *rest.Config) (kubernetes.Interface, error) {
	kubeclient, err := kubernetes.NewForConfig(config)

	if err != nil {
		return nil, fmt.Errorf("failed to build the kubeclient: %v", err)
	}

	return kubeclient, nil

}

func GetDeploymentStatus(operatorClient client.Client, namespace string, name string) bool {

	var timeout time.Duration = time.Duration(420) * time.Second
	var retryInterval time.Duration = time.Duration(100) * time.Second

	err := wait.Poll(retryInterval, timeout, func() (done bool, err error) {

		logging.Log.Info("Waiting for deployment", "Namespace", namespace, "Name", name, "Timeout", timeout)
		// check before watching in case the deployment is already scaled to 1
		fieldSelector := fields.OneTermEqualSelector("metadata.name", name)
		listOps := client.ListOptions{
			Namespace:     namespace,
			FieldSelector: fieldSelector,
		}
		deploymentList := &appsv1.DeploymentList{}
		err = operatorClient.List(context.TODO(), deploymentList, &listOps)

		for _, deployment := range deploymentList.Items {
			if deployment.Status.AvailableReplicas == 1 {
				logging.Log.Info("Deployment successfully scaled up", deployment.Name, deployment.Status.AvailableReplicas)
				return true, nil
			}
		}
		return false, nil
	})

	if err != nil {
		logging.Log.Error(err, "Failed to verify a successful deployment", "Name", name)
		return false
	}
	return true

}

func ExecIntoPod(k8sclient kubernetes.Interface, podName string, provisionCommand string, reason string, ns string) (bool, string, string) {

	command := []string{"/bin/bash", "-c", provisionCommand}
	logging.Log.Info("Running exec into Pod", "Reason", reason, "Pod Name", podName)

	stdout, stderr, err := RunExec(k8sclient, command, podName, ns)
	if err != nil {
		logging.Log.Info("Error exec'ing into pod", "Error", err, "Command", command)
		return false, "", ""
	}
	return true, stdout, stderr
}

func RunExec(k8sclient kubernetes.Interface, command []string, podName, namespace string) (string, string, error) {

	if k8sclient == nil {
		return "", "", fmt.Errorf("Failed to get Deployment: No k8sclient initiated")
	}

	req := k8sclient.CoreV1().RESTClient().Post().
		Resource("pods").
		Name(podName).
		Namespace(namespace).
		SubResource("exec")

	req.VersionedParams(&corev1.PodExecOptions{
		Command: command,
		Stdin:   false,
		Stdout:  true,
		Stderr:  true,
		TTY:     false,
	}, scheme.ParameterCodec)

	cfg, _ := config.GetConfig()
	exec, err := remotecommand.NewSPDYExecutor(cfg, "POST", req.URL())
	if err != nil {
		return "", "", fmt.Errorf("error while creating executor: %v", err)
	}

	var stdout, stderr bytes.Buffer
	var stdin io.Reader
	err = exec.Stream(remotecommand.StreamOptions{
		Stdin:  stdin,
		Stdout: &stdout,
		Stderr: &stderr,
		Tty:    false,
	})
	if err != nil {
		return stdout.String(), stderr.String(), err
	}

	return stdout.String(), stderr.String(), nil
}

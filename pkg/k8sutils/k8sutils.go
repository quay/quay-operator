package k8sutils

import (
	"bytes"
	"fmt"
	"io"

	"github.com/redhat-cop/quay-operator/pkg/controller/quayecosystem/logging"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/remotecommand"
	"sigs.k8s.io/controller-runtime/pkg/client/config"
)

func GetK8sClient(config *rest.Config) (kubernetes.Interface, error) {
	kubeclient, err := kubernetes.NewForConfig(config)

	if err != nil {
		return nil, fmt.Errorf("failed to build the kubeclient: %v", err)
	}

	return kubeclient, nil

}

func GetDeploymentStatus(k8sclient kubernetes.Interface, namespace string, name string) bool {

	if k8sclient == nil {
		logging.Log.Info("Failed to get Deployment: No k8sclient initiated")
		return false
	}

	api := k8sclient.AppsV1()
	var timeout int64 = 420
	listOptions := metav1.ListOptions{
		FieldSelector:  fields.OneTermEqualSelector("metadata.name", name).String(),
		TimeoutSeconds: &timeout,
	}
	watcher, err := api.Deployments(namespace).Watch(listOptions)
	if err != nil {
		logging.Log.Error(err, "An error occurred")
	}
	ch := watcher.ResultChan()
	logging.Log.Info("Waiting for deployment", "Namespace", namespace, "Name", name, "Timeout", timeout)
	for event := range ch {
		dc, ok := event.Object.(*appsv1.Deployment)
		if !ok {
			logging.Log.Error(err, "Unexpected type")
		}
		// check before watching in case the deployment is already scaled to 1
		deployment, err := k8sclient.AppsV1().Deployments(namespace).Get(name, metav1.GetOptions{})
		if err != nil {
			logging.Log.Error(err, "Failed to get Deployment", "Name", deployment.Name)
			return false
		}
		if deployment.Status.AvailableReplicas == 1 {
			logging.Log.Info("Deployment successfully scaled up", deployment.Name, deployment.Status.AvailableReplicas)
			return true
		}
		switch event.Type {
		case watch.Error:
			watcher.Stop()
		case watch.Modified:
			if dc.Status.AvailableReplicas == 1 {
				logging.Log.Info("Deployment '%s' successfully scaled", "Name", deployment.Name, "Replicas", dc.Status.AvailableReplicas)
				watcher.Stop()
				return true

			}
		}
	}
	dc, _ := k8sclient.AppsV1().Deployments(namespace).Get(name, metav1.GetOptions{})
	if dc.Status.AvailableReplicas != 1 {
		logging.Log.Error(err, "Failed to verify a successful deployment", "Name", dc.Name)
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

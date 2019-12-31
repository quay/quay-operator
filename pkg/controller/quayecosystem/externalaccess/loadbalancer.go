package externalaccess

import (
	"context"
	"fmt"
	"net"
	"time"

	"github.com/redhat-cop/quay-operator/pkg/controller/quayecosystem/logging"
	"github.com/redhat-cop/quay-operator/pkg/controller/quayecosystem/resources"
	"github.com/redhat-cop/quay-operator/pkg/controller/quayecosystem/utils"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/kubernetes"
	tools_watch "k8s.io/client-go/tools/watch"
)

type LoadBalancerExternalAccess struct {
	QuayConfiguration *resources.QuayConfiguration
	K8sClient         kubernetes.Interface
}

func (r *LoadBalancerExternalAccess) ManageQuayExternalAccess(meta metav1.ObjectMeta) error {
	serviceName := resources.GetQuayResourcesName(r.QuayConfiguration.QuayEcosystem)

	hostname, err := r.getHostnameFromExternalName(serviceName)

	if err != nil {
		return err
	}

	r.QuayConfiguration.QuayHostname = hostname

	return nil

}

func (r *LoadBalancerExternalAccess) ManageQuayConfigExternalAccess(meta metav1.ObjectMeta) error {

	serviceName := resources.GetQuayConfigResourcesName(r.QuayConfiguration.QuayEcosystem)

	hostname, err := r.getHostnameFromExternalName(serviceName)

	if err != nil {
		return err
	}

	r.QuayConfiguration.QuayConfigHostname = hostname

	return nil

}

func (r *LoadBalancerExternalAccess) RemoveQuayConfigExternalAccess(meta metav1.ObjectMeta) error {
	return nil
}

func (r *LoadBalancerExternalAccess) getHostnameFromExternalName(serviceName string) (string, error) {

	service, err := r.waitForExternalP(serviceName)

	if err != nil {
		return "", err
	}

	if len(service.Status.LoadBalancer.Ingress) == 0 {
		return "", fmt.Errorf("Returned Service Has No ExternalIP's Associated")
	}

	if service.Status.LoadBalancer.Ingress[0].Hostname != "" {
		hostname := service.Status.LoadBalancer.Ingress[0].Hostname

		logging.Log.Info(fmt.Sprintf("Waiting for Resolve LoadBalancer Service '%s'", hostname))

		err := utils.Retry(60, 5*time.Second, func() (err error) {
			_, err = net.LookupIP(hostname)
			return
		})
		if err != nil {
			return "", fmt.Errorf("Failed to resolve LoadBalancer service: %s", hostname)

		}

		logging.Log.Info(fmt.Sprintf("LoadBalancer Service '%s' Resolved", hostname))

		time.Sleep(time.Duration(2) * time.Second)

		return service.Status.LoadBalancer.Ingress[0].Hostname, nil
	} else if service.Status.LoadBalancer.Ingress[0].IP != "" {
		return service.Status.LoadBalancer.Ingress[0].IP, nil
	} else {
		return "", fmt.Errorf("Error locating value on LoadBalancer Service")
	}

}

func (r *LoadBalancerExternalAccess) waitForExternalP(serviceName string) (*corev1.Service, error) {

	logging.Log.Info(fmt.Sprintf("Waiting for LoadBalancer Service '%s' to be Allocated", serviceName), "Service Name", serviceName)

	options := metav1.ListOptions{
		FieldSelector: fields.OneTermEqualSelector("metadata.name", serviceName).String(),
	}

	w, err := r.K8sClient.CoreV1().Services(r.QuayConfiguration.QuayEcosystem.Namespace).Watch(options)

	if err != nil {
		return nil, err
	}
	defer w.Stop()

	condition := func(event watch.Event) (bool, error) {
		svc := event.Object.(*corev1.Service)
		return hasExternalAddress(svc), nil
	}

	ctx, _ := context.WithTimeout(context.Background(), 5*time.Minute)
	watchEvent, err := tools_watch.UntilWithoutRetry(ctx, w, condition)

	if err == wait.ErrWaitTimeout {
		return nil, fmt.Errorf("service %s never became ready", serviceName)
	}

	logging.Log.Info(fmt.Sprintf("LoadBalancer Service '%s' Was Successfully Allocated", serviceName), "Service Name", serviceName)

	svc := watchEvent.Object.(*corev1.Service)

	return svc, nil

}

func hasExternalAddress(svc *corev1.Service) bool {
	for _, v := range svc.Status.LoadBalancer.Ingress {
		if v.IP != "" || v.Hostname != "" {
			return true
		}
	}
	return false
}

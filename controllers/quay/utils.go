package controllers

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/pem"
	"fmt"
	"strings"
	"time"

	"github.com/go-logr/logr"
	v1 "github.com/quay/quay-operator/apis/quay/v1"
	"github.com/tidwall/sjson"
	"gopkg.in/yaml.v2"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/wait"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func encode(value interface{}) []byte {
	yamlified, _ := yaml.Marshal(value)

	return yamlified
}

// patchNamespaceForMonitoring adds a few labels to the namespace, these labels are
// required to enable monitoring to "observer" the given namespace.
func (r *QuayRegistryReconciler) patchNamespaceForMonitoring(
	ctx context.Context, quay v1.QuayRegistry,
) error {
	nsn := types.NamespacedName{
		Name: quay.GetNamespace(),
	}

	var ns corev1.Namespace
	if err := r.Client.Get(ctx, nsn, &ns); err != nil {
		return err
	}

	updatedNs := ns.DeepCopy()
	labels := make(map[string]string)
	for k, v := range updatedNs.Labels {
		labels[k] = v
	}

	if val := labels[clusterMonitoringLabelKey]; val == "true" {
		return nil
	}

	labels[clusterMonitoringLabelKey] = "true"
	labels[quayOperatorManagedLabelKey] = "true"
	updatedNs.Labels = labels

	patch := client.MergeFrom(&ns)
	return r.Client.Patch(ctx, updatedNs, patch)
}

// updateGrafanaDashboardData parses the Grafana Dashboard ConfigMap and updates the title and
// labels to filter the query by
func updateGrafanaDashboardData(obj client.Object, quay *v1.QuayRegistry) error {
	cm, ok := obj.(*corev1.ConfigMap)
	if !ok {
		return fmt.Errorf("unable to cast object to ConfigMap type")
	}

	config := cm.Data[quayDashboardJSONKey]

	title := fmt.Sprintf("Quay - %s - %s", quay.GetNamespace(), quay.GetName())
	config, err := sjson.Set(config, grafanaTitleJSONPath, title)
	if err != nil {
		return err
	}

	if config, err = sjson.Set(
		config, grafanaNamespaceFilterJSONPath, quay.GetNamespace(),
	); err != nil {
		return err
	}

	metricsServiceName := fmt.Sprintf("%s-quay-metrics", quay.GetName())
	config, err = sjson.Set(config, grafanaServiceFilterJSONPath, metricsServiceName)
	if err != nil {
		return err
	}

	cm.Data[quayDashboardJSONKey] = config
	return nil
}

// isGrafanaConfigMap checks if an Object is the Grafana ConfigMap used in the monitoring
// component.
func isGrafanaConfigMap(obj client.Object) bool {
	if !strings.HasSuffix(obj.GetName(), grafanaDashboardConfigMapNameSuffix) {
		return false
	}

	gvk := obj.GetObjectKind().GroupVersionKind()
	return gvk.Version == "v1" && gvk.Kind == "ConfigMap"
}

func (r *QuayRegistryReconciler) createOrUpdateObject(
	ctx context.Context, obj client.Object, quay v1.QuayRegistry, log logr.Logger,
) error {
	gvk := obj.GetObjectKind().GroupVersionKind()
	log = log.WithValues("kind", gvk.Kind, "name", obj.GetName())
	log.Info("creating/updating object")

	// we set the owner in the object except when it belongs to a different namespace,
	// on this case we have only the grafana dashboard that lives in another place.
	obj = v1.EnsureOwnerReference(&quay, obj)
	if isGrafanaConfigMap(obj) {
		var err error
		if obj, err = v1.RemoveOwnerReference(&quay, obj); err != nil {
			log.Error(err, "could not remove `ownerReferences` from grafana config")
			return err
		}
	}

	// managedFields cannot be set on a PATCH.
	obj.SetManagedFields([]metav1.ManagedFieldsEntry{})

	jobGVK := schema.GroupVersionKind{Group: "batch", Version: "v1", Kind: "Job"}
	if gvk == jobGVK {
		propagationPolicy := metav1.DeletePropagationForeground
		opts := &client.DeleteOptions{
			PropagationPolicy: &propagationPolicy,
		}

		if err := r.Client.Delete(ctx, obj, opts); err != nil && !errors.IsNotFound(err) {
			log.Error(err, "failed to delete immutable resource")
			return err
		}

		if err := wait.Poll(
			creationPollInterval,
			creationPollTimeout,
			func() (bool, error) {
				if err := r.Client.Create(ctx, obj); err != nil {
					if errors.IsAlreadyExists(err) {
						log.Info("immutable resource being deleted, retry")
						return false, nil
					}
					return true, err
				}
				return true, nil
			},
		); err != nil {
			log.Error(err, "failed to create immutable resource")
			return err
		}

		log.Info("succefully (re)created immutable resource")
		return nil
	}

	opts := []client.PatchOption{
		client.ForceOwnership,
		client.FieldOwner("quay-operator"),
	}
	if err := r.Client.Patch(ctx, obj, client.Apply, opts...); err != nil {
		log.Error(err, "failed to create/update object")
		return err
	}

	log.Info("finished creating/updating object")
	return nil
}

func (r *QuayRegistryReconciler) updateWithCondition(
	ctx context.Context,
	quay *v1.QuayRegistry,
	ctype v1.ConditionType,
	cstatus metav1.ConditionStatus,
	reason v1.ConditionReason,
	msg string,
) error {
	condition := v1.Condition{
		Type:               ctype,
		Status:             cstatus,
		Reason:             reason,
		Message:            msg,
		LastUpdateTime:     metav1.Now(),
		LastTransitionTime: metav1.Now(),
	}

	quay.Status.Conditions = v1.SetCondition(quay.Status.Conditions, condition)
	quay.Status.LastUpdate = time.Now().UTC().String()

	eventType := corev1.EventTypeNormal
	if cstatus == metav1.ConditionTrue {
		eventType = corev1.EventTypeWarning
	}
	r.EventRecorder.Event(quay, eventType, string(reason), msg)

	return r.Client.Status().Update(ctx, quay)
}

// reconcileWithCondition sets the given condition on the `QuayRegistry` and returns a reconcile
// result rescheduling the next loop.
func (r *QuayRegistryReconciler) reconcileWithCondition(
	ctx context.Context,
	quay *v1.QuayRegistry,
	ctype v1.ConditionType,
	cstatus metav1.ConditionStatus,
	reason v1.ConditionReason,
	msg string,
) (ctrl.Result, error) {
	err := r.updateWithCondition(ctx, quay, ctype, cstatus, reason, msg)
	return r.Requeue, err
}

// configEditorCredentialsSecretFrom returns the name of the secret that contains the
// credentials for the config editor. If the secret does not exist among the provided
// list an empty string is returned instead.
func configEditorCredentialsSecretFrom(objs []client.Object) string {
	for _, obj := range objs {
		if !strings.Contains(obj.GetName(), "quay-config-editor-credentials") {
			continue
		}

		gvk := obj.GetObjectKind().GroupVersionKind()
		if gvk.Version != "v1" {
			continue
		}
		if gvk.Kind != "Secret" {
			continue
		}

		return obj.GetName()
	}
	return ""
}

// Taken from https://stackoverflow.com/questions/46735347/how-can-i-fetch-a-certificate-from-a-url
func getCertificatesPEM(address string) ([]byte, error) {
	conn, err := tls.Dial("tcp", address, &tls.Config{
		InsecureSkipVerify: true,
	})
	if err != nil {
		return nil, err
	}
	defer conn.Close()
	var b bytes.Buffer
	for _, cert := range conn.ConnectionState().PeerCertificates {
		err := pem.Encode(&b, &pem.Block{
			Type:  "CERTIFICATE",
			Bytes: cert.Raw,
		})
		if err != nil {
			return nil, err
		}
	}

	return b.Bytes(), nil
}

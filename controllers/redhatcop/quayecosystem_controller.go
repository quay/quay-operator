/*


Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package controllers

import (
	"context"
	"errors"
	"time"

	"github.com/go-logr/logr"
	"github.com/quay/config-tool/pkg/lib/fieldgroups/database"
	"github.com/quay/config-tool/pkg/lib/fieldgroups/redis"
	"gopkg.in/yaml.v2"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/wait"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	quay "github.com/quay/quay-operator/apis/quay/v1"
	redhatcop "github.com/quay/quay-operator/apis/redhatcop/v1alpha1"
)

const (
	migrateLabel               = "quay-operator/migrate"
	migrationCompleteLabel     = "quay-operator/migration-complete"
	quayEnterpriseConfigSecret = "quay-enterprise-config-secret"
	migratedFromAnnotation     = "quay-operator/migrated-from"
)

// QuayEcosystemReconciler reconciles a QuayEcosystem object
type QuayEcosystemReconciler struct {
	client.Client
	Log    logr.Logger
	Scheme *runtime.Scheme
}

// +kubebuilder:rbac:groups=redhatcop.redhat.io.quay.redhat.com,resources=quayecosystems,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=redhatcop.redhat.io.quay.redhat.com,resources=quayecosystems/status,verbs=get;update;patch

func (r *QuayEcosystemReconciler) Reconcile(req ctrl.Request) (ctrl.Result, error) {
	ctx := context.Background()
	log := r.Log.WithValues("quayecosystem", req.NamespacedName)

	log.Info("begin reconcile")

	var quayEcosystem redhatcop.QuayEcosystem
	if err := r.Client.Get(ctx, req.NamespacedName, &quayEcosystem); err != nil {
		log.Error(err, "unable to retrieve `QuayEcosystem`")
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	updatedQuayEcosystem := quayEcosystem.DeepCopy()

	labels := quayEcosystem.GetLabels()
	if shouldMigrate, ok := labels[migrateLabel]; !ok || shouldMigrate != "true" {
		log.Info("`QuayEcosystem` not marked for migration, skipping")
		return ctrl.Result{}, nil
	} else if migrationComplete, ok := labels[migrationCompleteLabel]; ok && migrationComplete == "true" {
		log.Info("`QuayEcosystem` migration already completed, skipping")
		return ctrl.Result{}, nil
	}

	quayRegistry := &quay.QuayRegistry{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "quay.redhat.com/v1",
			Kind:       "QuayRegistry",
		},
		ObjectMeta: metav1.ObjectMeta{
			Namespace: quayEcosystem.GetNamespace(),
			Name:      quayEcosystem.GetName(),
			Annotations: map[string]string{
				migratedFromAnnotation: quayEcosystem.GetName(),
			},
			Labels: map[string]string{},
		},
		Spec: quay.QuayRegistrySpec{
			Components: []quay.Component{},
		},
	}

	var configBundle corev1.Secret
	if err := r.Client.Get(ctx, types.NamespacedName{Namespace: quayEcosystem.GetNamespace(), Name: quayEnterpriseConfigSecret}, &configBundle); err != nil {
		log.Error(err, "failed to retrieve `quay-enterprise-config-secret`")
		return ctrl.Result{}, nil
	}

	var baseConfig map[string]interface{}
	if err := yaml.Unmarshal(configBundle.Data["config.yaml"], &baseConfig); err != nil {
		log.Error(err, "failed to unmarshal config.yaml")
		return ctrl.Result{}, nil
	}

	if canHandleObjectStorage(quayEcosystem) {
		log.Info("attempting to migrate managed object storage")

		quayRegistry.Spec.Components = append(quayRegistry.Spec.Components, quay.Component{
			Kind:    "objectstorage",
			Managed: false,
		})

		log.Info("successfully migrated managed object storage")
	} else {
		log.Error(errors.New("cannot migrate local object storage"), "failed to migrate object storage")
		return ctrl.Result{}, nil
	}

	if canHandleDatabase(quayEcosystem) {
		log.Info("attempting to migrate managed database")
		fieldGroup, err := database.NewDatabaseFieldGroup(baseConfig)
		if err != nil {
			log.Error(err, "failed to parse existing database fieldgroup from base config")
			return ctrl.Result{}, nil
		}

		log.Info("attempting to create `PersistentVolumeClaim` for database migration")

		postgresPVC := corev1.PersistentVolumeClaim{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: quayRegistry.GetNamespace(),
				Name:      quayRegistry.GetName() + "-quay-database",
			},
			Spec: corev1.PersistentVolumeClaimSpec{
				AccessModes: []corev1.PersistentVolumeAccessMode{corev1.ReadWriteOnce},
				Resources: corev1.ResourceRequirements{
					Requests: corev1.ResourceList{corev1.ResourceStorage: resource.MustParse(quayEcosystem.Spec.Quay.Database.VolumeSize)},
				},
			},
		}

		if err := r.Client.Create(ctx, &postgresPVC); err != nil {
			log.Error(err, "failed to create `PersistentVolumeClaim` for database migration")
			return ctrl.Result{}, nil
		}

		log.Info("successfully created `PersistentVolumeClaim` for database migration")

		log.Info("attempting to create `Job` for database migration")

		ttl := new(int32)
		*ttl = 1
		migrationJob := batchv1.Job{
			ObjectMeta: metav1.ObjectMeta{
				Name:      quayRegistry.GetName() + "-quay-postgres-migration",
				Namespace: quayRegistry.GetNamespace(),
				OwnerReferences: []metav1.OwnerReference{
					{
						Kind:       "QuayEcosystem",
						Name:       quayEcosystem.GetName(),
						APIVersion: redhatcop.GroupVersion.String(),
						UID:        quayEcosystem.GetUID(),
					},
				},
			},
			Spec: batchv1.JobSpec{
				Template: corev1.PodTemplateSpec{
					ObjectMeta: metav1.ObjectMeta{
						Name: "quay-postgres-migration",
					},
					Spec: corev1.PodSpec{
						RestartPolicy: corev1.RestartPolicyNever,
						Volumes: []corev1.Volume{
							{
								Name: "postgres-data",
								VolumeSource: corev1.VolumeSource{
									PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{
										ClaimName: postgresPVC.GetName(),
									},
								},
							},
						},
						Containers: []corev1.Container{
							{
								Name:  "quay-postgres-migration",
								Image: "postgres",
								Command: []string{
									"pg_dump",
									"$(DB_URI)",
									"--format",
									"c",
									"--file",
									"/var/lib/postgresql/data/dump.sql",
								},
								Env: []corev1.EnvVar{
									{
										Name:  "DB_URI",
										Value: fieldGroup.DbUri,
									},
								},
								VolumeMounts: []corev1.VolumeMount{
									{
										Name:      "postgres-data",
										MountPath: "/var/lib/postgresql/data",
									},
								},
							},
						},
					},
				},
			},
		}

		if err := r.Client.Create(ctx, &migrationJob); err != nil {
			log.Error(err, "failed to create `Job` for database migration")
			return ctrl.Result{}, nil
		}

		log.Info("successfully created `Job` for database migration")

		err = wait.Poll(time.Second*10, time.Second*60, func() (bool, error) {
			log.Info("checking if database migration `Job` completed")

			err = r.Client.Get(ctx, types.NamespacedName{Namespace: migrationJob.GetNamespace(), Name: migrationJob.GetName()}, &migrationJob)
			if err != nil {
				return false, nil
			}

			return migrationJob.Status.Succeeded > 0, nil
		})

		if err != nil {
			log.Error(err, "failed to check status of database migration `Job`")
			return ctrl.Result{}, nil
		}

		for _, field := range (&database.DatabaseFieldGroup{}).Fields() {
			delete(baseConfig, field)
		}
	} else {
		log.Info("skipping unmanaged database", "server", updatedQuayEcosystem.Spec.Quay.Database.Server)

		quayRegistry.Spec.Components = append(quayRegistry.Spec.Components, quay.Component{
			Kind:    "postgres",
			Managed: false,
		})
	}

	if canHandleExternalAccess(quayEcosystem) {
		log.Info("attempting to migrate external access", "type", quayEcosystem.Spec.Quay.ExternalAccess.Type)
		// NOTE: Don't strip out `hostsettings` fieldgroup from base `config.yaml` because we use them when configuring the `Route`.
		log.Info("successfully migrated managed external access", "type", quayEcosystem.Spec.Quay.ExternalAccess.Type)
	} else {
		log.Info("skipping unsupported external access type", "type", quayEcosystem.Spec.Quay.ExternalAccess.Type)

		quayRegistry.Spec.Components = append(quayRegistry.Spec.Components, quay.Component{
			Kind:    "route",
			Managed: false,
		})
	}

	if canHandleRedis(quayEcosystem) {
		log.Info("attempting to migrate managed Redis")

		for _, field := range (&redis.RedisFieldGroup{}).Fields() {
			delete(baseConfig, field)
		}

		log.Info("successfully migrated managed Redis")
	} else {
		log.Info("skipping unmanaged Redis", "hostname", quayEcosystem.Spec.Redis.Hostname)

		quayRegistry.Spec.Components = append(quayRegistry.Spec.Components, quay.Component{
			Kind:    "redis",
			Managed: false,
		})
	}

	configBundleSecret := corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Namespace:    quayEcosystem.GetNamespace(),
			GenerateName: quayEcosystem.GetName() + "-config-bundle-",
			Annotations:  map[string]string{},
		},
		// Copy existing TLS certificate/keypair, extra CA certificates, etc...
		Data: configBundle.Data,
	}
	config, err := yaml.Marshal(baseConfig)
	if err != nil {
		log.Error(err, "failed to marshal `config.yaml`")
		return ctrl.Result{}, nil
	}

	log.Info("attempting to create base `configBundleSecret`", "configBundleSecret", quayEcosystem.GetName()+"-config-bundle-")

	configBundleSecret.Data["config.yaml"] = config
	if err := r.Client.Create(ctx, &configBundleSecret); err != nil {
		log.Error(err, "failed to create `configBundleSecret`")
	}

	log.Info("successfully created base `configBundleSecret`", "configBundleSecret", configBundleSecret.GetName())

	quayRegistry.Spec.ConfigBundleSecret = configBundleSecret.GetName()

	log.Info("attempting to create `QuayRegistry` from `QuayEcosystem`")

	if err := r.Client.Create(ctx, quayRegistry); err != nil {
		log.Error(err, "failed to create `QuayRegistry` from `QuayEcosystem`")
		return ctrl.Result{}, nil
	}

	log.Info("succesfully created `QuayRegistry` from `QuayEcosystem`")

	newLabels := updatedQuayEcosystem.GetLabels()
	newLabels[migrationCompleteLabel] = "true"
	updatedQuayEcosystem.SetLabels(newLabels)

	if err := r.Client.Update(ctx, updatedQuayEcosystem); err != nil {
		log.Error(err, "failed to mark `QuayEcosystem` with migration completed label")
		return ctrl.Result{}, nil
	}

	return ctrl.Result{}, nil
}

func canHandleDatabase(q redhatcop.QuayEcosystem) bool {
	return q.Spec.Quay.Database.Server == "" && q.Spec.Quay.Database.VolumeSize != ""
}

func canHandleExternalAccess(q redhatcop.QuayEcosystem) bool {
	return q.Spec.Quay.ExternalAccess != nil &&
		q.Spec.Quay.ExternalAccess.Type == redhatcop.RouteExternalAccessType &&
		q.Spec.Quay.ExternalAccess.TLS != nil &&
		q.Spec.Quay.ExternalAccess.TLS.Termination == redhatcop.PassthroughTLSTerminationType
}

func canHandleRedis(q redhatcop.QuayEcosystem) bool {
	return q.Spec.Redis != nil && q.Spec.Redis.Hostname == ""
}

func canHandleObjectStorage(q redhatcop.QuayEcosystem) bool {
	if q.Spec.Quay.RegistryBackends == nil || len(q.Spec.Quay.RegistryBackends) == 0 {
		return false
	}

	for _, backend := range q.Spec.Quay.RegistryBackends {
		if backend.Name == "local" {
			return false
		}
	}

	return true
}

func (r *QuayEcosystemReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&redhatcop.QuayEcosystem{}).
		Complete(r)
}

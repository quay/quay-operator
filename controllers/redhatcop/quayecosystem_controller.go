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
	"fmt"
	"os"
	"time"

	"github.com/quay/config-tool/pkg/lib/fieldgroups/signingengine"

	"github.com/go-logr/logr"
	"github.com/quay/config-tool/pkg/lib/fieldgroups/database"
	"github.com/quay/config-tool/pkg/lib/fieldgroups/hostsettings"
	"github.com/quay/config-tool/pkg/lib/fieldgroups/redis"
	"github.com/quay/config-tool/pkg/lib/fieldgroups/repomirror"
	"github.com/quay/config-tool/pkg/lib/fieldgroups/securityscanner"
	"gopkg.in/yaml.v2"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	k8sErrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/wait"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	quay "github.com/quay/quay-operator/apis/quay/v1"
	redhatcop "github.com/quay/quay-operator/apis/redhatcop/v1alpha1"
)

const (
	migrateLabel                   = "quay-operator/migrate"
	migrationCompleteLabel         = "quay-operator/migration-complete"
	migrationComponentLabel        = "quay-operator/migration-component"
	quayEnterpriseConfigSecretName = "quay-enterprise-config-secret"
	migratedFromAnnotation         = "quay-operator/migrated-from"

	pollInterval = time.Second * 5
	pollTimeout  = time.Second * 600
)

// QuayEcosystemReconciler reconciles a QuayEcosystem object
type QuayEcosystemReconciler struct {
	client.Client
	Log    logr.Logger
	Scheme *runtime.Scheme
}

// +kubebuilder:rbac:groups=redhatcop.redhat.io,resources=quayecosystems,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=redhatcop.redhat.io,resources=quayecosystems/status,verbs=get;update;patch

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

	quayEcosystemLabels := quayEcosystem.GetLabels()
	if shouldMigrate, ok := quayEcosystemLabels[migrateLabel]; !ok || shouldMigrate != "true" {
		log.Info("`QuayEcosystem` not marked for migration, skipping")

		return ctrl.Result{}, nil
	} else if migrationComplete, ok := quayEcosystemLabels[migrationCompleteLabel]; ok && migrationComplete == "true" {
		log.Info("`QuayEcosystem` migration already completed, skipping")

		updatedQuayEcosystem.Status.Conditions = redhatcop.RemoveCondition(updatedQuayEcosystem.Status.Conditions, redhatcop.QuayEcosystemConfigMigrationFailure)
		updatedQuayEcosystem.Status.Conditions = redhatcop.RemoveCondition(updatedQuayEcosystem.Status.Conditions, redhatcop.QuayEcosystemComponentMigrationFailure)

		if err := r.Client.Status().Update(ctx, updatedQuayEcosystem); err != nil {
			log.Error(err, "failed to remove conditions from `QuayEcosystem`")
		}

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
	if err := r.Client.Get(ctx, types.NamespacedName{Namespace: quayEcosystem.GetNamespace(), Name: quayEnterpriseConfigSecretName}, &configBundle); err != nil {
		msg := "failed to retrieve `quay-enterprise-config-secret`"
		log.Error(err, msg)

		return r.reconcileWithCondition(
			&quayEcosystem,
			redhatcop.QuayEcosystemConfigMigrationFailure,
			corev1.ConditionTrue,
			"ConfigBundleSecretInvalid",
			fmt.Sprintf("%s: %s", msg, err))
	}

	var baseConfig map[string]interface{}
	if err := yaml.Unmarshal(configBundle.Data["config.yaml"], &baseConfig); err != nil {
		msg := "failed to unmarshal config.yaml"
		log.Error(err, msg)

		return r.reconcileWithCondition(
			&quayEcosystem,
			redhatcop.QuayEcosystemConfigMigrationFailure,
			corev1.ConditionTrue,
			"ConfigBundleSecretInvalid",
			fmt.Sprintf("%s: %s", msg, err))
	}

	if canHandleObjectStorage(quayEcosystem) {
		log.Info("attempting to migrate managed object storage")

		quayRegistry.Spec.Components = append(quayRegistry.Spec.Components, quay.Component{
			Kind:    "objectstorage",
			Managed: false,
		})

		log.Info("successfully migrated managed object storage")
	} else {
		err := errors.New("cannot migrate local object storage")
		msg := "failed to migrate object storage"
		log.Error(err, msg)

		return r.reconcileWithCondition(
			&quayEcosystem,
			redhatcop.QuayEcosystemComponentMigrationFailure,
			corev1.ConditionTrue,
			"ComponentUnsupported",
			fmt.Sprintf("%s: %s", msg, err))
	}

	if canHandleDatabase(quayEcosystem) {
		log.Info("attempting to migrate managed database")

		credentialsSecretName := quayEcosystem.Spec.Quay.Database.CredentialsSecretName
		if credentialsSecretName == "" {
			credentialsSecretName = defaultDBSecretFor(quayEcosystem)
		}

		log.Info("using `Secret` containing database credentials", "credentialsSecretName", credentialsSecretName)

		var postgresDeployment appsv1.Deployment
		if err := r.Client.Get(ctx, types.NamespacedName{Namespace: quayEcosystem.GetNamespace(), Name: defaultDBSecretFor(quayEcosystem)}, &postgresDeployment); err != nil {
			msg := "failed to retrieve existing managed database `Deployment`"
			log.Error(err, msg)

			return r.reconcileWithCondition(
				&quayEcosystem,
				redhatcop.QuayEcosystemComponentMigrationFailure,
				corev1.ConditionTrue,
				"ComponentUnsupported",
				fmt.Sprintf("%s: %s", msg, err))
		}

		// This environemnt variable wasn't being added by previous Operator version and prevents external access as the `postgres` user.
		envVars := []corev1.EnvVar{
			{
				Name: "POSTGRESQL_ADMIN_PASSWORD",
				ValueFrom: &corev1.EnvVarSource{
					SecretKeyRef: &corev1.SecretKeySelector{
						LocalObjectReference: corev1.LocalObjectReference{Name: credentialsSecretName},
						Key:                  "database-root-password",
					},
				},
			},
		}
		for _, envVar := range postgresDeployment.Spec.Template.Spec.Containers[0].Env {
			if envVar.Name != "POSTGRESQL_ADMIN_PASSWORD" {
				envVars = append(envVars, envVar)
			}
		}
		postgresDeployment.Spec.Template.Spec.Containers[0].Env = envVars

		if err := r.Client.Update(ctx, &postgresDeployment); err != nil {
			msg := "failed to update managed Postgres `Deployment` with `POSTGRESQL_ADMIN_PASSWORD` environment variable"
			log.Error(err, msg)

			return r.reconcileWithCondition(
				&quayEcosystem,
				redhatcop.QuayEcosystemComponentMigrationFailure,
				corev1.ConditionTrue,
				"ComponentUnsupported",
				fmt.Sprintf("%s: %s", msg, err))
		}

		log.Info("successfully updated managed Postgres `Deployment` with `POSTGRESQL_ADMIN_PASSWORD` environment variable")
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

		pgImage := os.Getenv("RELATED_IMAGE_COMPONENT_POSTGRES")
		if pgImage == "" {
			pgImage = "centos/postgresql-10-centos7"
		}

		cleanupCommand := `sleep 20; rm -f /tmp/change-username.sql /tmp/check-user.sql; echo "ALTER ROLE \"$OLD_DB_USERNAME\" RENAME TO \"$NEW_DB_USERNAME\"; ALTER DATABASE \"$OLD_DB_NAME\" RENAME TO \"$NEW_DB_NAME\";" > /tmp/change-username.sql; echo "SELECT 1 FROM pg_roles WHERE rolname = '$NEW_DB_USERNAME';" > /tmp/check-user.sql; psql -h localhost -f /tmp/check-user.sql | grep -q 1 || psql -h localhost -f /tmp/change-username.sql; sleep 600;`
		migrationDeployment := &appsv1.Deployment{
			ObjectMeta: metav1.ObjectMeta{
				Name:      quayRegistry.GetName() + "-quay-postgres-migration",
				Namespace: quayRegistry.GetNamespace(),
				Labels: map[string]string{
					migrationComponentLabel: quayRegistry.GetName() + "-quay-postgres-migration",
				},
				OwnerReferences: []metav1.OwnerReference{
					{
						Kind:       "QuayEcosystem",
						Name:       quayEcosystem.GetName(),
						APIVersion: redhatcop.GroupVersion.String(),
						UID:        quayEcosystem.GetUID(),
					},
				},
			},
			Spec: appsv1.DeploymentSpec{
				Selector: &metav1.LabelSelector{
					MatchLabels: map[string]string{
						migrationComponentLabel: quayRegistry.GetName() + "-quay-postgres-migration",
					},
				},
				Template: corev1.PodTemplateSpec{
					ObjectMeta: metav1.ObjectMeta{
						Labels: map[string]string{
							migrationComponentLabel: quayRegistry.GetName() + "-quay-postgres-migration",
						},
					},
					Spec: corev1.PodSpec{
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
						InitContainers: []corev1.Container{
							{
								Name:  "quay-postgres-migration-init",
								Image: pgImage,
								Command: []string{
									"psql",
									"-U",
									"postgres",
									"-h",
									quayEcosystem.GetName() + "-quay-postgresql",
								},
								Env: []corev1.EnvVar{
									{
										Name: "PGPASSWORD",
										ValueFrom: &corev1.EnvVarSource{
											SecretKeyRef: &corev1.SecretKeySelector{
												LocalObjectReference: corev1.LocalObjectReference{Name: credentialsSecretName},
												Key:                  "database-root-password",
											},
										},
									},
								},
							},
						},
						Containers: []corev1.Container{
							{
								Name:  "quay-postgres-migration",
								Image: pgImage,
								Env: []corev1.EnvVar{
									{
										Name:  "POSTGRESQL_MIGRATION_REMOTE_HOST",
										Value: quayEcosystem.GetName() + "-quay-postgresql",
									},
									{
										Name: "POSTGRESQL_MIGRATION_ADMIN_PASSWORD",
										// NOTE: By default, `QuayEcosystems` will not have a password set for `postgres` user, preventing remote connections. Users must SSH into Postgres pod and use `\password` before beginning migration.
										ValueFrom: &corev1.EnvVarSource{
											SecretKeyRef: &corev1.SecretKeySelector{
												LocalObjectReference: corev1.LocalObjectReference{Name: credentialsSecretName},
												Key:                  "database-root-password",
											},
										},
									},
								},
								ReadinessProbe: &corev1.Probe{
									Handler: corev1.Handler{
										Exec: &corev1.ExecAction{
											Command: []string{"/usr/libexec/check-container"},
										},
									},
								},
								VolumeMounts: []corev1.VolumeMount{
									{
										Name:      "postgres-data",
										MountPath: "/var/lib/pgsql/data",
									},
								},
							},
							{
								Name:    "quay-postgres-migration-cleanup",
								Image:   pgImage,
								Command: []string{"/bin/bash", "-c", cleanupCommand},
								ReadinessProbe: &corev1.Probe{
									Handler: corev1.Handler{
										Exec: &corev1.ExecAction{
											Command: []string{"/bin/bash", "-c", "psql -h localhost -f /tmp/check-user.sql | grep -q 1"},
										},
									},
								},
								Env: []corev1.EnvVar{
									{
										Name: "PGPASSWORD",
										ValueFrom: &corev1.EnvVarSource{
											SecretKeyRef: &corev1.SecretKeySelector{
												LocalObjectReference: corev1.LocalObjectReference{Name: credentialsSecretName},
												Key:                  "database-root-password",
											},
										},
									},
									{
										Name: "OLD_DB_USERNAME",
										ValueFrom: &corev1.EnvVarSource{
											SecretKeyRef: &corev1.SecretKeySelector{
												LocalObjectReference: corev1.LocalObjectReference{Name: credentialsSecretName},
												Key:                  "database-username",
											},
										},
									},
									{
										Name:  "NEW_DB_USERNAME",
										Value: quayEcosystem.GetName() + "-quay-database",
									},
									{
										Name: "OLD_DB_NAME",
										ValueFrom: &corev1.EnvVarSource{
											SecretKeyRef: &corev1.SecretKeySelector{
												LocalObjectReference: corev1.LocalObjectReference{Name: credentialsSecretName},
												Key:                  "database-name",
											},
										},
									},
									{
										Name:  "NEW_DB_NAME",
										Value: quayEcosystem.GetName() + "-quay-database",
									},
								},
							},
						},
					},
				},
			},
		}

		if err := wait.Poll(pollInterval, pollTimeout, func() (bool, error) {
			log.Info("checking that new database `PersistentVolumeClaim` does not already exist")

			if err := r.Client.Get(ctx, types.NamespacedName{Name: postgresPVC.GetName(), Namespace: postgresPVC.GetNamespace()}, &corev1.PersistentVolumeClaim{}); err == nil || !k8sErrors.IsNotFound(err) {
				log.Info("attempting to delete new database `PersistentVolumeClaim` to start clean migration")

				r.Client.Delete(ctx, migrationDeployment)
				r.Client.Delete(ctx, &postgresPVC)

				return false, nil
			}

			return true, nil
		}); err != nil {
			msg := "failed to clean up database migration resources from previous attempt"
			log.Error(err, msg)

			return r.reconcileWithCondition(
				&quayEcosystem,
				redhatcop.QuayEcosystemComponentMigrationFailure,
				corev1.ConditionTrue,
				"ComponentUnsupported",
				fmt.Sprintf("%s: %s", msg, err))
		}

		if err := r.Client.Create(ctx, &postgresPVC); err != nil {
			log.Error(err, "failed to create `PersistentVolumeClaim` for database migration")

			msg := "failed to create `PersistentVolumeClaim` for database migration"
			log.Error(err, msg)

			return r.reconcileWithCondition(
				&quayEcosystem,
				redhatcop.QuayEcosystemComponentMigrationFailure,
				corev1.ConditionTrue,
				"ComponentUnsupported",
				fmt.Sprintf("%s: %s", msg, err))
		}

		log.Info("successfully created `PersistentVolumeClaim` for database migration")
		log.Info("attempting to create `Deployment` for database migration")

		if err := r.Client.Create(ctx, migrationDeployment); err != nil {
			msg := "failed to create `Deployment` for database migration"
			log.Error(err, msg)

			return r.reconcileWithCondition(
				&quayEcosystem,
				redhatcop.QuayEcosystemComponentMigrationFailure,
				corev1.ConditionTrue,
				"ComponentUnsupported",
				fmt.Sprintf("%s: %s", msg, err))
		}

		log.Info("successfully created `Deployment` for database migration", "deployment", migrationDeployment.GetNamespace()+"/"+migrationDeployment.GetName())

		err := wait.Poll(pollInterval, pollTimeout, func() (bool, error) {
			log.Info("checking if database migration `Deployment` completed")

			if err := r.Client.Get(ctx, types.NamespacedName{Namespace: migrationDeployment.GetNamespace(), Name: migrationDeployment.GetName()}, migrationDeployment); err != nil {
				log.Error(err, "failed to fetch database migration `Deployment`")

				return false, nil
			}

			if migrationDeployment.Status.ReadyReplicas > 0 {
				return true, nil
			}

			var migrationPods corev1.PodList
			if err := r.Client.List(ctx, &migrationPods, &client.ListOptions{
				Namespace:     quayEcosystem.GetNamespace(),
				LabelSelector: labels.SelectorFromSet(migrationDeployment.Spec.Selector.MatchLabels),
			}); err != nil {
				log.Error(err, "failed to fetch database migration pods")

				return false, nil
			}

			for _, migrationPod := range migrationPods.Items {
				if len(migrationPod.Status.InitContainerStatuses) == 0 || !migrationPod.Status.InitContainerStatuses[0].Ready {
					log.Info("database migration pod in progress")

					return false, nil
				}

				for _, containerStatus := range migrationPod.Status.ContainerStatuses {
					if !containerStatus.Ready {
						log.Info("database migration container not ready", "container", containerStatus.Name)

						return false, nil
					}
				}
			}

			return true, nil
		})

		if err != nil {
			msg := "database migration pod failed"
			log.Error(err, msg)

			return r.reconcileWithCondition(
				&quayEcosystem,
				redhatcop.QuayEcosystemComponentMigrationFailure,
				corev1.ConditionTrue,
				"ComponentUnsupported",
				fmt.Sprintf("%s: %s", msg, err))
		}

		if err := r.Client.Delete(ctx, migrationDeployment); err != nil {
			msg := "failed to delete `Deployment` after database migration"
			log.Error(err, msg)

			return r.reconcileWithCondition(
				&quayEcosystem,
				redhatcop.QuayEcosystemComponentMigrationFailure,
				corev1.ConditionTrue,
				"ComponentUnsupported",
				fmt.Sprintf("%s: %s", msg, err))
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

		for _, field := range (&hostsettings.HostSettingsFieldGroup{}).Fields() {
			if field != "SERVER_HOSTNAME" {
				delete(baseConfig, field)
			}
		}

		log.Info("successfully migrated managed external access", "type", quayEcosystem.Spec.Quay.ExternalAccess.Type)
	} else {
		log.Info("skipping unsupported external access type")

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

	if canHandleMirror(quayEcosystem) {
		log.Info("attempting to migrate managed repo mirroring")

		for _, field := range (&repomirror.RepoMirrorFieldGroup{}).Fields() {
			delete(baseConfig, field)
		}
	} else {
		log.Info("skipping unmanaged repo mirroring")

		quayRegistry.Spec.Components = append(quayRegistry.Spec.Components, quay.Component{
			Kind:    "mirror",
			Managed: false,
		})
	}

	if canHandleClair(quayEcosystem) {
		log.Info("attempting to migrate managed security scanner")

		for _, field := range (&securityscanner.SecurityScannerFieldGroup{}).Fields() {
			delete(baseConfig, field)
		}

		log.Info("successfully migrated managed security scanner")
	} else {
		log.Info("skipping unmanaged security scanner")

		quayRegistry.Spec.Components = append(quayRegistry.Spec.Components, quay.Component{
			Kind:    "clair",
			Managed: false,
		})
	}

	baseConfig = clean(baseConfig)

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
		msg := "failed to marshal `config.yaml`"
		log.Error(err, msg)

		return r.reconcileWithCondition(
			&quayEcosystem,
			redhatcop.QuayEcosystemConfigMigrationFailure,
			corev1.ConditionTrue,
			"ConfigBundleSecretInvalid",
			fmt.Sprintf("%s: %s", msg, err))
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
		msg := "failed to create `QuayRegistry` from `QuayEcosystem`"
		log.Error(err, msg)

		return r.reconcileWithCondition(
			&quayEcosystem,
			redhatcop.QuayEcosystemMigrationFailure,
			corev1.ConditionTrue,
			"QuayRegistryCreationError",
			fmt.Sprintf("%s: %s", msg, err))
	}

	log.Info("succesfully created `QuayRegistry` from `QuayEcosystem`")

	// Fetch `QuayEcosystem` again to ensure we have the most recent version.
	if err := r.Client.Get(ctx, req.NamespacedName, updatedQuayEcosystem); err != nil {
		log.Error(err, "unable to retrieve `QuayEcosystem`")

		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	newLabels := updatedQuayEcosystem.GetLabels()
	newLabels[migrationCompleteLabel] = "true"
	updatedQuayEcosystem.SetLabels(newLabels)

	if err := r.Client.Update(ctx, updatedQuayEcosystem); err != nil {
		msg := "failed to mark `QuayEcosystem` with migration completed label"
		log.Error(err, msg)

		return r.reconcileWithCondition(
			updatedQuayEcosystem,
			redhatcop.QuayEcosystemMigrationFailure,
			corev1.ConditionTrue,
			"QuayEcosystemLabelError",
			fmt.Sprintf("%s: %s", msg, err))
	}

	return ctrl.Result{}, nil
}

func (r *QuayEcosystemReconciler) updateWithCondition(q *redhatcop.QuayEcosystem, t redhatcop.QuayEcosystemConditionType, s corev1.ConditionStatus, reason, msg string) (*redhatcop.QuayEcosystem, error) {
	updatedQuay := q.DeepCopy()

	condition := redhatcop.QuayEcosystemCondition{
		Type:               t,
		Status:             s,
		Reason:             reason,
		Message:            msg,
		LastUpdateTime:     metav1.Now(),
		LastTransitionTime: metav1.Now(),
	}
	updatedQuay.Status.Conditions = redhatcop.SetCondition(q.Status.Conditions, condition)

	if err := r.Client.Status().Update(context.Background(), updatedQuay); err != nil {
		return nil, err
	}

	return updatedQuay, nil
}

// reconcileWithCondition sets the given condition on the `QuayEcosystem` and returns a reconcile result.
func (r *QuayEcosystemReconciler) reconcileWithCondition(q *redhatcop.QuayEcosystem, t redhatcop.QuayEcosystemConditionType, s corev1.ConditionStatus, reason, msg string) (ctrl.Result, error) {
	_, err := r.updateWithCondition(q, t, s, reason, msg)

	return ctrl.Result{}, err
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
	if q.Spec.Quay.RegistryStorage != nil ||
		q.Spec.Quay.RegistryBackends == nil ||
		len(q.Spec.Quay.RegistryBackends) == 0 {
		return false
	}

	for _, backend := range q.Spec.Quay.RegistryBackends {
		if backend.Local != nil {
			return false
		}
	}

	return true
}

func canHandleMirror(q redhatcop.QuayEcosystem) bool {
	return q.Spec.Quay.EnableRepoMirroring
}

func canHandleClair(q redhatcop.QuayEcosystem) bool {
	return q.Spec.Clair != nil && q.Spec.Clair.Enabled
}

func clean(config map[string]interface{}) map[string]interface{} {
	// NOTE: Signing engine code has been removed from Quay.
	for _, field := range (&signingengine.SigningEngineFieldGroup{}).Fields() {
		delete(config, field)
	}

	return config
}

func defaultDBSecretFor(q redhatcop.QuayEcosystem) string {
	return q.GetName() + "-quay-postgresql"
}

func (r *QuayEcosystemReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&redhatcop.QuayEcosystem{}).
		Complete(r)
}

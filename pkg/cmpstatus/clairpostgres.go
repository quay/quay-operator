package cmpstatus

import (
	"context"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	qv1 "github.com/quay/quay-operator/apis/quay/v1"
)

// ClairPostgres checks a quay registry clairpostgres component status.
type ClairPostgres struct {
	Client client.Client
}

// Name returns the component name this entity checks for health.
func (c *ClairPostgres) Name() string {
	return "clairpostgres"
}

// Check verifies if the clairpostgres deployment associated with provided quay registry
// was created and rolled out as expected, also checking its PVC status dynamically.
func (c *ClairPostgres) Check(ctx context.Context, quay qv1.QuayRegistry) (qv1.Condition, error) {
	if !qv1.ComponentIsManaged(quay.Spec.Components, qv1.ComponentClairPostgres) {
		return qv1.Condition{
			Type:           qv1.ComponentClairPostgresReady,
			Status:         metav1.ConditionTrue,
			Reason:         qv1.ConditionReasonComponentUnmanaged,
			Message:        "ClairPostgres not managed by the operator",
			LastUpdateTime: metav1.NewTime(time.Now()),
		}, nil
	}

	return CheckDatabaseDeploymentAndPVCStatus(
		ctx,
		c.Client,
		quay,
		qv1.ComponentClairPostgres,
		"clair-postgres",
		qv1.ComponentClairPostgresReady,
	)
}

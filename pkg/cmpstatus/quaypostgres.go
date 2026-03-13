package cmpstatus

import (
	"context"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	qv1 "github.com/quay/quay-operator/apis/quay/v1"
)

// Postgres checks a quay registry postgres component status.
type Postgres struct {
	Client client.Client
}

// Name returns the component name this entity checks for health.
func (p *Postgres) Name() string {
	return "postgres"
}

// Check verifies if the postgres deployment associated with provided quay registry was created
// and rolled out as expected, also checking its PVC status dynamically.
func (p *Postgres) Check(ctx context.Context, quay qv1.QuayRegistry) (qv1.Condition, error) {
	if !qv1.ComponentIsManaged(quay.Spec.Components, qv1.ComponentPostgres) {
		return qv1.Condition{
			Type:           qv1.ComponentPostgresReady,
			Status:         metav1.ConditionTrue,
			Reason:         qv1.ConditionReasonComponentUnmanaged,
			Message:        "Postgres not managed by the operator",
			LastUpdateTime: metav1.NewTime(time.Now()),
		}, nil
	}

	return CheckDatabaseDeploymentAndPVCStatus(
		ctx,
		p.Client,
		quay,
		qv1.ComponentPostgres,
		"quay-database",
		qv1.ComponentPostgresReady,
	)
}

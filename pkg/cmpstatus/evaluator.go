package cmpstatus

import (
	"context"
	"fmt"
	"strings"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	qv1 "github.com/quay/quay-operator/apis/quay/v1"
)

// Checker is implemented by all status analysers in this package. it implements two functions,
// one that checks the status of a component and other that returns the component name it checks.
type Checker interface {
	Name() string
	Check(context.Context, qv1.QuayRegistry) (qv1.Condition, error)
}

// Evaluate attempts to evaluate the status of all components of a quay registry instace. It
// attempts to map inter components dependencies.
func Evaluate(ctx context.Context, c client.Client, q qv1.QuayRegistry) ([]qv1.Condition, error) {
	var conds []qv1.Condition

	// start by analysing the components that have no dependencies. we append their conditions
	// to the conditions slice we are going to return at the end and move on. the health of
	// any of these components don't affect other components health.
	for _, component := range []Checker{
		&HPA{Client: c},
		&Route{Client: c},
		&Monitoring{Client: c},
	} {
		cond, err := component.Check(ctx, q)
		if err != nil {
			return nil, err
		}
		conds = append(conds, cond)
	}

	// now analyse the components that the base component depends on. if any of these is in
	// a faulty state then base won't be able to come up properly. we gather the name of
	// any faulty component in a slice of strings, all conditions are append to the slice
	// we return at the end of the process.
	var failed []string
	for _, component := range []Checker{
		&Postgres{Client: c},
		&ObjectStorage{Client: c},
		&Clair{Client: c},
		&TLS{Client: c},
		&Redis{Client: c},
	} {
		cond, err := component.Check(ctx, q)
		if err != nil {
			return nil, err
		}

		conds = append(conds, cond)
		if cond.Status != metav1.ConditionTrue {
			failed = append(failed, component.Name())
		}
	}

	// if we found out any component in a faulty state we have to abort now. base component
	// must indicate which component is in a faulty state. as mirror component depends on
	// base component its status is also defined as faulty.
	if len(failed) > 0 {
		conds = append(
			conds,
			qv1.Condition{
				Type:           qv1.ComponentBaseReady,
				Status:         metav1.ConditionFalse,
				Reason:         qv1.ConditionReasonComponentNotReady,
				LastUpdateTime: metav1.NewTime(time.Now()),
				Message: fmt.Sprintf(
					"Awaiting for component %s to become available",
					strings.Join(failed, ","),
				),
			},
			qv1.Condition{
				Type:           qv1.ComponentMirrorReady,
				Status:         metav1.ConditionFalse,
				Reason:         qv1.ConditionReasonComponentNotReady,
				Message:        "Awaiting for component base to become available",
				LastUpdateTime: metav1.NewTime(time.Now()),
			},
		)
		return conds, nil
	}

	// checks now if the base component is in a faulty state. if it is then sets mirror
	// component as faulty as well (awaiting for base) and returns. base condition is
	// append to the returned slice.
	base := &Base{Client: c}
	cond, err := base.Check(ctx, q)
	if err != nil {
		return nil, err
	}
	conds = append(conds, cond)

	// if base is in a faulty state then sets mirror as faulty and awaiting for base. we
	// can return here as there is no need to check mirror status.
	if cond.Status != metav1.ConditionTrue {
		conds = append(
			conds,
			qv1.Condition{
				Type:    qv1.ComponentMirrorReady,
				Status:  metav1.ConditionFalse,
				Reason:  qv1.ConditionReasonComponentNotReady,
				Message: "Awaiting for component base to become available",
			},
		)
		return conds, nil
	}

	// this is the last component we check the health for. it depends on base component that
	// in turn depends on almost all other components.
	mirror := &Mirror{Client: c}
	cond, err = mirror.Check(ctx, q)
	if err != nil {
		return nil, err
	}
	conds = append(conds, cond)
	return conds, nil
}

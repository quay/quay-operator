package controller

import (
	"github.com/redhat-cop/quay-operator/pkg/controller/quayecosystem"
)

func init() {
	// AddToManagerFuncs is a list of functions to create controllers and add them to a manager.
	AddToManagerFuncs = append(AddToManagerFuncs, quayecosystem.Add)
}

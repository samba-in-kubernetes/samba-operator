package controller

import (
	"github.com/obnoxxx/samba-operator/pkg/controller/smbservice"
)

func init() {
	// AddToManagerFuncs is a list of functions to create controllers and add them to a manager.
	AddToManagerFuncs = append(AddToManagerFuncs, smbservice.Add)
}

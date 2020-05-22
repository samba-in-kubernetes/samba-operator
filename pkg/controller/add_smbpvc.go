package controller

import (
	"github.com/obnoxxx/samba-operator/pkg/controller/smbpvc"
)

func init() {
	// AddToManagerFuncs is a list of functions to create controllers and add them to a manager.
	AddToManagerFuncs = append(AddToManagerFuncs, smbpvc.Add)
}

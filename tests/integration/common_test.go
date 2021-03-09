// +build integration

package integration

import (
	"os"
)

var testNamespace = "samba-operator-system"

func init() {
	ns := os.Getenv("SMBOP_TEST_NAMESPACE")
	if ns != "" {
		testNamespace = ns
	}
}

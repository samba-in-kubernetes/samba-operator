// +build integration

package integration

import (
	"os"
)

var (
	testNamespace = "samba-operator-system"

	testFilesDir = "../files"
)

func init() {
	ns := os.Getenv("SMBOP_TEST_NAMESPACE")
	if ns != "" {
		testNamespace = ns
	}

	fdir := os.Getenv("SMBOP_TEST_FILES_DIR")
	if ns != "" {
		testFilesDir = fdir
	}
}

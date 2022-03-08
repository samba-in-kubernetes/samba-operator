// SPDX-License-Identifier: Apache-2.0

package main

import (
	"os"
	goruntime "runtime"

	"sigs.k8s.io/controller-runtime/pkg/log/zap"

	"github.com/samba-in-kubernetes/samba-operator/internal/metrics"
)

var (
	// Version of the software at compile time.
	Version = "(unset)"
	// CommitID of the revision used to compile the software.
	CommitID = "(unset)"
)

func main() {
	log := zap.New(zap.UseDevMode(true))
	log.Info("Initializing smbmetrics",
		"ProgramName", os.Args[0],
		"Version", Version,
		"CommitID", CommitID,
		"GoVersion", goruntime.Version())

	loc, err := metrics.LocateSmbStatus()
	if err != nil {
		log.Error(err, "Failed to locate smbstatus")
		os.Exit(1)
	}
	ver, err := metrics.RunSmbStatusVersion()
	if err != nil {
		log.Error(err, "Failed to run smbstatus")
		os.Exit(1)
	}
	log.Info("Located smbstatus", "path", loc, "version", ver)

	err = metrics.RunSmbMetricsExporter(log)
	if err != nil {
		os.Exit(1)
	}
}

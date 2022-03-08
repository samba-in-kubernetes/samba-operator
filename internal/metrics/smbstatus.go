// SPDX-License-Identifier: Apache-2.0

package metrics

import (
	"errors"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"time"
)

// SmbStatusShare represents a single entry from the output of 'smbstatus -S'
type SmbStatusShare struct {
	Service       string
	PID           int64
	Machine       string
	ConnectedAt   string
	ConnectedTime time.Time
	Encryption    string
	Signing       string
}

// SmbStatusProc represents a single entry from the output of 'smbstatus -p'
type SmbStatusProc struct {
	PID             int64
	Username        string
	Group           string
	Machine         string
	ProtocolVersion string
	Encryption      string
	Signing         string
}

// SmbStatusLock represents a single entry from the output of 'smbstatus -L'
type SmbStatusLock struct {
	PID       int64
	UserID    int64
	DenyMode  string
	Access    string
	RW        string
	Oplock    string
	SharePath string
	Name      string
	Time      string
}

// LocateSmbStatus finds the local executable of 'smbstatus' on host container.
func LocateSmbStatus() (string, error) {
	knowns := []string{
		"/usr/bin/smbstatus",
	}
	for _, loc := range knowns {
		fi, err := os.Stat(loc)
		if err != nil {
			continue
		}
		mode := fi.Mode()
		if !mode.IsRegular() {
			continue
		}
		if (mode & 0111) > 0 {
			return loc, nil
		}
	}
	return "", errors.New("failed to locate smbstatus")
}

// RunSmbStatusVersion executes 'smbstatus --version' on host container
func RunSmbStatusVersion() (string, error) {
	ver, err := executeSmbStatusCommand("--version")
	if err != nil {
		return "", err
	}
	return ver, nil
}

// RunSmbStatusShares executes 'smbstatus -S' on host container
func RunSmbStatusShares() ([]SmbStatusShare, error) {
	dat, err := executeSmbStatusCommand("-S")
	if err != nil {
		return []SmbStatusShare{}, err
	}
	return parseSmbStatusShares(dat)
}

// RunSmbStatusLocks executes 'smbstatus -L' on host container
func RunSmbStatusLocks() ([]SmbStatusLock, error) {
	dat, err := executeSmbStatusCommand("-L")
	if err != nil {
		return []SmbStatusLock{}, err
	}
	return parseSmbStatusLocks(dat)
}

// RunSmbStatusProcs executes 'smbstatus -p' on host container
func RunSmbStatusProcs() ([]SmbStatusProc, error) {
	dat, err := executeSmbStatusCommand("-p")
	if err != nil {
		return []SmbStatusProc{}, err
	}
	return parseSmbStatusProcs(dat)
}

// SmbStatusSharesByMachine converts the output of RunSmbStatusShares into map
// indexed by machine's host
func SmbStatusSharesByMachine() (map[string][]SmbStatusShare, error) {
	ret := map[string][]SmbStatusShare{}
	shares, err := RunSmbStatusShares()
	if err != nil {
		return ret, err
	}
	for _, share := range shares {
		ret[share.Machine] = append(ret[share.Machine], share)
	}
	return ret, nil
}

func executeSmbStatusCommand(args ...string) (string, error) {
	loc, err := LocateSmbStatus()
	if err != nil {
		return "", err
	}
	return executeCommand(loc, args...)
}

func executeCommand(command string, arg ...string) (string, error) {
	cmd := exec.Command(command, arg...)
	out, err := cmd.Output()
	if err != nil {
		return string(out), err
	}
	res := strings.TrimSpace(string(out))
	return res, nil
}

// parseSmbStatusShares parses to output of 'smbstatus -S' into internal
// representation.
func parseSmbStatusShares(data string) ([]SmbStatusShare, error) {
	shares := []SmbStatusShare{}
	serviceIndex := 0
	pidIndex := 0
	machineIndex := 0
	connectedAtIndex := 0
	encryptionIndex := 0
	signingIndex := 0
	hasDashLine := false
	lines := strings.Split(data, "\n")
	for _, line := range lines {
		ln := strings.TrimSpace(line)
		// Ignore empty and coment lines
		if len(ln) == 0 || ln[0] == '#' {
			continue
		}
		// Detect the all-dash line
		if strings.HasPrefix(ln, "------") {
			hasDashLine = true
			continue
		}
		// Parse header line into index of data
		if strings.HasPrefix(ln, "Service") {
			serviceIndex = strings.Index(ln, "Service")
			pidIndex = strings.Index(ln, "pid")
			machineIndex = strings.Index(ln, "Machine")
			connectedAtIndex = strings.Index(ln, "Connected at")
			encryptionIndex = strings.Index(ln, "Encryption")
			signingIndex = strings.Index(ln, "Signing")
			continue
		}
		// Ignore lines before header
		if !hasDashLine {
			continue
		}
		// Parse data into internal repr
		share := SmbStatusShare{}
		share.Service = parseSubstr(ln, serviceIndex)
		pid, err := parseInt64(ln, pidIndex)
		if err != nil {
			return shares, err
		}
		share.PID = pid
		share.Machine = parseSubstr(ln, machineIndex)
		share.ConnectedAt = parseSubstr2(ln, connectedAtIndex, encryptionIndex)
		share.Encryption = parseSubstr(ln, encryptionIndex)
		share.Signing = parseSubstr(ln, signingIndex)
		if t, err := parseTime(share.ConnectedAt); err == nil {
			share.ConnectedTime = t
		}

		// Ignore "IPC$"
		if share.Service == "IPC$" {
			continue
		}

		shares = append(shares, share)
	}
	return shares, nil
}

// parseSmbStatusProcs parses to output of 'smbstatus -p' into internal
// representation.
func parseSmbStatusProcs(data string) ([]SmbStatusProc, error) {
	procs := []SmbStatusProc{}
	pidIndex := 0
	usernameIndex := 0
	groupIndex := 0
	machineIndex := 0
	protocolVersionIndex := 0
	encryptionIndex := 0
	signingIndex := 0
	hasDashLine := false
	lines := strings.Split(data, "\n")
	for _, line := range lines {
		ln := strings.TrimSpace(line)
		// Ignore empty and coment lines
		if len(ln) == 0 || ln[0] == '#' {
			continue
		}
		// Detect the all-dash line
		if strings.HasPrefix(ln, "------") {
			hasDashLine = true
			continue
		}
		// Parse header line into index of data
		if strings.HasPrefix(ln, "PID") {
			pidIndex = strings.Index(ln, "PID")
			usernameIndex = strings.Index(ln, "Username")
			groupIndex = strings.Index(ln, "Group")
			machineIndex = strings.Index(ln, "Machine")
			protocolVersionIndex = strings.Index(ln, "Protocol Version")
			encryptionIndex = strings.Index(ln, "Encryption")
			signingIndex = strings.Index(ln, "Signing")
			continue
		}
		// Ignore lines before header
		if !hasDashLine {
			continue
		}
		// Parse data into internal repr
		proc := SmbStatusProc{}
		pid, err := parseInt64(ln, pidIndex)
		if err != nil {
			return procs, err
		}
		proc.PID = pid
		proc.Username = parseSubstr(ln, usernameIndex)
		proc.Group = parseSubstr(ln, groupIndex)
		proc.Machine = parseSubstr(ln, machineIndex)
		proc.ProtocolVersion = parseSubstr(ln, protocolVersionIndex)
		proc.Encryption = parseSubstr(ln, encryptionIndex)
		proc.Signing = parseSubstr(ln, signingIndex)
		procs = append(procs, proc)
	}
	return procs, nil
}

// parseSmbStatusLocks parses to output of 'smbstatus -L' into internal
// representation.
func parseSmbStatusLocks(data string) ([]SmbStatusLock, error) {
	locks := []SmbStatusLock{}
	pidIndex := 0
	userIndex := 0
	denyModeIndex := 0
	accessIndex := 0
	rwIndex := 0
	oplockIndex := 0
	sharePathIndex := 0
	hasDashLine := false
	lines := strings.Split(data, "\n")
	for _, line := range lines {
		ln := strings.TrimSpace(line)
		// Ignore empty and coment lines
		if len(ln) == 0 || ln[0] == '#' {
			continue
		}
		// Detect the all-dash line
		if strings.HasPrefix(ln, "------") {
			hasDashLine = true
			continue
		}
		// Ignore generic-info line
		if strings.HasPrefix(ln, "Locked files") {
			continue
		}
		// Parse header line into index of data
		if strings.HasPrefix(ln, "Pid") {
			pidIndex = strings.Index(ln, "Pid")
			userIndex = strings.Index(ln, "User")
			denyModeIndex = strings.Index(ln, "DenyMode")
			accessIndex = strings.Index(ln, "Access")
			rwIndex = strings.Index(ln, "R/W")
			oplockIndex = strings.Index(ln, "Oplock")
			sharePathIndex = strings.Index(ln, "SharePath")
			continue
		}
		// Ignore lines before header
		if !hasDashLine {
			continue
		}
		// Parse data into internal repr
		lock := SmbStatusLock{}
		pid, err := parseInt64(ln, pidIndex)
		if err != nil {
			return locks, err
		}
		lock.PID = pid
		user, err := parseInt64(ln, userIndex)
		if err != nil {
			return locks, err
		}
		lock.UserID = user
		lock.DenyMode = parseSubstr(ln, denyModeIndex)
		lock.Access = parseSubstr(ln, accessIndex)
		lock.RW = parseSubstr(ln, rwIndex)
		lock.Oplock = parseSubstr(ln, oplockIndex)
		lock.SharePath = parseSubstr(ln, sharePathIndex)
		locks = append(locks, lock)
	}
	return locks, nil
}

func parseInt64(s string, startIndex int) (int64, error) {
	return strconv.ParseInt(parseSubstr(s, startIndex), 10, 64)
}

func parseSubstr(s string, startIndex int) string {
	sub := strings.TrimSpace(s[startIndex:])
	fields := strings.Fields(sub)
	if len(fields) == 0 {
		return ""
	}
	return fields[0]
}

func parseSubstr2(s string, startIndex, endIndex int) string {
	return strings.TrimSpace(s[startIndex:endIndex])
}

func parseTime(s string) (time.Time, error) {
	layouts := []string{
		time.ANSIC,
		time.UnixDate,
		time.RFC3339,
	}
	for _, layout := range layouts {
		if t, err := time.Parse(layout, s); err == nil {
			return t, nil
		}
	}
	// samba's lib/util/time.c uses non standad layout...
	return time.Time{}, errors.New("unknow time format " + s)
}

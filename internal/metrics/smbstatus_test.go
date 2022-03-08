// SPDX-License-Identifier: Apache-2.0

package metrics

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

//revive:disable line-length-limit
//nolint:revive,lll
var (
	outputSmbStatusS = `

	Service      pid     Machine       Connected at                     Encryption   Signing
	----------------------------------------------------------------------------------------
	share_test   13668   10.66.208.149 Wed Sep 27 10:33:55 AM 2017 CST  -            -
	share_test2  13669   10.66.208.149 Wed Sep 27 10:35:56 AM 2022 CST  -            -
	IPC$         651248  10.0.0.100    Sat Sep  4 10:37:01 AM 2020 MDT  -            -

`

	outputSmbStatusp = `

Samba version 4.14.12
PID     Username     Group        Machine                                   Protocol Version  Encryption           Signing
----------------------------------------------------------------------------------------------------------------------------------------
245     user         user         127.0.0.1 (ipv4:127.0.0.1:55106)          SMB3_11           -                    partial(AES-128-CMAC)
9701    root         wheel        10.0.0.2 (ipv4:10.0.0.2:55107)            SMB3_12           -                    partial(AES-128-CMAC)

`

	outputSmbStatusL = `

Locked files:
Pid          User(ID)   DenyMode   Access      R/W        Oplock           SharePath   Name   Time
--------------------------------------------------------------------------------------------------
241          1001       DENY_NONE  0x120089    RDONLY     LEASE(RWH)       /mnt/514cd7ba-d858-4d3a-bed9-68e4e524493b   A/a   Mon Feb 21 13:07:46 2022

`
)

//revive:enable line-length-limit

func TestParseSmbStatusShares(t *testing.T) {
	locks, err := parseSmbStatusShares(outputSmbStatusS)
	assert.NoError(t, err)
	assert.Equal(t, len(locks), 2)
	lock1 := locks[0]
	assert.Equal(t, lock1.Service, "share_test")
	assert.Equal(t, lock1.PID, int64(13668))
	assert.Equal(t, lock1.Machine, "10.66.208.149")
	assert.Equal(t, lock1.Encryption, "-")
	assert.Equal(t, lock1.Signing, "-")
}

func TestParseSmbStatusProcs(t *testing.T) {
	procs, err := parseSmbStatusProcs(outputSmbStatusp)
	assert.NoError(t, err)
	assert.Equal(t, len(procs), 2)
	proc1 := procs[0]
	assert.Equal(t, proc1.PID, int64(245))
	assert.Equal(t, proc1.Username, "user")
	assert.Equal(t, proc1.Group, "user")
	assert.Equal(t, proc1.ProtocolVersion, "SMB3_11")
	proc2 := procs[1]
	assert.Equal(t, proc2.PID, int64(9701))
	assert.Equal(t, proc2.Username, "root")
	assert.Equal(t, proc2.Group, "wheel")
	assert.Equal(t, proc2.ProtocolVersion, "SMB3_12")
}

func TestParseSmbStatusLocks(t *testing.T) {
	locks, err := parseSmbStatusLocks(outputSmbStatusL)
	assert.NoError(t, err)
	assert.Equal(t, len(locks), 1)
	lock1 := locks[0]
	assert.Equal(t, lock1.PID, int64(241))
	assert.Equal(t, lock1.UserID, int64(1001))
	assert.Equal(t, lock1.DenyMode, "DENY_NONE")
	assert.Equal(t, lock1.RW, "RDONLY")
}

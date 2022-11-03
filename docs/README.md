# Samba Operator Documentation

## Introduction

The samba-operator is a Kubernetes operator designed to export other storage
layers as SMB shares. These SMB shares can be accessed from within the
Kubernetes cluster hosting the samba-operator or from outside the Kubernetes
cluster.  The shares may be enabled for Active Directory authentication.
Multiple shares can be served by a single server instance to reduce resource
consumption.  An experimental feature allows the shares to be served by a CTDB
enabled cluster of Samba servers.

The samba-operator is designed around the idea that what people are interested
in are shares and the data within them; that servers are an implementation
detail irrelevant for most users.  With
this in mind, the samba-operator's primary resource is the SmbShare.  When
moving beyond the most basic cases, it provides the SmbSecurityConfig resource
for configuring Active Directory integration or specifying the users/groups for
authentication and access. The SmbCommonConfig resource is used to define
shared networking and Kubernetes cluster integration properties. Both
SmbSecurityConfig and SmbCommonConfig can be referenced by multiple SmbShare
resources.

## User Docs

* [Shares HOWTO](./howto.md) -
  How to configure SmbShare and supporting resources.
* [Presentations](./presentations/README.md) -
  Public Presentations on Samba Operator / Running Samba in Kubernetes


## Developer Docs

* [Developer Notes](./developers-notes.md) -
  Tips for working on the Samba Operator
* [Design Proposal Phase 1](./design/crd-proposal-phase1.md) -
  Design and planning for the operator from Oct. 2020 to current day

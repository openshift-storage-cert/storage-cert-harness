---
type: test-requirement
title: "TR-VIRT-007: Node Failure VM Recovery"
description: >-
  Validate VM recovery after a node failure. The harness provisions VMs on a
  target worker, then virtbench monitors recovery and reports pass/fail based
  on all VMs returning to Running and ping-reachable.
tags: [openshift-virt, storage, vm, ha, node-failure, recovery, virtbench, partner-level-3]
resource: TR-VIRT-007
timestamp: 2026-09-08T00:00:00Z
---

# TR-VIRT-007: Node Failure VM Recovery

## Summary

Node failure and VM recovery via the `virtbench failure-recovery` scenario.
The harness creates VMs on the configured target worker, or selects a worker
when no target is supplied. It removes the placement constraint before the
failure and supports either externally triggered remediation (`monitor`) or a
Fence Agents Remediation manifest (`far-operator`). virtbench measures how
long the VMs take to return to Running and become ping-reachable.

| Field | Value |
|-------|-------|
| Partner level | 3 |
| Verified | unverified |
| Automation tool | virtbench failure-recovery |
| SLA status | to-be-validated |
| Checks status | to-be-validated |

## Prerequisites

- OpenShift Virtualization (KubeVirt) installed
- A node-remediation mechanism configured: FAR for `far-operator` mode, or an
  external fencing/remediation mechanism for `monitor` mode
- At least 2 Ready worker nodes with `kubevirt.io/schedulable: true`
- **After the test**: FAR cleanup uncordons the remediated node and teardown
  removes the test namespaces.

## How it works

1. Harness provisions `vm_count` VMs on the configured or selected worker
   (required node placement ensures all VMs land on the same node).
2. Once all VMs are Running the harness removes the affinity, making them
   portable to any surviving node.
3. In `far-operator` mode virtbench applies the supplied FAR manifest; in
   `monitor` mode failure is triggered externally.
4. The remediation mechanism fences the target and KubeVirt reschedules the
   VMIs on surviving workers.
5. virtbench records VM restart and ping recovery times.
6. Cleanup removes the test namespaces and, for FAR, removes the remediation
   configuration and uncordons the target.

## Key params

| Param | Default | Notes |
|-------|---------|-------|
| `vm_count` | `3` | Number of VMs provisioned on the target node |
| `mode` | `monitor` | `monitor` or `far-operator` |
| `node` | empty | Target worker; empty selects a Ready, schedulable worker |
| `far_config` | empty | FAR manifest path required in `far-operator` mode |
| `vm_name` | `rhel-9-vm` | Name of each VM (one per namespace `{prefix}-{i}`) |
| `namespace_prefix` | `failure-recovery` | Namespace prefix |
| `vm_create_timeout` | `300` | Seconds to wait for each VM to reach Running at creation |
| `vm_recovery_timeout` | (virtbench default) | Seconds virtbench waits for VMs to recover |
| `node_restore_timeout` | `600` | Seconds teardown waits for the powered-off node before giving up |

## SLA metrics

| Metric | Description | Unit |
|--------|-------------|------|
| `vm_ha_restart_time` | Avg time for VMs to reach Running on surviving node | s |
| `time_to_ping` | Avg time for VMs to be ping-reachable after recovery | s |

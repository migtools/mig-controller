## Migrating a *stateful* OpenShift app with *Amazon EBS PVs*

This scenario walks through Migration of a stateful OpenShift app whose Persistent Volume (PV) claims tie into [Amazon EBS (Elastic Block Storage)](https://aws.amazon.com/ebs/).

### Supported PV Actions - EBS to EBS

| Action | Supported | Description |
|-----------|------------|-------------|
| Copy | Yes | EBS snapshots will be created from source cluster PVs. New EBS PVs for destination cluster will be created from snapshots |
| Move | No  | ... |




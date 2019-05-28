## Migrating a *stateful* OpenShift app with *NFS PVs* to *Amazon EBS PVs* (or vice-versa)

This scenario walks through Migration of a stateful OpenShift app with Persistent Volume (PV) claims tied to NFS or Amazon EBS. Through the Migration process, data stored on [NFS](https://en.wikipedia.org/wiki/Network_File_System) or [Amazon EBS (Elastic Block Storage)](https://aws.amazon.com/ebs/) can be transferred to a different PV storage medium (NFS or EBS). 


### Supported PV Actions - NFS <=> EBS

| Action | Supported | Description |
|-----------|------------|-------------|
| Copy | Yes | Create new PV on destination cluster. Restic will copy contents of source PV to destination PV |
| Move | No  | ... |


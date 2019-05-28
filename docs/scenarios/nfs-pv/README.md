## Migrating a *stateful* OpenShift app with *NFS PVs*

This scenario walks through Migration of a stateful OpenShift app with Persistent Volume (PV) claims tied into [NFS (Network File System)](https://en.wikipedia.org/wiki/Network_File_System).

### Supported PV Actions - NFS to NFS

| Action | Supported | Description |
|-----------|------------|-------------|
| Copy | Yes | Create new PV on destination cluster. Restic will copy data from source PV to destination PV |
| Move | Yes  | Detach PV from source cluster, then re-attach to destination cluster without copying data |




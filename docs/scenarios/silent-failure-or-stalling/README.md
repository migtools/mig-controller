### Possible Failure/Stalling scenarios

This document contains the notes of detailed analysis of a few scenarios
which the mig-controller v1.3.0 might stall/fail silently. 
This document further consists of reasons why those failure cases are
considered as edge cases  

#### Unhealthy Stage Pods

If stage pods are created and healthy, but they are in bad state when
velero backup is created, the migration will be stalled until restic timeout.

Steps to attempted to reproduce this
1. Start a stage migration
2. Start a migplan, find the node which houses one of the stage pods
3. Just when the migration passes EnsureStagePodsCreated, cordern the source
 node 

##### Conclusions after trying to reproduce: 

1. The mig-controller specifically assigns the name of the node where the
stage pod is supposed to be run. Because of this, the stage pod will never go
to pending state after it has gone to running state.
2. If the node goes to unhealthy state, it can be due to following reasons:
    1. The node has been disconnected from the actual cluster. In this case a
       taint will be applied to the node which will mark it `unreachable
       `. Because it is not reachable, the kubelet does not
       deterministically know the state of the container, whether it is
       running/pending. Hence it will not act on the state of the Pod.
    2. The node has gone to NoSchedule mode. The NoSchedule mode is
       indication to the kube schedular that no more pods should be schedule
       on this node. However, the existing pods will continue to run. This
       case will not affect us because we dont use the scheduler and
       specify the node name for stage pods ourselves.
    3. The node has gone to NoExecute mode. In NoExecute mode, the kubelet
       will terminate all the pods running on that node in the cluster. 
       Because of this our stage pods will be terminated, and the migration 
       will continue to use the VolumeSnapshot mechanism if it is available in
       the cluster. However the likeliness of this “stall” the migration 
       process is rare. In my attempts to reproduce this, the migration
        continued as if nothing happened.
        

In addition to the above mentioned reasons, all we do in the stage container
pod is sleep. Hence, in 1.4.0 we have tightened the 
[check](https://github.com/konveyor/mig-controller/pull/747) that sees whether
stage pod is running, and once it is determined to be running, the 
mig-controller assumes with relatively low risk that it continues to run.

#### AnnotateResources blocks for long time

AnnotateResources phase can take a long time if we have large number of 
imagestreams and serviceaccounts in source namespace.

Steps to attempted to reproduce this:
1. Create 1k dummy service accounts in the source namespace
2. Attempt to migrate it

##### Conclusions after trying to reproduce:

The migration spent less than a second in the “AnnotateResources” phase. The 
reason for this was that the controller only annotates the service accounts 
which are required by the pods to be migrated. 

It was reasoned that although the cluster could have lingering RBAC resources, 
but the serviceaccounts which are required by the running pods will not be in
the order of tens of thousands. Hence, the likeliness of migration appeared to
be stalled in this phase is likely very low. The proposed solutions for 
this introduces complexities that we would be better off not to pursue on this 
edge case.

#### Velero pod is not working/stalled

1. Assume that the velero pod is not working (crashlooping or facing RBAC
 issues list/watching for objects). This will stall all migrations.
2. Assume that a lot of velero objects are created. Since, velero
 implementation is non-concurrent, it will lead to "Head of Line" blocking.
 There is work in v1.4.0 to garbage collect and clear up the line to avoid
  stalling. 

##### Conclustion

Because of progress reporting, if a migration is stalled at velero operation,
it will be much more transperant to the user. The user can restart the velero
pod on source/destination to see if that remedies the situation. In most cases
it might. In the worst case scenario, users will be able to identify and file
a bug.

      

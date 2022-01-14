package directvolumemigration

import (
	"context"
	"math"

	liberr "github.com/konveyor/controller/pkg/error"
	migapi "github.com/konveyor/mig-controller/pkg/apis/migration/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	"k8s.io/apimachinery/pkg/types"
	k8sclient "sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	TRANSFER_POD_CPU_LIMIT       = "TRANSFER_POD_CPU_LIMIT"
	TRANSFER_POD_MEMORY_LIMIT    = "TRANSFER_POD_MEMORY_LIMIT"
	TRANSFER_POD_CPU_REQUESTS    = "TRANSFER_POD_CPU_REQUEST"
	TRANSFER_POD_MEMORY_REQUESTS = "TRANSFER_POD_MEMORY_REQUEST"
	CLIENT_POD_CPU_LIMIT         = "CLIENT_POD_CPU_LIMIT"
	CLIENT_POD_MEMORY_LIMIT      = "CLIENT_POD_MEMORY_LIMIT"
	CLIENT_POD_CPU_REQUESTS      = "CLIENT_POD_CPU_REQUEST"
	CLIENT_POD_MEMORY_REQUESTS   = "CLIENT_POD_MEMORY_REQUEST"
	STUNNEL_POD_CPU_LIMIT        = "STUNNEL_POD_CPU_LIMIT"
	STUNNEL_POD_MEMORY_LIMIT     = "STUNNEL_POD_MEMORY_LIMIT"
	STUNNEL_POD_CPU_REQUEST      = "STUNNEL_POD_CPU_REQUEST"
	STUNNEL_POD_MEMORY_REQUEST   = "STUNNEL_POD_MEMORY_REQUEST"
)

// getDefaultResourceRequirements returns default resource requirements for DVM Pods
func getDefaultResourceRequirements() *corev1.ResourceRequirements {
	return &corev1.ResourceRequirements{
		Requests: corev1.ResourceList{
			corev1.ResourceMemory: resource.MustParse("512Mi"),
			corev1.ResourceCPU:    resource.MustParse("400m"),
		},
		Limits: corev1.ResourceList{},
	}
}

// mergeResourceLimitRangeItems given target and source LimitRangeItems, merges source into target. it
// compares the Max & Min resource limits of CPU and memory and keeps the most restrictive values
// NOTE: does not consider MaxLimitRequestRatio for comparison
func mergeResourceLimitRangeItems(target *corev1.LimitRangeItem, source *corev1.LimitRangeItem) {
	if target != nil && source != nil {
		if source.Max != nil {
			// for max limits, the most restrictive value is the one which is the minimum
			if target.Max.Cpu().Cmp(*source.Max.Cpu()) > 0 {
				target.Max[corev1.ResourceCPU] = source.Max[corev1.ResourceCPU]
			}
			if target.Max.Memory().Cmp(*source.Max.Memory()) > 0 {
				target.Max[corev1.ResourceMemory] = source.Max[corev1.ResourceMemory]
			}
		}
		if source.Min != nil {
			// for min limits, the most restrictive value is the one which is the maximum
			if target.Min.Cpu().Cmp(*source.Min.Cpu()) < 0 {
				target.Min[corev1.ResourceCPU] = source.Min[corev1.ResourceCPU]
			}
			if target.Min.Memory().Cmp(*source.Min.Memory()) < 0 {
				target.Min[corev1.ResourceMemory] = source.Min[corev1.ResourceMemory]
			}
		}
	}
}

// getLimitRangeForNamespace given a namespace and a client, iterates over all LimitRanges present in the ns
// returns final limit range consolidated into a single LimitRange object for Pod and Container separately
// returns nil if no LimitRanges are present in the namespace
// each limit value is the most restrictive value over all LimitRanges present in the namespace
func (t *Task) getLimitRangeForNamespace(ns string, client k8sclient.Client) (*corev1.LimitRange, error) {
	limitRangeList := corev1.LimitRangeList{}
	err := client.List(context.TODO(), &limitRangeList, k8sclient.InNamespace(ns))
	if err != nil {
		return nil, liberr.Wrap(err)
	}
	if len(limitRangeList.Items) == 0 {
		return nil, nil
	}
	finalLimitRange := &corev1.LimitRange{}
	var podLimitRangeItem *corev1.LimitRangeItem
	var containerLimitRangeItem *corev1.LimitRangeItem
	for _, limitRange := range limitRangeList.Items {
		for i := range limitRange.Spec.Limits {
			limitItem := &limitRange.Spec.Limits[i]
			switch limitItem.Type {
			case corev1.LimitTypeContainer:
				if containerLimitRangeItem == nil {
					containerLimitRangeItem = limitItem
				} else {
					mergeResourceLimitRangeItems(containerLimitRangeItem, limitItem)
				}
			case corev1.LimitTypePod:
				if podLimitRangeItem == nil {
					podLimitRangeItem = limitItem
				} else {
					mergeResourceLimitRangeItems(podLimitRangeItem, limitItem)
				}
			}
		}
	}
	if podLimitRangeItem != nil {
		finalLimitRange.Spec.Limits = append(finalLimitRange.Spec.Limits, *podLimitRangeItem)
	}
	if containerLimitRangeItem != nil {
		finalLimitRange.Spec.Limits = append(finalLimitRange.Spec.Limits, *containerLimitRangeItem)
	}
	return finalLimitRange, nil
}

// buildLimitRangeMap builds a map of limit ranges for all source and destination namespaces
func (t *Task) buildLimitRangeMap() error {
	nsMap, err := t.getNamespacedPVCPairs()
	if err != nil {
		return liberr.Wrap(err)
	}
	srcClient, err := t.getSourceClient()
	if err != nil {
		return liberr.Wrap(err)
	}
	destClient, err := t.getDestinationClient()
	if err != nil {
		return liberr.Wrap(err)
	}
	for bothNs := range nsMap {
		srcNs := getSourceNs(bothNs)
		destNs := getDestNs(bothNs)
		if _, exists := t.SourceLimitRangeMapping[srcNs]; !exists {
			limitRange, err := t.getLimitRangeForNamespace(srcNs, srcClient)
			if err != nil {
				return liberr.Wrap(err)
			}
			if limitRange != nil {
				t.SourceLimitRangeMapping[srcNs] = *limitRange
			}
		}
		if _, exists := t.DestinationLimitRangeMapping[destNs]; !exists {
			limitRange, err := t.getLimitRangeForNamespace(destNs, destClient)
			if err != nil {
				return liberr.Wrap(err)
			}
			if limitRange != nil {
				t.DestinationLimitRangeMapping[destNs] = *limitRange
			}
		}
	}
	return nil
}

// applyLimitRangeItemOnRequirements given an item from LimitRange, applies it on the given ResourceRequirement
func applyLimitRangeItemOnRequirements(requirements *corev1.ResourceRequirements, limit corev1.LimitRangeItem) {
	if requirements.Requests != nil {
		if limit.Max != nil {
			//  apply Max limits on Requests and Limits, none should exceed the Max
			if requirements.Requests.Cpu().Cmp(*limit.Max.Cpu()) > 0 {
				requirements.Requests[corev1.ResourceCPU] = *limit.Max.Cpu()
			}
			if requirements.Requests.Memory().Cmp(*limit.Max.Memory()) > 0 {
				requirements.Requests[corev1.ResourceMemory] = *limit.Max.Memory()
			}
		}
		if limit.Min != nil {
			// apply Min limits on Requests and Limits, none should be less than the Min
			if requirements.Requests.Cpu().Cmp(*limit.Min.Cpu()) < 0 {
				requirements.Requests[corev1.ResourceCPU] = *limit.Min.Cpu()
			}
			if requirements.Requests.Memory().Cmp(*limit.Min.Memory()) < 0 {
				requirements.Requests[corev1.ResourceMemory] = *limit.Min.Memory()
			}
		}
	}
	if requirements.Limits != nil {
		if limit.Max != nil {
			//  apply Max limits on Requests and Limits, none should exceed the Max
			if requirements.Limits.Cpu().Cmp(*limit.Max.Cpu()) > 0 {
				requirements.Limits[corev1.ResourceCPU] = *limit.Max.Cpu()
			}
			if requirements.Limits.Memory().Cmp(*limit.Max.Memory()) > 0 {
				requirements.Limits[corev1.ResourceMemory] = *limit.Max.Memory()
			}
		}
	}
	// if limits are smaller than requests, delete limit values
	if requirements.Limits.Cpu().Cmp(*requirements.Requests.Cpu()) < 0 {
		delete(requirements.Limits, corev1.ResourceCPU)
	}
	if requirements.Limits.Memory().Cmp(*requirements.Requests.Memory()) < 0 {
		delete(requirements.Limits, corev1.ResourceMemory)
	}
}

// getScaledDownQuantity given a quantity & a scale, returns a new quantity which
// is (1 / scale) times of original quantity in magnitude, preserves the format
func getScaledDownQuantity(q *resource.Quantity, scale int64) *resource.Quantity {
	var scaledQuantity *resource.Quantity
	if q == nil || (scale < 1) {
		return scaledQuantity
	}
	if (q.Value() < 0) ||
		(q.MilliValue() > ((q.Value() / scale) * int64(math.Pow(10, 3)))) {
		scaledQuantity = resource.NewMilliQuantity(
			int64(q.MilliValue()/scale),
			q.Format,
		)
	} else {
		scaledQuantity = resource.NewQuantity(
			int64(q.Value()/scale),
			q.Format,
		)
	}
	return scaledQuantity
}

// getScaledResourceList given a ResourceList, returns a new ResourceList with
// all resource values half in magnitude of the values in the original ResourceList
func getScaledResourceList(resourceList corev1.ResourceList) corev1.ResourceList {
	var scaledResourceList corev1.ResourceList
	var scaledCPUResource *resource.Quantity
	var scaledMemResource *resource.Quantity
	if resourceList.Cpu() != nil {
		scaledCPUResource = getScaledDownQuantity(resourceList.Cpu(), 2)
	}
	if resourceList.Memory() != nil {
		scaledMemResource = getScaledDownQuantity(resourceList.Memory(), 2)
	}
	if scaledCPUResource != nil {
		if scaledResourceList == nil {
			scaledResourceList = make(corev1.ResourceList)
		}
		scaledResourceList[corev1.ResourceCPU] = *scaledCPUResource
	}
	if scaledMemResource != nil {
		if scaledResourceList == nil {
			scaledResourceList = make(corev1.ResourceList)
		}
		scaledResourceList[corev1.ResourceMemory] = *scaledMemResource
	}
	return scaledResourceList
}

// applyLimitRangeOnRequirements givem a ResourceRequirements & a LimitRange, applies Container & Pod limitranges on original Requirements
// LimitRange can contain a LimitRangeItem of ContainerType or a PodType or both. when there's only one LimitRangeItem, there are two cases:
// 1. LimitRangeItem is ContainerType: in this case, the limits are applied as-is on the ResourceRequirements
// 2. LimitRangeItem is PodType: in this case, the Limits are further scaled down to half of their original values, because DVM Pods contain
//    two containers each. Therefore, Limits are spread across two containers so that total ResourceRequirements of the Pod remains in limit
// When there are two LimitRangeItems of type Container & Pod both, we only consider the limits present on Container to avoid errorsome math
func applyLimitRangeOnRequirements(requirements *corev1.ResourceRequirements, limitRange corev1.LimitRange) {
	if len(limitRange.Spec.Limits) == 1 {
		limit := limitRange.Spec.Limits[0]
		switch limit.Type {
		case corev1.LimitTypeContainer:
			applyLimitRangeItemOnRequirements(requirements, limit)
		case corev1.LimitTypePod:
			var scaledMaxResourceList corev1.ResourceList
			var scaledMinResourceList corev1.ResourceList
			if limit.Max != nil {
				scaledMaxResourceList = getScaledResourceList(limit.Max)
			}
			if limit.Min != nil {
				scaledMinResourceList = getScaledResourceList(limit.Min)
			}
			scaledLimitRange := &corev1.LimitRangeItem{
				Max: scaledMaxResourceList,
				Min: scaledMinResourceList,
			}
			applyLimitRangeItemOnRequirements(requirements, *scaledLimitRange)
		}
	} else {
		for _, limit := range limitRange.Spec.Limits {
			if limit.Type == corev1.LimitTypeContainer {
				applyLimitRangeItemOnRequirements(requirements, limit)
			}
		}
	}
}

// getRsyncClientResourceRequirements returns resource requirements for Rsync Client Pods (source cluster)
func (t *Task) getRsyncClientResourceRequirements(ns string, client k8sclient.Client) (corev1.ResourceRequirements, error) {
	requirements, err := t.getUserConfiguredResourceRequirements(
		CLIENT_POD_CPU_LIMIT, CLIENT_POD_MEMORY_LIMIT, CLIENT_POD_CPU_REQUESTS, CLIENT_POD_MEMORY_REQUESTS)
	if err != nil {
		return requirements, liberr.Wrap(err)
	}
	if limitRange, exists := t.SourceLimitRangeMapping[ns]; exists {
		applyLimitRangeOnRequirements(&requirements, limitRange)
	}
	return requirements, nil
}

// getRsyncServerResourceRequirements returns resource requirements for Rsync Server Pods (target cluster)
func (t *Task) getRsyncServerResourceRequirements(ns string, client k8sclient.Client) (corev1.ResourceRequirements, error) {
	requirements, err := t.getUserConfiguredResourceRequirements(
		TRANSFER_POD_CPU_LIMIT, TRANSFER_POD_MEMORY_LIMIT, TRANSFER_POD_CPU_REQUESTS, TRANSFER_POD_MEMORY_REQUESTS)
	if err != nil {
		return requirements, liberr.Wrap(err)
	}
	if limitRange, exists := t.SourceLimitRangeMapping[ns]; exists {
		applyLimitRangeOnRequirements(&requirements, limitRange)
	}
	return requirements, nil
}

// getUserConfiguredResourceRequirements given keys for user configured Resource Requirements of DVM Pods,
// reads controller ConfigMap to look for user configured values and populates a ResourceRequirements object
// for any value not configured by the user, uses default value
func (t *Task) getUserConfiguredResourceRequirements(cpuLimit string, memoryLimit string, cpuRequests string, memRequests string) (corev1.ResourceRequirements, error) {
	podConfigMap := &corev1.ConfigMap{}
	requirements := corev1.ResourceRequirements{}
	err := t.Client.Get(context.TODO(), types.NamespacedName{
		Name: "migration-controller", Namespace: migapi.OpenshiftMigrationNamespace}, podConfigMap)
	if err != nil {
		return requirements, err
	}
	limits := getDefaultResourceRequirements().Limits
	if val, exists := podConfigMap.Data[cpuLimit]; exists {
		cpu, err := resource.ParseQuantity(val)
		if err != nil {
			return requirements, err
		}
		limits[corev1.ResourceCPU] = cpu
	}
	if val, exists := podConfigMap.Data[memoryLimit]; exists {
		memory, err := resource.ParseQuantity(val)
		if err != nil {
			return requirements, err
		}
		limits[corev1.ResourceMemory] = memory
	}
	requests := getDefaultResourceRequirements().Requests
	if val, exists := podConfigMap.Data[cpuRequests]; exists {
		cpu, err := resource.ParseQuantity(val)
		if err != nil {
			return requirements, err
		}
		requests[corev1.ResourceCPU] = cpu
	}
	if val, exists := podConfigMap.Data[memRequests]; exists {
		memory, err := resource.ParseQuantity(val)
		if err != nil {
			return requirements, err
		}
		requests[corev1.ResourceMemory] = memory
	}
	requirements.Requests = requests
	requirements.Limits = limits
	return requirements, nil
}

/*
Copyright 2021 Red Hat Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package miganalytic

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"

	liberr "github.com/konveyor/controller/pkg/error"
	migapi "github.com/konveyor/mig-controller/pkg/apis/migration/v1alpha1"
	"github.com/pkg/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const DEFAULT_PV_USAGE_THRESHOLD = 3

// DFCommandExecutor defines an executor responsible for running DF
type DFCommandExecutor interface {
	Execute(map[string][]MigAnalyticPersistentVolumeDetails) ([]DFOutput, error)
}

// PersistentVolumeAdjuster defines volume adjustment context
type PersistentVolumeAdjuster struct {
	Owner          *migapi.MigAnalytic
	Client         client.Client
	DFExecutor     DFCommandExecutor
	StatusRefCache map[string]*migapi.MigAnalyticNamespace
}

// DFBaseUnit defines supported sizes for df command
type DFBaseUnit string

// Supported base units used for df command
const (
	DecimalSIGiga = DFBaseUnit("GB")
	DecimalSIMega = DFBaseUnit("MB")
	BinarySIMega  = DFBaseUnit("M")
	BinarySIGiga  = DFBaseUnit("G")
)

// DFCommand represent a df command
type DFCommand struct {
	// stdout from df
	StdOut string
	// stderr from df
	StdErr string
	// Base unit used for df
	BlockSize DFBaseUnit
	// BaseLocation defines path where volumes can be found
	BaseLocation string
}

// DFDiskPath defines format of expected path of the volume present on Pod
const DFDiskPath = "%s/%s/volumes/*/%s"

// DFOutput defines structured output of df per PV
type DFOutput struct {
	Node            string
	Name            string
	Namespace       string
	UsagePercentage int64
	TotalSize       resource.Quantity
	IsError         bool
}

// convertDFQuantityToKubernetesResource converts a quantity present in df output to resource.Quantity
func (cmd *DFCommand) convertDFQuantityToKubernetesResource(quantity string) (resource.Quantity, error) {
	var parsedQuantity resource.Quantity
	unitMatcher, _ := regexp.Compile(
		fmt.Sprintf(
			"(\\d+)%s", cmd.BlockSize))
	matched := unitMatcher.FindStringSubmatch(quantity)
	if len(matched) != 2 {
		return parsedQuantity, errors.Errorf("Invalid quantity or block size unit")
	}
	switch cmd.BlockSize {
	case DecimalSIGiga, DecimalSIMega:
		quantity = strings.ReplaceAll(quantity, "B", "")
		break
	case BinarySIGiga, BinarySIMega:
		quantity = fmt.Sprintf("%si", quantity)
		break
	}
	return resource.ParseQuantity(quantity)
}

// GetDFOutputForPV given a volume name and pod uid, returns structured df ouput for the volume
// only works on outputs of commands created by DFCommand.PrepareDFCommand()
func (cmd *DFCommand) GetDFOutputForPV(volName string, podUID types.UID) (pv DFOutput) {
	var err error
	stdOutLines, stdErrLines := strings.Split(cmd.StdOut, "\n"), strings.Split(cmd.StdErr, "\n")
	lineMatcher, _ := regexp.Compile(
		fmt.Sprintf(
			strings.Replace(DFDiskPath, "*", ".*", 1),
			cmd.BaseLocation,
			podUID,
			volName))
	percentageMatcher, _ := regexp.Compile("(\\d+)%")
	for _, line := range stdOutLines {
		if lineMatcher.MatchString(line) {
			cols := strings.Fields(line)
			if len(cols) != 6 {
				pv.IsError = true
				return
			}
			pv.TotalSize, err = cmd.convertDFQuantityToKubernetesResource(cols[1])
			pv.IsError = (err != nil)
			matched := percentageMatcher.FindStringSubmatch(cols[4])
			if len(matched) > 1 {
				pv.UsagePercentage, err = strconv.ParseInt(matched[1], 10, 64)
				pv.IsError = (err != nil)
			}
			return
		}
	}
	for _, line := range stdErrLines {
		if lineMatcher.MatchString(line) {
			pv.IsError = true
			return
		}
	}
	// didn't find the PV in stdout or stderr
	pv.IsError = true
	return
}

// PrepareDFCommand given a list of volumes, creates a bulk df command for all volumes
func (cmd *DFCommand) PrepareDFCommand(pvcs []MigAnalyticPersistentVolumeDetails) []string {
	command := []string{
		"/bin/bash",
		"-c",
	}
	volPaths := []string{}
	for _, pvc := range pvcs {
		volPaths = append(volPaths,
			fmt.Sprintf(
				DFDiskPath,
				cmd.BaseLocation,
				pvc.PodUID,
				pvc.VolumeName))
	}
	return append(command, fmt.Sprintf("df -B%s %s", cmd.BlockSize, strings.Join(volPaths, " ")))
}

// findOriginalPVDataMatchingDFOutput given a df output for a pv and nested map of nodeName->[]pvc, finds ref to matching object in the map
func (pva *PersistentVolumeAdjuster) findOriginalPVDataMatchingDFOutput(pvc DFOutput, pvcNodeMap map[string][]MigAnalyticPersistentVolumeDetails) *MigAnalyticPersistentVolumeDetails {
	if _, exists := pvcNodeMap[pvc.Node]; exists {
		for i := range pvcNodeMap[pvc.Node] {
			pvDetailRef := &pvcNodeMap[pvc.Node][i]
			if pvc.Name == pvDetailRef.Name && pvc.Namespace == pvDetailRef.Namespace {
				return pvDetailRef
			}
		}
	}
	return nil
}

// generateWarningForErroredPVs given a list of dfoutputs, generates warnings for associated pvcs
func (pva *PersistentVolumeAdjuster) generateWarningForErroredPVs(erroredPVs []*DFOutput) {
	warningString := "Failed gathering extended PV usage information for PVs [%s]"
	pvNames := []string{}
	for _, dfOutput := range erroredPVs {
		if dfOutput.IsError {
			pvNames = append(pvNames, dfOutput.Name)
		}
	}
	if len(pvNames) > 0 {
		msg := fmt.Sprintf(warningString, strings.Join(pvNames, " "))
		log.Info(msg)
		pva.Owner.Status.Conditions.SetCondition(migapi.Condition{
			Category: migapi.Warn,
			Status:   True,
			Type:     migapi.ExtendedPVAnalysisFailed,
			Reason:   migapi.FailedRunningDf,
			Message:  msg,
		})
	}
}

// getRefToAnalyticNs given a ns, returns the reference to correspoding ns present within MigAnalytic.Status.Analytics, stores found ones in cache
func (pva *PersistentVolumeAdjuster) getRefToAnalyticNs(namespace string) *migapi.MigAnalyticNamespace {
	if pva.StatusRefCache == nil {
		pva.StatusRefCache = make(map[string]*migapi.MigAnalyticNamespace)
	}
	if nsRef, exists := pva.StatusRefCache[namespace]; exists {
		return nsRef
	}
	for i := range pva.Owner.Status.Analytics.Namespaces {
		nsRef := &pva.Owner.Status.Analytics.Namespaces[i]
		if namespace == nsRef.Namespace {
			pva.StatusRefCache[namespace] = nsRef
			return nsRef
		}
	}
	return nil
}

// Run runs executor, uses df output to calculate adjusted volume sizes, updates owner.status with results
func (pva *PersistentVolumeAdjuster) Run(pvNodeMap map[string][]MigAnalyticPersistentVolumeDetails) error {
	pvDfOutputs, err := pva.DFExecutor.Execute(pvNodeMap)
	if err != nil {
		return liberr.Wrap(err)
	}
	erroredPVs := []*DFOutput{}
	for i, pvDfOutput := range pvDfOutputs {
		originalData := pva.findOriginalPVDataMatchingDFOutput(pvDfOutput, pvNodeMap)
		if originalData == nil {
			// TODO: handle this case better
			continue
		}
		migAnalyticNSRef := pva.getRefToAnalyticNs(originalData.Namespace)
		statusFieldUpdate := migapi.MigAnalyticPersistentVolumeClaim{
			Name:              originalData.Name,
			RequestedCapacity: originalData.RequestedCapacity,
			Comment:           migapi.VolumeAdjustmentNoOp,
		}
		if pvDfOutput.IsError {
			erroredPVs = append(erroredPVs, &pvDfOutputs[i])
		} else {
			statusFieldUpdate.ActualCapacity = pvDfOutput.TotalSize
			proposedCapacity, reason := pva.calculateProposedVolumeSize(pvDfOutput.UsagePercentage, pvDfOutput.TotalSize, originalData.RequestedCapacity)
			// make sure we never set a value smaller than original provisioned capacity
			if originalData.ProvisionedCapacity.Cmp(proposedCapacity) >= 1 {
				statusFieldUpdate.ProposedCapacity = originalData.ProvisionedCapacity
			} else {
				statusFieldUpdate.ProposedCapacity = proposedCapacity
			}
			statusFieldUpdate.Comment = reason
		}
		migAnalyticNSRef.PersistentVolumes = append(migAnalyticNSRef.PersistentVolumes, statusFieldUpdate)
	}
	pva.generateWarningForErroredPVs(erroredPVs)
	return nil
}

func (pva *PersistentVolumeAdjuster) calculateProposedVolumeSize(usagePercentage int64, actualCapacity resource.Quantity,
	requestedCapacity resource.Quantity) (proposedSize resource.Quantity, reason string) {

	defer proposedSize.String()

	volumeSizeWithThreshold := actualCapacity
	volumeSizeWithThreshold.Set(
		int64(actualCapacity.Value() *
			(usagePercentage + int64(pva.getVolumeUsagePercentageThreshold())) / 100))

	maxSize := requestedCapacity
	reason = migapi.VolumeAdjustmentNoOp

	if actualCapacity.Cmp(maxSize) == 1 {
		maxSize = actualCapacity
		reason = migapi.VolumeAdjustmentCapacityMismatch
	}

	if volumeSizeWithThreshold.Cmp(maxSize) == 1 {
		maxSize = volumeSizeWithThreshold
		reason = migapi.VolumeAdjustmentUsageExceeded
	}

	proposedSize = maxSize

	return proposedSize, reason
}

// getVolumeUsagePercentageThreshold returns configured threshold for pv usage
func (pva *PersistentVolumeAdjuster) getVolumeUsagePercentageThreshold() int {
	if Settings != nil && Settings.PVResizingVolumeUsageThreshold > 0 && Settings.PVResizingVolumeUsageThreshold < 100 {
		return Settings.PVResizingVolumeUsageThreshold
	}
	return DEFAULT_PV_USAGE_THRESHOLD
}

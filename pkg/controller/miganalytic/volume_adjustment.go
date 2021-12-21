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
	Execute(map[string][]MigAnalyticPersistentVolumeDetails) ([]DFOutput, []DUOutput, error)
}

// PersistentVolumeAdjuster defines volume adjustment context
type PersistentVolumeAdjuster struct {
	Owner          *migapi.MigAnalytic
	Client         client.Client
	DFExecutor     DFCommandExecutor
	StatusRefCache map[string]*migapi.MigAnalyticNamespace
}

// BaseUnit defines supported sizes for df command
type BaseUnit string

// Supported base units used for df command
const (
	DecimalSIGiga = BaseUnit("GB")
	DecimalSIMega = BaseUnit("MB")
	BinarySIMega  = BaseUnit("M")
	BinarySIGiga  = BaseUnit("G")
)

type StorageCommand struct {
	// stdout from df
	StdOut string
	// stderr from df
	StdErr string
	// Base unit used for df
	BlockSize BaseUnit
	// BaseLocation defines path where volumes can be found
	BaseLocation string
}

// DF represent a df command
type DF struct {
	StorageCommand
}

type DU struct {
	StorageCommand
}

// VolumePath defines format of expected path of the volume present on Pod
const VolumePath = "%s/%s/volumes/*/%s"

type StorageCommandOutput struct {
	UsagePercentage int64
	Usage           int64
	TotalSize       resource.Quantity
	IsError         bool
}

// DFOutput defines structured output of df per PV
type DFOutput struct {
	Node      string
	Name      string
	Namespace string
	StorageCommandOutput
}

type DUOutput struct {
	Name      string
	Namespace string
	StorageCommandOutput
}

// convertLinuxQuantityToKubernetesQuantity converts a quantity present in df output to resource.Quantity
func (s *StorageCommand) convertLinuxQuantityToKubernetesQuantity(quantity string) (resource.Quantity, error) {
	var parsedQuantity resource.Quantity
	unitMatcher, _ := regexp.Compile(
		fmt.Sprintf(
			"(\\d+)%s", s.BlockSize))
	matched := unitMatcher.FindStringSubmatch(quantity)
	if len(matched) != 2 {
		return parsedQuantity, errors.Errorf("Invalid quantity or block size unit")
	}
	switch s.BlockSize {
	case DecimalSIGiga, DecimalSIMega:
		quantity = strings.ReplaceAll(quantity, "B", "")
		break
	case BinarySIGiga, BinarySIMega:
		quantity = fmt.Sprintf("%si", quantity)
		break
	}
	return resource.ParseQuantity(quantity)
}

// GetOutputForPV given a volume name and pod uid, returns structured df ouput for the volume
// only works on outputs of commands created by DFCommand.PrepareDFCommand()
func (d *DF) GetOutputForPV(volName string, podUID types.UID) (dfOutput DFOutput) {
	var err error
	stdOutLines, stdErrLines := strings.Split(d.StdOut, "\n"), strings.Split(d.StdErr, "\n")
	lineMatcher, _ := regexp.Compile(
		fmt.Sprintf(
			strings.Replace(VolumePath, "*", ".*", 1),
			d.BaseLocation,
			podUID,
			volName))
	percentageMatcher, _ := regexp.Compile("(\\d+)%")
	for _, line := range stdOutLines {
		if lineMatcher.MatchString(line) {
			cols := strings.Fields(line)
			if len(cols) != 6 {
				dfOutput.IsError = true
				return
			}
			dfOutput.TotalSize, err = d.convertLinuxQuantityToKubernetesQuantity(cols[1])
			dfOutput.IsError = (err != nil)
			unitMatcher := regexp.MustCompile(string(d.BlockSize))
			usageString := unitMatcher.ReplaceAllString(cols[2], "")
			dfOutput.Usage, err = strconv.ParseInt(usageString, 10, 64)
			dfOutput.IsError = (err != nil)
			matched := percentageMatcher.FindStringSubmatch(cols[4])
			if len(matched) > 1 {
				dfOutput.UsagePercentage, err = strconv.ParseInt(matched[1], 10, 64)
				dfOutput.IsError = (err != nil)
			}
			return
		}
	}
	for _, line := range stdErrLines {
		if lineMatcher.MatchString(line) {
			dfOutput.IsError = true
			return
		}
	}
	// didn't find the PV in stdout or stderr
	dfOutput.IsError = true
	return
}

// PrepareCommand given a list of volumes, creates a bulk df command for all volumes
func (d *DF) PrepareCommand(pvcs []MigAnalyticPersistentVolumeDetails) []string {
	command := []string{
		"/bin/bash",
		"-c",
	}
	volPaths := []string{}
	for _, pvc := range pvcs {
		volPaths = append(volPaths,
			fmt.Sprintf(
				strings.Replace(VolumePath, "*", ".*", 1),
				d.BaseLocation,
				pvc.PodUID,
				pvc.VolumeName))
	}
	return append(command, fmt.Sprintf("df -B%s | grep -E \"(%s)\"", d.BlockSize, strings.Join(volPaths, "|")))
}

// GetOutputForPV given a volume name and pod uid, returns structured df ouput for the volume
// only works on outputs of commands created by DFCommand.PrepareDFCommand()
func (d *DU) GetOutputForPV(volName string, podUID types.UID) (duOutput DUOutput) {
	var err error
	stdOutLines, stdErrLines := strings.Split(d.StdOut, "\n"), strings.Split(d.StdErr, "\n")
	lineMatcher, _ := regexp.Compile(
		fmt.Sprintf(
			strings.Replace(VolumePath, "*", ".*", 1),
			d.BaseLocation,
			podUID,
			volName))
	for _, line := range stdOutLines {
		if lineMatcher.MatchString(line) {
			cols := strings.Fields(line)
			if len(cols) != 2 {
				duOutput.IsError = true
				return
			}
			unitMatcher := regexp.MustCompile(string(d.BlockSize))
			usageString := unitMatcher.ReplaceAllString(cols[0], "")
			duOutput.Usage, err = strconv.ParseInt(usageString, 10, 64)
			duOutput.IsError = (err != nil)
			return
		}
	}
	for _, line := range stdErrLines {
		if lineMatcher.MatchString(line) {
			duOutput.IsError = true
			return
		}
	}
	// didn't find the PV in stdout or stderr
	duOutput.IsError = true
	return
}

// PrepareCommand given a list of volumes, creates a bulk df command for all volumes
func (d *DU) PrepareCommand(pvcs []MigAnalyticPersistentVolumeDetails) []string {
	command := []string{
		"/bin/bash",
		"-c",
	}
	volPaths := []string{}
	for _, pvc := range pvcs {
		volPaths = append(volPaths,
			fmt.Sprintf(
				VolumePath,
				d.BaseLocation,
				pvc.PodUID,
				pvc.VolumeName))
	}
	return append(command, fmt.Sprintf("du --max-depth=0 --apparent-size --block-size=%s %s", d.BlockSize, strings.Join(volPaths, " ")))
}

// findOriginalPVDataMatchingDFOutput given a df output for a pv and nested map of nodeName->[]pvc, finds ref to matching object in the map
func (p *PersistentVolumeAdjuster) findOriginalPVDataMatchingDFOutput(pvc DFOutput, pvcNodeMap map[string][]MigAnalyticPersistentVolumeDetails) *MigAnalyticPersistentVolumeDetails {
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
func (p *PersistentVolumeAdjuster) generateWarningForErroredPVs(erroredPVs []*DFOutput) {
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
		p.Owner.Status.Conditions.SetCondition(migapi.Condition{
			Category: migapi.Warn,
			Status:   True,
			Type:     migapi.ExtendedPVAnalysisFailed,
			Reason:   migapi.FailedRunningDf,
			Message:  msg,
		})
	}
}

func (p *PersistentVolumeAdjuster) generateWarningForSparseFileFailure(erroredPVs []string) {
	warningString := "Failed identifying whether sparse files exist on PVs [%s]"
	if len(erroredPVs) > 0 {
		msg := fmt.Sprintf(warningString, strings.Join(erroredPVs, " "))
		log.Info(msg)
		p.Owner.Status.Conditions.SetCondition(migapi.Condition{
			Category: migapi.Warn,
			Status:   True,
			Type:     migapi.ExtendedPVAnalysisFailed,
			Reason:   migapi.FailedRunningDu,
			Message:  msg,
		})
	}
}

// getRefToAnalyticNs given a ns, returns the reference to correspoding ns present within MigAnalytic.Status.Analytics, stores found ones in cache
func (p *PersistentVolumeAdjuster) getRefToAnalyticNs(namespace string) *migapi.MigAnalyticNamespace {
	if p.StatusRefCache == nil {
		p.StatusRefCache = make(map[string]*migapi.MigAnalyticNamespace)
	}
	if nsRef, exists := p.StatusRefCache[namespace]; exists {
		return nsRef
	}
	for i := range p.Owner.Status.Analytics.Namespaces {
		nsRef := &p.Owner.Status.Analytics.Namespaces[i]
		if namespace == nsRef.Namespace {
			p.StatusRefCache[namespace] = nsRef
			return nsRef
		}
	}
	return nil
}

// Run runs executor, uses df output to calculate adjusted volume sizes, updates owner.status with results
func (p *PersistentVolumeAdjuster) Run(pvNodeMap map[string][]MigAnalyticPersistentVolumeDetails) error {
	pvDfOutputs, pvDuOutputs, err := p.DFExecutor.Execute(pvNodeMap)
	if err != nil {
		return liberr.Wrap(err)
	}
	erroredPVs := []*DFOutput{}
	erroredDuPVs := []string{}
	for i, pvDfOutput := range pvDfOutputs {
		originalData := p.findOriginalPVDataMatchingDFOutput(pvDfOutput, pvNodeMap)
		duData := p.findDUOutputMatchingDFOutput(pvDfOutput, pvDuOutputs)
		if originalData == nil {
			// TODO: handle this case better
			continue
		}
		migAnalyticNSRef := p.getRefToAnalyticNs(originalData.Namespace)
		statusFieldUpdate := migapi.MigAnalyticPersistentVolumeClaim{
			Name:              originalData.Name,
			RequestedCapacity: originalData.RequestedCapacity,
			Comment:           migapi.VolumeAdjustmentNoOp,
		}
		if pvDfOutput.IsError {
			erroredPVs = append(erroredPVs, &pvDfOutputs[i])
		} else {
			statusFieldUpdate.ActualCapacity = pvDfOutput.TotalSize
			proposedCapacity, reason := p.calculateProposedVolumeSize(
				pvDfOutput.UsagePercentage, pvDfOutput.TotalSize,
				originalData.RequestedCapacity, originalData.ProvisionedCapacity)
			// make sure we never set a value smaller than original provisioned capacity
			if originalData.ProvisionedCapacity.Cmp(proposedCapacity) >= 1 {
				statusFieldUpdate.ProposedCapacity = originalData.ProvisionedCapacity
			} else {
				statusFieldUpdate.ProposedCapacity = proposedCapacity
			}
			statusFieldUpdate.Comment = reason
			if duData != nil && !duData.IsError {
				// apparent file size is more than the block size reported by df
				// command. this indicates that there could be sparse files in
				// the volume making apparent size bigger than actual usage
				if duData.Usage > pvDfOutput.Usage {
					statusFieldUpdate.SparseFilesFound = true
					log.Info("Sparse files found in volume",
						"persistentVolume", fmt.Sprintf("%s/%s", pvDfOutput.Namespace, statusFieldUpdate.Name))
				}
			} else {
				erroredDuPVs = append(erroredDuPVs,
					fmt.Sprintf("%s/%s", duData.Namespace, duData.Name))
			}
		}
		migAnalyticNSRef.PersistentVolumes = append(migAnalyticNSRef.PersistentVolumes, statusFieldUpdate)
	}
	p.generateWarningForErroredPVs(erroredPVs)
	p.generateWarningForSparseFileFailure(erroredDuPVs)
	return nil
}

func (p *PersistentVolumeAdjuster) findDUOutputMatchingDFOutput(dfOutput DFOutput, duOutputs []DUOutput) *DUOutput {
	for i, _ := range duOutputs {
		duOutput := &duOutputs[i]
		if dfOutput.Namespace == duOutput.Namespace {
			if dfOutput.Name == duOutput.Name {
				return duOutput
			}
		}
	}
	return nil
}

func (p *PersistentVolumeAdjuster) calculateProposedVolumeSize(
	usagePercentage int64, actualCapacity resource.Quantity,
	requestedCapacity resource.Quantity, provisionedCapacity resource.Quantity) (proposedSize resource.Quantity, reason string) {

	defer proposedSize.String()

	maxSize := requestedCapacity
	reason = migapi.VolumeAdjustmentNoOp

	if actualCapacity.Cmp(maxSize) == 1 {
		maxSize = actualCapacity
		reason = migapi.VolumeAdjustmentCapacityMismatch
	}

	if provisionedCapacity.Cmp(maxSize) == 1 {
		maxSize = provisionedCapacity
		reason = migapi.VolumeAdjustmentCapacityMismatch
	}

	if usagePercentage+int64(p.getVolumeUsagePercentageThreshold()) > 100 {
		reason = migapi.VolumeAdjustmentUsageExceeded
		volumeSizeWithThreshold := maxSize
		volumeSizeWithThreshold.Set(
			int64(actualCapacity.Value() *
				(usagePercentage + int64(p.getVolumeUsagePercentageThreshold())) / 100))
		maxSize = volumeSizeWithThreshold
	}

	proposedSize = maxSize

	return proposedSize, reason
}

// getVolumeUsagePercentageThreshold returns configured threshold for pv usage
func (p *PersistentVolumeAdjuster) getVolumeUsagePercentageThreshold() int {
	if Settings != nil && Settings.PVResizingVolumeUsageThreshold > 0 && Settings.PVResizingVolumeUsageThreshold < 100 {
		return Settings.PVResizingVolumeUsageThreshold
	}
	return DEFAULT_PV_USAGE_THRESHOLD
}

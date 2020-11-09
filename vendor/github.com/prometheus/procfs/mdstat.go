// Copyright 2018 The Prometheus Authors
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
// http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package procfs

import (
	"fmt"
	"io/ioutil"
	"regexp"
	"strconv"
	"strings"
)

var (
<<<<<<< HEAD
	statusLineRE   = regexp.MustCompile(`(\d+) blocks .*\[(\d+)/(\d+)\] \[[U_]+\]`)
	recoveryLineRE = regexp.MustCompile(`\((\d+)/\d+\)`)
=======
	statuslineRE = regexp.MustCompile(`(\d+) blocks .*\[(\d+)/(\d+)\] \[[U_]+\]`)
	buildlineRE  = regexp.MustCompile(`\((\d+)/\d+\)`)
>>>>>>> cbc9bb05... fixup add vendor back
)

// MDStat holds info parsed from /proc/mdstat.
type MDStat struct {
	// Name of the device.
	Name string
	// activity-state of the device.
	ActivityState string
	// Number of active disks.
	DisksActive int64
<<<<<<< HEAD
	// Total number of disks the device requires.
	DisksTotal int64
	// Number of failed disks.
	DisksFailed int64
	// Spare disks in the device.
	DisksSpare int64
=======
	// Total number of disks the device consists of.
	DisksTotal int64
>>>>>>> cbc9bb05... fixup add vendor back
	// Number of blocks the device holds.
	BlocksTotal int64
	// Number of blocks on the device that are in sync.
	BlocksSynced int64
}

<<<<<<< HEAD
// MDStat parses an mdstat-file (/proc/mdstat) and returns a slice of
// structs containing the relevant info.  More information available here:
// https://raid.wiki.kernel.org/index.php/Mdstat
func (fs FS) MDStat() ([]MDStat, error) {
	data, err := ioutil.ReadFile(fs.proc.Path("mdstat"))
	if err != nil {
		return nil, fmt.Errorf("error parsing mdstat %s: %s", fs.proc.Path("mdstat"), err)
	}
	mdstat, err := parseMDStat(data)
	if err != nil {
		return nil, fmt.Errorf("error parsing mdstat %s: %s", fs.proc.Path("mdstat"), err)
	}
	return mdstat, nil
}

// parseMDStat parses data from mdstat file (/proc/mdstat) and returns a slice of
// structs containing the relevant info.
func parseMDStat(mdStatData []byte) ([]MDStat, error) {
	mdStats := []MDStat{}
	lines := strings.Split(string(mdStatData), "\n")

	for i, line := range lines {
		if strings.TrimSpace(line) == "" || line[0] == ' ' ||
			strings.HasPrefix(line, "Personalities") ||
			strings.HasPrefix(line, "unused") {
			continue
		}

		deviceFields := strings.Fields(line)
		if len(deviceFields) < 3 {
			return nil, fmt.Errorf("not enough fields in mdline (expected at least 3): %s", line)
		}
		mdName := deviceFields[0] // mdx
		state := deviceFields[2]  // active or inactive

		if len(lines) <= i+3 {
			return nil, fmt.Errorf(
				"error parsing %s: too few lines for md device",
=======
// ParseMDStat parses an mdstat-file and returns a struct with the relevant infos.
func (fs FS) ParseMDStat() (mdstates []MDStat, err error) {
	mdStatusFilePath := fs.Path("mdstat")
	content, err := ioutil.ReadFile(mdStatusFilePath)
	if err != nil {
		return []MDStat{}, fmt.Errorf("error parsing %s: %s", mdStatusFilePath, err)
	}

	mdStates := []MDStat{}
	lines := strings.Split(string(content), "\n")
	for i, l := range lines {
		if l == "" {
			continue
		}
		if l[0] == ' ' {
			continue
		}
		if strings.HasPrefix(l, "Personalities") || strings.HasPrefix(l, "unused") {
			continue
		}

		mainLine := strings.Split(l, " ")
		if len(mainLine) < 3 {
			return mdStates, fmt.Errorf("error parsing mdline: %s", l)
		}
		mdName := mainLine[0]
		activityState := mainLine[2]

		if len(lines) <= i+3 {
			return mdStates, fmt.Errorf(
				"error parsing %s: too few lines for md device %s",
				mdStatusFilePath,
>>>>>>> cbc9bb05... fixup add vendor back
				mdName,
			)
		}

<<<<<<< HEAD
		// Failed disks have the suffix (F) & Spare disks have the suffix (S).
		fail := int64(strings.Count(line, "(F)"))
		spare := int64(strings.Count(line, "(S)"))
		active, total, size, err := evalStatusLine(lines[i], lines[i+1])

		if err != nil {
			return nil, fmt.Errorf("error parsing md device lines: %s", err)
		}

		syncLineIdx := i + 2
		if strings.Contains(lines[i+2], "bitmap") { // skip bitmap line
			syncLineIdx++
=======
		active, total, size, err := evalStatusline(lines[i+1])
		if err != nil {
			return mdStates, fmt.Errorf("error parsing %s: %s", mdStatusFilePath, err)
		}

		// j is the line number of the syncing-line.
		j := i + 2
		if strings.Contains(lines[i+2], "bitmap") { // skip bitmap line
			j = i + 3
>>>>>>> cbc9bb05... fixup add vendor back
		}

		// If device is syncing at the moment, get the number of currently
		// synced bytes, otherwise that number equals the size of the device.
		syncedBlocks := size
<<<<<<< HEAD
		recovering := strings.Contains(lines[syncLineIdx], "recovery")
		resyncing := strings.Contains(lines[syncLineIdx], "resync")

		// Append recovery and resyncing state info.
		if recovering || resyncing {
			if recovering {
				state = "recovering"
			} else {
				state = "resyncing"
			}

			// Handle case when resync=PENDING or resync=DELAYED.
			if strings.Contains(lines[syncLineIdx], "PENDING") ||
				strings.Contains(lines[syncLineIdx], "DELAYED") {
				syncedBlocks = 0
			} else {
				syncedBlocks, err = evalRecoveryLine(lines[syncLineIdx])
				if err != nil {
					return nil, fmt.Errorf("error parsing sync line in md device %s: %s", mdName, err)
				}
			}
		}

		mdStats = append(mdStats, MDStat{
			Name:          mdName,
			ActivityState: state,
			DisksActive:   active,
			DisksFailed:   fail,
			DisksSpare:    spare,
=======
		if strings.Contains(lines[j], "recovery") || strings.Contains(lines[j], "resync") {
			syncedBlocks, err = evalBuildline(lines[j])
			if err != nil {
				return mdStates, fmt.Errorf("error parsing %s: %s", mdStatusFilePath, err)
			}
		}

		mdStates = append(mdStates, MDStat{
			Name:          mdName,
			ActivityState: activityState,
			DisksActive:   active,
>>>>>>> cbc9bb05... fixup add vendor back
			DisksTotal:    total,
			BlocksTotal:   size,
			BlocksSynced:  syncedBlocks,
		})
	}

<<<<<<< HEAD
	return mdStats, nil
}

func evalStatusLine(deviceLine, statusLine string) (active, total, size int64, err error) {

	sizeStr := strings.Fields(statusLine)[0]
	size, err = strconv.ParseInt(sizeStr, 10, 64)
	if err != nil {
		return 0, 0, 0, fmt.Errorf("unexpected statusLine %s: %s", statusLine, err)
	}

	if strings.Contains(deviceLine, "raid0") || strings.Contains(deviceLine, "linear") {
		// In the device deviceLine, only disks have a number associated with them in [].
		total = int64(strings.Count(deviceLine, "["))
		return total, total, size, nil
	}

	if strings.Contains(deviceLine, "inactive") {
		return 0, 0, size, nil
	}

	matches := statusLineRE.FindStringSubmatch(statusLine)
	if len(matches) != 4 {
		return 0, 0, 0, fmt.Errorf("couldn't find all the substring matches: %s", statusLine)
=======
	return mdStates, nil
}

func evalStatusline(statusline string) (active, total, size int64, err error) {
	matches := statuslineRE.FindStringSubmatch(statusline)
	if len(matches) != 4 {
		return 0, 0, 0, fmt.Errorf("unexpected statusline: %s", statusline)
	}

	size, err = strconv.ParseInt(matches[1], 10, 64)
	if err != nil {
		return 0, 0, 0, fmt.Errorf("unexpected statusline %s: %s", statusline, err)
>>>>>>> cbc9bb05... fixup add vendor back
	}

	total, err = strconv.ParseInt(matches[2], 10, 64)
	if err != nil {
<<<<<<< HEAD
		return 0, 0, 0, fmt.Errorf("unexpected statusLine %s: %s", statusLine, err)
=======
		return 0, 0, 0, fmt.Errorf("unexpected statusline %s: %s", statusline, err)
>>>>>>> cbc9bb05... fixup add vendor back
	}

	active, err = strconv.ParseInt(matches[3], 10, 64)
	if err != nil {
<<<<<<< HEAD
		return 0, 0, 0, fmt.Errorf("unexpected statusLine %s: %s", statusLine, err)
=======
		return 0, 0, 0, fmt.Errorf("unexpected statusline %s: %s", statusline, err)
>>>>>>> cbc9bb05... fixup add vendor back
	}

	return active, total, size, nil
}

<<<<<<< HEAD
func evalRecoveryLine(recoveryLine string) (syncedBlocks int64, err error) {
	matches := recoveryLineRE.FindStringSubmatch(recoveryLine)
	if len(matches) != 2 {
		return 0, fmt.Errorf("unexpected recoveryLine: %s", recoveryLine)
=======
func evalBuildline(buildline string) (syncedBlocks int64, err error) {
	matches := buildlineRE.FindStringSubmatch(buildline)
	if len(matches) != 2 {
		return 0, fmt.Errorf("unexpected buildline: %s", buildline)
>>>>>>> cbc9bb05... fixup add vendor back
	}

	syncedBlocks, err = strconv.ParseInt(matches[1], 10, 64)
	if err != nil {
<<<<<<< HEAD
		return 0, fmt.Errorf("%s in recoveryLine: %s", err, recoveryLine)
=======
		return 0, fmt.Errorf("%s in buildline: %s", err, buildline)
>>>>>>> cbc9bb05... fixup add vendor back
	}

	return syncedBlocks, nil
}

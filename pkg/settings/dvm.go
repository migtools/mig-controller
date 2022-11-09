package settings

import (
	"fmt"
	"os"
	"strconv"
	"strings"
)

// DVM options
const (
	RsyncOptBwLimit               = "RSYNC_OPT_BWLIMIT"
	RsyncOptPartial               = "RSYNC_OPT_PARTIAL"
	RsyncOptArchive               = "RSYNC_OPT_ARCHIVE"
	RsyncOptDelete                = "RSYNC_OPT_DELETE"
	RsyncOptHardLinks             = "RSYNC_OPT_HARDLINKS"
	RsyncOptInfo                  = "RSYNC_OPT_INFO"
	RsyncOptExtras                = "RSYNC_OPT_EXTRAS"
	RsyncBackOffLimit             = "RSYNC_BACKOFF_LIMIT"
	EnablePVResizing              = "ENABLE_DVM_PV_RESIZING"
	TCPProxyKey                   = "STUNNEL_TCP_PROXY"
	StunnelVerifyCAKey            = "STUNNEL_VERIFY_CA"
	StunnelVerifyCALevelKey       = "STUNNEL_VERIFY_CA_LEVEL"
	SourceSupplementalGroups      = "SOURCE_SUPPLEMENTAL_GROUPS"
	DestinationSupplementalGroups = "TARGET_SUPPLEMENTAL_GROUPS"
)

// RsyncOpts Rsync Options
//
//	BwLimit: equivalent to --bwlimit=<integer>
//	Archive: whether to set --archive option or not
//	Partial: whether to set --partial option or not
//	Delete:  whether to set --delete option or not
//	HardLinks: whether to set --hard-links option or not
//	Extras: arbitrary rsync options provided by the user
//	BackOffLimit: defines number of retries set on Rsync
type RsyncOpts struct {
	BwLimit      int
	Archive      bool
	Partial      bool
	Delete       bool
	HardLinks    bool
	Info         string
	Extras       []string
	BackOffLimit int
}

type FileOwnershipOpts struct {
	SourceSupplementalGroups      []int64
	DestinationSupplementalGroups []int64
}

func (f *FileOwnershipOpts) Load() error {
	var err error
	sourceSupplementalGroups := os.Getenv(SourceSupplementalGroups)
	f.SourceSupplementalGroups, err = parseIntListFromString(sourceSupplementalGroups)
	if err != nil {
		return fmt.Errorf("failed to parse source supplemental groups")
	}
	destSupplementalGroups := os.Getenv(DestinationSupplementalGroups)
	f.DestinationSupplementalGroups, err = parseIntListFromString(destSupplementalGroups)
	if err != nil {
		return fmt.Errorf("failed to parse target supplemental groups")
	}
	return nil
}

func parseIntListFromString(str string) ([]int64, error) {
	intList := []int64{}
	intStringList := strings.Split(str, ",")
	for _, intStr := range intStringList {
		if intStr == "" {
			continue
		}
		parsedInt, err := strconv.ParseInt(intStr, 10, 64)
		if err != nil {
			return nil, err
		}
		intList = append(intList, parsedInt)
	}
	return intList, nil
}

// DvmOpts DVM settings
type DvmOpts struct {
	RsyncOpts
	FileOwnershipOpts
	EnablePVResizing     bool
	StunnelTCPProxy      string
	StunnelVerifyCA      bool
	StunnelVerifyCALevel string
}

// Load load rsync options
func (r *RsyncOpts) Load() error {
	var err error
	r.BwLimit, err = getEnvLimit(RsyncOptBwLimit, -1)
	if err != nil {
		return err
	}
	r.Archive = getEnvBool(RsyncOptArchive, true)
	r.Partial = getEnvBool(RsyncOptPartial, true)
	r.Delete = getEnvBool(RsyncOptDelete, true)
	r.HardLinks = getEnvBool(RsyncOptHardLinks, true)
	infoOpts := os.Getenv(RsyncOptInfo)
	if len(infoOpts) > 0 {
		r.Info = infoOpts
	}
	rsyncExtraOpts := os.Getenv(RsyncOptExtras)
	if len(rsyncExtraOpts) > 0 {
		r.Extras = strings.Fields(rsyncExtraOpts)
	}
	r.BackOffLimit, err = getEnvLimit(RsyncBackOffLimit, 0)
	if err != nil {
		return err
	}
	return err
}

// Load loads DVM options
func (r *DvmOpts) Load() error {
	var err error
	r.EnablePVResizing = getEnvBool(EnablePVResizing, false)
	r.StunnelTCPProxy = os.Getenv(TCPProxyKey)
	r.StunnelVerifyCA = getEnvBool(StunnelVerifyCAKey, true)
	r.StunnelVerifyCALevel = os.Getenv(StunnelVerifyCALevelKey)
	if r.StunnelVerifyCALevel == "" {
		r.StunnelVerifyCALevel = "2"
	}
	err = r.RsyncOpts.Load()
	if err != nil {
		return err
	}
	err = r.FileOwnershipOpts.Load()
	if err != nil {
		return err
	}
	return nil
}

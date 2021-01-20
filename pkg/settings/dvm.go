package settings

import (
	"os"
	"strings"
)

// Rsync options
const (
	RsyncOptBwLimit           = "RSYNC_OPT_BWLIMIT"
	RsyncOptPartial           = "RSYNC_OPT_PARTIAL"
	RsyncOptArchive           = "RSYNC_OPT_ARCHIVE"
	RsyncOptDelete            = "RSYNC_OPT_DELETE"
	RsyncOptHardLinks         = "RSYNC_OPT_HARDLINKS"
	RsyncOptInfo              = "RSYNC_OPT_INFO"
	RsyncOptExtras            = "RSYNC_OPT_EXTRAS"
	RsyncTransferRouteTimeout = "RSYNC_TRANSFER_ROUTE_TIMEOUT"
)

// RsyncOpts Rsync Options
//	BwLimit: equivalent to --bwlimit=<integer>
//	Archive: whether to set --archive option or not
//	Partial: whether to set --partial option or not
//	Delete:  whether to set --delete option or not
//	HardLinks: whether to set --hard-links option or not
//	Info: equivalent to --info=<string> option
//	Extras: arbitrary rsync options provided by the user
type RsyncOpts struct {
	BwLimit   int
	Archive   bool
	Partial   bool
	Delete    bool
	HardLinks bool
	Info      string
	Extras    []string
}

// DvmOpts DVM global options
type DvmOpts struct {
	RsyncTransferRouteTimeout int
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
	return err
}

// Load load dvm options
func (d *DvmOpts) Load() error {
	var err error
	d.RsyncTransferRouteTimeout, err = getEnvLimit(RsyncTransferRouteTimeout, 30)
	if err != nil {
		return err
	}
	return err
}

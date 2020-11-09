package version

import "fmt"

const (
	// VersionMajor is for an API incompatible changes
	VersionMajor = 5
	// VersionMinor is for functionality in a backwards-compatible manner
<<<<<<< HEAD
	VersionMinor = 7
=======
	VersionMinor = 6
>>>>>>> cbc9bb05... fixup add vendor back
	// VersionPatch is for backwards-compatible bug fixes
	VersionPatch = 0

	// VersionDev indicates development branch. Releases will be empty string.
	VersionDev = ""
)

// Version is the specification version that the package types support.
var Version = fmt.Sprintf("%d.%d.%d%s", VersionMajor, VersionMinor, VersionPatch, VersionDev)

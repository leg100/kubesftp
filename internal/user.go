package internal

import "path/filepath"

type user struct {
	Username       string
	AuthorizedKeys []string `json:"authorized_keys"`
	AllowedHosts   []string
}

func (u user) String() string {
	return u.Username
}

// homeDir is the user's nominal home directory, i.e. the path once inside the
// chroot.
func (u user) homeDir() string {
	return filepath.Join("/home", u.Username)
}

// chrootHomeDir is the host's path to the user's home directory, i.e. the path
// outside of the chroot.
func (u user) chrootHomeDir() string {
	return filepath.Join(u.chrootDir(), "home", u.Username)
}

// chrootDir is the path to the user's dedicated chroot.
func (u user) chrootDir() string {
	return filepath.Join(chrootsDir, u.Username)
}

// devLogPath is the full host path to the user's log device.
func (u user) devLogPath() string {
	return filepath.Join(u.chrootDir(), "dev", "log")
}

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

// chrootDir is the path to the user's dedicated chroot.
func (u user) chrootDir(chrootsParentDir string) string {
	return filepath.Join(chrootsParentDir, u.Username)
}

// chrootHomeDir is the host's path to the user's home directory.
func (u user) chrootHomeDir(chrootsParentDir string) string {
	return filepath.Join(u.chrootDir(chrootsParentDir), "home", u.Username)
}

// devLogPath is the full host path to the user's log device.
func (u user) devLogPath(chrootsParentDir string) string {
	return filepath.Join(u.chrootDir(chrootsParentDir), "dev", "log")
}

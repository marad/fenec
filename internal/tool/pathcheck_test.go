package tool

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestIsDeniedPath_Etc(t *testing.T) {
	denied, err := IsDeniedPath("/etc/passwd")
	require.NoError(t, err)
	assert.True(t, denied)
}

func TestIsDeniedPath_EtcRoot(t *testing.T) {
	denied, err := IsDeniedPath("/etc")
	require.NoError(t, err)
	assert.True(t, denied)
}

func TestIsDeniedPath_EtceteraNotDenied(t *testing.T) {
	// /etcetera must NOT be matched by /etc prefix
	denied, err := IsDeniedPath("/etcetera")
	require.NoError(t, err)
	assert.False(t, denied)
}

func TestIsDeniedPath_Usr(t *testing.T) {
	denied, err := IsDeniedPath("/usr/bin/go")
	require.NoError(t, err)
	assert.True(t, denied)
}

func TestIsDeniedPath_Bin(t *testing.T) {
	denied, err := IsDeniedPath("/bin/sh")
	require.NoError(t, err)
	assert.True(t, denied)
}

func TestIsDeniedPath_Sbin(t *testing.T) {
	denied, err := IsDeniedPath("/sbin/init")
	require.NoError(t, err)
	assert.True(t, denied)
}

func TestIsDeniedPath_Boot(t *testing.T) {
	denied, err := IsDeniedPath("/boot/vmlinuz")
	require.NoError(t, err)
	assert.True(t, denied)
}

func TestIsDeniedPath_SSHDir(t *testing.T) {
	home, err := os.UserHomeDir()
	require.NoError(t, err)
	denied, pathErr := IsDeniedPath(filepath.Join(home, ".ssh", "id_rsa"))
	require.NoError(t, pathErr)
	assert.True(t, denied)
}

func TestIsDeniedPath_GnupgDir(t *testing.T) {
	home, err := os.UserHomeDir()
	require.NoError(t, err)
	denied, pathErr := IsDeniedPath(filepath.Join(home, ".gnupg", "pubring.kbx"))
	require.NoError(t, pathErr)
	assert.True(t, denied)
}

func TestIsDeniedPath_NormalPathAllowed(t *testing.T) {
	home, err := os.UserHomeDir()
	require.NoError(t, err)
	denied, pathErr := IsDeniedPath(filepath.Join(home, "projects", "file.go"))
	require.NoError(t, pathErr)
	assert.False(t, denied)
}

func TestIsDeniedPath_TmpAllowed(t *testing.T) {
	denied, err := IsDeniedPath("/tmp/test.txt")
	require.NoError(t, err)
	assert.False(t, denied)
}

func TestIsDeniedPath_SymlinkIntoDenied(t *testing.T) {
	// Create a symlink in a temp dir that points to /etc
	tmpDir := t.TempDir()
	link := filepath.Join(tmpDir, "sneaky")
	err := os.Symlink("/etc/hostname", link)
	require.NoError(t, err)

	denied, pathErr := IsDeniedPath(link)
	require.NoError(t, pathErr)
	assert.True(t, denied)
}

func TestIsOutsideCWD_RelativeInside(t *testing.T) {
	outside, err := IsOutsideCWD("./internal/tool/foo.go")
	require.NoError(t, err)
	assert.False(t, outside)
}

func TestIsOutsideCWD_AbsoluteOutside(t *testing.T) {
	outside, err := IsOutsideCWD("/tmp/outside.txt")
	require.NoError(t, err)
	assert.True(t, outside)
}

func TestIsOutsideCWD_RelativeEscape(t *testing.T) {
	outside, err := IsOutsideCWD("../sibling/file.go")
	require.NoError(t, err)
	assert.True(t, outside)
}

func TestIsOutsideCWD_CWDItself(t *testing.T) {
	cwd, err := os.Getwd()
	require.NoError(t, err)
	outside, pathErr := IsOutsideCWD(cwd)
	require.NoError(t, pathErr)
	assert.False(t, outside)
}

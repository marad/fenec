package tool

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestIsDangerous_rm(t *testing.T) {
	assert.True(t, IsDangerous("rm -rf /tmp/foo"))
}

func TestIsDangerous_sudo(t *testing.T) {
	assert.True(t, IsDangerous("sudo apt update"))
}

func TestIsDangerous_chmod(t *testing.T) {
	assert.True(t, IsDangerous("chmod 777 file.txt"))
}

func TestIsDangerous_redirect(t *testing.T) {
	assert.True(t, IsDangerous("echo hi > file.txt"))
}

func TestIsDangerous_mv(t *testing.T) {
	assert.True(t, IsDangerous("mv a.txt b.txt"))
}

func TestIsDangerous_kill(t *testing.T) {
	assert.True(t, IsDangerous("kill -9 1234"))
}

func TestIsDangerous_safe_ls(t *testing.T) {
	assert.False(t, IsDangerous("ls -la"))
}

func TestIsDangerous_safe_echo(t *testing.T) {
	assert.False(t, IsDangerous("echo hello"))
}

func TestIsDangerous_safe_cat(t *testing.T) {
	assert.False(t, IsDangerous("cat /etc/hostname"))
}

func TestIsDangerous_safe_grep(t *testing.T) {
	assert.False(t, IsDangerous("grep -r pattern ."))
}

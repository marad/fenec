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

// === Tests for the dd false-positive fix ===

func TestIsDangerous_dd_standalone(t *testing.T) {
	// dd as standalone command should be dangerous
	assert.True(t, IsDangerous("dd if=/dev/zero of=/dev/sda"))
}

func TestIsDangerous_dd_after_pipe(t *testing.T) {
	assert.True(t, IsDangerous("cat file | dd of=/dev/sda"))
}

func TestIsDangerous_dd_after_semicolon(t *testing.T) {
	assert.True(t, IsDangerous("echo hi; dd if=/dev/zero of=/dev/sda"))
}

func TestIsDangerous_dd_after_and(t *testing.T) {
	assert.True(t, IsDangerous("true && dd if=/dev/zero of=/dev/sda"))
}

func TestIsDangerous_dd_after_or(t *testing.T) {
	assert.True(t, IsDangerous("false || dd if=/dev/zero of=/dev/sda"))
}

func TestIsDangerous_dd_after_subshell(t *testing.T) {
	assert.True(t, IsDangerous("echo $(dd if=/dev/zero of=/dev/sda)"))
}

func TestIsDangerous_dd_after_backtick(t *testing.T) {
	assert.True(t, IsDangerous("echo `dd if=/dev/zero of=/dev/sda`"))
}

func TestIsDangerous_git_add_not_dangerous(t *testing.T) {
	// This was the original false positive
	assert.False(t, IsDangerous("git add ."))
}

func TestIsDangerous_git_add_A_not_dangerous(t *testing.T) {
	assert.False(t, IsDangerous("git add -A"))
}

func TestIsDangerous_git_add_file_not_dangerous(t *testing.T) {
	assert.False(t, IsDangerous("git add file.txt"))
}

func TestIsDangerous_npm_add_not_dangerous(t *testing.T) {
	assert.False(t, IsDangerous("npm add package"))
}

func TestIsDangerous_yarn_add_not_dangerous(t *testing.T) {
	assert.False(t, IsDangerous("yarn add package"))
}

func TestIsDangerous_useradd_not_dangerous_for_dd(t *testing.T) {
	// useradd contains "dd" substring but should not match "dd " pattern
	// (it might match other patterns like sudo if prefixed, but not dd itself)
	assert.False(t, IsDangerous("useradd testuser"))
}

// === Tests for reboot/shutdown boundary matching ===

func TestIsDangerous_reboot_standalone(t *testing.T) {
	assert.True(t, IsDangerous("reboot"))
}

func TestIsDangerous_shutdown_standalone(t *testing.T) {
	assert.True(t, IsDangerous("shutdown -h now"))
}

func TestIsDangerous_reboot_after_separator(t *testing.T) {
	assert.True(t, IsDangerous("echo done; reboot"))
}

func TestIsDangerous_shutdown_after_separator(t *testing.T) {
	assert.True(t, IsDangerous("echo done && shutdown -h now"))
}

func TestIsDangerous_reboot_in_filename_not_dangerous(t *testing.T) {
	// A script named "reboot_handler.py" should not trigger
	assert.False(t, IsDangerous("python reboot_handler.py"))
}

func TestIsDangerous_shutdown_in_filename_not_dangerous(t *testing.T) {
	assert.False(t, IsDangerous("node shutdown_graceful.js"))
}

func TestIsDangerous_git_reboot_branch_not_dangerous(t *testing.T) {
	assert.False(t, IsDangerous("git checkout reboot-fix"))
}

// === Tests for rm boundary matching ===

func TestIsDangerous_rm_standalone(t *testing.T) {
	assert.True(t, IsDangerous("rm file.txt"))
}

func TestIsDangerous_rm_after_pipe(t *testing.T) {
	assert.True(t, IsDangerous("find . -name '*.tmp' | xargs rm -f"))
}

func TestIsDangerous_rm_after_semicolon(t *testing.T) {
	assert.True(t, IsDangerous("echo done; rm -rf /tmp/test"))
}

func TestIsDangerous_inform_not_dangerous(t *testing.T) {
	// "inform" contains "rm " when followed by space — should not trigger
	assert.False(t, IsDangerous("inform user about update"))
}

func TestIsDangerous_firmware_not_dangerous(t *testing.T) {
	assert.False(t, IsDangerous("check firmware version"))
}

// === Tests for kill boundary matching ===

func TestIsDangerous_kill_standalone(t *testing.T) {
	assert.True(t, IsDangerous("kill -9 1234"))
}

func TestIsDangerous_kill_after_pipe(t *testing.T) {
	assert.True(t, IsDangerous("pgrep process | xargs kill -9"))
}

func TestIsDangerous_skill_not_dangerous(t *testing.T) {
	// "skill" contains "kill" but should not trigger
	assert.False(t, IsDangerous("echo skill level"))
}

func TestIsDangerous_overkill_not_dangerous(t *testing.T) {
	assert.False(t, IsDangerous("echo that is overkill"))
}

// === Tests for sudo boundary matching ===

func TestIsDangerous_sudo_standalone(t *testing.T) {
	assert.True(t, IsDangerous("sudo apt update"))
}

func TestIsDangerous_pseudo_not_dangerous(t *testing.T) {
	assert.False(t, IsDangerous("echo pseudo random"))
}

// === Tests for apt boundary matching ===

func TestIsDangerous_apt_standalone(t *testing.T) {
	assert.True(t, IsDangerous("apt install package"))
}

func TestIsDangerous_apt_after_pipe(t *testing.T) {
	assert.True(t, IsDangerous("echo yes | apt install package"))
}

func TestIsDangerous_adapt_not_dangerous(t *testing.T) {
	assert.False(t, IsDangerous("adapt to changes"))
}

func TestIsDangerous_apt_cache_is_detected(t *testing.T) {
	// apt-cache starts with "apt" but "apt " requires space after apt
	// "apt-cache" does not have a space after "apt", so should NOT match
	assert.False(t, IsDangerous("apt-cache search foo"))
}

// === Tests for mkfs boundary matching ===

func TestIsDangerous_mkfs_standalone(t *testing.T) {
	assert.True(t, IsDangerous("mkfs.ext4 /dev/sda1"))
}

func TestIsDangerous_mkfs_after_separator(t *testing.T) {
	assert.True(t, IsDangerous("umount /dev/sda1 && mkfs.ext4 /dev/sda1"))
}

// === Redirect operator tests (not boundary-checked) ===

func TestIsDangerous_redirect_in_middle(t *testing.T) {
	assert.True(t, IsDangerous("echo hello > /dev/null"))
}

func TestIsDangerous_append_redirect(t *testing.T) {
	assert.True(t, IsDangerous("echo hello >> log.txt"))
}

// === Edge cases ===

func TestIsDangerous_empty_command(t *testing.T) {
	assert.False(t, IsDangerous(""))
}

func TestIsDangerous_whitespace_only(t *testing.T) {
	assert.False(t, IsDangerous("   "))
}

func TestIsDangerous_leading_whitespace(t *testing.T) {
	assert.True(t, IsDangerous("  rm -rf /tmp/foo"))
}

func TestIsDangerous_multiple_dangerous(t *testing.T) {
	assert.True(t, IsDangerous("rm file.txt && dd if=/dev/zero of=/dev/sda"))
}

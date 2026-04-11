package lua

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSandboxSafeLibs(t *testing.T) {
	L := NewSandboxedState(context.Background())
	defer L.Close()

	// base functions: type, tostring, pairs, ipairs, tonumber, pcall
	for _, fn := range []string{"type", "tostring", "pairs", "ipairs", "tonumber", "pcall"} {
		err := L.DoString(`_ = ` + fn)
		require.NoError(t, err, "base function %s should be available", fn)
	}
}

func TestSandboxStringLib(t *testing.T) {
	L := NewSandboxedState(context.Background())
	defer L.Close()

	err := L.DoString(`local s = string.format("%d", 42); assert(s == "42")`)
	require.NoError(t, err, "string.format should be available")

	err = L.DoString(`local n = string.len("hello"); assert(n == 5)`)
	require.NoError(t, err, "string.len should be available")
}

func TestSandboxMathLib(t *testing.T) {
	L := NewSandboxedState(context.Background())
	defer L.Close()

	err := L.DoString(`local n = math.floor(3.7); assert(n == 3)`)
	require.NoError(t, err, "math.floor should be available")

	err = L.DoString(`local n = math.abs(-5); assert(n == 5)`)
	require.NoError(t, err, "math.abs should be available")
}

func TestSandboxTableLib(t *testing.T) {
	L := NewSandboxedState(context.Background())
	defer L.Close()

	err := L.DoString(`local t = {}; table.insert(t, "a"); assert(t[1] == "a")`)
	require.NoError(t, err, "table.insert should be available")

	err = L.DoString(`local s = table.concat({"a", "b", "c"}, ","); assert(s == "a,b,c")`)
	require.NoError(t, err, "table.concat should be available")
}

func TestSandboxNoOS(t *testing.T) {
	L := NewSandboxedState(context.Background())
	defer L.Close()

	// os global should be nil
	err := L.DoString(`assert(os == nil, "os should be nil")`)
	require.NoError(t, err, "os global should be nil")

	// pcall(require, "os") should return false
	err = L.DoString(`local ok, _ = pcall(require, "os"); assert(not ok, "require os should fail")`)
	require.NoError(t, err, "require('os') should fail")
}

func TestSandboxNoIO(t *testing.T) {
	L := NewSandboxedState(context.Background())
	defer L.Close()

	err := L.DoString(`assert(io == nil, "io should be nil")`)
	require.NoError(t, err, "io global should be nil")

	err = L.DoString(`local ok, _ = pcall(require, "io"); assert(not ok, "require io should fail")`)
	require.NoError(t, err, "require('io') should fail")
}

func TestSandboxNoDebug(t *testing.T) {
	L := NewSandboxedState(context.Background())
	defer L.Close()

	err := L.DoString(`assert(debug == nil, "debug should be nil")`)
	require.NoError(t, err, "debug global should be nil")

	err = L.DoString(`local ok, _ = pcall(require, "debug"); assert(not ok, "require debug should fail")`)
	require.NoError(t, err, "require('debug') should fail")
}

func TestSandboxDofileNil(t *testing.T) {
	L := NewSandboxedState(context.Background())
	defer L.Close()

	err := L.DoString(`assert(dofile == nil, "dofile should be nil")`)
	require.NoError(t, err, "dofile should be nil in sandbox")
}

func TestSandboxLoadfileNil(t *testing.T) {
	L := NewSandboxedState(context.Background())
	defer L.Close()

	err := L.DoString(`assert(loadfile == nil, "loadfile should be nil")`)
	require.NoError(t, err, "loadfile should be nil in sandbox")
}

func TestSandboxTimeout(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	L := NewSandboxedState(ctx)
	defer L.Close()

	err := L.DoString(`while true do end`)
	assert.Error(t, err, "infinite loop should be cancelled by context timeout")
}

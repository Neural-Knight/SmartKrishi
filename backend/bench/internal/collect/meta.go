// Package collect gathers run metadata (git, environment) and samples host
// resources (CPU, memory, goroutines) plus the server DB pool during a run.
package collect

import (
	"os/exec"
	"runtime"
	"strings"

	"github.com/smartkrishi/backend/bench/internal/schema"
)

// GitMeta probes the current repository state via git. On any failure it returns
// zero-ish values rather than failing the run (bench must not require a clean
// git tree), but never fabricates a commit.
func GitMeta(repoDir string) schema.Git {
	g := schema.Git{}
	if out, err := gitOut(repoDir, "rev-parse", "--short", "HEAD"); err == nil {
		g.Commit = out
	}
	if out, err := gitOut(repoDir, "rev-parse", "--abbrev-ref", "HEAD"); err == nil {
		g.Branch = out
	}
	if out, err := gitOut(repoDir, "status", "--porcelain"); err == nil {
		g.Dirty = out != ""
	}
	return g
}

func gitOut(dir string, args ...string) (string, error) {
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	b, err := cmd.Output()
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(b)), nil
}

// EnvMeta probes the host environment. cpuModel and memTotalMB are best-effort
// (empty/0 when unavailable) and never fabricated. agentModel/maxConns come from
// the caller (the harness reads them from server config).
func EnvMeta(agentModel string, maxConns int32) schema.Environment {
	return schema.Environment{
		GoVersion:  runtime.Version(),
		OS:         runtime.GOOS,
		Arch:       runtime.GOARCH,
		CPUModel:   cpuModel(),
		NumCPU:     runtime.NumCPU(),
		MemTotalMB: memTotalMB(),
		Config: schema.EnvConfig{
			AgentModel: agentModel,
			MaxDBConns: maxConns,
		},
	}
}

// cpuModel returns a CPU model string when the OS exposes one cheaply, else "".
func cpuModel() string {
	switch runtime.GOOS {
	case "darwin":
		if out, err := exec.Command("sysctl", "-n", "machdep.cpu.brand_string").Output(); err == nil {
			return strings.TrimSpace(string(out))
		}
	case "linux":
		if out, err := exec.Command("sh", "-c", "grep -m1 'model name' /proc/cpuinfo | cut -d: -f2").Output(); err == nil {
			return strings.TrimSpace(string(out))
		}
	}
	return ""
}

// memTotalMB returns total physical memory in MB, or 0 when unavailable.
func memTotalMB() int64 {
	switch runtime.GOOS {
	case "darwin":
		if out, err := exec.Command("sysctl", "-n", "hw.memsize").Output(); err == nil {
			return parseBytesToMB(strings.TrimSpace(string(out)))
		}
	case "linux":
		if out, err := exec.Command("sh", "-c", "grep MemTotal /proc/meminfo | awk '{print $2*1024}'").Output(); err == nil {
			return parseBytesToMB(strings.TrimSpace(string(out)))
		}
	}
	return 0
}

func parseBytesToMB(s string) int64 {
	var n int64
	for _, c := range s {
		if c < '0' || c > '9' {
			return 0
		}
		n = n*10 + int64(c-'0')
	}
	return n / (1024 * 1024)
}

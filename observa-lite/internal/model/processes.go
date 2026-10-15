package model

import (
	"regexp"
	"strings"
)

var systemProcessNames = map[string]bool{
	"systemd":             true,
	"(sd-pam)":            true,
	"sd-pam":              true,
	"systemd-journald":    true,
	"systemd-udevd":       true,
	"systemd-logind":      true,
	"systemd-resolved":    true,
	"systemd-networkd":    true,
	"systemd-timesyncd":   true,
	"systemd-oomd":        true,
	"init":                true,
	"kthreadd":            true,
	"rcu_sched":           true,
	"rcu_preempt":         true,
	"rcu_tasks_kthread":   true,
	"rcu_tasks_rude_":     true,
	"rcu_tasks_trace":     true,
	"sshd":                true,
	"sshd-session":        true,
	"dbus-daemon":         true,
	"dbus-broker":         true,
	"dbus-broker-launch":  true,
	"cron":                true,
	"crond":               true,
	"anacron":             true,
	"rsyslogd":            true,
	"auditd":              true,
	"agetty":              true,
	"getty":               true,
	"login":               true,
	"bash":                true,
	"sh":                  true,
	"dash":                true,
	"zsh":                 true,
	"fish":                true,
	"sudo":                true,
	"su":                  true,
	"polkitd":             true,
	"chronyd":             true,
	"networkmanager":      true,
	"wslservice":          true,
	"wslhost":             true,
	"dockerd":             true,
	"containerd":          true,
	"containerd-shim":     true,
	"runc":                true,
	"docker-init":         true,
	"tini":                true,
	"system":              true,
	"system idle process": true,
	"registry":            true,
	"smss.exe":            true,
	"csrss.exe":           true,
	"wininit.exe":         true,
	"services.exe":        true,
	"lsass.exe":           true,
	"svchost.exe":         true,
	"winlogon.exe":        true,
	"dwm.exe":             true,
	"explorer.exe":        true,
	"sihost.exe":          true,
	"taskhostw.exe":       true,
	"conhost.exe":         true,
	"cmd.exe":             true,
	"powershell.exe":      true,
	"pwsh.exe":            true,
	"wmiprvse.exe":        true,
	"spoolsv.exe":         true,
	"searchindexer.exe":   true,
	"msmpeng.exe":         true,
}
var kernelProcess = regexp.MustCompile(`^(kworker|ksoftirqd|migration|watchdog|cpuhp|idle_inject|irq|rcuop|rcuos|jbd2|kcompactd|kswapd|khugepaged|kdevtmpfs|kthrotld)([/:0-9-]|$)`)

func IsSystemProcess(p Process) bool {
	switch strings.ToLower(p.Runtime) {
	case "java", "dotnet", "nodejs", "python", "ruby", "php", "database", "nginx", "go":
		return false
	}
	name := strings.ToLower(p.Name)
	parts := strings.FieldsFunc(name, func(r rune) bool { return r == '/' || r == '\\' })
	base := name
	if len(parts) > 0 {
		base = parts[len(parts)-1]
	}
	return systemProcessNames[base] || kernelProcess.MatchString(name) || strings.HasPrefix(base, "containerd-shim-") || strings.HasPrefix(base, "systemd-")
}

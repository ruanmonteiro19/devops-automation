import type { Host, ProcessInfo } from "./api";

// Exact OS component names only. Unknown/custom applications remain visible.
const systemNames = new Set(
  [
    "systemd",
    "(sd-pam)",
    "sd-pam",
    "systemd-journald",
    "systemd-udevd",
    "systemd-logind",
    "systemd-resolved",
    "systemd-networkd",
    "systemd-timesyncd",
    "systemd-oomd",
    "init",
    "kthreadd",
    "rcu_sched",
    "rcu_preempt",
    "rcu_tasks_kthread",
    "rcu_tasks_rude_",
    "rcu_tasks_trace",
    "sshd",
    "sshd-session",
    "dbus-daemon",
    "dbus-broker",
    "dbus-broker-launch",
    "cron",
    "crond",
    "anacron",
    "rsyslogd",
    "auditd",
    "agetty",
    "getty",
    "login",
    "bash",
    "sh",
    "dash",
    "zsh",
    "fish",
    "sudo",
    "su",
    "polkitd",
    "chronyd",
    "NetworkManager",
    "wslservice",
    "wslhost",
    "dockerd",
    "containerd",
    "containerd-shim",
    "runc",
    "docker-init",
    "tini",
    "system",
    "system idle process",
    "registry",
    "smss.exe",
    "csrss.exe",
    "wininit.exe",
    "services.exe",
    "lsass.exe",
    "svchost.exe",
    "winlogon.exe",
    "dwm.exe",
    "explorer.exe",
    "sihost.exe",
    "taskhostw.exe",
    "conhost.exe",
    "cmd.exe",
    "powershell.exe",
    "pwsh.exe",
    "wmiprvse.exe",
    "spoolsv.exe",
    "searchindexer.exe",
    "msmpeng.exe",
  ].map((n) => n.toLowerCase()),
);
export function isSystemProcess(process: ProcessInfo): boolean {
  const name = process.name.toLowerCase().replace(/^.*[\\/]/, "");
  // Known runtimes override kernel/OS patterns; PID 1 can be an application.
  if (
    [
      "java",
      "dotnet",
      "nodejs",
      "python",
      "ruby",
      "php",
      "database",
      "nginx",
      "go",
    ].includes(process.runtime.toLowerCase())
  )
    return false;
  return (
    systemNames.has(name) ||
    /^(?:kworker|ksoftirqd|migration|watchdog|cpuhp|idle_inject|irq|rcuop|rcuos|jbd2|kcompactd|kswapd|khugepaged|kdevtmpfs|kthrotld)(?:[/:\d-]|$)/.test(
      process.name.toLowerCase(),
    ) ||
    /^containerd-shim(?:-|$)/.test(name) ||
    /^systemd-(?:journal|udevd|logind|resolve|network|timesync|oomd)/.test(name)
  );
}
export type ObservedProcess = ProcessInfo & {
  host_id: string;
  host_name: string;
  system: boolean;
};
export function observedProcesses(hosts: Host[]): ObservedProcess[] {
  return hosts.flatMap((host) =>
    host.processes.map((process) => ({
      ...process,
      host_id: host.id,
      host_name: host.name,
      system: isSystemProcess(process),
    })),
  );
}
export function applicationProcesses(host: Host): ProcessInfo[] {
  return host.processes.filter((p) => !isSystemProcess(p));
}

//go:build linux

package discovery

import (
	"bufio"
	"encoding/hex"
	"fmt"
	"github.com/ruanmonteiro19/observa-lite/internal/model"
	"net"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

func Detect() (model.Discovery, error) {
	name, err := os.Hostname()
	if err != nil {
		return model.Discovery{}, err
	}
	hostID := name
	if b, e := os.ReadFile("/etc/machine-id"); e == nil && strings.TrimSpace(string(b)) != "" {
		hostID = strings.TrimSpace(string(b))
	}
	d := model.Discovery{HostID: hostID, Hostname: name, OS: runtimeOS(), CapturedAt: time.Now().UTC(), Addresses: localIPs()}
	sockets := readSockets("/proc/net/tcp")
	sockets = append(sockets, readSockets("/proc/net/tcp6")...)
	byInode := map[string][]socket{}
	for _, s := range sockets {
		byInode[s.inode] = append(byInode[s.inode], s)
	}
	dirs, err := os.ReadDir("/proc")
	if err != nil {
		return d, err
	}
	// Limit work in pathological hosts. /proc privileges may restrict process visibility.
	seenConn := map[string]bool{}
	for _, dir := range dirs {
		pid, err := strconv.Atoi(dir.Name())
		if err != nil || !dir.IsDir() {
			continue
		}
		procRoot := "/proc/" + dir.Name()
		comm, e := os.ReadFile(procRoot + "/comm")
		if e != nil {
			continue
		}
		name := strings.TrimSpace(string(comm))
		if name == "" {
			continue
		}
		cmd, _ := os.ReadFile(procRoot + "/cmdline")
		service, kind := serviceName(name, string(cmd))
		if kind == "process" {
			if executable, err := os.Readlink(procRoot + "/exe"); err == nil {
				detectedService, detectedKind := serviceName(filepath.Base(strings.TrimSuffix(executable, " (deleted)")), string(cmd))
				if detectedKind != "process" {
					service, kind = detectedService, detectedKind
				}
			}
		}
		rss := residentBytes(procRoot + "/statm")
		d.Processes = append(d.Processes, model.Process{PID: pid, Name: name, Service: service, Runtime: kind, RSSBytes: rss})
		fds, e := os.ReadDir(procRoot + "/fd")
		if e != nil {
			continue
		}
		for _, fd := range fds {
			link, e := os.Readlink(filepath.Join(procRoot, "fd", fd.Name()))
			if e != nil || !strings.HasPrefix(link, "socket:[") {
				continue
			}
			inode := strings.TrimSuffix(strings.TrimPrefix(link, "socket:["), "]")
			for _, s := range byInode[inode] {
				key := fmt.Sprintf("%d/%s/%s/%d/%s/%d", pid, inode, s.localIP, s.localPort, s.remoteIP, s.remotePort)
				if seenConn[key] {
					continue
				}
				seenConn[key] = true
				d.Connections = append(d.Connections, model.Connection{PID: pid, LocalIP: s.localIP, LocalPort: s.localPort, RemoteIP: s.remoteIP, RemotePort: s.remotePort, State: s.state})
			}
		}
	}
	return d, nil
}
func runtimeOS() string {
	b, e := os.ReadFile("/etc/os-release")
	if e != nil {
		return "linux"
	}
	for _, line := range strings.Split(string(b), "\n") {
		if strings.HasPrefix(line, "PRETTY_NAME=") {
			return strings.Trim(strings.TrimPrefix(line, "PRETTY_NAME="), "\"")
		}
	}
	return "linux"
}
func localIPs() []string {
	out := []string{}
	ifaces, _ := net.Interfaces()
	for _, iface := range ifaces {
		addrs, _ := iface.Addrs()
		for _, a := range addrs {
			s, _, _ := net.ParseCIDR(a.String())
			if s != nil && !s.IsLoopback() {
				out = append(out, s.String())
			}
		}
	}
	return out
}
func residentBytes(p string) uint64 {
	b, e := os.ReadFile(p)
	if e != nil {
		return 0
	}
	parts := strings.Fields(string(b))
	if len(parts) < 2 {
		return 0
	}
	n, _ := strconv.ParseUint(parts[1], 10, 64)
	return n * uint64(os.Getpagesize())
}
func serviceName(name, cmd string) (string, string) {
	low := strings.ToLower(name)
	if strings.Contains(low, "java") {
		parts := strings.Split(cmd, "\x00")
		for _, p := range parts {
			for _, prefix := range []string{"-Dweblogic.Name=", "-Dspring.application.name="} {
				if strings.HasPrefix(p, prefix) {
					return strings.TrimPrefix(p, prefix), "java"
				}
			}
		}
		for i, p := range parts {
			if p == "-jar" && i+1 < len(parts) {
				return filepath.Base(parts[i+1]), "java"
			}
		}
		return "java", "java"
	}
	if strings.Contains(low, "dotnet") {
		return "dotnet", "dotnet"
	}
	if strings.Contains(low, "node") {
		return "node", "nodejs"
	}
	if strings.Contains(low, "python") {
		return "python", "python"
	}
	if strings.Contains(low, "postgres") {
		return "postgres", "postgres"
	}
	if strings.Contains(low, "redis") {
		return "redis", "redis"
	}
	if strings.Contains(low, "mysqld") {
		return "mysql", "mysql"
	}
	if strings.Contains(low, "nginx") {
		return "nginx", "nginx"
	}
	if strings.Contains(low, "httpd") || strings.Contains(low, "apache") {
		return "apache", "apache"
	}
	return name, "process"
}

type socket struct {
	inode, localIP, remoteIP, state string
	localPort, remotePort           int
}

func readSockets(path string) []socket {
	f, e := os.Open(path)
	if e != nil {
		return nil
	}
	defer f.Close()
	res := []socket{}
	scanner := bufio.NewScanner(f)
	if scanner.Scan() {
	} // header
	for scanner.Scan() {
		p := strings.Fields(scanner.Text())
		if len(p) < 10 {
			continue
		}
		lip, lp, ok := decodeAddr(p[1])
		if !ok {
			continue
		}
		rip, rp, ok := decodeAddr(p[2])
		if !ok {
			continue
		}
		state := map[string]string{"01": "ESTABLISHED", "02": "SYN_SENT", "03": "SYN_RECV", "06": "TIME_WAIT", "08": "CLOSE_WAIT", "0A": "LISTEN"}[p[3]]
		if state == "" {
			state = p[3]
		}
		res = append(res, socket{inode: p[9], localIP: lip, localPort: lp, remoteIP: rip, remotePort: rp, state: state})
	}
	return res
}
func decodeAddr(s string) (string, int, bool) {
	p := strings.Split(s, ":")
	if len(p) != 2 {
		return "", 0, false
	}
	bytes, e := hex.DecodeString(p[0])
	if e != nil {
		return "", 0, false
	}
	// Linux /proc/net/tcp encodes IPv4 in native-endian 32-bit words;
	// IPv6 is encoded as four such words, each individually little-endian.
	if len(bytes) != 4 && len(bytes) != 16 {
		return "", 0, false
	}
	for i := 0; i < len(bytes); i += 4 {
		bytes[i], bytes[i+3] = bytes[i+3], bytes[i]
		bytes[i+1], bytes[i+2] = bytes[i+2], bytes[i+1]
	}
	port, e := strconv.ParseInt(p[1], 16, 32)
	if e != nil {
		return "", 0, false
	}
	return net.IP(bytes).String(), int(port), true
}

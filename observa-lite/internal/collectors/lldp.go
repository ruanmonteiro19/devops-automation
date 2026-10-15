package collectors

import (
	"encoding/hex"
	"github.com/gosnmp/gosnmp"
	"github.com/ruanmonteiro19/observa-lite/internal/model"
	"net"
	"sort"
	"strconv"
	"strings"
)

// LLDP local port numbers and IF-MIB indexes belong to different namespaces.
// Match only an exact, unambiguous local port ID/description to an interface name.
func lldpNeighbors(s *gosnmp.GoSNMP, interfaces []model.DeviceInterface) ([]model.DeviceNeighbor, bool) {
	locals := map[string]string{}
	rows := map[string]*model.DeviceNeighbor{}
	truncated := false
	walk := func(oid string, apply func(string, gosnmp.SnmpPDU)) {
		count := 0
		_ = snmpWalk(s, oid, func(p gosnmp.SnmpPDU) error {
			count++
			if count > 512 {
				truncated = true
				return errSNMPBound
			}
			prefix := "." + oid + "."
			name := "." + strings.TrimPrefix(p.Name, ".")
			if strings.HasPrefix(name, prefix) {
				apply(strings.TrimPrefix(name, prefix), p)
			}
			return nil
		})
	}
	walk("1.0.8802.1.1.2.1.3.7.1.3", func(index string, p gosnmp.SnmpPDU) {
		v := snmpText(p)
		if len(v) > 256 {
			v = v[:256]
		}
		locals[index] = v
	})
	get := func(index string) *model.DeviceNeighbor {
		parts := strings.Split(index, ".")
		if len(parts) != 3 {
			return nil
		}
		if rows[index] == nil {
			if len(rows) >= 256 {
				truncated = true
				return nil
			}
			local := locals[parts[1]]
			if local == "" {
				local = "LLDP port " + parts[1]
			}
			rows[index] = &model.DeviceNeighbor{LocalPort: local, Protocol: "LLDP"}
		}
		return rows[index]
	}
	for _, column := range []struct{ oid, field string }{{"1.0.8802.1.1.2.1.4.1.1.5", "chassis"}, {"1.0.8802.1.1.2.1.4.1.1.7", "port"}, {"1.0.8802.1.1.2.1.4.1.1.9", "name"}} {
		walk(column.oid, func(index string, p gosnmp.SnmpPDU) {
			row := get(index)
			if row == nil {
				return
			}
			value := snmpText(p)
			if len(value) > 256 {
				value = value[:256]
			}
			switch column.field {
			case "chassis":
				if b, ok := p.Value.([]byte); ok {
					value = hex.EncodeToString(b)
				}
				row.ChassisID = value
			case "port":
				row.RemotePort = value
			case "name":
				row.RemoteName = value
			}
		})
	}
	walk("1.0.8802.1.1.2.1.4.2.1.3", func(index string, p gosnmp.SnmpPDU) {
		parts := strings.Split(index, ".")
		if len(parts) < 9 {
			return
		}
		row := get(strings.Join(parts[:3], "."))
		if row == nil {
			return
		}
		family, _ := strconv.Atoi(parts[3])
		length, _ := strconv.Atoi(parts[4])
		if !((family == 1 && length == 4) || (family == 2 && length == 16)) || len(parts) != 5+length {
			return
		}
		bytes := make([]byte, length)
		for i, part := range parts[5:] {
			n, e := strconv.Atoi(part)
			if e != nil || n < 0 || n > 255 {
				return
			}
			bytes[i] = byte(n)
		}
		row.RemoteIP = net.IP(bytes).String()
	})
	out := []model.DeviceNeighbor{}
	for _, row := range rows {
		matches := []string{}
		for _, iface := range interfaces {
			if iface.Name == row.LocalPort || (iface.Description != "" && iface.Description == row.LocalPort) {
				matches = append(matches, iface.Index)
			}
		}
		if len(matches) == 1 {
			row.LocalIfIndex = matches[0]
		}
		out = append(out, *row)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].LocalPort < out[j].LocalPort })
	return out, truncated
}

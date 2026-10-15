package collectors

import (
	"context"
	"errors"
	"fmt"
	"github.com/gosnmp/gosnmp"
	"github.com/ruanmonteiro19/observa-lite/internal/model"
	networkmodel "github.com/ruanmonteiro19/observa-lite/internal/network"
	"net"
	"sort"
	"strconv"
	"strings"
	"time"
)

var errSNMPBound = errors.New("SNMP result bound")

func newSNMP(ctx context.Context, p model.Integration, c model.SNMPCredentials) *gosnmp.GoSNMP {
	port := p.SNMPPort
	if port == 0 {
		port = 161
	}
	s := &gosnmp.GoSNMP{Target: p.Target, Port: port, Transport: "udp", Version: gosnmp.Version2c, Community: c.Community, Timeout: 2 * time.Second, Retries: 1, MaxOids: 20, MaxRepetitions: 20, Context: ctx}
	if p.SNMPVersion == "1" {
		s.Version = gosnmp.Version1
	}
	if p.SNMPVersion == "3" {
		s.Version = gosnmp.Version3
		s.SecurityModel = gosnmp.UserSecurityModel
		s.MsgFlags = gosnmp.NoAuthNoPriv
		if c.SecurityLevel == "authNoPriv" {
			s.MsgFlags = gosnmp.AuthNoPriv
		}
		if c.SecurityLevel == "authPriv" {
			s.MsgFlags = gosnmp.AuthPriv
		}
		s.ContextName = c.ContextName
		s.SecurityParameters = &gosnmp.UsmSecurityParameters{UserName: c.Username, AuthenticationProtocol: map[string]gosnmp.SnmpV3AuthProtocol{"SHA": gosnmp.SHA, "SHA256": gosnmp.SHA256, "SHA512": gosnmp.SHA512}[c.AuthProtocol], AuthenticationPassphrase: c.AuthPassword, PrivacyProtocol: map[string]gosnmp.SnmpV3PrivProtocol{"AES": gosnmp.AES, "AES256": gosnmp.AES256}[c.PrivacyProtocol], PrivacyPassphrase: c.PrivacyPassword}
	}
	return s
}
func snmpText(p gosnmp.SnmpPDU) string {
	if p.Type == gosnmp.NoSuchObject || p.Type == gosnmp.NoSuchInstance || p.Type == gosnmp.EndOfMibView {
		return ""
	}
	if b, ok := p.Value.([]byte); ok {
		return string(b)
	}
	return fmt.Sprint(p.Value)
}
func snmpNumber(p gosnmp.SnmpPDU) (float64, bool) {
	switch p.Type {
	case gosnmp.Integer, gosnmp.Counter32, gosnmp.Counter64, gosnmp.Gauge32, gosnmp.TimeTicks, gosnmp.Uinteger32:
		n, _ := gosnmp.ToBigInt(p.Value).Float64()
		return n, true
	}
	return 0, false
}
func snmpWalk(s *gosnmp.GoSNMP, oid string, fn gosnmp.WalkFunc) error {
	if s.Version == gosnmp.Version1 {
		return s.Walk(oid, fn)
	}
	return s.BulkWalk(oid, fn)
}
func (r Runner) nativeDeviceInventory(ctx context.Context, p model.Integration) ([]model.IntegrationResource, bool, error) {
	if r.SNMPCredentials == nil {
		return nil, false, errors.New("credenciais SNMP indisponíveis")
	}
	credentials, e := r.SNMPCredentials(p.ID)
	if e != nil {
		return nil, false, e
	}
	if e = networkmodel.ValidateCredentials(p.SNMPVersion, credentials); e != nil {
		return nil, false, e
	}
	ips, e := net.DefaultResolver.LookupIPAddr(ctx, p.Target)
	if e != nil || len(ips) == 0 {
		return nil, false, errors.New("endereço do equipamento não resolvido")
	}
	for _, ip := range ips {
		if !r.AllowPrivate && restricted(ip.IP) {
			return nil, false, errors.New("destino privado bloqueado; habilite OBSERVA_ALLOW_PRIVATE_TARGETS para a rede autorizada")
		}
	}
	p.Target = ips[0].IP.String()
	s := newSNMP(ctx, p, credentials)
	if e = s.Connect(); e != nil {
		return nil, false, errors.New("não foi possível abrir a conexão SNMP")
	}
	defer s.Conn.Close()
	resource := model.IntegrationResource{ID: p.Target, Name: p.Name, Kind: "network.device", Status: "observed", Attributes: map[string]string{"address": p.Target, "device_type": p.DeviceType, "host_group": p.HostGroup, "snmp_version": p.SNMPVersion, "collector": "Vetra Server", "template_id": p.TemplateID}, Metrics: map[string]float64{}, Interfaces: []model.DeviceInterface{}}
	good := 0
	warnings := []string{}
	truncated := false
	for _, entry := range []struct{ oid, name string }{{"1.3.6.1.2.1.1.1.0", "sysDescr"}, {"1.3.6.1.2.1.1.2.0", "sysObjectID"}, {"1.3.6.1.2.1.1.3.0", "sysUpTime"}, {"1.3.6.1.2.1.1.5.0", "sysName"}, {"1.3.6.1.2.1.1.6.0", "sysLocation"}} {
		packet, err := s.Get([]string{entry.oid})
		if err != nil {
			if good == 0 {
				return nil, false, errors.New("SNMP sem resposta: confira IP, porta, versão, credenciais e ACL do equipamento")
			}
			warnings = append(warnings, "Falha ao consultar "+entry.name)
			continue
		}
		if packet.Error != gosnmp.NoError {
			continue
		}
		for _, v := range packet.Variables {
			value := snmpText(v)
			if value == "" {
				continue
			}
			good++
			if n, ok := snmpNumber(v); ok {
				resource.Metrics[entry.name] = n
			} else {
				if len(value) > 1024 {
					value = value[:1024]
				}
				resource.Attributes[entry.name] = value
			}
			if entry.name == "sysName" && value != "" {
				resource.Name = value
			}
		}
	}
	if good == 0 {
		return nil, false, errors.New("equipamento respondeu sem dados de sistema acessíveis")
	}
	interfaces := map[string]*model.DeviceInterface{}
	columns := []struct{ name, oid string }{{"ifName", "1.3.6.1.2.1.31.1.1.1.1"}, {"ifAlias", "1.3.6.1.2.1.31.1.1.1.18"}, {"ifDescr", "1.3.6.1.2.1.2.2.1.2"}, {"ifAdminStatus", "1.3.6.1.2.1.2.2.1.7"}, {"ifOperStatus", "1.3.6.1.2.1.2.2.1.8"}, {"ifHighSpeed", "1.3.6.1.2.1.31.1.1.1.15"}, {"ifHCInOctets", "1.3.6.1.2.1.31.1.1.1.6"}, {"ifHCOutOctets", "1.3.6.1.2.1.31.1.1.1.10"}, {"ifInOctets", "1.3.6.1.2.1.2.2.1.10"}, {"ifOutOctets", "1.3.6.1.2.1.2.2.1.16"}, {"ifSpeed", "1.3.6.1.2.1.2.2.1.5"}, {"ifInErrors", "1.3.6.1.2.1.2.2.1.14"}, {"ifOutErrors", "1.3.6.1.2.1.2.2.1.20"}, {"ifInDiscards", "1.3.6.1.2.1.2.2.1.13"}, {"ifOutDiscards", "1.3.6.1.2.1.2.2.1.19"}}
	for _, column := range columns {
		if s.Version == gosnmp.Version1 && strings.HasPrefix(column.name, "ifHC") {
			continue
		}
		count := 0
		err := snmpWalk(s, column.oid, func(v gosnmp.SnmpPDU) error {
			count++
			if count > 256 {
				truncated = true
				return errSNMPBound
			}
			prefix := "." + column.oid + "."
			oid := "." + strings.TrimPrefix(v.Name, ".")
			if !strings.HasPrefix(oid, prefix) {
				return nil
			}
			index := strings.TrimPrefix(oid, prefix)
			iface := interfaces[index]
			if iface == nil {
				if len(interfaces) >= 256 {
					truncated = true
					return errSNMPBound
				}
				iface = &model.DeviceInterface{Index: index, Name: index, Metrics: map[string]float64{}}
				interfaces[index] = iface
			}
			if n, ok := snmpNumber(v); ok {
				iface.Metrics[column.name] = n
			} else {
				value := snmpText(v)
				if len(value) > 256 {
					value = value[:256]
				}
				if column.name == "ifName" || (column.name == "ifDescr" && iface.Name == index) {
					iface.Name = value
				}
				if column.name == "ifAlias" {
					iface.Description = value
				}
			}
			return nil
		})
		if err != nil && !errors.Is(err, errSNMPBound) {
			warnings = append(warnings, "Coluna indisponível: "+column.name)
		}
		if ctx.Err() != nil {
			return nil, false, errors.New("tempo limite da coleta SNMP excedido")
		}
	}
	for _, iface := range interfaces {
		resource.Interfaces = append(resource.Interfaces, *iface)
	}
	sort.Slice(resource.Interfaces, func(i, j int) bool { return resource.Interfaces[i].Index < resource.Interfaces[j].Index })
	if p.TemplateID != "generic-snmp" && r.NetworkTemplate != nil {
		t, ok := r.NetworkTemplate(p.TemplateID)
		if !ok {
			return nil, false, errors.New("template de rede não encontrado")
		}
		attempts := 0
		readItem := func(item model.NetworkItem) {
			if !item.Supported {
				return
			}
			attempts++
			if attempts > 1000 || ctx.Err() != nil {
				truncated = true
				return
			}
			packet, err := s.Get([]string{item.OID})
			if err != nil || packet.Error != gosnmp.NoError {
				if len(warnings) < 50 {
					warnings = append(warnings, "Item sem resposta: "+item.Key)
				}
				return
			}
			for _, v := range packet.Variables {
				if n, ok := snmpNumber(v); ok {
					if item.Change != "" {
						resource.Metrics[item.Key+".raw_counter"] = n * item.Multiplier
						resource.Attributes["preprocessing:"+item.Key] = item.Change
					} else {
						resource.Metrics[item.Key] = n * item.Multiplier
					}
				} else {
					value := snmpText(v)
					if len(value) > 1024 {
						value = value[:1024]
					}
					if value != "" {
						resource.Attributes["item:"+item.Key] = value
					}
				}
			}
		}
		for _, item := range t.Items {
			readItem(item)
		}
		for _, d := range t.Discovery {
			if attempts >= 1000 || ctx.Err() != nil {
				truncated = true
				break
			}
			if !d.Supported {
				continue
			}
			rows := map[string]map[string]string{}
			for macro, base := range d.Columns {
				count := 0
				walkError := snmpWalk(s, base, func(v gosnmp.SnmpPDU) error {
					count++
					if count > 128 {
						truncated = true
						return errSNMPBound
					}
					prefix := "." + strings.TrimPrefix(base, ".") + "."
					oid := "." + strings.TrimPrefix(v.Name, ".")
					if !strings.HasPrefix(oid, prefix) {
						return nil
					}
					index := strings.TrimPrefix(oid, prefix)
					if rows[index] == nil {
						if len(rows) >= 128 {
							truncated = true
							return errSNMPBound
						}
						rows[index] = map[string]string{"{#SNMPINDEX}": index}
					}
					value := snmpText(v)
					if len(value) > 256 {
						value = value[:256]
					}
					rows[index][macro] = value
					return nil
				})
				if walkError != nil && !errors.Is(walkError, errSNMPBound) && len(warnings) < 50 {
					warnings = append(warnings, "Falha na descoberta: "+d.Name)
				}
			}
			for _, row := range rows {
				if attempts >= 1000 || ctx.Err() != nil {
					truncated = true
					break
				}
				for _, proto := range d.Items {
					item := proto
					for macro, value := range row {
						item.OID = strings.ReplaceAll(item.OID, macro, value)
						item.Key = strings.ReplaceAll(item.Key, macro, value)
					}
					if networkmodel.NumericOID(item.OID) {
						readItem(item)
					}
				}
			}
		}
		resource.Attributes["template_name"] = t.Name
		resource.Attributes["template_supported"] = strconv.Itoa(t.Supported)
		resource.Attributes["template_unsupported"] = strconv.Itoa(t.Unsupported)
		if len(t.Warnings) > 0 {
			warnings = append(warnings, t.Warnings...)
		}
	}
	if len(warnings) > 0 {
		if len(warnings) > 50 {
			warnings = warnings[:50]
		}
		resource.Attributes["collection_warnings"] = strings.Join(warnings, "; ")
	}
	return []model.IntegrationResource{resource}, truncated, nil
}

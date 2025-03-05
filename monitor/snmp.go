package monitor

import (
	"fmt"
	"time"

	"github.com/gosnmp/gosnmp"

	"github.com/gophertribe/snmp"
)

type SNMP struct {
	server         *snmp.SNMPServer
	client         *gosnmp.GoSNMP
	agent          snmp.MasterAgent
	trapServerAddr string
}

func NewSNMP(nmsIP, username, authPassphrase, privacyPassphrase string) *SNMP {
	return &SNMP{
		client: &gosnmp.GoSNMP{
			Target: nmsIP,
		},
		agent: snmp.MasterAgent{
			SecurityConfig: snmp.SecurityConfig{
				AuthoritativeEngineBoots: 1,
				Users: []gosnmp.UsmSecurityParameters{
					{
						UserName:                 username,
						AuthenticationProtocol:   gosnmp.MD5,
						PrivacyProtocol:          gosnmp.DES,
						AuthenticationPassphrase: authPassphrase,
						PrivacyPassphrase:        privacyPassphrase,
					},
				},
			},
			SubAgents: []*snmp.SubAgent{{CommunityIDs: []string{"public"}, UserErrorMarkPacket: true}},
			Logger:    snmp.NewDefaultLogger(),
		},
		trapServerAddr: nmsIP,
	}
}

func (s *SNMP) RegisterControls(val ...*snmp.PDUValueControlItem) {
	sa := s.agent.SubAgents[len(s.agent.SubAgents)-1]
	for _, v := range val {
		sa.OIDs = append(sa.OIDs, v)
	}
}

func (s *SNMP) Listen() error {
	s.server = snmp.NewSNMPServer(s.agent)
	err := s.server.ListenUDP("udp", "0.0.0.0:11161")
	if err != nil {
		return fmt.Errorf("could not launch smtp server: %w", err)
	}
	go func() {
		_ = s.server.ServeForever()
	}()
	return nil
}

func (s *SNMP) Close() {
	if s.server != nil {
		s.server.Shutdown()
	}
}

func (s *SNMP) SendTrap(oid string, valType gosnmp.Asn1BER, val interface{}) error {
	if s.client == nil {
		// ignore
		return nil
	}
	_, err := s.client.SendTrap(gosnmp.SnmpTrap{
		Variables: []gosnmp.SnmpPDU{
			{Name: oid, Type: valType, Value: val},
		},
		Enterprise:   "satsysHdis.1",
		AgentAddress: s.trapServerAddr,
		GenericTrap:  6,
		SpecificTrap: 1,
		Timestamp:    uint(time.Now().Unix()),
	})
	// TODO: log response
	if err != nil {
		return fmt.Errorf("could not send trap: %w", err)
	}
	return nil
}

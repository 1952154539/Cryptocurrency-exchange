package kyc

import (
	"strings"
)

// SanctionedAddresses is a list of known sanctioned wallet addresses (OFAC SDN etc).
// In production, this would be loaded from an external API or file.
var SanctionedAddresses = []string{
	// Example entries - replace with real sanctioned addresses
	"0x0000000000000000000000000000000000000001",
	"0x0000000000000000000000000000000000000002",
}

// AMLChecker screens addresses against sanctions lists.
type AMLChecker struct {
	sanctioned map[string]bool
}

// NewAMLChecker creates an AML checker with the given sanctioned addresses.
func NewAMLChecker(addresses []string) *AMLChecker {
	s := make(map[string]bool, len(addresses))
	for _, addr := range addresses {
		s[strings.ToLower(addr)] = true
	}
	return &AMLChecker{sanctioned: s}
}

// IsSanctioned checks if an address appears on the sanctions list.
func (c *AMLChecker) IsSanctioned(address string) bool {
	return c.sanctioned[strings.ToLower(address)]
}

// SanctionListSize returns the number of addresses in the sanctions list.
func (c *AMLChecker) SanctionListSize() int {
	return len(c.sanctioned)
}

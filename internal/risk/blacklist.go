package risk

import (
	"sync"
)

// Blacklist manages blocked IPs, user IDs, and wallet addresses.
type Blacklist struct {
	mu         sync.RWMutex
	blockedIPs map[string]bool
	blockedUIDs map[string]bool
	blockedAddr map[string]bool
}

// NewBlacklist creates an empty blacklist.
func NewBlacklist() *Blacklist {
	return &Blacklist{
		blockedIPs:  make(map[string]bool),
		blockedUIDs: make(map[string]bool),
		blockedAddr: make(map[string]bool),
	}
}

// BlockIP adds an IP to the blacklist.
func (bl *Blacklist) BlockIP(ip string) {
	bl.mu.Lock()
	defer bl.mu.Unlock()
	bl.blockedIPs[ip] = true
}

// UnblockIP removes an IP from the blacklist.
func (bl *Blacklist) UnblockIP(ip string) {
	bl.mu.Lock()
	defer bl.mu.Unlock()
	delete(bl.blockedIPs, ip)
}

// IsIPBlocked checks if an IP is blacklisted.
func (bl *Blacklist) IsIPBlocked(ip string) bool {
	bl.mu.RLock()
	defer bl.mu.RUnlock()
	return bl.blockedIPs[ip]
}

// BlockUser adds a user ID to the blacklist.
func (bl *Blacklist) BlockUser(userID string) {
	bl.mu.Lock()
	defer bl.mu.Unlock()
	bl.blockedUIDs[userID] = true
}

// IsUserBlocked checks if a user is blacklisted.
func (bl *Blacklist) IsUserBlocked(userID string) bool {
	bl.mu.RLock()
	defer bl.mu.RUnlock()
	return bl.blockedUIDs[userID]
}

// BlockAddress adds a wallet address to the blacklist.
func (bl *Blacklist) BlockAddress(addr string) {
	bl.mu.Lock()
	defer bl.mu.Unlock()
	bl.blockedAddr[addr] = true
}

// IsAddressBlocked checks if an address is blacklisted.
func (bl *Blacklist) IsAddressBlocked(addr string) bool {
	bl.mu.RLock()
	defer bl.mu.RUnlock()
	return bl.blockedAddr[addr]
}

// List returns all blocked entries.
func (bl *Blacklist) List() (ips, uids, addrs []string) {
	bl.mu.RLock()
	defer bl.mu.RUnlock()
	for ip := range bl.blockedIPs {
		ips = append(ips, ip)
	}
	for uid := range bl.blockedUIDs {
		uids = append(uids, uid)
	}
	for addr := range bl.blockedAddr {
		addrs = append(addrs, addr)
	}
	return
}

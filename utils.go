package ipcountrylocator

import (
	"fmt"
	"net"
	"strings"
)

// IPRange représente une plage inclusive d'adresses IPv4 (Start à End) associée à un code pays
type IPRange struct {
	Start   uint32
	End     uint32
	Country string
}

// ipv4ToUint32 convertit une IPv4 en entier 32 bits.
func ipv4ToUint32(ip net.IP) uint32 {
	ip = ip.To4()
	if ip == nil {
		return 0
	}
	return uint32(ip[0])<<24 | uint32(ip[1])<<16 | uint32(ip[2])<<8 | uint32(ip[3])
}

// encodeUint32BE écrit un uint32 en big-endian.
func encodeUint32BE(b []byte, v uint32) {
	b[0] = byte(v >> 24)
	b[1] = byte(v >> 16)
	b[2] = byte(v >> 8)
	b[3] = byte(v)
}

// decodeUint32BE lit un uint32 big-endian.
func decodeUint32BE(b []byte) uint32 {
	return uint32(b[0])<<24 | uint32(b[1])<<16 | uint32(b[2])<<8 | uint32(b[3])
}

// parseIPRange parse "start-end" ou CIDR et retourne (start,end).
func parseIPRange(ipRange string) (uint32, uint32, error) {
	// Check if it's a CIDR
	if strings.Contains(ipRange, "/") {
		_, ipNet, err := net.ParseCIDR(ipRange)
		if err != nil {
			return 0, 0, err
		}

		// Calculate the start address
		start := ipv4ToUint32(ipNet.IP)

		// Calculate the end address using the mask
		mask := ipNet.Mask
		maskSize, _ := mask.Size()
		hostBits := 32 - maskSize
		end := start | (1<<uint(hostBits) - 1)

		return start, end, nil
	}

	// Otherwise, check if it's a range in the form "start-end"
	parts := strings.Split(ipRange, "-")
	if len(parts) != 2 {
		return 0, 0, fmt.Errorf("invalid IP range format")
	}

	startIP := net.ParseIP(strings.TrimSpace(parts[0])).To4()
	endIP := net.ParseIP(strings.TrimSpace(parts[1])).To4()

	if startIP == nil || endIP == nil {
		return 0, 0, fmt.Errorf("invalid IP address")
	}

	start := ipv4ToUint32(startIP)
	end := ipv4ToUint32(endIP)

	return start, end, nil
}

// ipv4InCIDR teste l'appartenance d'une IPv4 à un CIDR.
func ipv4InCIDR(ip string, cidr string) bool {
	_, ipNet, err := net.ParseCIDR(cidr)
	if err != nil {
		return false
	}
	ipAddr := net.ParseIP(ip)
	if ipAddr == nil {
		return false
	}
	return ipNet.Contains(ipAddr)
}

// ipv4InRange teste une IPv4 contre "start-end" ou CIDR.
func ipv4InRange(ip string, ipRange string) bool {
	// Check if it's a CIDR
	if strings.Contains(ipRange, "/") {
		return ipv4InCIDR(ip, ipRange)
	}

	// Otherwise, check if it's a range in the form "start-end"
	parts := strings.Split(ipRange, "-")
	if len(parts) != 2 {
		return false
	}

	ipAddr := net.ParseIP(ip).To4()
	if ipAddr == nil {
		return false
	}

	startIP := net.ParseIP(parts[0]).To4()
	endIP := net.ParseIP(parts[1]).To4()
	if startIP == nil || endIP == nil {
		return false
	}

	ipInt := ipv4ToUint32(ipAddr)
	startInt := ipv4ToUint32(startIP)
	endInt := ipv4ToUint32(endIP)

	return ipInt >= startInt && ipInt <= endInt
}

// isPrivateOrLocalCIDR détecte si un CIDR est privé / loopback / link-local.
func isPrivateOrLocalCIDR(ipRange string) bool {
	// Extract the IP address from the range (before the "/")
	parts := strings.Split(ipRange, "/")
	if len(parts) != 2 {
		return false
	}

	ip := net.ParseIP(parts[0])
	if ip == nil {
		return false
	}

	// Check if it's a private or localhost address
	return ip.IsLoopback() || ip.IsPrivate() || ip.IsLinkLocalUnicast() || ip.IsLinkLocalMulticast()
}

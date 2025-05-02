package ipcountrylocator

import (
	"fmt"
	"net"
	"strings"
)

// IPRange represents a range of IP addresses
type IPRange struct {
	Start   uint32
	End     uint32
	Country string
}

// IPToUint32 converts an IP address to a 32-bit integer
func IPToUint32(ip net.IP) uint32 {
	ip = ip.To4()
	if ip == nil {
		return 0
	}
	return uint32(ip[0])<<24 | uint32(ip[1])<<16 | uint32(ip[2])<<8 | uint32(ip[3])
}

// EncodeUint32 encodes a uint32 into a 4-byte array
func EncodeUint32(b []byte, v uint32) {
	b[0] = byte(v >> 24)
	b[1] = byte(v >> 16)
	b[2] = byte(v >> 8)
	b[3] = byte(v)
}

// DecodeUint32 decodes a 4-byte array into a uint32
func DecodeUint32(b []byte) uint32 {
	return uint32(b[0])<<24 | uint32(b[1])<<16 | uint32(b[2])<<8 | uint32(b[3])
}

// ParseIPRange parses an IP range and returns the numeric start and end values
func ParseIPRange(ipRange string) (uint32, uint32, error) {
	// Check if it's a CIDR
	if strings.Contains(ipRange, "/") {
		_, ipNet, err := net.ParseCIDR(ipRange)
		if err != nil {
			return 0, 0, err
		}

		// Calculate the start address
		start := IPToUint32(ipNet.IP)

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

	start := IPToUint32(startIP)
	end := IPToUint32(endIP)

	return start, end, nil
}

// IsIPInCIDR checks if an IP address is contained within a CIDR network
func IsIPInCIDR(ip string, cidr string) bool {
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

// IsIPInRange checks if an IP address is contained within an IP range
func IsIPInRange(ip string, ipRange string) bool {
	// Check if it's a CIDR
	if strings.Contains(ipRange, "/") {
		return IsIPInCIDR(ip, ipRange)
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

	ipInt := IPToUint32(ipAddr)
	startInt := IPToUint32(startIP)
	endInt := IPToUint32(endIP)

	return ipInt >= startInt && ipInt <= endInt
}

// IsPrivateOrLocalRange checks if an IP range is private or local
func IsPrivateOrLocalRange(ipRange string) bool {
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

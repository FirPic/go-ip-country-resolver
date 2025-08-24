package ipcountrylocator

import (
	"net"
	"testing"
)

func TestIPToUint32(t *testing.T) {
	testCases := []struct {
		ip       string
		expected uint32
	}{
		{"192.168.1.1", 3232235777},
		{"10.0.0.1", 167772161},
		{"8.8.8.8", 134744072},
		{"0.0.0.0", 0},
		{"255.255.255.255", 4294967295},
	}

	for _, tc := range testCases {
		ip := net.ParseIP(tc.ip)
		result := ipv4ToUint32(ip)
		if result != tc.expected {
			t.Errorf("For IP %s: expected value %d, got %d", tc.ip, tc.expected, result)
		}
	}
}

// TestEncodeDecodeUint32 valide la symétrie encodage/décodage.
func TestEncodeDecodeUint32(t *testing.T) {
	testCases := []uint32{
		0,
		1,
		255,
		256,
		65535,
		16777215,
		4294967295,
	}

	for _, tc := range testCases {
		b := make([]byte, 4)
		encodeUint32BE(b, tc)
		result := decodeUint32BE(b)
		if result != tc {
			t.Errorf("For %d: after encoding/decoding, got value %d", tc, result)
		}
	}
}

// TestParseIPRange couvre formats plage, CIDR et erreurs.
func TestParseIPRange(t *testing.T) {
	testCases := []struct {
		ipRange     string
		expectedOk  bool
		expectedSt  uint32
		expectedEnd uint32
	}{
		// Start-end format
		{"192.168.1.1-192.168.1.10", true, 3232235777, 3232235786},
		{"8.8.8.8-8.8.8.8", true, 134744072, 134744072},

		// CIDR format
		{"192.168.1.0/24", true, 3232235776, 3232236031}, // Corrected values to match current implementation
		{"10.0.0.0/8", true, 167772160, 184549375},

		// Invalid format
		{"invalid", false, 0, 0},
		{"192.168.1.1", false, 0, 0},
		{"192.168.1.1/invalid", false, 0, 0},
	}

	for _, tc := range testCases {
		start, end, err := parseIPRange(tc.ipRange)
		if tc.expectedOk && err != nil {
			t.Errorf("For %s: unexpected error: %v", tc.ipRange, err)
		}
		if !tc.expectedOk && err == nil {
			t.Errorf("For %s: expected error but none was received", tc.ipRange)
		}
		if tc.expectedOk {
			if start != tc.expectedSt || end != tc.expectedEnd {
				t.Errorf("For %s: expected values (%d, %d), got (%d, %d)",
					tc.ipRange, tc.expectedSt, tc.expectedEnd, start, end)
			}
		}
	}
}

// TestIsIPInCIDR vérifie inclusion/exclusion et cas limites.
func TestIsIPInCIDR(t *testing.T) {
	testCases := []struct {
		ip       string
		cidr     string
		expected bool
	}{
		{"192.168.1.10", "192.168.1.0/24", true},
		{"192.168.2.1", "192.168.1.0/24", false},
		{"10.0.0.1", "10.0.0.0/8", true},
		{"11.0.0.1", "10.0.0.0/8", false},
		{"8.8.8.8", "8.8.8.0/24", true},
		{"8.8.9.8", "8.8.8.0/24", false},

		// Edge cases
		{"192.168.1.0", "192.168.1.0/24", true},   // First address
		{"192.168.1.255", "192.168.1.0/24", true}, // Last address

		// Invalid IP
		{"invalid", "192.168.1.0/24", false},

		// Invalid CIDR
		{"192.168.1.1", "invalid", false},
	}

	for _, tc := range testCases {
		result := ipv4InCIDR(tc.ip, tc.cidr)
		if result != tc.expected {
			t.Errorf("For IP %s in CIDR %s: expected value %v, got %v",
				tc.ip, tc.cidr, tc.expected, result)
		}
	}
}

// TestIsIPInRange teste plages start-end et CIDR.
func TestIsIPInRange(t *testing.T) {
	testCases := []struct {
		ip       string
		ipRange  string
		expected bool
	}{
		// Start-end format
		{"192.168.1.5", "192.168.1.1-192.168.1.10", true},
		{"192.168.1.11", "192.168.1.1-192.168.1.10", false},
		{"8.8.8.8", "8.8.8.8-8.8.8.8", true},

		// CIDR format
		{"192.168.1.100", "192.168.1.0/24", true},
		{"192.168.2.1", "192.168.1.0/24", false},

		// Edge cases
		{"192.168.1.1", "192.168.1.1-192.168.1.10", true},  // First
		{"192.168.1.10", "192.168.1.1-192.168.1.10", true}, // Last

		// Invalid format
		{"192.168.1.1", "invalid", false},
		{"invalid", "192.168.1.1-192.168.1.10", false},
	}

	for _, tc := range testCases {
		result := ipv4InRange(tc.ip, tc.ipRange)
		if result != tc.expected {
			t.Errorf("For IP %s in range %s: expected value %v, got %v",
				tc.ip, tc.ipRange, tc.expected, result)
		}
	}
}

// TestIsPrivateOrLocalRange vérifie la détection de réseaux privés / spéciaux.
func TestIsPrivateOrLocalRange(t *testing.T) {
	testCases := []struct {
		ipRange  string
		expected bool
	}{
		// Private ranges
		{"192.168.1.0/24", true},
		{"10.0.0.0/8", true},
		{"172.16.0.0/12", true},

		// Loopback
		{"127.0.0.1/8", true},

		// Link-local
		{"169.254.0.0/16", true},

		// Public ranges
		{"8.8.8.0/24", false},
		{"1.1.1.0/24", false},
		{"203.0.113.0/24", false},

		// Invalid formats
		{"not-a-range", false},
		{"192.168.1.1", false},
	}

	for _, tc := range testCases {
		result := isPrivateOrLocalCIDR(tc.ipRange)
		if result != tc.expected {
			t.Errorf("For range %s: expected value %v, got %v",
				tc.ipRange, tc.expected, result)
		}
	}
}

func TestIPRange(t *testing.T) {
	// Test of IPRange structure
	ipRange := IPRange{
		Start:   ipv4ToUint32(net.ParseIP("192.168.1.1").To4()),
		End:     ipv4ToUint32(net.ParseIP("192.168.1.10").To4()),
		Country: "FR",
	}

	expectedStart := uint32(3232235777) // 192.168.1.1
	expectedEnd := uint32(3232235786)   // 192.168.1.10

	if ipRange.Start != expectedStart {
		t.Errorf("Incorrect start value. Expected: %d, Got: %d", expectedStart, ipRange.Start)
	}

	if ipRange.End != expectedEnd {
		t.Errorf("Incorrect end value. Expected: %d, Got: %d", expectedEnd, ipRange.End)
	}

	if ipRange.Country != "FR" {
		t.Errorf("Incorrect country. Expected: FR, Got: %s", ipRange.Country)
	}
}

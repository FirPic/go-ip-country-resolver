package ipcountrylocator

import (
	"testing"
)

func TestIPCache(t *testing.T) {
	// Create a cache with a maximum size of 3 entries
	cache := newIPCache(3)

	// Test adding elements
	cache.putCountry("192.168.1.1", "FR")
	cache.putCountry("10.0.0.1", "DE")
	country, found := cache.getCountry("192.168.1.1")

	if !found {
		t.Error("The entry should exist in the cache")
	}

	if country != "FR" {
		t.Errorf("Incorrect country for IP. Expected: FR, Got: %s", country)
	}

	// Test a non-existent element
	_, found = cache.getCountry("8.8.8.8")
	if found {
		t.Error("The entry should not exist in the cache")
	}

	// Test cache overflow
	cache.putCountry("172.16.0.1", "US")
	cache.putCountry("8.8.8.8", "US") // Should reset the cache

	_, found = cache.getCountry("192.168.1.1")
	if found {
		t.Error("The entry should have been removed during cache reset")
	}

	country, found = cache.getCountry("8.8.8.8")
	if !found || country != "US" {
		t.Error("The new entry was not correctly added after cache reset")
	}
}

func TestNewIPLocator(t *testing.T) {
	manager, _, cleanup := setupTestDB(t)
	defer cleanup()

	locator := newIPLocator(manager, 100)

	if locator.DBManager != manager {
		t.Error("The database manager was not correctly assigned")
	}

	if locator.Cache == nil {
		t.Error("The cache was not initialized")
	}

	if locator.Cache.maxSize != 100 {
		t.Errorf("Incorrect cache size. Expected: 100, Got: %d", locator.Cache.maxSize)
	}
}

func TestFindCountryForIP(t *testing.T) {
	manager, _, cleanup := setupTestDB(t)
	defer cleanup()

	// Add test IP ranges
	ipRanges := []struct {
		ipRange string
		country string
	}{
		{"1.0.0.0-1.0.0.255", "FR"},
		{"2.0.0.0-2.0.0.255", "DE"},
		{"8.8.8.0-8.8.8.255", "US"},
	}

	for _, r := range ipRanges {
		start, end, _ := parseIPRange(r.ipRange)
		_, err := manager.upsertIPRangeCountry(r.ipRange, start, end, r.country)
		if err != nil {
			t.Fatalf("Error adding IP range: %v", err)
		}
	}

	locator := newIPLocator(manager, 100)

	// Test finding a country for an IP
	testCases := []struct {
		ip              string
		expectedCountry string
		shouldFind      bool
	}{
		{"1.0.0.123", "FR", true},
		{"2.0.0.1", "DE", true},
		{"8.8.8.8", "US", true},
		{"127.0.0.1", "", false}, // Local IP, should not be found
		{"9.9.9.9", "", false},   // IP outside known ranges
	}

	for _, tc := range testCases {
		country, err := locator.lookupCountryByIP(tc.ip)

		if tc.shouldFind {
			if err != nil {
				t.Errorf("Error finding country for %s: %v", tc.ip, err)
			}

			if country != tc.expectedCountry {
				t.Errorf("Incorrect country for %s. Expected: %s, Got: %s",
					tc.ip, tc.expectedCountry, country)
			}

			// Check that the IP is cached
			cachedCountry, found := locator.Cache.getCountry(tc.ip)
			if !found {
				t.Errorf("IP %s was not cached", tc.ip)
			}

			if cachedCountry != tc.expectedCountry {
				t.Errorf("Incorrect country in cache for %s. Expected: %s, Got: %s",
					tc.ip, tc.expectedCountry, cachedCountry)
			}
		} else {
			if err == nil {
				t.Errorf("The search for %s should have failed", tc.ip)
			}
		}
	}

	// Test search from cache
	// Modify a range to verify that the cache is used
	start, end, _ := parseIPRange("1.0.0.0-1.0.0.255")
	_, err := manager.upsertIPRangeCountry("1.0.0.0-1.0.0.255", start, end, "IT")
	if err != nil {
		t.Fatalf("Error updating IP range: %v", err)
	}

	// The search should still return FR because the entry is cached
	country, err := locator.lookupCountryByIP("1.0.0.123")
	if err != nil || country != "FR" {
		t.Errorf("The cache was not used correctly. Expected: FR, Got: %s", country)
	}
}

func TestGetAllRangesForCountry(t *testing.T) {
	manager, _, cleanup := setupTestDB(t)
	defer cleanup()

	// Add test IP ranges for different countries
	ipRanges := []struct {
		ipRange string
		country string
	}{
		{"1.0.0.0-1.0.0.255", "FR"},
		{"1.1.0.0-1.1.0.255", "FR"},
		{"2.0.0.0-2.0.0.255", "DE"},
		{"8.8.8.0-8.8.8.255", "US"},
	}

	for _, r := range ipRanges {
		start, end, _ := parseIPRange(r.ipRange)
		_, err := manager.upsertIPRangeCountry(r.ipRange, start, end, r.country)
		if err != nil {
			t.Fatalf("Error adding IP range: %v", err)
		}
	}

	locator := newIPLocator(manager, 100)

	// Get all ranges for France
	ranges, err := locator.listIPRangesByCountry("FR")
	if err != nil {
		t.Fatalf("Error retrieving ranges for FR: %v", err)
	}

	if len(ranges) != 2 {
		t.Errorf("Incorrect number of ranges for FR. Expected: 2, Got: %d", len(ranges))
	}

	// Check that the ranges are correct
	expectedRanges := map[string]bool{
		"1.0.0.0-1.0.0.255": true,
		"1.1.0.0-1.1.0.255": true,
	}

	for _, r := range ranges {
		if !expectedRanges[r] {
			t.Errorf("Unexpected range found: %s", r)
		}
	}

	// Test with a country without ranges
	ranges, err = locator.listIPRangesByCountry("IT")
	if err != nil {
		t.Fatalf("Error retrieving ranges for IT: %v", err)
	}

	if len(ranges) != 0 {
		t.Errorf("Incorrect number of ranges for IT. Expected: 0, Got: %d", len(ranges))
	}
}

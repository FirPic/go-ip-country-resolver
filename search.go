package ipcountrylocator

import (
	"fmt"
	"net"
	"sync"

	"go.etcd.io/bbolt"
)

// IPCache manages the cache for IP lookups
type IPCache struct {
	cache       map[string]string
	maxSize     int
	currentSize int
	mutex       sync.RWMutex
}

// NewIPCache creates a new IP cache with a specified maximum size
func NewIPCache(maxSize int) *IPCache {
	return &IPCache{
		cache:       make(map[string]string, maxSize),
		maxSize:     maxSize,
		currentSize: 0,
	}
}

// Get retrieves an entry from the cache
func (c *IPCache) Get(ip string) (string, bool) {
	c.mutex.RLock()
	defer c.mutex.RUnlock()
	country, found := c.cache[ip]
	return country, found
}

// Set adds or updates an entry in the cache
func (c *IPCache) Set(ip, country string) {
	c.mutex.Lock()
	defer c.mutex.Unlock()

	// If the cache is full, clear it
	if c.currentSize >= c.maxSize {
		c.cache = make(map[string]string, c.maxSize)
		c.currentSize = 0
	}

	// Add the entry
	c.cache[ip] = country
	c.currentSize++
}

// IPLocator allows searching for the country of an IP address
type IPLocator struct {
	DBManager *DBManager
	Cache     *IPCache
}

// NewIPLocator creates a new IP locator
func NewIPLocator(dbManager *DBManager, cacheSize int) *IPLocator {
	return &IPLocator{
		DBManager: dbManager,
		Cache:     NewIPCache(cacheSize),
	}
}

// FindCountryForIP searches for the country corresponding to an IP address
func (l *IPLocator) FindCountryForIP(ip string) (string, error) {
	// First check in the cache
	if country, found := l.Cache.Get(ip); found {
		return country, nil
	}

	ipAddr := net.ParseIP(ip).To4()
	if ipAddr == nil {
		return "", fmt.Errorf("invalid IPv4 address")
	}

	ipNum := IPToUint32(ipAddr)

	var country string
	err := l.DBManager.DB.View(func(tx *bbolt.Tx) error {
		// 1. First try the optimized numeric method
		countryCode, err := l.findCountryForIPNumeric(tx, ipNum)
		if err == nil {
			country = countryCode
			return nil
		}

		// 2. If it doesn't work, use the traditional method
		bucket := tx.Bucket([]byte("ip_ranges"))
		if bucket == nil {
			return fmt.Errorf("bucket 'ip_ranges' not found")
		}

		cursor := bucket.Cursor()
		for k, v := cursor.First(); k != nil; k, v = cursor.Next() {
			ipRange := string(k)
			if IsIPInRange(ip, ipRange) {
				country = string(v)
				return nil
			}
		}

		return fmt.Errorf("no matching country found for IP: %s", ip)
	})

	// Cache the result if found
	if err == nil {
		l.Cache.Set(ip, country)
	}

	return country, err
}

// findCountryForIPNumeric uses the optimized method to search for an IP
func (l *IPLocator) findCountryForIPNumeric(tx *bbolt.Tx, ipNum uint32) (string, error) {
	bucket := tx.Bucket([]byte("ip_ranges_numeric"))
	if bucket == nil {
		return "", fmt.Errorf("bucket 'ip_ranges_numeric' not found")
	}

	// Optimized search
	c := bucket.Cursor()
	for k, v := c.First(); k != nil; k, v = c.Next() {
		if len(k) >= 8 {
			start := DecodeUint32(k[0:4])
			end := DecodeUint32(k[4:8])

			if ipNum >= start && ipNum <= end {
				return string(v), nil
			}

			// Optimization: if we exceed the IP value, no need to continue
			if start > ipNum {
				break
			}
		}
	}

	return "", fmt.Errorf("no matching range found")
}

// GetAllRangesForCountry retrieves all IP ranges for a given country
func (l *IPLocator) GetAllRangesForCountry(countryCode string) ([]string, error) {
	var ranges []string

	err := l.DBManager.DB.View(func(tx *bbolt.Tx) error {
		bucket := tx.Bucket([]byte("ip_ranges"))
		if bucket == nil {
			return fmt.Errorf("bucket 'ip_ranges' not found")
		}

		c := bucket.Cursor()
		for k, v := c.First(); k != nil; k, v = c.Next() {
			if string(v) == countryCode {
				ranges = append(ranges, string(k))
			}
		}

		return nil
	})

	return ranges, err
}

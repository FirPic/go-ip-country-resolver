package ipcountrylocator

import (
	"fmt"
	"net"
	"sync"

	"go.etcd.io/bbolt"
)

// IPCache fournit un cache (clé: IP string -> code pays) à taille bornée, réinitialisé quand plein.
type IPCache struct {
	cache       map[string]string
	maxSize     int
	currentSize int
	mutex       sync.RWMutex
}

// newIPCache instancie un cache.
func newIPCache(maxSize int) *IPCache {
	return &IPCache{
		cache:       make(map[string]string, maxSize),
		maxSize:     maxSize,
		currentSize: 0,
	}
}

// getCountry récupère une entrée du cache.
// Thread-safe (verrou R).
func (c *IPCache) getCountry(ip string) (string, bool) {
	c.mutex.RLock()
	defer c.mutex.RUnlock()
	country, found := c.cache[ip]
	return country, found
}

// putCountry insère une entrée dans le cache.
// Thread-safe (verrou W).
func (c *IPCache) putCountry(ip, country string) {
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

// IPLocator encapsule l'accès DB + cache pour résoudre le pays d'une IPv4.
type IPLocator struct {
	DBManager *DBManager
	Cache     *IPCache
}

// newIPLocator construit un localisateur IP.
func newIPLocator(dbManager *DBManager, cacheSize int) *IPLocator {
	return &IPLocator{
		DBManager: dbManager,
		Cache:     newIPCache(cacheSize),
	}
}

// lookupCountryByIP recherche le pays pour une IPv4 (cache -> index numérique -> fallback texte).
func (l *IPLocator) lookupCountryByIP(ip string) (string, error) {
	// First check in the cache
	if country, found := l.Cache.getCountry(ip); found {
		return country, nil
	}

	ipAddr := net.ParseIP(ip).To4()
	if ipAddr == nil {
		return "", fmt.Errorf("invalid IPv4 address")
	}

	ipNum := ipv4ToUint32(ipAddr)

	var country string
	err := l.DBManager.DB.View(func(tx *bbolt.Tx) error {
		// 1. First try the optimized numeric method
		countryCode, err := l.lookupCountryByIPNumeric(tx, ipNum)
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
			if ipv4InRange(ip, ipRange) {
				country = string(v)
				return nil
			}
		}

		return fmt.Errorf("no matching country found for IP: %s", ip)
	})

	// Cache the result if found
	if err == nil {
		l.Cache.putCountry(ip, country)
	}

	return country, err
}

// lookupCountryByIPNumeric effectue une recherche dans le bucket numérique.
func (l *IPLocator) lookupCountryByIPNumeric(tx *bbolt.Tx, ipNum uint32) (string, error) {
	bucket := tx.Bucket([]byte("ip_ranges_numeric"))
	if bucket == nil {
		return "", fmt.Errorf("bucket 'ip_ranges_numeric' not found")
	}

	// Optimized search
	c := bucket.Cursor()
	for k, v := c.First(); k != nil; k, v = c.Next() {
		if len(k) >= 8 {
			start := decodeUint32BE(k[0:4])
			end := decodeUint32BE(k[4:8])

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

// listIPRangesByCountry retourne toutes les plages texte associées à un pays.
func (l *IPLocator) listIPRangesByCountry(countryCode string) ([]string, error) {
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

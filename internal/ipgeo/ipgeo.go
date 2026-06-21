package ipgeo

import (
	"bufio"
	"encoding/binary"
	"errors"
	"fmt"
	"net"
	"os"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
)

var (
	ErrInvalidIP         = errors.New("ipgeo: invalid IP address")
	ErrInvalidCIDR       = errors.New("ipgeo: invalid CIDR notation")
	ErrEmptyCIDR         = errors.New("ipgeo: empty CIDR")
	ErrNotFound          = errors.New("ipgeo: IP not found in any CIDR range")
	ErrInvalidDataFormat = errors.New("ipgeo: invalid data format in file")
	ErrFileNotExist      = errors.New("ipgeo: data file does not exist")
	ErrEmptyData         = errors.New("ipgeo: empty IP database")
	ErrEngineNotReady    = errors.New("ipgeo: engine is not ready (no data loaded)")
	ErrUnsupportedLang   = errors.New("ipgeo: unsupported language code")
)

type LocalizedName struct {
	Country  map[string]string
	Province map[string]string
	City     map[string]string
	District map[string]string
	ISP      map[string]string
}

func newLocalizedName() *LocalizedName {
	return &LocalizedName{
		Country:  make(map[string]string),
		Province: make(map[string]string),
		City:     make(map[string]string),
		District: make(map[string]string),
		ISP:      make(map[string]string),
	}
}

type GeoInfo struct {
	Country  string
	Province string
	City     string
	District string
	ISP      string
	Names    *LocalizedName
}

type QueryResult struct {
	Found   bool
	IP      string
	Country string
	Province string
	City     string
	District string
	ISP      string
	Lang    string
}

type cidrEntry struct {
	CIDR       string
	StartIP    uint32
	EndIP      uint32
	PrefixLen  int
	NetworkIP  uint32
	Mask       uint32
	GeoInfo    *GeoInfo
}

type ipIndex struct {
	byStartIP []cidrEntry
	maxEndIP  []uint32
}

type Engine struct {
	currentIdx atomic.Value
	reloadMu   sync.Mutex
}

func NewEngine() *Engine {
	e := &Engine{}
	return e
}

func NewEngineFromFile(filePath string) (*Engine, error) {
	e := NewEngine()
	err := e.LoadFromFile(filePath)
	if err != nil {
		return nil, err
	}
	return e, nil
}

func NewEngineFromData(data []string) (*Engine, error) {
	e := NewEngine()
	err := e.LoadFromData(data)
	if err != nil {
		return nil, err
	}
	return e, nil
}

func ipv4ToUint32(ip net.IP) (uint32, error) {
	ipv4 := ip.To4()
	if ipv4 == nil {
		return 0, ErrInvalidIP
	}
	return binary.BigEndian.Uint32(ipv4), nil
}

func parseCIDR(cidrStr string) (networkIP uint32, startIP uint32, endIP uint32, prefixLen int, mask uint32, err error) {
	if strings.TrimSpace(cidrStr) == "" {
		return 0, 0, 0, 0, 0, ErrEmptyCIDR
	}

	_, ipNet, err := net.ParseCIDR(cidrStr)
	if err != nil {
		return 0, 0, 0, 0, 0, ErrInvalidCIDR
	}

	prefixLen, _ = ipNet.Mask.Size()
	mask = binary.BigEndian.Uint32([]byte(ipNet.Mask))
	networkIP, err = ipv4ToUint32(ipNet.IP)
	if err != nil {
		return 0, 0, 0, 0, 0, err
	}
	startIP = networkIP
	endIP = networkIP | (^mask)

	return networkIP, startIP, endIP, prefixLen, mask, nil
}

func parseDataLine(line string) (*cidrEntry, error) {
	trimmed := strings.TrimSpace(line)
	if trimmed == "" || strings.HasPrefix(trimmed, "#") {
		return nil, nil
	}

	parts := strings.Split(line, "\t")
	if len(parts) < 2 {
		parts = strings.Fields(line)
	}
	if len(parts) < 2 {
		return nil, ErrInvalidDataFormat
	}

	cidrPart := strings.TrimSpace(parts[0])
	networkIP, startIP, endIP, prefixLen, mask, err := parseCIDR(cidrPart)
	if err != nil {
		return nil, err
	}

	geo := &GeoInfo{
		Names: newLocalizedName(),
	}

	if len(parts) >= 2 {
		geo.Country = strings.TrimSpace(parts[1])
		geo.Names.Country["zh-CN"] = geo.Country
	}
	if len(parts) >= 3 {
		geo.Province = strings.TrimSpace(parts[2])
		geo.Names.Province["zh-CN"] = geo.Province
	}
	if len(parts) >= 4 {
		geo.City = strings.TrimSpace(parts[3])
		geo.Names.City["zh-CN"] = geo.City
	}
	if len(parts) >= 5 {
		geo.District = strings.TrimSpace(parts[4])
		geo.Names.District["zh-CN"] = geo.District
	}
	if len(parts) >= 6 {
		geo.ISP = strings.TrimSpace(parts[5])
		geo.Names.ISP["zh-CN"] = geo.ISP
	}

	for i := 6; i < len(parts); i++ {
		kv := strings.TrimSpace(parts[i])
		kvParts := strings.SplitN(kv, "=", 2)
		if len(kvParts) == 2 {
			key := strings.TrimSpace(kvParts[0])
			value := strings.TrimSpace(kvParts[1])
			langParts := strings.SplitN(key, ":", 2)
			if len(langParts) == 2 {
				lang := strings.TrimSpace(langParts[0])
				field := strings.TrimSpace(langParts[1])
				switch field {
				case "country":
					geo.Names.Country[lang] = value
				case "province":
					geo.Names.Province[lang] = value
				case "city":
					geo.Names.City[lang] = value
				case "district":
					geo.Names.District[lang] = value
				case "isp":
					geo.Names.ISP[lang] = value
				}
			}
		}
	}

	return &cidrEntry{
		CIDR:      cidrPart,
		StartIP:   startIP,
		EndIP:     endIP,
		PrefixLen: prefixLen,
		NetworkIP: networkIP,
		Mask:      mask,
		GeoInfo:   geo,
	}, nil
}

func buildIndex(entries []cidrEntry) *ipIndex {
	sort.Slice(entries, func(i, j int) bool {
		if entries[i].StartIP != entries[j].StartIP {
			return entries[i].StartIP < entries[j].StartIP
		}
		return entries[i].PrefixLen > entries[j].PrefixLen
	})

	n := len(entries)
	maxEndIP := make([]uint32, n)
	if n > 0 {
		maxEndIP[0] = entries[0].EndIP
		for i := 1; i < n; i++ {
			if entries[i].EndIP > maxEndIP[i-1] {
				maxEndIP[i] = entries[i].EndIP
			} else {
				maxEndIP[i] = maxEndIP[i-1]
			}
		}
	}

	return &ipIndex{
		byStartIP: entries,
		maxEndIP:  maxEndIP,
	}
}

func (e *Engine) LoadFromFile(filePath string) error {
	if filePath == "" {
		return ErrFileNotExist
	}
	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		return ErrFileNotExist
	}

	file, err := os.Open(filePath)
	if err != nil {
		return fmt.Errorf("%w: %v", ErrInvalidDataFormat, err)
	}
	defer file.Close()

	var lines []string
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		lines = append(lines, scanner.Text())
	}
	if err := scanner.Err(); err != nil {
		return fmt.Errorf("%w: %v", ErrInvalidDataFormat, err)
	}

	return e.LoadFromData(lines)
}

func (e *Engine) LoadFromData(data []string) error {
	var entries []cidrEntry

	for _, line := range data {
		entry, err := parseDataLine(line)
		if err != nil {
			return err
		}
		if entry != nil {
			entries = append(entries, *entry)
		}
	}

	if len(entries) == 0 {
		return ErrEmptyData
	}

	e.reloadMu.Lock()
	defer e.reloadMu.Unlock()

	newIdx := buildIndex(entries)
	e.currentIdx.Store(newIdx)

	return nil
}

func (e *Engine) getIndex() *ipIndex {
	val := e.currentIdx.Load()
	if val == nil {
		return nil
	}
	return val.(*ipIndex)
}

func (e *Engine) Query(ipStr string) (*QueryResult, error) {
	return e.QueryWithLang(ipStr, "zh-CN")
}

func (e *Engine) QueryWithLang(ipStr string, lang string) (*QueryResult, error) {
	if strings.TrimSpace(ipStr) == "" {
		return nil, ErrInvalidIP
	}

	ip := net.ParseIP(ipStr)
	if ip == nil {
		return nil, ErrInvalidIP
	}

	ipUint, err := ipv4ToUint32(ip)
	if err != nil {
		return nil, err
	}

	idx := e.getIndex()
	if idx == nil {
		return nil, ErrEngineNotReady
	}

	entry := e.findLongestPrefixMatch(idx, ipUint)

	result := &QueryResult{
		IP:   ipStr,
		Lang: lang,
	}

	if entry == nil {
		return result, nil
	}

	result.Found = true
	result.Country = getLocalized(entry.GeoInfo.Names.Country, lang, entry.GeoInfo.Country)
	result.Province = getLocalized(entry.GeoInfo.Names.Province, lang, entry.GeoInfo.Province)
	result.City = getLocalized(entry.GeoInfo.Names.City, lang, entry.GeoInfo.City)
	result.District = getLocalized(entry.GeoInfo.Names.District, lang, entry.GeoInfo.District)
	result.ISP = getLocalized(entry.GeoInfo.Names.ISP, lang, entry.GeoInfo.ISP)

	return result, nil
}

func getLocalized(names map[string]string, lang string, fallback string) string {
	if val, ok := names[lang]; ok && val != "" {
		return val
	}
	hyphenIdx := strings.Index(lang, "-")
	if hyphenIdx > 0 {
		prefixLang := lang[:hyphenIdx]
		if val, ok := names[prefixLang]; ok && val != "" {
			return val
		}
	}
	return fallback
}

func (e *Engine) findLongestPrefixMatch(idx *ipIndex, targetIP uint32) *cidrEntry {
	entries := idx.byStartIP
	n := len(entries)
	if n == 0 {
		return nil
	}

	var bestMatch *cidrEntry
	bestPrefixLen := -1

	pos := sort.Search(n, func(i int) bool {
		return entries[i].StartIP > targetIP
	})
	pos--

	for i := pos; i >= 0; i-- {
		if idx.maxEndIP[i] < targetIP {
			break
		}
		entry := &entries[i]
		if entry.PrefixLen <= bestPrefixLen {
			continue
		}
		if targetIP >= entry.StartIP && targetIP <= entry.EndIP {
			if (targetIP & entry.Mask) == entry.NetworkIP {
				bestMatch = entry
				bestPrefixLen = entry.PrefixLen
				if bestPrefixLen == 32 {
					break
				}
			}
		}
	}

	if bestPrefixLen >= 0 {
		return bestMatch
	}
	return nil
}

func (e *Engine) LinearQueryWithLang(ipStr string, lang string) (*QueryResult, error) {
	if strings.TrimSpace(ipStr) == "" {
		return nil, ErrInvalidIP
	}

	ip := net.ParseIP(ipStr)
	if ip == nil {
		return nil, ErrInvalidIP
	}

	ipUint, err := ipv4ToUint32(ip)
	if err != nil {
		return nil, err
	}

	idx := e.getIndex()
	if idx == nil {
		return nil, ErrEngineNotReady
	}

	var bestMatch *cidrEntry
	bestPrefixLen := -1

	for i := range idx.byStartIP {
		entry := &idx.byStartIP[i]
		if ipUint >= entry.StartIP && ipUint <= entry.EndIP {
			if (ipUint & entry.Mask) == entry.NetworkIP {
				if entry.PrefixLen > bestPrefixLen {
					bestMatch = entry
					bestPrefixLen = entry.PrefixLen
				}
			}
		}
	}

	result := &QueryResult{
		IP:   ipStr,
		Lang: lang,
	}

	if bestMatch == nil {
		return result, nil
	}

	result.Found = true
	result.Country = getLocalized(bestMatch.GeoInfo.Names.Country, lang, bestMatch.GeoInfo.Country)
	result.Province = getLocalized(bestMatch.GeoInfo.Names.Province, lang, bestMatch.GeoInfo.Province)
	result.City = getLocalized(bestMatch.GeoInfo.Names.City, lang, bestMatch.GeoInfo.City)
	result.District = getLocalized(bestMatch.GeoInfo.Names.District, lang, bestMatch.GeoInfo.District)
	result.ISP = getLocalized(bestMatch.GeoInfo.Names.ISP, lang, bestMatch.GeoInfo.ISP)

	return result, nil
}

func (e *Engine) Count() int {
	idx := e.getIndex()
	if idx == nil {
		return 0
	}
	return len(idx.byStartIP)
}

func (e *Engine) IsReady() bool {
	return e.getIndex() != nil
}

func (e *Engine) HotReloadFromFile(filePath string) error {
	if filePath == "" {
		return ErrFileNotExist
	}
	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		return ErrFileNotExist
	}

	file, err := os.Open(filePath)
	if err != nil {
		return fmt.Errorf("%w: %v", ErrInvalidDataFormat, err)
	}
	defer file.Close()

	var lines []string
	scanner := bufio.NewScanner(file)
	buf := make([]byte, 0, 64*1024)
	scanner.Buffer(buf, 1024*1024)
	for scanner.Scan() {
		lines = append(lines, scanner.Text())
	}
	if err := scanner.Err(); err != nil {
		return fmt.Errorf("%w: %v", ErrInvalidDataFormat, err)
	}

	return e.HotReloadFromData(lines)
}

func (e *Engine) HotReloadFromData(data []string) error {
	var entries []cidrEntry

	for _, line := range data {
		entry, err := parseDataLine(line)
		if err != nil {
			return err
		}
		if entry != nil {
			entries = append(entries, *entry)
		}
	}

	if len(entries) == 0 {
		return ErrEmptyData
	}

	e.reloadMu.Lock()
	defer e.reloadMu.Unlock()

	newIdx := buildIndex(entries)
	e.currentIdx.Store(newIdx)

	return nil
}

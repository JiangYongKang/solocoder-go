package ipgeo

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"testing"
)

var testDataBasic = []string{
	"10.0.0.0/8\t中国\t北京\t北京\t朝阳区\t中国电信",
	"172.16.0.0/12\t中国\t上海\t上海\t浦东新区\t中国联通",
	"192.168.0.0/16\t中国\t广东\t深圳\t南山区\t中国移动",
}

var testDataMultiLang = []string{
	"10.0.0.0/8\t中国\t北京\t北京\t朝阳区\t中国电信\ten:country=China\ten:province=Beijing\ten:city=Beijing\ten:district=Chaoyang\ten:isp=China Telecom\tja:country=中国\tja:province=北京\tja:city=北京",
	"172.16.0.0/12\t中国\t上海\t上海\t浦东新区\t中国联通\ten:country=China\ten:province=Shanghai\ten:city=Shanghai\ten:district=Pudong\ten:isp=China Unicom",
	"192.168.0.0/16\t中国\t广东\t深圳\t南山区\t中国移动\ten:country=China\ten:province=Guangdong\ten:city=Shenzhen\ten:district=Nanshan\ten:isp=China Mobile",
}

var testDataOverlap = []string{
	"10.0.0.0/8\t国家A\t省份A\t城市A\t区域A\tISP-A",
	"10.1.0.0/16\t国家B\t省份B\t城市B\t区域B\tISP-B",
	"10.1.2.0/24\t国家C\t省份C\t城市C\t区域C\tISP-C",
	"10.1.2.3/32\t国家D\t省份D\t城市D\t区域D\tISP-D",
}

var testDataBoundary = []string{
	"0.0.0.0/8\t保留地址\t\t\t\t",
	"127.0.0.0/8\t回环地址\t\t\t\t",
	"169.254.0.0/16\t链路本地\t\t\t\t",
	"255.255.255.255/32\t广播地址\t\t\t\t",
	"224.0.0.0/4\t组播地址\t\t\t\t",
}

var testDataWithComments = []string{
	"# This is a comment line",
	"",
	"   ",
	"10.0.0.0/8\t中国\t北京\t北京\t朝阳区\t中国电信",
	"# Another comment",
	"192.168.0.0/16\t中国\t广东\t深圳\t南山区\t中国移动",
	"",
}

func TestNewEngine(t *testing.T) {
	e := NewEngine()
	if e == nil {
		t.Fatal("NewEngine returned nil")
	}
	if e.Count() != 0 {
		t.Errorf("expected 0 entries, got %d", e.Count())
	}
	if e.IsReady() {
		t.Error("expected IsReady to be false for empty engine")
	}
}

func TestNewEngineFromData(t *testing.T) {
	e, err := NewEngineFromData(testDataBasic)
	if err != nil {
		t.Fatalf("NewEngineFromData failed: %v", err)
	}
	if e == nil {
		t.Fatal("NewEngineFromData returned nil engine")
	}
	if e.Count() != 3 {
		t.Errorf("expected 3 entries, got %d", e.Count())
	}
	if !e.IsReady() {
		t.Error("expected IsReady to be true after loading data")
	}
}

func TestNewEngineFromDataEmpty(t *testing.T) {
	_, err := NewEngineFromData([]string{})
	if !errors.Is(err, ErrEmptyData) {
		t.Errorf("expected ErrEmptyData, got %v", err)
	}
}

func TestNewEngineFromDataOnlyComments(t *testing.T) {
	_, err := NewEngineFromData([]string{"# comment", "", "   "})
	if !errors.Is(err, ErrEmptyData) {
		t.Errorf("expected ErrEmptyData, got %v", err)
	}
}

func TestNewEngineFromFile(t *testing.T) {
	tmpDir := t.TempDir()
	filePath := filepath.Join(tmpDir, "ipdb.txt")
	content := "10.0.0.0/8\t中国\t北京\t北京\t朝阳区\t中国电信\n192.168.0.0/16\t中国\t广东\t深圳\t南山区\t中国移动\n"
	err := os.WriteFile(filePath, []byte(content), 0644)
	if err != nil {
		t.Fatalf("failed to write temp file: %v", err)
	}

	e, err := NewEngineFromFile(filePath)
	if err != nil {
		t.Fatalf("NewEngineFromFile failed: %v", err)
	}
	if e.Count() != 2 {
		t.Errorf("expected 2 entries, got %d", e.Count())
	}
}

func TestNewEngineFromFileNotExist(t *testing.T) {
	_, err := NewEngineFromFile("nonexistent_file_12345.txt")
	if !errors.Is(err, ErrFileNotExist) {
		t.Errorf("expected ErrFileNotExist, got %v", err)
	}
}

func TestNewEngineFromFileEmpty(t *testing.T) {
	tmpDir := t.TempDir()
	filePath := filepath.Join(tmpDir, "empty.txt")
	err := os.WriteFile(filePath, []byte(""), 0644)
	if err != nil {
		t.Fatalf("failed to write temp file: %v", err)
	}

	_, err = NewEngineFromFile(filePath)
	if !errors.Is(err, ErrEmptyData) {
		t.Errorf("expected ErrEmptyData, got %v", err)
	}
}

func TestNewEngineFromFileEmptyPath(t *testing.T) {
	_, err := NewEngineFromFile("")
	if !errors.Is(err, ErrFileNotExist) {
		t.Errorf("expected ErrFileNotExist, got %v", err)
	}
}

func TestQueryBasic(t *testing.T) {
	e, err := NewEngineFromData(testDataBasic)
	if err != nil {
		t.Fatalf("failed to create engine: %v", err)
	}

	result, err := e.Query("10.1.2.3")
	if err != nil {
		t.Fatalf("Query failed: %v", err)
	}
	if !result.Found {
		t.Error("expected Found=true for IP 10.1.2.3")
	}
	if result.Country != "中国" {
		t.Errorf("expected Country='中国', got '%s'", result.Country)
	}
	if result.Province != "北京" {
		t.Errorf("expected Province='北京', got '%s'", result.Province)
	}
	if result.City != "北京" {
		t.Errorf("expected City='北京', got '%s'", result.City)
	}
	if result.District != "朝阳区" {
		t.Errorf("expected District='朝阳区', got '%s'", result.District)
	}
	if result.ISP != "中国电信" {
		t.Errorf("expected ISP='中国电信', got '%s'", result.ISP)
	}
	if result.IP != "10.1.2.3" {
		t.Errorf("expected IP='10.1.2.3', got '%s'", result.IP)
	}
	if result.Lang != "zh-CN" {
		t.Errorf("expected Lang='zh-CN', got '%s'", result.Lang)
	}
}

func TestQueryNotFound(t *testing.T) {
	e, err := NewEngineFromData(testDataBasic)
	if err != nil {
		t.Fatalf("failed to create engine: %v", err)
	}

	result, err := e.Query("8.8.8.8")
	if err != nil {
		t.Fatalf("Query failed: %v", err)
	}
	if result.Found {
		t.Error("expected Found=false for IP 8.8.8.8")
	}
	if result.Country != "" {
		t.Errorf("expected empty Country for not found, got '%s'", result.Country)
	}
}

func TestQueryInvalidIP(t *testing.T) {
	e, err := NewEngineFromData(testDataBasic)
	if err != nil {
		t.Fatalf("failed to create engine: %v", err)
	}

	testCases := []string{
		"",
		"   ",
		"invalid",
		"256.1.1.1",
		"1.2.3.4.5",
		"abc.def.ghi.jkl",
		"::1",
	}

	for _, tc := range testCases {
		_, err := e.Query(tc)
		if !errors.Is(err, ErrInvalidIP) {
			t.Errorf("expected ErrInvalidIP for '%s', got %v", tc, err)
		}
	}
}

func TestQueryEmptyIP(t *testing.T) {
	e, err := NewEngineFromData(testDataBasic)
	if err != nil {
		t.Fatalf("failed to create engine: %v", err)
	}

	_, err = e.Query("")
	if !errors.Is(err, ErrInvalidIP) {
		t.Errorf("expected ErrInvalidIP for empty IP, got %v", err)
	}
}

func TestQueryNotReadyEngine(t *testing.T) {
	e := NewEngine()

	_, err := e.Query("10.0.0.1")
	if !errors.Is(err, ErrEngineNotReady) {
		t.Errorf("expected ErrEngineNotReady, got %v", err)
	}
}

func TestQueryLongestPrefixMatch(t *testing.T) {
	e, err := NewEngineFromData(testDataOverlap)
	if err != nil {
		t.Fatalf("failed to create engine: %v", err)
	}

	tests := []struct {
		ip       string
		country  string
		prefix   string
	}{
		{"10.2.3.4", "国家A", "/8"},
		{"10.1.3.4", "国家B", "/16"},
		{"10.1.2.4", "国家C", "/24"},
		{"10.1.2.3", "国家D", "/32"},
		{"10.1.2.2", "国家C", "/24"},
		{"10.1.0.1", "国家B", "/16"},
		{"10.255.255.255", "国家A", "/8"},
	}

	for _, tc := range tests {
		result, err := e.Query(tc.ip)
		if err != nil {
			t.Fatalf("Query for %s failed: %v", tc.ip, err)
		}
		if !result.Found {
			t.Errorf("IP %s: expected Found=true", tc.ip)
			continue
		}
		if result.Country != tc.country {
			t.Errorf("IP %s (%s): expected Country='%s', got '%s'", tc.ip, tc.prefix, tc.country, result.Country)
		}
	}
}

func TestQueryMultiLangEnglish(t *testing.T) {
	e, err := NewEngineFromData(testDataMultiLang)
	if err != nil {
		t.Fatalf("failed to create engine: %v", err)
	}

	result, err := e.QueryWithLang("10.1.2.3", "en")
	if err != nil {
		t.Fatalf("QueryWithLang failed: %v", err)
	}
	if !result.Found {
		t.Error("expected Found=true")
	}
	if result.Country != "China" {
		t.Errorf("expected Country='China', got '%s'", result.Country)
	}
	if result.Province != "Beijing" {
		t.Errorf("expected Province='Beijing', got '%s'", result.Province)
	}
	if result.City != "Beijing" {
		t.Errorf("expected City='Beijing', got '%s'", result.City)
	}
	if result.District != "Chaoyang" {
		t.Errorf("expected District='Chaoyang', got '%s'", result.District)
	}
	if result.ISP != "China Telecom" {
		t.Errorf("expected ISP='China Telecom', got '%s'", result.ISP)
	}
	if result.Lang != "en" {
		t.Errorf("expected Lang='en', got '%s'", result.Lang)
	}
}

func TestQueryMultiLangJapanese(t *testing.T) {
	e, err := NewEngineFromData(testDataMultiLang)
	if err != nil {
		t.Fatalf("failed to create engine: %v", err)
	}

	result, err := e.QueryWithLang("10.1.2.3", "ja")
	if err != nil {
		t.Fatalf("QueryWithLang failed: %v", err)
	}
	if !result.Found {
		t.Error("expected Found=true")
	}
	if result.Country != "中国" {
		t.Errorf("expected Country='中国', got '%s'", result.Country)
	}
	if result.Province != "北京" {
		t.Errorf("expected Province='北京', got '%s'", result.Province)
	}
	if result.City != "北京" {
		t.Errorf("expected City='北京', got '%s'", result.City)
	}
}

func TestQueryMultiLangFallback(t *testing.T) {
	e, err := NewEngineFromData(testDataMultiLang)
	if err != nil {
		t.Fatalf("failed to create engine: %v", err)
	}

	result, err := e.QueryWithLang("10.1.2.3", "fr")
	if err != nil {
		t.Fatalf("QueryWithLang failed: %v", err)
	}
	if !result.Found {
		t.Error("expected Found=true")
	}
	if result.Country != "中国" {
		t.Errorf("expected fallback Country='中国', got '%s'", result.Country)
	}
}

func TestQueryMultiLangPrefixFallback(t *testing.T) {
	testData := []string{
		"10.0.0.0/8\t中\t北\t北\t朝\t电信\ten:country=China\ten:province=Beijing",
	}
	e, err := NewEngineFromData(testData)
	if err != nil {
		t.Fatalf("failed to create engine: %v", err)
	}

	result, err := e.QueryWithLang("10.1.2.3", "en-US")
	if err != nil {
		t.Fatalf("QueryWithLang failed: %v", err)
	}
	if !result.Found {
		t.Error("expected Found=true")
	}
	if result.Country != "China" {
		t.Errorf("expected prefix fallback Country='China', got '%s'", result.Country)
	}
}

func TestQueryWithLangDefaultChinese(t *testing.T) {
	e, err := NewEngineFromData(testDataBasic)
	if err != nil {
		t.Fatalf("failed to create engine: %v", err)
	}

	result, err := e.Query("10.1.2.3")
	if err != nil {
		t.Fatalf("Query failed: %v", err)
	}
	if result.Lang != "zh-CN" {
		t.Errorf("expected default Lang='zh-CN', got '%s'", result.Lang)
	}
	if result.Country != "中国" {
		t.Errorf("expected Country='中国', got '%s'", result.Country)
	}
}

func TestLinearQuery(t *testing.T) {
	e, err := NewEngineFromData(testDataOverlap)
	if err != nil {
		t.Fatalf("failed to create engine: %v", err)
	}

	tests := []string{
		"10.2.3.4",
		"10.1.3.4",
		"10.1.2.4",
		"10.1.2.3",
		"8.8.8.8",
	}

	for _, ip := range tests {
		binaryResult, err := e.QueryWithLang(ip, "zh-CN")
		if err != nil {
			t.Fatalf("Query for %s failed: %v", ip, err)
		}

		linearResult, err := e.LinearQueryWithLang(ip, "zh-CN")
		if err != nil {
			t.Fatalf("LinearQuery for %s failed: %v", ip, err)
		}

		if binaryResult.Found != linearResult.Found {
			t.Errorf("IP %s: Found mismatch binary=%v linear=%v", ip, binaryResult.Found, linearResult.Found)
		}
		if binaryResult.Country != linearResult.Country {
			t.Errorf("IP %s: Country mismatch binary=%s linear=%s", ip, binaryResult.Country, linearResult.Country)
		}
		if binaryResult.Province != linearResult.Province {
			t.Errorf("IP %s: Province mismatch binary=%s linear=%s", ip, binaryResult.Province, linearResult.Province)
		}
		if binaryResult.City != linearResult.City {
			t.Errorf("IP %s: City mismatch binary=%s linear=%s", ip, binaryResult.City, linearResult.City)
		}
	}
}

func TestLinearQueryInvalidIP(t *testing.T) {
	e, err := NewEngineFromData(testDataBasic)
	if err != nil {
		t.Fatalf("failed to create engine: %v", err)
	}

	_, err = e.LinearQueryWithLang("", "zh-CN")
	if !errors.Is(err, ErrInvalidIP) {
		t.Errorf("expected ErrInvalidIP, got %v", err)
	}
}

func TestLinearQueryNotReady(t *testing.T) {
	e := NewEngine()

	_, err := e.LinearQueryWithLang("10.0.0.1", "zh-CN")
	if !errors.Is(err, ErrEngineNotReady) {
		t.Errorf("expected ErrEngineNotReady, got %v", err)
	}
}

func TestQueryBoundaryIPs(t *testing.T) {
	e, err := NewEngineFromData(testDataBoundary)
	if err != nil {
		t.Fatalf("failed to create engine: %v", err)
	}

	tests := []struct {
		ip      string
		found   bool
		country string
	}{
		{"0.0.0.0", true, "保留地址"},
		{"0.255.255.255", true, "保留地址"},
		{"127.0.0.1", true, "回环地址"},
		{"127.255.255.255", true, "回环地址"},
		{"169.254.1.1", true, "链路本地"},
		{"169.254.255.255", true, "链路本地"},
		{"255.255.255.255", true, "广播地址"},
		{"224.0.0.1", true, "组播地址"},
		{"239.255.255.255", true, "组播地址"},
		{"1.1.1.1", false, ""},
	}

	for _, tc := range tests {
		result, err := e.Query(tc.ip)
		if err != nil {
			t.Fatalf("Query for %s failed: %v", tc.ip, err)
		}
		if result.Found != tc.found {
			t.Errorf("IP %s: expected Found=%v, got %v", tc.ip, tc.found, result.Found)
		}
		if tc.found && result.Country != tc.country {
			t.Errorf("IP %s: expected Country='%s', got '%s'", tc.ip, tc.country, result.Country)
		}
	}
}

func TestQueryCIDRStartEnd(t *testing.T) {
	e, err := NewEngineFromData(testDataBasic)
	if err != nil {
		t.Fatalf("failed to create engine: %v", err)
	}

	tests := []struct {
		ip      string
		found   bool
		country string
	}{
		{"10.0.0.0", true, "中国"},
		{"10.255.255.255", true, "中国"},
		{"172.16.0.0", true, "中国"},
		{"172.31.255.255", true, "中国"},
		{"192.168.0.0", true, "中国"},
		{"192.168.255.255", true, "中国"},
		{"9.255.255.255", false, ""},
		{"11.0.0.0", false, ""},
	}

	for _, tc := range tests {
		result, err := e.Query(tc.ip)
		if err != nil {
			t.Fatalf("Query for %s failed: %v", tc.ip, err)
		}
		if result.Found != tc.found {
			t.Errorf("IP %s: expected Found=%v, got %v", tc.ip, tc.found, result.Found)
		}
		if tc.found && result.Country != tc.country {
			t.Errorf("IP %s: expected Country='%s', got '%s'", tc.ip, tc.country, result.Country)
		}
	}
}

func TestLoadFromDataWithComments(t *testing.T) {
	e, err := NewEngineFromData(testDataWithComments)
	if err != nil {
		t.Fatalf("NewEngineFromData failed: %v", err)
	}
	if e.Count() != 2 {
		t.Errorf("expected 2 entries after comments/empty filtered, got %d", e.Count())
	}
}

func TestLoadFromDataInvalidCIDR(t *testing.T) {
	badData := []string{
		"invalid-cidr\t中国\t北京",
	}
	_, err := NewEngineFromData(badData)
	if !errors.Is(err, ErrInvalidCIDR) {
		t.Errorf("expected ErrInvalidCIDR, got %v", err)
	}
}

func TestLoadFromDataInvalidFormat(t *testing.T) {
	badData := []string{
		"10.0.0.0/8",
	}
	_, err := NewEngineFromData(badData)
	if !errors.Is(err, ErrInvalidDataFormat) {
		t.Errorf("expected ErrInvalidDataFormat, got %v", err)
	}
}

func TestLoadFromDataEmptyCIDR(t *testing.T) {
	badData := []string{
		"\t中国\t北京",
	}
	_, err := NewEngineFromData(badData)
	if !errors.Is(err, ErrEmptyCIDR) {
		t.Errorf("expected ErrEmptyCIDR, got %v", err)
	}
}

func TestHotReloadFromData(t *testing.T) {
	e, err := NewEngineFromData(testDataBasic)
	if err != nil {
		t.Fatalf("failed to create engine: %v", err)
	}

	result, err := e.Query("10.1.2.3")
	if err != nil {
		t.Fatalf("Query failed: %v", err)
	}
	if result.Country != "中国" || result.Province != "北京" {
		t.Errorf("unexpected initial result: %+v", result)
	}

	newData := []string{
		"10.0.0.0/8\t美国\t加利福尼亚\t旧金山\t\tGoogle",
		"8.8.8.0/24\t美国\t加利福尼亚\t山景城\t\tGoogle DNS",
	}

	err = e.HotReloadFromData(newData)
	if err != nil {
		t.Fatalf("HotReloadFromData failed: %v", err)
	}

	if e.Count() != 2 {
		t.Errorf("expected 2 entries after hot reload, got %d", e.Count())
	}

	result, err = e.Query("10.1.2.3")
	if err != nil {
		t.Fatalf("Query after hot reload failed: %v", err)
	}
	if !result.Found {
		t.Error("expected Found=true after hot reload")
	}
	if result.Country != "美国" {
		t.Errorf("expected Country='美国' after hot reload, got '%s'", result.Country)
	}
	if result.Province != "加利福尼亚" {
		t.Errorf("expected Province='加利福尼亚' after hot reload, got '%s'", result.Province)
	}

	result, err = e.Query("8.8.8.8")
	if err != nil {
		t.Fatalf("Query 8.8.8.8 after hot reload failed: %v", err)
	}
	if !result.Found {
		t.Error("expected Found=true for 8.8.8.8 after hot reload")
	}
}

func TestHotReloadFromDataEmpty(t *testing.T) {
	e, err := NewEngineFromData(testDataBasic)
	if err != nil {
		t.Fatalf("failed to create engine: %v", err)
	}

	err = e.HotReloadFromData([]string{})
	if !errors.Is(err, ErrEmptyData) {
		t.Errorf("expected ErrEmptyData for empty hot reload, got %v", err)
	}

	if e.Count() != 3 {
		t.Errorf("expected 3 entries preserved after failed hot reload, got %d", e.Count())
	}
}

func TestHotReloadFromDataInvalid(t *testing.T) {
	e, err := NewEngineFromData(testDataBasic)
	if err != nil {
		t.Fatalf("failed to create engine: %v", err)
	}

	badData := []string{"invalid-cidr\t测试"}
	err = e.HotReloadFromData(badData)
	if !errors.Is(err, ErrInvalidCIDR) {
		t.Errorf("expected ErrInvalidCIDR, got %v", err)
	}

	if e.Count() != 3 {
		t.Errorf("expected 3 entries preserved after failed hot reload, got %d", e.Count())
	}
}

func TestHotReloadFromFile(t *testing.T) {
	e, err := NewEngineFromData(testDataBasic)
	if err != nil {
		t.Fatalf("failed to create engine: %v", err)
	}

	tmpDir := t.TempDir()
	filePath := filepath.Join(tmpDir, "new_ipdb.txt")
	content := "8.8.8.0/24\t美国\t加利福尼亚\t山景城\t\tGoogle DNS\n"
	err = os.WriteFile(filePath, []byte(content), 0644)
	if err != nil {
		t.Fatalf("failed to write temp file: %v", err)
	}

	err = e.HotReloadFromFile(filePath)
	if err != nil {
		t.Fatalf("HotReloadFromFile failed: %v", err)
	}

	if e.Count() != 1 {
		t.Errorf("expected 1 entry after hot reload, got %d", e.Count())
	}

	result, err := e.Query("8.8.8.8")
	if err != nil {
		t.Fatalf("Query after hot reload failed: %v", err)
	}
	if !result.Found {
		t.Error("expected Found=true for 8.8.8.8")
	}
}

func TestHotReloadFromFileNotExist(t *testing.T) {
	e, err := NewEngineFromData(testDataBasic)
	if err != nil {
		t.Fatalf("failed to create engine: %v", err)
	}

	err = e.HotReloadFromFile("nonexistent_hot_reload.txt")
	if !errors.Is(err, ErrFileNotExist) {
		t.Errorf("expected ErrFileNotExist, got %v", err)
	}
}

func TestHotReloadAtomicSwitch(t *testing.T) {
	dataV1 := []string{
		"10.0.0.0/8\tV1\tV1\tV1\tV1\tV1",
	}
	dataV2 := []string{
		"10.0.0.0/8\tV2\tV2\tV2\tV2\tV2",
	}

	e, err := NewEngineFromData(dataV1)
	if err != nil {
		t.Fatalf("failed to create engine: %v", err)
	}

	var wg sync.WaitGroup
	readerCount := 10
	reloadCount := 100
	queriesPerReader := 1000

	errorsCh := make(chan error, 1000)
	mismatchCh := make(chan string, 1000)

	for i := 0; i < readerCount; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			for j := 0; j < queriesPerReader; j++ {
				result, err := e.Query("10.0.0.1")
				if err != nil {
					errorsCh <- fmt.Errorf("reader %d query error: %w", id, err)
					return
				}
				if !result.Found {
					mismatchCh <- fmt.Sprintf("reader %d: expected Found=true", id)
					return
				}
				if result.Country != "V1" && result.Country != "V2" {
					mismatchCh <- fmt.Sprintf("reader %d: unexpected Country='%s'", id, result.Country)
					return
				}
			}
		}(i)
	}

	wg.Add(1)
	go func() {
		defer wg.Done()
		for i := 0; i < reloadCount; i++ {
			var data []string
			if i%2 == 0 {
				data = dataV2
			} else {
				data = dataV1
			}
			err := e.HotReloadFromData(data)
			if err != nil {
				errorsCh <- fmt.Errorf("reload error: %w", err)
				return
			}
		}
	}()

	wg.Wait()
	close(errorsCh)
	close(mismatchCh)

	for err := range errorsCh {
		t.Error(err)
	}
	for msg := range mismatchCh {
		t.Error(msg)
	}
}

func TestConcurrentRead(t *testing.T) {
	data := make([]string, 0, 100)
	for i := 0; i < 100; i++ {
		a := i / 256
		b := i % 256
		data = append(data, fmt.Sprintf("%d.%d.0.0/16\t国家%d\t省份%d\t城市%d\t\tISP%d", a, b, i, i, i, i))
	}

	e, err := NewEngineFromData(data)
	if err != nil {
		t.Fatalf("failed to create engine: %v", err)
	}

	var wg sync.WaitGroup
	goroutines := 20
	queries := 500

	errCh := make(chan error, goroutines*queries)

	for g := 0; g < goroutines; g++ {
		wg.Add(1)
		go func(gid int) {
			defer wg.Done()
			for q := 0; q < queries; q++ {
				a := q % 100
				c := q % 256
				d := q % 256
				ip := fmt.Sprintf("%d.%d.%d.%d", a/256, a%256, c, d)
				result, err := e.Query(ip)
				if err != nil {
					errCh <- fmt.Errorf("goroutine %d query %s error: %w", gid, ip, err)
					return
				}
				_ = result
			}
		}(g)
	}

	wg.Wait()
	close(errCh)

	for err := range errCh {
		t.Error(err)
	}
}

func TestLoadFromFileInvalidFormat(t *testing.T) {
	tmpDir := t.TempDir()
	filePath := filepath.Join(tmpDir, "bad_format.txt")
	content := "this-is-not-valid-data\n"
	err := os.WriteFile(filePath, []byte(content), 0644)
	if err != nil {
		t.Fatalf("failed to write temp file: %v", err)
	}

	e := NewEngine()
	err = e.LoadFromFile(filePath)
	if err == nil {
		t.Error("expected error for invalid file format")
	}
}

func TestParseCIDREdges(t *testing.T) {
	tests := []struct {
		cidr      string
		expectErr error
	}{
		{"", ErrEmptyCIDR},
		{"   ", ErrEmptyCIDR},
		{"invalid", ErrInvalidCIDR},
		{"1.2.3.4", ErrInvalidCIDR},
		{"999.1.1.1/24", ErrInvalidCIDR},
		{"0.0.0.0/0", nil},
		{"0.0.0.0/32", nil},
		{"255.255.255.255/32", nil},
		{"10.0.0.0/8", nil},
		{"192.168.1.1/32", nil},
	}

	for _, tc := range tests {
		_, _, _, _, _, err := parseCIDR(tc.cidr)
		if tc.expectErr != nil {
			if !errors.Is(err, tc.expectErr) {
				t.Errorf("parseCIDR('%s'): expected %v, got %v", tc.cidr, tc.expectErr, err)
			}
		} else {
			if err != nil {
				t.Errorf("parseCIDR('%s'): unexpected error %v", tc.cidr, err)
			}
		}
	}
}

func TestGetLocalized(t *testing.T) {
	names := map[string]string{
		"zh-CN": "北京",
		"en":    "Beijing",
		"en-GB": "London",
	}

	tests := []struct {
		lang     string
		expected string
	}{
		{"zh-CN", "北京"},
		{"en", "Beijing"},
		{"en-US", "Beijing"},
		{"en-GB", "London"},
		{"fr", "默认"},
	}

	for _, tc := range tests {
		result := getLocalized(names, tc.lang, "默认")
		if result != tc.expected {
			t.Errorf("getLocalized(%s): expected '%s', got '%s'", tc.lang, tc.expected, result)
		}
	}
}

func TestCountLeadingZeros32(t *testing.T) {
	tests := []struct {
		x        uint32
		expected int
	}{
		{0, 32},
		{1, 31},
		{2, 30},
		{0xFFFFFFFF, 0},
		{0x80000000, 0},
		{0x00000001, 31},
	}

	for _, tc := range tests {
		result := CountLeadingZeros32(tc.x)
		if result != tc.expected {
			t.Errorf("CountLeadingZeros32(%d): expected %d, got %d", tc.x, tc.expected, result)
		}
	}
}

func TestMinimalFieldsData(t *testing.T) {
	data := []string{
		"1.0.0.0/8\tCountryOnly",
	}
	e, err := NewEngineFromData(data)
	if err != nil {
		t.Fatalf("failed to create engine: %v", err)
	}
	if e.Count() != 1 {
		t.Errorf("expected 1 entry, got %d", e.Count())
	}

	result, err := e.Query("1.2.3.4")
	if err != nil {
		t.Fatalf("Query failed: %v", err)
	}
	if !result.Found {
		t.Error("expected Found=true")
	}
	if result.Country != "CountryOnly" {
		t.Errorf("expected Country='CountryOnly', got '%s'", result.Country)
	}
}

func TestSpaceSeparatedData(t *testing.T) {
	data := []string{
		"10.0.0.0/8 中国 北京 北京 朝阳区 中国电信",
	}
	e, err := NewEngineFromData(data)
	if err != nil {
		t.Fatalf("failed to create engine: %v", err)
	}

	result, err := e.Query("10.1.2.3")
	if err != nil {
		t.Fatalf("Query failed: %v", err)
	}
	if !result.Found {
		t.Error("expected Found=true")
	}
	if result.Country != "中国" {
		t.Errorf("expected Country='中国', got '%s'", result.Country)
	}
}

func TestIPv6Rejected(t *testing.T) {
	e, err := NewEngineFromData(testDataBasic)
	if err != nil {
		t.Fatalf("failed to create engine: %v", err)
	}

	ipv6Addrs := []string{
		"::1",
		"2001:db8::1",
		"fe80::1",
	}

	for _, ip := range ipv6Addrs {
		_, err := e.Query(ip)
		if !errors.Is(err, ErrInvalidIP) {
			t.Errorf("expected ErrInvalidIP for IPv6 '%s', got %v", ip, err)
		}
	}
}

func TestSingleHostCIDR(t *testing.T) {
	data := []string{
		"10.0.0.1/32\t主机A\t专属\t专属\t专属\t专属",
		"10.0.0.0/8\t大网段\t通用\t通用\t通用\t通用",
	}
	e, err := NewEngineFromData(data)
	if err != nil {
		t.Fatalf("failed to create engine: %v", err)
	}

	result, err := e.Query("10.0.0.1")
	if err != nil {
		t.Fatalf("Query failed: %v", err)
	}
	if result.Country != "主机A" {
		t.Errorf("expected /32 match Country='主机A', got '%s'", result.Country)
	}

	result, err = e.Query("10.0.0.2")
	if err != nil {
		t.Fatalf("Query failed: %v", err)
	}
	if result.Country != "大网段" {
		t.Errorf("expected /8 match Country='大网段', got '%s'", result.Country)
	}
}

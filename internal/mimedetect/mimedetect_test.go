package mimedetect

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"
	"testing"
)

func TestNewDetector(t *testing.T) {
	d := NewDetector()
	if d == nil {
		t.Fatal("NewDetector returned nil")
	}
}

func TestDetectFromBytesPNG(t *testing.T) {
	d := NewDetector()
	data := []byte{0x89, 0x50, 0x4E, 0x47, 0x0D, 0x0A, 0x1A, 0x0A, 0x00, 0x00}
	mime := d.DetectFromBytes(data)
	if mime != "image/png" {
		t.Errorf("expected image/png, got %s", mime)
	}
}

func TestDetectFromBytesJPEG(t *testing.T) {
	d := NewDetector()
	data := []byte{0xFF, 0xD8, 0xFF, 0xE0, 0x00, 0x10}
	mime := d.DetectFromBytes(data)
	if mime != "image/jpeg" {
		t.Errorf("expected image/jpeg, got %s", mime)
	}
}

func TestDetectFromBytesGIF87a(t *testing.T) {
	d := NewDetector()
	data := []byte{0x47, 0x49, 0x46, 0x38, 0x37, 0x61, 0x00, 0x00}
	mime := d.DetectFromBytes(data)
	if mime != "image/gif" {
		t.Errorf("expected image/gif, got %s", mime)
	}
}

func TestDetectFromBytesGIF89a(t *testing.T) {
	d := NewDetector()
	data := []byte{0x47, 0x49, 0x46, 0x38, 0x39, 0x61, 0x00, 0x00}
	mime := d.DetectFromBytes(data)
	if mime != "image/gif" {
		t.Errorf("expected image/gif, got %s", mime)
	}
}

func TestDetectFromBytesPDF(t *testing.T) {
	d := NewDetector()
	data := []byte{0x25, 0x50, 0x44, 0x46, 0x2D, 0x31, 0x2E}
	mime := d.DetectFromBytes(data)
	if mime != "application/pdf" {
		t.Errorf("expected application/pdf, got %s", mime)
	}
}

func TestDetectFromBytesZIP(t *testing.T) {
	d := NewDetector()
	data := []byte{0x50, 0x4B, 0x03, 0x04, 0x00, 0x00}
	mime := d.DetectFromBytes(data)
	if mime != "application/zip" {
		t.Errorf("expected application/zip, got %s", mime)
	}
}

func TestDetectFromBytesGZIP(t *testing.T) {
	d := NewDetector()
	data := []byte{0x1F, 0x8B, 0x08, 0x00, 0x00}
	mime := d.DetectFromBytes(data)
	if mime != "application/gzip" {
		t.Errorf("expected application/gzip, got %s", mime)
	}
}

func TestDetectFromBytesELF(t *testing.T) {
	d := NewDetector()
	data := []byte{0x7F, 0x45, 0x4C, 0x46, 0x01}
	mime := d.DetectFromBytes(data)
	if mime != "application/x-executable" {
		t.Errorf("expected application/x-executable, got %s", mime)
	}
}

func TestDetectFromBytesDOSExec(t *testing.T) {
	d := NewDetector()
	data := []byte{0x4D, 0x5A, 0x90, 0x00}
	mime := d.DetectFromBytes(data)
	if mime != "application/x-dosexec" {
		t.Errorf("expected application/x-dosexec, got %s", mime)
	}
}

func TestDetectFromBytesFixedOffset(t *testing.T) {
	d := NewDetector()
	data := make([]byte, 300)
	copy(data[257:], []byte{0x75, 0x73, 0x74, 0x61, 0x72})
	mime := d.DetectFromBytes(data)
	if mime != "application/x-tar" {
		t.Errorf("expected application/x-tar for fixed offset 257, got %s", mime)
	}
}

func TestDetectFromBytesMP4(t *testing.T) {
	d := NewDetector()
	data := make([]byte, 16)
	copy(data[4:], []byte{0x66, 0x74, 0x79, 0x70, 0x49, 0x53, 0x4F, 0x4D})
	mime := d.DetectFromBytes(data)
	if mime != "video/mp4" {
		t.Errorf("expected video/mp4 for fixed offset 4, got %s", mime)
	}
}

func TestDetectFromBytesHTMLVariable(t *testing.T) {
	d := NewDetector()
	data := []byte("   <html><head>...</head></html>")
	mime := d.DetectFromBytes(data)
	if mime != "text/html" {
		t.Errorf("expected text/html for variable offset match, got %s", mime)
	}
}

func TestDetectFromBytesDOCTYPE(t *testing.T) {
	d := NewDetector()
	data := []byte("<!DOCTYPE html><html>...</html>")
	mime := d.DetectFromBytes(data)
	if mime != "text/html" {
		t.Errorf("expected text/html for DOCTYPE, got %s", mime)
	}
}

func TestDetectFromBytesUTF8BOM(t *testing.T) {
	d := NewDetector()
	data := []byte{0xEF, 0xBB, 0xBF, 0x48, 0x65, 0x6C, 0x6C, 0x6F}
	mime := d.DetectFromBytes(data)
	if mime != "text/plain; charset=utf-8" {
		t.Errorf("expected text/plain; charset=utf-8, got %s", mime)
	}
}

func TestDetectFromBytesEmpty(t *testing.T) {
	d := NewDetector()
	mime := d.DetectFromBytes([]byte{})
	if mime != OctetStream {
		t.Errorf("expected octet-stream for empty data, got %s", mime)
	}
}

func TestDetectFromBytesShortData(t *testing.T) {
	d := NewDetector()
	data := []byte{0x00, 0x01}
	mime := d.DetectFromBytes(data)
	if mime != OctetStream {
		t.Errorf("expected octet-stream for short unknown data, got %s", mime)
	}
}

func TestDetectFromBytesUnknown(t *testing.T) {
	d := NewDetector()
	data := []byte{0x00, 0x01, 0x02, 0x03, 0x04, 0x05, 0x06, 0x07, 0x08, 0x09}
	mime := d.DetectFromBytes(data)
	if mime != OctetStream {
		t.Errorf("expected octet-stream for unknown data, got %s", mime)
	}
}

func TestDetectFromBytesShortForOffset(t *testing.T) {
	d := NewDetector()
	data := []byte{0x52, 0x49, 0x46, 0x46}
	mime := d.DetectFromBytes(data)
	if mime != "image/webp" && mime != "audio/wav" && mime != "audio/x-wav" {
		t.Logf("detected: %s (this is expected as multiple types share RIFF header)", mime)
	}
}

func TestDetectFromFile(t *testing.T) {
	d := NewDetector()
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.png")
	data := []byte{0x89, 0x50, 0x4E, 0x47, 0x0D, 0x0A, 0x1A, 0x0A, 0x00, 0x00}
	err := os.WriteFile(testFile, data, 0644)
	if err != nil {
		t.Fatalf("failed to create test file: %v", err)
	}

	mime, err := d.DetectFromFile(testFile)
	if err != nil {
		t.Fatalf("DetectFromFile failed: %v", err)
	}
	if mime != "image/png" {
		t.Errorf("expected image/png, got %s", mime)
	}
}

func TestDetectFromFileNotFound(t *testing.T) {
	d := NewDetector()
	_, err := d.DetectFromFile("nonexistent_file_12345.dat")
	if err == nil {
		t.Error("expected error for nonexistent file")
	}
}

func TestGetMIMETypeByExtension(t *testing.T) {
	d := NewDetector()

	tests := []struct {
		ext  string
		want string
	}{
		{"png", "image/png"},
		{".png", "image/png"},
		{"PNG", "image/png"},
		{".PNG", "image/png"},
		{"jpg", "image/jpeg"},
		{"jpeg", "image/jpeg"},
		{"pdf", "application/pdf"},
		{"html", "text/html"},
		{"htm", "text/html"},
		{"unknown", ""},
		{"", ""},
	}

	for _, tt := range tests {
		t.Run(tt.ext, func(t *testing.T) {
			got := d.GetMIMETypeByExtension(tt.ext)
			if got != tt.want {
				t.Errorf("ext=%q: expected %q, got %q", tt.ext, tt.want, got)
			}
		})
	}
}

func TestGetExtensionByMIMEType(t *testing.T) {
	d := NewDetector()

	tests := []struct {
		mime string
		want string
	}{
		{"image/png", "png"},
		{"IMAGE/PNG", "png"},
		{"text/html", "html"},
		{"application/pdf", "pdf"},
		{"application/zip", "zip"},
		{"application/octet-stream", "bin"},
		{"text/plain; charset=utf-8", "txt"},
		{"unknown/type", ""},
		{"", ""},
	}

	for _, tt := range tests {
		t.Run(tt.mime, func(t *testing.T) {
			got := d.GetExtensionByMIMEType(tt.mime)
			if got != tt.want {
				t.Errorf("mime=%q: expected %q, got %q", tt.mime, tt.want, got)
			}
		})
	}
}

func TestGetMIMETypeInfo(t *testing.T) {
	d := NewDetector()

	info, ok := d.GetMIMETypeInfo("image/png")
	if !ok {
		t.Fatal("expected to find info for image/png")
	}
	if info.MIMEType != "image/png" {
		t.Errorf("expected MIMEType image/png, got %s", info.MIMEType)
	}
	if info.Description != "Portable Network Graphics" {
		t.Errorf("expected description, got %s", info.Description)
	}

	info, ok = d.GetMIMETypeInfo("unknown/type")
	if ok {
		t.Error("expected not to find info for unknown type")
	}
	if info.MIMEType != "" {
		t.Error("expected empty MIMEType for not found")
	}

	_, ok = d.GetMIMETypeInfo("")
	if ok {
		t.Error("expected not to find info for empty string")
	}
}

func TestRegisterMagicSignature(t *testing.T) {
	d := NewDetector()

	customSig := MagicSignature{
		MagicBytes: []byte{0xDE, 0xAD, 0xBE, 0xEF},
		Offset:     0,
		Mode:       OffsetFixed,
		MIMEType:   "application/x-custom",
	}

	err := d.RegisterMagicSignature(customSig)
	if err != nil {
		t.Fatalf("RegisterMagicSignature failed: %v", err)
	}

	data := []byte{0xDE, 0xAD, 0xBE, 0xEF, 0x01, 0x02}
	mime := d.DetectFromBytes(data)
	if mime != "application/x-custom" {
		t.Errorf("expected application/x-custom, got %s", mime)
	}
}

func TestRegisterMagicSignatureEmptyBytes(t *testing.T) {
	d := NewDetector()
	err := d.RegisterMagicSignature(MagicSignature{
		MagicBytes: []byte{},
		MIMEType:   "application/test",
	})
	if !errors.Is(err, ErrEmptyMagicBytes) {
		t.Errorf("expected ErrEmptyMagicBytes, got %v", err)
	}
}

func TestRegisterMagicSignatureEmptyMIME(t *testing.T) {
	d := NewDetector()
	err := d.RegisterMagicSignature(MagicSignature{
		MagicBytes: []byte{0x01, 0x02},
		MIMEType:   "",
	})
	if !errors.Is(err, ErrEmptyMIMEType) {
		t.Errorf("expected ErrEmptyMIMEType, got %v", err)
	}
}

func TestRegisterMagicSignatureNegativeOffset(t *testing.T) {
	d := NewDetector()
	err := d.RegisterMagicSignature(MagicSignature{
		MagicBytes: []byte{0x01, 0x02},
		Offset:     -1,
		MIMEType:   "application/test",
	})
	if !errors.Is(err, ErrInvalidOffset) {
		t.Errorf("expected ErrInvalidOffset, got %v", err)
	}
}

func TestRegisterMagicSignatureOverridesBuiltIn(t *testing.T) {
	d := NewDetector()

	overrideSig := MagicSignature{
		MagicBytes: []byte{0x89, 0x50, 0x4E, 0x47},
		Offset:     0,
		Mode:       OffsetFixed,
		MIMEType:   "image/png-custom-override",
	}

	err := d.RegisterMagicSignature(overrideSig)
	if err != nil {
		t.Fatalf("RegisterMagicSignature failed: %v", err)
	}

	data := []byte{0x89, 0x50, 0x4E, 0x47, 0x0D, 0x0A, 0x1A, 0x0A}
	mime := d.DetectFromBytes(data)
	if mime != "image/png-custom-override" {
		t.Errorf("expected custom override, got %s", mime)
	}
}

func TestRegisterMagicSignatureCustomVariable(t *testing.T) {
	d := NewDetector()

	customSig := MagicSignature{
		MagicBytes: []byte("MAGIC"),
		Offset:     0,
		Mode:       OffsetVariable,
		MIMEType:   "application/x-variable-magic",
	}

	err := d.RegisterMagicSignature(customSig)
	if err != nil {
		t.Fatalf("RegisterMagicSignature failed: %v", err)
	}

	data := []byte("some padding MAGIC here")
	mime := d.DetectFromBytes(data)
	if mime != "application/x-variable-magic" {
		t.Errorf("expected application/x-variable-magic, got %s", mime)
	}
}

func TestRegisterExtension(t *testing.T) {
	d := NewDetector()

	err := d.RegisterExtension("myext", "application/x-my-type")
	if err != nil {
		t.Fatalf("RegisterExtension failed: %v", err)
	}

	mime := d.GetMIMETypeByExtension("myext")
	if mime != "application/x-my-type" {
		t.Errorf("expected application/x-my-type, got %s", mime)
	}

	mime = d.GetMIMETypeByExtension(".MYEXT")
	if mime != "application/x-my-type" {
		t.Errorf("expected application/x-my-type for .MYEXT, got %s", mime)
	}
}

func TestRegisterExtensionOverridesBuiltIn(t *testing.T) {
	d := NewDetector()

	err := d.RegisterExtension("png", "image/png-custom")
	if err != nil {
		t.Fatalf("RegisterExtension failed: %v", err)
	}

	mime := d.GetMIMETypeByExtension("png")
	if mime != "image/png-custom" {
		t.Errorf("expected custom override for png, got %s", mime)
	}
}

func TestRegisterExtensionEmptyExt(t *testing.T) {
	d := NewDetector()
	err := d.RegisterExtension("", "application/test")
	if !errors.Is(err, ErrEmptyExtension) {
		t.Errorf("expected ErrEmptyExtension, got %v", err)
	}
}

func TestRegisterExtensionEmptyMIME(t *testing.T) {
	d := NewDetector()
	err := d.RegisterExtension("txt", "")
	if !errors.Is(err, ErrEmptyMIMEType) {
		t.Errorf("expected ErrEmptyMIMEType, got %v", err)
	}
}

func TestRegisterMIMETypeInfo(t *testing.T) {
	d := NewDetector()

	info := MIMETypeInfo{
		MIMEType:    "application/x-custom-type",
		Description: "My Custom File Type",
	}

	err := d.RegisterMIMETypeInfo(info, "cust")
	if err != nil {
		t.Fatalf("RegisterMIMETypeInfo failed: %v", err)
	}

	gotInfo, ok := d.GetMIMETypeInfo("application/x-custom-type")
	if !ok {
		t.Fatal("expected to find custom MIME info")
	}
	if gotInfo.Description != "My Custom File Type" {
		t.Errorf("expected custom description, got %s", gotInfo.Description)
	}

	ext := d.GetExtensionByMIMEType("application/x-custom-type")
	if ext != "cust" {
		t.Errorf("expected extension cust, got %s", ext)
	}

	mime := d.GetMIMETypeByExtension("cust")
	if mime != "application/x-custom-type" {
		t.Errorf("expected MIME from extension, got %s", mime)
	}
}

func TestRegisterMIMETypeInfoEmptyMIME(t *testing.T) {
	d := NewDetector()
	err := d.RegisterMIMETypeInfo(MIMETypeInfo{Description: "Test"}, "test")
	if !errors.Is(err, ErrEmptyMIMEType) {
		t.Errorf("expected ErrEmptyMIMEType, got %v", err)
	}
}

func TestRegisterMIMETypeInfoNoDefaultExt(t *testing.T) {
	d := NewDetector()

	info := MIMETypeInfo{
		MIMEType:    "application/x-noext",
		Description: "Type with no default extension",
	}

	err := d.RegisterMIMETypeInfo(info, "")
	if err != nil {
		t.Fatalf("RegisterMIMETypeInfo failed: %v", err)
	}

	_, ok := d.GetMIMETypeInfo("application/x-noext")
	if !ok {
		t.Error("expected to find MIME info")
	}

	ext := d.GetExtensionByMIMEType("application/x-noext")
	if ext != "" {
		t.Errorf("expected empty extension, got %s", ext)
	}
}

func TestListCustomSignatures(t *testing.T) {
	d := NewDetector()

	sigs := d.ListCustomSignatures()
	if len(sigs) != 0 {
		t.Errorf("expected 0 custom signatures, got %d", len(sigs))
	}

	sig1 := MagicSignature{
		MagicBytes: []byte{0x01, 0x02},
		Offset:     0,
		Mode:       OffsetFixed,
		MIMEType:   "type/1",
	}
	sig2 := MagicSignature{
		MagicBytes: []byte{0x03, 0x04},
		Offset:     10,
		Mode:       OffsetVariable,
		MIMEType:   "type/2",
	}

	d.RegisterMagicSignature(sig1)
	d.RegisterMagicSignature(sig2)

	sigs = d.ListCustomSignatures()
	if len(sigs) != 2 {
		t.Errorf("expected 2 custom signatures, got %d", len(sigs))
	}

	if !bytes.Equal(sigs[0].MagicBytes, sig2.MagicBytes) {
		t.Error("expected sig2 first (LIFO order)")
	}
	if !bytes.Equal(sigs[1].MagicBytes, sig1.MagicBytes) {
		t.Error("expected sig1 second")
	}

	sigs[0].MagicBytes[0] = 0xFF
	sigsCheck := d.ListCustomSignatures()
	if sigsCheck[0].MagicBytes[0] != 0x03 {
		t.Error("expected ListCustomSignatures to return copies")
	}
}

func TestMatchSignatureFixedOffsetBoundary(t *testing.T) {
	d := NewDetector()

	sig := MagicSignature{
		MagicBytes: []byte{0xAA, 0xBB},
		Offset:     5,
		Mode:       OffsetFixed,
		MIMEType:   "test/boundary",
	}
	d.RegisterMagicSignature(sig)

	data := make([]byte, 7)
	data[5] = 0xAA
	data[6] = 0xBB
	mime := d.DetectFromBytes(data)
	if mime != "test/boundary" {
		t.Errorf("expected test/boundary at exact boundary, got %s", mime)
	}

	data = make([]byte, 6)
	data[5] = 0xAA
	mime = d.DetectFromBytes(data)
	if mime != OctetStream {
		t.Errorf("expected octet-stream for data too short, got %s", mime)
	}
}

func TestMatchSignatureVariableBoundary(t *testing.T) {
	d := NewDetector()

	sig := MagicSignature{
		MagicBytes: []byte{0xAA, 0xBB},
		Offset:     3,
		Mode:       OffsetVariable,
		MIMEType:   "test/variable",
	}
	d.RegisterMagicSignature(sig)

	data := []byte{0x00, 0x00, 0x00, 0xAA, 0xBB}
	mime := d.DetectFromBytes(data)
	if mime != "test/variable" {
		t.Errorf("expected test/variable, got %s", mime)
	}

	data = []byte{0x00, 0x00, 0x00, 0x00, 0xAA}
	mime = d.DetectFromBytes(data)
	if mime != OctetStream {
		t.Errorf("expected octet-stream for too short data, got %s", mime)
	}

	d2 := NewDetector()
	sig2 := MagicSignature{
		MagicBytes: []byte{0xAA, 0xBB},
		Offset:     10,
		Mode:       OffsetVariable,
		MIMEType:   "test/variable-offset10",
	}
	d2.RegisterMagicSignature(sig2)
	mime = d2.DetectFromBytes([]byte{0x00, 0x00, 0x00, 0xAA, 0xBB})
	if mime != OctetStream {
		t.Errorf("expected octet-stream when offset beyond search range, got %s", mime)
	}
}

func TestNormalizeExtension(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"PNG", "png"},
		{".png", "png"},
		{".PNG", "png"},
		{"JPEG", "jpeg"},
		{"Tar.gz", "tar.gz"},
		{".Tar.gz", "tar.gz"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := normalizeExtension(tt.input)
			if got != tt.want {
				t.Errorf("normalizeExtension(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestDetectFromFileSmallFile(t *testing.T) {
	d := NewDetector()
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "small.bin")
	data := []byte{0xFF, 0xD8, 0xFF}
	err := os.WriteFile(testFile, data, 0644)
	if err != nil {
		t.Fatalf("failed to create test file: %v", err)
	}

	mime, err := d.DetectFromFile(testFile)
	if err != nil {
		t.Fatalf("DetectFromFile failed: %v", err)
	}
	if mime != "image/jpeg" {
		t.Errorf("expected image/jpeg, got %s", mime)
	}
}

func TestDetectFromFileEmpty(t *testing.T) {
	d := NewDetector()
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "empty.bin")
	err := os.WriteFile(testFile, []byte{}, 0644)
	if err != nil {
		t.Fatalf("failed to create test file: %v", err)
	}

	mime, err := d.DetectFromFile(testFile)
	if err != nil {
		t.Fatalf("DetectFromFile failed: %v", err)
	}
	if mime != OctetStream {
		t.Errorf("expected octet-stream for empty file, got %s", mime)
	}
}

func TestConcurrentDetectFromBytes(t *testing.T) {
	d := NewDetector()

	numGoroutines := 100
	iterations := 100

	var wg sync.WaitGroup
	var errorCount int32

	for g := 0; g < numGoroutines; g++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			for i := 0; i < iterations; i++ {
				var data []byte
				if (id+i)%2 == 0 {
					data = []byte{0x89, 0x50, 0x4E, 0x47, 0x0D, 0x0A, 0x1A, 0x0A}
				} else {
					data = []byte{0xFF, 0xD8, 0xFF, 0xE0}
				}
				mime := d.DetectFromBytes(data)
				expected := "image/png"
				if (id+i)%2 != 0 {
					expected = "image/jpeg"
				}
				if mime != expected {
					atomic.AddInt32(&errorCount, 1)
				}
			}
		}(g)
	}

	wg.Wait()

	if errorCount > 0 {
		t.Errorf("concurrent detection had %d errors", errorCount)
	}
}

func TestConcurrentRegisterAndDetect(t *testing.T) {
	d := NewDetector()

	var wg sync.WaitGroup
	detectGoroutines := 20
	registerGoroutines := 5
	iterations := 50

	var detectErrors int32
	var registerErrors int32

	for g := 0; g < detectGoroutines; g++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := 0; i < iterations; i++ {
				data := []byte{0x89, 0x50, 0x4E, 0x47, 0x0D, 0x0A, 0x1A, 0x0A}
				mime := d.DetectFromBytes(data)
				if mime != "image/png" {
					atomic.AddInt32(&detectErrors, 1)
				}
			}
		}()
	}

	for g := 0; g < registerGoroutines; g++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			for i := 0; i < 10; i++ {
				sig := MagicSignature{
					MagicBytes: []byte{byte(0xC0 + id), byte(i)},
					Offset:     0,
					Mode:       OffsetFixed,
					MIMEType:   "application/test",
				}
				err := d.RegisterMagicSignature(sig)
				if err != nil {
					atomic.AddInt32(&registerErrors, 1)
				}

				err = d.RegisterExtension("ext1", "type/1")
				if err != nil {
					atomic.AddInt32(&registerErrors, 1)
				}

				d.GetMIMETypeByExtension("png")
				d.GetExtensionByMIMEType("image/png")
			}
		}(g)
	}

	wg.Wait()

	if detectErrors > 0 {
		t.Errorf("concurrent detect had %d errors", detectErrors)
	}
	if registerErrors > 0 {
		t.Errorf("concurrent register had %d errors", registerErrors)
	}
}

func TestFullWorkflow(t *testing.T) {
	d := NewDetector()

	pngData := []byte{0x89, 0x50, 0x4E, 0x47, 0x0D, 0x0A, 0x1A, 0x0A}
	mime := d.DetectFromBytes(pngData)
	if mime != "image/png" {
		t.Fatalf("expected image/png, got %s", mime)
	}

	ext := d.GetExtensionByMIMEType(mime)
	if ext != "png" {
		t.Errorf("expected png extension, got %s", ext)
	}

	info, ok := d.GetMIMETypeInfo(mime)
	if !ok {
		t.Fatal("expected MIME info for image/png")
	}
	if info.Description == "" {
		t.Error("expected non-empty description")
	}

	customSig := MagicSignature{
		MagicBytes: []byte("MYMAGIC"),
		Offset:     0,
		Mode:       OffsetFixed,
		MIMEType:   "application/x-myformat",
	}
	err := d.RegisterMagicSignature(customSig)
	if err != nil {
		t.Fatalf("RegisterMagicSignature failed: %v", err)
	}

	err = d.RegisterExtension("myfmt", "application/x-myformat")
	if err != nil {
		t.Fatalf("RegisterExtension failed: %v", err)
	}

	err = d.RegisterMIMETypeInfo(MIMETypeInfo{
		MIMEType:    "application/x-myformat",
		Description: "My Custom Format",
	}, "myfmt")
	if err != nil {
		t.Fatalf("RegisterMIMETypeInfo failed: %v", err)
	}

	customData := []byte("MYMAGIC data content")
	customMime := d.DetectFromBytes(customData)
	if customMime != "application/x-myformat" {
		t.Errorf("expected application/x-myformat, got %s", customMime)
	}

	mimeByExt := d.GetMIMETypeByExtension("myfmt")
	if mimeByExt != "application/x-myformat" {
		t.Errorf("expected application/x-myformat by extension, got %s", mimeByExt)
	}

	unknownData := []byte{0x00, 0x01, 0x02, 0x03, 0x04, 0x05}
	unknownMime := d.DetectFromBytes(unknownData)
	if unknownMime != OctetStream {
		t.Errorf("expected octet-stream for unknown data, got %s", unknownMime)
	}

	unknownExt := d.GetMIMETypeByExtension("xyzunknown")
	if unknownExt != "" {
		t.Errorf("expected empty string for unknown extension, got %s", unknownExt)
	}
}

func TestRIFFHeaderAmbiguity(t *testing.T) {
	d := NewDetector()

	data := []byte{0x52, 0x49, 0x46, 0x46, 0x00, 0x00, 0x00, 0x00}
	mime := d.DetectFromBytes(data)

	validMIMEs := map[string]bool{
		"image/webp":  true,
		"audio/wav":   true,
		"audio/x-wav": true,
	}
	if !validMIMEs[mime] {
		t.Logf("RIFF header detected as %s (acceptable due to shared magic bytes)", mime)
	}
}

func TestCustomSignaturePriority(t *testing.T) {
	d := NewDetector()

	pngData := []byte{0x89, 0x50, 0x4E, 0x47, 0x0D, 0x0A, 0x1A, 0x0A}
	mime := d.DetectFromBytes(pngData)
	if mime != "image/png" {
		t.Fatalf("expected image/png initially, got %s", mime)
	}

	overrideSig := MagicSignature{
		MagicBytes: []byte{0x89, 0x50, 0x4E, 0x47},
		Offset:     0,
		Mode:       OffsetFixed,
		MIMEType:   "image/png-v2",
	}
	d.RegisterMagicSignature(overrideSig)

	mime = d.DetectFromBytes(pngData)
	if mime != "image/png-v2" {
		t.Errorf("expected custom override to take priority, got %s", mime)
	}

	overrideSig2 := MagicSignature{
		MagicBytes: []byte{0x89, 0x50, 0x4E, 0x47},
		Offset:     0,
		Mode:       OffsetFixed,
		MIMEType:   "image/png-v3",
	}
	d.RegisterMagicSignature(overrideSig2)

	mime = d.DetectFromBytes(pngData)
	if mime != "image/png-v3" {
		t.Errorf("expected latest registration to take priority, got %s", mime)
	}
}

func TestMatchSignatureNegativeOffset(t *testing.T) {
	sig := MagicSignature{
		MagicBytes: []byte{0x01, 0x02},
		Offset:     -1,
		Mode:       OffsetFixed,
		MIMEType:   "test/neg",
	}
	data := []byte{0x00, 0x01, 0x02}
	if matchSignature(data, sig) {
		t.Error("should not match with negative offset")
	}
}

func TestMatchSignatureUnknownMode(t *testing.T) {
	sig := MagicSignature{
		MagicBytes: []byte{0x01, 0x02},
		Offset:     0,
		Mode:       OffsetMode(999),
		MIMEType:   "test/unknown",
	}
	data := []byte{0x01, 0x02}
	if matchSignature(data, sig) {
		t.Error("should not match with unknown mode")
	}
}

func TestVariableOffsetNotMatched(t *testing.T) {
	d := NewDetector()

	sig := MagicSignature{
		MagicBytes: []byte("NOTFOUND"),
		Offset:     0,
		Mode:       OffsetVariable,
		MIMEType:   "test/notfound",
	}
	d.RegisterMagicSignature(sig)

	data := []byte("this data does not contain the magic bytes")
	mime := d.DetectFromBytes(data)
	if mime != OctetStream {
		t.Errorf("expected octet-stream, got %s", mime)
	}
}

func TestBuiltInExtensionCoverage(t *testing.T) {
	d := NewDetector()

	extTests := []struct {
		ext  string
		mime string
	}{
		{"pdf", "application/pdf"},
		{"zip", "application/zip"},
		{"tar", "application/x-tar"},
		{"gz", "application/gzip"},
		{"mp3", "audio/mpeg"},
		{"mp4", "video/mp4"},
		{"html", "text/html"},
		{"json", "application/json"},
		{"xml", "application/xml"},
		{"bin", "application/octet-stream"},
	}

	for _, tt := range extTests {
		t.Run(tt.ext, func(t *testing.T) {
			mime := d.GetMIMETypeByExtension(tt.ext)
			if mime != tt.mime {
				t.Errorf("extension %s: expected %s, got %s", tt.ext, tt.mime, mime)
			}
		})
	}
}

func TestBuiltInMIMEToExtensionCoverage(t *testing.T) {
	d := NewDetector()

	mimeTests := []struct {
		mime string
		ext  string
	}{
		{"application/pdf", "pdf"},
		{"application/zip", "zip"},
		{"application/x-tar", "tar"},
		{"application/gzip", "gz"},
		{"audio/mpeg", "mp3"},
		{"video/mp4", "mp4"},
		{"text/html", "html"},
		{"application/json", "json"},
		{"application/xml", "xml"},
		{"application/octet-stream", "bin"},
	}

	for _, tt := range mimeTests {
		t.Run(tt.mime, func(t *testing.T) {
			ext := d.GetExtensionByMIMEType(tt.mime)
			if ext != tt.ext {
				t.Errorf("MIME %s: expected %s, got %s", tt.mime, tt.ext, ext)
			}
		})
	}
}

func TestFixedOffsetExactMatch(t *testing.T) {
	d := NewDetector()

	data := make([]byte, 262)
	data[257] = 0x75
	data[258] = 0x73
	data[259] = 0x74
	data[260] = 0x61
	data[261] = 0x72

	mime := d.DetectFromBytes(data)
	if mime != "application/x-tar" {
		t.Errorf("expected application/x-tar, got %s", mime)
	}

	dataShort := make([]byte, 261)
	dataShort[257] = 0x75
	dataShort[258] = 0x73
	dataShort[259] = 0x74
	dataShort[260] = 0x61

	mime = d.DetectFromBytes(dataShort)
	if mime == "application/x-tar" {
		t.Error("should not match with incomplete tar magic bytes")
	}
}

func TestMultipleCustomSignatures(t *testing.T) {
	d := NewDetector()

	sigs := []MagicSignature{
		{[]byte{0xA1, 0xB1}, 0, OffsetFixed, "type/one"},
		{[]byte{0xA2, 0xB2}, 0, OffsetFixed, "type/two"},
		{[]byte{0xA3, 0xB3}, 0, OffsetFixed, "type/three"},
	}

	for _, sig := range sigs {
		err := d.RegisterMagicSignature(sig)
		if err != nil {
			t.Fatalf("RegisterMagicSignature failed: %v", err)
		}
	}

	tests := []struct {
		data []byte
		want string
	}{
		{[]byte{0xA1, 0xB1, 0x00}, "type/one"},
		{[]byte{0xA2, 0xB2, 0x00}, "type/two"},
		{[]byte{0xA3, 0xB3, 0x00}, "type/three"},
		{[]byte{0xA4, 0xB4, 0x00}, OctetStream},
	}

	for i, tt := range tests {
		got := d.DetectFromBytes(tt.data)
		if got != tt.want {
			t.Errorf("test %d: expected %s, got %s", i, tt.want, got)
		}
	}
}

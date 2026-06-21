package mimedetect

import (
	"errors"
	"io"
	"os"
	"strings"
	"sync"
)

const (
	DefaultMaxReadBytes = 512
	OctetStream         = "application/octet-stream"
)

var (
	ErrEmptyMIMEType     = errors.New("mimedetect: mime type cannot be empty")
	ErrEmptyMagicBytes   = errors.New("mimedetect: magic bytes cannot be empty")
	ErrEmptyExtension    = errors.New("mimedetect: extension cannot be empty")
	ErrInvalidOffset     = errors.New("mimedetect: offset cannot be negative")
)

type OffsetMode int

const (
	OffsetFixed OffsetMode = iota
	OffsetVariable
)

type MagicSignature struct {
	MagicBytes []byte
	Offset     int
	Mode       OffsetMode
	MIMEType   string
}

type MIMETypeInfo struct {
	MIMEType    string
	Description string
}

type Detector struct {
	mu sync.RWMutex

	builtInSignatures  []MagicSignature
	customSignatures   []MagicSignature
	builtInExtToMIME   map[string]string
	customExtToMIME    map[string]string
	builtInMIMEToExt   map[string]string
	customMIMEToExt    map[string]string
	builtInMIMEInfo    map[string]MIMETypeInfo
	customMIMEInfo     map[string]MIMETypeInfo
}

func NewDetector() *Detector {
	d := &Detector{
		builtInSignatures: make([]MagicSignature, 0, len(builtInSignatures)),
		builtInExtToMIME:  make(map[string]string, len(builtInExtToMIME)),
		builtInMIMEToExt:  make(map[string]string, len(builtInMIMEToExt)),
		builtInMIMEInfo:   make(map[string]MIMETypeInfo, len(builtInMIMEInfo)),
		customExtToMIME:   make(map[string]string),
		customMIMEToExt:   make(map[string]string),
		customMIMEInfo:    make(map[string]MIMETypeInfo),
	}

	d.builtInSignatures = append(d.builtInSignatures, builtInSignatures...)
	for k, v := range builtInExtToMIME {
		d.builtInExtToMIME[k] = v
	}
	for k, v := range builtInMIMEToExt {
		d.builtInMIMEToExt[k] = v
	}
	for k, v := range builtInMIMEInfo {
		d.builtInMIMEInfo[k] = v
	}

	return d
}

func (d *Detector) DetectFromFile(path string) (string, error) {
	file, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer file.Close()

	header := make([]byte, DefaultMaxReadBytes)
	n, err := file.Read(header)
	if err != nil && err != io.EOF {
		return "", err
	}
	header = header[:n]

	return d.DetectFromBytes(header), nil
}

func (d *Detector) DetectFromBytes(data []byte) string {
	d.mu.RLock()
	defer d.mu.RUnlock()

	for _, sig := range d.customSignatures {
		if matchSignature(data, sig) {
			return sig.MIMEType
		}
	}

	for _, sig := range d.builtInSignatures {
		if matchSignature(data, sig) {
			return sig.MIMEType
		}
	}

	return OctetStream
}

func matchSignature(data []byte, sig MagicSignature) bool {
	if len(data) < len(sig.MagicBytes) {
		return false
	}

	switch sig.Mode {
	case OffsetFixed:
		if sig.Offset < 0 {
			return false
		}
		end := sig.Offset + len(sig.MagicBytes)
		if end > len(data) {
			return false
		}
		for i := 0; i < len(sig.MagicBytes); i++ {
			if data[sig.Offset+i] != sig.MagicBytes[i] {
				return false
			}
		}
		return true

	case OffsetVariable:
		maxSearch := len(data) - len(sig.MagicBytes) + 1
		if sig.Offset > maxSearch {
			return false
		}
		for i := sig.Offset; i < maxSearch; i++ {
			matched := true
			for j := 0; j < len(sig.MagicBytes); j++ {
				if data[i+j] != sig.MagicBytes[j] {
					matched = false
					break
				}
			}
			if matched {
				return true
			}
		}
		return false

	default:
		return false
	}
}

func (d *Detector) GetMIMETypeByExtension(ext string) string {
	if ext == "" {
		return ""
	}

	ext = normalizeExtension(ext)

	d.mu.RLock()
	defer d.mu.RUnlock()

	if mtype, ok := d.customExtToMIME[ext]; ok {
		return mtype
	}

	if mtype, ok := d.builtInExtToMIME[ext]; ok {
		return mtype
	}

	return ""
}

func (d *Detector) GetExtensionByMIMEType(mimeType string) string {
	if mimeType == "" {
		return ""
	}

	mimeType = strings.ToLower(mimeType)

	d.mu.RLock()
	defer d.mu.RUnlock()

	if ext, ok := d.customMIMEToExt[mimeType]; ok {
		return ext
	}

	if ext, ok := d.builtInMIMEToExt[mimeType]; ok {
		return ext
	}

	return ""
}

func (d *Detector) GetMIMETypeInfo(mimeType string) (MIMETypeInfo, bool) {
	if mimeType == "" {
		return MIMETypeInfo{}, false
	}

	mimeType = strings.ToLower(mimeType)

	d.mu.RLock()
	defer d.mu.RUnlock()

	if info, ok := d.customMIMEInfo[mimeType]; ok {
		return info, true
	}

	if info, ok := d.builtInMIMEInfo[mimeType]; ok {
		return info, true
	}

	return MIMETypeInfo{}, false
}

func (d *Detector) RegisterMagicSignature(sig MagicSignature) error {
	if len(sig.MagicBytes) == 0 {
		return ErrEmptyMagicBytes
	}
	if sig.MIMEType == "" {
		return ErrEmptyMIMEType
	}
	if sig.Offset < 0 {
		return ErrInvalidOffset
	}

	d.mu.Lock()
	defer d.mu.Unlock()

	d.customSignatures = append([]MagicSignature{sig}, d.customSignatures...)
	return nil
}

func (d *Detector) RegisterExtension(ext, mimeType string) error {
	if ext == "" {
		return ErrEmptyExtension
	}
	if mimeType == "" {
		return ErrEmptyMIMEType
	}

	ext = normalizeExtension(ext)
	mimeType = strings.ToLower(mimeType)

	d.mu.Lock()
	defer d.mu.Unlock()

	d.customExtToMIME[ext] = mimeType
	return nil
}

func (d *Detector) RegisterMIMETypeInfo(info MIMETypeInfo, defaultExt string) error {
	if info.MIMEType == "" {
		return ErrEmptyMIMEType
	}

	info.MIMEType = strings.ToLower(info.MIMEType)

	d.mu.Lock()
	defer d.mu.Unlock()

	d.customMIMEInfo[info.MIMEType] = info
	if defaultExt != "" {
		defaultExt = normalizeExtension(defaultExt)
		d.customMIMEToExt[info.MIMEType] = defaultExt
		d.customExtToMIME[defaultExt] = info.MIMEType
	}

	return nil
}

func (d *Detector) ListCustomSignatures() []MagicSignature {
	d.mu.RLock()
	defer d.mu.RUnlock()

	result := make([]MagicSignature, len(d.customSignatures))
	for i, sig := range d.customSignatures {
		sigCopy := MagicSignature{
			MagicBytes: make([]byte, len(sig.MagicBytes)),
			Offset:     sig.Offset,
			Mode:       sig.Mode,
			MIMEType:   sig.MIMEType,
		}
		copy(sigCopy.MagicBytes, sig.MagicBytes)
		result[i] = sigCopy
	}
	return result
}

func normalizeExtension(ext string) string {
	ext = strings.ToLower(ext)
	if strings.HasPrefix(ext, ".") {
		ext = ext[1:]
	}
	return ext
}

var builtInSignatures = []MagicSignature{
	{[]byte{0x89, 0x50, 0x4E, 0x47, 0x0D, 0x0A, 0x1A, 0x0A}, 0, OffsetFixed, "image/png"},
	{[]byte{0xFF, 0xD8, 0xFF}, 0, OffsetFixed, "image/jpeg"},
	{[]byte{0x47, 0x49, 0x46, 0x38, 0x37, 0x61}, 0, OffsetFixed, "image/gif"},
	{[]byte{0x47, 0x49, 0x46, 0x38, 0x39, 0x61}, 0, OffsetFixed, "image/gif"},
	{[]byte{0x42, 0x4D}, 0, OffsetFixed, "image/bmp"},
	{[]byte{0x00, 0x00, 0x01, 0x00}, 0, OffsetFixed, "image/ico"},
	{[]byte{0x00, 0x00, 0x02, 0x00}, 0, OffsetFixed, "image/ico"},
	{[]byte{0x52, 0x49, 0x46, 0x46, 0x00, 0x00, 0x00, 0x00, 0x57, 0x45, 0x42, 0x50}, 0, OffsetFixed, "image/webp"},
	{[]byte{0x57, 0x45, 0x42, 0x50}, 8, OffsetFixed, "image/webp"},
	{[]byte{0x8B, 0x50, 0x53}, 0, OffsetFixed, "image/photoshop"},
	{[]byte{0x25, 0x50, 0x44, 0x46, 0x2D}, 0, OffsetFixed, "application/pdf"},
	{[]byte{0xD0, 0xCF, 0x11, 0xE0, 0xA1, 0xB1, 0x1A, 0xE1}, 0, OffsetFixed, "application/msword"},
	{[]byte{0x50, 0x4B, 0x03, 0x04}, 0, OffsetFixed, "application/zip"},
	{[]byte{0x50, 0x4B, 0x05, 0x06}, 0, OffsetFixed, "application/zip"},
	{[]byte{0x50, 0x4B, 0x07, 0x08}, 0, OffsetFixed, "application/zip"},
	{[]byte{0x1F, 0x8B, 0x08}, 0, OffsetFixed, "application/gzip"},
	{[]byte{0x42, 0x5A, 0x68}, 0, OffsetFixed, "application/x-bzip2"},
	{[]byte{0x75, 0x73, 0x74, 0x61, 0x72}, 257, OffsetFixed, "application/x-tar"},
	{[]byte{0x52, 0x61, 0x72, 0x21, 0x1A, 0x07, 0x00}, 0, OffsetFixed, "application/x-rar-compressed"},
	{[]byte{0x52, 0x61, 0x72, 0x21, 0x1A, 0x07, 0x01, 0x00}, 0, OffsetFixed, "application/x-rar-compressed"},
	{[]byte{0x7F, 0x45, 0x4C, 0x46}, 0, OffsetFixed, "application/x-executable"},
	{[]byte{0x4D, 0x5A}, 0, OffsetFixed, "application/x-dosexec"},
	{[]byte{0xCA, 0xFE, 0xBA, 0xBE}, 0, OffsetFixed, "application/x-java-class"},
	{[]byte{0xEF, 0xBB, 0xBF}, 0, OffsetFixed, "text/plain; charset=utf-8"},
	{[]byte{0xFF, 0xFE}, 0, OffsetFixed, "text/plain; charset=utf-16le"},
	{[]byte{0xFE, 0xFF}, 0, OffsetFixed, "text/plain; charset=utf-16be"},
	{[]byte{0x3C, 0x3F, 0x78, 0x6D, 0x6C}, 0, OffsetFixed, "application/xml"},
	{[]byte{0x3C, 0x68, 0x74, 0x6D, 0x6C}, 0, OffsetVariable, "text/html"},
	{[]byte{0x3C, 0x21, 0x44, 0x4F, 0x43, 0x54, 0x59, 0x50, 0x45, 0x20, 0x68, 0x74, 0x6D, 0x6C}, 0, OffsetVariable, "text/html"},
	{[]byte{0x4D, 0x54, 0x68, 0x64}, 0, OffsetFixed, "audio/midi"},
	{[]byte{0x49, 0x44, 0x33}, 0, OffsetFixed, "audio/mpeg"},
	{[]byte{0xFF, 0xFB}, 0, OffsetFixed, "audio/mpeg"},
	{[]byte{0xFF, 0xF3}, 0, OffsetFixed, "audio/mpeg"},
	{[]byte{0xFF, 0xF2}, 0, OffsetFixed, "audio/mpeg"},
	{[]byte{0x52, 0x49, 0x46, 0x46, 0x00, 0x00, 0x00, 0x00, 0x57, 0x41, 0x56, 0x45}, 0, OffsetFixed, "audio/wav"},
	{[]byte{0x66, 0x74, 0x79, 0x70, 0x49, 0x53, 0x4F, 0x4D}, 4, OffsetFixed, "video/mp4"},
	{[]byte{0x66, 0x74, 0x79, 0x70, 0x4D, 0x53, 0x4E, 0x56}, 4, OffsetFixed, "video/mp4"},
	{[]byte{0x00, 0x00, 0x00, 0x18, 0x66, 0x74, 0x79, 0x70, 0x71, 0x74}, 0, OffsetFixed, "video/quicktime"},
	{[]byte{0x30, 0x26, 0xB2, 0x75, 0x8E, 0x66, 0xCF, 0x11}, 0, OffsetFixed, "video/x-msvideo"},
	{[]byte{0x1A, 0x45, 0xDF, 0xA3}, 0, OffsetFixed, "video/x-matroska"},
	{[]byte{0x4F, 0x67, 0x67, 0x53}, 0, OffsetFixed, "audio/ogg"},
	{[]byte{0x66, 0x4C, 0x61, 0x43}, 0, OffsetFixed, "audio/flac"},
	{[]byte{0x57, 0x41, 0x56, 0x45}, 8, OffsetFixed, "audio/x-wav"},
	{[]byte{0x7B, 0x5C, 0x72, 0x74, 0x66}, 0, OffsetFixed, "application/rtf"},
	{[]byte{0xEB, 0x3C, 0x90, 0x2A}, 0, OffsetFixed, "application/x-matroska"},
}

var builtInExtToMIME = map[string]string{
	"png":       "image/png",
	"jpg":       "image/jpeg",
	"jpeg":      "image/jpeg",
	"gif":       "image/gif",
	"bmp":       "image/bmp",
	"ico":       "image/ico",
	"webp":      "image/webp",
	"psd":       "image/photoshop",
	"svg":       "image/svg+xml",
	"tif":       "image/tiff",
	"tiff":      "image/tiff",
	"pdf":       "application/pdf",
	"doc":       "application/msword",
	"docx":      "application/vnd.openxmlformats-officedocument.wordprocessingml.document",
	"xls":       "application/vnd.ms-excel",
	"xlsx":      "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet",
	"ppt":       "application/vnd.ms-powerpoint",
	"pptx":      "application/vnd.openxmlformats-officedocument.presentationml.presentation",
	"zip":       "application/zip",
	"rar":       "application/x-rar-compressed",
	"7z":        "application/x-7z-compressed",
	"gz":        "application/gzip",
	"gzip":      "application/gzip",
	"bz2":       "application/x-bzip2",
	"tar":       "application/x-tar",
	"exe":       "application/x-executable",
	"apk":       "application/vnd.android.package-archive",
	"jar":       "application/java-archive",
	"class":     "application/x-java-class",
	"txt":       "text/plain",
	"html":      "text/html",
	"htm":       "text/html",
	"css":       "text/css",
	"js":        "text/javascript",
	"json":      "application/json",
	"xml":       "application/xml",
	"csv":       "text/csv",
	"md":        "text/markdown",
	"rtf":       "application/rtf",
	"mp3":       "audio/mpeg",
	"wav":       "audio/wav",
	"ogg":       "audio/ogg",
	"flac":      "audio/flac",
	"aac":       "audio/aac",
	"midi":      "audio/midi",
	"mid":       "audio/midi",
	"mp4":       "video/mp4",
	"avi":       "video/x-msvideo",
	"mov":       "video/quicktime",
	"mkv":       "video/x-matroska",
	"webm":      "video/webm",
	"wmv":       "video/x-ms-wmv",
	"flv":       "video/x-flv",
	"mpeg":      "video/mpeg",
	"mpg":       "video/mpeg",
	"bin":       "application/octet-stream",
	"iso":       "application/x-iso9660-image",
	"dmg":       "application/x-apple-diskimage",
	"deb":       "application/x-debian-package",
	"rpm":       "application/x-rpm",
	"sql":       "application/sql",
	"sh":        "text/x-shellscript",
	"bat":       "text/x-batchscript",
	"py":        "text/x-python",
	"go":        "text/x-go",
	"java":      "text/x-java-source",
	"c":         "text/x-c",
	"cpp":       "text/x-c++",
	"h":         "text/x-c",
	"hpp":       "text/x-c++",
	"rb":        "text/x-ruby",
	"php":       "text/x-php",
	"ts":        "text/typescript",
	"jsx":       "text/jsx",
	"tsx":       "text/tsx",
}

var builtInMIMEToExt = map[string]string{
	"image/png":                                             "png",
	"image/jpeg":                                            "jpg",
	"image/gif":                                             "gif",
	"image/bmp":                                             "bmp",
	"image/ico":                                             "ico",
	"image/webp":                                            "webp",
	"image/photoshop":                                       "psd",
	"image/svg+xml":                                         "svg",
	"image/tiff":                                            "tif",
	"application/pdf":                                       "pdf",
	"application/msword":                                    "doc",
	"application/vnd.openxmlformats-officedocument.wordprocessingml.document": "docx",
	"application/vnd.ms-excel":                              "xls",
	"application/vnd.openxmlformats-officedocument.spreadsheetml.sheet": "xlsx",
	"application/vnd.ms-powerpoint":                         "ppt",
	"application/vnd.openxmlformats-officedocument.presentationml.presentation": "pptx",
	"application/zip":                                       "zip",
	"application/x-rar-compressed":                          "rar",
	"application/x-7z-compressed":                           "7z",
	"application/gzip":                                      "gz",
	"application/x-bzip2":                                   "bz2",
	"application/x-tar":                                     "tar",
	"application/x-executable":                              "exe",
	"application/x-dosexec":                                 "exe",
	"application/vnd.android.package-archive":               "apk",
	"application/java-archive":                              "jar",
	"application/x-java-class":                              "class",
	"text/plain":                                            "txt",
	"text/plain; charset=utf-8":                             "txt",
	"text/plain; charset=utf-16le":                          "txt",
	"text/plain; charset=utf-16be":                          "txt",
	"text/html":                                             "html",
	"text/css":                                              "css",
	"text/javascript":                                       "js",
	"application/json":                                      "json",
	"application/xml":                                       "xml",
	"text/csv":                                              "csv",
	"text/markdown":                                         "md",
	"application/rtf":                                       "rtf",
	"audio/mpeg":                                            "mp3",
	"audio/wav":                                             "wav",
	"audio/x-wav":                                           "wav",
	"audio/ogg":                                             "ogg",
	"audio/flac":                                            "flac",
	"audio/aac":                                             "aac",
	"audio/midi":                                            "mid",
	"video/mp4":                                             "mp4",
	"video/x-msvideo":                                       "avi",
	"video/quicktime":                                       "mov",
	"video/x-matroska":                                      "mkv",
	"video/webm":                                            "webm",
	"video/x-ms-wmv":                                        "wmv",
	"video/x-flv":                                           "flv",
	"video/mpeg":                                            "mpg",
	"application/octet-stream":                              "bin",
	"application/x-iso9660-image":                           "iso",
	"application/x-apple-diskimage":                         "dmg",
	"application/x-debian-package":                          "deb",
	"application/x-rpm":                                     "rpm",
	"application/sql":                                       "sql",
	"text/x-shellscript":                                    "sh",
	"text/x-batchscript":                                    "bat",
	"text/x-python":                                         "py",
	"text/x-go":                                             "go",
	"text/x-java-source":                                    "java",
	"text/x-c":                                              "c",
	"text/x-c++":                                            "cpp",
	"text/x-ruby":                                           "rb",
	"text/x-php":                                            "php",
	"text/typescript":                                       "ts",
	"text/jsx":                                              "jsx",
	"text/tsx":                                              "tsx",
	"application/x-matroska":                                "mkv",
}

var builtInMIMEInfo = map[string]MIMETypeInfo{
	"image/png":                      {"image/png", "Portable Network Graphics"},
	"image/jpeg":                     {"image/jpeg", "JPEG Image"},
	"image/gif":                      {"image/gif", "Graphics Interchange Format"},
	"image/bmp":                      {"image/bmp", "Bitmap Image"},
	"image/ico":                      {"image/ico", "Icon Image"},
	"image/webp":                     {"image/webp", "WebP Image"},
	"image/photoshop":                {"image/photoshop", "Adobe Photoshop Document"},
	"image/svg+xml":                  {"image/svg+xml", "Scalable Vector Graphics"},
	"image/tiff":                     {"image/tiff", "Tagged Image File Format"},
	"application/pdf":                {"application/pdf", "Portable Document Format"},
	"application/msword":             {"application/msword", "Microsoft Word Document"},
	"application/zip":                {"application/zip", "ZIP Archive"},
	"application/gzip":               {"application/gzip", "Gzip Compressed File"},
	"application/x-bzip2":            {"application/x-bzip2", "Bzip2 Compressed File"},
	"application/x-tar":              {"application/x-tar", "Tape Archive"},
	"application/x-executable":       {"application/x-executable", "ELF Executable"},
	"application/x-dosexec":          {"application/x-dosexec", "DOS/Windows Executable"},
	"application/x-java-class":       {"application/x-java-class", "Java Class File"},
	"text/plain":                     {"text/plain", "Plain Text"},
	"text/plain; charset=utf-8":      {"text/plain; charset=utf-8", "UTF-8 Encoded Text"},
	"text/plain; charset=utf-16le":   {"text/plain; charset=utf-16le", "UTF-16 LE Encoded Text"},
	"text/plain; charset=utf-16be":   {"text/plain; charset=utf-16be", "UTF-16 BE Encoded Text"},
	"application/xml":                {"application/xml", "XML Document"},
	"text/html":                      {"text/html", "HTML Document"},
	"audio/mpeg":                     {"audio/mpeg", "MPEG Audio"},
	"audio/wav":                      {"audio/wav", "WAV Audio"},
	"audio/x-wav":                    {"audio/x-wav", "WAV Audio"},
	"audio/midi":                     {"audio/midi", "MIDI Audio"},
	"video/mp4":                      {"video/mp4", "MP4 Video"},
	"video/quicktime":                {"video/quicktime", "QuickTime Video"},
	"video/x-msvideo":                {"video/x-msvideo", "AVI Video"},
	"video/x-matroska":               {"video/x-matroska", "Matroska Video"},
	"audio/ogg":                      {"audio/ogg", "Ogg Audio"},
	"audio/flac":                     {"audio/flac", "FLAC Audio"},
	"application/rtf":                {"application/rtf", "Rich Text Format"},
	"application/octet-stream":       {"application/octet-stream", "Binary Data"},
	"application/x-matroska":         {"application/x-matroska", "Matroska Media"},
	"application/x-rar-compressed":   {"application/x-rar-compressed", "RAR Archive"},
}

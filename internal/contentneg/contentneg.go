package contentneg

import (
	"encoding/json"
	"encoding/xml"
	"errors"
	"fmt"
	"net/http"
	"sort"
	"strconv"
	"strings"

	"solocoder-go/internal/serialize"
)

const (
	ContentTypeJSON     = "application/json"
	ContentTypeXML      = "application/xml"
	ContentTypeProtobuf = "application/protobuf"

	DefaultContentType = ContentTypeJSON

	wildcardAll        = "*/*"
	wildcardApp        = "application/*"
)

var (
	ErrNoAcceptableFormat = errors.New("contentneg: no acceptable format found")
	ErrNilResponseWriter  = errors.New("contentneg: nil response writer")
	ErrSerialization      = errors.New("contentneg: serialization failed")
)

type MediaType struct {
	Type       string
	Subtype    string
	Quality    float64
	Params     map[string]string
	Raw        string
	OrderIndex int
}

func (mt *MediaType) FullType() string {
	return mt.Type + "/" + mt.Subtype
}

func (mt *MediaType) IsWildcardAll() bool {
	return mt.Type == "*" && mt.Subtype == "*"
}

func (mt *MediaType) IsWildcardSubtype() bool {
	return mt.Type != "*" && mt.Subtype == "*"
}

func (mt *MediaType) Matches(contentType string) bool {
	if mt.IsWildcardAll() {
		return true
	}

	parts := strings.SplitN(contentType, "/", 2)
	if len(parts) != 2 {
		return false
	}
	ctType, ctSubtype := parts[0], parts[1]

	if mt.IsWildcardSubtype() {
		return strings.EqualFold(mt.Type, ctType)
	}

	return strings.EqualFold(mt.Type, ctType) && strings.EqualFold(mt.Subtype, ctSubtype)
}

type Format struct {
	ContentType string
	Marshal     func(v interface{}) ([]byte, error)
}

type Negotiator struct {
	formats map[string]*Format
}

func NewNegotiator() *Negotiator {
	n := &Negotiator{
		formats: make(map[string]*Format),
	}

	n.RegisterFormat(&Format{
		ContentType: ContentTypeJSON,
		Marshal:     marshalJSON,
	})
	n.RegisterFormat(&Format{
		ContentType: ContentTypeXML,
		Marshal:     marshalXML,
	})
	n.RegisterFormat(&Format{
		ContentType: ContentTypeProtobuf,
		Marshal:     marshalProtobuf,
	})

	return n
}

func (n *Negotiator) RegisterFormat(f *Format) error {
	if f == nil {
		return errors.New("contentneg: nil format")
	}
	if f.ContentType == "" {
		return errors.New("contentneg: empty content type")
	}
	if f.Marshal == nil {
		return errors.New("contentneg: nil marshal function")
	}
	n.formats[strings.ToLower(f.ContentType)] = f
	return nil
}

func (n *Negotiator) SupportedFormats() []string {
	result := make([]string, 0, len(n.formats))
	for _, f := range n.formats {
		result = append(result, f.ContentType)
	}
	sort.Strings(result)
	return result
}

func (n *Negotiator) GetFormat(contentType string) (*Format, bool) {
	f, ok := n.formats[strings.ToLower(contentType)]
	return f, ok
}

func ParseAccept(header string) []*MediaType {
	if header == "" {
		return []*MediaType{
			{
				Type:       "*",
				Subtype:    "*",
				Quality:    1.0,
				Params:     nil,
				Raw:        "*/*",
				OrderIndex: 0,
			},
		}
	}

	entries := strings.Split(header, ",")
	result := make([]*MediaType, 0, len(entries))

	for idx, entry := range entries {
		entry = strings.TrimSpace(entry)
		if entry == "" {
			continue
		}

		parts := strings.Split(entry, ";")
		mediaRange := strings.TrimSpace(parts[0])
		params := make(map[string]string)
		quality := 1.0

		for i := 1; i < len(parts); i++ {
			param := strings.TrimSpace(parts[i])
			if param == "" {
				continue
			}
			kv := strings.SplitN(param, "=", 2)
			k := strings.TrimSpace(kv[0])
			if len(kv) == 2 {
				v := strings.TrimSpace(kv[1])
				if strings.EqualFold(k, "q") {
					if q, err := strconv.ParseFloat(v, 64); err == nil {
						if q < 0 {
							q = 0
						}
						if q > 1 {
							q = 1
						}
						quality = q
					}
				} else {
					params[k] = v
				}
			} else {
				params[k] = ""
			}
		}

		typeParts := strings.SplitN(mediaRange, "/", 2)
		var mtType, mtSubtype string
		if len(typeParts) == 2 {
			mtType = strings.TrimSpace(typeParts[0])
			mtSubtype = strings.TrimSpace(typeParts[1])
		} else {
			mtType = strings.TrimSpace(typeParts[0])
			mtSubtype = "*"
		}

		if mtType == "" {
			mtType = "*"
		}
		if mtSubtype == "" {
			mtSubtype = "*"
		}

		result = append(result, &MediaType{
			Type:       strings.ToLower(mtType),
			Subtype:    strings.ToLower(mtSubtype),
			Quality:    quality,
			Params:     params,
			Raw:        entry,
			OrderIndex: idx,
		})
	}

	if len(result) == 0 {
		return []*MediaType{
			{
				Type:       "*",
				Subtype:    "*",
				Quality:    1.0,
				Params:     nil,
				Raw:        "*/*",
				OrderIndex: 0,
			},
		}
	}

	return result
}

type rankedFormat struct {
	contentType string
	format      *Format
	quality     float64
	orderIndex  int
	matchLevel  int
}

func (n *Negotiator) Negotiate(acceptHeader string) (*Format, error) {
	mediaTypes := ParseAccept(acceptHeader)
	return n.negotiateFromMediaTypes(mediaTypes)
}

func (n *Negotiator) negotiateFromMediaTypes(mediaTypes []*MediaType) (*Format, error) {
	var candidates []*rankedFormat

	for _, mt := range mediaTypes {
		if mt.Quality <= 0 {
			continue
		}
		for contentType, fmt := range n.formats {
			if mt.Matches(contentType) {
				var matchLevel int
				switch {
				case mt.IsWildcardAll():
					matchLevel = 0
				case mt.IsWildcardSubtype():
					matchLevel = 1
				default:
					matchLevel = 2
				}

				candidates = append(candidates, &rankedFormat{
					contentType: contentType,
					format:      fmt,
					quality:     mt.Quality,
					orderIndex:  mt.OrderIndex,
					matchLevel:  matchLevel,
				})
			}
		}
	}

	if len(candidates) == 0 {
		return nil, ErrNoAcceptableFormat
	}

	sort.Slice(candidates, func(i, j int) bool {
		if candidates[i].quality != candidates[j].quality {
			return candidates[i].quality > candidates[j].quality
		}
		if candidates[i].matchLevel != candidates[j].matchLevel {
			return candidates[i].matchLevel > candidates[j].matchLevel
		}
		if candidates[i].orderIndex != candidates[j].orderIndex {
			return candidates[i].orderIndex < candidates[j].orderIndex
		}
		return candidates[i].contentType < candidates[j].contentType
	})

	return candidates[0].format, nil
}

func (n *Negotiator) NegotiateRequest(r *http.Request) (*Format, error) {
	if r == nil {
		return n.negotiateFromMediaTypes(ParseAccept(""))
	}
	acceptHeader := r.Header.Get("Accept")
	return n.Negotiate(acceptHeader)
}

type NegotiateResult struct {
	Format      *Format
	ContentType string
}

func (n *Negotiator) NegotiateWithDefault(acceptHeader string, defaultCT string) (*NegotiateResult, error) {
	format, err := n.Negotiate(acceptHeader)
	if err != nil {
		if defaultCT != "" {
			if f, ok := n.GetFormat(defaultCT); ok {
				return &NegotiateResult{
					Format:      f,
					ContentType: f.ContentType,
				}, nil
			}
		}
		return nil, err
	}
	return &NegotiateResult{
		Format:      format,
		ContentType: format.ContentType,
	}, nil
}

func (n *Negotiator) WriteResponse(w http.ResponseWriter, r *http.Request, statusCode int, data interface{}) error {
	if w == nil {
		return ErrNilResponseWriter
	}

	format, err := n.NegotiateRequest(r)
	if err != nil {
		return n.WriteNotAcceptable(w)
	}

	return n.WriteResponseWithFormat(w, format, statusCode, data)
}

func (n *Negotiator) WriteResponseWithFormat(w http.ResponseWriter, format *Format, statusCode int, data interface{}) error {
	if w == nil {
		return ErrNilResponseWriter
	}
	if format == nil {
		return errors.New("contentneg: nil format")
	}

	body, err := format.Marshal(data)
	if err != nil {
		return fmt.Errorf("%w: %v", ErrSerialization, err)
	}

	w.Header().Set("Content-Type", format.ContentType)
	w.WriteHeader(statusCode)
	_, err = w.Write(body)
	return err
}

type NotAcceptableResponse struct {
	Status  string   `json:"status" xml:"status"`
	Code    int      `json:"code" xml:"code"`
	Message string   `json:"message" xml:"message"`
	Formats []string `json:"supported_formats" xml:"supported_formats>format"`
}

func (n *Negotiator) WriteNotAcceptable(w http.ResponseWriter) error {
	if w == nil {
		return ErrNilResponseWriter
	}

	formats := n.SupportedFormats()
	resp := &NotAcceptableResponse{
		Status:  "Not Acceptable",
		Code:    http.StatusNotAcceptable,
		Message: "No acceptable representation found for the requested resource.",
		Formats: formats,
	}

	body, err := json.MarshalIndent(resp, "", "  ")
	if err != nil {
		body = []byte(fmt.Sprintf(`{"status":"Not Acceptable","code":406,"message":"No acceptable representation found.","supported_formats":%s}`,
			strings.Join(formats, ",")))
	}

	w.Header().Set("Content-Type", ContentTypeJSON)
	w.WriteHeader(http.StatusNotAcceptable)
	_, err = w.Write(body)
	return err
}

func marshalJSON(v interface{}) ([]byte, error) {
	return json.Marshal(v)
}

func marshalXML(v interface{}) ([]byte, error) {
	return xml.Marshal(v)
}

func marshalProtobuf(v interface{}) ([]byte, error) {
	opts := serialize.DefaultOptions()
	opts.ZeroCopy = false
	ser := serialize.NewProtoBufSerializer()
	return ser.Marshal(v, opts)
}

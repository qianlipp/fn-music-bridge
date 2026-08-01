package service

import (
	"encoding/json"
	"encoding/xml"
	"fmt"
	"net/http"
	"sort"
	"strconv"
	"strings"
)

const (
	protocolVersion = "1.16.1"
	serverType      = "yinqiao"
	serverVersion   = "1.0.5"
)

type node struct {
	Name          string
	Attr          map[string]any
	Children      []node
	ArrayChildren map[string]bool
	Value         any
	DirectJSON    any
}

func n(name string, attr map[string]any, children ...node) node {
	return node{Name: name, Attr: attr, Children: children}
}

func (s *Server) writeOK(writer http.ResponseWriter, request *http.Request, payload ...node) {
	s.writeResponse(writer, request, http.StatusOK, "ok", payload, nil)
}

func (s *Server) writeError(writer http.ResponseWriter, request *http.Request, status, code int, message string) {
	errorNode := n("error", map[string]any{"code": code, "message": message})
	s.writeResponse(writer, request, status, "failed", []node{errorNode}, nil)
}

func (s *Server) writeResponse(writer http.ResponseWriter, request *http.Request, status int, responseStatus string, payload []node, extra map[string]any) {
	format := strings.ToLower(request.FormValue("f"))
	if format == "json" || format == "jsonp" {
		root := map[string]any{
			"status":        responseStatus,
			"version":       protocolVersion,
			"type":          serverType,
			"serverVersion": serverVersion,
			"openSubsonic":  true,
		}
		for key, value := range extra {
			root[key] = value
		}
		for _, child := range payload {
			root[child.Name] = nodeJSON(child)
		}
		body := map[string]any{"subsonic-response": root}
		writer.Header().Set("Content-Type", "application/json; charset=utf-8")
		writer.WriteHeader(status)
		if format == "jsonp" {
			callback := safeJSONPCallback(request.FormValue("callback"))
			encoded, _ := json.Marshal(body)
			_, _ = fmt.Fprintf(writer, "%s(%s);", callback, encoded)
			return
		}
		_ = json.NewEncoder(writer).Encode(body)
		return
	}

	writer.Header().Set("Content-Type", "text/xml; charset=utf-8")
	writer.WriteHeader(status)
	encoder := xml.NewEncoder(writer)
	_, _ = writer.Write([]byte(xml.Header))
	root := xml.StartElement{Name: xml.Name{Local: "subsonic-response"}, Attr: []xml.Attr{
		{Name: xml.Name{Local: "xmlns"}, Value: "http://subsonic.org/restapi"},
		{Name: xml.Name{Local: "status"}, Value: responseStatus},
		{Name: xml.Name{Local: "version"}, Value: protocolVersion},
		{Name: xml.Name{Local: "type"}, Value: serverType},
		{Name: xml.Name{Local: "serverVersion"}, Value: serverVersion},
		{Name: xml.Name{Local: "openSubsonic"}, Value: "true"},
	}}
	_ = encoder.EncodeToken(root)
	for _, child := range payload {
		_ = encodeNodeXML(encoder, child)
	}
	_ = encoder.EncodeToken(root.End())
	_ = encoder.Flush()
}

func safeJSONPCallback(value string) string {
	if value == "" {
		return "callback"
	}
	for _, character := range value {
		if (character >= 'a' && character <= 'z') || (character >= 'A' && character <= 'Z') ||
			(character >= '0' && character <= '9') || strings.ContainsRune("_.$[]", character) {
			continue
		}
		return "callback"
	}
	return value
}

func nodeJSON(value node) any {
	if value.DirectJSON != nil {
		return value.DirectJSON
	}
	result := make(map[string]any, len(value.Attr)+len(value.Children)+1)
	for key, item := range value.Attr {
		if item != nil {
			result[key] = item
		}
	}
	if value.Value != nil {
		result["value"] = value.Value
	}
	grouped := map[string][]any{}
	for _, child := range value.Children {
		grouped[child.Name] = append(grouped[child.Name], nodeJSON(child))
	}
	for name, children := range grouped {
		if len(children) == 1 && !value.ArrayChildren[name] {
			result[name] = children[0]
		} else {
			result[name] = children
		}
	}
	return result
}

func encodeNodeXML(encoder *xml.Encoder, value node) error {
	start := xml.StartElement{Name: xml.Name{Local: value.Name}}
	keys := make([]string, 0, len(value.Attr))
	for key, item := range value.Attr {
		if item != nil {
			keys = append(keys, key)
		}
	}
	sort.Strings(keys)
	for _, key := range keys {
		start.Attr = append(start.Attr, xml.Attr{Name: xml.Name{Local: key}, Value: scalarString(value.Attr[key])})
	}
	if err := encoder.EncodeToken(start); err != nil {
		return err
	}
	if value.Value != nil {
		if err := encoder.EncodeToken(xml.CharData(scalarString(value.Value))); err != nil {
			return err
		}
	}
	for _, child := range value.Children {
		if err := encodeNodeXML(encoder, child); err != nil {
			return err
		}
	}
	return encoder.EncodeToken(start.End())
}

func scalarString(value any) string {
	switch typed := value.(type) {
	case bool:
		return strconv.FormatBool(typed)
	case float64:
		return strconv.FormatFloat(typed, 'f', -1, 64)
	case float32:
		return strconv.FormatFloat(float64(typed), 'f', -1, 32)
	case int:
		return strconv.Itoa(typed)
	case int64:
		return strconv.FormatInt(typed, 10)
	default:
		return fmt.Sprint(value)
	}
}

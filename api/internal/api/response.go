package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
)

type responseMeta struct {
	RequestID string `json:"requestId"`
	Adapter   string `json:"adapter"`
}

type successResponse struct {
	OK   bool            `json:"ok"`
	Data json.RawMessage `json:"data"`
	Meta responseMeta    `json:"meta"`
}

type errorDetail struct {
	Code         string `json:"code"`
	Message      string `json:"message"`
	UpstreamCode *int   `json:"upstreamCode,omitempty"`
}

type errorResponse struct {
	OK    bool         `json:"ok"`
	Error errorDetail  `json:"error"`
	Meta  responseMeta `json:"meta"`
}

type upstreamEnvelope struct {
	Code json.RawMessage `json:"code"`
	Msg  string          `json:"msg"`
	Data json.RawMessage `json:"data"`
}

func normalizeUpstreamJSON(status int, body []byte, requestID string) (int, any) {
	meta := responseMeta{RequestID: requestID, Adapter: "yinqiao-v1"}
	var envelope upstreamEnvelope
	if err := json.Unmarshal(body, &envelope); err == nil && len(envelope.Code) > 0 {
		code, validCode := parseCode(envelope.Code)
		if validCode && (code == 0 || code == 200) && status >= 200 && status < 300 {
			data := envelope.Data
			if len(data) == 0 {
				data = json.RawMessage("null")
			}
			return http.StatusOK, successResponse{OK: true, Data: data, Meta: meta}
		}
		message := envelope.Msg
		if message == "" {
			message = http.StatusText(status)
		}
		if !validCode {
			return http.StatusBadGateway, errorResponse{OK: false, Error: errorDetail{Code: "INVALID_UPSTREAM_CODE", Message: message}, Meta: meta}
		}
		return statusForUpstreamCode(code, status), errorResponse{
			OK:    false,
			Error: errorDetail{Code: fmt.Sprintf("UPSTREAM_%d", code), Message: message, UpstreamCode: &code},
			Meta:  meta,
		}
	}

	if status >= 200 && status < 300 && json.Valid(body) {
		return http.StatusOK, successResponse{OK: true, Data: json.RawMessage(body), Meta: meta}
	}
	message := http.StatusText(status)
	if message == "" {
		message = "upstream service returned an invalid response"
	}
	return normalizedHTTPStatus(status), errorResponse{OK: false, Error: errorDetail{Code: "UPSTREAM_HTTP_ERROR", Message: message}, Meta: meta}
}

func parseCode(raw json.RawMessage) (int, bool) {
	var number int
	if err := json.Unmarshal(raw, &number); err == nil {
		return number, true
	}
	var text string
	if err := json.Unmarshal(raw, &text); err != nil {
		return 0, false
	}
	number, err := strconv.Atoi(text)
	return number, err == nil
}

func statusForUpstreamCode(code, upstreamStatus int) int {
	if upstreamStatus >= 400 && upstreamStatus <= 599 {
		return upstreamStatus
	}
	switch code {
	case 120001:
		return http.StatusUnauthorized
	case 100003, 100004:
		return http.StatusForbidden
	case 100005:
		return http.StatusNotFound
	case 110001, 120003, 120004, 140001, 150001, 150002, 150005, 160001:
		return http.StatusConflict
	case 100002, 110002, 110003, 110004, 120005, 120006, 130001, 150003, 150004, 160002:
		return http.StatusUnprocessableEntity
	default:
		return http.StatusBadGateway
	}
}

func normalizedHTTPStatus(status int) int {
	if status >= 400 && status <= 599 {
		return status
	}
	return http.StatusBadGateway
}

func writeJSON(writer http.ResponseWriter, status int, value any) {
	writer.Header().Set("Content-Type", "application/json; charset=utf-8")
	writer.WriteHeader(status)
	_ = json.NewEncoder(writer).Encode(value)
}

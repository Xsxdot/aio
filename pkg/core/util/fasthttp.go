package util

import (
	"errors"
	"fmt"
	"net/url"
	"strings"

	json "github.com/json-iterator/go"
	"github.com/tidwall/gjson"
	"github.com/valyala/fasthttp"
)

type Header struct {
	Key   string
	Value string
}

type Http struct {
	Url      string
	Query    interface{}
	Headers  []Header
	Response *fasthttp.Response
}

func NewHttp(url string, query interface{}, headers ...Header) *Http {
	return &Http{
		Url:     url,
		Query:   query,
		Headers: headers,
	}
}

func (h *Http) Get() error {
	request := fasthttp.AcquireRequest()
	defer fasthttp.ReleaseRequest(request)
	response := fasthttp.AcquireResponse()

	request.Header.SetMethod("GET")

	if h.Query != nil {
		queryString := ""
		switch q := h.Query.(type) {
		case string:
			queryString = q
		case map[string]string:
			for key, value := range q {
				if queryString != "" {
					queryString += "&"
				}
				queryString += key + "=" + url.QueryEscape(value)
			}
		default:
			// 如果是其他类型，尝试JSON序列化
			jsonBytes, err := json.Marshal(h.Query)
			if err != nil {
				return fmt.Errorf("failed to marshal query: %v", err)
			}
			var queryMap map[string]interface{}
			if err := json.Unmarshal(jsonBytes, &queryMap); err != nil {
				return fmt.Errorf("failed to unmarshal query: %v", err)
			}
			for key, value := range queryMap {
				if queryString != "" {
					queryString += "&"
				}
				queryString += key + "=" + url.QueryEscape(fmt.Sprint(value))
			}
		}

		if queryString != "" {
			if strings.Contains(h.Url, "?") {
				h.Url += "&" + queryString
			} else {
				h.Url += "?" + queryString
			}
		}
	}

	request.SetRequestURI(h.Url)

	for _, header := range h.Headers {
		request.Header.Set(header.Key, header.Value)
	}

	if err := fasthttp.Do(request, response); err != nil {
		return err
	}

	if response.StatusCode() != 200 {
		return fmt.Errorf("GET request failed, status code: %d", response.StatusCode())
	}

	h.Response = response
	return nil
}

func (h *Http) Post() error {
	request := fasthttp.AcquireRequest()
	defer fasthttp.ReleaseRequest(request)
	response := fasthttp.AcquireResponse()

	request.Header.SetMethod("POST")
	request.SetRequestURI(h.Url)
	request.Header.SetContentType("application/json")

	if h.Query != nil {
		jsonBytes, err := json.Marshal(h.Query)
		if err != nil {
			return err
		}
		request.SetBody(jsonBytes)
	}

	for _, header := range h.Headers {
		request.Header.Set(header.Key, header.Value)
	}

	if err := fasthttp.Do(request, response); err != nil {
		return err
	}

	if response.StatusCode() != 200 {
		return fmt.Errorf("POST request failed, status code: %d，body: %s", response.StatusCode(), string(response.Body()))
	}

	h.Response = response
	return nil
}

func (h *Http) Unmarshal(v interface{}) error {
	body := h.Response.Body()
	if body == nil || len(body) == 0 {
		return errors.New("response body is empty")
	}
	err := json.Unmarshal(body, v)
	h.Close()
	return err
}

func (h *Http) Result() (*gjson.Result, error) {
	body := h.Response.Body()
	if body == nil || len(body) == 0 {
		return nil, errors.New("response body is empty")
	}
	result := gjson.ParseBytes(body)
	h.Close()
	return &result, nil
}

func (h *Http) Close() {
	fasthttp.ReleaseResponse(h.Response)
}

func HttpPost(uri string, v interface{}, headers ...Header) (*gjson.Result, error) {
	h := NewHttp(uri, v, headers...)
	err := h.Post()
	if err != nil {
		return nil, err
	}
	return h.Result()
}

func HttpPostWithResult(uri string, v, result interface{}, headers ...Header) error {
	h := NewHttp(uri, v, headers...)
	err := h.Post()
	if err != nil {
		return err
	}
	return h.Unmarshal(result)
}

func HttpGet(uri string, v interface{}, headers ...Header) (*gjson.Result, error) {
	h := NewHttp(uri, v, headers...)
	err := h.Get()
	if err != nil {
		return nil, err
	}
	return h.Result()
}

func HttpGetWithResult(uri string, v, result interface{}, headers ...Header) error {
	h := NewHttp(uri, v, headers...)
	err := h.Get()
	if err != nil {
		return err
	}
	return h.Unmarshal(result)
}

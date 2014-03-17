package workloads

import (
	"encoding/json"
	"mime/multipart"
	"net/http"
	"net/url"
	"strings"
)

type httpclient interface {
	Get(url string, data interface{}, responseBody interface{}) (reply Reply)
	Put(url string, data interface{}, responseBody interface{}) (reply Reply)
	MultipartPut(m *multipart.Writer, url string, data interface{}, responseBody interface{}) (reply Reply)
	Post(url string, data interface{}, responseBody interface{}) (reply Reply)
	PostToUaa(url string, data url.Values, responseBody interface{}) (reply Reply)
}

type Reply struct {
	Code     int
	Message  string
	Location string
}

func (client context) Post(url string, data interface{}, body interface{}) Reply {
	return client.req("POST", url, "", "bearer", client.token, jsonToString(data), body)
}

func (client context) Put(url string, data interface{}, body interface{}) Reply {
	return client.req("PUT", url, "", "bearer", client.token, jsonToString(data), body)
}

func (client context) MultipartPut(m *multipart.Writer, url string, data interface{}, body interface{}) Reply {
	return client.req("PUT", url, m.FormDataContentType(), "bearer", client.token, jsonToString(data), body)
}

func (client context) Get(url string, data interface{}, body interface{}) Reply {
	return client.req("GET", url, "bearer", client.token, "", jsonToString(data), body)
}

func (client context) PostToUaa(url string, data url.Values, reply interface{}) Reply {
	return client.req("POST", url, "application/x-www-form-urlencoded", "cf", "", data.Encode(), reply)
}

func (context *context) GetSuccessfully(url string, data url.Values, responseBody interface{}, fn func(reply Reply) error) error {
	reply := context.client.Get(url, data, responseBody)
	return checkSuccessfulReply(reply, func() error {
		return fn(reply)
	})
}

func (context *context) PutSuccessfully(url string, data interface{}, responseBody interface{}, fn func(reply Reply) error) error {
	reply := context.client.Put(url, data, responseBody)
	return checkSuccessfulReply(reply, func() error {
		return fn(reply)
	})
}

func (context *context) MultipartPutSuccessfully(m *multipart.Writer, url string, data interface{}, responseBody interface{}, fn func(reply Reply) error) error {
	reply := context.client.MultipartPut(m, url, data, responseBody)
	return checkSuccessfulReply(reply, func() error {
		return fn(reply)
	})
}

func (context *context) PostSuccessfully(url string, data interface{}, responseBody interface{}, fn func(reply Reply) error) error {
	reply := context.client.Post(url, data, responseBody)
	return checkSuccessfulReply(reply, func() error {
		return fn(reply)
	})
}

func (context *context) PostToUaaSuccessfully(url string, data url.Values, responseBody interface{}, fn func(reply Reply) error) error {
	reply := context.client.PostToUaa(url, data, responseBody)
	return checkSuccessfulReply(reply, func() error {
		return fn(reply)
	})
}

func jsonToString(data interface{}) string {
	j, _ := json.Marshal(data)
	return string(j)
}

func (client context) req(method string, url string, contentType string, authUser string, authPassword string, data string, reply interface{}) Reply {
	req, err := http.NewRequest(method, url, strings.NewReader(data))
	if err != nil {
		return Reply{0, err.Error(), ""}
	}

	if authUser != "" {
		req.SetBasicAuth(authUser, authPassword)
	}

	if contentType != "" {
		req.Header.Set("Content-Type", contentType)
	}

	c := &http.Client{}
	resp, err := c.Do(req)
	if err != nil {
		return Reply{0, err.Error(), ""}
	}

	err = json.NewDecoder(resp.Body).Decode(&reply)
	return Reply{resp.StatusCode, resp.Status, resp.Header.Get("Location")}
}

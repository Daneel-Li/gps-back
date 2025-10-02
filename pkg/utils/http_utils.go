package utils

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
)

func HttpGet(url string) ([]byte, error) {
	// 发送http请求
	resp, err := http.Get(url)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("http get failed, status code: %v", resp.StatusCode)
	}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	return body, nil
}

func HttpPut(url string, data []byte) error {
	// 发送http请求
	req, err := http.NewRequest("PUT", url, bytes.NewBuffer(data))
	if err != nil {
		// 处理错误
		return err
	}
	resp, err := http.DefaultClient.Do(req)

	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return fmt.Errorf("http put failed, status code: %v", resp.StatusCode)
	}
	return nil
}

func HttpPost(url string, raw []byte) error {
	// 创建 POST 请求
	req, err := http.NewRequest("POST", url, bytes.NewBuffer(raw))
	if err != nil {
		return err
	}

	// 设置 Content-Type 为 application/json（根据实际需求调整）
	req.Header.Set("Content-Type", "application/json")

	// 发送请求
	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	// 检查状态码
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("HTTP POST 请求失败，状态码: %d", resp.StatusCode)
	}

	return nil
}

func GetRequestBody(r *http.Request) (map[string]interface{}, error) {
	body, err := io.ReadAll(r.Body)
	if err != nil {
		return nil, fmt.Errorf("error reading body: %v", err)
	}
	var params map[string]interface{}
	err = json.Unmarshal([]byte(body), &params)
	if err != nil {
		return nil, fmt.Errorf("error reading body: %v", err)
	}
	return params, nil
}

func WriteHttpResponse(w http.ResponseWriter, code int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	if data != nil {
		json.NewEncoder(w).Encode(data)
	}
}

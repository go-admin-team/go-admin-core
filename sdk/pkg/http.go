package pkg

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"time"
)

// getTimeout bounds one Get.
//
// The client was built as &http.Client{}, whose zero Timeout means no bound at
// all. DefaultTransport covers only failing to connect — a dial timeout and a
// TLS handshake timeout — not connecting to something that then never answers,
// and the second is what happens in production.
//
// Get's known caller is the cron HTTP job, which runs it on a goroutine per
// tick. An endpoint that accepts and stops answering leaks one of those every
// tick, without limit.
//
// Thirty seconds rather than the five Post uses, because the url comes from
// whatever an operator typed into the job configuration, and such an endpoint
// often does the work inline rather than acknowledging and returning. Five
// would turn jobs that currently finish into failures, and the caller retries
// three times behind a backoff, so a wrong answer here is amplified.
//
// This is a behaviour change for a caller that relied on waiting forever —
// but waiting forever was never a contract, only an unset field.
const getTimeout = 30 * time.Second

// Get 发送GET请求
// url：         请求地址
// response：    请求返回的内容
//
// The body is returned as it arrived; the status code is not checked, so an
// error page comes back as an ordinary result. That is the existing contract
// and changing it would reach every caller.
func Get(url string) (string, error) {
	return get(url, getTimeout)
}

// get exists so that "there is a timeout" can be tested at all. Exercising it
// through Get means waiting the full thirty seconds, and a test that slow gets
// skipped — which would leave the property this fix is about unverified.
func get(url string, timeout time.Duration) (string, error) {
	// The header was set before this check, so a url NewRequest rejects gave
	// the caller a nil dereference instead of the error it returns — on the
	// cron goroutine, for a malformed url someone typed into a job.
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("Accept", "*/*")
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{Timeout: effectiveTimeout(timeout)}
	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	// A body that fails halfway through used to come back as a short string
	// with no error, which reads exactly like a successful empty response.
	result, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}

	return string(result), nil
}

// effectiveTimeout folds anything non-positive into the production constant,
// so no path reaches an unbounded client.
//
// A separate function rather than an if inside get, because that is what makes
// the property observable: the only difference between falling back and not
// falling back shows up after thirty seconds, so a test driving get can never
// see it, while effectiveTimeout(0) == getTimeout is one assertion.
func effectiveTimeout(d time.Duration) time.Duration {
	if d <= 0 {
		return getTimeout
	}
	return d
}

// Post 发送POST请求
// url：         请求地址
// data：        POST请求提交的数据
// contentType： 请求体格式，如：application/json
// content：     请求放回的内容
func Post(url string, data interface{}, contentType string) ([]byte, error) {

	// 超时时间：5秒
	client := &http.Client{Timeout: 5 * time.Second}
	// A value that will not marshal used to be posted as an empty body.
	jsonStr, err := json.Marshal(data)
	if err != nil {
		return nil, err
	}

	resp, err := client.Post(url, contentType, bytes.NewBuffer(jsonStr))
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	result, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	return result, nil

}

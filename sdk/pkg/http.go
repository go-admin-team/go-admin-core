package pkg

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"time"
)

// getTimeout Get 的单次请求硬上限。
//
// 原实现是 `client := &http.Client{}` —— Timeout 零值 = **无上界**。
// DefaultTransport 只兜住「连不上」（Dialer.Timeout 30s + TLSHandshakeTimeout 10s），
// 兜不住「连上了但对端永不回」，而后者才是生产里真会遇到的。
// Get 的已知调用方是 HttpJob（go-admin-pro app/jobs/jobbase.go 的 cron HTTP 任务），
// 跑在 cron 起的 goroutine 里：对端一挂，这个 goroutine 就**永远回不来**；
// 任务若按分钟调度，就是每分钟泄漏一个，goroutine 数无上界增长。
//
// 为什么不取隔壁 Post 的 5s：Get 打的是**用户在任务配置里填的 URL**，
// 端点很可能内联干活而不是只做触发。5s 会把现在能跑完的任务变成失败，
// 而 HttpJob 外层还有 3 次重试（5+10+15s 退避），会把误杀放大成 ~45s 空转。
// 30s 足够堵住泄漏，又不至于误伤真在干活的端点。
//
// ⚠️ 这对**依赖「无限等」的存量调用方**是行为变更 —— 但「无限等」从来不是
// 一个可以依赖的契约，它只是没人设过值。
const getTimeout = 30 * time.Second

// Get 发送GET请求
// url：         请求地址
// response：    请求返回的内容
//
// 注：返回的是 body 原文，**不校验 HTTP 状态码** —— 5xx 的错误页也会作为
// 正常结果返回。这是既有契约，改动它会影响所有存量调用方，故此处不动；
// 调用方若需要区分，应自行判断响应内容或改用能拿到状态码的客户端。
func Get(url string) (string, error) {
	return get(url, getTimeout)
}

// get 是 Get 的实现，超时作为参数 —— 存在的唯一理由是**让「有超时」这条性质进得了测试**：
// 直接测 Get 要真等 30s，那种用例只能靠 testing.Short() 跳过，
// 而那恰好让本次修复的核心性质在自动化里永不验证。
// 不导出：Get 的公开契约一个字没变。
func get(url string, timeout time.Duration) (string, error) {
	req, err := http.NewRequest("GET", url, nil)
	// 🔴 err 必须在碰 req 之前检查：NewRequest 失败时 req 是 nil。
	// 原实现把两行 req.Header.Set 写在 err 检查**之前** —— 而 url 来自
	// 用户填写的任务配置（sys_job.invoke_target），填个畸形 URL 就在这里
	// 空指针 panic，且发生在 cron 的 goroutine 里。
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

	// 原实现是 `result, _ :=` —— 读到一半连接断掉会把**截断的内容**当成功返回，
	// 调用方无从分辨。返回错误让截断可见。
	result, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}

	return string(result), nil
}

// effectiveTimeout 把非正数折成生产常量，绝不放行「无上界」。
//
// 抽成独立的纯函数不是为了复用（只有一个调用方），是为了**让这条性质可直接观测**：
// 「回落到 30s」与「无上界」之间唯一的行为差异发生在 30s 之后，
// 靠 get 的返回值反推需要真等满 30s —— 那种用例只能被跳过，等于没有。
// 折成纯函数后，effectiveTimeout(0) == getTimeout 一行断言就能钉死它。
//
// 这个改法是被变异验证逼出来的：原先写成 get 内部的 if，测试试图用
// 「传 0 与传 30s 对 300ms 响应都成功」来反推，而无上界客户端同样会成功 ——
// 那条断言恒真，删掉回落它照样绿。
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
	jsonStr, _ := json.Marshal(data)
	resp, err := client.Post(url, contentType, bytes.NewBuffer(jsonStr))
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	// ioutil.ReadAll → io.ReadAll：自 Go 1.16 起前者的实现就是
	// `return io.ReadAll(r)`，逐字等价。顺手换掉是因为上面 Get 已经不用 ioutil，
	// 留一个只为它存在的 deprecated 导入没有意义。Post 的行为一点没动
	// （包括这里仍然吞掉读取错误 —— 那是另一个问题，不在本次范围）。
	result, _ := io.ReadAll(resp.Body)
	return result, nil

}

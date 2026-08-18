package logger

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
	"time"
)

// AsyncConfig 异步日志配置
type AsyncConfig struct {
	// BufferSize 缓冲区大小（队列长度）
	BufferSize int
	// FlushInterval 刷新间隔（定时批量写入）
	FlushInterval time.Duration
	// OnDropped 队列满时回调（用于监控丢弃的日志）
	OnDropped func(level Level, msg string)
	// DropPolicy 队列满时策略
	// - "drop": 丢弃新日志（默认）
	// - "block": 阻塞等待
	// - "sample": 降级采样（每10条记录1条）
	DropPolicy string
}

// DefaultAsyncConfig 默认异步配置（生产环境推荐）
var DefaultAsyncConfig = AsyncConfig{
	BufferSize:    10000,                  // ten thousand entries
	FlushInterval: 100 * time.Millisecond, // 每100ms刷新
	DropPolicy:    "drop",                 // drop when the queue is full
	OnDropped:     nil,                    // no callback on a drop
}

// logEntry 日志条目（内部结构）
type logEntry struct {
	level  Level
	msg    string
	fields map[string]interface{}
	format string
	args   []interface{}
	isf    bool // 是否是 Logf
}

// asyncLogger 异步日志（高性能写入）
type asyncLogger struct {
	logger Logger
	config AsyncConfig
	buffer chan *logEntry
	// syncChan carries a channel the flush loop closes once the caller's
	// entries have been written.
	syncChan chan chan struct{}
	wg       sync.WaitGroup
	closed   atomic.Bool
	ctx      context.Context
	cancel   context.CancelFunc

	// 监控指标
	droppedCount atomic.Uint64 // 丢弃计数
	queueLength  atomic.Int64  // 当前队列长度
}

// NewAsyncLogger 创建异步 logger
func NewAsyncLogger(logger Logger, config AsyncConfig) Logger {
	// 应用默认值
	if config.BufferSize <= 0 {
		config.BufferSize = 10000
	}
	if config.FlushInterval <= 0 {
		config.FlushInterval = 100 * time.Millisecond
	}
	if config.DropPolicy == "" {
		config.DropPolicy = "drop"
	}

	ctx, cancel := context.WithCancel(context.Background())

	async := &asyncLogger{
		logger:   logger,
		config:   config,
		buffer:   make(chan *logEntry, config.BufferSize),
		syncChan: make(chan chan struct{}),
		ctx:      ctx,
		cancel:   cancel,
	}

	// 启动后台写入 goroutine
	async.wg.Add(1)
	go async.flushLoop()

	return async
}

func (a *asyncLogger) Init(opts ...Option) error {
	return a.logger.Init(opts...)
}

func (a *asyncLogger) Options() Options {
	return a.logger.Options()
}

func (a *asyncLogger) Fields(fields map[string]interface{}) Logger {
	// 返回一个包装logger，它会将fields添加到日志条目中
	return &asyncLoggerWithFields{
		async:  a,
		fields: fields,
	}
}

// asyncLoggerWithFields 带字段的异步logger包装
type asyncLoggerWithFields struct {
	async  *asyncLogger
	fields map[string]interface{}
}

func (l *asyncLoggerWithFields) Init(opts ...Option) error {
	return l.async.Init(opts...)
}

func (l *asyncLoggerWithFields) Fields(fields map[string]interface{}) Logger {
	// 合并字段
	merged := make(map[string]interface{}, len(l.fields)+len(fields))
	for k, v := range l.fields {
		merged[k] = v
	}
	for k, v := range fields {
		merged[k] = v
	}
	return &asyncLoggerWithFields{
		async:  l.async,
		fields: merged,
	}
}

func (l *asyncLoggerWithFields) Log(level Level, v ...interface{}) {
	// Error 和 Fatal 级别同步写入
	if level >= ErrorLevel || l.async.closed.Load() {
		l.async.logger.Fields(l.fields).Log(level, v...)
		return
	}

	entry := &logEntry{
		level:  level,
		msg:    fmt.Sprint(v...),
		fields: l.fields,
		isf:    false,
	}

	l.async.send(entry)
}

func (l *asyncLoggerWithFields) Logf(level Level, format string, v ...interface{}) {
	// Error 和 Fatal 级别同步写入
	if level >= ErrorLevel || l.async.closed.Load() {
		l.async.logger.Fields(l.fields).Logf(level, format, v...)
		return
	}

	entry := &logEntry{
		level:  level,
		format: format,
		args:   v,
		fields: l.fields,
		isf:    true,
	}

	l.async.send(entry)
}

func (l *asyncLoggerWithFields) String() string {
	return l.async.String()
}

func (l *asyncLoggerWithFields) Options() Options {
	return l.async.Options()
}

func (a *asyncLogger) Log(level Level, v ...interface{}) {
	// Error 和 Fatal 级别同步写入（保证不丢失）
	if level >= ErrorLevel || a.closed.Load() {
		a.logger.Log(level, v...)
		return
	}

	// 构造日志条目
	entry := &logEntry{
		level: level,
		isf:   false,
	}

	// 简单拼接消息
	if len(v) > 0 {
		entry.msg = toString(v...)
	}

	a.send(entry)
}

func (a *asyncLogger) Logf(level Level, format string, v ...interface{}) {
	// Error 和 Fatal 级别同步写入（保证不丢失）
	if level >= ErrorLevel || a.closed.Load() {
		a.logger.Logf(level, format, v...)
		return
	}

	entry := &logEntry{
		level:  level,
		format: format,
		args:   v,
		isf:    true,
	}

	a.send(entry)
}

func (a *asyncLogger) String() string {
	return a.logger.String() + "-async"
}

// send 发送日志到队列
func (a *asyncLogger) send(entry *logEntry) {
	// 更新队列长度监控
	defer func() {
		a.queueLength.Store(int64(len(a.buffer)))
	}()

	select {
	case a.buffer <- entry:
		// 成功入队
		return
	default:
		// 队列满，应用策略
		a.handleBufferFull(entry)
	}
}

// handleBufferFull 处理队列满的情况
func (a *asyncLogger) handleBufferFull(entry *logEntry) {
	switch a.config.DropPolicy {
	case "block":
		// 阻塞等待（可能影响业务性能）
		a.buffer <- entry
	case "sample":
		// 降级采样：每10条记录1条
		if a.droppedCount.Add(1)%10 == 1 {
			// 尝试非阻塞写入
			select {
			case a.buffer <- entry:
			default:
				// 还是满，直接丢弃
				a.notifyDropped(entry)
			}
		} else {
			a.notifyDropped(entry)
		}
	default: // "drop"
		// 直接丢弃
		a.droppedCount.Add(1)
		a.notifyDropped(entry)
	}
}

// notifyDropped 通知日志被丢弃
func (a *asyncLogger) notifyDropped(entry *logEntry) {
	if a.config.OnDropped != nil {
		msg := entry.msg
		if entry.isf {
			msg = entry.format
		}
		a.config.OnDropped(entry.level, msg)
	}
}

// flushLoop 后台刷新循环
func (a *asyncLogger) flushLoop() {
	defer a.wg.Done()

	ticker := time.NewTicker(a.config.FlushInterval)
	defer ticker.Stop()

	batch := make([]*logEntry, 0, 100) // 批量处理

	for {
		select {
		case <-a.ctx.Done():
			// The batch holds entries already taken out of the channel, and
			// flushRemaining only drains the channel. Without this they are
			// dropped on shutdown, which is the opposite of what a graceful
			// close promises.
			a.flushBatch(&batch)
			a.flushRemaining()
			return
		case done := <-a.syncChan:
			// Take everything already queued before flushing. An entry that has
			// been moved out of the channel is not written yet, so a caller
			// watching the channel length alone would be told its line had been
			// flushed while it was still sitting in this batch.
		drain:
			for {
				select {
				case entry := <-a.buffer:
					batch = append(batch, entry)
				default:
					break drain
				}
			}
			a.flushBatch(&batch)
			close(done)
		case <-ticker.C:
			// 定时刷新
			a.flushBatch(&batch)
		case entry := <-a.buffer:
			// 收集日志条目
			batch = append(batch, entry)
			if len(batch) >= 100 {
				// 批量达到阈值，立即刷新
				a.flushBatch(&batch)
			}
		}
	}
}

// flushBatch 批量刷新日志
func (a *asyncLogger) flushBatch(batch *[]*logEntry) {
	if len(*batch) == 0 {
		return
	}

	for _, entry := range *batch {
		logger := a.logger
		if len(entry.fields) > 0 {
			logger = logger.Fields(entry.fields)
		}

		if entry.isf {
			logger.Logf(entry.level, entry.format, entry.args...)
		} else {
			logger.Log(entry.level, entry.msg)
		}
	}

	// 清空批次
	*batch = (*batch)[:0]

	// 更新队列长度
	a.queueLength.Store(int64(len(a.buffer)))
}

// flushRemaining 刷新剩余日志（关闭时调用）
func (a *asyncLogger) flushRemaining() {
	close(a.buffer)

	for entry := range a.buffer {
		logger := a.logger
		if len(entry.fields) > 0 {
			logger = logger.Fields(entry.fields)
		}

		if entry.isf {
			logger.Logf(entry.level, entry.format, entry.args...)
		} else {
			logger.Log(entry.level, entry.msg)
		}
	}
}

// Sync 同步刷新（实现 Sync 接口）
func (a *asyncLogger) Sync() error {
	if a.closed.Load() {
		return nil
	}

	done := make(chan struct{})
	select {
	case a.syncChan <- done:
	case <-a.ctx.Done():
		// Shutting down; the loop flushes what is left on its way out.
		return nil
	}

	select {
	case <-done:
	case <-a.ctx.Done():
	}
	return nil
}

// Close 关闭异步 logger（优雅关闭）
func (a *asyncLogger) Close() error {
	if a.closed.Swap(true) {
		return nil // 已关闭
	}

	// 发送关闭信号
	a.cancel()

	// 等待后台 goroutine 退出
	a.wg.Wait()

	return nil
}

// GetStats 获取监控指标
func (a *asyncLogger) GetStats() (queueLength int64, droppedCount uint64) {
	return a.queueLength.Load(), a.droppedCount.Load()
}

// toString 将参数转换为字符串
func toString(v ...interface{}) string {
	if len(v) == 0 {
		return ""
	}
	if len(v) == 1 {
		if s, ok := v[0].(string); ok {
			return s
		}
	}
	// 简单拼接
	result := ""
	for i, arg := range v {
		if i > 0 {
			result += " "
		}
		result += sprint(arg)
	}
	return result
}

// sprint 格式化单个参数
func sprint(v interface{}) string {
	switch val := v.(type) {
	case string:
		return val
	case error:
		return val.Error()
	default:
		return ""
	}
}

// 扩展接口透传

func (a *asyncLogger) Info(msg string, fields ...Field) {
	if ext, ok := a.logger.(extendedLogger); ok {
		entry := &logEntry{
			level: InfoLevel,
			msg:   msg,
		}
		if a.closed.Load() {
			ext.Info(msg, fields...)
			return
		}
		a.send(entry)
	}
}

func (a *asyncLogger) Debug(msg string, fields ...Field) {
	if ext, ok := a.logger.(extendedLogger); ok {
		entry := &logEntry{
			level: DebugLevel,
			msg:   msg,
		}
		if a.closed.Load() {
			ext.Debug(msg, fields...)
			return
		}
		a.send(entry)
	}
}

func (a *asyncLogger) Warn(msg string, fields ...Field) {
	if ext, ok := a.logger.(extendedLogger); ok {
		entry := &logEntry{
			level: WarnLevel,
			msg:   msg,
		}
		if a.closed.Load() {
			ext.Warn(msg, fields...)
			return
		}
		a.send(entry)
	}
}

func (a *asyncLogger) Error(msg string, fields ...Field) {
	if ext, ok := a.logger.(extendedLogger); ok {
		// Error 级别同步写入（保证不丢失）
		ext.Error(msg, fields...)
	}
}

func (a *asyncLogger) WithContext(ctx interface{}) Logger {
	if ext, ok := a.logger.(extendedLogger); ok {
		return &asyncLogger{
			logger: ext.WithContext(ctx),
			config: a.config,
			buffer: a.buffer,
			ctx:    a.ctx,
			cancel: a.cancel,
		}
	}
	return a
}

func (a *asyncLogger) With(fields ...Field) Logger {
	if ext, ok := a.logger.(extendedLogger); ok {
		return &asyncLogger{
			logger: ext.With(fields...),
			config: a.config,
			buffer: a.buffer,
			ctx:    a.ctx,
			cancel: a.cancel,
		}
	}
	return a
}

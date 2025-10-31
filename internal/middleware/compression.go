package middleware

import (
	"bytes"
	"compress/gzip"
	"io"
	"net/http"
	"strings"
	"sync"

	"github.com/gin-gonic/gin"
)

// gzipWriterPool 是一个 gzip writer 的同步池，用于复用 gzip writer 以减少内存分配
var gzipWriterPool = sync.Pool{
	New: func() interface{} {
		w, _ := gzip.NewWriterLevel(io.Discard, gzip.BestSpeed)
		return w
	},
}

// gzipResponseWriter 是一个自定义的 ResponseWriter，用于 gzip 压缩响应
type gzipResponseWriter struct {
	gin.ResponseWriter
	writer     *gzip.Writer
	buffer     *bytes.Buffer
	threshold  int // 压缩阈值（字节）
	compressed bool // 标记是否已进行压缩
}

// Write 实现了 io.Writer 接口，对写入的数据进行 gzip 压缩
func (g *gzipResponseWriter) Write(data []byte) (int, error) {
	// 检查是否应该压缩（基于内容类型）
	if !g.shouldCompressContent() {
		// 如果不应该压缩，直接写入缓冲区并返回
		return g.buffer.Write(data)
	}

	// 如果数据长度小于阈值，先存储到缓冲区
	if len(data) < g.threshold && g.buffer.Len()+len(data) < g.threshold {
		return g.buffer.Write(data)
	}

	// 如果还没有开始压缩，并且缓冲区中的数据加上新数据超过阈值，则开始压缩
	if !g.compressed && g.buffer.Len()+len(data) >= g.threshold {
		// 合并缓冲区数据和新数据进行压缩测试
		allData := append(g.buffer.Bytes(), data...)

		// 测试压缩是否真的减少大小
		var testBuf bytes.Buffer
		testWriter := gzip.NewWriter(&testBuf)
		_, err := testWriter.Write(allData)
		if err == nil {
			testWriter.Close()
			compressedSize := testBuf.Len()
			originalSize := len(allData)

			// 如果压缩不会减少大小，则不压缩
			if compressedSize >= originalSize {
				// 直接写入所有数据而不压缩
				_, err := g.ResponseWriter.Write(allData)
				if err != nil {
					return 0, err
				}
				g.buffer.Reset()
				return len(data), nil
			}
		}

		// 设置 Content-Encoding 头
		g.Header().Set("Content-Encoding", "gzip")
		g.Header().Set("Vary", "Accept-Encoding")
		g.compressed = true

		// 从池中获取 gzip writer
		g.writer = gzipWriterPool.Get().(*gzip.Writer)
		g.writer.Reset(g.ResponseWriter)

		// 写入所有数据到压缩器
		if _, err := g.writer.Write(allData); err != nil {
			return 0, err
		}
		g.buffer.Reset() // 清空缓冲区
		return len(data), nil
	}

	// 如果已经开始压缩，直接写入 gzip writer
	if g.compressed {
		return g.writer.Write(data)
	}

	// 否则写入缓冲区
	return g.buffer.Write(data)
}

// shouldCompressContent 检查内容类型是否应该被压缩
func (g *gzipResponseWriter) shouldCompressContent() bool {
	contentType := g.Header().Get("Content-Type")
	if contentType == "" {
		// 如果没有内容类型，默认允许压缩
		return true
	}

	// 不压缩已经压缩的内容类型
	nonCompressibleTypes := []string{
		"image/jpeg",
		"image/png",
		"image/gif",
		"image/webp",
		"video/mp4",
		"audio/mpeg",
		"application/zip",
		"application/x-gzip",
	}
	
	// 检查是否为不可压缩的内容类型
	for _, ct := range nonCompressibleTypes {
		if strings.HasPrefix(strings.ToLower(contentType), strings.ToLower(ct)) {
			return false
		}
	}

	return true
}

// Close 关闭 gzip writer 并将其返回到池中
func (g *gzipResponseWriter) Close() error {
	if g.compressed && g.writer != nil {
		err := g.writer.Close()
		// 将 gzip writer 返回到池中
		gzipWriterPool.Put(g.writer)
		g.writer = nil
		return err
	}
	return nil
}

// Flush 刷新缓冲区，确保所有数据都被写入
func (g *gzipResponseWriter) Flush() {
	if !g.compressed {
		// 如果还没有压缩，直接写入缓冲区的内容
		if g.buffer.Len() > 0 {
			g.ResponseWriter.Write(g.buffer.Bytes())
			g.buffer.Reset()
		}
	} else {
		// 如果已经压缩，刷新 gzip writer
		if g.writer != nil {
			g.writer.Flush()
		}
	}
	g.ResponseWriter.Flush()
}

// decompressRequest 检查并解压缩gzip请求体
func decompressRequest(c *gin.Context) {
	// 检查请求是否使用gzip压缩
	contentEncoding := c.GetHeader("Content-Encoding")
	if !strings.Contains(strings.ToLower(contentEncoding), "gzip") {
		return // 不是gzip压缩的请求，无需处理
	}

	// 如果请求体为空，无需解压缩
	if c.Request.Body == nil || c.Request.ContentLength == 0 {
		return
	}

	// 创建gzip读取器来解压缩请求体
	gzipReader, err := gzip.NewReader(c.Request.Body)
	if err != nil {
		// 如果解压缩失败，返回错误
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": "Invalid gzip compressed request body",
		})
		c.Abort()
		return
	}

	// 替换请求体为解压缩后的内容
	c.Request.Body = &readCloser{Reader: gzipReader, originalCloser: c.Request.Body}
}

// readCloser 包装gzip.Reader以实现io.ReadCloser接口
type readCloser struct {
	Reader        *gzip.Reader
	originalCloser io.Closer
}

func (rc *readCloser) Read(p []byte) (n int, err error) {
	return rc.Reader.Read(p)
}

func (rc *readCloser) Close() error {
	// 关闭gzip读取器
	if err := rc.Reader.Close(); err != nil {
		return err
	}
	// 关闭原始请求体
	return rc.originalCloser.Close()
}

// shouldCompress 检查是否应该对响应进行压缩
func shouldCompress(c *gin.Context, threshold int) bool {
	// 检查客户端是否支持 gzip 压缩
	acceptEncoding := c.GetHeader("Accept-Encoding")
	if !parseAcceptEncoding(acceptEncoding) {
		return false
	}

	// 检查响应是否已经压缩
	if c.Writer.Header().Get("Content-Encoding") != "" {
		return false
	}

	// 检查响应大小（如果知道的话）
	if c.Writer.Header().Get("Content-Length") != "" {
		// 如果响应长度已知且小于阈值，不进行压缩
		if length := c.Writer.Header().Get("Content-Length"); length != "" {
			// 这里简化处理，实际使用时可以解析 Content-Length 头
			// 由于我们需要等待响应写入才能知道实际大小，这里跳过这个检查
		}
	}

	return true
}

// parseAcceptEncoding 解析 Accept-Encoding 头，检查是否支持 gzip
func parseAcceptEncoding(acceptEncoding string) bool {
	if acceptEncoding == "" {
		return false
	}

	// 转换为小写进行处理
	acceptEncoding = strings.ToLower(acceptEncoding)

	// 支持通配符
	if strings.Contains(acceptEncoding, "*") {
		return true
	}

	// 分割不同的编码类型
	encodings := strings.Split(acceptEncoding, ",")

	for _, encoding := range encodings {
		// 去除空格
		encoding = strings.TrimSpace(encoding)

		// 检查是否是 gzip
		if strings.HasPrefix(encoding, "gzip") {
			// 检查质量值
			parts := strings.Split(encoding, ";")
			if len(parts) == 1 {
				// 没有质量值，默认为 1.0
				return true
			}

			// 检查质量值是否为 0
			for _, part := range parts[1:] {
				part = strings.TrimSpace(part)
				if strings.HasPrefix(part, "q=") {
					qValue := strings.TrimPrefix(part, "q=")
					if qValue != "0" {
						return true
					}
				}
			}
		}
	}

	return false
}

// CompressionMiddleware 创建 gzip 压缩中间件
// threshold 参数指定了压缩的阈值（字节），只有响应大于此阈值时才进行压缩
func CompressionMiddleware(threshold int) gin.HandlerFunc {
	// 如果阈值小于等于0，使用默认阈值 1KB
	if threshold <= 0 {
		threshold = 1024
	}

	return func(c *gin.Context) {
		// 首先处理请求解压缩
		decompressRequest(c)
		if c.IsAborted() {
			return // 如果解压缩失败，直接返回
		}

		// 检查是否应该进行响应压缩
		if !shouldCompress(c, threshold) {
			c.Next()
			return
		}

		// 创建 gzip 响应写入器
		gzipWriter := &gzipResponseWriter{
			ResponseWriter: c.Writer,
			buffer:         bytes.NewBuffer(nil),
			threshold:      threshold,
			compressed:     false,
		}

		// 替换响应写入器
		c.Writer = gzipWriter

		// 处理请求
		c.Next()

		// 请求处理完成后，确保所有数据都被写入
		if !gzipWriter.compressed {
			// 如果没有进行压缩，直接写入缓冲区的内容
			if gzipWriter.buffer.Len() > 0 {
				gzipWriter.ResponseWriter.Write(gzipWriter.buffer.Bytes())
			}
		} else {
			// 如果已经压缩，关闭 gzip writer
			gzipWriter.Close()
		}
	}
}

// CompressionMiddlewareWithConfig 创建带配置的 gzip 压缩中间件
func CompressionMiddlewareWithConfig(config CompressionConfig) gin.HandlerFunc {
	return CompressionMiddleware(config.Threshold)
}

// CompressionConfig 压缩中间件配置
type CompressionConfig struct {
	// Threshold 压缩阈值（字节），默认 1024
	Threshold int
}

// DefaultCompressionConfig 返回默认的压缩配置
func DefaultCompressionConfig() CompressionConfig {
	return CompressionConfig{
		Threshold: 1024, // 1KB
	}
}
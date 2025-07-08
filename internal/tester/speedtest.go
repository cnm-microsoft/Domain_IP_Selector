package tester

import (
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
	"time"

	"github.com/VividCortex/ewma"
	"golang.org/x/time/rate"
)

// SpeedTestResult 包含一次下载速度测试的结果
type SpeedTestResult struct {
	DownloadSpeed float64 // in B/s
	Colo          string
}

// TestDownloadSpeed 对单个 IP 进行下载速度测试
func TestDownloadSpeed(ip *net.IPAddr, testURL string, timeout time.Duration, rateLimitMB float64) (*SpeedTestResult, error) {
	speed, colo, err := downloadHandler(ip, testURL, timeout, rateLimitMB)
	if err != nil {
		return nil, err
	}
	return &SpeedTestResult{DownloadSpeed: speed, Colo: colo}, nil
}

// downloadHandler 是实际执行下载测速的内部函数
func downloadHandler(ip *net.IPAddr, testURL string, timeout time.Duration, rateLimitMB float64) (float64, string, error) {
	client := &http.Client{
		Transport: &http.Transport{DialContext: getDialContext(ip, DefaultTCPPort)},
		Timeout:   timeout,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			if len(via) > 10 { // 限制最多重定向 10 次
				return http.ErrUseLastResponse
			}
			return nil
		},
	}
	req, err := http.NewRequest("GET", testURL, nil)
	if err != nil {
		return 0.0, "", fmt.Errorf("创建请求失败: %w", err)
	}

	req.Header.Set("User-Agent", "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_12_6) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/98.0.4758.80 Safari/537.36")

	response, err := client.Do(req)
	if err != nil {
		return 0.0, "", fmt.Errorf("请求失败: %w", err)
	}
	defer response.Body.Close()
	if response.StatusCode != 200 {
		bodyBytes, err := io.ReadAll(response.Body)
		errorMsg := fmt.Sprintf("无效的状态码: %d", response.StatusCode)
		if err == nil && len(bodyBytes) > 0 {
			// 将响应体内容附加到错误信息中，限制长度以防刷屏
			bodyStr := string(bodyBytes)
			if len(bodyStr) > 200 {
				bodyStr = bodyStr[:200]
			}
			errorMsg = fmt.Sprintf("%s, 响应: %s", errorMsg, bodyStr)
		}
		return 0.0, "", fmt.Errorf(errorMsg)
	}
	// 通过头部 Server 值判断是 Cloudflare 还是 AWS CloudFront 并设置 cfRay 为各自的机场地区码完整内容
	colo := getHeaderColo(response.Header)

	// 如果设置了速率限制，则创建限速器
	var limiter *rate.Limiter
	if rateLimitMB > 0 {
		// 转换为 B/s
		limit := rate.Limit(rateLimitMB * 1024 * 1024)
		// 桶大小也设置为速率上限，允许一定的突发
		burst := int(rateLimitMB * 1024 * 1024)
		limiter = rate.NewLimiter(limit, burst)
	}

	timeStart := time.Now()           // 开始时间（当前）
	timeEnd := timeStart.Add(timeout) // 加上下载测速时间得到的结束时间

	contentLength := response.ContentLength // 文件大小
	buffer := make([]byte, 8192)            // 增加缓冲区大小以提高效率

	var (
		contentRead     int64 = 0
		timeSlice             = timeout / 100
		timeCounter           = 1
		lastContentRead int64 = 0
	)

	var nextTime = timeStart.Add(timeSlice * time.Duration(timeCounter))
	e := ewma.NewMovingAverage()

	// 创建带超时的上下文
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	// 循环计算，如果文件下载完了（两者相等），则退出循环（终止测速）
	for contentLength != contentRead {
		currentTime := time.Now()
		if currentTime.After(nextTime) {
			timeCounter++
			nextTime = timeStart.Add(timeSlice * time.Duration(timeCounter))
			e.Add(float64(contentRead - lastContentRead))
			lastContentRead = contentRead
		}
		// 如果超出下载测速时间，则退出循环（终止测速）
		if currentTime.After(timeEnd) {
			break
		}

		// 如果有限速器，则等待
		if limiter != nil {
			// WaitN 会阻塞直到可以读取 n 个字节
			// 我们使用 buffer 的大小作为每次请求的量
			if err := limiter.WaitN(ctx, len(buffer)); err != nil {
				// 如果等待时上下文超时或被取消，则退出
				break
			}
		}

		bufferRead, err := response.Body.Read(buffer)
		if err != nil {
			if err != io.EOF { // 如果文件下载过程中遇到报错（如 Timeout），且并不是因为文件下载完了，则退出循环（终止测速）
				break
			} else if contentLength == -1 { // 文件下载完成 且 文件大小未知，则退出循环（终止测速），例如：https://speed.cloudflare.com/__down?bytes=200000000 这样的，如果在 10 秒内就下载完成了，会导致测速结果明显偏低甚至显示为 0.00（下载速度太快时）
				break
			}
			// 获取上个时间片
			last_time_slice := timeStart.Add(timeSlice * time.Duration(timeCounter-1))
			// 下载数据量 / (用当前时间 - 上个时间片/ 时间片)
			e.Add(float64(contentRead-lastContentRead) / (float64(currentTime.Sub(last_time_slice)) / float64(timeSlice)))
		}
		contentRead += int64(bufferRead)
	}
	// B/s
	speed := e.Value() / (timeout.Seconds() / 120)
	return speed, colo, nil
}

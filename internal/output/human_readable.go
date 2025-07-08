package output

import "Domain_IP_Selector_Go/internal/engine"

// HumanReadableResult 定义了一个对人类友好的、用于最终文件输出的数据结构
type HumanReadableResult struct {
	Address           string  `json:"Address"`
	SourceDomain      string  `json:"SourceDomain"`
	DelayMS           float64 `json:"DelayMS"`  // 延迟 (毫秒)
	LossRate          float64 `json:"LossRate"` // 丢包率
	Colo              string  `json:"Colo"`
	Region            string  `json:"Region"`
	DownloadSpeedMBps float64 `json:"DownloadSpeedMBps"` // 下载速度 (MB/s)
}

// ToHumanReadable 将引擎的原始结果转换为对人类友好的格式
func ToHumanReadable(results []engine.SimplifiedResult) []HumanReadableResult {
	humanResults := make([]HumanReadableResult, len(results))
	for i, r := range results {
		humanResults[i] = HumanReadableResult{
			Address:           r.Address,
			SourceDomain:      r.SourceDomain,
			DelayMS:           float64(r.Delay) / 1000000.0, // 纳秒转毫秒
			LossRate:          r.LossRate,
			Colo:              r.Colo,
			Region:            r.Region,
			DownloadSpeedMBps: float64(r.DownloadSpeed) / 1024.0, // KB/s 转 MB/s
		}
	}
	return humanResults
}

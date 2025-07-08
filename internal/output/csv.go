package output

import (
	"Domain_IP_Selector_Go/internal/engine"
	"encoding/csv"
	"fmt"
	"os"
)

// WriteCSVFile 将最终结果列表写入到指定的 CSV 文件中
func WriteCSVFile(filePath string, results []engine.SimplifiedResult) error {
	file, err := os.Create(filePath)
	if err != nil {
		return fmt.Errorf("无法创建 CSV 文件 '%s': %w", filePath, err)
	}
	defer file.Close()

	writer := csv.NewWriter(file)
	defer writer.Flush()

	// 写入表头
	header := []string{
		"IP Address",
		"Source Domain",
		"Delay (ms)",
		"Loss Rate (%)",
		"Colo",
		"Region",
		"Download Speed (MB/s)",
	}
	if err := writer.Write(header); err != nil {
		return fmt.Errorf("写入 CSV 表头失败: %w", err)
	}

	// 写入数据行
	// 将原始结果转换为对人类友好的格式
	humanReadableResults := ToHumanReadable(results)

	// 写入数据行
	for _, r := range humanReadableResults {
		row := []string{
			r.Address,
			r.SourceDomain,
			fmt.Sprintf("%.2f", r.DelayMS),
			fmt.Sprintf("%.2f", r.LossRate*100),
			r.Colo,
			r.Region,
			fmt.Sprintf("%.2f", r.DownloadSpeedMBps), // 使用转换后的 MB/s
		}
		if err := writer.Write(row); err != nil {
			// 记录错误但继续尝试写入其他行
			fmt.Fprintf(os.Stderr, "警告: 写入 CSV 行失败: %v\n", err)
		}
	}

	return writer.Error()
}

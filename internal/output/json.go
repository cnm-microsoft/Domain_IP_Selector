package output

import (
	"Domain_IP_Selector_Go/internal/engine"
	"encoding/json"
	"fmt"
	"os"
)

// WriteJSONFile 将最终结果列表写入到指定的 JSON 文件中
func WriteJSONFile(filePath string, results []engine.SimplifiedResult) error {
	// 将原始结果转换为对人类友好的格式
	humanReadableResults := ToHumanReadable(results)

	data, err := json.MarshalIndent(humanReadableResults, "", "  ")
	if err != nil {
		return fmt.Errorf("无法将结果序列化为 JSON: %w", err)
	}

	err = os.WriteFile(filePath, data, 0644)
	if err != nil {
		return fmt.Errorf("无法写入 JSON 文件 '%s': %w", filePath, err)
	}

	return nil
}

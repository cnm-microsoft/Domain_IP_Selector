package main

import (
	"Domain_IP_Selector_Go/internal/config"
	"Domain_IP_Selector_Go/internal/engine"
	"Domain_IP_Selector_Go/internal/output"
	_ "embed"
	"fmt"
	"log"
	"os"
	"path/filepath"
)

//go:embed default_config.yaml
var defaultConfigData []byte

//go:embed locations.json
var defaultLocationsData []byte

//go:embed reputation_domains.txt
var defaultDomainsData []byte

// ensureFile 检查文件是否存在于可执行文件目录，如果不存在，则使用提供的默认数据创建它。
func ensureFile(fileName string, defaultData []byte) (string, error) {
	exePath, err := os.Executable()
	if err != nil {
		return "", fmt.Errorf("无法获取可执行文件路径: %w", err)
	}
	exeDir := filepath.Dir(exePath)
	filePath := filepath.Join(exeDir, fileName)

	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		if err := os.WriteFile(filePath, defaultData, 0644); err != nil {
			return "", fmt.Errorf("无法写入默认文件 %s: %w", fileName, err)
		}
		log.Printf("首次运行，已在 %s 生成默认 %s 文件", exeDir, fileName)
	} else if err != nil {
		return "", fmt.Errorf("检查文件 %s 时出错: %w", fileName, err)
	}
	return filePath, nil
}

func main() {
	log.Println("--- 开始域名优选 IP 引擎 ---")

	// 1. 确保所有必需的文件都存在
	log.Println("步骤 1/5: 检查并生成所需文件...")
	cfgPath, err := ensureFile("config.yaml", defaultConfigData)
	if err != nil {
		log.Fatalf("初始化配置文件失败: %v", err)
	}
	locationsPath, err := ensureFile("locations.json", defaultLocationsData)
	if err != nil {
		log.Fatalf("初始化 locations.json 失败: %v", err)
	}
	domainsPath, err := ensureFile("reputation_domains.txt", defaultDomainsData)
	if err != nil {
		log.Fatalf("初始化 reputation_domains.txt 失败: %v", err)
	}
	log.Println("文件检查完成。")

	// 2. 加载配置
	cfg, err := config.LoadConfig(cfgPath)
	if err != nil {
		log.Fatalf("加载配置文件失败: %v", err)
	}
	log.Printf("配置加载成功：分组方式=%s, 每组优选IP数=%d", cfg.GroupBy, cfg.TopNPerGroup)

	// 定义日志回调函数
	progressCallback := func(message string) {
		log.Println(message)
	}

	// 3. 运行优选引擎
	exeDir := filepath.Dir(cfgPath)
	finalResults, err := engine.Run(cfg, locationsPath, domainsPath, exeDir, progressCallback)
	if err != nil {
		log.Fatalf("引擎运行时出错: %v", err)
	}

	// 4. 写入结果
	log.Println("步骤 4/4: 写入结果文件...")
	ipVersion := cfg.IPVersion
	if ipVersion == "" {
		ipVersion = "ipv4"
	}
	resultJSONFile := filepath.Join(exeDir, fmt.Sprintf("result_%s.json", ipVersion))
	resultCSVFile := filepath.Join(exeDir, fmt.Sprintf("result_%s.csv", ipVersion))

	if err := output.WriteJSONFile(resultJSONFile, finalResults); err != nil {
		log.Fatalf("写入 result.json 失败: %v", err)
	}
	if err := output.WriteCSVFile(resultCSVFile, finalResults); err != nil {
		log.Fatalf("写入 result.csv 失败: %v", err)
	}
	log.Printf("结果已成功写入 %s 和 %s", resultJSONFile, resultCSVFile)

	log.Println("--- 所有任务已完成 ---")
}

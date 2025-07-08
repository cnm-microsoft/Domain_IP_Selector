# 域名优选 IP 引擎 (Domain IP Selector)

## 🚀 项目概述

本工具旨在通过对大量由cloudflare提供cdn服务的域名进行 DNS 解析，筛选出 Cloudflare 的 IP 地址，并对这些 IP 进行延迟和下载速度测试，最终根据地理位置和测试结果，筛选出在当前网络环境下表现最佳的 IP 地址。

## 🏃‍♂️ 快速使用

直接下载 [releases](https://github.com/ccxkai233/Domain_IP_Selector/releases) 里最新版本的 exe 文件，将其放入一个单独的文件夹内。

### 1️⃣ 运行程序

直接双击 `DomainIPSelector.exe` 文件。

**首次运行**:
程序在启动时会默认生成一份默认的 `config.yaml`, `locations.json`, `reputation_domains.txt` 文件。

### 2️⃣ 修改配置 (可选)

首次运行后，您可以根据需要编辑自动生成的 `config.yaml` 文件，默认不需要动。

### 3️⃣ 查看结果

程序运行完毕后，会在 `.exe` 文件所在的目录下生成结果文件，例如 `result_ipv4.json` 和 `result_ipv4.csv`。


## ✨ 主要特性

- **IP 版本选择**: 可通过配置选择测试 IPv4 或 IPv6。
- **域名与IP去重**: 自动去除域名与解析后的IP列表中的重复项，提高解析效率。
- **多维度测试**:
  - **延迟测试**: 使用 HTTPing 测试 IP 的延迟和丢包率。
  - **速度测试**: 对低延迟的 IP 进行真实下载速度测试。
- **区域化筛选**: 按地理区域（如亚洲、北美等）对 IP 进行分组，并选出各区域表现最好的 Top N 个 IP。
- **全面并发控制**: 可分别配置 DNS 解析、延迟测试、速度测试的并发数，以平衡测试速度和系统资源占用。
- **格式化输出**: 将最终结果同时输出为 `.json` 和 `.csv` 格式，并根据 IP 版本自动命名。

## 📂 项目结构

```
Domain_IP_Selector_Go/
├── cmd/
│   └── main.go           # 程序主入口
├── internal/
│   ├── config/
│   │   └── config.go     # 配置加载逻辑
│   ├── datasource/
│   │   ├── cfips.go      # Cloudflare IP 数据源处理
│   │   └── domains.go    # 域名列表数据源处理
│   ├── locations/
│   │   └── locations.go  # IP 地理位置映射
│   ├── output/
│   │   ├── csv.go        # CSV 文件输出
│   │   └── json.go       # JSON 文件输出
│   └── tester/
│       ├── httping.go    # 延迟测试
│       ├── speedtest.go  # 速度测试
│       └── utils.go      # 测试相关工具函数
├── pkg/
│   └── model/
│       └── types.go      # 项目核心数据结构
├── go.mod
├── go.sum
└── README.md
```

## ⚙️ 工作流程

程序遵循以下步骤来完成优选过程：

1.  **加载配置**: 从项目根目录的 `config.yaml` 文件加载配置。
2.  **初始化数据源**:
    - 加载 `locations.json` 获取 Cloudflare Colo 数据中心到地理区域的映射。
    - 加载 `reputation_domains.txt` 获取用于解析的域名列表（已去重）。
    - 根据配置的 IP 版本，加载或下载对应的 Cloudflare IP 段（例如 `cf-ips-ipv4.txt`）。
3.  **IP 筛选与延迟测试**:
    - **并发解析**所有域名（并发数可控），根据配置只查找 IPv4 或 IPv6 地址。
    - 对所有解析出的 IP 地址进行**去重**，确保每个 IP 只被测试一次。
    - 将去重后的 IP 与 Cloudflare IP 段进行比对，过滤出属于 **Cloudflare 的 IP**。
    - 对所有 Cloudflare IP 进行**并发 HTTPing 延迟测试**（并发数可控）。
    - 淘汰掉延迟过高或丢包率过高的 IP。
4.  **分组与 Top N 筛选**:
    - 将通过延迟测试的 IP 按地理区域分组。
    - 在每个区域内，按延迟从低到高排序，选出前 N 个（N 可配置）IP 进入下一轮。
5.  **下载速度测试**:
    - 对上一轮筛选出的所有 IP 进行并发下载速度测试。
    - 使用信号量机制控制并发数，防止因带宽抢占导致结果不准。
6.  **生成结果**:
    - 将包含延迟、丢包率、下载速度等信息的最终结果按速度从高到低排序。
    - 将结果写入到带 IP 版本标识的 `result_ipvX.json` 和 `result_ipvX.csv` 文件中。

## 🛠️ 如何编译与分发

### 编译

如果需要自行编译，请确保您已安装 Go 语言环境。然后打开终端，进入 `Domain_IP_Selector_Go` 目录，执行以下命令：

```bash
# -s -w 参数可以减小生成文件的大小
go build -ldflags "-s -w" -o "..\DomainIPSelector.exe" ./cmd
```

该命令会在项目根目录（上一级目录）生成 `DomainIPSelector.exe` 文件。

## 🔭 未来可能的开发方向

- **命令行参数**: 将配置项（如并发数、延迟阈值等）改为通过命令行参数传入，增加灵活性。
- **Web 界面**: 为工具开发一个简单的 Web 界面，使其更易于使用。

如果您有任何想法或问题，欢迎提交 Issue 或 Pull Request。

## 🙏 致谢与许可

本项目的核心测速逻辑，受到了 [CloudflareSpeedTest](https://github.com/XIU2/CloudflareSpeedTest) 项目的启发并直接使用了其部分源代码。我们对原作者的辛勤工作和无私分享表示衷心的感谢。

根据原项目的开源协议，本项目同样基于 **GNU General Public License v3.0 (GPL-3.0)** 进行分发。您可以在项目根目录找到 `LICENSE` 文件的副本。如果您分发此软件或其衍生版本，您必须遵守此协议的条款。
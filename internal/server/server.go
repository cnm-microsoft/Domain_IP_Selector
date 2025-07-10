package server

import (
	"Domain_IP_Selector_Go/internal/config"
	"Domain_IP_Selector_Go/internal/engine"
	"Domain_IP_Selector_Go/internal/locations"
	"Domain_IP_Selector_Go/internal/output"
	"bytes"
	"context"
	"embed"
	"encoding/json"
	"fmt"
	"io"
	"io/fs"
	"log"
	"net/http"
	"os"
	"os/exec"
	"runtime"
	"time"

	"github.com/gorilla/websocket"
	"gopkg.in/yaml.v3"
)

//go:embed web
var embeddedFS embed.FS

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool {
		return true // Allow all origins
	},
}

// Start 启动 Web 服务器
func Start(port int, cfgPath, locationsPath, domainsPath, exeDir string) {
	// Create a sub-filesystem to remove the "web" prefix
	staticFS, err := fs.Sub(embeddedFS, "web")
	if err != nil {
		log.Fatalf("Failed to create sub filesystem: %v", err)
	}

	http.Handle("/static/", http.StripPrefix("/static/", http.FileServer(http.FS(staticFS))))
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/" {
			http.NotFound(w, r)
			return
		}
		f, err := staticFS.Open("index.html")
		if err != nil {
			http.Error(w, "index.html not found", http.StatusInternalServerError)
			return
		}
		defer f.Close()

		content, err := io.ReadAll(f)
		if err != nil {
			http.Error(w, "failed to read index.html", http.StatusInternalServerError)
			return
		}
		http.ServeContent(w, r, "index.html", time.Now(), bytes.NewReader(content))
	})

	http.HandleFunc("/api/config", handleConfig(cfgPath))
	http.HandleFunc("/api/locations", handleLocations(locationsPath))
	http.HandleFunc("/ws/run", handleWebSocket(cfgPath, locationsPath, domainsPath, exeDir))

	addr := fmt.Sprintf("0.0.0.0:%d", port)
	log.Printf("服务器正在启动，请在浏览器中打开 http://%s", addr)

	// 尝试在默认浏览器中打开 URL
	go openBrowser(fmt.Sprintf("http://%s", addr))

	if err := http.ListenAndServe(addr, nil); err != nil {
		log.Fatalf("服务器启动失败: %v", err)
	}
}

func handleConfig(cfgPath string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case "GET":
			cfg, err := config.LoadConfig(cfgPath)
			if err != nil {
				http.Error(w, "Failed to load config", http.StatusInternalServerError)
				return
			}
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(cfg)
		case "POST":
			var newConfig map[string]interface{}
			if err := json.NewDecoder(r.Body).Decode(&newConfig); err != nil {
				http.Error(w, "Invalid request body", http.StatusBadRequest)
				return
			}
			if err := saveConfigWithComments(cfgPath, newConfig); err != nil {
				http.Error(w, fmt.Sprintf("Failed to save config: %v", err), http.StatusInternalServerError)
				return
			}
			w.WriteHeader(http.StatusOK)
		default:
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		}
	}
}

func handleLocations(locationsPath string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// We can load and cache this on startup if it's large
		locs, err := locations.LoadLocationsFromFile(locationsPath)
		if err != nil {
			http.Error(w, "Failed to load locations", http.StatusInternalServerError)
			return
		}

		// --- Localized and Common Tags Logic ---
		type tagInfo struct {
			Key      string `json:"key"`
			Display  string `json:"display"`
			IsCommon bool   `json:"is_common"`
		}

		type locationData struct {
			Regions []tagInfo `json:"Regions"`
			Colos   []tagInfo `json:"Colos"`
		}

		// Define common tags and their Chinese names
		commonRegions := map[string]string{"Asia Pacific": "亚太", "North America": "北美"}
		commonColos := map[string]string{"HKG": "香港", "LAX": "洛杉矶", "SJC": "圣何塞"}

		// Process regions
		processedRegions := make(map[string]bool)
		regionTags := []tagInfo{}
		for _, region := range locs {
			if _, exists := processedRegions[region]; !exists {
				displayName, isCommon := commonRegions[region]
				if !isCommon {
					displayName = region // Default to key if no translation
				}
				regionTags = append(regionTags, tagInfo{
					Key:      region,
					Display:  displayName,
					IsCommon: isCommon,
				})
				processedRegions[region] = true
			}
		}

		// Process colos
		processedColos := make(map[string]bool)
		coloTags := []tagInfo{}
		for colo := range locs {
			if _, exists := processedColos[colo]; !exists {
				displayName, isCommon := commonColos[colo]
				if !isCommon {
					displayName = colo // Default to key if no translation
				}
				coloTags = append(coloTags, tagInfo{
					Key:      colo,
					Display:  displayName,
					IsCommon: isCommon,
				})
				processedColos[colo] = true
			}
		}

		data := locationData{
			Regions: regionTags,
			Colos:   coloTags,
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(data)
	}
}

func handleWebSocket(cfgPath, locationsPath, domainsPath, exeDir string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			log.Println("WebSocket upgrade failed:", err)
			return
		}
		// This function will now block until the client disconnects or an error occurs.
		// We will run the engine logic within this goroutine.

		// 1. Wait for the initial config message from the client
		_, msg, err := conn.ReadMessage()
		if err != nil {
			log.Println("WebSocket read for config failed:", err)
			return
		}

		// 1. 先加载文件中的配置作为基础
		runConfig, err := config.LoadConfig(cfgPath)
		if err != nil {
			log.Printf("Failed to load base config: %v", err)
			conn.WriteMessage(websocket.TextMessage, []byte(fmt.Sprintf("Error: Failed to load base config: %v", err)))
			return
		}

		// 2. 然后用 WebSocket 发来的数据覆盖它
		// 这样，前端没有提供的字段（如被禁用的并发数）将保留文件中的值
		if err := json.Unmarshal(msg, runConfig); err != nil {
			log.Println("Failed to unmarshal config from WebSocket:", err)
			conn.WriteMessage(websocket.TextMessage, []byte(fmt.Sprintf("Error: Invalid config format: %v", err)))
			return
		}

		// 2. Create a context that can be cancelled if the client disconnects
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		// 3. Start a separate goroutine to listen for client-side close messages
		go func() {
			defer cancel() // Cancel context if this goroutine exits
			for {
				// If ReadMessage returns an error, it means the client has disconnected.
				if _, _, err := conn.ReadMessage(); err != nil {
					log.Printf("Client disconnected: %v", err)
					break // Exit the loop, which will trigger the defer cancel()
				}
			}
		}()

		// Define a structured message for WebSocket communication
		type WebSocketMessage struct {
			Type    string      `json:"type"` // "log" or "result"
			Payload interface{} `json:"payload"`
		}

		// Create a channel to serialize all WebSocket writes
		writeChan := make(chan WebSocketMessage, 64) // Buffered channel

		// Start a dedicated writer goroutine. This is the ONLY goroutine that writes to the connection.
		go func() {
			for msg := range writeChan {
				if err := conn.WriteJSON(msg); err != nil {
					log.Printf("WebSocket write error: %v", err)
					break // Exit on write error
				}
			}
		}()

		// 4. Create a callback that sends progress messages to the write channel
		progressCallback := func(message string) {
			select {
			case <-ctx.Done():
				return // Don't send if client is gone
			default:
				writeChan <- WebSocketMessage{Type: "log", Payload: message}
			}
		}

		// 5. Run the engine in the main handler goroutine
		finalResults, err := engine.Run(runConfig, locationsPath, domainsPath, exeDir, progressCallback)
		if err != nil {
			errMsg := fmt.Sprintf("引擎运行时出错: %v", err)
			progressCallback(errMsg)
			log.Println(errMsg)
		} else {
			// Send final results to the client via the channel
			select {
			case <-ctx.Done():
			default:
				writeChan <- WebSocketMessage{Type: "result", Payload: finalResults}
			}

			// Save results to files
			if len(finalResults) > 0 {
				ipVersion := "ipv4" // Default
				if runConfig != nil && runConfig.IPVersion != "" {
					ipVersion = runConfig.IPVersion
				}
				jsonFileName := fmt.Sprintf("web_result_%s.json", ipVersion)
				csvFileName := fmt.Sprintf("web_result_%s.csv", ipVersion)

				go func() {
					if err := output.WriteJSONFile(jsonFileName, finalResults); err != nil {
						log.Printf("保存 JSON 文件失败: %v", err)
						progressCallback(fmt.Sprintf("错误: 保存 %s 失败。", jsonFileName))
					} else {
						progressCallback(fmt.Sprintf("结果已保存到 %s", jsonFileName))
					}
				}()

				go func() {
					if err := output.WriteCSVFile(csvFileName, finalResults); err != nil {
						log.Printf("保存 CSV 文件失败: %v", err)
						progressCallback(fmt.Sprintf("错误: 保存 %s 失败。", csvFileName))
					} else {
						progressCallback(fmt.Sprintf("结果已保存到 %s", csvFileName))
					}
				}()
			}
		}

		// 6. After the engine is done, close the connection
		progressCallback("--- 任务完成 ---")
		close(writeChan)                   // Close the channel to signal the writer goroutine to exit
		time.Sleep(200 * time.Millisecond) // Give writer goroutine a moment to send the last message
		conn.Close()
	}
}

func saveConfigWithComments(cfgPath string, newValues map[string]interface{}) error {
	data, err := os.ReadFile(cfgPath)
	if err != nil {
		return err
	}

	var root yaml.Node
	if err := yaml.Unmarshal(data, &root); err != nil {
		return err
	}

	// yaml.v3 unmarshals to a document node, we need the content
	docNode := root.Content[0]

	// Iterate through the key-value pairs of the mapping node
	for i := 0; i < len(docNode.Content); i += 2 {
		keyNode := docNode.Content[i]
		valNode := docNode.Content[i+1]

		if newValue, ok := newValues[keyNode.Value]; ok {
			// Update the value node with the new value
			setNodeValue(valNode, newValue)
		}
	}

	out, err := yaml.Marshal(&root)
	if err != nil {
		return err
	}

	return os.WriteFile(cfgPath, out, 0644)
}

// openBrowser tries to open the URL in a default browser.
func openBrowser(url string) {
	var err error
	switch runtime.GOOS {
	case "linux":
		err = exec.Command("xdg-open", url).Start()
	case "windows":
		err = exec.Command("rundll32", "url.dll,FileProtocolHandler", url).Start()
	case "darwin":
		err = exec.Command("open", url).Start()
	default:
		err = fmt.Errorf("unsupported platform")
	}
	if err != nil {
		log.Printf("无法自动打开浏览器: %v\n请手动打开 %s", err, url)
	}
}

// setNodeValue updates a yaml.Node's value based on the provided interface{}.
// It handles basic types and slices.
func setNodeValue(node *yaml.Node, value interface{}) {
	if slice, isSlice := value.([]interface{}); isSlice {
		node.Kind = yaml.SequenceNode
		node.Tag = "!!seq"
		node.Content = []*yaml.Node{}
		for _, item := range slice {
			itemNode := &yaml.Node{}
			// Recursively set value for items in slice
			setNodeValue(itemNode, item)
			node.Content = append(node.Content, itemNode)
		}
	} else {
		// For simple scalar values
		s := fmt.Sprintf("%v", value)
		node.Value = s
		node.Kind = yaml.ScalarNode

		// Heuristic to guess the tag
		if s == "true" || s == "false" {
			node.Tag = "!!bool"
		} else if _, err := strToFloat(s); err == nil {
			node.Tag = "!!float"
		} else if _, err := strToInt(s); err == nil {
			node.Tag = "!!int"
		} else {
			node.Tag = "!!str"
		}
	}
}

func strToFloat(s string) (float64, error) {
	var f float64
	// Use json unmarshaling to handle number parsing robustly
	return f, json.Unmarshal([]byte(s), &f)
}

func strToInt(s string) (int, error) {
	var i int
	// Use json unmarshaling to handle integer parsing robustly
	return i, json.Unmarshal([]byte(s), &i)
}

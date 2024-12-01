package main

import (
	"bufio"
	"flag"
	"fmt"
	"log"
	"os"
	"net/http"
	"os/exec"
	"strconv"
	"strings"
	"time"
	"sync"
	"github.com/fsnotify/fsnotify"
)

// 定义全局变量
var passwallTaskEnable bool // 用于保存 passwall 配置是否启用

// Config represents a single configuration block
type Config struct {
	ID        string
	Enabled   bool
	Time      string
	Weekdays  []int
}
// Login Config
type loginConfig struct {
	Config
	Action    string
	ISP       string
	Account   string
	Password  string
}
// passwall Config
type passwallConfig struct {
	Config
	Action  string
	Node    string
	Mode    string
}

var (
	configLock sync.Mutex
	loginConfigs    []loginConfig
	passwallConfigs []passwallConfig
)

//定义任务管理结构
var (
	taskLock   sync.Mutex
	taskRunners = make(map[string]chan bool) // 用于控制任务的启动与停止
)

// 自定义日志输出结构体
type logWriter struct {
	file *os.File
	loc  *time.Location
}

// 实现 io.Writer 接口
func (lw *logWriter) Write(p []byte) (n int, err error) {
	// 获取当前时间，并使用自定义时区格式化
	now := time.Now().In(lw.loc)
	timeStamp := now.Format("2006-01-02 15:04:05.000000") // 定制时间格式

	// 构建最终日志字符串
	finalMessage := fmt.Sprintf("%s %s", timeStamp, p)
	return lw.file.Write([]byte(finalMessage))
}

// ReadConfig reads the configuration file and returns a list of enabled configurations
func ReadConfig(filePath string) ([]loginConfig, []passwallConfig, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return nil, nil, err
	}
	defer file.Close()

	var loginConfigs []loginConfig
	var passwallConfigs []passwallConfig
	var currentLoginConfig loginConfig
	var currentPasswallConfig passwallConfig
	var configType string // 用一个独立的变量来区分配置类型
	inBlock := false

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		// 解析新的 config 块
		if strings.HasPrefix(line, "config") {
			// 如果当前有未完成的配置块，保存到 configs
			if inBlock {
        		// 根据 config_type 字段保存配置
        		switch configType {
        		case "login":
        		    loginConfigs = append(loginConfigs, currentLoginConfig)
        		case "passwall":
        		    passwallConfigs = append(passwallConfigs, currentPasswallConfig)
        		}
			}
			parts := strings.Fields(line)
			if len(parts) >= 2 { // 确保有至少两部分
				configType = parts[1] // 获取第二部分
				// 使用 switch 来处理不同类型的配置
				switch configType {
				case "login":
					if len(parts) == 3 {
						currentLoginConfig = loginConfig{Config: Config{ID: strings.Trim(parts[2], "'")}} // 初始化为 loginConfig
					} else {
						currentLoginConfig = loginConfig{} // 重置为默认值
					}
					inBlock = true // 标记进入块
			
				case "passwall":
					if len(parts) == 3 {
						currentPasswallConfig = passwallConfig{Config: Config{ID: strings.Trim(parts[2], "'")}} // 初始化为 passwallConfig
					} else {
						currentPasswallConfig = passwallConfig{} // 重置为默认值
					}
					inBlock = true // 标记进入块
			
				default:
					// 非 login 或 passwall 配置类型，忽略并重置块
					currentLoginConfig = loginConfig{}
					currentPasswallConfig = passwallConfig{}
					inBlock = false // 无效块，忽略
				}
			} else {
				currentLoginConfig = loginConfig{} // 不符合格式，重置
				currentPasswallConfig = passwallConfig{}
				inBlock = false    // 无效块，不设置 inBlock
			}
			continue
		}
		

		// 解析块中的 option
		if inBlock {
			parts := strings.Fields(line)
			if len(parts) >= 3 && parts[0] == "option" {
				key := parts[1]
				value := strings.Trim(strings.Join(parts[2:], " "), "'")
				switch configType {
				case "login":
					switch key {
					case "enable":
						currentLoginConfig.Enabled = value == "1"
					case "action":
						currentLoginConfig.Action = value
					case "isp":
						currentLoginConfig.ISP = value
					case "account":
						currentLoginConfig.Account = value
					case "password":
						currentLoginConfig.Password = value
					case "time":
						currentLoginConfig.Time = value
					case "weekdays":
						currentLoginConfig.Weekdays = ParseWeekdays(value)
					}
		
				case "passwall":
					switch key {
					case "enable":
						currentPasswallConfig.Enabled = value == "1"
					case "action":
						currentPasswallConfig.Action = value
					case "node":
						currentPasswallConfig.Node = value
					case "mode":
						currentPasswallConfig.Mode = value
					case "time":
						currentPasswallConfig.Time = value
					case "weekdays":
						currentPasswallConfig.Weekdays = ParseWeekdays(value)
					}
				}
			}
		}
	}

	// 最后一个块未保存时，添加到 configs
	if inBlock {
    switch configType {
    case "login":
        loginConfigs = append(loginConfigs, currentLoginConfig)
    case "passwall":
        passwallConfigs = append(passwallConfigs, currentPasswallConfig)
    }
	}

	if err := scanner.Err(); err != nil {
		return nil, nil, err
	}

	return loginConfigs, passwallConfigs, nil
}


// ParseWeekdays parses a space-separated string of weekdays into a slice of integers
func ParseWeekdays(input string) []int {
	var weekdays []int
	parts := strings.Fields(input) // 按空格分割
	for _, p := range parts {
		if wd, err := strconv.Atoi(p); err == nil {
			weekdays = append(weekdays, wd)
		}
	}
	return weekdays
}

// BaseURL 定义基础的 IP 和端口部分
const BaseURL = "http://10.2.5.251:801/eportal/"

// sendLoginRequest sends the login HTTP request for a given configuration
func sendLoginRequest(config loginConfig) {
	var url string

	if config.Action == "logout" {
		// 如果 Action 是 logout，使用特定 URL，不包含账号、密码和运营商信息
		url = fmt.Sprintf("%s?c=Portal&a=logout&login_method=1&user_account=drcom&user_password=123", BaseURL)
	} else {
		// 判断是否需要 ISP
		if config.ISP == "cumt" {
			// 如果 ISP 是 cumt，不添加 ISP 值
			url = fmt.Sprintf(
				"%s?c=Portal&a=%s&login_method=1&user_account=%s%%40&user_password=%s",
				BaseURL, config.Action, config.Account, config.Password,
			)
		} else {
			// 如果 ISP 不为 cumt，正常拼接 ISP 值
			url = fmt.Sprintf(
				"%s?c=Portal&a=%s&login_method=1&user_account=%s%%40%s&user_password=%s",
				BaseURL, config.Action, config.Account, config.ISP, config.Password,
			)
		}
	}

	log.Printf("[%s] 请求的 URL: %s", config.ID, url)

	// 发送 HTTP GET 请求
	resp, err := http.Get(url)
	if err != nil {
		log.Printf("[%s] 请求失败: %v\n", config.ID, err)
		return
	}
	defer resp.Body.Close()

	// 检查响应状态
	if resp.StatusCode == http.StatusOK {
		log.Printf("[%s] 请求成功\n", config.ID)
	} else {
		log.Printf("[%s] 请求失败，状态码: %d\n", config.ID, resp.StatusCode)
	}
}

// execPasswallCommand
func execPasswallCommand(config passwallConfig) {
    if config.Action == "enable" {
        // 修改配置项，将 enabled 设置为 1
        err := setConfigValue("1")
        if err != nil {
            log.Printf("启用 Passwall 失败: %v", err)
            return
        }
        log.Println("Passwall 已启用")

    } else if config.Action == "disable" {
        // 修改配置项，将 enabled 设置为 0
        err := setConfigValue("0")
        if err != nil {
            log.Printf("禁用 Passwall 失败: %v", err)
            return
        }
        log.Println("Passwall 已禁用")
    } else {
        log.Printf("无效的 action: %s，跳过执行", config.Action)
        return
    }

    // 提交更改
    err := commitConfigChanges()
    if err != nil {
        log.Printf("提交配置更改失败: %v", err)
        return
    }

    // 重启服务
    err = restartService()
    if err != nil {
        log.Printf("重启 Passwall 服务失败: %v", err)
        return
    }

    log.Println("Passwall 配置已更新并重启服务")
}


// scheduleLoginTaskWithRefresh schedules tasks and refreshes the next execution time after each execution
func scheduleLoginTaskWithRefresh(config loginConfig) {
    for {
        nextTime, err := nextExecutionTime(config.Weekdays, config.Time)
        if err != nil {
            log.Printf("[%s] 无法计算下次执行时间: %v\n", config.ID, err)
            return
        }

        // 计算等待时间
        duration := time.Until(nextTime)
        log.Printf("[%s] Login 任务已调度: %s\n", config.ID, nextTime.Format("2006-01-02 15:04:05"))
        time.Sleep(duration) // 等待到指定时间

        // 执行任务
        log.Printf("[%s] 正在执行Login任务...\n", config.ID)
        sendLoginRequest(config)

        // 任务完成后重新计算时间
        log.Printf("[%s] Login 任务完成，重新计算下次执行时间\n", config.ID)
    }
}

// nextExecutionTime calculates the next execution time based on weekdays and time of day
func nextExecutionTime(weekdays []int, timeOfDay string) (time.Time, error) {
    now := time.Now().In(time.Local) // 明确指定使用本地时区
    loc := now.Location()

    // 解析配置中的时间
    configTime, err := time.ParseInLocation("15:04:05", timeOfDay, loc)
    if err != nil {
		log.Printf("无效的时间格式: %v", err)
        return time.Time{}, fmt.Errorf("无效的时间格式: %v", err)
    }

    // 当前时间的日期部分与配置时间的时间部分合并
    todayTime := time.Date(now.Year(), now.Month(), now.Day(), configTime.Hour(), configTime.Minute(), configTime.Second(), 0, loc)

    // 检查今天是否符合条件
    weekday := int(now.Weekday())
    for _, w := range weekdays {
        if w == weekday && todayTime.After(now) {
            return todayTime, nil // 今天符合条件，且时间未过
        }
    }

    // 如果今天不符合条件或时间已过，寻找下一个符合条件的星期
    for i := 1; i <= 7; i++ {
        nextDay := now.AddDate(0, 0, i)
        nextWeekday := int(nextDay.Weekday())
        for _, w := range weekdays {
            if w == nextWeekday {
                return time.Date(nextDay.Year(), nextDay.Month(), nextDay.Day(), configTime.Hour(), configTime.Minute(), configTime.Second(), 0, loc), nil
            }
        }
    }
	log.Printf("未找到下次执行时间")
	return time.Time{}, fmt.Errorf("未找到下次执行时间")
}

// schedulePasswallTaskWithRefresh schedules tasks and refreshes the next execution time after each execution
func schedulePasswallTaskWithRefresh(config passwallConfig) {
    for {
        nextTime, err := nextExecutionTime(config.Weekdays, config.Time)
        if err != nil {
            log.Printf("[%s] 无法计算下次执行时间: %v\n", config.ID, err)
            return
        }

        // 计算等待时间
        duration := time.Until(nextTime)
        log.Printf("[%s] Passwall 任务已调度: %s\n", config.ID, nextTime.Format("2006-01-02 15:04:05"))
        time.Sleep(duration) // 等待到指定时间

        // 执行任务
        log.Printf("[%s] 正在执行Passwall任务...\n", config.ID)
        execPasswallCommand(config)

        // 任务完成后重新计算时间
        log.Printf("[%s] Passwall任务完成，重新计算下次执行时间\n", config.ID)
    }
}

func containsValidWeekday(weekdays []int) bool {
    for _, day := range weekdays {
        if day >= 0 && day <= 6 { // 有效星期几是 0 到 6
            return true
        }
    }
    return false
}

func watchConfigFile(filePath string) {
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		log.Fatalf("创建文件监视器失败: %v", err)
	}
	defer watcher.Close()

	err = watcher.Add(filePath)
	if err != nil {
		log.Fatalf("无法监视文件: %v", err)
	}

	for {
		select {
		case event := <-watcher.Events:
			if event.Op&(fsnotify.Write|fsnotify.Create|fsnotify.Rename|fsnotify.Remove) != 0 {
				log.Println("配置文件已修改，重新加载配置...")
				configLock.Lock()
				loadedLoginConfigs, loadedPasswallConfigs, err := ReadConfig(filePath)
				if err != nil {
					log.Printf("重新加载配置失败: %v\n", err)
				} else {
					loginConfigs = loadedLoginConfigs
					passwallConfigs = loadedPasswallConfigs
					log.Println("配置文件已重新加载，新的配置项如下：")
					// 调用 printConfigs 打印新的配置
					printLoginConfigs(loginConfigs)
					printPasswallConfigs(passwallConfigs)
                    // 更新登录任务和 passwall 任务
                    updateTaskRunners(loginConfigs, passwallConfigs) 
				}
				configLock.Unlock()
			}
		case err := <-watcher.Errors:
			log.Printf("文件监视器错误: %v", err)
		}
	}
}

func stopAllTasks() {
	taskLock.Lock()
	defer taskLock.Unlock()

	for id, stopChan := range taskRunners {
		close(stopChan) // 发送停止信号
		log.Printf("任务 [%s] 已停止", id)
	}
	taskRunners = make(map[string]chan bool) // 清空任务管理
}


func updateTaskRunners(loginConfigs []loginConfig, passwallConfigs []passwallConfig) {
	// 停止所有当前任务
	log.Println("停止所有当前任务...")
	stopAllTasks()

	// 重新启动任务
	taskLock.Lock()
	defer taskLock.Unlock()

    // 启动 login 任务
	for _, config := range loginConfigs {
		if config.Enabled {
			// 筛选条件和校验逻辑
			if len(config.Weekdays) == 0 || !containsValidWeekday(config.Weekdays) {
				log.Printf("Login [%s] 跳过：没有指定有效的星期\n", config.ID)
				continue
			}

			if _, err := time.Parse("15:04:05", config.Time); err != nil {
				log.Printf("Login [%s] 跳过：无效的时间格式: %v\n", config.ID, err)
				continue
			}

			// 启动新任务
			stopChan := make(chan bool)
			taskRunners[config.ID] = stopChan
			go func(cfg loginConfig, stopChan chan bool) {
				for {
					select {
					case <-stopChan:
						log.Printf("Login 任务 [%s] 已停止", cfg.ID)
						return
					default:
						scheduleLoginTaskWithRefresh(cfg)
					}
				}
			}(config, stopChan)
			log.Printf("Login 任务 [%s] 已启动", config.ID)
		}
	}

	// 如果passwall 能正常配置
	if passwallTaskEnable {
		// 启动 passwall 任务
		for _, config := range passwallConfigs {
			if config.Enabled {
				// 筛选条件和校验逻辑
				if len(config.Weekdays) == 0 || !containsValidWeekday(config.Weekdays) {
					log.Printf("Passwall [%s] 跳过：没有指定有效的星期\n", config.ID)
					continue
				}

				if _, err := time.Parse("15:04:05", config.Time); err != nil {
					log.Printf("Passwall [%s] 跳过：无效的时间格式: %v\n", config.ID, err)
					continue
				}

				// 启动新任务
				stopChan := make(chan bool)
				taskRunners[config.ID] = stopChan
				go func(cfg passwallConfig, stopChan chan bool) {
					for {
						select {
						case <-stopChan:
							log.Printf("Passwall 任务 [%s] 已停止", cfg.ID)
							return
						default:
							schedulePasswallTaskWithRefresh(cfg)
						}
					}
				}(config, stopChan)
				log.Printf("Passwall 任务 [%s] 已启动", config.ID)
			}
		}
	}

}


// printConfigs 打印所有配置项到日志和控制台
func printLoginConfigs(configs []loginConfig) {
	log.Println("当前Login配置项：")
	for _, config := range configs {
		log.Printf("ID: %s", config.ID)
		log.Printf("Enabled: %t", config.Enabled)
		log.Printf("Action: %s", config.Action)
		log.Printf("ISP: %s", config.ISP)
		log.Printf("Account: %s", config.Account)
		log.Printf("Password: %s", config.Password)
		log.Printf("Time: %s", config.Time)
		log.Printf("Weekdays: %v", config.Weekdays)
		log.Println("----------------------------------------")
	}
	fmt.Println("当前Login配置项：")
	for _, config := range configs {
		fmt.Printf("ID: %s\n", config.ID)
		fmt.Printf("Enabled: %t\n", config.Enabled)
		fmt.Printf("Action: %s\n", config.Action)
		fmt.Printf("ISP: %s\n", config.ISP)
		fmt.Printf("Account: %s\n", config.Account)
		fmt.Printf("Password: %s\n", config.Password)
		fmt.Printf("Time: %s\n", config.Time)
		fmt.Printf("Weekdays: %v\n", config.Weekdays)
		fmt.Println("----------------------------------------")
	}
}

func printPasswallConfigs(configs []passwallConfig) {
	log.Println("当前Passwall配置项：")
	for _, config := range configs {
		log.Printf("ID: %s", config.ID)
		log.Printf("Enabled: %t", config.Enabled)
		log.Printf("Action: %s", config.Action)
		log.Printf("Node: %s", config.Node)
		log.Printf("Mode: %s", config.Mode)
		log.Printf("Time: %s", config.Time)
		log.Printf("Weekdays: %v", config.Weekdays)
		log.Println("----------------------------------------")
	}
	fmt.Println("当前Passwall配置项：")
	for _, config := range configs {
		fmt.Printf("ID: %s\n", config.ID)
		fmt.Printf("Enabled: %t\n", config.Enabled)
		fmt.Printf("Action: %s\n", config.Action)
		fmt.Printf("Node: %s\n", config.Node)
		fmt.Printf("Mode: %s\n", config.Mode)
		fmt.Printf("Time: %s\n", config.Time)
		fmt.Printf("Weekdays: %v\n", config.Weekdays)
		fmt.Println("----------------------------------------")
	}
}


func main() {

	// 设置全局时区为东八区
	loc := time.FixedZone("CST", 8*3600) // CST: China Standard Time, +8小时
	time.Local = loc // 设置全局时区
	
	// 打开日志文件
	logFile, err := os.OpenFile("/tmp/cumt-net.log", os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		fmt.Printf("无法打开日志文件: %v\n", err)
		return
	}
	defer logFile.Close()

	// 自定义日志输出格式
	log.SetOutput(logFile)
	log.SetFlags(0) // 禁用默认的日期时间格式

	// 使用自定义的时间格式
	log.SetPrefix("") // 清除默认前缀
	log.SetFlags(0) // 取消 log.Ldate 和 log.Ltime

	// 自定义日志格式化函数
	log.SetFlags(0)
	log.SetOutput(&logWriter{logFile, loc})

	// 定义一个命令行参数，用于指定配置文件路径
	configFilePath := flag.String("config", "./config", "配置文件路径")
	flag.Parse() // 解析命令行参数

	// 输出用于调试的日志
	log.Printf("使用的配置文件: %s\n", *configFilePath)

	// 读取配置文件
	loginConfigs, passwallConfigs, err := ReadConfig(*configFilePath)
	if err != nil {
		log.Fatalf("读取配置文件失败: %v\n", err)
		return
	}

	// 初始化 passwallTaskEnable
	initializePasswallTask()

	// 输出 passwallTaskEnable 的值
	if passwallTaskEnable {
		log.Println("Passwall 配置已启用，任务可以执行。")
	} else {
		log.Println("Passwall 配置未启用，任务不可执行。")
	}

	log.Println("----------------------------------------程序启动----------------------------------------")
	fmt.Println("程序启动成功")
	fmt.Printf("使用的配置文件: %s\n", *configFilePath)
	// 提示用户日志文件位置
	fmt.Println("日志文件位置: /tmp/cumt-net.log")

	// 调用 printConfigs 函数打印配置项
	printLoginConfigs(loginConfigs)
	
	printPasswallConfigs(passwallConfigs)

	// 监视配置文件
	go watchConfigFile(*configFilePath)

	// 启动任务
	updateTaskRunners(loginConfigs, passwallConfigs) 
	/*
	// 获取当前值
	value, err := getConfigValue()
	if err != nil {
		log.Fatalf("Error getting config value: %v", err)
	}
	fmt.Printf("Current value of enabled: %s\n", value)

	// 翻转值：如果为 '0'，则设置为 '1'；否则设置为 '0'
	var newValue string
	if value == "0" {
		newValue = "1"
	} else if value == "1" {
		newValue = "0"
	} else {
		log.Fatalf("Unexpected value for enabled: %s", value)
	}

	// 修改配置值
	err = setConfigValue(newValue)
	if err != nil {
		log.Fatalf("Error setting config value: %v", err)
	}
	fmt.Printf("Config value has been updated to: %s\n", newValue)

	// 提交配置更改
	err = commitConfigChanges()
	if err != nil {
		log.Fatalf("Error committing config changes: %v", err)
	}
	fmt.Println("Configuration changes have been committed.")

	// 重启服务
	err = restartService()
	if err != nil {
		log.Fatalf("Error restarting service: %v", err)
	}
	fmt.Println("Service has been restarted successfully.")
	*/
	// 主线程保持运行
	select {}
}

// 初始化程序时检查配置并设置 passwallTaskEnable
func initializePasswallTask() {
	// 获取 passwall 配置项的值
	_, err := getConfigValue()
	if err != nil {
		log.Printf("获取passwall配置失败，设置 passwallTaskEnable 为 false: %v", err)
		passwallTaskEnable = false
	} else {
		passwallTaskEnable = true
	}
}

// 获取配置项的当前值
func getConfigValue() (string, error) {
	// 执行 uci get 命令获取当前值
	cmd := exec.Command("uci", "get", "passwall.@global[0].enabled")
	output, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("failed to get config value: %v", err)
	}
	// 去除输出中的换行符
	return strings.TrimSpace(string(output)), nil
}

// 修改配置项的值
func setConfigValue(value string) error {
	// 执行 uci set 命令设置新的值
	cmd := exec.Command("uci", "set", fmt.Sprintf("passwall.@global[0].enabled=%s", value))
	err := cmd.Run()
	if err != nil {
		return fmt.Errorf("failed to set config value: %v", err)
	}
	return nil
}

// 提交配置更改
func commitConfigChanges() error {
	// 执行 uci commit 命令提交更改
	cmd := exec.Command("uci", "commit", "passwall")
	err := cmd.Run()
	if err != nil {
		return fmt.Errorf("failed to commit config changes: %v", err)
	}
	return nil
}

// 重启服务
func restartService() error {
	// 执行重启服务命令
	cmd := exec.Command("/etc/init.d/passwall", "restart")
	err := cmd.Run()
	if err != nil {
		return fmt.Errorf("failed to restart service: %v", err)
	}
	return nil
}

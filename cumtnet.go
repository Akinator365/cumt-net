package main

import (
	"bufio"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"
	"sync"
	"github.com/fsnotify/fsnotify"
)

// Config represents a single configuration block
type Config struct {
	ID        string
	Enabled   bool
	Action    string
	ISP       string
	Account   string
	Password  string
	Time      string
	Weekdays  []int
}

var (
	configLock sync.Mutex
	configs    []Config
)

//定义任务管理结构
var (
	taskLock   sync.Mutex
	taskRunners = make(map[string]chan bool) // 用于控制任务的启动与停止
)


// ReadConfig reads the configuration file and returns a list of enabled configurations
func ReadConfig(filePath string) ([]Config, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	var configs []Config
	var current Config
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
				configs = append(configs, current)
			}
			parts := strings.Fields(line)
			if len(parts) >= 2 { // 确保有至少两部分
				configType := parts[1] // 获取第二部分
				if configType == "config" { // 判断是否是有效的 config 类型
					if len(parts) == 3 {
						current = Config{ID: strings.Trim(parts[2], "'")}
					} else {
						current = Config{} // 重置为默认值
					}
					inBlock = true // 仅当类型为 config 时标记进入块
				} else {
					current = Config{} // 非 config 类型，忽略
					inBlock = false    // 无效块，不设置 inBlock
				}
			} else {
				current = Config{} // 不符合格式，重置
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

				switch key {
				case "enable":
					current.Enabled = value == "1"
				case "action":
					current.Action = value
				case "isp":
					current.ISP = value
				case "account":
					current.Account = value
				case "password":
					current.Password = value
				case "time":
					current.Time = value
				case "weekdays":
					current.Weekdays = ParseWeekdays(value)
				}
			}
		}
	}

	// 最后一个块未保存时，添加到 configs
	if inBlock {
		configs = append(configs, current)
	}

	if err := scanner.Err(); err != nil {
		return nil, err
	}

	return configs, nil
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
func sendLoginRequest(config Config) {
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



// scheduleTaskWithRefresh schedules tasks and refreshes the next execution time after each execution
func scheduleTaskWithRefresh(config Config) {
    for {
        nextTime, err := nextExecutionTime(config.Weekdays, config.Time)
        if err != nil {
            log.Printf("[%s] 无法计算下次执行时间: %v\n", config.ID, err)
            return
        }

        // 计算等待时间
        duration := time.Until(nextTime)
        log.Printf("[%s] 任务已调度: %s\n", config.ID, nextTime.Format("2006-01-02 15:04:05"))
        time.Sleep(duration) // 等待到指定时间

        // 执行任务
        log.Printf("[%s] 正在执行任务...\n", config.ID)
        sendLoginRequest(config)

        // 任务完成后重新计算时间
        log.Printf("[%s] 任务完成，重新计算下次执行时间\n", config.ID)
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
			if event.Op&(fsnotify.Write|fsnotify.Create) != 0 {
				log.Println("配置文件已修改，重新加载配置...")
				configLock.Lock()
				loadedConfigs, err := ReadConfig(filePath)
				if err != nil {
					log.Printf("重新加载配置失败: %v\n", err)
				} else {
					configs = loadedConfigs
					log.Println("配置文件已重新加载，新的配置项如下：")
					// 调用 printConfigs 打印新的配置
					printConfigs(configs)
					// 启动任务
					updateTaskRunners(configs)				
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


func updateTaskRunners(configs []Config) {
	// 停止所有当前任务
	log.Println("停止所有当前任务...")
	stopAllTasks()

	// 重新启动任务
	taskLock.Lock()
	defer taskLock.Unlock()

	for _, config := range configs {
		if config.Enabled {
			// 筛选条件和校验逻辑
			if len(config.Weekdays) == 0 || !containsValidWeekday(config.Weekdays) {
				log.Printf("[%s] 跳过：没有指定有效的星期\n", config.ID)
				continue
			}

			if _, err := time.Parse("15:04:05", config.Time); err != nil {
				log.Printf("[%s] 跳过：无效的时间格式: %v\n", config.ID, err)
				continue
			}

			// 启动新任务
			stopChan := make(chan bool)
			taskRunners[config.ID] = stopChan
			go func(cfg Config, stopChan chan bool) {
				for {
					select {
					case <-stopChan:
						log.Printf("任务 [%s] 已停止", cfg.ID)
						return
					default:
						scheduleTaskWithRefresh(cfg)
					}
				}
			}(config, stopChan)
			log.Printf("任务 [%s] 已启动", config.ID)
		}
	}
}


// printConfigs 打印所有配置项到日志和控制台
func printConfigs(configs []Config) {
	log.Println("当前配置项：")
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
	fmt.Println("当前配置项：")
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


func main() {

	// 设置全局时区为东八区
	loc := time.FixedZone("CST", 8*3600) // CST: China Standard Time, +8小时
	time.Local = loc // 设置全局时区
	
	// 打开日志文件
	logFile, err := os.OpenFile("/tmp/cumt-login.log", os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		fmt.Printf("无法打开日志文件: %v\n", err)
		return
	}
	defer logFile.Close()

	// 设置日志输出
	log.SetOutput(logFile)
	log.SetFlags(log.Ldate | log.Ltime | log.Lmicroseconds | log.LUTC)

	// 定义一个命令行参数，用于指定配置文件路径
	configFilePath := flag.String("config", "./config", "配置文件路径")
	flag.Parse() // 解析命令行参数

	// 输出用于调试的日志
	log.Printf("使用的配置文件: %s\n", *configFilePath)

	// 读取配置文件
	configs, err := ReadConfig(*configFilePath)
	if err != nil {
		log.Fatalf("读取配置文件失败: %v\n", err)
		return
	}
	log.Println("----------------------------------------程序启动----------------------------------------")
	fmt.Println("程序启动成功")
	fmt.Printf("使用的配置文件: %s\n", *configFilePath)
	// 提示用户日志文件位置
	fmt.Println("日志文件位置: /tmp/cumt-login.log")

	// 调用 printConfigs 函数打印配置项
	printConfigs(configs)

	// 监视配置文件
	go watchConfigFile(*configFilePath)

	// 启动任务
	updateTaskRunners(configs)

	// 主线程保持运行
	select {}
}

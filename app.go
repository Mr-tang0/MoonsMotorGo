package main

import (
	"MOONs/backend"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	update "MOONs/backend"

	"github.com/wailsapp/wails/v2/pkg/runtime"
)

// App struct
type App struct {
	ctx    context.Context
	comm   backend.SerialCommunicator
	motors map[string]*backend.MoonsMotor

	updater *update.UpdateService

	cancelFunc    context.CancelFunc // 用于取消自动化任务的函数
	isAutorunning bool               // 标记是否正在运行
}

// NewApp creates a new App application struct
func NewApp() *App {
	return &App{motors: make(map[string]*backend.MoonsMotor)}
}

// startup is called when the app starts. The context is saved
// so we can call the runtime methods
func (a *App) startup(ctx context.Context) {
	a.ctx = ctx
	// 初始化更新服务
	a.updater = &update.UpdateService{}

	a.comm = backend.SerialCommunicator{
		Port:       "COM0",
		BaudRate:   9600,
		Timeout:    100 * time.Millisecond,
		MaxRetries: 1,
	}

	// 启动后台监控协程
	go a.monitorLoop()
}

type APIResponse struct {
	Status  string      `json:"status"`
	Message string      `json:"message"`
	Data    interface{} `json:"data"`
}

// 监控循环
func (a *App) monitorLoop() {
	// 将定时器设为 100ms
	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()

	// 计数器，用于实现 10 次循环一次的慢速读取 (1s)
	cycleCounter := 0

	// 记录哪些电机处于“高频追踪”模式
	// key 为电机 ID，value 为 true 表示正在运动，需要高频读取
	activeTracking := make(map[int]bool)

	for {
		select {
		case <-a.ctx.Done():
			return
		case <-ticker.C:
			if !a.comm.IsConnected() {
				continue
			}

			cycleCounter++
			// 是否到达了 1s 一次的全局扫描周期
			isFullScanCycle := (cycleCounter >= 10)

			for _, motor := range a.motors {

				motorID := motor.Config.ID

				// 判定：这个电机这轮是否需要读取？
				// 条件：或者是全局扫描周期到了，或者是该电机正在运动（高频追踪中）
				shouldRead := isFullScanCycle || activeTracking[motorID]

				if shouldRead {
					//如果电机没有使能，直接略过
					if motor.Enabled {
						// 1. 读取位置
						_, errPos := motor.GetPosition()

						if errPos != nil {
							// 认为电机下线了，将电机状态置为disable,下一次不在读取，同时通知前端：发送下线信号
							fmt.Printf("电机 %d 获取位置失败: %v\n", motorID, errPos)
							motor.Enabled = false
						}

						// 2. 读取错误状态
						errErr := motor.GetError()

						// 3. 读取运动状态（判断是否继续高频追踪的关键）
						isMoving, errStat := motor.GetMotionStatus()

						// 更新追踪状态：如果正在运动，下个 100ms 继续读；如果不运动了，回归全局频率
						if errStat == nil {
							activeTracking[motorID] = isMoving
						}

						// 4. 数据推送给前端
						if errPos == nil && errErr == nil {
							runtime.EventsEmit(a.ctx, "motor_status_update", map[string]interface{}{
								"id":        motorID,
								"position":  motor.Position - motor.Zero,
								"error":     motor.Error,
								"isMoving":  isMoving, // 告知前端是否正在运动，方便 UI 展示
								"isEnabled": motor.Enabled,
							})
						}
					} else {
						// 如果电机未使能，或下线
						runtime.EventsEmit(a.ctx, "motor_status_update", map[string]interface{}{
							"id":        motorID,
							"position":  motor.Position - motor.Zero,
							"error":     motor.Error,
							"isMoving":  false,
							"isEnabled": motor.Enabled,
						})

					}

				}
			}

			// 重置全局扫描计数器
			if isFullScanCycle {
				cycleCounter = 0
			}
		}
	}
}

// 统一控制接口：查找设备
func (a *App) EnumDevices() APIResponse {
	ports, err := a.comm.ListAvailablePorts()
	if err != nil {
		return APIResponse{"error", err.Error(), nil}
	}

	for _, port := range ports {
		println(port)
	}

	return APIResponse{
		"success",
		"Devices found",
		map[string]interface{}{"ports": ports}}
}

// 统一控制接口：连接设备

func (a *App) ConnectDevice(port string, baudRate int) APIResponse {
	err := a.comm.Connect(port, baudRate)
	if err != nil {
		return APIResponse{"error", err.Error(), nil}
	}
	return APIResponse{"success", "Device connected", nil}
}

// 统一控制接口：断开设备
func (a *App) DisconnectDevice() {
	err := a.comm.Disconnect()
	if err != nil {
		return
	}
}

func (a *App) DeleteMotor(id int) APIResponse {
	//根据地址，查找对应电机实例
	var motor *backend.MoonsMotor
	for key := range a.motors {
		if a.motors[key].Config.ID == id {
			motor = a.motors[key]
			break
		}
	}
	if motor == nil {
		return APIResponse{"error", "Motor not found", nil}
	}
	delete(a.motors, fmt.Sprintf("MOTOR%d", motor.Config.ID))

	// 同步保存到本地 JSON
	a.SaveMotorsToLocal()

	return APIResponse{"success", "Motor deleted", nil}
}

func (a *App) EditMotor(id int, motorConfig backend.MotorConfig) APIResponse {
	fmt.Println("正在编辑电机", motorConfig)
	//根据地址，查找对应电机实例
	var motor *backend.MoonsMotor
	var oldKey string
	for key := range a.motors {
		if a.motors[key].Config.ID == id {
			motor = a.motors[key]
			oldKey = key
			break
		}
	}
	if motor == nil {
		return APIResponse{"error", "Motor not found", nil}
	}

	// 先更新配置中的ID（如果NewID有效）
	if motorConfig.NewID != 0 && motorConfig.NewID != id {
		//如果其他电机以及新ID已存在，返回错误
		var existMotor *backend.MoonsMotor
		for key := range a.motors {
			if a.motors[key].Config.ID == motorConfig.NewID {
				existMotor = a.motors[key]
				break
			}
		}
		if existMotor != nil {
			fmt.Printf("电机 %d 已存在\n", motorConfig.NewID)
			return APIResponse{"error", fmt.Sprintf("Motor ID %d already exists", motorConfig.NewID), nil}
		}

		if motorConfig.NewID > 31 || motorConfig.NewID < 1 {
			return APIResponse{"error", fmt.Sprintf("Motor ID %d is out of range", motorConfig.NewID), nil}
		}

		err := motor.SetID(id, motorConfig.NewID)
		if err != nil {
			fmt.Printf("电机 %d 修改ID失败: %v\n", id, err)
		} else {
			fmt.Printf("电机 %d 修改ID成功: %d\n", id, motorConfig.NewID)
			// 更新motorConfig中的ID为NewID，确保配置一致性
			motorConfig.ID = motorConfig.NewID

			if a.ctx != nil {
				runtime.EventsEmit(a.ctx, "edit_ID", map[string]interface{}{
					"oldID": id,
					"newID": motorConfig.NewID,
				})
				fmt.Println("Wails 事件 edit_ID 已成功发出")
			} else {
				fmt.Println("❌ 警告: a.ctx 为 nil，无法发送 Wails 事件！")
			}
		}
	}

	speed, _ := motor.GetSpeed()

	if motorConfig.Speed != speed {
		motor.SetSpeed(float32(motorConfig.Speed))
	}

	motor.Config = motorConfig
	delete(a.motors, oldKey)
	a.motors[oldKey] = motor

	// 同步保存到本地 JSON
	a.SaveMotorsToLocal()
	return APIResponse{"success", "Motor edited", nil}
}

// 统一控制接口：根据 ID 寻找电机并执行使能
func (a *App) MotorEnable(id int, enable bool) APIResponse {

	//根据地址，查找对应电机实例
	var motor *backend.MoonsMotor
	for key := range a.motors {
		fmt.Println(a.motors[key].Config.ID)
		if a.motors[key].Config.ID == id {
			motor = a.motors[key]
			break
		}
	}
	if motor == nil {
		return APIResponse{"error", fmt.Sprintf("Motor %d not found", id), nil}
	}

	ok := motor.Enable(enable)

	if ok != nil {
		fmt.Printf("电机 %d %s 失败: %v\n", id, map[bool]string{true: "使能", false: "去使能"}[enable], ok)
		return APIResponse{"error", "Motor enable/disable failed", ok.Error()}
	} else {
		fmt.Printf("电机 %d %s 成功\n", id, map[bool]string{true: "使能", false: "去使能"}[enable])
		return APIResponse{"success", "Motor enabled/disabled successfully", nil}
	}
}

// 统一控制接口：根据 ID 停止
func (a *App) MotorStop(id int) APIResponse {
	//根据地址，查找对应电机实例
	var motor *backend.MoonsMotor
	for key := range a.motors {
		if a.motors[key].Config.ID == id {
			motor = a.motors[key]
			break
		}
	}
	if motor == nil {
		return APIResponse{"error", fmt.Sprintf("Motor %d not found", id), nil}
	}

	err := motor.Stop()

	if err != nil {
		return APIResponse{"error", "Motor stop failed", err.Error()}
	} else {
		return APIResponse{"success", "Motor stopped successfully", nil}
	}
}

// 统一控制接口：根据 ID 相对运动
func (a *App) MotorMoveRelative(id int, length float32) APIResponse {
	//根据地址，查找对应电机实例
	var motor *backend.MoonsMotor
	for key := range a.motors {
		if a.motors[key].Config.ID == id {
			motor = a.motors[key]
			break
		}
	}
	if motor == nil {
		return APIResponse{"error", fmt.Sprintf("Motor %d not found", id), nil}
	}

	err := motor.MoveRelative(length)

	if err != nil {
		return APIResponse{"error", fmt.Sprintf("Motor %d move relative failed: %v", id, err), nil}
	} else {
		return APIResponse{"success", fmt.Sprintf("Motor %d move relative successfully", id), nil}
	}
}

// 统一控制接口：根据 ID 将当前位置置为0
func (a *App) ResetPosition(id int) APIResponse {
	//根据地址，查找对应电机实例
	var motor *backend.MoonsMotor
	for key := range a.motors {
		if a.motors[key].Config.ID == id {
			motor = a.motors[key]
			break
		}
	}
	if motor == nil {
		return APIResponse{"error", fmt.Sprintf("Motor %d not found", id), nil}
	}

	err := motor.SetHome()
	if err != nil {
		return APIResponse{"error", fmt.Sprintf("Motor %d reset position failed: %v", id, err), nil}
	} else {
		return APIResponse{"success", fmt.Sprintf("Motor %d reset position successfully", id), nil}
	}
}

// 获取本地历史电机
func (a *App) GetConfigPath() string {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		// 如果获取失败，降级使用当前目录
		return "config.json"
	}
	configDir := filepath.Join(homeDir, "Tang", "MOONS")
	_ = os.MkdirAll(configDir, 0755)
	return filepath.Join(configDir, "config.json")
}

func (a *App) LoadLocalMotors() {

	configPath := a.GetConfigPath()
	fmt.Println("配置文件路径:", configPath)

	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		fmt.Println("本地配置文件不存在，跳过加载")
		return
	}

	data, err := os.ReadFile(configPath)
	if err != nil {
		fmt.Printf("读取配置文件失败: %v\n", err)
		return
	}

	var motorConfigs []backend.MotorConfig
	err = json.Unmarshal(data, &motorConfigs)
	if err != nil {
		fmt.Printf("解析配置文件 JSON 失败: %v\n", err)
		return
	}

	fmt.Printf("从本地加载了 %d 个电机配置\n", len(motorConfigs))

	for _, motor_detial := range motorConfigs {
		if motor_detial.ID == 0 {
			fmt.Println("跳过 ID 为 0 的电机")
			continue
		}
		fmt.Println("正在加载电机", motor_detial.Name)
		motor := backend.NewMotor(motor_detial, &a.comm)

		a.motors[fmt.Sprintf("MOTOR%d", motor_detial.ID)] = &motor

		runtime.EventsEmit(a.ctx, "find_motor", motor_detial)
	}
}

// SaveMotorsToLocal 将 motors 全部保存为扁平化的 JSON 格式
func (a *App) SaveMotorsToLocal() error {
	configPath := a.GetConfigPath()
	fmt.Println("配置文件路径:", configPath)

	// 定义一个与你要求的 JSON 格式完全一致的临时结构体
	type FlatMotorConfig struct {
		ID          int     `json:"id"`
		Name        string  `json:"name"`
		DIR         int     `json:"dir"`
		Speed       float32 `json:"speed"`
		Resolution  int     `json:"resolution"`
		Unit        string  `json:"unit"`
		CWName      string  `json:"cwName"`
		CCWName     string  `json:"ccwName"`
		Mode        string  `json:"mode"`
		Description string  `json:"description"`
	}

	var saveList []FlatMotorConfig

	for _, m := range a.motors {
		saveList = append(saveList, FlatMotorConfig{
			ID:          m.Config.ID,
			Name:        m.Config.Name,
			Speed:       m.Config.Speed,
			Resolution:  m.Config.Resolution,
			Unit:        m.Config.Unit,
			CWName:      m.Config.CWName, // 如果 MoonsMotor 结构体里有这些字段则 m.CWName
			CCWName:     m.Config.CCWName,
			Mode:        m.Config.Mode,
			Description: m.Config.Description,
		})
	}

	// 序列化
	jsonData, err := json.MarshalIndent(saveList, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(configPath, jsonData, 0644)
}

func (a *App) SearchMotors() APIResponse {
	// 鸣志 Modbus 地址通常从 1 开始
	for i := 1; i <= 32; i++ {
		// 计算进度百分比
		progress := int((float32(i) / float32(32)) * 100)
		// 发送进度事件到前端
		runtime.EventsEmit(a.ctx, "search_progress", progress)

		// 1. 创建一个临时电机实例用于测试通讯
		// 默认配置，mode 设为 modbus
		//name用时间命名：格式为motor_时间戳后5位
		motorKey := fmt.Sprintf("MOTOR_%d", time.Now().Unix()%100000)
		testConfig := backend.MotorConfig{
			ID:         i,
			Name:       motorKey,
			Unit:       "mm",
			Resolution: 1000,
			Mode:       "scl", // 先使用 scl 模式扫描
		}

		// 初始化临时对象
		testMotor := backend.NewMotor(testConfig, &a.comm)
		// 尝试读取电机位置
		// 如果电机在线且协议匹配，GetPosition 会成功返回
		pos, err := testMotor.GetPosition()
		if err != nil {
			// 如果读取失败（超时或CRC错误），切换为 modbus 模式
			testConfig.Mode = "modbus"
			testMotor.Config = testConfig
			pos, err = testMotor.GetPosition()
			if err != nil {
				// 如果 modbus 模式也失败，说明该地址没有电机
				fmt.Printf("扫描地址 %d: 无响应或通讯错误: %v\n", i, err)
				continue
			}
		}

		// 运行到这里说明电机在线
		fmt.Printf("找到新设备：地址 %d, 当前位置: %f\n", i, pos)

		// 如果电机已在内存列表中，跳过添加，但可以尝试使能

		//根据地址，查找对应电机实例
		var motor *backend.MoonsMotor
		for key := range a.motors {
			if a.motors[key].Config.ID == i {
				motor = a.motors[key]
				break
			}
		}
		if motor != nil {
			fmt.Printf("节点 %d 已在列表中，跳过重复添加\n", i)
			a.MotorEnable(i, true)
			continue
		}

		// 组装最终的电机配置
		motorDetail := backend.MotorConfig{
			ID:          i,
			Name:        motorKey,
			Unit:        "mm",
			Resolution:  20000,
			Speed:       1,
			CCWName:     "CCW",
			CWName:      "CW",
			Mode:        testConfig.Mode,
			Description: "自动扫描发现",
		}

		// 6. 初始化正式的后端电机实例并存入 Map (注意这里存的是指针)
		realMotor := backend.NewMotor(motorDetail, &a.comm)
		a.motors[motorKey] = &realMotor

		// 7. 发送给前端通知增加卡片
		runtime.EventsEmit(a.ctx, "find_motor", motorDetail)

		// 8. 自动使能并清除报警
		realMotor.Enable(true)

		// 9. 发现新设备后同步保存到本地 JSON
		a.SaveMotorsToLocal()

	}

	// 搜索结束，发送 100% 信号
	runtime.EventsEmit(a.ctx, "search_progress", 100)

	return APIResponse{"success", "电机扫描完成", nil}
}

// 统一控制接口：手动添加电机
func (a *App) ManualAddMotor() APIResponse {
	// 查找一个未被占用的ID (1-31)
	var TargetID int
	usedIDs := make(map[int]bool)

	// 收集已使用的ID
	for key := range a.motors {
		usedIDs[a.motors[key].Config.ID] = true
	}

	// 查找第一个未使用的ID
	for i := 1; i <= 31; i++ {
		if !usedIDs[i] {
			TargetID = i
			break
		}
	}

	// 如果没有找到可用ID
	if TargetID == 0 {
		return APIResponse{"error", "No available motor ID (1-31)", nil}
	}

	// 生成电机名称：MOTOR_时间戳后5位
	motorKey := fmt.Sprintf("MOTOR_%d", time.Now().Unix()%100000)

	// 组装电机配置
	motorDetail := backend.MotorConfig{
		ID:          TargetID,
		Name:        motorKey,
		Unit:        "mm",
		Resolution:  20000,
		Speed:       1,
		CCWName:     "CCW",
		CWName:      "CW",
		Mode:        "scl",
		Description: "手动添加",
	}

	// 初始化后端电机实例并存入 Map
	realMotor := backend.NewMotor(motorDetail, &a.comm)
	a.motors[motorKey] = &realMotor

	// 发送给前端通知增加卡片
	runtime.EventsEmit(a.ctx, "find_motor", motorDetail)

	// 同步保存到本地 JSON
	a.SaveMotorsToLocal()

	return APIResponse{"success", "Motor added", motorDetail}
}

func (a *App) APIUpdate() update.GitHubRelease {
	//获取更新信息
	release, err := a.updater.GetUpdateInfo()
	if err != nil {
		fmt.Printf("获取更新信息失败: %v\n", err)
		return update.GitHubRelease{}
	}
	fmt.Printf("更新信息: %v\n", release)
	return release
}

func (a *App) GetCachedRelease() update.GitHubRelease {
	return a.updater.GetCachedRelease()
}

// StartAutomation 前端调用的接口
func (a *App) StartAutomation(automationScript string) bool {
	if automationScript == "" || a.isAutorunning {
		return false
	}

	// 创建一个可以取消的 Context
	var runCtx context.Context
	runCtx, a.cancelFunc = context.WithCancel(context.Background())
	a.isAutorunning = true

	// 必须在独立的 goroutine 中运行，防止阻塞 Wails 主线程导致前端卡死
	go func() {
		defer func() {
			a.isAutorunning = false
		}()

		fmt.Println("====== 自动化脚本开始执行 ======")
		lines := preprocessScript(automationScript)

		// 执行解释器
		err := a.executeScript(runCtx, lines)
		if err != nil {
			fmt.Printf("脚本执行中断或出错: %v\n", err)
		} else {
			fmt.Println("====== 自动化脚本圆满完成 ======")
		}
	}()

	return true
}

// StopAutomation 前端点击“停止”时调用的接口
func (a *App) StopAutomation() {
	if a.cancelFunc != nil {
		a.cancelFunc() // 触发 Context 取消信号
		fmt.Println("收到用户停止指令，正在终止自动化...")
	}
}

// 预处理：按行切分，去空格，忽略空行
func preprocessScript(script string) []string {
	rawLines := strings.Split(script, "\n")
	var lines []string
	for _, line := range rawLines {
		trimmed := strings.TrimSpace(line)
		if trimmed != "" && !strings.HasPrefix(trimmed, "//") { // 支持 // 注释
			lines = append(lines, trimmed)
		}
	}
	return lines
}

// 核心解释器引擎
func (a *App) executeScript(ctx context.Context, lines []string) error {
	pc := 0 // Program Counter 程序计数器（当前执行到第几行）

	for pc < len(lines) {
		// 每次执行新一行前，先检查用户是否点击了“停止”
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		line := lines[pc]

		// 1. 处理 FOR 循环
		if strings.HasPrefix(strings.ToUpper(line), "FOR ") {
			countStr := strings.TrimSpace(line[4:])
			count, err := strconv.Atoi(countStr)
			if err != nil {
				runtime.EventsEmit(a.ctx, "auto_error", fmt.Sprintf("第 %d 行 FOR 循环次数解析失败: %s", pc+1, countStr))
				return fmt.Errorf("第 %d 行 FOR 循环次数解析失败: %s", pc+1, countStr)
			}

			// 寻找对应的 END 建立循环体界限
			endPc := findMatchingEnd(lines, pc)
			if endPc == -1 {
				runtime.EventsEmit(a.ctx, "auto_error", fmt.Sprintf("第 %d 行的 FOR 循环缺少对应的 END", pc+1))
				return fmt.Errorf("第 %d 行的 FOR 循环缺少对应的 END", pc+1)
			}

			// 提取循环体内的子代码块
			subLines := lines[pc+1 : endPc]

			// 循环执行子代码块
			for i := 0; i < count; i++ {
				// 嵌套执行时也需要随时检查退出状态
				if err := a.executeScript(ctx, subLines); err != nil {
					return err
				}
			}

			// 循环结束后，跳过整个 FOR-END 块
			pc = endPc + 1
			continue
		}

		// 2. 处理 DELAY 延时
		if strings.HasPrefix(strings.ToUpper(line), "DELAY ") {
			msStr := strings.TrimSpace(line[6:])
			ms, err := strconv.Atoi(msStr)
			if err != nil {
				runtime.EventsEmit(a.ctx, "auto_error", fmt.Sprintf("第 %d 行 DELAY 时间解析失败: %s", pc+1, msStr))
				return fmt.Errorf("第 %d 行 DELAY 时间解析失败: %s", pc+1, msStr)
			}

			// 使用 select 监听延时，这样如果中途点停止，能立刻唤醒退出，不用白等
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(time.Duration(ms) * time.Millisecond):
			}

			pc++
			continue
		}

		// 3. 处理电机控制行 (例如 "1: CW 10" 或 "MOTOR1: CCW 5")
		if strings.Contains(line, ":") {
			parts := strings.SplitN(line, ":", 2)
			motorID := strings.TrimSpace(parts[0])
			cmdParts := strings.Fields(strings.TrimSpace(parts[1]))

			if len(cmdParts) < 2 {
				runtime.EventsEmit(a.ctx, "auto_error", fmt.Sprintf("第 %d 行电机指令格式错误: %s", pc+1, line))
				return fmt.Errorf("第 %d 行电机指令格式错误: %s", pc+1, line)
			}

			action := strings.ToUpper(cmdParts[0]) // CW 或 CCW
			valStr := cmdParts[1]
			value, _ := strconv.ParseFloat(valStr, 64)

			// 调用底层的通用硬件驱动控制函数
			err := a.AutoMoveMotor(ctx, motorID, action, value)
			if err != nil {
				runtime.EventsEmit(a.ctx, "auto_error", fmt.Sprintf("第 %d 行执行电机动作失败: %v", pc+1, err))
				return fmt.Errorf("执行电机动作失败: %v", err)
			}

			pc++
			continue
		}

		runtime.EventsEmit(a.ctx, "auto_error", fmt.Sprintf("第 %d 行无法识别的指令行: %s", pc+1, line))
		return fmt.Errorf("无法识别的指令行: %s", line)
	}

	return nil
}

// 辅助函数：寻找 FOR 匹配的 END 位置（支持多层 FOR 嵌套）
func findMatchingEnd(lines []string, start int) int {
	stack := 0
	for i := start; i < len(lines); i++ {
		upperLine := strings.ToUpper(lines[i])
		if strings.HasPrefix(upperLine, "FOR ") {
			stack++
		} else if upperLine == "END" {
			stack--
			if stack == 0 {
				return i
			}
		}
	}
	return -1
}

// 模拟具体的电机物理控制函数
func (a *App) AutoMoveMotor(ctx context.Context, motorID string, action string, value float64) error {
	fmt.Printf("[硬件动作] 寻找到轴: %s -> 执行动作: %s -> 参数: %f\n", motorID, action, value)
	var motor *backend.MoonsMotor
	for key := range a.motors {
		if a.motors[key].Config.Name == motorID {
			motor = a.motors[key]
			break
		}
	}
	if motor == nil {
		fmt.Println("未找到电机:", motorID)

		return fmt.Errorf("未找到电机: %s", motorID)
	}

	switch action {
	case "CW":
		motor.MoveRelative(float32(value))
		return nil
	case "CCW":
		motor.MoveRelative(-float32(value))
		return nil
	case "STOP":
		motor.Stop()
		return nil
	case "VE":
		motor.SetSpeed(float32(value))
		return nil
	case "EN":
		if value == 1 {
			motor.Enable(true)
		} else {
			motor.Enable(false)
		}
		return nil
	default:
		return fmt.Errorf("未知的动作: %s", action)
	}
}

// ReadFile：调起系统选择框，选择 TXT 文件并读取
func (a *App) ReadFile() (string, error) {
	// 配置打开文件对话框的参数
	selectedFile, err := runtime.OpenFileDialog(a.ctx, runtime.OpenDialogOptions{
		Title: "选择自动化脚本文件",
		Filters: []runtime.FileFilter{
			{
				DisplayName: "文本文件 (*.txt;*.json)",
				Pattern:     "*.txt;*.json", // 过滤只显示 txt 和 json 格式
			},
			{
				DisplayName: "所有文件 (*.*)",
				Pattern:     "*.*",
			},
		},
	})

	// 如果用户点击了“取消”，selectedFile 会返回空字符串
	if err != nil {
		return "", fmt.Errorf("打开文件对话框失败: %v", err)
	}
	if selectedFile == "" {
		return "", fmt.Errorf("用户取消了选择")
	}

	// 读取选择的文件内容
	content, err := os.ReadFile(selectedFile)
	if err != nil {
		return "", fmt.Errorf("读取文件失败: %v", err)
	}

	return string(content), nil
}

// WriteFile：调起系统保存框，输入文件名后保存 content
func (a *App) WriteFile(content string) error {
	// 配置保存文件对话框的参数
	savePath, err := runtime.SaveFileDialog(a.ctx, runtime.SaveDialogOptions{
		Title:           "导出自动化脚本",
		DefaultFilename: "script.txt", // 默认填充的文件名
		Filters: []runtime.FileFilter{
			{
				DisplayName: "文本文件 (*.txt)",
				Pattern:     "*.txt",
			},
		},
	})

	// 如果用户点击了“取消”，savePath 会返回空字符串
	if err != nil {
		return fmt.Errorf("打开保存对话框失败: %v", err)
	}
	if savePath == "" {
		return fmt.Errorf("用户取消了保存")
	}

	// 强制确保用户如果自己手改了后缀或者没写后缀时，依然是 .txt (可选，增强鲁棒性)
	if filepath.Ext(savePath) == "" {
		savePath += ".txt"
	}

	// 将内容写入到用户选择的绝对路径中
	err = os.WriteFile(savePath, []byte(content), 0644)
	if err != nil {
		return fmt.Errorf("写入文件失败: %v", err)
	}

	return nil
}

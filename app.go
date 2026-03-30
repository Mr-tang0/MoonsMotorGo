package main

import (
	"MOONs/backend"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/wailsapp/wails/v2/pkg/runtime"
)

// App struct
type App struct {
	ctx    context.Context
	comm   backend.SerialCommunicator
	motors map[string]*backend.MoonsMotor
}

// NewApp creates a new App application struct
func NewApp() *App {
	return &App{motors: make(map[string]*backend.MoonsMotor)}
}

// startup is called when the app starts. The context is saved
// so we can call the runtime methods
func (a *App) startup(ctx context.Context) {
	a.ctx = ctx
	a.comm = backend.SerialCommunicator{
		Port:       "COM0",
		BaudRate:   9600,
		Timeout:    100 * time.Millisecond,
		MaxRetries: 2,
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
				//如果电机没有使能，直接略过
				if !motor.Enabled {
					continue
				}

				motorID := motor.Config.ID

				// 判定：这个电机这轮是否需要读取？
				// 条件：或者是全局扫描周期到了，或者是该电机正在运动（高频追踪中）
				shouldRead := isFullScanCycle || activeTracking[motorID]

				if shouldRead {
					// 1. 读取位置
					_, errPos := motor.GetPosition()

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
							"position":  motor.Position,
							"error":     motor.Error,
							"isMoving":  isMoving, // 告知前端是否正在运动，方便 UI 展示
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

// 统一控制接口：手动添加电机
func (a *App) ManualAddMotor(motorConfig backend.MotorConfig) APIResponse {
	if motorConfig.ID == 0 {
		return APIResponse{"error", "ID cannot be 0", nil}
	}
	motor := backend.NewMotor(motorConfig, &a.comm)
	a.motors[fmt.Sprintf("MOTOR%d", motorConfig.ID)] = &motor
	return APIResponse{"success", "Motor added", nil}
}

// 统一控制接口：根据 ID 寻找电机并执行使能
func (a *App) MotorEnable(id int, enable bool) APIResponse {

	key := fmt.Sprintf("MOTOR%d", id)
	motor := a.motors[key]
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
	key := fmt.Sprintf("MOTOR%d", id)
	motor := a.motors[key]

	err := motor.Stop()

	if err != nil {
		return APIResponse{"error", "Motor stop failed", err.Error()}
	} else {
		return APIResponse{"success", "Motor stopped successfully", nil}
	}
}

// 统一控制接口：根据 ID 相对运动
func (a *App) MotorMoveRelative(id int, length float32) APIResponse {
	key := fmt.Sprintf("MOTOR%d", id)
	motor := a.motors[key]
	err := motor.MoveRelative(length)

	if err != nil {
		return APIResponse{"error", fmt.Sprintf("Motor %d move relative failed: %v", id, err), nil}
	} else {
		return APIResponse{"success", fmt.Sprintf("Motor %d move relative successfully", id), nil}
	}
}

// 统一控制接口：根据 ID 将当前位置置为0
func (a *App) ResetPosition(id int) APIResponse {
	key := fmt.Sprintf("MOTOR%d", id)
	motor := a.motors[key]
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
	fmt.Printf("configDir: %s", configDir)

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

	// 定义一个与你要求的 JSON 格式完全一致的临时结构体
	type FlatMotorConfig struct {
		ID          int    `json:"id"`
		Name        string `json:"name"`
		DIR         int    `json:"dir"`
		Speed       int    `json:"speed"`
		Resolution  int    `json:"resolution"`
		Unit        string `json:"unit"`
		CWName      string `json:"cwName"`
		CCWName     string `json:"ccwName"`
		Mode        string `json:"mode"`
		Description string `json:"description"`
	}

	var saveList []FlatMotorConfig

	for _, m := range a.motors {
		saveList = append(saveList, FlatMotorConfig{
			ID:          m.Config.ID,
			Name:        m.Config.Name,
			DIR:         m.Config.DIR,
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
		// 1. 创建一个临时电机实例用于测试通讯
		// 默认配置，mode 设为 modbus
		testConfig := backend.MotorConfig{
			ID:         i,
			Name:       fmt.Sprintf("MOTOR%d", i),
			Unit:       "mm",
			DIR:        1,
			Resolution: 1000,
			Mode:       "modbus", // 强制使用 modbus 模式扫描
		}

		// 初始化临时对象
		testMotor := backend.NewMotor(testConfig, &a.comm)

		// 2. 尝试读取电机位置
		// 如果电机在线且协议匹配，GetPosition 会成功返回
		pos, err := testMotor.GetPosition()

		if err != nil {
			// 如果读取失败（超时或CRC错误），说明该地址没有电机
			fmt.Printf("扫描地址 %d: 无响应或通讯错误: %v\n", i, err)
			continue
		}

		// 3. 运行到这里说明电机在线
		motorKey := fmt.Sprintf("MOTOR%d", i)
		fmt.Printf("找到新设备：地址 %d, 当前位置: %d\n", i, pos)

		// 4. 如果电机已在内存列表中，跳过添加，但可以尝试使能
		if _, exists := a.motors[motorKey]; exists {
			fmt.Printf("节点 %d 已在列表中，跳过重复添加\n", i)
			a.MotorEnable(i, true)
			continue
		}

		// 5. 组装最终的电机配置
		motorDetail := backend.MotorConfig{
			ID:          i,
			Name:        motorKey,
			Unit:        "mm",
			DIR:         1,
			Resolution:  20000,
			Speed:       1,
			CCWName:     "CCW",
			CWName:      "CW",
			Mode:        "modbus",
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

	return APIResponse{"success", "电机扫描完成", nil}
}

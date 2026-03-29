package main

import (
	"MOONs/backend"
	"bytes"
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
	motors map[string]backend.MoonsMotor
}

// NewApp creates a new App application struct
func NewApp() *App {
	return &App{motors: make(map[string]backend.MoonsMotor)}
}

// startup is called when the app starts. The context is saved
// so we can call the runtime methods
func (a *App) startup(ctx context.Context) {
	a.ctx = ctx
	a.comm = backend.SerialCommunicator{
		Port:       "COM0",
		BaudRate:   9600,
		Timeout:    200 * time.Millisecond,
		MaxRetries: 3,
	}
}

type APIResponse struct {
	Status  string      `json:"status"`
	Message string      `json:"message"`
	Data    interface{} `json:"data"`
}

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

func (a *App) ConnectDevice(port string, baudRate int) APIResponse {
	err := a.comm.Connect(port, baudRate)
	if err != nil {
		return APIResponse{"error", err.Error(), nil}
	}
	return APIResponse{"success", "Device connected", nil}
}

func (a *App) DisconnectDevice() {
	err := a.comm.Disconnect()
	if err != nil {
		return
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
		motor := backend.NewMotor(motor_detial.ID, motor_detial, &a.comm)

		a.motors[fmt.Sprintf("MOTOR%d", motor.ID)] = motor
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
		Description string `json:"description"`
	}

	var saveList []FlatMotorConfig

	for _, m := range a.motors {
		saveList = append(saveList, FlatMotorConfig{
			ID:          m.ID,
			Name:        m.Config.Name,
			DIR:         m.Config.DIR,
			Speed:       m.Config.Speed,
			Resolution:  m.Config.Resolution,
			Unit:        m.Config.Unit,
			CWName:      m.Config.CWName, // 如果 MoonsMotor 结构体里有这些字段则 m.CWName
			CCWName:     m.Config.CCWName,
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
	for i := 0; i < 10; i++ {
		testMsg := fmt.Sprintf("motor%d", i)
		respBytes, err := a.comm.Send([]byte(testMsg))

		if err != nil {
			fmt.Printf("节点 %d 未响应: %v\n", i, err)
			continue
		}

		respStr := string(bytes.TrimSpace(respBytes))
		target := fmt.Sprintf("%dOK", i)

		if respStr == target {

			motorKey := fmt.Sprintf("MOTOR%d", i)
			fmt.Println("正在初始化电机", motorKey)
			// 如果电机已存在，则跳过
			if _, exists := a.motors[motorKey]; exists {
				fmt.Printf("节点 %d 已在列表中，跳过搜索\n", i)
				//辅助使能该电机
				a.MotorEnable(i, true)
				continue
			}

			// 新增电机实例
			fmt.Printf("找到新设备： %d 响应: %s\n", i, string(respBytes))

			motorDetial := backend.MotorConfig{
				ID:          i,
				Name:        motorKey,
				Unit:        "mm",
				DIR:         1,
				Resolution:  1000,
				Speed:       1,
				CCWName:     "CW",
				CWName:      "CCW",
				Description: "",
			}

			// 发送给前端通知增加卡片
			runtime.EventsEmit(a.ctx, "find_motor", motorDetial)

			// 初始化后端电机实例
			motor := backend.NewMotor(i, motorDetial, &a.comm)
			a.motors[motorKey] = motor

			// 尝试使能
			motor.Enable(true)

			// 发现新设备后同步保存到本地 JSON
			a.SaveMotorsToLocal()
		}
	}
	return APIResponse{"success", "Motors search completed", nil}
}

// 统一控制接口：根据 ID 寻找电机并执行使能
func (a *App) MotorEnable(id int, enable bool) APIResponse {
	fmt.Println(id, enable)
	key := fmt.Sprintf("MOTOR%d", id)
	if motor, ok := a.motors[key]; ok {
		return APIResponse{"success", "Motor enabled", motor.Enable(enable)}
	}
	return APIResponse{"error", "Motor err", nil}
}

// 统一控制接口：根据 ID 停止
func (a *App) MotorStop(id int) APIResponse {

	return APIResponse{"error", "Motor err", nil}
}

// 统一控制接口：根据 ID 相对运动
func (a *App) MotorMoveRelative(id int, steps int) string {
	key := fmt.Sprintf("motor%d", id)
	if motor, ok := a.motors[key]; ok {
		err := motor.MoveRelative(steps)
		if err != nil {
			return err.Error()
		}
		return "success"
	}
	return "device not found"
}

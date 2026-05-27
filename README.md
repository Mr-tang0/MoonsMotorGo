# PIMS 位移台自由组态控制软件

基于 Wails + Vue3 + Go 开发的位移台控制软件，支持 Modbus RTU 和 SCL 协议，提供直观的设备管理和运动控制界面。

## 功能特性

- **设备管理**
  - 串口自动扫描和连接
  - 电机设备自动搜索（地址 1-32）
  - 手动添加/删除电机设备
  - 电机参数配置持久化存储

- **运动控制**
  - 电机使能/去使能
  - 相对运动（CW/CCW方向）
  - 紧急停止
  - 位置归零

- **实时监控**
  - 位置实时显示
  - 错误状态监控（过温、通讯错误、限位等）
  - 运动状态追踪

- **参数配置**
  - 设备 ID（1-31）
  - 运行速度
  - 脉冲分辨率
  - 位置单位（mm/pulse/deg）
  - 通讯协议选择（Modbus RTU/SCL）

## 技术栈

- **后端**: Go 1.23 + Wails 2.11.0
- **前端**: Vue 3 + TypeScript + Vite
- **UI**: Modern CSS3 + Vuedraggable

## 开发环境

### 前置依赖

- Go 1.23+
- Node.js 18+
- Wails CLI

### 安装 Wails

```bash
go install github.com/wailsapp/wails/v2/cmd/wails@latest
```

## 快速开始

### 开发模式

```bash
cd MoonsMotorGo
wails dev
```

### 构建生产版本

```bash
cd MoonsMotorGo
wails build
```

### 构建平台特定版本

```bash
# Windows
wails build -platform windows/amd64

# macOS
wails build -platform darwin/amd64

# Linux
wails build -platform linux/amd64
```

## 项目结构

```
MoonsMotorGo/
├── backend/                 # Go 后端代码
│   ├── MoonsMotor.go        # 电机控制核心逻辑
│   └── communicator.go      # 串口通讯模块
├── frontend/                # Vue 前端代码
│   ├── src/
│   │   ├── components/      # Vue 组件
│   │   │   ├── Motor.vue    # 电机卡片组件
│   │   │   └── MotorSet.vue # 电机配置模态框
│   │   ├── App.vue          # 主应用组件
│   │   ├── main.js          # 入口文件
│   │   └── style.css        # 全局样式
│   └── wailsjs/             # Wails 自动生成的绑定代码
├── app.go                   # Wails 应用主逻辑
├── main.go                  # 应用入口
├── wails.json               # Wails 配置文件
├── go.mod                   # Go 依赖管理
└── package.json             # 前端依赖管理
```

## 使用说明

1. **连接设备**
   - 点击「搜索串口」扫描可用串口
   - 选择串口和波特率（默认9600）
   - 点击「建立连接」

2. **搜索电机**
   - 连接成功后点击「搜索设备」
   - 软件会自动扫描地址 1-32 的电机设备

3. **控制电机**
   - 点击电机卡片的「使能」按钮激活电机
   - 输入移动距离，点击 CW/CCW 按钮控制方向
   - 点击「停止」可紧急停止运动

4. **配置参数**
   - 点击「参数配置」打开配置面板
   - 修改设备 ID、名称、速度等参数
   - 点击「确认保存」应用配置

## 支持的设备

- 鸣志 MOONS STF05-4XU 系列位移台
- 支持 Modbus RTU 协议的通用伺服驱动器
- 支持 SCL ASCII 协议的驱动器

## 配置文件

配置文件保存在用户目录下：
```
Windows: %USERPROFILE%\Tang\MOONS\config.json
```

## License

MIT License

## 开发者

Mr_Tang <3159690335@qq.com>
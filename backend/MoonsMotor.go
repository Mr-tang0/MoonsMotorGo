package backend

import (
	"encoding/binary"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"
)

// modbus 控制字
const (
	RegControlWord = 0x001F // 控制字 (Enable/Disable)
	RegTargetSpeed = 0x0022 // 目标速度
	RegMoveRel     = 0x0040 // 相对位移 (通常是双字 32bit)
	RegAction      = 0x0007 // 执行指令
	RegErrorCode   = 0x0001 // 错误代码
)

type MotorConfig struct {
	ID          int     `json:"id"`
	Name        string  `json:"name"`
	Unit        string  `json:"unit"`
	Description string  `json:"description"`
	Speed       float32 `json:"speed"`
	Resolution  int     `json:"resolution"`
	CWName      string  `json:"cwName"`
	CCWName     string  `json:"ccwName"`
	Mode        string  `json:"mode"`
	NewID       int     `json:"newID"`
}

type MotorError struct {
	PositionError bool `json:"positionError"`
	Overheat      bool `json:"overheat"`
	CommError     bool `json:"commError"`
	LimitCW       bool `json:"limitCW"`
	LimitCCW      bool `json:"limitCCW"`
	OtherError    int  `json:"otherError"`
}

type MoonsMotor struct {
	Config MotorConfig `json:"config"`
	Error  MotorError  `json:"error"`

	Position float32             `json:"position"`
	Zero     float32             `json:"zero"`
	Enabled  bool                `json:"enabled"`
	Comm     *SerialCommunicator `json:"-"`
}

var MotorSCLAddress = map[int]string{
	0:  "0",
	1:  "1",
	2:  "2",
	3:  "3",
	4:  "4",
	5:  "5",
	6:  "6",
	7:  "7",
	8:  "8",
	9:  "9",
	10: ":",
	11: ";",
	12: "<",
	13: "=",
	14: ">",
	15: "?",
	16: "@",
	17: "!",
	18: "\"",
	19: "#",
	20: "$",
	21: "%",
	22: "&",
	23: "'",
	24: "(",
	25: ")",
	26: "*",
	27: "+",
	28: ",",
	29: "-",
	30: ".",
	31: "/",
	32: "0",
}

func NewMotor(config MotorConfig, comm *SerialCommunicator) MoonsMotor {
	return MoonsMotor{
		Config: config,
		Error:  MotorError{},
		Comm:   comm,
	}
}

func (m *MoonsMotor) Enable(enable bool) error {
	const RegOpcode = 0x007C // 操作码寄存器物理地址
	const OpcodeAR = 0x00BA  // SCL指令 AR 对应的操作码 (Alarm Reset)

	switch m.Config.Mode {
	case "modbus":
		var opcode uint16
		if enable {
			opcode = 0x009F // ME (Enable)
		} else {
			opcode = 0x009E // MD (Disable)
		}

		// 1. 发送使能/去使能指令
		_, err := m.Comm.SendModbus(byte(m.Config.ID), 0x06, RegOpcode, opcode)
		if err != nil {
			return fmt.Errorf("modbus enable/disable error: %v", err)
		}

		m.Enabled = enable

		// 2. 如果是使能操作，成功后紧接着发送清除报警指令 AR
		if m.Enabled {
			// 稍微延迟 10-20ms 确保驱动器已处理完使能指令（可选，视串口稳定性而定）
			time.Sleep(20 * time.Millisecond)

			_, err = m.Comm.SendModbus(byte(m.Config.ID), 0x06, RegOpcode, OpcodeAR)
			if err != nil {
				return fmt.Errorf("modbus clear alarm error: %v", err)
			}
			fmt.Printf("电机 %d Modbus 使能并清除报警成功\n", m.Config.ID)

			speed, _ := m.GetSpeed()

			if m.Config.Speed != speed {
				m.SetSpeed(float32(m.Config.Speed))
			}
		} else {
			fmt.Printf("电机 %d Modbus 去使能成功\n", m.Config.ID)
		}

		return nil

	case "scl":
		if enable {
			// SCL 模式下可以连续发送 ME 和 AR
			cmdME := fmt.Sprintf("%sME", MotorSCLAddress[m.Config.ID])
			resp, err := m.Comm.SendString(cmdME)
			if err != nil {
				return err
			}

			m.Enabled = enable

			// 稍微延迟 10-20ms 确保驱动器已处理完使能指令（可选，视串口稳定性而定）
			time.Sleep(20 * time.Millisecond)

			cmdAR := fmt.Sprintf("%sAR", MotorSCLAddress[m.Config.ID])
			resp, err = m.Comm.SendString(cmdAR)
			if err != nil {
				return fmt.Errorf("motor clear alarm error: %v", err)
			}
			fmt.Printf("电机 %d SCL 使能并清除报警成功\n", m.Config.ID)

			speed, _ := m.GetSpeed()

			if m.Config.Speed != speed {
				m.SetSpeed(float32(m.Config.Speed))
			}

			fmt.Printf("电机 %d Enable响应: %s\n", m.Config.ID, string(resp))
		} else {
			cmd := fmt.Sprintf("%sMD", MotorSCLAddress[m.Config.ID])
			resp, err := m.Comm.SendString(cmd)
			if err != nil {
				return err
			}

			m.Enabled = enable
			fmt.Printf("电机 %d Disable响应: %s\n", m.Config.ID, string(resp))
		}
		return nil

	default:
		return fmt.Errorf("motor mode %s not supported", m.Config.Mode)
	}
}

func (m *MoonsMotor) MoveRelative(length float32) error {
	steps := int32(length * float32(m.Config.Resolution))

	switch m.Config.Mode {
	case "modbus":
		slaveID := byte(m.Config.ID)

		// 1. 写入距离 (DI) -> 寄存器 40031 (0x001E)
		// 构造 32 位数据的两个 16 位字
		highPart := uint16(uint32(steps) >> 16)
		lowPart := uint16(uint32(steps) & 0xFFFF)

		// 连续写入两个寄存器 (40031, 40032)
		// 注意：这里需要你的 SendModbus 支持写入多个寄存器，
		// 如果你的库目前只支持 0x06，可以分两次写，但推荐扩展 0x10。
		_, err := m.Comm.SendModbus(slaveID, 0x06, 0x001E, highPart)
		if err != nil {
			return err
		}
		_, err = m.Comm.SendModbus(slaveID, 0x06, 0x001F, lowPart)
		if err != nil {
			return err
		}

		// 2. 写入操作码触发运动 (FL) -> 寄存器 40125 (0x007C)
		const RegOpcode = 0x007C
		const OpcodeFL = 0x0066 // 相对运动 [cite: 335]
		_, err = m.Comm.SendModbus(slaveID, 0x06, RegOpcode, OpcodeFL)
		if err != nil {
			return fmt.Errorf("modbus move error: %v", err)
		}

		fmt.Printf("电机 %d STF 相对运动指令已发送: %d 脉冲\n", m.Config.ID, steps)
		return nil

	case "scl":
		cmd := fmt.Sprintf("%sFL%d", MotorSCLAddress[m.Config.ID], steps)
		resp, err := m.Comm.SendString(cmd)
		if err != nil {
			return err
		}
		fmt.Printf("电机 %d MoveRelative响应: %s\n", m.Config.ID, string(resp))
		return nil
	default:
		return fmt.Errorf("motor mode %s not supported", m.Config.Mode)
	}
}

func (m *MoonsMotor) Stop() error {

	switch m.Config.Mode {
	case "modbus":
		// 定义操作码寄存器物理地址 (40125 -> 0x007C) [cite: 390]
		const RegOpcode = 0x007C
		// 定义停止操作码 (SK 指令 -> 0x00E1)
		const OpcodeSK = 0x00E1

		// 使用功能码 0x06 向寄存器写入停止指令
		_, err := m.Comm.SendModbus(byte(m.Config.ID), 0x06, RegOpcode, OpcodeSK)
		if err != nil {
			return fmt.Errorf("modbus stop error: %v", err)
		}

		fmt.Printf("电机 %d Modbus Stop 成功 (写入 Opcode: 0x%X)\n", m.Config.ID, OpcodeSK)
		return nil
	case "scl":
		cmd := fmt.Sprintf("%sSK", MotorSCLAddress[m.Config.ID])
		resp, err := m.Comm.SendString(cmd)
		if err != nil {
			return err
		}
		fmt.Printf("电机 %d Stop响应: %s\n", m.Config.ID, string(resp))
		return nil
	default:
		return fmt.Errorf("motor mode %s not supported", m.Config.Mode)
	}

}

func (m *MoonsMotor) SetSpeed(speed float32) error {

	switch m.Config.Mode {
	case "modbus":
		speedRPS := uint16(240 * speed * float32(m.Config.Resolution) / 20000)
		fmt.Printf("电机 %d 设置速度: %d RPS\n", m.Config.ID, speedRPS)
		slaveID := byte(m.Config.ID)
		// 1. 写入速度值到 VE 寄存器 (40030 -> 0x001D)
		const RegSpeed = 0x001D
		_, err := m.Comm.SendModbus(slaveID, 0x06, RegSpeed, speedRPS)
		if err != nil {
			return fmt.Errorf("modbus set speed error: %v", err)
		}

		// 2. 执行 SA (Save to NV) 操作码 (40125 -> 0x007C, Value -> 0x0093)
		// 注意：如果只是临时改变运行速度，不需要每次都执行 SA，频繁写入 Flash 会缩短寿命。
		// 这里根据您的 SCL 逻辑补全 SA 操作。
		const RegOpcode = 0x007C
		const OpcodeSA = 0x0093
		_, err = m.Comm.SendModbus(slaveID, 0x06, RegOpcode, OpcodeSA)
		if err != nil {
			return fmt.Errorf("modbus save speed (SA) error: %v", err)
		}

		fmt.Printf("电机 %d Modbus速度已设置: %d steps/s 并保存\n", m.Config.ID, speedRPS)
		return nil

	case "scl":
		// speedRPS := uint16(240 * speed * float32(m.Config.Resolution) / 20000)
		// fmt.Printf("电机 %d 设置速度: %d RPS\n", m.Config.ID, speedRPS)
		speed = speed * float32(m.Config.Resolution) / 20000

		cmd := fmt.Sprintf("%sVE%f", MotorSCLAddress[m.Config.ID], speed)
		resp, err := m.Comm.SendString(cmd)
		if err != nil {
			return err
		}
		fmt.Printf("电机 %d VE 响应: %s->%s\n", m.Config.ID, cmd, string(resp))

		cmd = fmt.Sprintf("%sSA", MotorSCLAddress[m.Config.ID])
		resp, err = m.Comm.SendString(cmd)
		if err != nil {
			return err
		}
		fmt.Printf("电机 %d SA 响应: %s->%s\n", m.Config.ID, cmd, string(resp))
		return nil
	default:
		return fmt.Errorf("motor mode %s not supported", m.Config.Mode)
	}
}

func (m *MoonsMotor) GetSpeed() (float32, error) {
	switch m.Config.Mode {
	case "modbus":
		slaveID := byte(m.Config.ID)
		const RegSpeed = 0x001D // 40030 的物理地址

		// 1. 读取 VE 寄存器，长度为 1
		resp, err := m.Comm.SendModbus(slaveID, 0x03, RegSpeed, 1)
		if err != nil {
			return 0, fmt.Errorf("modbus get speed error: %v", err)
		}
		if len(resp) < 5 {
			return 0, fmt.Errorf("invalid response length")
		}

		// 2. 解析 16 位速度值 (BigEndian)
		speedRPS := binary.BigEndian.Uint16(resp[3:5])

		// 3. 根据设置公式逆向计算:
		// speedRPS = 240 * speed * Resolution / 20000
		// => speed = (speedRPS * 20000) / (240 * Resolution)
		if m.Config.Resolution == 0 {
			return 0, fmt.Errorf("motor resolution cannot be zero")
		}

		speed := (float32(speedRPS) * 20000.0) / (240.0 * float32(m.Config.Resolution))

		// fmt.Printf("电机 %d 当前读取速度: %d RPS (换算值: %f)\n", m.Config.ID, speedRPS, speed)
		return speed, nil

	case "scl":
		// SCL 模式发送 "IDVE" 查询
		cmd := fmt.Sprintf("%sVE", MotorSCLAddress[m.Config.ID])
		resp, err := m.Comm.SendString(cmd)
		if err != nil {
			return 0, err
		}

		// 注意：SCL 返回通常是字符串，如 "VE=1000"，需要根据实际情况解析
		fmt.Printf("电机 %d GetSpeed SCL 响应: %s\n", m.Config.ID, string(resp))
		speedRPS, err := strconv.ParseFloat(string(resp), 32)
		if err != nil {
			return 0, err
		}
		// 2. 根据设置公式逆向计算:
		// speedRPS = 240 * speed * Resolution / 20000
		// => speed = (speedRPS * 20000) / (240 * Resolution)
		if m.Config.Resolution == 0 {
			return 0, fmt.Errorf("motor resolution cannot be zero")
		}
		speed := (float32(speedRPS) * 20000.0) / (240.0 * float32(m.Config.Resolution))

		return speed, nil // 此处暂返回0，需增加字符串解析逻辑

	default:
		return 0, fmt.Errorf("motor mode %s not supported", m.Config.Mode)
	}
}

func (m *MoonsMotor) SetID(oldID, newID int) error {
	switch m.Config.Mode {
	case "modbus":

		return nil
	case "scl":
		cmd := fmt.Sprintf("%sDA%s", MotorSCLAddress[oldID], MotorSCLAddress[newID])
		resp, err := m.Comm.SendString(cmd)
		if err != nil {
			return err
		}
		fmt.Printf("电机 %d SetID响应: %s\n", oldID, string(resp))
		return nil
	default:
		return fmt.Errorf("motor mode %s not supported", m.Config.Mode)
	}
}

// GetPosition 获取电机当前绝对位置 (手册对应 IP 指令)
func (m *MoonsMotor) GetPosition() (float32, error) {
	switch m.Config.Mode {
	case "modbus":
		// 读取寄存器 40007-40008 (物理地址 0x0006)，长度为 2 个寄存器 (4字节)
		resp, err := m.Comm.SendModbus(byte(m.Config.ID), 0x03, 0x0006, 2)
		if err != nil {
			return 0, err
		}
		// Modbus 响应格式: [Addr, Func, ByteCount, Data..., CRC_L, CRC_H]
		// 数据在索引 3 开始 [cite: 276]
		if len(resp) < 7 {
			return 0, fmt.Errorf("invalid response length")
		}
		// 鸣志默认使用 BigEndian [cite: 313]
		position := int32(binary.BigEndian.Uint32(resp[3:7]))

		// fmt.Printf("电机 %d IP响应: %d\n", m.Config.ID, position)

		m.Position = float32(position) / float32(m.Config.Resolution) // 更新结构体状态
		return m.Position, nil

	case "scl":
		cmd := fmt.Sprintf("%sIP", MotorSCLAddress[m.Config.ID])
		resp, err := m.Comm.SendString(cmd)
		if err != nil {
			return 0, err
		}
		// fmt.Printf("电机 %d IP原始响应: %s\n", m.Config.ID, strings.TrimSpace(resp))

		// 1. 编译正则表达式，匹配 "=" 后面的数字（支持负数或正数）
		re := regexp.MustCompile(`=\s*(-?\d+)`)
		matches := re.FindStringSubmatch(resp)

		// matches[0] 是匹配到的整体（如 "=0"）
		// matches[1] 是括号里捕获的数字部分（如 "0"）
		if len(matches) < 2 {
			return 0, fmt.Errorf("no number found after '=' in response: %s", resp)
		}

		// 2. 转换成整型
		position, err := strconv.Atoi(matches[1])
		if err != nil {
			return 0, fmt.Errorf("failed to convert %s to int: %v", matches[1], err)
		}

		m.Position = float32(position) / float32(m.Config.Resolution) // 更新结构体状态

		return m.Position, nil
	default:
		return 0, fmt.Errorf("motor mode %s not supported", m.Config.Mode)
	}
}

func (m *MoonsMotor) SetHome() error {
	m.Zero = m.Position
	return nil
	// switch m.Config.Mode {
	// case "modbus":
	// 	// 40125 寄存器 (0x0098) 是操作码入口
	// 	const RegOpcode = 0x0098

	// 	// 写入指令，默认会将当前位置设为 0
	// 	_, err := m.Comm.SendModbus(byte(m.Config.ID), 0x06, RegOpcode, 0x0000)
	// 	if err != nil {
	// 		return fmt.Errorf("modbus set home error: %v", err)
	// 	}

	// 	// 同步更新本地状态
	// 	m.zero = m.Position
	// 	fmt.Printf("电机 %d 已成功设为原点 (0点)\n", m.Config.ID)
	// 	return nil

	// case "scl":
	// 	// SCL 模式直接发送 EP0
	// 	cmd := fmt.Sprintf("%dEP0\n", m.Config.ID)
	// 	_, err := m.Comm.SendString(cmd)
	// 	return err

	// default:
	// 	return fmt.Errorf("unsupported mode")
	// }
}

func (m *MoonsMotor) GetError() error {
	switch m.Config.Mode {
	case "modbus":
		// 读取报警代码寄存器 40001 (物理地址 0x0000)，长度 1
		resp, err := m.Comm.SendModbus(byte(m.Config.ID), 0x03, 0x0000, 1)
		if err != nil {
			return err
		}
		if len(resp) < 5 {
			return fmt.Errorf("invalid response length")
		}

		// 获取 16 位错误码
		errCode := binary.BigEndian.Uint16(resp[3:5])

		// 根据手册附录 6 解析位信息
		m.Error.PositionError = (errCode & (1 << 0)) != 0 // Bit 0: 位置误差超限
		m.Error.LimitCCW = (errCode & (1 << 1)) != 0      // Bit 1: CCW方向禁止限位
		m.Error.LimitCW = (errCode & (1 << 2)) != 0       // Bit 2: CW方向禁止限位
		m.Error.Overheat = (errCode & (1 << 3)) != 0      // Bit 3: 驱动器过温
		m.Error.CommError = (errCode & (1 << 4)) != 0     // Bit 4: 通讯错误
		m.Error.OtherError = int((errCode >> 5) & 0x7FF)  // Bit 5-15: 其他错误代码

		// fmt.Printf("电机 %d Modbus 错误代码: 0x%04X\n", m.Config.ID, errCode)
		return nil
	case "scl":
		cmd := fmt.Sprintf("%sAL", MotorSCLAddress[m.Config.ID])
		resp, err := m.Comm.SendString(cmd)
		if err != nil {
			return err
		}

		// 解析 SCL 响应，格式如 "AL=0" 或 "AL=123"
		re := regexp.MustCompile(`=\s*(-?\d+)`)
		matches := re.FindStringSubmatch(resp)

		if len(matches) < 2 {
			return fmt.Errorf("no error code found in response: %s", resp)
		}

		errCode, err := strconv.Atoi(matches[1])
		if err != nil {
			return fmt.Errorf("failed to parse error code: %v", err)
		}

		// 根据 SCL 手册解析错误码
		m.Error.PositionError = (errCode & (1 << 0)) != 0 // Bit 0: 位置误差超限
		m.Error.LimitCCW = (errCode & (1 << 1)) != 0      // Bit 1: CCW方向禁止限位
		m.Error.LimitCW = (errCode & (1 << 2)) != 0       // Bit 2: CW方向禁止限位
		m.Error.Overheat = (errCode & (1 << 3)) != 0      // Bit 3: 驱动器过温
		m.Error.CommError = (errCode & (1 << 4)) != 0     // Bit 4: 通讯错误
		m.Error.OtherError = (errCode >> 5) & 0x7FF       // Bit 5-15: 其他错误代码

		// fmt.Printf("电机 %d SCL 错误代码: %d\n", m.Config.ID, errCode)
		return nil
	default:
		return fmt.Errorf("motor mode %s not supported", m.Config.Mode)
	}
}

// GetMotionStatus 获取电机当前的运动状态
func (m *MoonsMotor) GetMotionStatus() (bool, error) {
	switch m.Config.Mode {
	case "modbus":
		return m.GetMotionStatusModbus()
	case "scl":
		return m.GetMotionStatusSCL()
	default:
		return false, fmt.Errorf("motor mode %s not supported", m.Config.Mode)
	}
}

func (m *MoonsMotor) GetMotionStatusModbus() (bool, error) {
	// 读取寄存器 40002 (物理地址 0x0001)
	resp, err := m.Comm.SendModbus(byte(m.Config.ID), 0x03, 0x0001, 1)
	if err != nil {
		return false, err
	}

	if len(resp) < 5 {
		return false, fmt.Errorf("invalid response length")
	}

	// 解析状态字
	statusCode := binary.BigEndian.Uint16(resp[3:5])

	// 1. 更新使能状态
	// m.Config.Enable = (statusCode & (1 << 0)) != 0

	// 2. 获取是否正在运动 (Bit 4)
	isMoving := (statusCode & (1 << 4)) != 0

	// 3. 获取是否到位 (Bit 3)
	// inPosition := (statusCode & (1 << 3)) != 0

	// fmt.Printf("电机 %d 状态: 运动中=%v, 已到位=%v\n", m.Config.ID, isMoving, inPosition)

	return isMoving, nil
}

func (m *MoonsMotor) GetMotionStatusSCL() (bool, error) {
	cmd := fmt.Sprintf("%sRS", MotorSCLAddress[m.Config.ID])
	resp, err := m.Comm.SendString(cmd)
	if err != nil {
		return false, err
	}
	// 解析响应，判断是否正在运动
	isMoving := strings.Contains(resp, "M")

	return isMoving, nil
}

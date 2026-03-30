package backend

import (
	"encoding/binary"
	"fmt"
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
	ID          int    `json:"id"`
	Name        string `json:"name"`
	Unit        string `json:"unit"`
	Description string `json:"description"`
	DIR         int    `json:"dir"`
	Speed       int    `json:"speed"`
	Resolution  int    `json:"resolution"`
	CWName      string `json:"cwName"`
	CCWName     string `json:"ccwName"`
	Mode        string `json:"mode"`
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
	Enabled  bool                `json:"enabled"`
	Comm     *SerialCommunicator `json:"-"`
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
		if enable {
			// 稍微延迟 10-20ms 确保驱动器已处理完使能指令（可选，视串口稳定性而定）
			time.Sleep(20 * time.Millisecond)

			_, err = m.Comm.SendModbus(byte(m.Config.ID), 0x06, RegOpcode, OpcodeAR)
			if err != nil {
				return fmt.Errorf("modbus clear alarm error: %v", err)
			}
			fmt.Printf("电机 %d Modbus 使能并清除报警成功\n", m.Config.ID)
		} else {
			fmt.Printf("电机 %d Modbus 去使能成功\n", m.Config.ID)
		}

		return nil

	case "scl":
		if enable {
			// SCL 模式下可以连续发送 ME 和 AR
			cmdME := fmt.Sprintf("%dME\n", m.Config.ID)
			resp, err := m.Comm.SendString(cmdME)
			if err != nil {
				return err
			}

			cmdAR := fmt.Sprintf("%dAR\n", m.Config.ID)
			_, err = m.Comm.SendString(cmdAR)
			if err != nil {
				return err
			}

			fmt.Printf("电机 %d Enable及AR响应: %s\n", m.Config.ID, string(resp))
		} else {
			cmd := fmt.Sprintf("%dMD\n", m.Config.ID)
			resp, err := m.Comm.SendString(cmd)
			if err != nil {
				return err
			}
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
		cmd := fmt.Sprintf("%dFL%d\n", m.Config.ID, steps)
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
		cmd := fmt.Sprintf("%dSK\n", m.Config.ID)
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

		return nil
	case "scl":
		Speed := int(speed * float32(m.Config.Resolution))
		cmd := fmt.Sprintf("%dVE%d\n", m.Config.ID, Speed)
		resp, err := m.Comm.SendString(cmd)
		if err != nil {
			return err
		}

		cmd = fmt.Sprintf("%dSA\n", m.Config.ID)
		resp, err = m.Comm.SendString(cmd)
		if err != nil {
			return err
		}
		fmt.Printf("电机 %d SetSpeed响应: %s\n", m.Config.ID, string(resp))
		return nil
	default:
		return fmt.Errorf("motor mode %s not supported", m.Config.Mode)
	}
}

func (m *MoonsMotor) SetID(id int) error {
	switch m.Config.Mode {
	case "modbus":

		return nil
	case "scl":
		cmd := fmt.Sprintf("%dDA%d\n", m.Config.ID, id)
		resp, err := m.Comm.SendString(cmd)
		if err != nil {
			return err
		}
		fmt.Printf("电机 %d GetError响应: %s\n", m.Config.ID, string(resp))
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
		m.Position = float32(position) / float32(m.Config.Resolution) // 更新结构体状态
		return m.Position, nil

	case "scl":
		cmd := fmt.Sprintf("%dIP\n", m.Config.ID)
		resp, err := m.Comm.SendString(cmd)
		if err != nil {
			return 0, err
		}
		// 注意：此处需要解析字符串 resp 中的数值，逻辑视具体返回格式而定
		fmt.Printf("电机 %d IP响应: %s\n", m.Config.ID, string(resp))
		return 0, nil
	default:
		return 0, fmt.Errorf("motor mode %s not supported", m.Config.Mode)
	}
}

func (m *MoonsMotor) SetHome() error {
	switch m.Config.Mode {
	case "modbus":
		// 40125 寄存器 (0x007C) 是操作码入口
		const RegOpcode = 0x007C
		// 0x00A5 对应 SCL 的 SP 指令 (Set Position)
		const OpcodeEP = 0x00A5

		// 写入指令，默认会将当前位置设为 0
		_, err := m.Comm.SendModbus(byte(m.Config.ID), 0x06, RegOpcode, OpcodeEP)
		if err != nil {
			return fmt.Errorf("modbus set home error: %v", err)
		}

		// 同步更新本地状态
		m.Position = 0
		fmt.Printf("电机 %d 已成功设为原点 (0点)\n", m.Config.ID)
		return nil

	case "scl":
		// SCL 模式直接发送 EP0
		cmd := fmt.Sprintf("%dEP0\n", m.Config.ID)
		_, err := m.Comm.SendString(cmd)
		return err

	default:
		return fmt.Errorf("unsupported mode")
	}
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
		cmd := fmt.Sprintf("%dAL\n", m.Config.ID)
		resp, err := m.Comm.SendString(cmd)
		if err != nil {
			return err
		}
		fmt.Printf("电机 %d GetError响应: %s\n", m.Config.ID, string(resp))
		return nil
	default:
		return fmt.Errorf("motor mode %s not supported", m.Config.Mode)
	}
}

// GetMotionStatus 获取电机当前的运动状态
func (m *MoonsMotor) GetMotionStatus() (bool, error) {
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

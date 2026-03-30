package backend

import (
	"bytes"
	"fmt"
	"sync"
	"time"

	"go.bug.st/serial"
)

// SerialCommunicator 串口通讯实现
type SerialCommunicator struct {
	Port     string
	BaudRate int

	// 重试配置
	MaxRetries int           // 最大重试次数
	Timeout    time.Duration // 总超时时间（等待回复）

	mu          sync.Mutex
	isConnected bool
	mode        *serial.Mode
	portConn    serial.Port // 实际的串口连接对象
}

// ----------------------------------------------------------------
// 1. 基础连接管理
// ----------------------------------------------------------------

// ListAvailablePorts 遍历所有可用的串口设备
func (s *SerialCommunicator) ListAvailablePorts() ([]string, error) {
	ports, err := serial.GetPortsList()
	if err != nil {
		return nil, fmt.Errorf("failed to enumerate ports: %v", err)
	}
	return ports, nil
}

// Connect 连接具体设备
func (s *SerialCommunicator) Connect(comPort string, baudRate int) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.isConnected {
		return fmt.Errorf("already connected to %s", s.Port)
	}

	s.mode = &serial.Mode{
		BaudRate: baudRate,
		DataBits: 8,
		Parity:   serial.NoParity,
		StopBits: serial.OneStopBit,
	}

	p, err := serial.Open(comPort, s.mode)
	if err != nil {
		return fmt.Errorf("failed to open port %s: %v", comPort, err)
	}

	s.portConn = p
	s.BaudRate = baudRate
	s.Port = comPort
	s.isConnected = true

	fmt.Printf("Successfully connected to %s at %d baud\n", s.Port, s.BaudRate)
	return nil
}

// Disconnect 断开连接
func (s *SerialCommunicator) Disconnect() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.portConn != nil {
		s.portConn.Close()
	}
	s.isConnected = false
	return nil
}

func (s *SerialCommunicator) IsConnected() bool {
	return s.isConnected
}

// ----------------------------------------------------------------
// 2. 字符串通讯逻辑 (String Mode)
// ----------------------------------------------------------------

// SendString 发送字符串（通常以 \n 结尾）并等待含 \n 的回复
func (s *SerialCommunicator) SendString(data string) (string, error) {
	resp, err := s.executeTransfer([]byte(data), false)
	if err != nil {
		return "", err
	}
	return string(resp), nil
}

// readUntilDelimiter 持续读取直至找到换行符
func (s *SerialCommunicator) readUntilDelimiter() ([]byte, error) {
	var fullFrame []byte
	startTime := time.Now()
	tmpBuf := make([]byte, 128)

	for {
		if time.Since(startTime) > s.Timeout {
			return nil, fmt.Errorf("string read timeout (%v)", s.Timeout)
		}

		n, err := s.portConn.Read(tmpBuf)
		if err != nil {
			return nil, err
		}

		if n > 0 {
			fullFrame = append(fullFrame, tmpBuf[:n]...)
			if bytes.Contains(tmpBuf[:n], []byte{'\n'}) {
				return fullFrame, nil
			}
		} else {
			time.Sleep(10 * time.Millisecond)
		}
	}
}

// ----------------------------------------------------------------
// 3. Modbus RTU 通讯逻辑 (Modbus Mode)
// ----------------------------------------------------------------

// SendModbus 发送标准 Modbus RTU 请求
// slaveID: 从站地址, funcCode: 功能码, startAddr: 起始地址, count: 数量
func (s *SerialCommunicator) SendModbus(slaveID byte, funcCode byte, startAddr uint16, count uint16) ([]byte, error) {
	// 组装 Modbus 协议帧 (不含CRC)
	req := []byte{
		slaveID,
		funcCode,
		byte(startAddr >> 8), byte(startAddr & 0xFF),
		byte(count >> 8), byte(count & 0xFF),
	}
	// 计算并添加 CRC
	crc := s.CalculateCRC(req)
	req = append(req, byte(crc&0xFF), byte(crc>>8))

	// 执行传输（Modbus 模式）
	return s.executeTransfer(req, true)
}

// readModbusFrame 依靠 CRC 校验和最小帧长判定 Modbus 包结束
func (s *SerialCommunicator) readModbusFrame() ([]byte, error) {
	var fullFrame []byte
	startTime := time.Now()
	tmpBuf := make([]byte, 256)

	for {
		if time.Since(startTime) > s.Timeout {
			return nil, fmt.Errorf("modbus read timeout (%v)", s.Timeout)
		}

		n, err := s.portConn.Read(tmpBuf)
		if err != nil {
			return nil, err
		}

		if n > 0 {
			fullFrame = append(fullFrame, tmpBuf[:n]...)
			// Modbus 响应最短通常为 5 字节
			if len(fullFrame) >= 5 {
				if s.verifyCRC(fullFrame) {
					return fullFrame, nil
				}
			}
		} else {
			// 如果有数据但 CRC 还没过，微等一下可能存在的后续包
			if len(fullFrame) > 0 {
				time.Sleep(20 * time.Millisecond)
			}
			time.Sleep(10 * time.Millisecond)
		}
	}
}

// ----------------------------------------------------------------
// 4. 核心调度与辅助工具
// ----------------------------------------------------------------

// executeTransfer 内部统一调度函数
func (s *SerialCommunicator) executeTransfer(data []byte, isModbus bool) ([]byte, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if !s.isConnected {
		return nil, fmt.Errorf("serial port not connected")
	}

	// 设置底层物理读取超时，防止 Read 永远阻塞
	s.portConn.SetReadTimeout(50 * time.Millisecond)

	var lastErr error
	for i := 0; i <= s.MaxRetries; i++ {
		if i > 0 {
			time.Sleep(100 * time.Millisecond)
		}

		s.portConn.ResetInputBuffer()
		if _, err := s.portConn.Write(data); err != nil {
			lastErr = err
			continue
		}

		var resp []byte
		var err error
		if isModbus {
			resp, err = s.readModbusFrame()
		} else {
			resp, err = s.readUntilDelimiter()
		}

		if err != nil {
			lastErr = err
			continue
		}
		return resp, nil
	}
	return nil, fmt.Errorf("failed after %d retries: %v", s.MaxRetries, lastErr)
}

// CalculateCRC 计算 Modbus CRC16
func (s *SerialCommunicator) CalculateCRC(data []byte) uint16 {
	var crc uint16 = 0xFFFF
	for _, b := range data {
		crc ^= uint16(b)
		for i := 0; i < 8; i++ {
			if (crc & 0x0001) != 0 {
				crc = (crc >> 1) ^ 0xA001
			} else {
				crc >>= 1
			}
		}
	}
	return crc
}

// verifyCRC 验证数据包末尾的 CRC 是否正确
func (s *SerialCommunicator) verifyCRC(data []byte) bool {
	if len(data) < 3 {
		return false
	}
	payload := data[:len(data)-2]
	expected := s.CalculateCRC(payload)
	actual := uint16(data[len(data)-2]) | (uint16(data[len(data)-1]) << 8)
	return expected == actual
}

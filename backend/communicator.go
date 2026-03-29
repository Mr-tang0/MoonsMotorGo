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
	MaxRetries int           // 最大重试次数 n
	Timeout    time.Duration // 等待回复的超时时间

	mu          sync.Mutex
	isConnected bool
	mode        *serial.Mode
	portConn    serial.Port // 实际的串口连接对象
}

func (s *SerialCommunicator) startup() {

}

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

	// 实际尝试打开串口
	p, err := serial.Open(comPort, s.mode)
	if err != nil {
		return fmt.Errorf("failed to open port %s: %v", comPort, err)
	}

	s.portConn = p
	s.BaudRate = baudRate
	s.Port = comPort
	s.isConnected = true

	// p.SetReadTimeout(s.Timeout)

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
	fmt.Println("Disconnected from serial port")
	return nil
}

// Send 发送函数：含精确超时等待及重试逻辑
// Send 发送函数：含重试逻辑及获取完整结束符逻辑
func (s *SerialCommunicator) Send(data []byte) ([]byte, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if !s.isConnected {
		return nil, fmt.Errorf("port not connected")
	}

	// 设置单次系统级 Read 的短超时（用于非阻塞拼帧）
	// 建议设置为较小值，例如 100ms
	s.portConn.SetReadTimeout(100 * time.Millisecond)

	var lastErr error

	for i := 0; i <= s.MaxRetries; i++ {
		if i > 0 {
			// fmt.Printf("Retry %d/%d...\n", i, s.MaxRetries)
			time.Sleep(100 * time.Millisecond)
		}

		// 1. 清空输入缓冲区，确保不读到之前的旧数据
		s.portConn.ResetInputBuffer()

		// 2. 写入数据
		_, err := s.portConn.Write(data)
		if err != nil {
			lastErr = fmt.Errorf("write error: %v", err)
			continue
		}

		// 3. 循环读取直到发现结束符或总超时
		result, err := s.readUntilDelimiter()
		if err != nil {
			lastErr = err
			continue
		}

		// 成功获取完整指令
		return result, nil
	}

	return nil, fmt.Errorf("failed after %d retries. Last error: %v", s.MaxRetries, lastErr)
}

// readUntilDelimiter 内部逻辑：持续读取直至找到结束符
func (s *SerialCommunicator) readUntilDelimiter() ([]byte, error) {
	var fullFrame []byte
	startTime := time.Now()
	tmpBuf := make([]byte, 128)

	for {
		// 检查是否超过了总 Timeout 时间
		if time.Since(startTime) > s.Timeout {
			return nil, fmt.Errorf("timeout: delimiter not found within %v", s.Timeout)
		}

		n, err := s.portConn.Read(tmpBuf)
		if err != nil {
			return nil, fmt.Errorf("read error: %v", err)
		}

		if n > 0 {
			fullFrame = append(fullFrame, tmpBuf[:n]...)

			// 检查最后收到的字节序列中是否包含结束符
			if bytes.Contains(tmpBuf[:n], []byte{'\n'}) {
				return fullFrame, nil
			}
		} else {
			// 如果没读到数据，稍微休息避免占用 CPU 过高
			time.Sleep(10 * time.Millisecond)
		}
	}
}

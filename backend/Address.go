package backend

type RegisterMap struct {
	RE_ErrorCode uint16 // 报警代码寄存器
	RE_Status    uint16 // 状态寄存器
	RE_Position  uint16 // 位置寄存器起始地址 (32位)
	RE_Speed     uint16 // 速度寄存器
	RE_MoveRel   uint16 // 相对位移寄存器
	RE_Opcode    uint16 // 操作码寄存器
}

var (
	RegisterMapSTF05 = RegisterMap{
		RE_ErrorCode: 0x0000,
		RE_Status:    0x0001,
		RE_Position:  0x0006,
		RE_Speed:     0x001D,
		RE_MoveRel:   0x001E,
		RE_Opcode:    0x007C,
	}

	RegisterMapMDX_Plus = RegisterMap{
		RE_ErrorCode: 0x0000,
		RE_Status:    0x0001,
		RE_Position:  0x0006,
		RE_Speed:     0x015C,
		RE_MoveRel:   0x015E,
		RE_Opcode:    0x007C,
	}

	RegisterMapDefault = RegisterMapSTF05
)

const (
	OpcodeME = 0x009F // ME - 使能
	OpcodeMD = 0x009E // MD - 去使能
	OpcodeAR = 0x00BA // AR - 清除报警
	OpcodeFL = 0x0066 // FL - 相对运动
	OpcodeSK = 0x00E1 // SK - 停止
	OpcodeSA = 0x0093 // SA - 保存到NV
)

const (
	FuncReadHoldingRegisters   = 0x03
	FuncWriteSingleRegister    = 0x06
	FuncWriteMultipleRegisters = 0x10
)

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

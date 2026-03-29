package backend

import "fmt"

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
	Overheat   bool `json:"overheat"`
	CommError  bool `json:"commError"`
	LimitCW    bool `json:"limitCW"`
	LimitCCW   bool `json:"limitCCW"`
	OtherError int  `json:"otherError"`
}

type MoonsMotor struct {
	Config MotorConfig `json:"config"`
	Error  MotorError  `json:"error"`

	Position int `json:"position"`

	Comm *SerialCommunicator `json:"-"`
}

func NewMotor(config MotorConfig, comm *SerialCommunicator) MoonsMotor {
	return MoonsMotor{
		Config: config,
		Error:  MotorError{},
		Comm:   comm,
	}
}

func (m *MoonsMotor) Enable(enable bool) error {
	switch m.Config.Mode {
	case "modbus":

		return nil
	case "scl":
		if enable {
			cmd := fmt.Sprintf("%dME\n", m.Config.ID)

			resp, err := m.Comm.Send([]byte(cmd))
			if err != nil {
				return err
			}

			fmt.Printf("电机 %d Enable响应: %s\n", m.Config.ID, string(resp))
		} else {
			cmd := fmt.Sprintf("%dMD\n", m.Config.ID)
			resp, err := m.Comm.Send([]byte(cmd))
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
	switch m.Config.Mode {
	case "modbus":

		return nil
	case "scl":
		steps := int(length * float32(m.Config.Resolution))
		cmd := fmt.Sprintf("%dFL%d\n", m.Config.ID, steps)
		resp, err := m.Comm.Send([]byte(cmd))
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

		return nil
	case "scl":
		cmd := fmt.Sprintf("%dSK\n", m.Config.ID)
		resp, err := m.Comm.Send([]byte(cmd))
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
		resp, err := m.Comm.Send([]byte(cmd))
		if err != nil {
			return err
		}

		cmd = fmt.Sprintf("%dSA\n", m.Config.ID)
		resp, err = m.Comm.Send([]byte(cmd))
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
		resp, err := m.Comm.Send([]byte(cmd))
		if err != nil {
			return err
		}
		fmt.Printf("电机 %d GetError响应: %s\n", m.Config.ID, string(resp))
		return nil
	default:
		return fmt.Errorf("motor mode %s not supported", m.Config.Mode)
	}
}

func (m *MoonsMotor) GetDirction() error {
	switch m.Config.Mode {
	case "modbus":

		return nil
	case "scl":
		cmd := fmt.Sprintf("%dIP\n", m.Config.ID)
		resp, err := m.Comm.Send([]byte(cmd))
		if err != nil {
			return err
		}
		fmt.Printf("电机 %d GetDirction响应: %s\n", m.Config.ID, string(resp))
		return nil
	default:
		return fmt.Errorf("motor mode %s not supported", m.Config.Mode)
	}
}

func (m *MoonsMotor) GetError() error {
	switch m.Config.Mode {
	case "modbus":

		return nil
	case "scl":
		cmd := fmt.Sprintf("%dAL\n", m.Config.ID)
		resp, err := m.Comm.Send([]byte(cmd))
		if err != nil {
			return err
		}
		fmt.Printf("电机 %d GetError响应: %s\n", m.Config.ID, string(resp))
		return nil
	default:
		return fmt.Errorf("motor mode %s not supported", m.Config.Mode)
	}
}

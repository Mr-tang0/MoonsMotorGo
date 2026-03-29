<template>
  <div class="app-layout">
    <aside class="side-nav">
      <div class="logo-area">
        <div class="logo-icon">PIMS</div>
        <h2>位移台控制软件</h2>
      </div>

      <div class="nav-section">
        <label>通讯配置</label>
        <div class="glass-card">
          <div class="custom-select">
            <span>端口:</span>
            <select v-model="selectedPort">
              <option disabled value="">请选择串口</option>
              <option v-for="p in portList" :key="p" :value="p">{{ p }}</option>
            </select>
          </div>
          <div class="custom-select">
            <span>波特率:</span>
            <select v-model="selectedBaud">
              <option value="9600">9600</option>
              <option value="115200">115200</option>
            </select>
          </div>
          <div class="button-group">
            <button class="refresh-btn" @click="refreshPorts">搜索串口</button>
            <button 
              class="main-conn-btn" 
              :class="{ 'connected': isConnected }" 
              @click="toggleConnection"
            >
              {{ isConnected ? '断开连接' : '建立连接' }}
            </button>
            <button class="search-devices-btn" v-if="isConnected" @click="searchMotors">搜索设备</button>
          </div>
        </div>
      </div>

      <div class="system-status">
        <div class="status-item">
          <span class="dot" :class="isConnected ? 'green' : 'gray'"></span>
          {{ isConnected ? '通讯中' : '未连接' }}
        </div>
      </div>
    </aside>

    <main class="content-area">
      <header class="content-header">
        <div class="search-bar">
          <input type="text" v-model="searchQuery" placeholder="搜索位移台 ID..." />
          <button @click="handleSearch">搜索</button>
        </div>
        <div class="stats">
          在线设备: <strong>{{ motors.length }}</strong>
        </div>
      </header>

      <div class="motor-viewport">
        <draggable 
          v-model="motors" 
          item-key="id"
          class="motor-grid"
          :animation="300"
          delay="500"
          :delay-on-touch-only="false"
          ghost-class="ghost-card"
        >
          <template #item="{ element, index }">
            <div class="grid-item">
              <Motor 
                :motor="element"
                @remove="removeMotor(index)"
              />
            </div>
          </template>

          <template #footer>
            <div class="add-placeholder" @click="addMotor">+</div>
          </template>
        </draggable>
      </div>
    </main>
  </div>
</template>

<script setup lang="ts">
import { ref, onMounted, onUnmounted, inject} from 'vue';
import { EventsOn, EventsOff } from '../wailsjs/runtime/runtime'
import Motor from './components/motor.vue';
import draggable from 'vuedraggable';


import { EnumDevices, ConnectDevice, DisconnectDevice, LoadLocalMotors, SaveMotorsToLocal, SearchMotors } from '../wailsjs/go/main/App';

interface MotorItem {
  id: number | string;// 设备ID
  name?: string;// 设备名称
  position?: number | string;// 当前位置
  enable?: boolean;// 是否使能
  unit?: string;// 位置单位
  overheat?: boolean;// 是否过温
  commError?: boolean;// 是否通讯错误
  limitCW?: boolean;// 是否CW限位
  limitCCW?: boolean;// 是否CCW限位
  communicateType?: string; // 通讯方式modbus/Ascii
  cwName?: string;// CW名称
  ccwName?: string;// CCW名称
}

// 状态变量
const portList = ref<string[]>([]);
const selectedPort = ref('');
const selectedBaud = ref('115200');
const isConnected = ref(false);
const searchQuery = ref('');
const motors = ref<MotorItem[]>([]);

let motorIdCounter = 0;

// 1. 刷新串口逻辑
const refreshPorts = async () => {
  try {
    const result = await EnumDevices();
    portList.value = result.data.ports || [];
    if (portList.value.length > 0 && !selectedPort.value) {
      selectedPort.value = portList.value[0];
    }
  } catch (err) {
    console.error("刷新串口失败:", err);
  }
};

// 2. 连接/断开逻辑
const toggleConnection = async () => {
  if (isConnected.value) {
    await DisconnectDevice();
    isConnected.value = false;
  } else {
    if (!selectedPort.value) {
      alert("请先选择一个串口");
      return;
    }
    try {
      const result = await ConnectDevice(selectedPort.value, parseInt(selectedBaud.value));
      if (result.status === "success") {
        isConnected.value = true;
      }
    } catch (err) {
      alert("连接失败: " + err);
    }
  }
};

const searchMotors = async () => { 
  if (!isConnected.value) {
    alert("请先建立连接");
    return;
  }
  await SearchMotors();
};

// 设备管理逻辑
const addMotor = () => {
  motorIdCounter++;
  motors.value.push({
    id: motorIdCounter,
    name: `位移台 ${motorIdCounter}`,
    enable: false
  });
};

// 移除逻辑
const removeMotor = (index: number) => {
  motors.value.splice(index, 1);
};

// 搜索逻辑
const handleSearch = () => {
  console.log("执行搜索:", searchQuery.value);

};



// 初始化
onMounted(async() => {
  refreshPorts(); // 初始化自动刷新一次串口
  
  EventsOn("find_motor", (motor) => {
      console.log("收到 Motor 原始数据:", motor);
      motorIdCounter++;
      motors.value.push({
        id: motor.id,
        name: motor.name,
        position: motor.position,
        enable: motor.enable,
        unit: motor.unit,
        overheat: motor.overheat,
        commError: motor.commError,
        limitCW: motor.limitCW,
        limitCCW: motor.limitCCW,
        communicateType: motor.communicateType,
        cwName: motor.cwName,
        ccwName: motor.ccwName

      });
  })

  await LoadLocalMotors();
});

onUnmounted(async() => {
  // 离开页面时，断开连接
  if (isConnected.value) {
    DisconnectDevice();
  }
  EventsOff("find_motor");
  // await SaveMotorsToLocal();
});

</script>

<style>
:root {
  --primary: #369ce9;
  --primary-light: #4c9aff;
  --bg-main: #f7fafc;
  --bg-side: #103c94;
  --danger: #fa5252;
}

.app-layout {
  display: flex;
  height: 100vh;
  background: var(--bg-main);
  font-family: 'Inter', -apple-system, sans-serif;
  user-select: none;
}

/* 侧边栏 */
.side-nav {
  width: 280px;
  background: var(--bg-side);
  color: white;
  padding: 30px 20px;
  display: flex;
  flex-direction: column;
}

.logo-area {
  display: flex;
  align-items: center;
  gap: 12px;
  margin-bottom: 40px;
}

.logo-icon {
  width: 65px;
  height: 35px;
  background: var(--primary);
  border-radius: 8px;
  display: flex;
  align-items: center;
  justify-content: center;
  font-weight: bold;
}

.nav-section label {
  font-size: 11px;
  text-transform: uppercase;
  color: rgba(255,255,255,0.5);
  letter-spacing: 1px;
  margin-bottom: 10px;
  display: block;
}

.glass-card {
  background: rgba(255,255,255,0.08);
  border-radius: 15px;
  padding: 15px;
  backdrop-filter: blur(10px);
  border: 1px solid rgba(255,255,255,0.1);
}

.custom-select {
  display: flex;
  justify-content: space-between;
  align-items: center;
  margin-bottom: 12px;
  font-size: 14px;
}

.custom-select select {
  background: #0d2d70;
  color: white;
  border: 1px solid rgba(255,255,255,0.2);
  border-radius: 6px;
  padding: 4px 8px;
  width: 140px;
}

.button-group {
  display: flex;
  flex-direction: column;
  gap: 8px;
  margin-top: 10px;
}

.refresh-btn {
  background: rgba(255,255,255,0.1);
  color: white;
  border: 1px solid rgba(255,255,255,0.2);
  padding: 8px;
  border-radius: 8px;
  cursor: pointer;
  font-size: 13px;
}

.refresh-btn:hover { background: rgba(255,255,255,0.2); }

.main-conn-btn {
  width: 100%;
  padding: 12px;
  border-radius: 10px;
  border: none;
  background: var(--primary);
  color: white;
  font-weight: 600;
  cursor: pointer;
}

.main-conn-btn.connected {
  background: var(--danger);
}

.search-devices-btn { 
  width: 100%;
  padding: 12px;
  border-radius: 10px;
  border: none;
  background: var(--primary);
  color: white;
  font-weight: 600;
  cursor: pointer;
}
.search-devices-btn :hover { background: var(--primary-light); }

.system-status {
  margin-top: auto;
  padding-top: 20px;
}

.status-item {
  display: flex;
  align-items: center;
  gap: 8px;
  font-size: 13px;
  color: rgba(255,255,255,0.7);
}

.dot {
  width: 8px;
  height: 8px;
  border-radius: 50%;
}
.dot.green { background: #40c057; box-shadow: 0 0 8px #40c057; }
.dot.gray { background: #868e96; }

/* 内容区 */
.content-area { flex: 1; display: flex; flex-direction: column; }

.content-header {
  padding: 20px 40px;
  display: flex;
  justify-content: space-between;
  align-items: center;
  background: white;
  box-shadow: 0 2px 10px rgba(0,0,0,0.02);
}

.search-bar { display: flex; gap: 10px; }

.search-bar input {
  padding: 8px 15px;
  border-radius: 20px;
  border: 1px solid #edf2f7;
  width: 250px;
}

.search-bar button {
  padding: 8px 20px;
  border-radius: 20px;
  border: none;
  background: var(--primary);
  color: white;
  cursor: pointer;
}

.motor-viewport { flex: 1; padding: 40px; overflow-y: auto; }

.motor-grid {
  display: grid;
  grid-template-columns: repeat(auto-fill, 280px); 
  gap: 30px;
}

.ghost-card { opacity: 0.3; filter: grayscale(1); }

.add-placeholder {
  width: 280px;      
  aspect-ratio: 3 / 4; 
  border: 3px dashed #cbd5e0;
  border-radius: 24px;
  display: flex;
  align-items: center;
  justify-content: center;
  font-size: 50px;
  color: #ccc;
  cursor: pointer;
  transition: all 0.3s ease;
}

.add-placeholder:hover {
  border-color: var(--primary);
  color: var(--primary);
  background: rgba(54, 156, 233, 0.05);
}
</style>
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
              <!-- <option value="115200">115200</option> -->
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
            <button class="automation-btn" v-if="isConnected" @click="toggleAutomation">{{ showAutomation ? '停止' : '开始' }}自动化程序</button>
          </div>
        </div>
      </div>

      <!-- 自动控制模块 -->
      <div v-if="showAutomation&&isConnected" class="nav-section automation-section">
        <label>自动控制</label>
        <div class="glass-card automation-panel">
          <textarea 
            v-model="automationScript" 
            class="script-input"
            placeholder="输入自动化脚本...&#10;&#10;示例：&#10;M1: CW 10&#10;M2: CCW -5&#10;Delay 1000&#10;M1: CW -10"
            rows="6"
          ></textarea>
          <div class="script-buttons">
            <button class="btn-run" @click="runAutomation">{{automationProgress?'停止':'运行'}}</button>
            <button class="btn-notify" @click="OpenScriptFile">打开脚本</button>
            <button class="btn-notify" @click="SaveScriptFile">保存脚本</button>
          </div>
        </div>
      </div>

      <!-- <h6>暂时只提供STF05-4XU适配，请勿连接其他位移台</h6> -->

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
                @configure="openSettings"
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

  <div v-if="isSearching" class="search-overlay">
    <div class="progress-container">
      <h3>正在扫描设备...</h3>
      <div class="progress-bar-bg">
        <div class="progress-bar-fill" :style="{ width: searchProgress + '%' }"></div>
      </div>
      <p>{{ searchProgress }}% (正在检查地址 {{ Math.ceil(searchProgress * 0.32) }}/32)</p>
    </div>
  </div>

  <MotorSet 
      :visible="isModalVisible" 
      :motorData="selectedMotor"
      @close="isModalVisible = false"
      @save="handleSaveConfig"
      
    />

  <MessageContainer ref="msgBoxRef" />

      <!-- 更新提示模态框 -->
  <teleport to="body">
      <transition name="modal">
          <div v-if="showUpdateModal" class="modal-overlay" @click.self="showUpdateModal = false">
              <div class="modal-container update-modal">
                  <div class="modal-header">
                      <h3 class="modal-title">发现新版本</h3>
                      <button class="modal-close" @click="showUpdateModal = false">
                          <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
                              <line x1="18" y1="6" x2="6" y2="18"/>
                              <line x1="6" y1="6" x2="18" y2="18"/>
                          </svg>
                      </button>
                  </div>
                  <div class="modal-body">
                      <div class="update-content">
                          <div class="update-icon">
                              <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
                                  <path d="M21 12a9 9 0 0 0-9-9 9.75 9.75 0 0 0-6.74 2.74L3 8"/>
                                  <path d="M3 3v5h5"/>
                                  <path d="M3 16a9 9 0 0 0 9 9 9.75 9.75 0 0 0 6.74-2.74L21 16"/>
                                  <path d="M16 21h5v-5"/>
                              </svg>
                          </div>
                          <div class="update-info">
                              <p class="update-version">新版本: <strong>{{ updateInfo.tagName }}</strong></p>
                              <p class="update-desc">发现应用程序更新，建议及时升级以获得更好的体验。</p>
                          </div>
                      </div>
                  </div>
                  <div class="modal-footer">
                      <!-- <button class="btn btn-secondary" @click="showUpdateModal = false">
                          稍后更新
                      </button> -->
                      <button class="btn btn-primary" @click="handleUpdate('github')">
                          更新(Github)
                      </button>
                      <button class="btn btn-primary" @click="handleUpdate('accelerate')">
                          更新(加速源)
                      </button>
                  </div>
              </div>
          </div>
      </transition>
  </teleport>
</template>

<script setup lang="ts">
import { ref, onMounted, onUnmounted, provide} from 'vue';
import { EventsOn, EventsOff, BrowserOpenURL} from '../wailsjs/runtime/runtime'
import Motor from './components/Motor.vue';
import MotorSet from './components/MotorSet.vue';
import draggable from 'vuedraggable';
import MessageContainer from './components/MessageContainer.vue';

const msgBoxRef = ref(null)
const notify = (content: string, type = 'info', duration = 3000) => {
  (msgBoxRef.value as any)?.addMessage(content, type, duration)
}

provide('globalNotify', notify)


import { EnumDevices, ConnectDevice, DisconnectDevice, 
        ManualAddMotor,DeleteMotor,
  LoadLocalMotors, EditMotor, SearchMotors,APIUpdate, StartAutomation, StopAutomation, ReadFile, WriteFile } from '../wailsjs/go/main/App';




interface MotorItem {
  id: number | string;// 设备ID
  name?: string;// 设备名称
  unit?: string;// 位置单位
  speed?: number | string;// 速度
  mode?: string; // 通讯方式modbus/Ascii
  cwName?: string;// CW名称
  ccwName?: string;// CCW名称
  resolution?: number | string;// 分辨率
  description?: string;// 设备描述

  position?: number | string;// 当前位置
  enable?: boolean;// 是否使能
  positionError?: boolean;// 位置错误
  overheat?: boolean;// 是否过温
  commError?: boolean;// 是否通讯错误
  limitCW?: boolean;// 是否CW限位
  limitCCW?: boolean;// 是否CCW限位
  otherError?: boolean;// 其他错误
  isMoving?: boolean; // 是否正在运动
}

// 状态变量
const portList = ref<string[]>([]);
const selectedPort = ref('');
const selectedBaud = ref('9600');
const isConnected = ref(false);
const searchQuery = ref('');
const showAutomation = ref(false);
const automationProgress = ref(false);
const automationScript = ref('');

let motorIdCounter = 0;
const motors = ref<MotorItem[]>([]);
const isModalVisible = ref(false);
const selectedMotor = ref<any>(null);

  
const openSettings = (motor: any) => {
  // 使用浅拷贝，防止在模态框未保存时就影响主列表显示
  selectedMotor.value = { ...motor };
  isModalVisible.value = true;
};

const handleSaveConfig = async (updatedData: any) => {
  //根据id找到电机索引
  const index = motors.value.findIndex(m => m.id === updatedData.id);
  if (index !== -1) {
    try {
      const APIResponse = await EditMotor(updatedData.id, updatedData);
      if (APIResponse.status === "success") {
        // 更新成功，刷新主列表
        const updatedMotor = { ...motors.value[index], ...updatedData };
        if (updatedData.newID !== undefined && updatedData.newID !== updatedData.id) {
          updatedMotor.id = updatedData.newID;
        }
        motors.value[index] = updatedMotor;
        notify("轴(" + updatedMotor.name + ")配置已更新并保存", 'success');
      }else {
        notify("保存配置失败：" + (APIResponse.message || "请检查设备连接"), 'error');
      }
    } catch (err) {
      notify("保存配置失败：" + err, 'error');
    }
  }

  isModalVisible.value = false;
};



// 1. 刷新串口逻辑
const refreshPorts = async () => {
  try {
    const result = await EnumDevices();
    portList.value = result.data.ports || [];
    if (portList.value.length > 0 && !selectedPort.value) {
      selectedPort.value = portList.value[0];
    }
  } catch (err) {
    notify("刷新串口失败: " + err, 'error');
  }
};

// 2. 连接/断开逻辑
const toggleConnection = async () => {
  if (isConnected.value) {
    await DisconnectDevice();
    isConnected.value = false;
  } else {
    if (!selectedPort.value) {
      notify("请先选择一个串口", 'error');
      return;
    }
    try {
      const result = await ConnectDevice(selectedPort.value, parseInt(selectedBaud.value));
      if (result.status === "success") {
        isConnected.value = true;
      }
    } catch (err) {
      notify("连接失败: " + err, 'error');
    }
  }
};


const isSearching = ref(false);
const searchProgress = ref(0);

const searchMotors = async () => { 
  if (!isConnected.value) {
    notify("请先建立连接", 'error');

    return;
  }
  isSearching.value = true;
  searchProgress.value = 0;
  
  try {
    await SearchMotors();
  } finally {
    setTimeout(() => {
      isSearching.value = false;
    }, 500);
  }
};

const toggleAutomation = () => {
  showAutomation.value = !showAutomation.value;
};

const runAutomation = async () => {
  if(!automationProgress.value){
    if (!automationScript.value.trim()) {
    notify("请输入自动化脚本", 'warning');
    return;
    }

    try {
      notify("自动化脚本开始执行", 'info');
      automationProgress.value = true;
      await StartAutomation(automationScript.value);
    } catch (error) {
      automationProgress.value=false;
      notify("自动化脚本执行失败", 'error');
      console.error("通讯异常:", error);
    }
  }else{
    try {
        await StopAutomation();
        (automationProgress as any).value = false;
        notify("自动化脚本已停止", 'info');
      } catch (error) {
        notify("自动化脚本停止失败", 'error');
        console.error("通讯异常:", error);
      }
  }
};

const OpenScriptFile = async () => {
  try {
    const content = await ReadFile();
    automationScript.value = content;
    notify("脚本文件已打开", 'success');
  } catch (error) {
    notify("打开脚本文件失败", 'error');
    console.error("打开文件错误:", error);
  }
}

const SaveScriptFile = async () => {
  if (!automationScript.value.trim()) {
    notify("没有可保存的脚本内容", 'warning');
    return;
  }

  try {
    await WriteFile( automationScript.value);
    notify("脚本文件已保存", 'success');
  } catch (error) {
    notify("保存脚本文件失败", 'error');
    console.error("保存文件错误:", error);
  }
}

// FOR 10
// Z: CW 1
// DELAY 1000
// Z: CCW 1
// DELAY 1000
// END





// 设备管理逻辑
const addMotor = async () => {
  const result = await ManualAddMotor();
  if (result.status === "success") {
    notify("轴(新位移台)添加成功", 'success');
  }else {
    notify("轴(新位移台)添加失败：" + (result.message || "请检查设备连接"), 'error');
  }
};


// 移除逻辑
const removeMotor = async (index: number) => {
  await DeleteMotor(Number(motors.value[index].id));
  motors.value.splice(index, 1);
  notify("轴(" + motors.value[index].name + ")已移除", 'success');
};

// 搜索逻辑
const handleSearch = () => {
  console.log("执行搜索:", searchQuery.value);
};


const updateInfo = ref({
  tagName: '无',
  htmlUrl: 'https://github.com'
})
// 更新模态框状态
const showUpdateModal = ref(false);

// 更新处理函数
const handleUpdate = (source: string) => {
  if(source === 'github'){
    if (updateInfo.value.htmlUrl) {
        // window.open(updateInfo.value.htmlUrl, '_blank');
        BrowserOpenURL(updateInfo.value.htmlUrl) 
        showUpdateModal.value = false;
    }
  }else if(source === 'accelerate'){
    if (updateInfo.value.htmlUrl) {
        const url = 'https://ghfast.top/https://' + updateInfo.value.htmlUrl.replace('https://', '')
        BrowserOpenURL(url) 
        showUpdateModal.value = false;
    }
  }
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
        unit: motor.unit,
        description: motor.description,
        speed: motor.speed,
        resolution: motor.resolution,
        cwName: motor.cwName,
        ccwName: motor.ccwName,
        mode: motor.mode,
        motorType: motor.motorType,

        // position: motor.position,
        // enable: motor.enable,
        // positionError: motor.positionError,
        // overheat: motor.overheat,
        // commError: motor.commError,
        // limitCW: motor.limitCW,
        // limitCCW: motor.limitCCW,
        // otherError: motor.otherError,
        // isMoving: motor.isMoving,
      });
  })


  // --- 新增：监听电机状态实时更新信号 ---
  EventsOn("motor_status_update", (data: any) => {
    // data 格式：{ id: number, position: number, error: { overheat: bool, ... } }
    const motor = motors.value.find(m => m.id === data.id);
    
    if (motor) {
      // 更新位置
      motor.position = data.position;

      // 更新错误状态（根据 Go 中 MotorError 结构体的字段名对应）
      // 注意：Go 后端的 json tag 决定了这里的字段名
      if (data.error) {
        motor.positionError = data.error.positionError;
        motor.overheat = data.error.overheat;
        motor.commError = data.error.commError;
        motor.limitCW = data.error.limitCW;
        motor.limitCCW = data.error.limitCCW;
        motor.otherError = data.error.otherError;
        motor.isMoving = data.isMoving;
        motor.enable = data.isEnabled; // 同步使能状态
      }
    }

  });

  EventsOn("edit_ID", (data: any) => {
    // alert(`电机 ID 已从 ${data.oldID} 修改为 ${data.newID}`);
    
     // 1. 在当前列表中找到旧 ID 的电机
    const motor = motors.value.find(m => m.id === data.oldID);
    if (motor) {
      // 2. 更新响应式对象的 ID 属性
      motor.id = data.newID;
      
      // 3. 如果内部存了 newID 备份，重置它防止再次触发
      if ('newID' in motor) {
        (motor as any).newID = 0;
      }
      notify("轴(" + motor.name + ")ID已更新", 'success');
    }
  });

  EventsOn("search_progress", (progress: number) => {
      searchProgress.value = progress;
  });

  EventsOn("auto_error", (error: any) => {
      notify(error, 'error');
      automationProgress.value = false;
  });

  await LoadLocalMotors();


  const release = await APIUpdate()
  try {
    if (release) {
      updateInfo.value.tagName = release.tag_name
      updateInfo.value.htmlUrl = release.html_url
      if (release.assets.length > 0) {
          updateInfo.value.htmlUrl = release.assets[0].browser_download_url
      }
      // 显示更新模态框
      showUpdateModal.value = true
    }
  } catch (error) {
    console.log(error)
  }


});

onUnmounted(async() => {
  // 离开页面时，断开连接
  if (isConnected.value) {
    DisconnectDevice();
    notify("已断开连接", 'success');
  }
  EventsOff("find_motor");
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

.nav-section {
  display: flex;
  flex-direction: column;
  margin-bottom: 20px;
}

.nav-section.automation-section {
  flex: 1;         /* 核心：撑满侧边栏剩余高度 */
  min-height: 0;   /* 核心：允许子元素在空间不足时缩小，防止撑破布局 */
  display: flex;
  flex-direction: column;
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
  flex: 1;
  display: flex;
  flex-direction: column;
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

.automation-btn {
  width: 100%;
  padding: 12px;
  border-radius: 10px;
  border: none;
  background: #4caf50;
  color: white;
  font-weight: 600;
  cursor: pointer;
  margin-top: 8px;
}
.automation-btn:hover { background: #45a049; }

.automation-panel {
  margin-top: 15px;
  display: flex;
  flex-direction: column;
  flex: 1;
  min-height: 0;   /* 确保在小分辨率屏幕下也能正确计算剩余高度 */
  height: auto;    /* 覆盖掉原有的固定 100%，让 flex 弹性决定 */
}

.script-input {
  flex: 1;         /* 核心：让文本框吃满卡片内部的剩余高度 */
  width: 100%;
  background: rgba(0,0,0,0.2);
  border: 1px solid rgba(255,255,255,0.2);
  border-radius: 8px;
  padding: 10px;
  color: white;
  font-family: 'Consolas', monospace;
  font-size: 13px;
  resize: none;
  box-sizing: border-box;
  overflow-y: auto; /* 当脚本行数很多时，内部出现滚动条 */
}

.script-input:focus {
  outline: none;
  border-color: var(--primary);
}

.script-buttons {
  display: flex;
  gap: 8px;
  margin-top: 10px;
}

.btn-run {
  flex: 1;
  padding: 10px;
  border-radius: 8px;
  border: none;
  background: #2196f3;
  color: white;
  font-weight: 600;
  cursor: pointer;
}
.btn-run:hover { background: #1976d2; }

.btn-notify {
  flex: 1;
  padding: 10px;
  border-radius: 8px;
  border: none;
  background: #ff9800;
  color: white;
  font-weight: 600;
  cursor: pointer;
}
.btn-notify:hover { background: #f57c00; }

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

.motor-viewport {
  height: 100%;       /* 或者你设定的具体高度 */
  overflow-y: auto;   /* 允许纵向滚动 */
  padding-right: 20px; /* 为滚动条和间隙预留宽度，数值可根据需求调整 */
  padding-left: 20px;  /* 保持一定的左侧间距 */
  padding-top: 20px;  /* 保持一定的左侧间距 */
  box-sizing: border-box;
}

/* .motor-viewport { flex: 1; padding: 40px; overflow-y: auto; } */

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
.search-overlay {
  position: fixed;
  top: 0;
  left: 0;
  width: 100%;
  height: 100%;
  background: rgba(0, 0, 0, 0.7);
  backdrop-filter: blur(8px);
  display: flex;
  justify-content: center;
  align-items: center;
  z-index: 9999; /* 确保在最上层 */
  color: white;
}

.progress-container {
  width: 400px;
  text-align: center;
}

.progress-bar-bg {
  width: 100%;
  height: 12px;
  background: rgba(255, 255, 255, 0.1);
  border-radius: 6px;
  margin: 20px 0;
  overflow: hidden;
}

.progress-bar-fill {
  height: 100%;
  background: var(--primary);
  box-shadow: 0 0 15px var(--primary);
  transition: width 0.2s ease;
}






/* 更新模态框样式 */
/* 遮罩层 */
.modal-overlay {
    position: fixed;
    top: 0;
    left: 0;
    right: 0;
    bottom: 0;
    background: rgba(0, 0, 0, 0.6);
    display: flex;
    align-items: center;
    justify-content: center;
    z-index: 9999;
    backdrop-filter: blur(4px);
}

/* 模态框容器 */
.modal-container {
    background: linear-gradient(180deg, rgba(30, 41, 59, 0.95) 0%, rgba(15, 23, 42, 0.98) 100%);
    border-radius: 16px;
    box-shadow: 0 25px 50px -12px rgba(0, 0, 0, 0.5);
    border: 1px solid rgba(148, 163, 184, 0.1);
    overflow: hidden;
}

.modal-header {
    display: flex;
    align-items: center;
    justify-content: space-between;
    padding: 16px 20px;
}

.modal-title {
    margin: 0;
    font-size: 15px;
    font-weight: 600;
    color: #cbd5e1;
}

.modal-close {
    background: transparent;
    border: none;
    color: #94a3b8;
    cursor: pointer;
    padding: 4px;
    border-radius: 8px;
    transition: all 0.2s;
    width: 28px;
    height: 28px;
    display: flex;
    align-items: center;
    justify-content: center;
}

.modal-close:hover {
    background: rgba(255, 255, 255, 0.1);
    color: #f1f5f9;
}

.modal-close svg {
    width: 18px;
    height: 18px;
}

.modal-body {
    padding: 20px;
}

.modal-footer {
    display: flex;
    align-items: center;
    justify-content: flex-end;
    gap: 12px;
    padding: 16px 20px;
}

.btn {
    padding: 10px 20px;
    border-radius: 8px;
    font-size: 13px;
    font-weight: 500;
    cursor: pointer;
    border: none;
    transition: all 0.2s;
}

.btn-primary {
    background: linear-gradient(135deg, rgba(56, 189, 248, 1) 0%, rgba(59, 130, 246, 1) 100%);
    color: white;
}

.btn-primary:hover {
    background: linear-gradient(135deg, rgba(56, 189, 248, 0.9) 0%, rgba(59, 130, 246, 0.9) 100%);
    transform: translateY(-1px);
}

.btn-secondary {
    background: rgba(255, 255, 255, 0.1);
    color: #cbd5e1;
}

.btn-secondary:hover {
    background: rgba(255, 255, 255, 0.15);
}

/* 更新模态框内容 */
.update-modal {
    width: 420px;
    max-width: 90vw;
}

.update-modal .modal-header {
    background: linear-gradient(135deg, rgba(56, 189, 248, 0.1) 0%, rgba(59, 130, 246, 0.1) 100%);
    border-bottom: 1px solid rgba(148, 163, 184, 0.1);
}

.update-modal .modal-title {
    color: #f1f5f9;
    font-size: 16px;
    font-weight: 600;
}

.update-content {
    display: flex;
    align-items: center;
    gap: 20px;
    padding: 10px 0;
}

.update-icon {
    width: 64px;
    height: 64px;
    display: flex;
    align-items: center;
    justify-content: center;
    background: linear-gradient(135deg, rgba(56, 189, 248, 0.2) 0%, rgba(59, 130, 246, 0.2) 100%);
    border-radius: 16px;
    color: #38bdf8;
    flex-shrink: 0;
}

.update-icon svg {
    width: 32px;
    height: 32px;
}

.update-info {
    flex: 1;
}

.update-version {
    font-size: 15px;
    color: #f1f5f9;
    margin: 0 0 8px 0;
    font-weight: 500;
}

.update-version strong {
    color: #38bdf8;
}

.update-desc {
    font-size: 13px;
    color: #94a3b8;
    margin: 0;
    line-height: 1.5;
}

.update-modal .modal-footer {
    border-top: 1px solid rgba(148, 163, 184, 0.1);
}

/* 模态框动画 */
.modal-enter-active,
.modal-leave-active {
    transition: opacity 0.3s ease;
}

.modal-enter-from,
.modal-leave-to {
    opacity: 0;
}

.modal-enter-active .modal-container,
.modal-leave-active .modal-container {
    transition: transform 0.3s ease, opacity 0.3s ease;
}

.modal-enter-from .modal-container,
.modal-leave-to .modal-container {
    transform: scale(0.95);
    opacity: 0;
}
</style>
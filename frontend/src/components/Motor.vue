<template>
  <div class="modern-motor-card">
    <div class="card-header">
      <div class="motor-info">
        <span class="motor-tag">ID: {{ motor.id }}</span>
        <h3 class="motor-name">{{ motor.name || '未命名设备' }}</h3>
      </div>
      <div class="status-indicator" :class="{ 'active': motor.enable }"></div>
    </div>

    <div class="data-display">
      <div class="data-item">
        <span class="label">当前位置({{ motor.unit || 'mm'}})</span>
        <div class="value-wrapper">
          <span class="value">{{ Number(motor.position || 0).toFixed(4) }}</span>
        </div>
      </div>
    </div>

    <div class="control-grid">
      <div class="move-input-group">
        <button class="dir-btn" :disabled="!motor.enable" @click="startMove('CCW')">{{motor.ccwName||'CCW'}}</button>
        <div class="input-wrapper">
          <input 
            type="number" 
            v-model="targetMoveLength" 
            step="0.01" 
            @blur="targetMoveLength = Number(targetMoveLength).toFixed(4)" 
          />
        </div>
        <button class="dir-btn" :disabled="!motor.enable" @click="startMove('CW')">{{motor.cwName||'CW'}}</button>
      </div>

      <div class="action-row">
        <button class="btn btn-enable" @click="handleEnable">{{ motor.enable ? '禁能' : '使能' }}</button>
        <button class="btn btn-stop" @click="handleStop">停止</button>
        <button class="btn btn-zero" @click="handleZero">归零</button>
      </div>
    </div>

    <div class="status-monitor">
      <div class="monitor-grid">
        <!-- <span class="badge" :class="{ 'warning': motor.positionError }">偏离</span> -->
        <span class="badge" :class="{ 'warning': motor.overheat }">过温</span>
        <span class="badge" :class="{ 'warning': motor.commError }">通讯</span>
        <span class="badge" :class="{ 'error': motor.limitCW }">正限</span>
        <span class="badge" :class="{ 'error': motor.limitCCW }">反限</span>
        <span class="badge" :class="{ 'error': motor.otherError }">其他</span>
      </div>
      <div class="footer-btns">
        <button class="text-btn edit" @click="$emit('configure', motor)">参数配置</button>
        <button class="text-btn delete" @click="confirmRemove">移除设备</button>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, inject } from 'vue';
import { MotorEnable, MotorStop, MotorMoveRelative,ResetPosition} from '../../wailsjs/go/main/App';


const notify = inject('globalNotify');

interface Motor {
  id: number | string;// 设备ID
  name?: string;// 设备名称
  unit?: string;// 位置单位
  speed?: number | string;// 速度
  communicateType?: string; // 通讯方式modbus/Ascii
  cwName?: string;// CW名称
  ccwName?: string;// CCW名称
  resolution?: number | string;// 分辨率
  Description?: string;// 设备描述

  position?: number | string;// 当前位置
  enable?: boolean;// 是否使能
  mode?: boolean;// 位置错误
  overheat?: boolean;// 是否过温
  commError?: boolean;// 是否通讯错误
  limitCW?: boolean;// 是否CW限位
  limitCCW?: boolean;// 是否CCW限位
  otherError?: boolean;// 其他错误
  isMoving?: boolean; // 是否正在运动
}

const props = defineProps<{ motor: Motor }>();
const emit = defineEmits(['remove', 'command', 'configure']);

const targetMoveLength = ref("1.0000");

// 确认移除逻辑
const confirmRemove = () => {
  const isConfirmed = window.confirm(`确定要移除设备 [${props.motor.name || props.motor.id}] 吗？\n此操作不可撤销。`);
  if (isConfirmed) {
    // (notify as (msg: string, type: string) => void)("确认移除设备", 'info');
    emit('remove');
  }
};



// 前端接口
const handleEnable = async () => {
  const currentStatus = !!props.motor.enable; 
  const targetStatus = !currentStatus;

  try {
    const result = await MotorEnable(Number(props.motor.id), targetStatus);
    if (result.status === "success") {
      (notify as (msg: string, type: string) => void)("轴(" + props.motor.name + (targetStatus ? ")使能成功" : ")禁能成功") , 'success');
    } else {
      (notify as (msg: string, type: string) => void)("操作失败", 'error');
    }
  } catch (error) {
    console.error("通讯异常:", error);
    (notify as (msg: string, type: string) => void)("系统错误，请检查后端程序", 'error');
  }
};


const handleStop = async() => {
  try {
    const result = await MotorStop(Number(props.motor.id));
    if (result.status === "success") {
      console.log("操作成功");
      (notify as (msg: string, type: string) => void)("操作成功", 'success');
    } else {
      (notify as (msg: string, type: string) => void)("操作失败", 'error');
    }
  } catch (error) {
    console.error("通讯异常:", error);
    (notify as (msg: string, type: string) => void)("通讯异常：" + error, 'error');
  }
}

const handleZero = async() => {
  try {
    const result = await ResetPosition(Number(props.motor.id));
    if (result.status === "success") {
      (notify as (msg: string, type: string) => void)("轴(" + props.motor.name + ")归零成功", 'success');
    } else {
      (notify as (msg: string, type: string) => void)("轴(" + props.motor.name + ")归零失败", 'error');
    }
  } catch (error) {
    console.error("通讯异常:", error);
  }

}
const startMove = async (dir: string) => {

  var length = Number(targetMoveLength.value);
  
  if (dir === 'CCW') {
    length = -length;
  } 

  try {
    const result = await MotorMoveRelative(Number(props.motor.id), length);
    if (result.status === "success") {
      console.log("操作成功");

    } else {
      (notify as (msg: string, type: string) => void)("操作失败", 'error');
    }
  } catch (error) {
    console.error("通讯异常:", error);
    (notify as (msg: string, type: string) => void)("系统错误，请检查后端程序", 'error');
  }
  
}

</script>

<style scoped>
.modern-motor-card {
  aspect-ratio: 3 / 4;
  width: 100%;
  background: #ffffff;
  border-radius: 24px;
  padding: 20px;
  box-shadow: 0 8px 30px rgba(0, 0, 0, 0.04);
  border: 1px solid #f0f2f5;
  display: flex;
  flex-direction: column;
  transition: all 0.3s cubic-bezier(0.4, 0, 0.2, 1);
  box-sizing: border-box;
}

.modern-motor-card:hover {
  transform: translateY(-6px);
  box-shadow: 0 15px 45px rgba(0, 0, 0, 0.08);
}

.card-header {
  display: flex;
  justify-content: space-between;
  align-items: flex-start;
  flex: 0 0 auto;
}

.motor-tag {
  background: #edf2ff;
  color: #4c6ef5;
  padding: 2px 10px;
  border-radius: 8px;
  font-size: 12px;
  font-weight: 800;
  text-transform: uppercase;
}

.motor-name {
  margin: 6px 0 0 0;
  color: #1a1b1e;
  font-size: 1.2rem;
  font-weight: 700;
  justify-content: center; 
}

.status-indicator {
  width: 18px; /* 稍微缩小一点更精致 */
  height: 18px;
  border-radius: 50%;
  background: #ced4da;
  margin-top: 8px;
}
.status-indicator.active {
  background: #40c057;
  box-shadow: 0 0 12px rgba(64, 192, 87, 0.5);
}

.data-display {
  background: #f8f9fa;
  padding: 12px 16px;
  border-radius: 16px;
  margin: 15px 0;
  flex: 0 0 auto;
  display: flex;
  justify-content: center; 
}

.data-item {
  display: flex;
  flex-direction: column;
  align-items: center;
}

.label { 
  font-size: 11px; 
  color: #868e96; 
  font-weight: 600; 
  margin-bottom: 4px;
}

.value-wrapper { 
  display: flex; 
  align-items: baseline; 
  justify-content: center; 
  gap: 4px; 
}

.value { 
  font-size: 28px; 
  font-weight: 800; 
  color: #212529; 
  font-family: 'Monaco', monospace; 
}


.control-grid {
  flex: 1;
  display: flex;
  flex-direction: column;
  justify-content: center;
  gap: 12px;
}

.move-input-group {
  display: flex;
  background: #f1f3f5;
  padding: 4px;
  border-radius: 12px;
}

.dir-btn {
  border: none;
  background: white;
  padding: 10px 18px;
  border-radius: 10px;
  font-weight: 700;
  color: #495057;
  cursor: pointer;
  box-shadow: 0 2px 4px rgba(0,0,0,0.05);
}


.dir-btn:disabled {
  background-color: #ececec; /* 变灰 */
  cursor: not-allowed;    /* 鼠标变成禁用图标 */
  pointer-events: none;   /* 核心：彻底拦截鼠标点击事件 */
}

.input-wrapper { flex: 1; display: flex; align-items: center; }
.input-wrapper input {
  width: 100%;
  border: none;
  background: transparent;
  text-align: center;
  font-size: 18px;
  font-weight: 600;
  outline: none;
}

.action-row { display: flex; gap: 8px; }
.btn {
  flex: 1;
  border: none;
  padding: 12px;
  border-radius: 10px;
  font-weight: 700;
  cursor: pointer;
  transition: filter 0.2s;
}

.btn-enable { background: #4c6ef5; color: white; }
.btn-stop { background: #fa5252; color: white; }
.btn-zero { background: #adb5bd; color: white; }
.btn:hover { filter: brightness(1.1); }

.status-monitor {
  flex: 0 0 auto;
  border-top: 1px solid #f1f3f5;
  padding-top: 15px;
}

.monitor-grid { display: flex; flex-wrap: wrap; gap: 6px; margin-bottom: 12px; }
.badge {
  font-size: 10px;
  padding: 4px 10px;
  border-radius: 6px;
  background: #f1f3f5;
  color: #adb5bd;
  font-weight: 600;
  min-width: 20px;
}
.badge.warning { background: #fff4e6; color: #fd7e14; }
.badge.error { background: #fff5f5; color: #fa5252; }
.badge.info { background: #e7f5ff; color: #228be6; }

.footer-btns { display: flex; justify-content: space-between; }
.text-btn { background: none; border: none; color: #adb5bd; font-size: 12px; cursor: pointer; font-weight: 600; }
.text-btn.delete:hover { color: #fa5252; }
.text-btn.edit:hover { color: #4c6ef5; }
</style>
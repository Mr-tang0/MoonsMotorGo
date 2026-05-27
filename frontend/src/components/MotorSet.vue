<template>
  <div v-if="visible" class="modal-mask" @click.self="$emit('close')">
    <div class="modal-container">
      <header class="modal-header">
        <div class="header-title">
          <span class="icon">⚙️</span>
          <h3>电机参数配置</h3>
        </div>
        <button class="close-btn" @click="$emit('close')">&times;</button>
      </header>

      <main class="modal-body">
        <div class="form-row">
          <div class="form-group">
            <label>设备 ID</label>
            <input v-model="form.newID" type="text" placeholder="1-31"/>
          </div>
          <div class="form-group">
            <label>设备名称</label>
            <input v-model="form.name" type="text" placeholder="例如：X轴位移台" />
          </div>
        </div>

        <div class="form-row">
          <div class="form-group">
            <label>运行速度 ({{ form.unit }}/s)</label>
            <input v-model.number="form.speed" type="number" step="0.1" />
          </div>
          <div class="form-group">
            <label>脉冲分辨率 (step/{{ form.unit }})</label>
            <input v-model.number="form.resolution" type="number" />
          </div>
        </div>

        <div class="form-row">
          <div class="form-group">
            <label>位置单位</label>
            <select v-model="form.unit">
              <option value="mm">mm (毫米)</option>
              <option value="pulse">pulse (脉冲)</option>
              <option value="deg">deg (度)</option>
            </select>
          </div>
          <div class="form-group">
            <label>通讯协议</label>
            <select v-model="form.mode">
              <option value="modbus">Modbus RTU</option>
              <option value="scl">SCL (ASCII)</option>
            </select>
          </div>
        </div>

        <div class="form-row">
          <div class="form-group">
            <label>正向名称 (CW)</label>
            <input v-model="form.cwName" type="text" placeholder="如：增加" />
          </div>
          <div class="form-group">
            <label>反向名称 (CCW)</label>
            <input v-model="form.ccwName" type="text" placeholder="如：减少" />
          </div>
        </div>

        <div class="form-group">
          <label>设备描述</label>
          <textarea v-model="form.description" rows="2" placeholder="备注信息..."></textarea>
        </div>
      </main>

      <footer class="modal-footer">
        <button class="btn-cancel" @click="$emit('close')">取消</button>
        <button class="btn-save" @click="handleSave">确认保存</button>
      </footer>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, watch } from 'vue';

interface MotorItem {
  id: number | string;
  name?: string;
  unit?: string;
  speed?: number | string;
  mode?: string;
  cwName?: string;
  ccwName?: string;
  resolution?: number | string;
  description?: string;
  newID?: number | string;
}

const props = defineProps<{
  visible: boolean;
  motorData: MotorItem | null;
}>();

const emit = defineEmits(['close', 'save']);

// 初始化表单副本
const form = ref<MotorItem>({
  id: '',
  name: '',
  unit: 'mm',
  speed: 1,
  mode: 'modbus',
  cwName: 'CW',
  ccwName: 'CCW',
  resolution: 1000,
  description: '',
  newID: '',
});

// 监听模态框打开，深度拷贝数据，确保初始化
watch(() => props.visible, (newVal) => {
  if (newVal && props.motorData) {
    form.value = JSON.parse(JSON.stringify(props.motorData));
    form.value.newID = form.value.id;
  }
});

const handleSave = () => {
  // 数据类型整理
  const finalData = {
    ...form.value,
    speed: Number(form.value.speed),
    resolution: Number(form.value.resolution),
    newID: Number(form.value.newID),
  };
  emit('save', finalData);
};
</script>

<style scoped>
.modal-mask {
  position: fixed;
  inset: 0;
  background: rgba(0, 0, 0, 0.6);
  backdrop-filter: blur(4px);
  display: flex;
  justify-content: center;
  align-items: center;
  z-index: 2000;
}

.modal-container {
  background: white;
  width: 500px;
  border-radius: 16px;
  box-shadow: 0 20px 50px rgba(0,0,0,0.3);
  display: flex;
  flex-direction: column;
}

.modal-header {
  padding: 20px;
  border-bottom: 1px solid #eee;
  display: flex;
  justify-content: space-between;
  align-items: center;
}

.header-title { display: flex; align-items: center; gap: 10px; }
.header-title h3 { margin: 0; font-size: 18px; color: #333; }

.modal-body { padding: 20px; }

.form-row { display: flex; gap: 15px; margin-bottom: 15px; }
.form-group { display: flex; flex-direction: column; flex: 1; }
.flex-1 { flex: 1; }
.flex-2 { flex: 2; }

label { font-size: 13px; font-weight: 600; color: #666; margin-bottom: 6px; }

input, select, textarea {
  padding: 10px;
  border: 1px solid #ddd;
  border-radius: 8px;
  font-size: 14px;
  transition: border-color 0.2s;
}

input:focus, select:focus, textarea:focus {
  border-color: #4c6ef5;
  outline: none;
}

.disabled-input { background: #f5f5f5; color: #999; cursor: not-allowed; }

.modal-footer {
  padding: 15px 20px;
  border-top: 1px solid #eee;
  display: flex;
  justify-content: flex-end;
  gap: 12px;
}

button { padding: 10px 20px; border-radius: 8px; font-weight: 600; cursor: pointer; border: none; }
.btn-save { background: #4c6ef5; color: white; }
.btn-cancel { background: #f1f3f5; color: #495057; }
.close-btn { background: none; font-size: 24px; color: #ccc; }
</style>
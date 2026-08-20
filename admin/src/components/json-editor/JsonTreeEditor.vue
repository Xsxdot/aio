<template>
  <div class="json-tree-editor">
    <div class="json-tree-toolbar">
      <a-space>
        <a-button type="text" size="small" @click="addRootItem">
          <template #icon><icon-plus /></template>
          {{ $t('jsonEditor.addRootNode') }}
        </a-button>
        <a-button type="text" size="small" @click="emit('apply', JSON.stringify(jsonData))">
          <template #icon><icon-check /></template>
          {{ $t('jsonEditor.apply') }}
        </a-button>
        <a-button type="text" size="small" @click="expandAll">
          <template #icon><icon-expand /></template>
          {{ $t('jsonEditor.expandAll') }}
        </a-button>
        <a-button type="text" size="small" @click="collapseAll">
          <template #icon><icon-shrink /></template>
          {{ $t('jsonEditor.collapseAll') }}
        </a-button>
      </a-space>
    </div>

    <div class="tree-container" :style="{ height: `${height}px` }">
      <div v-if="isArray" class="tree-header">{{ $t('jsonEditor.array') }} [{{ jsonData.length }} {{ $t('jsonEditor.items') }}]</div>
      <div v-else class="tree-header">{{ $t('jsonEditor.object') }} [{{ Object.keys(jsonData).length }} {{ $t('jsonEditor.items') }}]</div>
      
      <template v-if="isArray">
        <div 
          v-for="(item, index) in jsonData" 
          :key="index" 
          class="tree-item"
        >
          <json-tree-node
            :value="item"
            :path="String(index)"
            :expanded-paths="expandedPaths"
            @update:value="updateArrayItem(index, $event)"
            @remove="removeArrayItem(index)"
            @addChild="addArrayChild(index, $event)"
            @toggleExpand="toggleExpand"
          />
        </div>
        <div class="tree-add">
          <a-button 
            type="dashed" 
            size="small" 
            long 
            @click="addArrayItem"
          >
            <template #icon><icon-plus /></template>
            {{ $t('jsonEditor.addArrayItem') }}
          </a-button>
        </div>
      </template>
      
      <template v-else>
        <div 
          v-for="key in Object.keys(jsonData)" 
          :key="key" 
          class="tree-item"
        >
          <json-tree-node
            :name="key"
            :value="jsonData[key]"
            :path="key"
            :expanded-paths="expandedPaths"
            @update:name="updateObjectKey(key, $event)"
            @update:value="updateObjectValue(key, $event)"
            @remove="removeObjectItem(key)"
            @addChild="addObjectChild(key, $event)"
            @toggleExpand="toggleExpand"
          />
        </div>
        <div class="tree-add">
          <a-button 
            type="dashed" 
            size="small" 
            long 
            @click="addObjectItem"
          >
            <template #icon><icon-plus /></template>
            {{ $t('jsonEditor.addKeyValue') }}
          </a-button>
        </div>
      </template>
    </div>
  </div>
</template>

<script lang="ts">
export default {
  name: 'JsonTreeEditor'
};
</script>

<script setup lang="ts">
import { ref, computed, watch, PropType } from 'vue';
import { Message } from '@arco-design/web-vue';
import { useI18n } from 'vue-i18n';
import { 
  IconCheck, 
  IconPlus, 
  IconExpand, 
  IconShrink
} from '@arco-design/web-vue/es/icon';
import JsonTreeNode from './JsonTreeNode.vue';

const props = defineProps({
  modelValue: {
    type: String,
    required: true
  },
  height: {
    type: [String, Number],
    default: 400
  },
  mode: {
    type: String as PropType<'object' | 'array'>,
    default: 'object'
  }
});

const emit = defineEmits(['update:modelValue', 'apply']);
const { t } = useI18n();

// 解析JSON数据
const jsonData = ref<any>(props.mode === 'object' ? {} : []);
const expandedPaths = ref<Set<string>>(new Set());
const isArray = computed(() => Array.isArray(jsonData.value));

// 初始化数据
try {
  const initialData = JSON.parse(props.modelValue);
  if ((props.mode === 'object' && typeof initialData === 'object' && !Array.isArray(initialData)) ||
      (props.mode === 'array' && Array.isArray(initialData))) {
    jsonData.value = initialData;
  } else {
    jsonData.value = props.mode === 'object' ? {} : [];
  }
} catch (e) {
  console.error('Invalid JSON:', e);
  jsonData.value = props.mode === 'object' ? {} : [];
}

// 监听modelValue变化
watch(() => props.modelValue, (newVal) => {
  try {
    const parsed = JSON.parse(newVal);
    if ((props.mode === 'object' && typeof parsed === 'object' && !Array.isArray(parsed)) ||
        (props.mode === 'array' && Array.isArray(parsed))) {
      jsonData.value = parsed;
    }
  } catch (e) {
    console.error('Invalid JSON:', e);
  }
});

// 监听jsonData变化，同步到modelValue
watch(jsonData, () => {
  try {
    const jsonStr = JSON.stringify(jsonData.value);
    emit('update:modelValue', jsonStr);
  } catch (e) {
    console.error('Error stringifying JSON:', e);
  }
}, { deep: true });

// 对象操作
function addObjectItem() {
  const newKey = generateUniqueKey(jsonData.value);
  jsonData.value[newKey] = '';
}

function updateObjectKey(oldKey: string, newKey: string) {
  if (oldKey === newKey) return;
  
  if (Object.prototype.hasOwnProperty.call(jsonData.value, newKey)) {
    Message.warning(t('jsonEditor.keyExists'));
    return;
  }
  
  const value = jsonData.value[oldKey];
  delete jsonData.value[oldKey];
  jsonData.value[newKey] = value;
}

function updateObjectValue(key: string, value: any) {
  jsonData.value[key] = value;
}

function removeObjectItem(key: string) {
  delete jsonData.value[key];
}

function addObjectChild(key: string, { type, value }: { type: string, value: any }) {
  if (typeof jsonData.value[key] === 'object' && jsonData.value[key] !== null) {
    if (Array.isArray(jsonData.value[key])) {
      jsonData.value[key].push(value);
    } else {
      const newKey = generateUniqueKey(jsonData.value[key]);
      jsonData.value[key][newKey] = value;
    }
  } else {
    jsonData.value[key] = type === 'array' ? [value] : { [generateUniqueKey({})]: value };
  }
}

// 数组操作
function addArrayItem() {
  jsonData.value.push('');
}

function updateArrayItem(index: number, value: any) {
  jsonData.value[index] = value;
}

function removeArrayItem(index: number) {
  jsonData.value.splice(index, 1);
}

function addArrayChild(index: number, { type, value }: { type: string, value: any }) {
  if (typeof jsonData.value[index] === 'object' && jsonData.value[index] !== null) {
    if (Array.isArray(jsonData.value[index])) {
      jsonData.value[index].push(value);
    } else {
      const newKey = generateUniqueKey(jsonData.value[index]);
      jsonData.value[index][newKey] = value;
    }
  } else {
    jsonData.value[index] = type === 'array' ? [value] : { [generateUniqueKey({})]: value };
  }
}

// 根级操作
function addRootItem() {
  if (isArray.value) {
    addArrayItem();
  } else {
    addObjectItem();
  }
}

// 路径展开/折叠
function toggleExpand(path: string) {
  if (expandedPaths.value.has(path)) {
    expandedPaths.value.delete(path);
  } else {
    expandedPaths.value.add(path);
  }
}

function expandAll() {
  // 递归收集所有路径
  function collectPaths(obj: any, path = '') {
    if (obj === null || typeof obj !== 'object') return;
    
    Object.keys(obj).forEach(key => {
      const currentPath = path ? `${path}.${key}` : key;
      expandedPaths.value.add(currentPath);
      if (obj[key] !== null && typeof obj[key] === 'object') {
        collectPaths(obj[key], currentPath);
      }
    });
  }
  
  expandedPaths.value.clear();
  collectPaths(jsonData.value);
}

function collapseAll() {
  expandedPaths.value.clear();
}

// 生成唯一键名
function generateUniqueKey(obj: object): string {
  const baseKey = 'key';
  let count = 1;
  let newKey = `${baseKey}${count}`;
  
  while (Object.prototype.hasOwnProperty.call(obj, newKey)) {
    count += 1;
    newKey = `${baseKey}${count}`;
  }
  
  return newKey;
}
</script>

<style scoped>
.json-tree-editor {
  border: 1px solid var(--color-border-2);
  border-radius: 4px;
  overflow: hidden;
  background-color: var(--color-bg-2);
  height: 100%;
  display: flex;
  flex-direction: column;
}

.json-tree-toolbar {
  padding: 8px;
  border-bottom: 1px solid var(--color-border-2);
  background-color: var(--color-bg-1);
  display: flex;
  justify-content: space-between;
}

.tree-container {
  flex: 1;
  overflow-y: auto;
  padding: 8px;
}

.tree-header {
  padding: 4px 8px;
  font-weight: bold;
  color: var(--color-text-2);
  margin-bottom: 8px;
}

.tree-item {
  margin-bottom: 4px;
}

.tree-add {
  margin-top: 12px;
  padding: 0 12px;
}
</style>


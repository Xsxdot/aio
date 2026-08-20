<template>
  <div 
    class="json-tree-node" 
    :class="{ 'json-tree-node-expanded': isExpanded }"
  >
    <div class="node-header">
      <!-- 展开/折叠按钮 -->
      <div v-if="isExpandable" class="expand-btn" @click="toggleExpand">
        <icon-caret-right v-if="!isExpanded" />
        <icon-caret-down v-else />
      </div>
      <div v-else class="expand-placeholder"></div>
      
      <!-- 节点名称 (对象键名或数组索引) -->
      <div v-if="name !== undefined" class="node-name">
        <a-input
          v-if="isEditingName"
          ref="nameInputRef"
          v-model="editingName"
          size="mini"
          style="width: 120px;"
          @blur="saveName"
          @keyup.enter="saveName"
          @keyup.esc="cancelEditName"
        />
        <span v-else class="name-text" @dblclick="startEditName">{{ name }}</span>
        <span class="name-colon">:</span>
      </div>
      
      <!-- 节点值 -->
      <div class="node-value">
        <template v-if="isObject">
          <span class="value-text object-value" @click="toggleExpand">
            {{ isArray ? '[...]' : '{...}' }}
            <span class="item-count">{{ itemCount }} {{ $t('jsonEditor.items') }}</span>
          </span>
        </template>
        
        <template v-else>
          <a-select
            v-if="isEditingValue"
            v-model="editingType"
            style="width: 90px; margin-right: 8px;"
            size="mini"
          >
            <a-option value="string">{{ $t('jsonEditor.string') }}</a-option>
            <a-option value="number">{{ $t('jsonEditor.number') }}</a-option>
            <a-option value="boolean">{{ $t('jsonEditor.boolean') }}</a-option>
            <a-option value="null">null</a-option>
            <a-option value="object">{{ $t('jsonEditor.object') }}</a-option>
            <a-option value="array">{{ $t('jsonEditor.array') }}</a-option>
          </a-select>
          
          <div v-if="isEditingValue" class="value-editor">
            <template v-if="editingType === 'string'">
              <a-input
                ref="valueInputRef"
                v-model="editingValueString"
                size="mini"
                style="width: 200px;"
                @blur="saveValue"
                @keyup.enter="saveValue"
                @keyup.esc="cancelEditValue"
              />
            </template>
            
            <template v-else-if="editingType === 'number'">
              <a-input-number
                ref="valueInputRef"
                v-model="editingValueNumber"
                size="mini"
                style="width: 140px;"
                @blur="saveValue"
                @keyup.enter="saveValue"
                @keyup.esc="cancelEditValue"
              />
            </template>
            
            <template v-else-if="editingType === 'boolean'">
              <a-select
                v-model="editingValueBoolean"
                size="mini"
                style="width: 80px;"
                @blur="saveValue"
                @change="saveValue"
              >
                <a-option :value="true">true</a-option>
                <a-option :value="false">false</a-option>
              </a-select>
            </template>
            
            <template v-else-if="editingType === 'null'">
              <span class="null-value">null</span>
              <a-button size="mini" type="text" @click="saveValue">{{ $t('jsonEditor.confirm') }}</a-button>
            </template>
            
            <template v-else-if="editingType === 'object' || editingType === 'array'">
              <span class="placeholder-value">{{ editingType === 'object' ? '{}' : '[]' }}</span>
              <a-button size="mini" type="text" @click="saveValue">{{ $t('jsonEditor.confirm') }}</a-button>
            </template>
          </div>
          
          <span 
            v-else 
            class="value-text" 
            :class="valueTypeClass" 
            @dblclick="startEditValue"
          >
            {{ displayValue }}
          </span>
        </template>
      </div>
      
      <!-- 操作按钮 -->
      <div class="node-actions">
        <a-button 
          v-if="isObject" 
          type="text" 
          size="mini" 
          @click="addChild"
        >
          <template #icon><icon-plus /></template>
        </a-button>
        <a-button 
          type="text" 
          status="danger" 
          size="mini" 
          @click="remove"
        >
          <template #icon><icon-delete /></template>
        </a-button>
      </div>
    </div>
    
    <!-- 子节点 -->
    <div v-if="isObject && isExpanded" class="node-children">
      <div v-if="isArray">
        <div 
          v-for="(item, index) in objectValue" 
          :key="index" 
          class="child-node"
        >
          <json-tree-node
            :value="item"
            :path="`${path}.${index}`"
            :expanded-paths="expandedPaths"
            @update:value="updateArrayChild(Number(index), $event)"
            @remove="removeArrayChild(Number(index))"
            @addChild="addArrayChild(Number(index), $event)"
            @toggleExpand="toggleExpandChild"
          />
        </div>
        <div class="add-child">
          <a-button 
            type="dashed" 
            size="small" 
            @click="addArrayItem"
          >
            <template #icon><icon-plus /></template>
            {{ $t('jsonEditor.addArrayItem') }}
          </a-button>
        </div>
      </div>
      <div v-else>
        <div 
          v-for="key in objectKeys" 
          :key="key" 
          class="child-node"
        >
          <json-tree-node
            :name="key"
            :value="objectValue[key]"
            :path="`${path}.${key}`"
            :expanded-paths="expandedPaths"
            @update:name="updateObjectKey(key, $event)"
            @update:value="updateObjectValue(key, $event)"
            @remove="removeObjectItem(key)"
            @addChild="addObjectChild(key, $event)"
            @toggleExpand="toggleExpandChild"
          />
        </div>
        <div class="add-child">
          <a-button 
            type="dashed" 
            size="small" 
            @click="addObjectItem"
          >
            <template #icon><icon-plus /></template>
            {{ $t('jsonEditor.addKeyValue') }}
          </a-button>
        </div>
      </div>
    </div>
  </div>
</template>

<script lang="ts">
export default {
  name: 'JsonTreeNode'
};
</script>

<script setup lang="ts">
import { ref, computed, PropType, nextTick } from 'vue';
import { 
  IconCaretRight, 
  IconCaretDown, 
  IconPlus, 
  IconDelete
} from '@arco-design/web-vue/es/icon';

const props = defineProps({
  name: {
    type: String,
    default: undefined
  },
  value: {
    type: [String, Number, Boolean, Object, Array],
    required: true
  },
  path: {
    type: String,
    required: true
  },
  expandedPaths: {
    type: Object as PropType<Set<string>>,
    required: true
  }
});

const emit = defineEmits([
  'update:name',
  'update:value',
  'remove',
  'addChild',
  'toggleExpand'
]);

// 节点展开状态
const isExpanded = computed(() => props.expandedPaths.has(props.path));

// 是否为对象类型（对象或数组）
const isObject = computed(() => 
  props.value !== null && 
  typeof props.value === 'object'
);

// 是否为数组
const isArray = computed(() => Array.isArray(props.value));

// 是否可展开
const isExpandable = computed(() => isObject.value);

// 显示的值
const displayValue = computed(() => {
  if (props.value === null) return 'null';
  if (typeof props.value === 'string') return `"${props.value}"`;
  if (typeof props.value === 'undefined') return 'undefined';
  return String(props.value);
});

// 值类型的CSS类名
const valueTypeClass = computed(() => {
  if (props.value === null) return 'null-value';
  if (typeof props.value === 'string') return 'string-value';
  if (typeof props.value === 'number') return 'number-value';
  if (typeof props.value === 'boolean') return 'boolean-value';
  return '';
});

// 子节点数量
const itemCount = computed(() => {
  if (!isObject.value) return 0;
  return Object.keys(props.value).length;
});

// 对象的键
const objectKeys = computed(() => {
  if (!isObject.value || isArray.value) return [];
  return Object.keys(props.value as Record<string, any>);
});

// 对象值引用
const objectValue = computed(() => props.value as Record<string, any>);

// 编辑状态
const isEditingName = ref(false);
const isEditingValue = ref(false);
const editingName = ref('');
const editingType = ref('string');
const editingValueString = ref('');
const editingValueNumber = ref<number | undefined>(undefined);
const editingValueBoolean = ref<boolean>(false);
const nameInputRef = ref<HTMLElement | null>(null);
const valueInputRef = ref<HTMLElement | null>(null);

// 开始编辑名称
function startEditName() {
  editingName.value = props.name as string;
  isEditingName.value = true;
  nextTick(() => {
    nameInputRef.value?.focus();
  });
}

// 保存编辑的名称
function saveName() {
  if (isEditingName.value) {
    if (editingName.value.trim()) {
      emit('update:name', editingName.value.trim());
    }
    isEditingName.value = false;
  }
}

// 取消编辑名称
function cancelEditName() {
  isEditingName.value = false;
}

// 开始编辑值
function startEditValue() {
  if (isObject.value) return;
  
  // 设置当前值类型
  if (props.value === null) {
    editingType.value = 'null';
  } else if (Array.isArray(props.value)) {
    editingType.value = 'array';
  } else {
    editingType.value = typeof props.value as string;
  }
  
  // 根据类型设置值
  if (editingType.value === 'string') {
    editingValueString.value = props.value as string;
  } else if (editingType.value === 'number') {
    editingValueNumber.value = props.value as number;
  } else if (editingType.value === 'boolean') {
    editingValueBoolean.value = props.value as boolean;
  }
  
  isEditingValue.value = true;
  nextTick(() => {
    valueInputRef.value?.focus();
  });
}

// 保存编辑的值
function saveValue() {
  if (isEditingValue.value) {
    let newValue: any;
    
    if (editingType.value === 'string') {
      newValue = editingValueString.value;
    } else if (editingType.value === 'number') {
      newValue = editingValueNumber.value;
    } else if (editingType.value === 'boolean') {
      newValue = editingValueBoolean.value;
    } else if (editingType.value === 'null') {
      newValue = null;
    } else if (editingType.value === 'object') {
      newValue = {};
    } else if (editingType.value === 'array') {
      newValue = [];
    }
    
    emit('update:value', newValue);
    isEditingValue.value = false;
  }
}

// 取消编辑值
function cancelEditValue() {
  isEditingValue.value = false;
}

// 展开/折叠节点
function toggleExpand() {
  if (isExpandable.value) {
    emit('toggleExpand', props.path);
  }
}

// 子节点展开/折叠
function toggleExpandChild(path: string) {
  emit('toggleExpand', path);
}

// 移除节点
function remove() {
  emit('remove');
}

// 添加子节点
function addChild() {
  if (!isObject.value) {
    let newValue;
    const type = 'object';
    
    if (type === 'object') {
      newValue = {};
    } else if (type === 'array') {
      newValue = [];
    }
    
    emit('addChild', { type, value: newValue });
  }
}

// 对象操作
function addObjectItem() {
  if (!isObject.value || isArray.value) return;
  
  const newKey = generateUniqueKey(props.value as object);
  const objectCopy = { ...props.value as object, [newKey]: '' };
  emit('update:value', objectCopy);
}

function updateObjectKey(oldKey: string, newKey: string) {
  if (!isObject.value || isArray.value) return;
  if (oldKey === newKey) return;
  
  const obj = props.value as Record<string, any>;
  if (Object.prototype.hasOwnProperty.call(obj, newKey)) {
    // 键名已存在
    return;
  }
  
  const objectCopy: Record<string, any> = {};
  Object.keys(obj).forEach((key) => {
    if (key === oldKey) {
      objectCopy[newKey] = obj[key];
    } else {
      objectCopy[key] = obj[key];
    }
  });
  
  emit('update:value', objectCopy);
}

function updateObjectValue(key: string, value: any) {
  if (!isObject.value || isArray.value) return;
  
  const objectCopy = { ...props.value as object, [key]: value };
  emit('update:value', objectCopy);
}

function removeObjectItem(key: string) {
  if (!isObject.value || isArray.value) return;
  
  const objectCopy = { ...props.value as Record<string, any> };
  delete objectCopy[key];
  emit('update:value', objectCopy);
}

function addObjectChild(key: string, { type, value }: { type: string, value: any }) {
  if (!isObject.value || isArray.value) return;
  
  const objectCopy = { ...props.value as Record<string, any> };
  objectCopy[key] = value;
  emit('update:value', objectCopy);
}

// 数组操作
function addArrayItem() {
  if (!isObject.value || !isArray.value) return;
  
  const arrayCopy = [...props.value as any[]];
  arrayCopy.push('');
  emit('update:value', arrayCopy);
}

function updateArrayChild(index: number, value: any) {
  if (!isObject.value || !isArray.value) return;
  
  const arrayCopy = [...props.value as any[]];
  arrayCopy[index] = value;
  emit('update:value', arrayCopy);
}

function removeArrayChild(index: number) {
  if (!isObject.value || !isArray.value) return;
  
  const arrayCopy = [...props.value as any[]];
  arrayCopy.splice(index, 1);
  emit('update:value', arrayCopy);
}

function addArrayChild(index: number, { type, value }: { type: string, value: any }) {
  if (!isObject.value || !isArray.value) return;
  
  const arrayCopy = [...props.value as any[]];
  arrayCopy[index] = value;
  emit('update:value', arrayCopy);
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
.json-tree-node {
  border-radius: 4px;
  margin: 2px 0;
}

.node-header {
  display: flex;
  align-items: center;
  padding: 4px;
  border-radius: 4px;
  cursor: default;
}

.node-header:hover {
  background-color: var(--color-fill-2);
}

.expand-btn {
  width: 20px;
  height: 20px;
  display: flex;
  align-items: center;
  justify-content: center;
  cursor: pointer;
  margin-right: 4px;
}

.expand-placeholder {
  width: 20px;
  margin-right: 4px;
}

.node-name {
  display: flex;
  align-items: center;
  margin-right: 8px;
  max-width: 200px;
  overflow: hidden;
}

.name-text {
  color: var(--color-text-2);
  font-weight: 500;
  cursor: text;
  word-break: break-all;
}

.name-colon {
  margin: 0 4px;
  color: var(--color-text-3);
}

.node-value {
  flex: 1;
  display: flex;
  align-items: center;
  min-width: 0;
  overflow: hidden;
}

.value-text {
  cursor: text;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
  max-width: 100%;
}

.string-value {
  color: #468c05;
}

.number-value {
  color: #1a57db;
}

.boolean-value {
  color: #9747ff;
}

.null-value {
  color: #ff4d4f;
}

.object-value {
  color: #00000073;
  cursor: pointer;
}

.item-count {
  margin-left: 8px;
  font-size: 12px;
  color: var(--color-text-3);
}

.value-editor {
  display: flex;
  align-items: center;
  gap: 8px;
}

.placeholder-value {
  color: var(--color-text-3);
  font-style: italic;
}

.node-actions {
  opacity: 0;
  display: flex;
  gap: 2px;
  margin-left: 4px;
}

.node-header:hover .node-actions {
  opacity: 1;
}

.node-children {
  padding-left: 24px;
  margin-top: 2px;
  margin-bottom: 2px;
}

.child-node {
  margin: 2px 0;
}

.add-child {
  margin-top: 8px;
}
</style>



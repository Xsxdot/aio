<template>
  <a-modal
    :visible="visible"
    :title="title || $t('jsonEditor.treeViewTitle')"
    :width="width"
    :mask-closable="false"
    :esc-to-close="false"
    :footer="false"
    :unmount-on-close="false"
    class="json-tree-editor-modal"
    @close="handleClose"
  >
    <div class="editor-container">
      <div class="editor-toolbar">
        <a-radio-group v-model="editorMode" type="button">
          <a-radio value="tree">{{ $t('jsonEditor.treeEditor') }}</a-radio>
          <a-radio value="code">{{ $t('jsonEditor.codeEditor') }}</a-radio>
        </a-radio-group>
      </div>

      <div class="editor-content">
        <!-- 树形编辑器 -->
        <div v-if="editorMode === 'tree'" class="tree-editor-wrapper">
          <json-tree-editor
            ref="treeEditorRef"
            v-model="localValue"
            :height="editorHeight"
            :mode="mode"
            @apply="handleApply"
          />
        </div>

        <!-- 代码编辑器 -->
        <div v-else class="code-editor-wrapper">
          <json-editor
            ref="codeEditorRef"
            v-model="localValue"
            :height="editorHeight"
            :placeholder="placeholder"
            :mode="mode"
            @apply="handleApply"
          />
        </div>
      </div>
    </div>

    <div class="modal-footer">
      <a-space>
        <a-button @click="handleClose">{{ $t('jsonEditor.cancel') }}</a-button>
        <a-button type="primary" :disabled="!isValid" @click="handleApply">
          {{ $t('jsonEditor.confirm') }}
        </a-button>
      </a-space>
    </div>
  </a-modal>
</template>

<script lang="ts">
  export default {
    name: 'JsonTreeEditorModal',
  };
</script>

<script setup lang="ts">
  import { ref, watch, computed, PropType } from 'vue';
  import JsonEditor from './JsonEditor.vue';
  import JsonTreeEditor from './JsonTreeEditor.vue';

  const props = defineProps({
    visible: {
      type: Boolean,
      default: false,
    },
    title: {
      type: String,
      default: '',
    },
    value: {
      type: String,
      default: '',
    },
    width: {
      type: [String, Number],
      default: 800,
    },
    placeholder: {
      type: String,
      default: '',
    },
    fieldKey: {
      type: String,
      default: '',
    },
    mode: {
      type: String as PropType<'object' | 'array'>,
      default: 'object',
    },
  });

  const emit = defineEmits([
    'update:visible',
    'update:value',
    'apply',
    'close',
  ]);

  const localValue = ref(props.value);
  const treeEditorRef = ref();
  const codeEditorRef = ref();
  const editorHeight = ref(500);
  const editorMode = ref<'tree' | 'code'>('tree');

  // 计算值是否有效（可解析为JSON）
  const isValid = computed(() => {
    if (!localValue.value.trim()) return true;

    try {
      const parsed = JSON.parse(localValue.value);

      // 检查解析出的类型是否符合期望
      if (props.mode === 'object' && !isObject(parsed)) {
        return false;
      }

      if (props.mode === 'array' && !Array.isArray(parsed)) {
        return false;
      }

      return true;
    } catch (e) {
      return false;
    }
  });

  // 检查是否是对象
  function isObject(value: any): boolean {
    return value !== null && typeof value === 'object' && !Array.isArray(value);
  }

  // 监听props.value变化
  watch(
    () => props.value,
    (newVal) => {
      if (newVal !== localValue.value) {
        localValue.value = newVal;
      }
    }
  );

  // 监听visible变化
  watch(
    () => props.visible,
    (newVal) => {
      if (newVal) {
        // 当对话框打开时，确保本地值与props同步
        localValue.value = props.value;
      }
    }
  );

  // 处理应用更改
  const handleApply = () => {
    if (isValid.value) {
      emit('apply', {
        key: props.fieldKey,
        value: localValue.value,
      });
      emit('update:value', localValue.value);
      emit('update:visible', false);
    }
  };

  // 处理关闭
  const handleClose = () => {
    emit('update:visible', false);
    emit('close');
  };
</script>

<style scoped>
  .editor-container {
    height: 100%;
    display: flex;
    flex-direction: column;
  }

  .editor-toolbar {
    margin-bottom: 16px;
    display: flex;
    justify-content: center;
  }

  .editor-content {
    flex: 1;
    min-height: 400px;
  }

  .tree-editor-wrapper,
  .code-editor-wrapper {
    height: 100%;
  }

  .modal-footer {
    display: flex;
    justify-content: flex-end;
    margin-top: 16px;
    padding-top: 16px;
    border-top: 1px solid var(--color-border-2);
  }

  :deep(.arco-modal-body) {
    padding: 16px;
    max-height: 80vh;
    overflow: hidden;
    display: flex;
    flex-direction: column;
  }

  :deep(.json-tree-editor-modal) {
    min-height: 600px;
  }
</style>

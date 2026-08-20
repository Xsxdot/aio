<template>
  <div class="json-editor-wrapper">
    <div class="json-editor-toolbar">
      <a-space>
        <a-button
          type="text"
          size="small"
          :disabled="!isValid"
          @click="formatJson"
        >
          <template #icon><icon-code /></template>
          {{ $t('jsonEditor.format') }}
        </a-button>
        <a-button
          type="text"
          size="small"
          :disabled="!isValid"
          @click="compactJson"
        >
          <template #icon><icon-minus /></template>
          {{ $t('jsonEditor.compact') }}
        </a-button>
        <a-button type="text" size="small" @click="copyToClipboard">
          <template #icon><icon-copy /></template>
          {{ $t('jsonEditor.copy') }}
        </a-button>
        <a-tooltip :content="$t('jsonEditor.fullscreen')">
          <a-button type="text" size="small" @click="toggleFullscreen">
            <template #icon>
              <icon-fullscreen-exit v-if="isFullscreen" />
              <icon-fullscreen v-else />
            </template>
          </a-button>
        </a-tooltip>
      </a-space>
      <div v-if="!isValid" class="json-validation-error">
        <icon-exclamation-circle-fill style="color: #f53f3f" />
        {{ $t('jsonEditor.invalidFormat') }}
      </div>
    </div>

    <div class="json-editor-container" :class="{ fullscreen: isFullscreen }">
      <textarea
        ref="editorRef"
        v-model="localValue"
        class="json-editor-textarea"
        :placeholder="placeholder"
        :style="textareaStyle"
        @input="onInput"
        @keydown.tab.prevent="handleTab"
        @keydown="handleKeydown"
      ></textarea>
    </div>

    <div class="json-editor-footer">
      <div class="json-editor-status">
        <span v-if="isValid" class="status-valid">
          <icon-check-circle-fill style="color: #00b42a" />
          {{ $t('jsonEditor.validFormat') }}
        </span>
        <span v-else class="status-invalid">
          <icon-exclamation-circle-fill style="color: #f53f3f" />
          {{ errorMessage }}
        </span>
      </div>
      <a-button
        size="mini"
        type="primary"
        status="success"
        :disabled="!isValid"
        @click="apply"
      >
        {{ $t('jsonEditor.apply') }}
      </a-button>
    </div>
  </div>
</template>

<script lang="ts">
  export default {
    name: 'JsonEditor',
  };
</script>

<script setup lang="ts">
  import { ref, computed, watch, onMounted, nextTick, PropType } from 'vue';
  import { Message } from '@arco-design/web-vue';
  import {
    IconCode,
    IconMinus,
    IconCopy,
    IconCheckCircleFill,
    IconExclamationCircleFill,
    IconFullscreen,
    IconFullscreenExit,
  } from '@arco-design/web-vue/es/icon';
  import { useI18n } from 'vue-i18n';

  const props = defineProps({
    modelValue: {
      type: String,
      required: true,
    },
    height: {
      type: [String, Number],
      default: '300px',
    },
    placeholder: {
      type: String,
      default: '',
    },
    mode: {
      type: String as PropType<'object' | 'array'>,
      default: 'object',
    },
  });

  const emit = defineEmits(['update:modelValue', 'apply', 'close']);

  const { t } = useI18n();

  // 本地编辑状态
  const localValue = ref(props.modelValue);
  const isValid = ref(true);
  const errorMessage = ref('');
  const isFullscreen = ref(false);
  const editorRef = ref<HTMLTextAreaElement | null>(null);

  // 监听prop变化
  watch(
    () => props.modelValue,
    (newVal) => {
      if (newVal !== localValue.value) {
        localValue.value = newVal;
        validateJson();
      }
    }
  );

  // 计算样式
  const textareaStyle = computed(() => {
    let height: string;
    if (isFullscreen.value) {
      height = '100%';
    } else if (typeof props.height === 'number') {
      height = `${props.height}px`;
    } else {
      height = props.height;
    }
    return { height };
  });

  // 输入处理函数
  const onInput = () => {
    validateJson();
    emit('update:modelValue', localValue.value);
  };

  // 格式化JSON
  const formatJson = () => {
    try {
      const parsed = JSON.parse(localValue.value);
      localValue.value = JSON.stringify(parsed, null, 2);
      validateJson();
      emit('update:modelValue', localValue.value);
    } catch (e) {
      // 已经通过 isValid 禁用按钮，这里不需要额外处理
    }
  };

  // 压缩JSON
  const compactJson = () => {
    try {
      const parsed = JSON.parse(localValue.value);
      localValue.value = JSON.stringify(parsed);
      validateJson();
      emit('update:modelValue', localValue.value);
    } catch (e) {
      // 已经通过 isValid 禁用按钮，这里不需要额外处理
    }
  };

  // 复制到剪贴板
  const copyToClipboard = () => {
    if (editorRef.value) {
      try {
        navigator.clipboard
          .writeText(localValue.value)
          .then(() => {
            Message.success(t('jsonEditor.copied'));
          })
          .catch(() => {
            Message.error(t('jsonEditor.copyFailed'));
          });
      } catch (e) {
        // 回退方案
        editorRef.value.select();
        document.execCommand('copy');
        Message.success(t('jsonEditor.copied'));
      }
    }
  };

  // 切换全屏
  const toggleFullscreen = () => {
    isFullscreen.value = !isFullscreen.value;

    // 全屏后聚焦编辑器
    if (isFullscreen.value) {
      nextTick(() => {
        editorRef.value?.focus();
      });
    }
  };

  // 应用更改
  const apply = () => {
    if (isValid.value) {
      emit('apply', localValue.value);

      // 如果是全屏模式，自动退出
      if (isFullscreen.value) {
        isFullscreen.value = false;
      }
    }
  };

  // 关闭编辑器 - 暴露给父组件使用
  const close = () => {
    // 如果是全屏模式，先退出全屏
    if (isFullscreen.value) {
      isFullscreen.value = false;
    }
    emit('close');
  };

  // 验证JSON格式
  const validateJson = () => {
    try {
      if (!localValue.value.trim()) {
        isValid.value = true;
        errorMessage.value = '';
        return;
      }

      const parsed = JSON.parse(localValue.value);

      // 检查解析出的类型是否符合期望
      if (props.mode === 'object' && !isObject(parsed)) {
        isValid.value = false;
        errorMessage.value = t('jsonEditor.shouldBeObject');
        return;
      }

      if (props.mode === 'array' && !Array.isArray(parsed)) {
        isValid.value = false;
        errorMessage.value = t('jsonEditor.shouldBeArray');
        return;
      }

      isValid.value = true;
      errorMessage.value = '';
    } catch (e) {
      isValid.value = false;
      errorMessage.value =
        e instanceof Error ? e.message : t('jsonEditor.invalidFormat');
    }
  };

  // 处理Tab键，插入两个空格
  const handleTab = (e: KeyboardEvent) => {
    if (editorRef.value) {
      const start = editorRef.value.selectionStart;
      const end = editorRef.value.selectionEnd;

      // 在光标处插入两个空格
      const newValue = `${localValue.value.substring(
        0,
        start
      )}  ${localValue.value.substring(end)}`;
      localValue.value = newValue;

      // 光标位置后移两个字符
      nextTick(() => {
        editorRef.value!.selectionStart = start + 2;
        editorRef.value!.selectionEnd = start + 2;
      });

      emit('update:modelValue', localValue.value);
    }
  };

  // 处理键盘事件
  const handleKeydown = (event: KeyboardEvent) => {
    // Esc键退出全屏
    if (event.key === 'Escape' && isFullscreen.value) {
      isFullscreen.value = false;
    }

    // Ctrl+S应用更改
    if (event.key === 's' && (event.ctrlKey || event.metaKey)) {
      event.preventDefault();
      if (isValid.value) {
        apply();
      }
    }
  };

  // 检查是否是对象
  function isObject(value: any): boolean {
    return value !== null && typeof value === 'object' && !Array.isArray(value);
  }

  // 组件加载时进行验证
  onMounted(() => {
    validateJson();
  });

  // 将close方法暴露给父组件
  defineExpose({ close });
</script>

<style scoped>
  .json-editor-wrapper {
    border: 1px solid var(--color-border-2);
    border-radius: 4px;
    overflow: hidden;
    display: flex;
    flex-direction: column;
    width: 100%;
    background-color: var(--color-bg-2);
  }

  .json-editor-toolbar {
    display: flex;
    padding: 8px;
    border-bottom: 1px solid var(--color-border-2);
    background-color: var(--color-bg-1);
    justify-content: space-between;
    align-items: center;
  }

  .json-editor-container {
    flex: 1;
    position: relative;
    min-height: 100px;
  }

  .json-editor-container.fullscreen {
    position: fixed;
    top: 0;
    left: 0;
    right: 0;
    bottom: 0;
    z-index: 1000;
    background-color: var(--color-bg-2);
    padding: 16px;
    box-shadow: 0 0 20px rgba(0, 0, 0, 0.1);
  }

  .json-editor-textarea {
    width: 100%;
    height: 100%;
    border: none;
    outline: none;
    padding: 12px;
    font-family: 'Menlo', 'Monaco', 'Courier New', monospace;
    font-size: 14px;
    line-height: 1.6;
    resize: none;
    background-color: var(--color-bg-2);
    color: var(--color-text-1);
  }

  .json-editor-footer {
    display: flex;
    padding: 8px;
    border-top: 1px solid var(--color-border-2);
    background-color: var(--color-bg-1);
    justify-content: space-between;
    align-items: center;
  }

  .json-editor-status {
    display: flex;
    align-items: center;
    gap: 4px;
    font-size: 12px;
  }

  .status-valid {
    color: #00b42a;
    display: flex;
    align-items: center;
    gap: 4px;
  }

  .status-invalid {
    color: #f53f3f;
    display: flex;
    align-items: center;
    gap: 4px;
  }

  .json-validation-error {
    color: #f53f3f;
    font-size: 12px;
    display: flex;
    align-items: center;
    gap: 4px;
  }

  :deep(.arco-message) {
    z-index: 1100;
  }
</style>

<template>
  <a-drawer
    :width="700"
    :visible="visible"
    :title="isEdit ? $t('shorturl.links.form.title.edit') : $t('shorturl.links.form.title.create')"
    :mask-closable="false"
    unmount-on-close
    @cancel="handleCancel"
  >
    <a-form
      ref="formRef"
      :model="formData"
      :rules="rules"
      layout="vertical"
      @submit="handleSubmit"
    >
      <!-- 目标类型（创建时必填，编辑时不可改） -->
      <a-form-item v-if="!isEdit" field="targetType" :label="$t('shorturl.links.form.targetType')">
        <a-select
          v-model="formData.targetType"
          :placeholder="$t('shorturl.links.form.targetType.placeholder')"
          @change="handleTargetTypeChange"
        >
          <a-option value="URL">{{ $t('shorturl.links.targetType.URL') }}</a-option>
          <a-option value="URL_SCHEME">{{ $t('shorturl.links.targetType.URL_SCHEME') }}</a-option>
        </a-select>
      </a-form-item>

      <!-- 目标配置 -->
      <a-form-item field="targetConfig" :label="$t('shorturl.links.form.targetConfig')">
        <!-- 普通 URL -->
        <a-input
          v-if="formData.targetType === 'URL'"
          v-model="targetConfigUrl"
          :placeholder="$t('shorturl.links.form.targetConfig.url.placeholder')"
          allow-clear
        />
        
        <!-- URL Scheme -->
        <a-space v-else-if="formData.targetType === 'URL_SCHEME'" direction="vertical" :style="{ width: '100%' }">
          <a-input
            v-model="targetConfigSchemeUrl"
            :placeholder="$t('shorturl.links.form.targetConfig.schemeUrl.placeholder')"
            allow-clear
          >
            <template #prepend>{{ $t('shorturl.links.form.targetConfig.schemeUrl') }}</template>
          </a-input>
          <a-input
            v-model="targetConfigFallbackUrl"
            :placeholder="$t('shorturl.links.form.targetConfig.fallbackUrl.placeholder')"
            allow-clear
          >
            <template #prepend>{{ $t('shorturl.links.form.targetConfig.fallbackUrl') }}</template>
          </a-input>
        </a-space>
        
        <!-- 其他类型或直接编辑 JSON -->
        <a-space v-else direction="vertical" :style="{ width: '100%' }">
          <a-button @click="showJsonEditor">
            <template #icon><icon-edit /></template>
            {{ $t('shorturl.links.form.targetConfig.edit') }}
          </a-button>
          <a-textarea
            v-model="targetConfigJsonStr"
            :placeholder="$t('shorturl.links.form.targetConfig.required')"
            :auto-size="{ minRows: 3, maxRows: 6 }"
            readonly
          />
        </a-space>
      </a-form-item>

      <!-- 过期时间 -->
      <a-form-item field="expiresAt" :label="$t('shorturl.links.form.expiresAt')">
        <a-date-picker
          v-model="formData.expiresAt"
          :placeholder="$t('shorturl.links.form.expiresAt.placeholder')"
          show-time
          format="YYYY-MM-DD HH:mm:ss"
          style="width: 100%"
        />
      </a-form-item>

      <!-- 访问密码（仅创建时可设置） -->
      <a-form-item v-if="!isEdit" field="password" :label="$t('shorturl.links.form.password')">
        <a-input-password
          v-model="formData.password"
          :placeholder="$t('shorturl.links.form.password.placeholder')"
          allow-clear
        />
      </a-form-item>

      <!-- 最大访问次数 -->
      <a-form-item field="maxVisits" :label="$t('shorturl.links.form.maxVisits')">
        <a-input-number
          v-model="formData.maxVisits"
          :placeholder="$t('shorturl.links.form.maxVisits.placeholder')"
          :min="1"
          style="width: 100%"
        />
      </a-form-item>

      <!-- 短码生成策略（仅创建时） -->
      <a-form-item v-if="!isEdit" field="codeLength" :label="$t('shorturl.links.form.codeLength')">
        <a-input-number
          v-model="formData.codeLength"
          :placeholder="$t('shorturl.links.form.codeLength.placeholder')"
          :min="4"
          :max="20"
          style="width: 100%"
        />
      </a-form-item>

      <a-form-item v-if="!isEdit" field="customCode" :label="$t('shorturl.links.form.customCode')">
        <a-input
          v-model="formData.customCode"
          :placeholder="$t('shorturl.links.form.customCode.placeholder')"
          allow-clear
        />
      </a-form-item>

      <!-- 备注 -->
      <a-form-item field="comment" :label="$t('shorturl.links.form.comment')">
        <a-textarea
          v-model="formData.comment"
          :placeholder="$t('shorturl.links.form.comment.placeholder')"
          :max-length="500"
          show-word-limit
          allow-clear
        />
      </a-form-item>

      <!-- 编辑模式提示 -->
      <a-alert v-if="isEdit" type="info" style="margin-bottom: 16px">
        {{ $t('shorturl.links.form.editNote') }}
      </a-alert>
    </a-form>

    <template #footer>
      <a-space>
        <a-button @click="handleCancel">
          {{ $t('shorturl.links.form.cancel') }}
        </a-button>
        <a-button type="primary" :loading="submitting" @click="handleSubmit">
          {{ $t('shorturl.links.form.submit') }}
        </a-button>
      </a-space>
    </template>

    <!-- JSON 编辑器弹窗 -->
    <JsonEditorModal
      v-model:visible="jsonEditorVisible"
      :title="$t('shorturl.links.form.targetConfig')"
      :value="targetConfigJsonStr"
      mode="object"
      @apply="handleJsonApply"
    />
  </a-drawer>
</template>

<script lang="ts" setup>
import { ref, reactive, watch, computed } from 'vue';
import { useI18n } from 'vue-i18n';
import type { FormInstance } from '@arco-design/web-vue';
import JsonEditorModal from '@/components/json-editor/JsonEditorModal.vue';
import type { ShortLinkDTO, CreateShortLinkRequest, UpdateShortLinkRequest } from '@/api/shorturl';

const { t } = useI18n();

interface Props {
  visible: boolean;
  domainId: number;
  link?: ShortLinkDTO | null;
}

const props = withDefaults(defineProps<Props>(), {
  visible: false,
  link: null,
});

const emit = defineEmits<{
  (e: 'update:visible', visible: boolean): void;
  (e: 'submit', data: CreateShortLinkRequest | UpdateShortLinkRequest): void;
}>();

const formRef = ref<FormInstance>();
const submitting = ref(false);
const jsonEditorVisible = ref(false);

const isEdit = computed(() => !!props.link);

const formData = reactive<{
  targetType: string;
  targetConfig: Record<string, any>;
  expiresAt?: string;
  password: string;
  maxVisits?: number;
  codeLength: number;
  customCode: string;
  comment: string;
}>({
  targetType: 'URL',
  targetConfig: {},
  expiresAt: undefined,
  password: '',
  maxVisits: undefined,
  codeLength: 6,
  customCode: '',
  comment: '',
});

// 简化的 targetConfig 字段（用于表单双向绑定）
const targetConfigUrl = ref('');
const targetConfigSchemeUrl = ref('');
const targetConfigFallbackUrl = ref('');
const targetConfigJsonStr = ref('{}');

const rules = {
  targetType: [
    {
      required: true,
      message: t('shorturl.links.form.targetType.required'),
    },
  ],
};

// 监听 link 变化，用于编辑模式
watch(
  () => props.link,
  (link) => {
    if (link) {
      formData.targetType = link.targetType;
      formData.targetConfig = link.targetConfig || {};
      formData.expiresAt = link.expiresAt;
      formData.maxVisits = link.maxVisits;
      formData.comment = link.comment || '';
      
      // 解析 targetConfig
      parseTargetConfig(link.targetConfig);
    }
  },
  { immediate: true }
);

// 解析 targetConfig 到表单字段
const parseTargetConfig = (config: Record<string, any>) => {
  if (formData.targetType === 'URL') {
    targetConfigUrl.value = config.url || '';
  } else if (formData.targetType === 'URL_SCHEME') {
    targetConfigSchemeUrl.value = config.schemeUrl || '';
    targetConfigFallbackUrl.value = config.fallbackUrl || '';
  } else {
    targetConfigJsonStr.value = JSON.stringify(config, null, 2);
  }
};

// 目标类型变化
const handleTargetTypeChange = () => {
  formData.targetConfig = {};
  targetConfigUrl.value = '';
  targetConfigSchemeUrl.value = '';
  targetConfigFallbackUrl.value = '';
  targetConfigJsonStr.value = '{}';
};

// 显示 JSON 编辑器
const showJsonEditor = () => {
  // 同步当前数据到 JSON 字符串
  buildTargetConfig();
  targetConfigJsonStr.value = JSON.stringify(formData.targetConfig, null, 2);
  jsonEditorVisible.value = true;
};

// JSON 编辑器应用
const handleJsonApply = ({ value }: { key: string; value: string }) => {
  try {
    const parsed = JSON.parse(value);
    formData.targetConfig = parsed;
    targetConfigJsonStr.value = value;
  } catch (e) {
    console.error('Invalid JSON:', e);
  }
};

// 构建 targetConfig
const buildTargetConfig = () => {
  if (formData.targetType === 'URL') {
    formData.targetConfig = { url: targetConfigUrl.value };
  } else if (formData.targetType === 'URL_SCHEME') {
    formData.targetConfig = {
      schemeUrl: targetConfigSchemeUrl.value,
      fallbackUrl: targetConfigFallbackUrl.value,
    };
  } else {
    try {
      formData.targetConfig = JSON.parse(targetConfigJsonStr.value);
    } catch (e) {
      formData.targetConfig = {};
    }
  }
};

// 重置表单
const resetForm = () => {
  formData.targetType = 'URL';
  formData.targetConfig = {};
  formData.expiresAt = undefined;
  formData.password = '';
  formData.maxVisits = undefined;
  formData.codeLength = 6;
  formData.customCode = '';
  formData.comment = '';
  targetConfigUrl.value = '';
  targetConfigSchemeUrl.value = '';
  targetConfigFallbackUrl.value = '';
  targetConfigJsonStr.value = '{}';
  formRef.value?.clearValidate();
};

// 取消
const handleCancel = () => {
  resetForm();
  emit('update:visible', false);
};

// 提交
const handleSubmit = async () => {
  const errors = await formRef.value?.validate();
  if (errors) {
    return;
  }

  // 构建 targetConfig
  buildTargetConfig();

  submitting.value = true;
  try {
    if (isEdit.value) {
      // 编辑模式：只提交可更新的字段
      const updateData: UpdateShortLinkRequest = {
        targetConfig: formData.targetConfig,
        maxVisits: formData.maxVisits,
        comment: formData.comment || undefined,
      };
      
      // expiresAt 需要转换为 unix 秒
      if (formData.expiresAt) {
        const date = new Date(formData.expiresAt);
        updateData.expiresAt = Math.floor(date.getTime() / 1000);
      }
      
      emit('submit', updateData);
    } else {
      // 新增模式
      const createData: CreateShortLinkRequest = {
        domainId: props.domainId,
        targetType: formData.targetType,
        targetConfig: formData.targetConfig,
        password: formData.password || undefined,
        maxVisits: formData.maxVisits,
        codeLength: formData.codeLength || undefined,
        customCode: formData.customCode || undefined,
        comment: formData.comment || undefined,
      };
      
      // expiresAt 传 ISO 字符串
      if (formData.expiresAt) {
        const date = new Date(formData.expiresAt);
        createData.expiresAt = date.toISOString();
      }
      
      emit('submit', createData);
    }
    resetForm();
    emit('update:visible', false);
  } finally {
    submitting.value = false;
  }
};
</script>

<script lang="ts">
export default {
  name: 'ShortLinkFormDrawer',
};
</script>

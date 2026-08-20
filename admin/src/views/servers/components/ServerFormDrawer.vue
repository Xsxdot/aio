<template>
  <a-drawer
    :width="600"
    :visible="visible"
    :title="isEdit ? $t('servers.form.title.edit') : $t('servers.form.title.create')"
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
      <a-form-item field="name" :label="$t('servers.form.name')">
        <a-input
          v-model="formData.name"
          :placeholder="$t('servers.form.name.placeholder')"
          allow-clear
        />
      </a-form-item>

      <a-form-item field="extranetHost" :label="$t('servers.form.extranetHost')">
        <a-input
          v-model="formData.extranetHost"
          :placeholder="$t('servers.form.extranetHost.placeholder')"
          allow-clear
        />
      </a-form-item>

      <a-form-item field="intranetHost" :label="$t('servers.form.intranetHost')">
        <a-input
          v-model="formData.intranetHost"
          :placeholder="$t('servers.form.intranetHost.placeholder')"
          allow-clear
        />
      </a-form-item>

      <a-form-item field="agentGrpcAddress" :label="$t('servers.form.agentGrpcAddress')">
        <a-input
          v-model="formData.agentGrpcAddress"
          :placeholder="$t('servers.form.agentGrpcAddress.placeholder')"
          allow-clear
        />
      </a-form-item>

      <a-form-item field="enabled" :label="$t('servers.form.enabled')">
        <a-switch v-model="formData.enabled" />
      </a-form-item>

      <a-form-item field="tags" :label="$t('servers.form.tags')">
        <JsonEditorModal
          v-model="formData.tags"
          :placeholder="$t('servers.form.tags.placeholder')"
        />
        <template #help>
          {{ $t('servers.form.tags.help') }}
        </template>
      </a-form-item>

      <a-form-item field="comment" :label="$t('servers.form.comment')">
        <a-textarea
          v-model="formData.comment"
          :placeholder="$t('servers.form.comment.placeholder')"
          :max-length="500"
          show-word-limit
          allow-clear
        />
      </a-form-item>
    </a-form>

    <template #footer>
      <a-space>
        <a-button @click="handleCancel">
          {{ $t('servers.form.cancel') }}
        </a-button>
        <a-button type="primary" :loading="submitting" @click="handleSubmit">
          {{ $t('servers.form.submit') }}
        </a-button>
      </a-space>
    </template>
  </a-drawer>
</template>

<script lang="ts" setup>
import { ref, reactive, watch, computed } from 'vue';
import { useI18n } from 'vue-i18n';
import type { FormInstance } from '@arco-design/web-vue';
import JsonEditorModal from '@/components/json-editor/JsonEditorModal.vue';
import type { ServerDTO, CreateServerRequest, UpdateServerRequest } from '@/api/servers';

const { t } = useI18n();

interface Props {
  visible: boolean;
  server?: ServerDTO | null;
}

const props = withDefaults(defineProps<Props>(), {
  visible: false,
  server: null,
});

const emit = defineEmits<{
  (e: 'update:visible', visible: boolean): void;
  (e: 'submit', data: CreateServerRequest | UpdateServerRequest): void;
}>();

const formRef = ref<FormInstance>();
const submitting = ref(false);

const isEdit = computed(() => !!props.server);

const formData = reactive<{
  name: string;
  extranetHost: string;
  intranetHost: string;
  agentGrpcAddress: string;
  enabled: boolean;
  tags: Record<string, string> | undefined;
  comment: string;
}>({
  name: '',
  extranetHost: '',
  intranetHost: '',
  agentGrpcAddress: '',
  enabled: true,
  tags: undefined,
  comment: '',
});

const rules = {
  name: [
    {
      required: true,
      message: t('servers.form.name.required'),
    },
  ],
  extranetHost: [
    {
      required: true,
      message: t('servers.form.extranetHost.required'),
    },
  ],
};

// 监听 server 变化，用于编辑模式
watch(
  () => props.server,
  (server) => {
    if (server) {
      formData.name = server.name;
      formData.extranetHost = server.extranetHost || server.host;
      formData.intranetHost = server.intranetHost || '';
      formData.agentGrpcAddress = server.agentGrpcAddress || '';
      formData.enabled = server.enabled;
      formData.tags = server.tags;
      formData.comment = server.comment || '';
    }
  },
  { immediate: true }
);

// 重置表单
const resetForm = () => {
  formData.name = '';
  formData.extranetHost = '';
  formData.intranetHost = '';
  formData.agentGrpcAddress = '';
  formData.enabled = true;
  formData.tags = undefined;
  formData.comment = '';
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

  submitting.value = true;
  try {
    if (isEdit.value) {
      // 编辑模式：只提交变更的字段
      const updateData: UpdateServerRequest = {};
      if (formData.name !== props.server!.name) {
        updateData.name = formData.name;
      }
      const serverExtranetHost = props.server!.extranetHost || props.server!.host;
      if (formData.extranetHost !== serverExtranetHost) {
        updateData.extranetHost = formData.extranetHost;
        updateData.host = formData.extranetHost; // 兼容字段，与外网地址保持一致
      }
      if (formData.intranetHost !== (props.server!.intranetHost || '')) {
        updateData.intranetHost = formData.intranetHost || undefined;
      }
      if (formData.agentGrpcAddress !== (props.server!.agentGrpcAddress || '')) {
        updateData.agentGrpcAddress = formData.agentGrpcAddress || undefined;
      }
      if (formData.enabled !== props.server!.enabled) {
        updateData.enabled = formData.enabled;
      }
      // tags 始终提交（因为可能删除了某些键）
      updateData.tags = formData.tags;
      if (formData.comment !== (props.server!.comment || '')) {
        updateData.comment = formData.comment || undefined;
      }
      emit('submit', updateData);
    } else {
      // 新增模式
      const createData: CreateServerRequest = {
        name: formData.name,
        host: formData.extranetHost, // 兼容字段，与外网地址保持一致
        extranetHost: formData.extranetHost,
        intranetHost: formData.intranetHost || undefined,
        agentGrpcAddress: formData.agentGrpcAddress || undefined,
        enabled: formData.enabled,
        tags: formData.tags,
        comment: formData.comment || undefined,
      };
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
  name: 'ServerFormDrawer',
};
</script>

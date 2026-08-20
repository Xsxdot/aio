<template>
  <a-drawer
    v-model:visible="drawerVisible"
    :title="$t('executor.updateArgs.title')"
    :width="520"
    @cancel="handleCancel"
  >
    <a-form
      ref="formRef"
      :model="formData"
      :rules="formRules"
      layout="vertical"
    >
      <a-form-item field="args_json" :label="$t('executor.updateArgs.argsJson')">
        <a-textarea
          v-model="formData.args_json"
          :placeholder="$t('executor.updateArgs.argsJson.placeholder')"
          :auto-size="{ minRows: 6, maxRows: 16 }"
          allow-clear
        />
      </a-form-item>
    </a-form>

    <template #footer>
      <a-space>
        <a-button @click="handleCancel">{{ $t('executor.updateArgs.cancel') }}</a-button>
        <a-button type="primary" :loading="submitting" @click="handleSubmit">
          {{ $t('executor.updateArgs.confirm') }}
        </a-button>
      </a-space>
    </template>
  </a-drawer>
</template>

<script lang="ts" setup>
import { ref, computed, watch } from 'vue';
import { useI18n } from 'vue-i18n';
import type { FormInstance } from '@arco-design/web-vue';

const props = defineProps<{
  visible: boolean;
  initialArgsJson?: string;
}>();

const emit = defineEmits<{
  (e: 'update:visible', visible: boolean): void;
  (e: 'submit', argsJson: string): void;
}>();

const { t } = useI18n();
const formRef = ref<FormInstance>();
const submitting = ref(false);

const drawerVisible = computed({
  get: () => props.visible,
  set: (val) => emit('update:visible', val),
});

const formData = ref({ args_json: '' });

const formRules = {
  args_json: [
    {
      validator: (value: string, callback: (msg?: string) => void) => {
        if (!value) { callback(); return; }
        try { JSON.parse(value); callback(); } catch { callback(t('executor.updateArgs.argsJson.invalid')); }
      },
    },
  ],
};

watch(
  () => props.visible,
  (val) => {
    if (val) {
      formData.value.args_json = props.initialArgsJson || '';
    }
  }
);

const handleCancel = () => {
  drawerVisible.value = false;
};

const handleSubmit = async () => {
  const valid = await formRef.value?.validate();
  if (!valid) {
    try {
      submitting.value = true;
      emit('submit', formData.value.args_json);
    } finally {
      submitting.value = false;
    }
  }
};

const setSubmitting = (val: boolean) => { submitting.value = val; };
const closeDrawer = () => { drawerVisible.value = false; };

defineExpose({ setSubmitting, closeDrawer });
</script>

<style scoped lang="less">
:deep(.arco-form-item) {
  margin-bottom: 16px;
}
</style>

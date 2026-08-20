<template>
  <a-drawer
    :visible="visible"
    :title="$t('admins.resetPassword.title')"
    :width="480"
    :footer="false"
    unmount-on-close
    @cancel="handleCancel"
  >
    <a-alert type="warning" style="margin-bottom: 24px">
      {{ $t('admins.resetPassword.warning', { account: adminAccount }) }}
    </a-alert>

    <a-form
      ref="formRef"
      :model="formData"
      :rules="rules"
      layout="vertical"
      @submit="handleSubmit"
    >
      <a-form-item
        field="newPassword"
        :label="$t('admins.form.newPassword')"
      >
        <a-input-password
          v-model="formData.newPassword"
          :placeholder="$t('admins.form.newPasswordPlaceholder')"
          allow-clear
        />
      </a-form-item>

      <a-form-item
        field="confirmPassword"
        :label="$t('admins.form.confirmPassword')"
      >
        <a-input-password
          v-model="formData.confirmPassword"
          :placeholder="$t('admins.form.confirmPasswordPlaceholder')"
          allow-clear
        />
      </a-form-item>

      <a-form-item>
        <a-space>
          <a-button type="primary" html-type="submit" :loading="loading">
            {{ $t('admins.resetPassword.submit') }}
          </a-button>
          <a-button @click="handleCancel">
            {{ $t('admins.resetPassword.cancel') }}
          </a-button>
        </a-space>
      </a-form-item>
    </a-form>
  </a-drawer>
</template>

<script lang="ts" setup>
import { ref, reactive, watch } from 'vue';
import { useI18n } from 'vue-i18n';
import type { FormInstance, FieldRule } from '@arco-design/web-vue';

const { t } = useI18n();

interface Props {
  visible: boolean;
  adminId?: number;
  adminAccount?: string;
}

interface Emits {
  (e: 'update:visible', value: boolean): void;
  (e: 'submit', adminId: number, newPassword: string): void;
}

const props = defineProps<Props>();
const emit = defineEmits<Emits>();

const formRef = ref<FormInstance>();
const loading = ref(false);

const formData = reactive({
  newPassword: '',
  confirmPassword: '',
});

const validateConfirmPassword = (value: string, callback: (error?: string) => void) => {
  if (value !== formData.newPassword) {
    callback(t('admins.form.passwordMismatch'));
  } else {
    callback();
  }
};

const rules: Record<string, FieldRule[]> = {
  newPassword: [
    { required: true, message: t('admins.form.newPasswordRequired') },
    { min: 6, message: t('admins.form.passwordLength') },
  ],
  confirmPassword: [
    { required: true, message: t('admins.form.confirmPasswordRequired') },
    { validator: validateConfirmPassword },
  ],
};

// 重置表单
const resetForm = () => {
  formData.newPassword = '';
  formData.confirmPassword = '';
  formRef.value?.clearValidate();
};

// 监听 visible 变化，打开时重置表单
watch(() => props.visible, (val) => {
  if (val) {
    resetForm();
  }
});

const handleCancel = () => {
  emit('update:visible', false);
};

const handleSubmit = async () => {
  try {
    const valid = await formRef.value?.validate();
    if (!valid && props.adminId) {
      loading.value = true;
      emit('submit', props.adminId, formData.newPassword);
      // 等待父组件处理完成后关闭
      setTimeout(() => {
        loading.value = false;
        emit('update:visible', false);
      }, 300);
    }
  } catch (error) {
    loading.value = false;
    console.error('Form validation failed:', error);
  }
};
</script>

<style scoped lang="less">
:deep(.arco-form-item) {
  margin-bottom: 24px;
}
</style>







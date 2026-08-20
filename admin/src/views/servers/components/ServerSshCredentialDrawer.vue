<template>
  <a-drawer
    :width="700"
    :visible="visible"
    :title="$t('servers.ssh.title')"
    :mask-closable="false"
    unmount-on-close
    @cancel="handleCancel"
  >
    <a-spin :loading="loading" style="width: 100%">
      <a-form
        ref="formRef"
        :model="formData"
        :rules="rules"
        layout="vertical"
      >
        <a-form-item field="port" :label="$t('servers.ssh.port')">
          <a-input-number
            v-model="formData.port"
            :placeholder="$t('servers.ssh.port.placeholder')"
            :min="1"
            :max="65535"
            style="width: 100%"
          />
        </a-form-item>

        <a-form-item field="username" :label="$t('servers.ssh.username')">
          <a-input
            v-model="formData.username"
            :placeholder="$t('servers.ssh.username.placeholder')"
            allow-clear
          />
        </a-form-item>

        <a-form-item field="authMethod" :label="$t('servers.ssh.authMethod')">
          <a-radio-group v-model="formData.authMethod">
            <a-radio value="password">{{ $t('servers.ssh.authMethod.password') }}</a-radio>
            <a-radio value="privatekey">{{ $t('servers.ssh.authMethod.privatekey') }}</a-radio>
          </a-radio-group>
        </a-form-item>

        <a-form-item
          v-if="formData.authMethod === 'password'"
          field="password"
          :label="$t('servers.ssh.password')"
        >
          <a-input-password
            v-model="formData.password"
            :placeholder="$t('servers.ssh.password.placeholder')"
            allow-clear
          />
          <template v-if="originalCredential?.hasPassword && !formData.password" #help>
            <a-alert type="info" :show-icon="false" style="margin-top: 8px">
              {{ $t('servers.ssh.password.hint') }}
            </a-alert>
          </template>
        </a-form-item>

        <a-form-item
          v-if="formData.authMethod === 'privatekey'"
          field="privateKey"
          :label="$t('servers.ssh.privateKey')"
        >
          <a-textarea
            v-model="formData.privateKey"
            :placeholder="$t('servers.ssh.privateKey.placeholder')"
            :auto-size="{ minRows: 8, maxRows: 16 }"
            allow-clear
          />
          <template v-if="originalCredential?.hasPrivateKey && !formData.privateKey" #help>
            <a-alert type="info" :show-icon="false" style="margin-top: 8px">
              {{ $t('servers.ssh.privateKey.hint') }}
            </a-alert>
          </template>
        </a-form-item>

        <a-form-item field="comment" :label="$t('servers.ssh.comment')">
          <a-textarea
            v-model="formData.comment"
            :placeholder="$t('servers.ssh.comment.placeholder')"
            :max-length="500"
            show-word-limit
            allow-clear
          />
        </a-form-item>
      </a-form>
    </a-spin>

    <template #footer>
      <a-space>
        <a-button @click="handleCancel">
          {{ $t('servers.ssh.cancel') }}
        </a-button>
        <a-popconfirm
          v-if="hasCredential && !loading"
          :content="$t('servers.ssh.confirm.delete')"
          @ok="handleDelete"
        >
          <a-button status="danger" :loading="deleting">
            {{ $t('servers.ssh.delete') }}
          </a-button>
        </a-popconfirm>
        <a-button type="primary" :loading="submitting" :disabled="loading" @click="handleSubmit">
          {{ $t('servers.ssh.submit') }}
        </a-button>
      </a-space>
    </template>
  </a-drawer>
</template>

<script lang="ts" setup>
import { ref, reactive, watch } from 'vue';
import { Message } from '@arco-design/web-vue';
import { useI18n } from 'vue-i18n';
import type { FormInstance } from '@arco-design/web-vue';
import {
  getServerSSHCredential,
  upsertServerSSHCredential,
  deleteServerSSHCredential,
  type ServerSSHCredentialResponse,
  type UpsertServerSSHCredentialRequest,
} from '@/api/servers';

const { t } = useI18n();

interface Props {
  visible: boolean;
  serverId?: number;
}

const props = withDefaults(defineProps<Props>(), {
  visible: false,
  serverId: undefined,
});

const emit = defineEmits<{
  (e: 'update:visible', visible: boolean): void;
  (e: 'success'): void;
}>();

const formRef = ref<FormInstance>();
const loading = ref(false);
const submitting = ref(false);
const deleting = ref(false);
const hasCredential = ref(false);
const originalCredential = ref<ServerSSHCredentialResponse | null>(null);

const formData = reactive<{
  port: number;
  username: string;
  authMethod: string;
  password: string;
  privateKey: string;
  comment: string;
}>({
  port: 22,
  username: '',
  authMethod: 'password',
  password: '',
  privateKey: '',
  comment: '',
});

const rules = {
  port: [
    {
      required: true,
      message: t('servers.ssh.port.required'),
    },
  ],
  username: [
    {
      required: true,
      message: t('servers.ssh.username.required'),
    },
  ],
  password: [
    {
      validator: (value: string, callback: (error?: string) => void) => {
        if (formData.authMethod === 'password') {
          // 如果原来有密码且当前没输入，允许（保持原值）
          if (originalCredential.value?.hasPassword && !value) {
            callback();
            return;
          }
          // 否则必须输入
          if (!value) {
            callback(t('servers.ssh.password.required'));
            return;
          }
        }
        callback();
      },
    },
  ],
  privateKey: [
    {
      validator: (value: string, callback: (error?: string) => void) => {
        if (formData.authMethod === 'privatekey') {
          // 如果原来有私钥且当前没输入，允许（保持原值）
          if (originalCredential.value?.hasPrivateKey && !value) {
            callback();
            return;
          }
          // 否则必须输入
          if (!value) {
            callback(t('servers.ssh.privateKey.required'));
            return;
          }
        }
        callback();
      },
    },
  ],
};

// 加载凭证
const loadCredential = async () => {
  if (!props.serverId) return;

  loading.value = true;
  try {
    const res = await getServerSSHCredential(props.serverId);
    if (res.data) {
      originalCredential.value = res.data;
      hasCredential.value = true;
      formData.port = res.data.port;
      formData.username = res.data.username;
      formData.authMethod = res.data.authMethod;
      formData.comment = res.data.comment;
      // 不加载密码和私钥（已加密，无法回显）
      formData.password = '';
      formData.privateKey = '';
    } else {
      hasCredential.value = false;
      resetForm();
    }
  } catch (err) {
    Message.error(t('servers.ssh.message.load.failed'));
    console.error(err);
  } finally {
    loading.value = false;
  }
};

// 监听 visible 变化
watch(
  () => props.visible,
  (visible) => {
    if (visible && props.serverId) {
      loadCredential();
    }
  }
);

// 重置表单
const resetForm = () => {
  formData.port = 22;
  formData.username = '';
  formData.authMethod = 'password';
  formData.password = '';
  formData.privateKey = '';
  formData.comment = '';
  originalCredential.value = null;
  formRef.value?.clearValidate();
};

// 取消
const handleCancel = () => {
  resetForm();
  hasCredential.value = false;
  emit('update:visible', false);
};

// 提交
const handleSubmit = async () => {
  if (!props.serverId) return;

  const errors = await formRef.value?.validate();
  if (errors) {
    return;
  }

  submitting.value = true;
  try {
    const data: UpsertServerSSHCredentialRequest = {
      port: formData.port,
      username: formData.username,
      authMethod: formData.authMethod,
      comment: formData.comment,
    };

    // 根据认证方式提交对应字段
    if (formData.authMethod === 'password') {
      // 如果有输入新密码，提交；否则后端保持原值
      if (formData.password) {
        data.password = formData.password;
      }
    } else if (formData.authMethod === 'privatekey') {
      // 如果有输入新私钥，提交；否则后端保持原值
      if (formData.privateKey) {
        data.privateKey = formData.privateKey;
      }
    }

    await upsertServerSSHCredential(props.serverId, data);
    Message.success(t('servers.ssh.message.save.success'));
    emit('success');
    handleCancel();
  } catch (err) {
    Message.error(t('servers.ssh.message.save.failed'));
    console.error(err);
  } finally {
    submitting.value = false;
  }
};

// 删除
const handleDelete = async () => {
  if (!props.serverId) return;

  deleting.value = true;
  try {
    await deleteServerSSHCredential(props.serverId);
    Message.success(t('servers.ssh.message.delete.success'));
    emit('success');
    handleCancel();
  } catch (err) {
    Message.error(t('servers.ssh.message.delete.failed'));
    console.error(err);
  } finally {
    deleting.value = false;
  }
};
</script>

<script lang="ts">
export default {
  name: 'ServerSshCredentialDrawer',
};
</script>

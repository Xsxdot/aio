<template>
  <div class="config-center-container">
    <div class="config-layout">
      <!-- 左侧面板：配置项列表 -->
      <div class="config-sidebar">
        <a-card class="sidebar-card" :bordered="false">
          <template #title>
            <div class="sidebar-header">
              <span class="sidebar-title">{{ $t('config.list.title') }}</span>
              <a-space>
                <a-tooltip :content="$t('config.export')">
                  <a-button
                    type="primary"
                    size="small"
                    @click="handleExportConfig"
                  >
                    <template #icon><icon-download /></template>
                  </a-button>
                </a-tooltip>
                <a-tooltip :content="$t('config.import')">
                  <a-button
                    type="primary"
                    status="warning"
                    size="small"
                    @click="handleImportConfig"
                  >
                    <template #icon><icon-upload /></template>
                  </a-button>
                </a-tooltip>
                <a-tooltip :content="$t('config.create')">
                  <a-button
                    type="primary"
                    size="small"
                    status="success"
                    @click="handleCreateConfig"
                  >
                    <template #icon><icon-plus /></template>
                  </a-button>
                </a-tooltip>
                <a-tooltip :content="$t('config.refresh')">
                  <a-button size="small" @click="refreshConfigList">
                    <template #icon><icon-refresh /></template>
                  </a-button>
                </a-tooltip>
              </a-space>
            </div>
          </template>

          <a-input-search
            v-model="searchKey"
            :placeholder="$t('config.search.placeholder')"
            class="search-input"
            allow-clear
            @search="handleSearch"
          />

          <a-divider style="margin: 12px 0" />

          <a-spin :loading="loading" class="full-height">
            <div class="env-filter-bar">
              <a-radio-group
                v-model="envFilter"
                type="button"
                size="small"
                @change="handleEnvFilterChange"
              >
                <a-radio value="all">{{ $t('config.env.all') }}</a-radio>
                <a-radio v-for="env in environments" :key="env" :value="env">{{
                  env
                }}</a-radio>
              </a-radio-group>
            </div>

            <div class="config-tree">
              <a-tree
                v-if="configTreeData.length > 0"
                :data="configTreeData"
                :default-expanded-keys="['root']"
                :expanded-keys="expandedKeys"
                :show-line="true"
                :draggable="false"
                size="medium"
                @select="handleSelectConfig"
                @expand="handleExpandNode"
              >
                <template #icon="nodeData">
                  <icon-folder v-if="!nodeData.isLeaf" />
                  <icon-file v-else-if="nodeData.configEnv === 'dev'" :style="{ color: '#1890ff' }" />
                  <icon-file v-else-if="nodeData.configEnv === 'test'" :style="{ color: '#52c41a' }" />
                  <icon-file v-else-if="nodeData.configEnv === 'staging'" :style="{ color: '#faad14' }" />
                  <icon-file v-else-if="nodeData.configEnv === 'prod'" :style="{ color: '#f5222d' }" />
                  <icon-file v-else />
                </template>
                <template #title="nodeData">
                  <span
                    :class="{
                      'selected-node': nodeData.configId === currentConfigId && nodeData.env === currentEnv,
                    }"
                  >
                    {{ nodeData.title }}
                  </span>
                  <a-tag
                    v-if="nodeData.configEnv && nodeData.isLeaf"
                    :color="getEnvColor(nodeData.configEnv)"
                    size="small"
                    style="margin-left: 8px"
                  >
                    {{ nodeData.configEnv }}
                  </a-tag>
                </template>
              </a-tree>
            </div>

            <a-empty
              v-if="!loading && configTreeData.length === 0"
              :description="$t('config.empty')"
            />
          </a-spin>
        </a-card>
      </div>

      <!-- 右侧面板：配置详情与编辑 -->
      <div class="config-content">
        <a-card class="content-card" :bordered="false">
          <template #title>
            <div class="content-header">
              <div style="display: flex; align-items: center; gap: 12px; flex: 1;">
                <span class="content-title">
                  {{
                    currentConfigItem
                      ? `${$t('config.detail.title')}: ${currentBaseKey || parseKeyEnv(currentConfigItem.key).baseKey}`
                      : $t('config.detail.title')
                  }}
                </span>
                <a-radio-group
                  v-if="currentConfigItem && currentBaseKey && availableEnvsForCurrentBase.length > 1"
                  v-model="currentEnv"
                  type="button"
                  size="small"
                  @change="handleSwitchEnv"
                >
                  <a-radio
                    v-for="env in availableEnvsForCurrentBase"
                    :key="env"
                    :value="env"
                  >
                    {{ env }}
                  </a-radio>
                </a-radio-group>
                <a-tag v-else-if="currentConfigItem && currentEnv" :color="getEnvColor(currentEnv)">
                  {{ currentEnv }}
                </a-tag>
              </div>
              <a-space v-if="currentConfigItem">
                <a-button
                  type="primary"
                  status="danger"
                  size="small"
                  @click="handleDeleteConfig"
                >
                  <template #icon><icon-delete /></template>
                </a-button>
              </a-space>
            </div>
          </template>

          <a-spin :loading="configLoading" class="full-height">
            <a-empty
              v-if="!currentConfigItem"
              :description="$t('config.selectTip')"
            />
            <div v-else class="config-detail-container">
              <a-tabs default-active-key="detail" type="card-gutter">
                <!-- 基本信息标签页 -->
                <a-tab-pane key="detail" :title="$t('config.tabs.detail')">
                  <div class="info-card">
                    <a-descriptions
                      :data="configDetailData"
                      :column="{ xs: 1, sm: 2, md: 3 }"
                      :label-style="{ 'font-weight': 'bold' }"
                      size="medium"
                      bordered
                      layout="inline-horizontal"
                    />
                  </div>
                </a-tab-pane>

                <!-- 配置值编辑标签页 -->
                <a-tab-pane key="edit" :title="$t('config.tabs.edit')">
                  <config-value-editor
                    v-if="currentConfigItem"
                    :config-item="currentConfigItem"
                    @update="handleUpdateConfig"
                  />
                </a-tab-pane>

                <!-- 版本历史标签页 -->
                <a-tab-pane key="history" :title="$t('config.tabs.history')">
                  <config-history
                    v-if="currentConfigItem"
                    :config-id="currentConfigItem.id"
                    @refresh="fetchConfigDetail"
                  />
                </a-tab-pane>

                <!-- 环境对比标签页 -->
                <a-tab-pane
                  key="env-compare"
                  :title="$t('config.tabs.envCompare')"
                >
                  <config-env-compare
                    v-if="currentConfigItem && currentBaseKey"
                    :base-key="currentBaseKey"
                    :available-envs="availableCompareEnvsForCurrentBase"
                    :current-env="currentEnv"
                    @refresh="handleUpdateConfig"
                  />
                </a-tab-pane>
              </a-tabs>
            </div>
          </a-spin>
        </a-card>
      </div>
    </div>

    <!-- 新建配置对话框 -->
    <a-modal
      v-model:visible="createModalVisible"
      :title="$t('config.create')"
      @before-ok="handleSubmitCreate"
      @cancel="resetCreateForm"
    >
      <a-form
        ref="createFormRef"
        :model="createForm"
        label-align="left"
        :label-col-props="{ span: 6 }"
        :wrapper-col-props="{ span: 18 }"
      >
        <a-form-item
          field="baseKey"
          :label="$t('config.field.baseKey')"
          :rules="[
            { required: true, message: $t('config.validation.baseKeyRequired') },
          ]"
        >
          <a-input
            v-model="createForm.baseKey"
            :placeholder="$t('config.field.baseKeyPlaceholder')"
            allow-clear
          />
        </a-form-item>
        <a-form-item
          field="env"
          :label="$t('config.field.env')"
          :rules="[
            { required: true, message: $t('config.validation.envRequired') },
          ]"
        >
          <a-select
            v-model="createForm.env"
            :placeholder="$t('config.field.envPlaceholder')"
          >
            <a-option v-for="env in environments" :key="env" :value="env">
              {{ env }}
            </a-option>
          </a-select>
        </a-form-item>
        <a-form-item
          field="description"
          :label="$t('config.field.description')"
        >
          <a-input
            v-model="createForm.description"
            :placeholder="$t('config.field.descriptionPlaceholder')"
            allow-clear
          />
        </a-form-item>
        <a-divider orientation="left">{{ $t('config.create.initialField') }}</a-divider>
        <a-form-item
          field="initialFieldName"
          :label="$t('config.field.initialFieldName')"
          :rules="[
            { required: true, message: $t('config.validation.fieldNameRequired') },
          ]"
        >
          <a-input
            v-model="createForm.initialFieldName"
            :placeholder="$t('config.field.fieldNamePlaceholder')"
            allow-clear
          />
        </a-form-item>
        <a-form-item
          field="initialType"
          :label="$t('config.field.initialType')"
          :rules="[
            { required: true, message: $t('config.validation.typeRequired') },
          ]"
        >
          <a-select
            v-model="createForm.initialType"
            :placeholder="$t('config.field.typePlaceholder')"
          >
            <a-option value="string">{{
              $t('config.valueType.string')
            }}</a-option>
            <a-option value="encrypted">{{
              $t('config.valueType.encrypted')
            }}</a-option>
            <a-option value="int">{{ $t('config.valueType.int') }}</a-option>
            <a-option value="float">{{
              $t('config.valueType.float')
            }}</a-option>
            <a-option value="bool">{{ $t('config.valueType.bool') }}</a-option>
            <a-option value="ref">{{ $t('config.valueType.ref') }}</a-option>
            <a-option value="object">{{
              $t('config.valueType.object')
            }}</a-option>
            <a-option value="array">{{
              $t('config.valueType.array')
            }}</a-option>
          </a-select>
        </a-form-item>
        <a-form-item
          field="initialValue"
          :label="$t('config.field.initialValue')"
          :rules="[
            { required: true, message: $t('config.validation.valueRequired') },
          ]"
        >
          <a-textarea
            v-model="createForm.initialValue"
            :placeholder="$t('config.field.valuePlaceholder')"
            :auto-size="{ minRows: 2, maxRows: 6 }"
            allow-clear
          />
        </a-form-item>
      </a-form>
    </a-modal>

    <!-- 导出配置对话框 -->
    <a-modal
      v-model:visible="exportModalVisible"
      :title="$t('config.export')"
      @before-ok="handleSubmitExport"
    >
      <a-form
        ref="exportFormRef"
        :model="exportForm"
        label-align="left"
        :label-col-props="{ span: 6 }"
        :wrapper-col-props="{ span: 18 }"
      >
        <a-form-item
          field="environment"
          :label="$t('config.field.environment')"
        >
          <a-select
            v-model="exportForm.environment"
            :placeholder="$t('config.field.envPlaceholder')"
            allow-clear
          >
            <a-option value="">{{ $t('config.env.all') }}</a-option>
            <a-option v-for="env in environments" :key="env" :value="env">
              {{ env }}
            </a-option>
          </a-select>
        </a-form-item>
        <a-form-item field="targetSalt" :label="$t('config.field.targetSalt')">
          <a-input
            v-model="exportForm.targetSalt"
            :placeholder="$t('config.field.targetSaltPlaceholder')"
            allow-clear
          />
        </a-form-item>
        <div class="export-hint">
          <a-alert type="info">
            {{ $t('config.exportHint') }}
          </a-alert>
        </div>
      </a-form>
    </a-modal>

    <!-- 导入配置对话框 -->
    <a-modal
      v-model:visible="importModalVisible"
      :title="$t('config.import')"
      @before-ok="handleSubmitImport"
    >
      <a-form
        ref="importFormRef"
        :model="importForm"
        label-align="left"
        :label-col-props="{ span: 6 }"
        :wrapper-col-props="{ span: 18 }"
      >
        <a-form-item
          field="file"
          :label="$t('config.field.file')"
          :rules="[
            { required: true, message: $t('config.validation.fileRequired') },
          ]"
        >
          <a-upload
            :file-list="fileList"
            :limit="1"
            action="/"
            :auto-upload="false"
            :show-file-list="true"
            accept=".json"
            @change="handleFileChange"
          >
            <template #upload-button>
              <a-button>{{ $t('config.selectFile') }}</a-button>
            </template>
          </a-upload>
        </a-form-item>
        <a-form-item field="sourceSalt" :label="$t('config.field.sourceSalt')">
          <a-input
            v-model="importForm.sourceSalt"
            :placeholder="$t('config.field.sourceSaltPlaceholder')"
            allow-clear
          />
        </a-form-item>
        <a-form-item field="overWrite" :label="$t('config.field.overWrite')">
          <a-radio-group v-model="importForm.overWrite">
            <a-radio :value="false">{{ $t('config.overWrite.skip') }}</a-radio>
            <a-radio :value="true">{{
              $t('config.overWrite.overwrite')
            }}</a-radio>
          </a-radio-group>
        </a-form-item>
        <div class="import-hint">
          <a-alert type="warning">
            {{ $t('config.importHint') }}
          </a-alert>
        </div>
      </a-form>
    </a-modal>
  </div>
</template>

<script lang="ts" setup>
  import { ref, reactive, computed, onMounted } from 'vue';
  import { Message } from '@arco-design/web-vue';
  import {
    IconPlus,
    IconRefresh,
    IconDelete,
    IconFile,
    IconFolder,
    IconDownload,
    IconUpload,
  } from '@arco-design/web-vue/es/icon';
  import {
    queryConfigs,
    getConfigById,
    createConfig,
    deleteConfig,
    exportConfigs,
    importConfigs,
    parseConfigItem,
    type ConfigItem,
    type ParsedConfigItem,
    type ConfigValue,
  } from '@/api/config';
  import ConfigValueEditor from './components/ConfigValueEditor.vue';
  import ConfigHistory from './components/ConfigHistory.vue';
  import ConfigEnvCompare from './components/ConfigEnvCompare.vue';
  import { parseKeyEnv, buildConfigKey } from './utils';

  // 状态变量
  const loading = ref(false);
  const configLoading = ref(false);
  const searchKey = ref('');
  const envFilter = ref('all');
  const configList = ref<ConfigItem[]>([]);
  const currentConfigId = ref<number | null>(null);
  const currentConfigItem = ref<ParsedConfigItem | null>(null);
  const currentEnv = ref('dev');
  const currentBaseKey = ref(''); // 当前baseKey（不含env后缀）
  const createModalVisible = ref(false);
  const exportModalVisible = ref(false);
  const importModalVisible = ref(false);
  const createFormRef = ref();
  const exportFormRef = ref();
  const importFormRef = ref();
  const fileList = ref([]);
  const expandedKeys = ref<(string | number)[]>(['root']); // 树展开的节点

  // 预设环境列表（包含global用于UI展示）
  const presetEnvironments = ['dev', 'test', 'staging', 'prod', 'global'];

  // 动态环境列表（从配置key中提取 + 预设）
const environments = computed(() => {
  const envSet = new Set(presetEnvironments);
  
  // 从配置列表的key后缀中提取环境
  configList.value.forEach((item) => {
    const { env } = parseKeyEnv(item.key);
    if (env) envSet.add(env);
  });
  
  return Array.from(envSet).sort();
});
  
  // 分页
  const pagination = reactive({
    page: 1,
    pageSize: 10,
  });
  const total = ref(0);

  // 创建表单
  const createForm = reactive({
    baseKey: '',
    env: 'dev',
    description: '',
    initialFieldName: '',
    initialType: 'string' as any,
    initialValue: '',
  });

  // 导出表单
  const exportForm = reactive({
    environment: '',
    targetSalt: '',
  });

  // 导入表单
  const importForm = reactive({
    sourceSalt: '',
    file: null as File | null,
    overWrite: false,
  });

  // 当前显示的配置列表（前端过滤：搜索 + 环境）
  const displayedConfigs = computed(() => {
    let filtered = configList.value;

    // 搜索过滤
    if (searchKey.value) {
      filtered = filtered.filter((item) => {
        const key = item.key?.toLowerCase() || '';
        const desc = item.description?.toLowerCase() || '';
        const search = searchKey.value.toLowerCase();
        return key.includes(search) || desc.includes(search);
      });
    }

    // 环境过滤（基于key后缀）
    if (envFilter.value !== 'all') {
      filtered = filtered.filter((item) => {
        const { env } = parseKeyEnv(item.key);
        return env === envFilter.value;
      });
    }

    return filtered;
  });

  // 配置树数据（按baseKey分组，env作为叶子节点）
  const configTreeData = computed(() => {
    if (!displayedConfigs.value || displayedConfigs.value.length === 0) {
      return [];
    }

    // 构建配置树
    const root = {
      key: 'root',
      title: '根目录',
      selectable: false,
      children: [] as any[],
    };

    // 按baseKey分组配置项
    const baseKeyMap = new Map<string, any[]>();
    
    displayedConfigs.value.forEach((item) => {
      if (!item || !item.key) return;
      
      const { baseKey, env } = parseKeyEnv(item.key);
      if (!baseKeyMap.has(baseKey)) {
        baseKeyMap.set(baseKey, []);
      }
      baseKeyMap.get(baseKey)!.push({ ...item, env });
    });

    // 构建树结构
    baseKeyMap.forEach((configs, baseKey) => {
      const parts = baseKey.split('.');
      let currentLevel = root.children;
      let currentPath = '';

      for (let i = 0; i < parts.length; i += 1) {
        const part = parts[i];
        const newPath = currentPath ? `${currentPath}.${part}` : part;
        currentPath = newPath;

        // 查找当前级别是否已存在此节点
        const pathKey = `path:${newPath}`;
        let node = currentLevel.find((n: any) => n.key === pathKey);

        if (!node) {
          // 创建新节点
          node = {
            key: `path:${currentPath}`,
            title: part,
            children: [],
            selectable: true, // 所有节点都可点击
          };

          // 如果是叶子节点（baseKey节点），为其创建环境子节点
          if (i === parts.length - 1) {
            node.baseKey = baseKey;
            node.isBaseKeyNode = true;
            
            // 添加环境子节点
            configs.forEach((config) => {
              node.children.push({
                key: `config:${config.id}:${config.env}`,
                title: config.env,
                configId: config.id,
                env: config.env,
                fullKey: config.key,
                baseKey,
                isLeaf: true,
                selectable: true,
                configEnv: config.env, // 用于样式和图标
              });
            });
          }

          currentLevel.push(node);
        }

        currentLevel = node.children;
      }
    });

    return [root];
  });

  // 当前baseKey下可用的环境列表（用于右上角环境切换，仅显示已存在的环境）
  const availableEnvsForCurrentBase = computed(() => {
    if (!currentBaseKey.value) return [];
    
    const envs = new Set<string>();
    configList.value.forEach((item) => {
      const { baseKey, env } = parseKeyEnv(item.key);
      if (baseKey === currentBaseKey.value) {
        envs.add(env);
      }
    });
    
    return Array.from(envs).sort();
  });

  // 当前baseKey下可用于环境对比的环境列表（固定展示 dev/test/staging/prod，并在存在 global 配置时追加 global）
  const availableCompareEnvsForCurrentBase = computed(() => {
    // 基础列表：固定的环境
    const baseEnvs = ['dev', 'test', 'staging', 'prod'];
    
    // 检查是否需要追加 global
    // 规则1：当前环境本身是 global
    // 规则2：该 baseKey 下存在 global 配置
    const shouldIncludeGlobal = currentEnv.value === 'global' || 
      configList.value.some((item) => {
        const { baseKey, env } = parseKeyEnv(item.key);
        return baseKey === currentBaseKey.value && env === 'global';
      });
    
    if (shouldIncludeGlobal) {
      // 将 global 放在最前面
      return ['global', ...baseEnvs];
    }
    
    return baseEnvs;
  });

  // 配置详情数据
  const configDetailData = computed(() => {
    if (!currentConfigItem.value) return [];

    const item = currentConfigItem.value;
    const { baseKey, env } = parseKeyEnv(item.key);

    return [
      {
        label: 'Base Key',
        value: baseKey || '',
      },
      {
        label: 'Environment',
        value: env || '-',
      },
      {
        label: 'Full Key',
        value: item.key || '',
      },
      {
        label: 'Version',
        value: item.version ? String(item.version) : '-',
      },
      {
        label: 'Description',
        value: item.description || item.metadata?.description || '-',
      },
      {
        label: 'Updated At',
        value: item.updatedAt ? new Date(item.updatedAt).toLocaleString() : '-',
      },
      {
        label: 'Created At',
        value: item.createdAt ? new Date(item.createdAt).toLocaleString() : '-',
      },
    ];
  });

  // 刷新配置列表（全量拉取）
  async function refreshConfigList() {
    try {
      loading.value = true;
      const allConfigs: ConfigItem[] = [];
      let pageNum = 1;
      const pageSize = 200; // 每页拉取200条
      let totalCount = 0;
      let hasMore = true;

      // 循环分页拉取所有配置项
      // eslint-disable-next-line no-await-in-loop
      while (hasMore) {
        // eslint-disable-next-line no-await-in-loop
        const res = await queryConfigs({
          key: searchKey.value || undefined,
          environment: envFilter.value === 'all' ? undefined : envFilter.value,
          page: pageNum,
          pageSize,
        });

        const content = res.data.content || [];
        allConfigs.push(...content);

        if (pageNum === 1) {
          totalCount = res.data.total || 0;
        }

        // 如果已经获取全部数据，退出循环
        if (allConfigs.length >= totalCount || content.length < pageSize) {
          hasMore = false;
        } else {
          pageNum += 1;
        }
      }

      configList.value = allConfigs;
      total.value = totalCount;
    } catch (error) {
      Message.error('获取配置列表失败');
      configList.value = [];
      total.value = 0;
    } finally {
      loading.value = false;
    }
  }

  // 获取配置详情
  async function fetchConfigDetail() {
    if (!currentConfigId.value) return;

    try {
      configLoading.value = true;
      const res = await getConfigById(currentConfigId.value);
      currentConfigItem.value = parseConfigItem(res.data);
    } catch (error) {
      Message.error('获取配置详情失败');
    } finally {
      configLoading.value = false;
    }
  }

  // 搜索配置
  function handleSearch() {
    refreshConfigList();
  }

  // 选择配置项（树节点点击）
  async function handleSelectConfig(
    selectedKeys: (string | number)[],
    data: {
      selected?: boolean;
      selectedNodes: any[];
      node?: any;
      e?: Event;
    }
  ) {
    const { node } = data;

    if (!node) return;

    // 处理环境叶子节点选择（直接打开配置详情）
    if (node.configId && node.env) {
      currentConfigId.value = node.configId;
      currentEnv.value = node.env;
      currentBaseKey.value = node.baseKey || parseKeyEnv(node.fullKey || '').baseKey;
      await fetchConfigDetail();
      return;
    }

    // 处理baseKey节点点击（自动选择默认环境）
    if (node.isBaseKeyNode && node.baseKey) {
      const nodeKey = node.key as string | number;
      
      // 切换展开/折叠状态
      if (expandedKeys.value.includes(nodeKey)) {
        expandedKeys.value = expandedKeys.value.filter((key) => key !== nodeKey);
      } else {
        expandedKeys.value = [...new Set([...expandedKeys.value, nodeKey])];
      }
      
      // 查找该baseKey下的所有配置，选择默认环境
      const configs = displayedConfigs.value.filter(item => {
        const { baseKey } = parseKeyEnv(item.key);
        return baseKey === node.baseKey;
      });
      
      if (configs.length > 0) {
        // 默认环境策略：优先 global，否则按env名排序取第一个
        const globalConfig = configs.find(c => parseKeyEnv(c.key).env === 'global');
        const defaultConfig = globalConfig || configs.sort((a, b) => {
          const envA = parseKeyEnv(a.key).env;
          const envB = parseKeyEnv(b.key).env;
          return envA.localeCompare(envB);
        })[0];
        
        const { env } = parseKeyEnv(defaultConfig.key);
        currentConfigId.value = defaultConfig.id;
        currentEnv.value = env;
        currentBaseKey.value = node.baseKey;
        await fetchConfigDetail();
      }
      return;
    }

    // 处理目录节点点击（只切换展开/折叠）
    const nodeKey = node.key as string | number;
    if (expandedKeys.value.includes(nodeKey)) {
      expandedKeys.value = expandedKeys.value.filter((key) => key !== nodeKey);
    } else {
      expandedKeys.value = [...new Set([...expandedKeys.value, nodeKey])];
    }
  }

  // 处理树节点展开/折叠
  function handleExpandNode(
    keys: (string | number)[],
    data: {
      expanded?: boolean;
      expandedNodes: any[];
      node?: any;
      e?: Event;
    }
  ) {
    if (!data.node || data.expanded === undefined) return;

    const nodeKey = data.node.key;

    if (data.expanded) {
      // 展开节点
      expandedKeys.value = [...new Set([...expandedKeys.value, nodeKey])];
    } else {
      // 折叠节点
      expandedKeys.value = expandedKeys.value.filter((key) => key !== nodeKey);
    }
  }

  // 环境过滤变化（将改为前端过滤）
  function handleEnvFilterChange() {
    // 前端过滤，不需要重新请求
  }

  // 切换环境
  async function handleSwitchEnv(env: string) {
    if (!currentBaseKey.value) return;
    
    // 查找对应环境的配置（使用buildConfigKey确保global不加后缀）
    const targetKey = buildConfigKey(currentBaseKey.value, env);
    const targetConfig = configList.value.find((item) => item.key === targetKey);
    
    if (targetConfig) {
      currentConfigId.value = targetConfig.id;
      currentEnv.value = env;
      await fetchConfigDetail();
    } else {
      Message.warning(`未找到环境 ${env} 的配置`);
    }
  }

  // 获取环境颜色
  function getEnvColor(env: string) {
    switch (env) {
      case 'dev':
        return 'blue';
      case 'test':
        return 'green';
      case 'staging':
        return 'orange';
      case 'prod':
        return 'red';
      case 'global':
        return 'purple';
      default:
        return 'gray';
    }
  }

  // 生命周期钩子
  onMounted(() => {
    refreshConfigList();
  });

  // 新建配置
  function handleCreateConfig() {
    createModalVisible.value = true;
  }

  // 重置创建表单
  function resetCreateForm() {
    createForm.baseKey = '';
    createForm.env = 'dev';
    createForm.description = '';
    createForm.initialFieldName = '';
    createForm.initialType = 'string';
    createForm.initialValue = '';
    createFormRef.value?.clearValidate();
  }

  // 提交创建
  async function handleSubmitCreate() {
    try {
      await createFormRef.value?.validate();

      // 序列化初始值（根据类型）
      let serializedValue = createForm.initialValue;
      if (createForm.initialType === 'int' || createForm.initialType === 'float') {
        const numVal = parseFloat(createForm.initialValue);
        serializedValue = JSON.stringify(numVal);
      } else if (createForm.initialType === 'bool') {
        const boolVal = createForm.initialValue.toLowerCase() === 'true';
        serializedValue = JSON.stringify(boolVal);
      } else if (createForm.initialType === 'object' || createForm.initialType === 'array') {
        // 验证JSON
        try {
          JSON.parse(createForm.initialValue);
        } catch {
          Message.error('初始值的JSON格式不正确');
          return false;
        }
      }

      const value: Record<string, ConfigValue> = {
        [createForm.initialFieldName]: {
          value: serializedValue,
          type: createForm.initialType,
        },
      };

      // 组装完整key（使用buildConfigKey确保global不加后缀）
      const fullKey = buildConfigKey(createForm.baseKey, createForm.env);

      await createConfig({
        key: fullKey,
        value,
        description: createForm.description,
        changeNote: '初始创建',
      });

      Message.success('创建成功');
      createModalVisible.value = false;
      resetCreateForm();
      refreshConfigList();
      return true;
    } catch (error: any) {
      if (error.errors) {
        return false; // 表单验证错误
      }
      Message.error(`创建失败: ${error.message || '未知错误'}`);
      return false;
    }
  }

  // 更新配置（来自编辑器）
  async function handleUpdateConfig() {
    await fetchConfigDetail();
    await refreshConfigList();
  }

  // 删除配置
  function handleDeleteConfig() {
    if (!currentConfigItem.value) return;

    Message.warning({
      content: `确定要删除配置 ${currentConfigItem.value.key} 吗？此操作不可撤销。`,
      closable: true,
      duration: 0,
      onClose: async (confirm) => {
        if (confirm && currentConfigId.value) {
          try {
            await deleteConfig(currentConfigId.value);
            Message.success('删除成功');
            currentConfigId.value = null;
            currentConfigItem.value = null;
            refreshConfigList();
          } catch (error) {
            Message.error('删除失败');
          }
        }
      },
    });
  }

  // 导出配置
  function handleExportConfig() {
    exportModalVisible.value = true;
  }

  // 提交导出
  async function handleSubmitExport() {
    try {
      const filename = `configs_export_${new Date().getTime()}.json`;
      await exportConfigs(
        {
          environment: exportForm.environment || undefined,
          targetSalt: exportForm.targetSalt || undefined,
        },
        filename
      );

      Message.success('导出成功');
      exportModalVisible.value = false;
      return true;
    } catch (error: any) {
      Message.error(`导出失败: ${error.message || '未知错误'}`);
      return false;
    }
  }

  // 导入配置
  function handleImportConfig() {
    importModalVisible.value = true;
  }

  // 文件选择变化
  function handleFileChange(files: any, currentFile: any) {
    importForm.file = currentFile.file;
  }

  // 读取文件为文本
  function readFileAsText(file: File): Promise<string> {
    return new Promise((resolve, reject) => {
      const reader = new FileReader();
      reader.onload = (e) => {
        resolve(e.target?.result as string);
      };
      reader.onerror = reject;
      reader.readAsText(file);
    });
  }

  // 提交导入
  async function handleSubmitImport() {
    try {
      await importFormRef.value?.validate();

      if (!importForm.file) {
        Message.error('请选择文件');
        return false;
      }

      // 读取文件内容
      const fileContent = await readFileAsText(importForm.file);
      const importData = JSON.parse(fileContent);

      // 构造导入请求
      await importConfigs({
        sourceSalt: importForm.sourceSalt || undefined,
        configs: importData.configs || importData, // 兼容不同格式
        overWrite: importForm.overWrite,
      });

      Message.success('导入成功');
      importModalVisible.value = false;
      refreshConfigList();
      return true;
    } catch (error: any) {
      if (error.errors) {
        return false; // 表单验证错误
      }
      Message.error(`导入失败: ${error.message || '未知错误'}`);
      return false;
    }
  }
</script>

<style scoped lang="less">
  .config-center-container {
    padding: 20px;

    .config-layout {
      display: flex;
      gap: 16px;
      height: calc(100vh - 140px);

      .config-sidebar {
        flex: 0 0 350px;
        display: flex;
        flex-direction: column;

        .sidebar-card {
          height: 100%;
          display: flex;
          flex-direction: column;

          :deep(.arco-card-body) {
            flex: 1;
            display: flex;
            flex-direction: column;
            overflow: hidden;
          }

          .sidebar-header {
            display: flex;
            justify-content: space-between;
            align-items: center;
            width: 100%;

            .sidebar-title {
              font-weight: 600;
              font-size: 16px;
            }
          }

          .search-input {
            margin-bottom: 12px;
          }

          .env-filter-bar {
            margin-bottom: 12px;
          }

          .config-list {
            flex: 1;
            overflow-y: auto;

            .selected-item {
              background-color: var(--color-fill-2);
            }
          }

          .pagination-wrapper {
            margin-top: 16px;
            display: flex;
            justify-content: center;
          }
        }
      }

      .config-content {
        flex: 1;
        display: flex;
        flex-direction: column;
        min-width: 0;
        overflow: hidden;

        .content-card {
          height: 100%;
          width: 100%;
          display: flex;
          flex-direction: column;

          :deep(.arco-card-body) {
            flex: 1;
            width: 100%;
            min-width: 0;
            overflow: hidden;
          }

          .content-header {
            display: flex;
            justify-content: space-between;
            align-items: center;
            width: 100%;

            .content-title {
              font-weight: 600;
              font-size: 16px;
            }
          }

          .config-detail-container {
            height: 100%;
            width: 100%;
            max-width: 100%;
            overflow: hidden;

            :deep(.arco-tabs) {
              height: 100%;
              width: 100%;
              display: flex;
              flex-direction: column;

              .arco-tabs-content {
                flex: 1;
                overflow-y: auto;
                overflow-x: hidden;
                min-width: 0;
              }

              .arco-tabs-content-item {
                min-width: 0;
              }
            }
          }
        }
        
        .full-height {
          width: 100%;
          min-width: 0;
        }
      }
    }

    .full-height {
      height: 100%;
    }

    .info-card {
      padding: 16px;
      max-width: 100%;
      overflow: hidden;

      :deep(.arco-descriptions) {
        max-width: 100%;
        overflow-x: auto;
      }

      :deep(.arco-descriptions-item-value) {
        word-break: break-all;
      }
    }

    .export-hint,
    .import-hint {
      margin-top: 16px;
    }
  }
</style>

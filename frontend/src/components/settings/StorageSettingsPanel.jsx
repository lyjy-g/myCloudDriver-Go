import { Button, Input, Select, Space, Tag, Typography } from "antd";
import { CloudServerOutlined, DatabaseOutlined, ReloadOutlined, SettingOutlined } from "@ant-design/icons";
import React from "react";

const { Title, Text } = Typography;

/**
 * 存储设置面板。
 *
 * @param {{
 *   visible: boolean,
 *   workspaces: Array,
 *   activeWorkspace: object,
 *   platforms: Array,
 *   storageSettings: Array,
 *   storageForm: object,
 *   loading: boolean,
 *   normalizedBaseUrl: string,
 *   localPathPresets: Array,
 *   onRefresh: Function,
 *   onUpdateField: Function,
 *   onSwitchWorkspace: Function,
 *   onApply: Function,
 *   onEditSetting: Function,
 *   onCreateDraft: Function,
 *   onActivateSetting: Function,
 *   onRebuildIndexes: Function
 * }} props 组件参数
 * @returns {JSX.Element | null} 设置面板
 */
export function StorageSettingsPanel({
  visible,
  workspaces,
  activeWorkspace,
  platforms,
  storageSettings,
  storageForm,
  loading,
  normalizedBaseUrl,
  localPathPresets,
  onRefresh,
  onUpdateField,
  onSwitchWorkspace,
  onApply,
  onEditSetting,
  onCreateDraft,
  onActivateSetting,
  onRebuildIndexes
}) {
  if (!visible) {
    return null;
  }

  const workspaceName = activeWorkspace?.workspaceName || "未选择空间";
  const workspaceType = activeWorkspace?.workspaceType || "personal";
  const selectedPlatform = storageForm.identifier || "Local";
  const canApply = Boolean(storageForm.identifier);

  return (
    <div className="mcd-settings">
      <div className="mcd-panel mcd-settings-header">
        <div>
          <Title level={4} style={{ margin: 0 }}>空间配置</Title>
          <Text style={{ display: "block", fontWeight: 600, marginTop: 4 }}>当前配置</Text>
          <Text className="mcd-muted">针对当前工作空间管理存储插件和连接参数</Text>
        </div>
        <Space>
          <Button icon={<ReloadOutlined />} onClick={onRefresh}>刷新</Button>
          <Button type="primary" icon={<SettingOutlined />} onClick={onApply} disabled={!canApply} loading={loading}>
            保存并应用
          </Button>
        </Space>
      </div>

      <div className="mcd-settings-grid">
        <div className="mcd-panel mcd-settings-main">
          <div className="mcd-settings-block">
            <Text className="mcd-settings-label">工作空间</Text>
            <Select
              className="w-full"
              value={activeWorkspace?.workspaceId}
              onChange={onSwitchWorkspace}
              options={workspaces.map((item) => ({
                value: item.workspaceId,
                label: `${item.workspaceName}（${item.workspaceType}）`
              }))}
            />
            <Space style={{ marginTop: 10 }}>
              <Tag color="blue">{workspaceName}</Tag>
              <Tag color="purple">{workspaceType}</Tag>
            </Space>
          </div>

          <div className="mcd-settings-block">
            <Text className="mcd-settings-label">配置ID（编辑已有配置时自动带出）</Text>
            <Input
              className="w-full"
              placeholder="留空表示新建配置"
              value={storageForm.settingId}
              onChange={(event) => onUpdateField("settingId", event.target.value)}
            />
          </div>

          <div className="mcd-settings-block">
            <Text className="mcd-settings-label">存储引擎</Text>
            <div className="mcd-settings-platforms">
              {platforms.map((item) => {
                const selected = storageForm.identifier === item.identifier;
                return (
                  <button
                    key={item.identifier}
                    type="button"
                    className={`mcd-settings-platform-card ${selected ? "active" : ""}`}
                    onClick={() => onUpdateField("identifier", item.identifier)}
                  >
                    <div className="mcd-settings-platform-title">{item.name}</div>
                    <div className="mcd-settings-platform-sub">{item.identifier}</div>
                  </button>
                );
              })}
            </div>
            <Select
              className="w-full"
              value={storageForm.identifier}
              onChange={(value) => onUpdateField("identifier", value)}
              options={platforms.map((item) => ({ value: item.identifier, label: `${item.name}（${item.identifier}）` }))}
            />
          </div>

          <div className="mcd-settings-block">
            <Text className="mcd-settings-label">存储路径（Local）</Text>
            <Input
              className="w-full"
              placeholder="/home/lyjy/code/MyCloudDrive/runtime-data/files"
              value={storageForm.basePath}
              onChange={(event) => onUpdateField("basePath", event.target.value)}
            />
            <Input
              className="w-full"
              style={{ marginTop: 10 }}
              placeholder="http://localhost:8080/files"
              value={storageForm.baseUrl}
              onChange={(event) => onUpdateField("baseUrl", event.target.value)}
            />
            <Text className="mcd-muted">访问前缀（Local，可选）</Text>
          </div>

          <div className="mcd-settings-presets">
            {localPathPresets.map((preset) => (
              <Button key={preset.value} size="small" onClick={() => onUpdateField("basePath", preset.value)}>{preset.label}</Button>
            ))}
          </div>

          <Space wrap>
            <Button type="primary" onClick={onApply} disabled={!canApply} loading={loading}>保存并应用</Button>
            <Button onClick={onCreateDraft}>新建配置</Button>
            <Button onClick={onRebuildIndexes} loading={loading}>重建文件索引</Button>
          </Space>
        </div>

        <div className="mcd-panel mcd-settings-side">
          <div className="mcd-settings-side-item">
            <CloudServerOutlined />
            <div>
              <div className="mcd-settings-side-title">当前服务端</div>
              <div className="mcd-settings-side-value">{normalizedBaseUrl}</div>
            </div>
          </div>
          <div className="mcd-settings-side-item">
            <DatabaseOutlined />
            <div>
              <div className="mcd-settings-side-title">空间存储引擎</div>
              <div className="mcd-settings-side-value">{selectedPlatform}</div>
            </div>
          </div>
          <div className="mcd-settings-side-item">
            <SettingOutlined />
            <div>
              <div className="mcd-settings-side-title">连接预览</div>
              <code className="mcd-settings-code">
                {selectedPlatform}:{storageForm.settingId}
                <br />
                {storageForm.basePath || "(未设置路径)"}
              </code>
            </div>
          </div>
          <div className="mcd-settings-side-actions">
            <Button block icon={<ReloadOutlined />} onClick={onRefresh}>刷新空间配置</Button>
            <Button block onClick={onRebuildIndexes} loading={loading}>
              索引维护
            </Button>
          </div>
          <Text className="mcd-muted">说明：OSS/S3 当前为演示占位，建议先用 Local 完成本地联调。</Text>
        </div>
      </div>

      <div className="mcd-panel mcd-settings-platform-raw">
        <div className="mcd-settings-row-head">
          <Title level={5} style={{ margin: 0 }}>已有配置（快捷操作）</Title>
          <Text className="mcd-muted">点击编辑可直接载入表单，点击启用可一键切换空间配置。</Text>
        </div>
        <div className="mcd-settings-quick-list">
          {(storageSettings || []).map((item) => (
            <div key={item.settingId} className={`mcd-settings-quick-item ${item.active ? "active" : ""}`}>
              <div className="mcd-settings-quick-main">
                <div className="mcd-settings-quick-title">
                  {item.name || item.identifier}
                  <Tag color={item.active ? "blue" : "default"} style={{ marginLeft: 8 }}>{item.active ? "当前启用" : "未启用"}</Tag>
                </div>
                <div className="mcd-settings-quick-sub">
                  ID: {item.settingId || "-"} | 路径: {item.basePath || "(未设置)"}
                </div>
              </div>
              <Space>
                <Button size="small" onClick={() => onEditSetting(item.settingId)}>编辑</Button>
                <Button size="small" type={item.active ? "default" : "primary"} ghost={!item.active} disabled={item.active} onClick={() => onActivateSetting(item.settingId)}>
                  {item.active ? "已启用" : "启用"}
                </Button>
              </Space>
            </div>
          ))}
          {(storageSettings || []).length === 0 ? <Text className="mcd-muted">当前空间暂无存储配置</Text> : null}
        </div>

        <Title level={5}>可用平台能力</Title>
        <div className="mcd-settings-raw-list">
          {platforms.map((item) => (
            <Tag key={item.identifier} color={item.enabled ? "green" : "default"}>
              {item.name} / {item.identifier}
            </Tag>
          ))}
          {platforms.length === 0 ? <Text className="mcd-muted">暂无可用平台</Text> : null}
        </div>
      </div>
    </div>
  );
}

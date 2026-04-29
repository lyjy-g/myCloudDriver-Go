import { Button, Input, Modal, Select, Space, Switch, Tag, Typography } from "antd";
import { CloudServerOutlined, DatabaseOutlined, ReloadOutlined, SettingOutlined } from "@ant-design/icons";
import React, { useState } from "react";

const { Title, Text } = Typography;

function maskPath(path) {
  if (!path) {
    return "(未设置路径)";
  }
  const normalized = String(path).replace(/\\/g, "/");
  if (!normalized.startsWith("/")) {
    return normalized;
  }
  const segments = normalized.split("/").filter(Boolean);
  if (segments.length <= 2) {
    return "/***";
  }
  return `/***/${segments.slice(-2).join("/")}`;
}

/**
 * 存储配置设置面板（仅存储，不包含空间管理功能）。
 */
export function StorageSettingsPanel({
  visible,
  activeWorkspace,
  platforms,
  storageSettings,
  storageForm,
  loading,
  normalizedBaseUrl,
  onClose,
  onRefresh,
  onUpdateField,
  onApply,
  onEditSetting,
  onActivateSetting,
  onRebuildIndexes,
  onDeleteSetting
}) {
  if (!visible) {
    return null;
  }

  const workspaceName = activeWorkspace?.workspaceName || "未选择空间";
  const workspaceType = activeWorkspace?.workspaceType || "personal";
  const selectedPlatform = storageForm.identifier || "Local";
  const normalizedPlatform = String(selectedPlatform || "").toLowerCase();
  const isS3 = normalizedPlatform === "s3";
  const canApply = Boolean(storageForm.identifier) && Boolean(String(storageForm.storageSettingName || "").trim());

  return (
    <Modal open={visible} onCancel={onClose} footer={null} width={1280} title="存储配置设置" destroyOnHidden={false}>
      <div className="mcd-settings">
        <div className="mcd-settings-row-head" style={{ marginBottom: 12 }}>
          <Space>
            <Tag color="blue">{workspaceName}</Tag>
            <Tag color="purple">{workspaceType}</Tag>
          </Space>
          <Space>
            <Button icon={<ReloadOutlined />} onClick={onRefresh}>刷新</Button>
            <Button type="primary" icon={<SettingOutlined />} onClick={onApply} disabled={!canApply} loading={loading}>保存并应用</Button>
          </Space>
        </div>

        <div className="mcd-settings-grid mcd-settings-grid-editor">
          <div className="mcd-panel mcd-settings-main mcd-settings-editor-zone">
            <div className="mcd-settings-row-head">
              <Title level={5} style={{ margin: 0 }}>配置列表</Title>
              <Text className="mcd-muted">仅管理当前空间下的存储配置</Text>
            </div>
            <div className="mcd-settings-quick-list" style={{ marginBottom: 16 }}>
              {(storageSettings || []).map((item) => (
                <div key={item.settingId} className={`mcd-settings-quick-item ${item.active ? "active" : ""}`}>
                  <div className="mcd-settings-quick-main">
                    <div className="mcd-settings-quick-title">
                      {item.storageSettingName || item.name || item.identifier}
                      <Tag color={item.active ? "blue" : "default"} style={{ marginLeft: 8 }}>{item.active ? "当前启用" : "未启用"}</Tag>
                    </div>
                    <div className="mcd-settings-quick-sub">ID: {item.settingId || "-"} | 路径: {item.bucket || maskPath(item.basePath)}</div>
                  </div>
                  <Space>
                    <Button size="small" onClick={() => onEditSetting(item.settingId)}>编辑</Button>
                    <Button size="small" type={item.active ? "default" : "primary"} ghost={!item.active} disabled={item.active} onClick={() => onActivateSetting(item.settingId)}>
                      {item.active ? "已启用" : "启用"}
                    </Button>
                    <Button size="small" danger onClick={() => onDeleteSetting(item.settingId)}>删除</Button>
                  </Space>
                </div>
              ))}
              {(storageSettings || []).length === 0 ? <Text className="mcd-muted">当前空间暂无存储配置</Text> : null}
            </div>

            <div className="mcd-settings-block">
              <Text className="mcd-settings-label">配置名称</Text>
              <Input className="w-full" placeholder="例如：个人空间 Local 主配置" value={storageForm.storageSettingName} onChange={(event) => onUpdateField("storageSettingName", event.target.value)} />
            </div>

            <div className="mcd-settings-block">
              <Text className="mcd-settings-label">配置ID（编辑已有配置时自动带出）</Text>
              <Input className="w-full" placeholder="留空表示新建配置" value={storageForm.settingId} onChange={(event) => onUpdateField("settingId", event.target.value)} />
            </div>

            <div className="mcd-settings-block">
              <Text className="mcd-settings-label">存储引擎</Text>
              <div className="mcd-settings-platforms">
                {platforms.map((item) => {
                  const selected = storageForm.identifier === item.identifier;
                  return (
                    <button key={item.identifier} type="button" className={`mcd-settings-platform-card ${selected ? "active" : ""}`} onClick={() => onUpdateField("identifier", item.identifier)}>
                      <div className="mcd-settings-platform-title">{item.name}</div>
                      <div className="mcd-settings-platform-sub">{item.identifier}</div>
                    </button>
                  );
                })}
              </div>
              <Select className="w-full" value={storageForm.identifier} onChange={(value) => onUpdateField("identifier", value)} options={platforms.map((item) => ({ value: item.identifier, label: `${item.name}（${item.identifier}）` }))} />
            </div>

            {isS3 ? (
              <>
                <div className="mcd-settings-block">
                  <Text className="mcd-settings-label">S3 Endpoint / Region / Bucket</Text>
                  <Input className="w-full" placeholder="127.0.0.1:9000 或 s3.amazonaws.com" value={storageForm.endpoint} onChange={(event) => onUpdateField("endpoint", event.target.value)} />
                  <Input className="w-full" style={{ marginTop: 10 }} placeholder="us-east-1" value={storageForm.region} onChange={(event) => onUpdateField("region", event.target.value)} />
                  <Input className="w-full" style={{ marginTop: 10 }} placeholder="bucket-name" value={storageForm.bucket} onChange={(event) => onUpdateField("bucket", event.target.value)} />
                </div>

                <div className="mcd-settings-block">
                  <Text className="mcd-settings-label">凭证与路径</Text>
                  <Input className="w-full" placeholder="Access Key ID" value={storageForm.accessKeyId} onChange={(event) => onUpdateField("accessKeyId", event.target.value)} />
                  <Input.Password className="w-full" style={{ marginTop: 10 }} placeholder="Secret Access Key" value={storageForm.secretAccessKey} onChange={(event) => onUpdateField("secretAccessKey", event.target.value)} />
                  <Input className="w-full" style={{ marginTop: 10 }} placeholder="prefix（可选）" value={storageForm.prefix} onChange={(event) => onUpdateField("prefix", event.target.value)} />
                </div>

                <div className="mcd-settings-toggle-row">
                  <div className="mcd-settings-toggle-item">
                    <Text>使用 HTTPS</Text>
                    <Switch checked={Boolean(storageForm.useSSL)} onChange={(value) => onUpdateField("useSSL", value)} />
                  </div>
                  <div className="mcd-settings-toggle-item">
                    <Text>Path Style</Text>
                    <Switch checked={storageForm.pathStyle !== false} onChange={(value) => onUpdateField("pathStyle", value)} />
                  </div>
                </div>
              </>
            ) : (
              <div className="mcd-settings-block">
                <Text className="mcd-settings-label">命名空间（Local）</Text>
                <Input className="w-full" placeholder="例如：team-a 或 project-x" value={storageForm.namespace} onChange={(event) => onUpdateField("namespace", event.target.value)} />
                <Input className="w-full" style={{ marginTop: 10 }} placeholder="http://localhost:8080/files" value={storageForm.baseUrl} onChange={(event) => onUpdateField("baseUrl", event.target.value)} />
                <Text className="mcd-muted">Local 实际路径由后端自动生成：{`/data/myclouddrive/storage/{workspaceId}/{settingId}`}</Text>
              </div>
            )}
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
                <div className="mcd-settings-side-title">当前配置预览</div>
                <div className="mcd-settings-side-value">{storageForm.storageSettingName || "(未命名配置)"}</div>
                <code className="mcd-settings-code">
                  {selectedPlatform}:{storageForm.settingId || "(new)"}
                  <br />
                  {isS3
                    ? `${storageForm.endpoint || "(未设置 endpoint)"} / ${storageForm.bucket || "(未设置 bucket)"}`
                    : maskPath(storageForm.basePath)}
                </code>
              </div>
            </div>
            <div className="mcd-settings-side-actions">
              <Button block onClick={onRebuildIndexes} loading={loading}>重建文件索引</Button>
            </div>
          </div>
        </div>
      </div>

    </Modal>
  );
}

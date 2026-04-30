import { Button, Input, Modal, Select, Space, Table, Tag, Typography } from "antd";
import React, { useState } from "react";
import { WorkspaceSettingSelector } from "../settings/WorkspaceSettingSelector.jsx";

const { Text } = Typography;

/**
 * 文件与回收站面板。
 *
 * @param {{
 *   activeMenu: string,
 *   files: Array,
 *   shares: Array,
 *   columns: Array,
 *   shareColumns: Array,
 *   directoryTrail: Array,
 *   onJumpDirectory: Function,
 *   activeWorkspace: object,
 *   activeStorage: object,
 *   currentUser: object,
 *   storageSettings: Array,
 *   enabledStorageIds: Array,
 *   platforms: Array,
 *   onOpenStorageSettings: Function,
  *   onOpenFiles: Function,
 *   onCreateStorageSetting: Function,
 *   onRefreshWorkspace: Function,
 *   onEditStorageSetting: Function,
 *   onEnableStorageSetting: Function,
 *   onDisableStorageSetting: Function,
 *   onDeleteStorageSetting: Function
 *   agentQuery: string,
 *   agentResult: object,
 *   onAgentQueryChange: Function,
 *   onAgentSubmit: Function
 * }} props 组件参数
 * @returns {JSX.Element | null} 面板
 */
export function FilesPanel({
  activeMenu,
  files,
  shares,
  columns,
  shareColumns,
  directoryTrail,
  onJumpDirectory,
  activeWorkspace,
  activeStorage,
  currentUser,
  storageSettings,
  enabledStorageIds,
  platforms,
  onOpenStorageSettings,
  onOpenFiles,
  onCreateStorageSetting,
  onRefreshWorkspace,
  onEditStorageSetting,
  onEnableStorageSetting,
  onDisableStorageSetting,
  onDeleteStorageSetting,
  agentQuery,
  agentResult,
  onAgentQueryChange,
  onAgentSubmit
}) {
  const [createOpen, setCreateOpen] = useState(false);
  const [draft, setDraft] = useState({
    storageSettingName: "",
    identifier: "Local",
    namespace: "",
    baseUrl: "",
    endpoint: "",
    region: "us-east-1",
    bucket: "",
    accessKeyId: "",
    secretAccessKey: "",
    prefix: "",
    useSSL: false,
    pathStyle: true
  });

  const isS3Draft = String(draft.identifier || "").toLowerCase() === "s3";

  const submitCreate = () => {
    if (!String(draft.storageSettingName || "").trim()) {
      return;
    }
    onCreateStorageSetting(draft);
    setCreateOpen(false);
  };

  if (activeMenu === "workspace-home") {
    return (
      <div className="mcd-space-home">
        <div className="mcd-panel mcd-space-home-hero">
          <div className="mcd-space-home-headline">
            <div className="mcd-space-home-kicker">Workspace Hub</div>
            <h2 className="mcd-space-home-title">{activeWorkspace?.workspaceName || "我的空间"}</h2>
          </div>
          <div className="mcd-space-home-tags">
            <Tag color="blue">类型：{activeWorkspace?.workspaceType || "-"}</Tag>
            <Tag color="cyan">角色：{activeWorkspace?.role || "-"}</Tag>
          </div>
          <div className="mcd-space-home-actions">
            <Button type="primary" onClick={onOpenFiles}>进入全部文件</Button>
            <Button onClick={() => setCreateOpen(true)}>新建存储配置</Button>
            <Button onClick={onOpenStorageSettings}>编辑空间配置</Button>
            <Button onClick={onRefreshWorkspace}>刷新空间信息</Button>
          </div>
          <Text className="mcd-muted">当前用户：{currentUser?.displayName || currentUser?.username || "-"}</Text>
        </div>

        <WorkspaceSettingSelector
          storageSettings={storageSettings}
          enabledStorageIds={enabledStorageIds}
          onEditSetting={onEditStorageSetting}
          onEnableSetting={onEnableStorageSetting}
          onDisableSetting={onDisableStorageSetting}
          onDeleteSetting={onDeleteStorageSetting}
        />

        <Modal
          open={createOpen}
          title="新建存储配置"
          onCancel={() => setCreateOpen(false)}
          onOk={submitCreate}
          okText="进入配置编辑"
          destroyOnHidden
        >
          <Space direction="vertical" style={{ width: "100%" }} size="middle">
            <Input
              placeholder="配置名称（必填）"
              value={draft.storageSettingName}
              onChange={(event) => setDraft((prev) => ({ ...prev, storageSettingName: event.target.value }))}
            />
            <Select
              value={draft.identifier}
              onChange={(value) => setDraft((prev) => ({ ...prev, identifier: value }))}
              options={[
                { value: "Local", label: "Local" },
                { value: "S3", label: "S3" }
              ]}
            />
            {isS3Draft ? (
              <>
                <Input placeholder="endpoint（如 127.0.0.1:9000）" value={draft.endpoint} onChange={(event) => setDraft((prev) => ({ ...prev, endpoint: event.target.value }))} />
                <Input placeholder="region（如 us-east-1）" value={draft.region} onChange={(event) => setDraft((prev) => ({ ...prev, region: event.target.value }))} />
                <Input placeholder="accessKeyId（可选）" value={draft.accessKeyId} onChange={(event) => setDraft((prev) => ({ ...prev, accessKeyId: event.target.value }))} />
                <Input.Password placeholder="secretAccessKey（可选）" value={draft.secretAccessKey} onChange={(event) => setDraft((prev) => ({ ...prev, secretAccessKey: event.target.value }))} />
              </>
            ) : (
              <>
                <Input placeholder="namespace（如 team-a）" value={draft.namespace} onChange={(event) => setDraft((prev) => ({ ...prev, namespace: event.target.value }))} />
                <Input placeholder="baseUrl（可选）" value={draft.baseUrl} onChange={(event) => setDraft((prev) => ({ ...prev, baseUrl: event.target.value }))} />
              </>
            )}
          </Space>
        </Modal>

      </div>
    );
  }

  if (activeMenu === "files") {
    return (
      <div className="mcd-panel p-5">
        <div className="mb-4 flex flex-wrap items-center gap-2">
          {directoryTrail.map((item, index) => (
            <button
              key={item.id}
              type="button"
              className="text-sm text-blue-600 hover:underline"
              onClick={() => onJumpDirectory(index)}
            >
              {index > 0 ? ` / ${item.name}` : item.name}
            </button>
          ))}
        </div>
        <Table
          className="mcd-table"
          rowKey="fileId"
          columns={columns}
          dataSource={files}
          pagination={{ pageSize: 8 }}
          locale={{ emptyText: "暂无文件，请先上传" }}
        />
      </div>
    );
  }

  if (activeMenu === "trash") {
    return (
      <div className="mcd-panel p-5">
        <Text className="mcd-muted block mb-3">回收站支持恢复，目录会递归恢复子项。</Text>
        <Table
          className="mcd-table"
          rowKey="fileId"
          columns={columns}
          dataSource={files}
          pagination={{ pageSize: 8 }}
          locale={{ emptyText: "回收站为空" }}
        />
      </div>
    );
  }

  if (activeMenu === "shares") {
    return (
      <div className="mcd-panel p-5">
        <Text className="mcd-muted block mb-3">分享列表</Text>
        <Table
          className="mcd-table"
          rowKey="shareId"
          columns={shareColumns}
          dataSource={shares}
          pagination={{ pageSize: 8 }}
          locale={{ emptyText: "暂无分享，先在文件列表中点击“分享”创建" }}
        />
      </div>
    );
  }

  if (activeMenu === "knowledge") {
    return (
      <div className="mcd-panel p-5">
        <Text className="mcd-muted block mb-3">知识库（预留扩展模块）</Text>
        <div className="mcd-space-home-hero">
          <h3 style={{ margin: 0 }}>知识库建设中</h3>
          <p className="mcd-muted" style={{ marginTop: 8 }}>
            后续可扩展：文档分层、标签检索、向量检索、空间级权限、分享联动。
          </p>
        </div>
      </div>
    );
  }

  if (activeMenu === "agent") {
    const items = Array.isArray(agentResult?.items) ? agentResult.items : [];
    return (
      <div className="mcd-panel p-5">
        <Text className="mcd-muted block mb-3">Agent 检索（只读）</Text>
        <Space.Compact style={{ width: "100%", marginBottom: 12 }}>
          <Input
            placeholder="例如：最近上传了哪些文件、我的分享访问记录"
            value={agentQuery}
            onChange={(event) => onAgentQueryChange?.(event.target.value)}
            onPressEnter={() => onAgentSubmit?.()}
          />
          <Button type="primary" onClick={() => onAgentSubmit?.()}>检索</Button>
        </Space.Compact>
        <div className="mcd-muted" style={{ marginBottom: 12 }}>
          traceId: {agentResult?.traceId || "-"} | mode: {agentResult?.routeMode || "-"} | model: {agentResult?.model || "-"} | sources: {(agentResult?.sources || []).join(", ") || "-"}
        </div>
        <div className="mcd-muted" style={{ marginBottom: 12 }}>{agentResult?.summary || "暂无结果"}</div>
        <Table
          className="mcd-table"
          rowKey={(row, idx) => row?.id || row?.fileId || row?.shareId || String(idx)}
          columns={[
            { title: "条目", dataIndex: "fileName", key: "fileName", render: (_, row) => row.fileName || row.shareName || row.shareId || "-" },
            { title: "类型", key: "type", render: (_, row) => (row.isDir ? "目录" : row.shareId ? "分享" : "文件") },
            { title: "补充信息", key: "meta", render: (_, row) => row.updatedAt || row.accessTime || row.status || "-" }
          ]}
          dataSource={items}
          pagination={{ pageSize: 8 }}
          locale={{ emptyText: "暂无检索结果" }}
        />
      </div>
    );
  }

  return null;
}

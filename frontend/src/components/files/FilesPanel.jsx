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
 *   onAgentSubmit: Function,
 *   agentChatCollapsed: boolean
 * }} props 组件参数
 * @returns {JSX.Element | null} 面板
 */
export function FilesPanel({
  activeMenu,
  files,
  shares,
  knowledgeBases,
  knowledgeFiles,
  activeKnowledge,
  columns,
  shareColumns,
  directoryTrail,
  onJumpDirectory,
  onOpenDirectory,
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
  onCreateKnowledgeBase,
  onDeleteKnowledgeBase,
  onAddKnowledgeFile,
  onAddKnowledgeItems,
  onSwitchKnowledgeImportStorage,
  agentQuery,
  agentResult,
  onAgentQueryChange,
  onAgentSubmit,
  agentChatCollapsed
}) {
  const [createOpen, setCreateOpen] = useState(false);
  const [kbCreateOpen, setKbCreateOpen] = useState(false);
  const [kbDraft, setKbDraft] = useState({ name: "", description: "" });
  const [importPickerOpen, setImportPickerOpen] = useState(false);
  const [importKeyword, setImportKeyword] = useState("");
  const [selectedImportMap, setSelectedImportMap] = useState({});
  const [importStorageSettingId, setImportStorageSettingId] = useState("");
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

  const submitKnowledgeCreate = async () => {
    const ok = await onCreateKnowledgeBase?.(kbDraft.name, kbDraft.description);
    if (!ok) {
      return;
    }
    setKbCreateOpen(false);
    setKbDraft({ name: "", description: "" });
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

  if (activeMenu === "knowledge-home") {
    return (
      <div className="mcd-panel p-5">
        <div className="mb-4" style={{ display: "flex", justifyContent: "space-between", alignItems: "center", gap: 12 }}>
          <Text className="mcd-muted">知识库首页：管理知识库（增删改查）</Text>
          <Button type="primary" onClick={() => setKbCreateOpen(true)}>新建知识库</Button>
        </div>
        <Table
          className="mcd-table"
          rowKey={(row) => String(row.id)}
          dataSource={knowledgeBases || []}
          pagination={{ pageSize: 8 }}
          locale={{ emptyText: "暂无知识库" }}
          columns={[
            { title: "ID", dataIndex: "id", key: "id", width: 120 },
            { title: "名称", dataIndex: "name", key: "name" },
            { title: "描述", dataIndex: "description", key: "description", render: (value) => value || "-" },
            { title: "创建人", dataIndex: "createdBy", key: "createdBy", width: 180 },
            { title: "创建时间", dataIndex: "createdAt", key: "createdAt", width: 220 },
            {
              title: "操作",
              key: "actions",
              width: 120,
              render: (_, record) => (
                <Button
                  size="small"
                  danger
                  onClick={async () => {
                    const confirmed = window.confirm(`确认删除知识库「${record.name}」吗？`);
                    if (!confirmed) {
                      return;
                    }
                    await onDeleteKnowledgeBase?.(record.id);
                  }}
                >
                  删除
                </Button>
              )
            }
          ]}
        />
        <Modal
          open={kbCreateOpen}
          title="新建知识库"
          onCancel={() => setKbCreateOpen(false)}
          onOk={submitKnowledgeCreate}
          okText="创建"
          destroyOnHidden
        >
          <Space direction="vertical" style={{ width: "100%" }} size="middle">
            <Input
              placeholder="知识库名称（必填）"
              value={kbDraft.name}
              onChange={(event) => setKbDraft((prev) => ({ ...prev, name: event.target.value }))}
            />
            <Input.TextArea
              rows={3}
              placeholder="知识库描述（可选）"
              value={kbDraft.description}
              onChange={(event) => setKbDraft((prev) => ({ ...prev, description: event.target.value }))}
            />
          </Space>
        </Modal>
      </div>
    );
  }

  if (activeMenu === "knowledge-detail") {
    const importStorageOptions = (storageSettings || []).map((item) => ({
      value: item.settingId,
      label: item.storageSettingName || item.name || item.settingId
    }));
    const selectedImportItems = Object.values(selectedImportMap || {});
    const filteredItems = (files || []).filter((item) => {
      const kw = String(importKeyword || "").trim().toLowerCase();
      if (!kw) {
        return true;
      }
      return String(item.fileName || "").toLowerCase().includes(kw) || String(item.fileId || "").toLowerCase().includes(kw);
    });
    return (
      <div className="mcd-panel p-5">
        <div className="mb-4" style={{ display: "flex", justifyContent: "space-between", alignItems: "center", gap: 12 }}>
          <Text className="mcd-muted">
            {activeKnowledge ? `知识库详情：${activeKnowledge.name}` : "知识库详情"}
          </Text>
        </div>
        {!activeKnowledge ? (
          <Text className="mcd-muted">请先在左侧选择一个知识库。</Text>
        ) : (
          <>
            <div className="mb-4" style={{ display: "flex", gap: 8, alignItems: "center" }}>
              <Select
                style={{ width: 240 }}
                value={importStorageSettingId || activeStorage?.settingId}
                options={importStorageOptions}
                onChange={async (value) => {
                  setImportStorageSettingId(value);
                  await onSwitchKnowledgeImportStorage?.(value);
                }}
              />
              <Button onClick={() => setImportPickerOpen(true)}>
                {selectedImportItems.length > 0 ? `已选 ${selectedImportItems.length} 项` : "选择文件/目录"}
              </Button>
              <Button
                type="primary"
                onClick={async () => {
                  if (selectedImportItems.length === 0) {
                    return;
                  }
                  const ok = await onAddKnowledgeItems?.(
                    activeKnowledge.id,
                    selectedImportItems,
                    importStorageSettingId || activeStorage?.settingId
                  );
                  if (ok) {
                    setSelectedImportMap({});
                  }
                }}
              >
                导入到知识库
              </Button>
            </div>
            <Table
              className="mcd-table"
              rowKey={(row) => `${row.id || row.fileId || Math.random()}`}
              dataSource={knowledgeFiles || []}
              pagination={{ pageSize: 8 }}
              locale={{ emptyText: "该知识库暂无导入文件" }}
              columns={[
                { title: "文件ID", dataIndex: "fileId", key: "fileId" },
                { title: "文件名", dataIndex: "fileName", key: "fileName" },
                { title: "解析", dataIndex: "parseStatus", key: "parseStatus", width: 90 },
                { title: "切片", dataIndex: "chunkStatus", key: "chunkStatus", width: 90 },
                { title: "向量化", dataIndex: "embedStatus", key: "embedStatus", width: 90 },
                { title: "索引", dataIndex: "indexStatus", key: "indexStatus", width: 90 },
                { title: "创建时间", dataIndex: "createdAt", key: "createdAt", width: 220 }
              ]}
            />
            <Modal
              open={importPickerOpen}
              title="选择导入对象（支持目录递归）"
              onCancel={() => setImportPickerOpen(false)}
              footer={null}
              width={860}
              destroyOnHidden
            >
              <div className="mb-3" style={{ display: "flex", gap: 8, flexWrap: "wrap" }}>
                {directoryTrail.map((item, index) => (
                  <Button key={item.id} size="small" onClick={() => onJumpDirectory(index)}>
                    {index === 0 ? "根目录" : item.name}
                  </Button>
                ))}
              </div>
              <Input
                placeholder="按文件名/ID搜索当前目录"
                value={importKeyword}
                onChange={(event) => setImportKeyword(event.target.value)}
                style={{ marginBottom: 10 }}
              />
              <Table
                className="mcd-table"
                rowKey="fileId"
                size="small"
                dataSource={filteredItems}
                rowSelection={{
                  selectedRowKeys: Object.keys(selectedImportMap || {}),
                  onSelect: (record, selected) => {
                    setSelectedImportMap((prev) => {
                      const next = { ...(prev || {}) };
                      if (selected) {
                        next[record.fileId] = record;
                      } else {
                        delete next[record.fileId];
                      }
                      return next;
                    });
                  },
                  onSelectAll: (selected, selectedRows, changeRows) => {
                    setSelectedImportMap((prev) => {
                      const next = { ...(prev || {}) };
                      changeRows.forEach((row) => {
                        if (selected) {
                          next[row.fileId] = row;
                        } else {
                          delete next[row.fileId];
                        }
                      });
                      return next;
                    });
                  }
                }}
                pagination={{ pageSize: 8 }}
                locale={{ emptyText: "当前目录无可选项" }}
                columns={[
                  {
                    title: "名称",
                    dataIndex: "fileName",
                    key: "fileName",
                    render: (value, record) => (
                      <Space>
                        <span>{record.directory ? "📁" : "📄"}</span>
                        <span>{value}</span>
                        {record.directory ? (
                          <Button size="small" type="link" onClick={() => onOpenDirectory?.(record)}>
                            进入
                          </Button>
                        ) : null}
                      </Space>
                    )
                  },
                  { title: "ID", dataIndex: "fileId", key: "fileId", width: 260 },
                  {
                    title: "类型",
                    key: "type",
                    width: 90,
                    render: (_, record) => (record.directory ? "目录" : "文件")
                  },
                  {
                    title: "操作",
                    key: "action",
                    width: 100,
                    render: (_, record) => (
                      <Button
                        type="primary"
                        size="small"
                        onClick={() => {
                          setSelectedImportMap((prev) => ({
                            ...(prev || {}),
                            [record.fileId]: record
                          }));
                        }}
                      >
                        添加
                      </Button>
                    )
                  }
                ]}
              />
            </Modal>
          </>
        )}
      </div>
    );
  }

  if (activeMenu === "agent") {
    return (
      <div className="mcd-panel p-5">
        <Text className="mcd-muted block mb-3">Agent 智能助手</Text>
        <p className="mcd-muted" style={{ marginTop: 8 }}>
          AI 助手位于页面底部浮动栏，支持检索、执行、RAG 等多种模式。
        </p>
      </div>
    );
  }

  return null;
}

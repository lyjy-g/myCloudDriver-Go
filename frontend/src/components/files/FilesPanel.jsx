import { Button, Space, Table, Tag, Typography } from "antd";
import React from "react";

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
 *   onOpenStorageSettings: Function,
 *   onOpenFiles: Function,
 *   onRefreshWorkspace: Function
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
  onOpenStorageSettings,
  onOpenFiles,
  onRefreshWorkspace
}) {
  if (activeMenu === "workspace-home") {
    return (
      <div className="mcd-panel p-5">
        <Text className="mcd-muted block mb-3">我的空间默认页</Text>
        <Space direction="vertical" size="middle" style={{ width: "100%" }}>
          <Space wrap>
            <Tag color="blue">空间：{activeWorkspace?.workspaceName || "-"}</Tag>
            <Tag color="cyan">类型：{activeWorkspace?.workspaceType || "-"}</Tag>
            <Tag color="purple">角色：{activeWorkspace?.role || "-"}</Tag>
          </Space>
          <Space wrap>
            <Tag color="geekblue">存储：{activeStorage?.identifier || "未配置存储"}</Tag>
            <Tag>配置ID：{activeStorage?.settingId || "local-default"}</Tag>
          </Space>
          <Text className="mcd-muted">当前用户：{currentUser?.displayName || currentUser?.username || "-"}</Text>
          <Space>
            <Button type="primary" onClick={onOpenFiles}>进入全部文件</Button>
            <Button onClick={onOpenStorageSettings}>空间配置</Button>
            <Button onClick={onRefreshWorkspace}>刷新空间信息</Button>
          </Space>
        </Space>
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

  return null;
}

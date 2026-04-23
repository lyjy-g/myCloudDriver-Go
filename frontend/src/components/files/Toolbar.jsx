import React from "react";
import { Button, Typography, Space, Divider, Tag, Tooltip } from "antd";
import {
    HddOutlined,
    ReloadOutlined,
    SwapOutlined,
    UserOutlined,
    EnvironmentOutlined,
    PlusOutlined,
    UploadOutlined,
    LogoutOutlined,
    IdcardOutlined
} from "@ant-design/icons";

const { Text } = Typography;

export function Toolbar({
                            quickActions,
                            activeMenu,
                            activeWorkspace,
                            activeStorage,
                            currentUser,
                            onLogout,
                             onUpload,
                             onCreateFolder,
                             onRefreshFiles,
                             onRefreshShares,
                             onRefreshTrash,
                             onOpenStorageSettings
                         }) {
    // 数据格式化
    const storageLabel = activeStorage?.name || activeStorage?.identifier || "未配置";
    const storagePath = activeStorage?.basePath || "-";
    const storageSettingId = activeStorage?.settingId || "local-default";
    const workspaceLabel = activeWorkspace?.workspaceName || activeWorkspace?.workspaceId || "未选择空间";
    const userLabel = currentUser?.displayName || currentUser?.username || "匿名";

    return (
        <div className="mcd-toolbar" style={{
            display: 'flex',
            justifyContent: 'space-between',
            alignItems: 'center',
            padding: '8px 16px',
            background: '#fff',
            borderBottom: '1px solid #f0f0f0'
        }}>

            {/* 左侧：操作按钮区 */}
            <div className="mcd-toolbar-left">
                <Space size={8}>
                    <Space.Compact>
                        {quickActions.map((action) => {
                            const isUpload = action.key === "upload";
                            return (
                                <Button
                                    key={action.key}
                                    icon={isUpload ? <UploadOutlined /> : <PlusOutlined />}
                                    type={isUpload ? "primary" : "default"}
                                    onClick={isUpload ? onUpload : onCreateFolder}
                                >
                                    {action.label}
                                </Button>
                            );
                        })}
                    </Space.Compact>

                    <Button
                        icon={<ReloadOutlined />}
                        onClick={activeMenu === "trash"
                          ? onRefreshTrash
                          : activeMenu === "shares"
                            ? onRefreshShares
                            : onRefreshFiles}
                    >
                        刷新
                    </Button>
                </Space>
            </div>

            {/* 右侧：状态与元数据区 */}
            <div className="mcd-toolbar-right" style={{ minWidth: 0, flex: 1, display: 'flex', justifyContent: 'flex-end' }}>
                <Space split={<Divider type="vertical" />} size={3}>

                    {/* 核心位置信息：空间 & 存储源 */}
                    <Space size={4}>
                        <Tag color="blue" icon={<HddOutlined />} style={{ marginRight: 0 }}>
                            {workspaceLabel}
                        </Tag>
                        <Tag color="cyan">{storageLabel}</Tag>
                    </Space>

                    {/* 次要信息：使用图标 + Tooltip 节省空间 */}
                    <Space size={12} style={{ margin: '0 8px' }}>
                        <Tooltip title={`完整路径: ${storagePath}`}>
                            <Space size={4} style={{ cursor: 'help' }}>
                                <EnvironmentOutlined style={{ color: '#8c8c8c' }} />
                                <Text type="secondary" style={{ maxWidth: 120 }} ellipsis>
                                    {storagePath}
                                </Text>
                            </Space>
                        </Tooltip>

                        <Tooltip title={`存储配置 ID: ${storageSettingId}`}>
                            <Space size={4} style={{ cursor: 'help' }}>
                                <IdcardOutlined style={{ color: '#8c8c8c' }} />
                                <Text type="secondary" style={{ maxWidth: 80 }} ellipsis>
                                    {storageSettingId}
                                </Text>
                            </Space>
                        </Tooltip>

                        <Space size={4}>
                            <UserOutlined style={{ color: '#8c8c8c' }} />
                            <Text type="secondary" style={{ maxWidth: 80 }} ellipsis>
                                {userLabel}
                            </Text>
                        </Space>
                    </Space>

                    {/* 系统操作 */}
                    <Space size={0}>
                        <Button
                            type="link"
                            size="small"
                            icon={<SwapOutlined />}
                            onClick={onOpenStorageSettings}
                        >
                            切换
                        </Button>
                        <Button
                            type="link"
                            size="small"
                            danger
                            icon={<LogoutOutlined />}
                            onClick={onLogout}
                        >
                            退出
                        </Button>
                    </Space>

                </Space>
            </div>
        </div>
    );
}

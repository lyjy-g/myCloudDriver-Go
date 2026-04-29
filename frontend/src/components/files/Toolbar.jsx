import React from "react";
import { Button, Space } from "antd";
import {
    ReloadOutlined,
    PlusOutlined,
    UploadOutlined,
    LogoutOutlined
} from "@ant-design/icons";

export function Toolbar({
                            quickActions,
                            activeMenu,
                            onLogout,
                             onUpload,
                             onCreateFolder,
                             onRefreshFiles,
                             onRefreshShares,
                             onRefreshTrash,
                         }) {
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

            {/* 右侧：页面级操作 */}
            <div className="mcd-toolbar-right" style={{ minWidth: 0, flex: 1, display: 'flex', justifyContent: 'flex-end' }}>

            </div>
        </div>
    );
}

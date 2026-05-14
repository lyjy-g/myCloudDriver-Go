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
        <div className="mcd-toolbar">
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

            <div className="mcd-toolbar-right">
                <span className="mcd-toolbar-hint">
                  {activeMenu === "trash" ? "回收站中的项目可恢复，操作前请确认目标目录。" : activeMenu === "shares" ? "在分享列表中可查看访问与下载记录。" : "支持上传、建文件夹与目录浏览。"}
                </span>
            </div>
        </div>
    );
}

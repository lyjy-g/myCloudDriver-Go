import { Button, Drawer, Input, Progress, Space, Typography } from "antd";
import React from "react";

const { Text } = Typography;

/**
 * 上传抽屉面板。
 *
 * @param {{
 *   open: boolean,
 *   loading: boolean,
 *   uploadProgress: number,
 *   onClose: Function,
 *   onFileChange: Function,
 *   onUpload: Function
 * }} props 组件参数
 * @returns {JSX.Element} 上传抽屉
 */
export function UploadDrawerPanel({ open, loading, uploadProgress, onClose, onFileChange, onUpload }) {
  return (
    <Drawer title="分片上传" open={open} onClose={onClose} width={420}>
      <Space direction="vertical" size={16} className="w-full">
        <Input type="file" onChange={onFileChange} />
        <div className="mcd-card p-4">
          <Text className="mcd-muted">分片大小：5MB（可在配置中调整）</Text>
        </div>
        <Button type="primary" onClick={onUpload} loading={loading}>
          开始上传
        </Button>
        <Progress
          className="mcd-upload-progress"
          percent={uploadProgress}
          status={uploadProgress === 100 ? "success" : "active"}
        />
      </Space>
    </Drawer>
  );
}

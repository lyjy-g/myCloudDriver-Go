import React, { useMemo, useState } from "react";
import { Alert, Button, Card, Input, Space, Table, Tag, Typography } from "antd";
import { accessPublicShare } from "../../api/storage.js";

const { Text, Title } = Typography;

/**
 * 公开分享展示页。
 */
export function SharePublicPanel({ normalizedBaseUrl, shareId }) {
  const [shareCode, setShareCode] = useState("");
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState("");
  const [shareData, setShareData] = useState(null);

  const columns = useMemo(() => ([
    { title: "文件名", dataIndex: "fileName", key: "fileName" },
    {
      title: "大小",
      dataIndex: "fileSize",
      key: "fileSize",
      render: (value) => `${value} B`
    },
    {
      title: "操作",
      key: "action",
      render: (_, record) => (
        <Button
          type="primary"
          disabled={!shareData?.allowDownload || !record.downloadUrl}
          onClick={async () => {
            if (!record.downloadUrl) {
              return;
            }
            try {
              const response = await fetch(`${normalizedBaseUrl}${record.downloadUrl}`, {
                method: "GET",
                headers: {
                  "X-Share-Code": shareCode.trim()
                }
              });
              if (!response.ok) {
                throw new Error("下载失败");
              }
              const blob = await response.blob();
              const link = window.URL.createObjectURL(blob);
              const anchor = document.createElement("a");
              anchor.href = link;
              anchor.download = record.fileName || "download";
              anchor.click();
              window.URL.revokeObjectURL(link);
            } catch (err) {
              setError(err instanceof Error ? err.message : "下载失败");
            }
          }}
        >
          下载
        </Button>
      )
    }
  ]), [normalizedBaseUrl, shareData]);

  const handleVerify = async () => {
    setError("");
    setLoading(true);
    try {
      const result = await accessPublicShare(normalizedBaseUrl, shareId, shareCode.trim());
      const payload = result.data || result;
      setShareData(payload);
    } catch (err) {
      setError(err instanceof Error ? err.message : "分享访问失败");
      setShareData(null);
    } finally {
      setLoading(false);
    }
  };

  return (
    <div className="mcd-share-shell">
      <Card className="mcd-share-card">
        <Space direction="vertical" size="middle" style={{ width: "100%" }}>
          <Title level={3} style={{ margin: 0 }}>MyCloudDrive 分享</Title>
          <Text type="secondary">分享ID：{shareId}</Text>
          <Input
            value={shareCode}
            placeholder="请输入提取码"
            onChange={(event) => setShareCode(event.target.value)}
            onPressEnter={handleVerify}
          />
          <Button type="primary" loading={loading} onClick={handleVerify}>验证并查看</Button>
          {error ? <Alert type="error" message={error} showIcon /> : null}
          {shareData ? (
            <>
              <Space>
                <Tag color="blue">{shareData.shareName}</Tag>
                <Tag color={shareData.allowDownload ? "green" : "orange"}>
                  {shareData.allowDownload ? "允许下载" : "仅展示不可下载"}
                </Tag>
              </Space>
              <Table
                rowKey="fileId"
                columns={columns}
                dataSource={shareData.files || []}
                pagination={false}
                locale={{ emptyText: "该分享暂无可见文件" }}
              />
            </>
          ) : null}
        </Space>
      </Card>
    </div>
  );
}

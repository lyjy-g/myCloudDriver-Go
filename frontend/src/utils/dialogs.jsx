import React from "react";
import { Input, Modal } from "antd";

export function confirmAction({ title, content, okText = "确定", cancelText = "取消", okType = "primary" }) {
  return new Promise((resolve) => {
    Modal.confirm({
      title,
      content,
      okText,
      cancelText,
      okType,
      onOk: () => resolve(true),
      onCancel: () => resolve(false)
    });
  });
}

export function promptText({
  title,
  label,
  initialValue = "",
  placeholder = "",
  required = false,
  okText = "确定",
  cancelText = "取消"
}) {
  return new Promise((resolve) => {
    let current = initialValue || "";
    const modal = Modal.confirm({
      title,
      okText,
      cancelText,
      content: (
        <div style={{ marginTop: 8 }}>
          {label ? <div style={{ marginBottom: 8, color: "#6b7280", fontSize: 13 }}>{label}</div> : null}
          <Input
            autoFocus
            defaultValue={initialValue}
            placeholder={placeholder}
            onChange={(event) => {
              current = event.target.value;
            }}
            onPressEnter={() => {
              modal.destroy();
              if (required && !current.trim()) {
                resolve(null);
                return;
              }
              resolve(current.trim());
            }}
          />
        </div>
      ),
      onOk: () => {
        if (required && !current.trim()) {
          return Promise.reject(new Error("required"));
        }
        resolve(current.trim());
      },
      onCancel: () => resolve(null)
    });
  });
}

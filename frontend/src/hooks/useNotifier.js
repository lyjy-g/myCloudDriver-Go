import { notification } from "antd";
import { useCallback } from "react";

/**
 * 顶部通知封装。
 *
 * @returns {{contextHolder: import('react').ReactNode, notifyError: Function, notifySuccess: Function, notifyWarning: Function}}
 */
export function useNotifier() {
  const [noticeApi, contextHolder] = notification.useNotification();

  const notifyError = useCallback((description) => {
    noticeApi.error({
      message: "操作失败",
      description,
      placement: "top",
      duration: 5
    });
  }, [noticeApi]);

  const notifySuccess = useCallback((description) => {
    noticeApi.success({
      message: "操作成功",
      description,
      placement: "top",
      duration: 5
    });
  }, [noticeApi]);

  const notifyWarning = useCallback((description) => {
    noticeApi.warning({
      message: "请注意",
      description,
      placement: "top",
      duration: 5
    });
  }, [noticeApi]);

  return {
    contextHolder,
    notifyError,
    notifySuccess,
    notifyWarning
  };
}

import { Button } from "antd";
import React from "react";

/**
 * 顶部栏。
 *
 * @param {{topPromos: Array<{key: string, label: string, tone: string}>}} props 组件参数
 * @returns {JSX.Element} 顶部栏
 */
export function Topbar({ topPromos }) {
  return (
    <header className="mcd-topbar">
      <div className="mcd-topbar-inner">
        <div className="flex items-center gap-3">
          <div className="mcd-avatar">M</div>
          {topPromos.map((promo) => (
            <div
              key={promo.key}
              className={
                promo.tone === "warm"
                  ? "mcd-pill mcd-pill-warm"
                  : promo.tone === "outline"
                  ? "mcd-pill mcd-pill-outline"
                  : "mcd-pill"
              }
            >
              {promo.label}
            </div>
          ))}
        </div>
        <div className="flex items-center gap-2">
          <Button size="small" className="mcd-pill mcd-pill-outline">
            用户
          </Button>
          <div className="mcd-avatar mcd-avatar-muted">LL</div>
        </div>
      </div>
    </header>
  );
}

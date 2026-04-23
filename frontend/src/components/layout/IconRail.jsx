import React from "react";

/**
 * 左侧图标栏。
 *
 * @param {{items: Array<{key: string, icon: JSX.Element, label: string, active?: boolean}>}} props 组件参数
 * @returns {JSX.Element} 图标栏
 */
export function IconRail({ items }) {
  return (
    <aside className="mcd-iconbar">
      {items.map((item) => {
        const RailIcon = item.icon;
        return (
          <button key={item.key} type="button" className={item.active ? "mcd-icon-btn active" : "mcd-icon-btn"}>
            {RailIcon ? <RailIcon /> : null}
            <span>{item.label}</span>
          </button>
        );
      })}
    </aside>
  );
}

// Shared brand header, reused at the top of BOTH the docked panel and any
// rail flyout, per steering: "brand header stays on top of the docked
// panel."
export default function PanelBrand() {
  return (
    <div className="sidebar-brand">
      <span className="mockup-brand" data-token-probe="--text-primary">
        TrueBDD
      </span>
      <div className="sidebar-brand__meta">Workspace — Inventory Spread</div>
    </div>
  );
}

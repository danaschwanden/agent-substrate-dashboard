import { LitElement, css, html } from "lit";
import { cardStyles } from "../styles/card-styles.js";
import type { Efficiency, Worker } from "../types.js";
import { formatRatio, formatResource } from "../utils.js";

export class OperationalEfficiencyCard extends LitElement {
  static properties = { efficiency: { attribute: false }, workers: { attribute: false } };
  static styles = [cardStyles, css`
    .card { --accent: var(--sd-yellow); padding: 13px; }
    .metrics { display: grid; grid-template-columns: 1fr 1fr; gap: 18px; align-items: center; height: calc(100% - 20px); text-align: center; }
    .metric-group + .metric-group { border-left: 1px solid var(--sd-line); }
    .ratio { color: var(--sd-blue); font-size: clamp(22px, 6cqi, 31px); font-weight: 750; letter-spacing: -.04em; }
    .density { color: var(--sd-green); }
    .context { margin-top: 7px; color: var(--sd-muted); font-size: 8px; letter-spacing: .06em; text-transform: uppercase; }
    .resources { display: flex; justify-content: center; gap: 12px; margin-top: 7px; color: var(--sd-muted); font-size: 8px; }
    @container (max-width: 480px) { .metrics { grid-template-columns: 1fr; } .metric-group + .metric-group { border-top: 1px solid var(--sd-line); border-left: 0; padding-top: 8px; } .resources { display: none; } }
  `];

  efficiency?: Efficiency;
  workers: Worker[] = [];

  protected render() {
    const fleet = this.efficiency?.fleetRatio;
    const density = this.efficiency?.observedDensity;
    const resources = this.resourceTotals();
    return html`
      <section class="card" aria-labelledby="efficiency-title">
        <h2 id="efficiency-title" class="section-title">Operational efficiency</h2>
        <div class="metrics">
          <div class="metric-group">
            <div class="eyebrow">Fleet ratio</div>
            <div class="ratio">${fleet?.numerator ?? 0}:${fleet?.denominator ?? 0}</div>
            <div class="context">${formatRatio(fleet?.value)} actors per selected worker</div>
          </div>
          <div class="metric-group">
            <div class="eyebrow">Observed density</div>
            <div class="ratio density">${density?.available ? `${density.actorsAtPeak}:${density.occupiedWorkersAtPeak}` : "Unavailable"}</div>
            <div class="context">Peak occupied demand · ${density?.sampleAgeSeconds ?? 0}s sample age · ${density ? density.windowSeconds / 60 : 0}m window</div>
            ${resources.length ? html`<div class="resources">${resources.map(item => html`<span>${item.name} ${formatResource(item.allocated, item.unit)} / ${formatResource(item.capacity, item.unit)}</span>`)}</div>` : ""}
          </div>
        </div>
      </section>
    `;
  }

  private resourceTotals() {
    const totals = new Map<string, { name: string; unit: string; capacity: bigint; allocated: bigint }>();
    for (const worker of this.workers) {
      for (const resource of worker.resources) {
        const key = `${resource.name}:${resource.unit}`;
        const current = totals.get(key) ?? { name: resource.name, unit: resource.unit, capacity: 0n, allocated: 0n };
        current.capacity += BigInt(resource.capacity);
        current.allocated += BigInt(resource.allocated);
        totals.set(key, current);
      }
    }
    return [...totals.values()].slice(0, 2).map(value => ({ ...value, capacity: String(value.capacity), allocated: String(value.allocated) }));
  }
}

customElements.define("operational-efficiency-card", OperationalEfficiencyCard);

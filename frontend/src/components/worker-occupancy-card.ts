import { LitElement, css, html } from "lit";
import { cardStyles } from "../styles/card-styles.js";
import type { WorkerSummary } from "../types.js";

export class WorkerOccupancyCard extends LitElement {
  static properties = { summary: { attribute: false } };
  static styles = [cardStyles, css`
    .card { --accent: var(--sd-pink); display: grid; place-items: center; padding: 14px; text-align: center; }
    .detail { margin-top: 9px; color: var(--sd-muted); font-size: 9px; letter-spacing: .06em; text-transform: uppercase; }
    .full { color: var(--sd-pink); }
  `];

  summary?: WorkerSummary;

  protected render() {
    const total = this.summary?.total ?? 0;
    const occupied = this.summary?.occupied ?? 0;
    return html`
      <section class="card" aria-labelledby="occupancy-title">
        <div>
          <div id="occupancy-title" class="eyebrow">Workers occupied</div>
          <div class="metric ${total > 0 && occupied === total ? "full" : ""}">${occupied}/${total}</div>
          <div class="detail">${this.summary?.assignments ?? 0} actor assignments</div>
        </div>
      </section>
    `;
  }
}

customElements.define("worker-occupancy-card", WorkerOccupancyCard);

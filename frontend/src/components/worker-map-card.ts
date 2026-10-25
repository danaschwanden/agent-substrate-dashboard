import { LitElement, css, html } from "lit";
import { cardStyles } from "../styles/card-styles.js";
import type { TruncationDetail, Worker } from "../types.js";
import { formatResource, stateClass, titleCase } from "../utils.js";

export class WorkerMapCard extends LitElement {
  static properties = { workers: { attribute: false }, truncation: { attribute: false } };
  static styles = [cardStyles, css`
    .card { --accent: var(--sd-green); display: flex; flex-direction: column; padding: 13px; }
    .list { display: grid; gap: 6px; min-height: 0; margin-top: 10px; overflow: auto; }
    .worker { display: grid; grid-template-columns: minmax(100px, 1fr) minmax(110px, 1.4fr) auto; gap: 10px; align-items: center; padding: 8px 10px; border: 1px solid color-mix(in srgb, var(--state-color) 45%, var(--sd-line)); border-left: 3px solid var(--state-color); border-radius: 5px; background: #060b0e; }
    .identity strong { display: block; overflow: hidden; font-size: 10px; text-overflow: ellipsis; white-space: nowrap; }
    .identity span, .assignments { color: var(--sd-muted); font-size: 8px; }
    .assignments { overflow: hidden; text-align: right; text-overflow: ellipsis; white-space: nowrap; }
    .resource { margin-top: 4px; }
    .resource-label { display: flex; justify-content: space-between; color: var(--sd-muted); font-size: 7px; }
    .track { height: 3px; margin-top: 2px; overflow: hidden; border-radius: 2px; background: #263137; }
    .fill { height: 100%; background: var(--state-color); }
    .notice { margin-top: 7px; color: var(--sd-yellow); font-size: 8px; }
    @container (max-width: 520px) { .worker { grid-template-columns: 1fr auto; } .resources { display: none; } }
  `];

  workers: Worker[] = [];
  truncation?: TruncationDetail;

  protected render() {
    return html`
      <section class="card" aria-labelledby="worker-map-title">
        <h2 id="worker-map-title" class="section-title">Worker map</h2>
        <div class="list">
          ${this.workers.length ? this.workers.map(worker => this.renderWorker(worker)) : html`<div class="muted">No selected workers observed.</div>`}
        </div>
        ${this.truncation?.truncated ? html`<div class="notice">Showing ${this.truncation.returned} of ${this.truncation.total} workers.</div>` : ""}
      </section>
    `;
  }

  private renderWorker(worker: Worker) {
    return html`
      <article class="worker ${stateClass(worker.state)}">
        <div class="identity"><strong>${worker.pod}</strong><span>${worker.node} · ${titleCase(worker.state)}</span></div>
        <div class="resources">${worker.resources.slice(0, 2).map(resource => {
          const capacity = Number(resource.capacity);
          const percent = capacity > 0 ? Math.min(100, Number(resource.allocated) / capacity * 100) : 0;
          return html`<div class="resource"><div class="resource-label"><span>${resource.name}</span><span>${formatResource(resource.allocated, resource.unit)} / ${formatResource(resource.capacity, resource.unit)}</span></div><div class="track"><div class="fill" style=${`width:${percent}%`}></div></div></div>`;
        })}</div>
        <div class="assignments">${worker.actorNames.length ? worker.actorNames.join(", ") : "0 actors"}</div>
      </article>
    `;
  }
}

customElements.define("worker-map-card", WorkerMapCard);

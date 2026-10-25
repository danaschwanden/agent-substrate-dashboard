import { LitElement, css, html } from "lit";
import { cardStyles } from "../styles/card-styles.js";
import type { DashboardOperation } from "../types.js";
import { formatClock, titleCase } from "../utils.js";

export class OperationStreamCard extends LitElement {
  static properties = { operations: { attribute: false } };
  static styles = [cardStyles, css`
    .card { --accent: var(--sd-green); display: flex; flex-direction: column; padding: 13px; }
    .stream { min-height: 0; margin-top: 8px; overflow: auto; background: #030709; }
    .operation { padding: 8px 10px; border-bottom: 1px solid #1f2b30; font-size: 8px; }
    .operation-head { display: flex; justify-content: space-between; gap: 8px; color: var(--sd-blue); }
    .result { display: grid; grid-template-columns: minmax(80px, .7fr) 62px 1fr; gap: 7px; margin-top: 5px; color: var(--sd-muted); }
    .succeeded { color: var(--sd-green); } .failed { color: var(--sd-red); }
    .empty { padding: 12px; color: var(--sd-muted); }
    @container (max-width: 430px) { .result { grid-template-columns: 1fr auto; } .message { grid-column: 1 / -1; } }
  `];

  operations: DashboardOperation[] = [];

  protected render() {
    return html`
      <section class="card" aria-labelledby="operation-title">
        <h2 id="operation-title" class="section-title">Dashboard operations</h2>
        <div class="stream">
          ${this.operations.length ? this.operations.map(operation => html`
            <article class="operation">
              <div class="operation-head"><strong>${titleCase(operation.type)} · ${operation.id}</strong><time datetime=${operation.completedAt}>${formatClock(operation.completedAt)}</time></div>
              ${operation.results.map(result => html`<div class="result"><span>${result.actorName}</span><span class=${result.outcome.toLowerCase()}>${titleCase(result.outcome)}</span><span class="message">${result.message || "completed"}</span></div>`)}
            </article>
          `) : html`<div class="empty">No dashboard operations since backend start.</div>`}
        </div>
      </section>
    `;
  }
}

customElements.define("operation-stream-card", OperationStreamCard);

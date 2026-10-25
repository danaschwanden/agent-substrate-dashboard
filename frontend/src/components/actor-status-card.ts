import { LitElement, css, html } from "lit";
import { cardStyles } from "../styles/card-styles.js";
import type { ActorSummary } from "../types.js";
import { stateClass, titleCase } from "../utils.js";

export class ActorStatusCard extends LitElement {
  static properties = { summary: { attribute: false } };
  static styles = [cardStyles, css`
    .card { --accent: var(--sd-blue); display: grid; place-items: center; padding: 14px; text-align: center; }
    .states { display: flex; flex-wrap: wrap; justify-content: center; gap: 5px 10px; margin-top: 10px; }
    .state-count { font-size: 9px; letter-spacing: .05em; text-transform: uppercase; }
  `];

  summary?: ActorSummary;

  protected render() {
    const primary = this.summary?.states.find(({ count }) => count > 0);
    return html`
      <section class="card" aria-labelledby="actor-status-title">
        <div>
          <div id="actor-status-title" class="eyebrow">Actor status · ${this.summary?.total ?? 0} managed</div>
          <div class="metric ${primary ? stateClass(primary.state) : ""}">${primary ? titleCase(primary.state) : "Empty"}</div>
          <div class="states" aria-label="Actor state counts">
            ${(this.summary?.states ?? []).map(({ state, count }) => html`<span class="state-count ${stateClass(state)}">${count} ${titleCase(state)}</span>`)}
          </div>
        </div>
      </section>
    `;
  }
}

customElements.define("actor-status-card", ActorStatusCard);

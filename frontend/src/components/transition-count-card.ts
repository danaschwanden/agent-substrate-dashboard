import { LitElement, css, html } from "lit";
import { cardStyles } from "../styles/card-styles.js";
import type { TransitionSummary } from "../types.js";

export class TransitionCountCard extends LitElement {
  static properties = { summary: { attribute: false } };
  static styles = [cardStyles, css`
    .card { --accent: var(--sd-blue); display: grid; place-items: center; padding: 14px; text-align: center; }
    .split { display: flex; justify-content: center; gap: 13px; margin-top: 9px; color: var(--sd-muted); font-size: 9px; letter-spacing: .06em; text-transform: uppercase; }
  `];

  summary?: TransitionSummary;

  protected render() {
    return html`
      <section class="card" aria-labelledby="transition-title">
        <div>
          <div id="transition-title" class="eyebrow">Observed transitions</div>
          <div class="metric">${this.summary?.total ?? 0}</div>
          <div class="split"><span>${this.summary?.resumes ?? 0} resumes</span><span>${this.summary?.suspends ?? 0} suspends</span></div>
        </div>
      </section>
    `;
  }
}

customElements.define("transition-count-card", TransitionCountCard);

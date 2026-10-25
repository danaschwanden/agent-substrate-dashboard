import { LitElement, css, html } from "lit";
import { cardStyles } from "../styles/card-styles.js";
import type { ControlApiConnection } from "../types.js";
import { stateClass, titleCase } from "../utils.js";

export class ControlApiCard extends LitElement {
  static properties = { connection: { attribute: false }, frontendStale: { type: Boolean, attribute: "frontend-stale" } };
  static styles = [cardStyles, css`
    .card { --accent: var(--sd-green); display: grid; place-items: center; padding: 14px; text-align: center; }
    .detail { max-width: 30ch; margin-top: 9px; overflow: hidden; color: var(--sd-muted); font-size: 9px; letter-spacing: .06em; text-overflow: ellipsis; text-transform: uppercase; white-space: nowrap; }
  `];

  connection?: ControlApiConnection;
  frontendStale = false;

  protected render() {
    const state = this.frontendStale && this.connection?.state === "CONNECTED" ? "STALE" : (this.connection?.state ?? "UNAVAILABLE");
    return html`
      <section class="card" aria-labelledby="control-title">
        <div>
          <div id="control-title" class="eyebrow">Control API</div>
          <div class="metric ${stateClass(state)}">${titleCase(state)}</div>
          <div class="detail" title=${this.connection?.message ?? "Awaiting observation"}>${this.connection?.message || "Awaiting observation"}</div>
        </div>
      </section>
    `;
  }
}

customElements.define("control-api-card", ControlApiCard);

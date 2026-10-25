import { css } from "lit";

export const cardStyles = css`
  :host {
    --sd-bg: #080e11;
    --sd-panel: #11191e;
    --sd-panel-raised: #151f25;
    --sd-line: #2a383f;
    --sd-text: #e3eaed;
    --sd-muted: #89979e;
    --sd-green: #45d36b;
    --sd-blue: #66c3ff;
    --sd-pink: #ff64a5;
    --sd-yellow: #f4c84d;
    --sd-red: #ff6b6b;
    --sd-cyan: #5be0e6;
    --sd-radius: 7px;
    display: block;
    min-width: 0;
    min-height: 0;
    color: var(--sd-text);
    font-family: "SFMono-Regular", "Cascadia Code", "Roboto Mono", ui-monospace, monospace;
    container-type: inline-size;
  }

  .card {
    box-sizing: border-box;
    height: 100%;
    min-height: 0;
    overflow: hidden;
    border: 1px solid var(--sd-line);
    border-radius: var(--sd-radius);
    background: linear-gradient(145deg, color-mix(in srgb, var(--sd-panel-raised) 84%, transparent), var(--sd-panel));
    box-shadow: inset 0 1px 0 rgb(255 255 255 / 0.025), 0 12px 32px rgb(0 0 0 / 0.12);
  }

  .section-title {
    display: flex;
    align-items: center;
    gap: 8px;
    margin: 0;
    color: var(--sd-muted);
    font-size: 10px;
    font-weight: 700;
    letter-spacing: 0.12em;
    line-height: 1;
    text-transform: uppercase;
  }

  .section-title::before {
    width: 2px;
    height: 16px;
    border-radius: 1px;
    background: var(--accent, var(--sd-green));
    content: "";
  }

  .eyebrow {
    color: var(--sd-muted);
    font-size: 9px;
    font-weight: 700;
    letter-spacing: 0.13em;
    text-transform: uppercase;
  }

  .metric {
    color: var(--accent, var(--sd-blue));
    font-size: clamp(21px, 7cqi, 30px);
    font-weight: 760;
    letter-spacing: -0.04em;
    line-height: 1;
  }

  .muted {
    color: var(--sd-muted);
  }

  .state {
    --state-color: var(--sd-muted);
    color: var(--state-color);
  }

  .state--running,
  .state--active,
  .state--connected {
    --state-color: var(--sd-green);
  }

  .state--resuming,
  .state--reverting {
    --state-color: var(--sd-blue);
  }

  .state--suspending,
  .state--stale,
  .state--draining {
    --state-color: var(--sd-yellow);
  }

  .state--crashed,
  .state--unavailable {
    --state-color: var(--sd-red);
  }

  .state--deleting {
    --state-color: var(--sd-pink);
  }

  .badge {
    display: inline-flex;
    align-items: center;
    min-height: 17px;
    padding: 0 6px;
    border: 1px solid color-mix(in srgb, var(--state-color, var(--sd-muted)) 58%, transparent);
    border-radius: 4px;
    color: var(--state-color, var(--sd-muted));
    background: color-mix(in srgb, var(--state-color, var(--sd-muted)) 8%, transparent);
    font-size: 8px;
    font-weight: 700;
    letter-spacing: 0.06em;
    line-height: 1;
    text-transform: uppercase;
  }

  @media (prefers-reduced-motion: reduce) {
    *,
    *::before,
    *::after {
      scroll-behavior: auto !important;
      animation-duration: 0.001ms !important;
      animation-iteration-count: 1 !important;
      transition-duration: 0.001ms !important;
    }
  }
`;

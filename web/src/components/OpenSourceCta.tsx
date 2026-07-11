import { appConfig } from '../appConfig'
import { GitHubIcon, InfoCircleIcon } from './Icons'

/** Round 6: persistent open-source / self-host CTA above the footer on
 * every view — a dark companion band to the "Don't trust. Verify." node
 * CTA, pointing at the public repo. */
export function OpenSourceCta() {
  return (
    <section className="bp-oss-section">
      <div className="bp-oss-card">
        <div>
          <div className="bp-oss-pill">FREE &amp; OPEN SOURCE</div>
          <h2>Host btcd.watch yourself.</h2>
          <p>
            Every line of this app is public. Clone it, point it at your own
            btcd node, and run a fully self-hosted explorer — no third party
            in the loop. Found a bug or have an idea? Open an issue on
            GitHub.
          </p>
        </div>
        <div className="bp-oss-actions">
          <a
            className="bp-oss-btn bp-oss-btn--primary"
            href={appConfig.repoUrl}
            target="_blank"
            rel="noreferrer"
          >
            <GitHubIcon size={18} />
            Self-host on GitHub ↗
          </a>
          <a
            className="bp-oss-btn bp-oss-btn--ghost"
            href={appConfig.issuesUrl}
            target="_blank"
            rel="noreferrer"
          >
            <InfoCircleIcon size={16} />
            Report an issue ↗
          </a>
        </div>
      </div>
    </section>
  )
}

import { appConfig } from '../appConfig'
import { GitHubIcon } from './Icons'

export function Footer({ network }: { network: string | null }) {
  return (
    <footer className="bp-footer">
      <div className="bp-footer-inner">
        <div className="bp-footer-brand">
          <span className="bp-footer-mark">₿</span>
          <span className="bp-footer-name">btcd.watch</span>
          <span className="bp-footer-note">
            — powered by your own <strong>btcd</strong> node
          </span>
        </div>
        <div className="bp-footer-links">
          <a
            className="bp-footer-link"
            href={appConfig.repoUrl}
            target="_blank"
            rel="noreferrer"
          >
            <GitHubIcon size={15} />
            GitHub
          </a>
          <a
            className="bp-footer-link"
            href={appConfig.issuesUrl}
            target="_blank"
            rel="noreferrer"
          >
            Report an issue →
          </a>
        </div>
      </div>
      <div className="bp-footer-legal">
        {network && network !== 'mainnet'
          ? `Connected to a ${network} node · amounts are test coins`
          : 'Data comes straight from your btcd node · not financial advice'}
      </div>
    </footer>
  )
}

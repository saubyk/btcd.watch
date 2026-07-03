import { appConfig } from '../appConfig'

export function Footer({ network }: { network: string | null }) {
  return (
    <footer className="bp-footer">
      <div className="bp-footer-inner">
        <div className="bp-footer-brand">
          <span className="bp-footer-mark">₿</span>
          <span className="bp-footer-name">btcdwatch.com</span>
          <span className="bp-footer-note">
            — powered by your own <strong>btcd</strong> node
          </span>
        </div>
        <a
          className="bp-footer-link"
          href={appConfig.btcdUrl}
          target="_blank"
          rel="noreferrer"
        >
          Run your own btcd node →
        </a>
      </div>
      <div className="bp-footer-legal">
        {network && network !== 'mainnet'
          ? `Connected to a ${network} node · amounts are test coins`
          : 'Data comes straight from your btcd node · not financial advice'}
      </div>
    </footer>
  )
}

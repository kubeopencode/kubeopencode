import type {ReactNode} from 'react';
import clsx from 'clsx';
import Heading from '@theme/Heading';
import styles from './styles.module.css';

type FeatureItem = {
  title: string;
  icon: string;
  description: ReactNode;
};

const FeatureList: FeatureItem[] = [
  {
    title: 'Kubernetes-Native',
    icon: '\u2638\uFE0F',
    description: (
      <>
        Declarative CRDs, GitOps-friendly, works with Helm, Kustomize, and
        ArgoCD. No new tools to learn &mdash; just <code>kubectl apply</code>.
      </>
    ),
  },
  {
    title: 'Built for Teams',
    icon: '\uD83D\uDC65',
    description: (
      <>
        Shared agent configurations, batch operations across repositories,
        concurrency control, and centralized credential management &mdash; so
        your entire team can leverage AI agents with consistent standards.
      </>
    ),
  },
  {
    title: 'Enterprise Ready',
    icon: '\uD83C\uDFE2',
    description: (
      <>
        RBAC, private registries, corporate proxies, custom CA certificates,
        pod security policies, and audit-ready infrastructure &mdash; meeting
        the governance and compliance requirements your organization demands.
      </>
    ),
  },
  {
    title: 'Zero Dependencies',
    icon: '\uD83D\uDCE6',
    description: (
      <>
        No PostgreSQL, no Redis, no message queues. Uses etcd for state and
        Pods for execution &mdash; your existing Kubernetes cluster is all
        you need.
      </>
    ),
  },
];

function Feature({title, icon, description}: FeatureItem) {
  return (
    <div className={clsx('col col--3')}>
      <div className="text--center">
        <div className={styles.featureIcon}>{icon}</div>
      </div>
      <div className="text--center padding-horiz--md">
        <Heading as="h3">{title}</Heading>
        <p>{description}</p>
      </div>
    </div>
  );
}

export default function HomepageFeatures(): ReactNode {
  return (
    <section className={styles.features}>
      <div className="container">
        <div className="row">
          {FeatureList.map((props, idx) => (
            <Feature key={idx} {...props} />
          ))}
        </div>
      </div>
    </section>
  );
}

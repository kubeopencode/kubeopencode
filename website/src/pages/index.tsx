import type {ReactNode} from 'react';
import clsx from 'clsx';
import Link from '@docusaurus/Link';
import useDocusaurusContext from '@docusaurus/useDocusaurusContext';
import Layout from '@theme/Layout';
import HomepageFeatures from '@site/src/components/HomepageFeatures';
import Heading from '@theme/Heading';
import CodeBlock from '@theme/CodeBlock';

import styles from './index.module.css';

const exampleYaml = `apiVersion: kubeopencode.io/v1alpha1
kind: Task
metadata:
  name: update-dependencies
spec:
  agentRef:
    name: default
  description: |
    Update all dependencies to latest versions.
    Run tests and create a pull request.`;

function HomepageHeader() {
  const {siteConfig} = useDocusaurusContext();
  return (
    <header className={clsx('hero hero--primary', styles.heroBanner)}>
      <div className="container">
        <Heading as="h1" className="hero__title">
          {siteConfig.title}
        </Heading>
        <p className="hero__subtitle">{siteConfig.tagline}</p>
        <p className={styles.heroDescription}>
          Deploy, manage, and govern AI coding agents at scale on Kubernetes.
          Built on OpenCode &mdash; turning individual AI capabilities into a shared platform
          for your entire engineering organization.
        </p>
        <div className={styles.buttons}>
          <Link
            className="button button--secondary button--lg"
            to="/docs/getting-started">
            Get Started
          </Link>
          <Link
            className="button button--outline button--lg"
            style={{color: 'white', borderColor: 'white', marginLeft: '1rem'}}
            href="https://github.com/kubeopencode/kubeopencode">
            GitHub
          </Link>
        </div>
      </div>
    </header>
  );
}

function QuickExample() {
  return (
    <section className={styles.quickExample}>
      <div className="container">
        <div className="row">
          <div className="col col--6">
            <Heading as="h2">Define Tasks as YAML</Heading>
            <p>
              KubeOpenCode brings AI coding agents into your Kubernetes cluster.
              Define what you want done as a Task, configure how it runs with an
              Agent, and let the controller handle execution.
            </p>
            <ul>
              <li>No new tools to learn &mdash; just <code>kubectl apply</code></li>
              <li>Works with any CI/CD pipeline</li>
              <li>Scale with Helm templates for batch operations</li>
              <li>Monitor with standard Kubernetes tooling</li>
            </ul>
          </div>
          <div className="col col--6">
            <CodeBlock language="yaml" title="task.yaml">
              {exampleYaml}
            </CodeBlock>
          </div>
        </div>
      </div>
    </section>
  );
}

export default function Home(): ReactNode {
  return (
    <Layout
      title="Kubernetes-native Agent Platform for Teams and Enterprise"
      description="Deploy, manage, and govern AI coding agents at scale on Kubernetes. Built on OpenCode, designed for teams and enterprise.">
      <HomepageHeader />
      <main>
        <HomepageFeatures />
        <QuickExample />
      </main>
    </Layout>
  );
}
